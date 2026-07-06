package cmd

import (
	"encoding/xml"
	"fmt"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/testutil/itestutil"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	versionsDB       *db.DB
	versionsTestCase = testutil.MakeTestCase(beforeEachVersions, nil)
)

func setupVersions() {
	db, err := db.NewDB(config.ValkeyConfig{
		Address:       "127.0.0.1",
		Port:          9999,
		DatabaseIndex: 2,
	})
	if err != nil {
		panic(err)
	}
	versionsDB = db
}

func teardownVersions() {
	versionsDB.Close()
}

func beforeEachVersions() {
	versionsDB.Clear()
}

func setPatchlines(ce *MockExecutor, patchlines ...string) {
	patchlinesMap := make(map[string]*time.Time)
	for _, patchline := range patchlines {
		patchlinesMap[patchline] = nil
	}
	ce.WithPatchlinesFeed(&hytale.PatchlinesFeed{
		Patchlines: patchlinesMap,
	})
}

func setVersion(ce *MockExecutor, side hytale.Side, patchline string, version string) {
	if side == hytale.Client {
		ce.WithGameFeed(&hytale.GameFeed{
			Version: &hytale.GameReleaseVersion{
				Version: version,
			},
			Patchline: patchline,
		}, patchline)
	} else {
		ce.WithMavenFeed(&hytale.MavenFeed{
			Version: &hytale.MavenVersioning{
				XMLName: xml.Name{
					Local: "versioning",
				},
				Latest: version,
				Versions: []hytale.MavenVersion{
					{
						Version: version,
					},
				},
			},
			Patchline: patchline,
		}, patchline)
	}
}

func TestVersion(t *testing.T) {
	setupVersions()
	t.Cleanup(teardownVersions)

	t.Run("/version client", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		version := "2026.01.17-4b0f30090"
		setVersion(ce, hytale.Client, "release", version)

		ctx := NewMockContext(ce).WithOptionSubCommand("client")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Client")
		itestutil.AssertEmbedTitleContains(t, embed, "Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version client patchline=pre-release", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		version := "2026.08.14-1a2b3c4d5"
		setVersion(ce, hytale.Client, "pre-release", version)

		ctx := NewMockContext(ce).WithOptionSubCommand("client")
		ctx = ctx.WithOptionString("patchline", "pre-release")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Client")
		itestutil.AssertEmbedTitleContains(t, embed, "Pre Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version server", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		version := "1.6.25"
		setVersion(ce, hytale.Server, "release", version)

		ctx := NewMockContext(ce).WithOptionSubCommand("server")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Server")
		itestutil.AssertEmbedTitleContains(t, embed, "Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version server patchline=pre-release", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		version := "1.1.0"
		setVersion(ce, hytale.Server, "pre-release", version)

		ctx := NewMockContext(ce).WithOptionSubCommand("server")
		ctx = ctx.WithOptionString("patchline", "pre-release")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Server")
		itestutil.AssertEmbedTitleContains(t, embed, "Pre Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("invalid patchline", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")

		ctx := NewMockContext(ce).WithOptionSubCommand("client")
		ctx = ctx.WithOptionString("patchline", "invalid")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "Patchline must be one of")
	}))

	t.Run("missing feed", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)

		ctx := NewMockContext(ce).WithOptionSubCommand("client")
		ctx.RunCommand("version")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "Could not retrieve the latest Hytale version")
	}))
}

func setLauncherRelease(ce *MockExecutor, version string) {
	ce.WithLauncherReleaseFeed(&hytale.LauncherReleaseFeed{
		Release: &hytale.LauncherRelease{
			Version: version,
		},
	})
}

func TestLauncher(t *testing.T) {
	setupVersions()
	t.Cleanup(teardownVersions)

	t.Run("/launcher success", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		version := "2026.01.12-e43ec47"
		setLauncherRelease(ce, version)

		ctx := NewMockContext(ce)
		ctx.RunCommand("launcher")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Equal(t, 1, len(ctx.replies[0].Embeds))
		embed := ctx.replies[0].Embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Latest Hytale Launcher Version")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/launcher missing feed", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)

		ctx := NewMockContext(ce)
		ctx.RunCommand("launcher")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "Could not retrieve the latest Hytale Launcher version")
	}))
}

func setLauncherArticles(ce *MockExecutor, articles *hytale.ArticleList) {
	ce.WithLauncherPostFeed(&hytale.LauncherPostFeed{
		Articles: articles,
	})
}

func verifyButton(component discordgo.MessageComponent, expectedLabel string) discordgo.Button {
	button, ok := component.(discordgo.Button)
	if !ok {
		panic(fmt.Errorf("component %s not a button", component))
	}
	if button.Label != expectedLabel {
		panic(fmt.Errorf("expected button label %s but got %s", expectedLabel, button.Label))
	}
	return button
}

func extractButtons(components []discordgo.MessageComponent) (discordgo.Button, discordgo.Button) {
	if len(components) < 1 {
		panic("no components found")
	}
	actionRow, ok := components[0].(discordgo.ActionsRow)
	if !ok {
		panic("action row not in first component slot")
	}
	subComponents := actionRow.Components
	if len(subComponents) != 2 {
		panic("action row does not contain exactly two components")
	}
	return verifyButton(subComponents[0], "Back"), verifyButton(subComponents[1], "Forward")
}

func TestArticles(t *testing.T) {
	setupVersions()
	t.Cleanup(teardownVersions)

	t.Run("no articles", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		articles := &hytale.ArticleList{
			Articles: []*hytale.Article{},
		}
		setLauncherArticles(ce, articles)

		ctx := NewMockContext(ce)
		ctx.RunCommand("articles")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "No articles found.")
	}))

	t.Run("one article", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		articles := &hytale.ArticleList{
			Articles: []*hytale.Article{
				{
					Title:       "Test Article",
					Description: "Test description",
					DestURL:     "https://example.com",
					ImageURL:    "https://example.com/image.png",
				},
			},
		}
		setLauncherArticles(ce, articles)

		ctx := NewMockContext(ce)
		ctx.RunCommand("articles")

		assert.Equal(t, 1, len(ctx.replies))
		assert.Equal(t, 1, len(ctx.replies[0].Embeds))
		embed := ctx.replies[0].Embeds[0]
		assert.Equal(t, "Test Article", embed.Title)
		assert.Equal(t, "Test description", embed.Description)
		back, forward := extractButtons(ctx.replies[0].Components)
		assert.True(t, back.Disabled)
		assert.True(t, forward.Disabled)
	}))

	t.Run("one article - buttons ignored", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		articles := &hytale.ArticleList{
			Articles: []*hytale.Article{
				{
					Title:       "Test Article",
					Description: "Test description",
					DestURL:     "https://example.com",
					ImageURL:    "https://example.com/image.png",
				},
			},
		}
		setLauncherArticles(ce, articles)

		ctx := NewMockContext(ce)
		ctx.RunCommand("articles")
		back, forward := extractButtons(ctx.replies[0].Components)

		ctx2 := NewMockContext(ce).WithComponent(back.CustomID)
		ctx2.RunInteraction()
		assert.Equal(t, 0, len(ctx2.edits)) // nothing changed

		ctx3 := NewMockContext(ce).WithComponent(forward.CustomID)
		ctx3.RunInteraction()
		assert.Equal(t, 0, len(ctx3.edits)) // nothing changed
	}))

	t.Run("two articles", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		articles := &hytale.ArticleList{
			Articles: []*hytale.Article{
				{
					Title:       "Latest",
					Description: "Latest article",
					DestURL:     "https://latest.com",
					ImageURL:    "https://latest.com/img.png",
				},
				{
					Title:       "Earlier",
					Description: "Earlier article",
					DestURL:     "https://earlier.com",
					ImageURL:    "https://earlier.com/img.png",
				},
			},
		}
		setLauncherArticles(ce, articles)

		// Run /articles
		ctx := NewMockContext(ce)
		ctx.RunCommand("articles")
		assert.Equal(t, 1, len(ctx.replies))
		assert.Equal(t, "Latest", ctx.replies[0].Embeds[0].Title)
		back, forward := extractButtons(ctx.replies[0].Components)
		assert.False(t, back.Disabled)
		assert.True(t, forward.Disabled)

		// Click back
		ctx2 := NewMockContext(ce).WithComponent(back.CustomID)
		ctx2.RunInteraction()
		assert.Equal(t, 1, len(ctx2.edits))
		assert.Equal(t, "Earlier", ctx2.edits[0].Embeds[0].Title)
		back2, forward2 := extractButtons(ctx2.edits[0].Components)
		assert.True(t, back2.Disabled)
		assert.False(t, forward2.Disabled)

		// Click forward
		ctx3 := NewMockContext(ce).WithComponent(forward2.CustomID)
		ctx3.RunInteraction()
		assert.Equal(t, 1, len(ctx3.edits))
		assert.Equal(t, "Latest", ctx3.edits[0].Embeds[0].Title)
		back3, forward3 := extractButtons(ctx3.edits[0].Components)
		assert.False(t, back3.Disabled)
		assert.True(t, forward3.Disabled)
	}))

	t.Run("three articles", versionsTestCase(func(t *testing.T) {
		ce, err := InitMockExecutorWithMockFeeds(nil, versionsDB)
		require.NoError(t, err)
		setPatchlines(ce, "release", "pre-release")
		articles := &hytale.ArticleList{
			Articles: []*hytale.Article{
				{
					Title:       "Latest",
					Description: "Latest article",
					DestURL:     "https://latest.com",
					ImageURL:    "https://latest.com/img.png",
				},
				{
					Title:       "Middle",
					Description: "Middle article",
					DestURL:     "https://middle.com",
					ImageURL:    "https://middle.com/img.png",
				},
				{
					Title:       "Earliest",
					Description: "Earliest article",
					DestURL:     "https://earliest.com",
					ImageURL:    "https://earliest.com/img.png",
				},
			},
		}
		setLauncherArticles(ce, articles)

		// Run /articles
		ctx := NewMockContext(ce)
		ctx.RunCommand("articles")
		assert.Equal(t, 1, len(ctx.replies))
		assert.Equal(t, "Latest", ctx.replies[0].Embeds[0].Title)
		back, forward := extractButtons(ctx.replies[0].Components)
		assert.False(t, back.Disabled)
		assert.True(t, forward.Disabled)

		// Click back to Middle
		ctx2 := NewMockContext(ce).WithComponent(back.CustomID)
		ctx2.RunInteraction()
		assert.Equal(t, 1, len(ctx2.edits))
		assert.Equal(t, "Middle", ctx2.edits[0].Embeds[0].Title)
		back2, forward2 := extractButtons(ctx2.edits[0].Components)
		assert.False(t, back2.Disabled)
		assert.False(t, forward2.Disabled)

		// Click back again to Earliest (index 2)
		ctx3 := NewMockContext(ce).WithComponent(back2.CustomID)
		ctx3.RunInteraction()
		assert.Equal(t, 1, len(ctx3.edits))
		assert.Equal(t, "Earliest", ctx3.edits[0].Embeds[0].Title)
		back3, forward3 := extractButtons(ctx3.edits[0].Components)
		assert.True(t, back3.Disabled)
		assert.False(t, forward3.Disabled)

		// Click forward to Middle (index 1)
		ctx4 := NewMockContext(ce).WithComponent(forward3.CustomID)
		ctx4.RunInteraction()
		assert.Equal(t, 1, len(ctx4.edits))
		assert.Equal(t, "Middle", ctx4.edits[0].Embeds[0].Title)
		back4, forward4 := extractButtons(ctx4.edits[0].Components)
		assert.False(t, back4.Disabled)
		assert.False(t, forward4.Disabled)

		// Click forward again to Latest (index 0)
		ctx5 := NewMockContext(ce).WithComponent(forward4.CustomID)
		ctx5.RunInteraction()
		assert.Equal(t, 1, len(ctx5.edits))
		assert.Equal(t, "Latest", ctx5.edits[0].Embeds[0].Title)
		back5, forward5 := extractButtons(ctx5.edits[0].Components)
		assert.False(t, back5.Disabled)
		assert.True(t, forward5.Disabled)
	}))
}
