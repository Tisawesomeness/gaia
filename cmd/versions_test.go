package cmd

import (
	"encoding/xml"
	"testing"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/itestutil"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/stretchr/testify/assert"
)

var (
	versionsCE       *CommandExecutor
	versionsTestCase = testutil.MakeTestCase(beforeEachVersions, nil)
)

func init() {
	ce, err := InitMockExecutor(nil)
	if err != nil {
		panic(err)
	}
	versionsCE = ce
}

func teardownVersions() {
	versionsCE.DB.Close()
}

func beforeEachVersions() {
	versionsCE.DB.Clear()
	versionsCE.HytaleFeeds.Feeds = make(map[hytale.FeedType]hytale.Feed)
}

func setVersion(side hytale.Side, patchline hytale.Patchline, version string) {
	feedType := hytale.GetFeedType(patchline, side)
	if side == hytale.Client {
		versionsCE.HytaleFeeds.Feeds[feedType] = hytale.GameReleaseFeed{
			Version: &hytale.GameReleaseVersion{
				Version: version,
			},
			Patchline: patchline,
		}
	} else {
		versionsCE.HytaleFeeds.Feeds[feedType] = hytale.MavenFeed{
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
				LastUpdated: "20260605155755",
			},
			Patchline: patchline,
		}
	}
}

func TestVersionsCommands(t *testing.T) {
	t.Cleanup(teardownVersions)

	t.Run("/version client", versionsTestCase(func(t *testing.T) {
		version := "2026.01.17-4b0f30090"
		setVersion(hytale.Client, hytale.Release, version)

		ctx := NewMockContext(versionsCE).WithOptionSubCommand("client")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Client")
		itestutil.AssertEmbedTitleContains(t, embed, "Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version client patchline=pre-release", versionsTestCase(func(t *testing.T) {
		version := "2026.08.14-1a2b3c4d5"
		setVersion(hytale.Client, hytale.PreRelease, version)

		ctx := NewMockContext(versionsCE).WithOptionSubCommand("client")
		ctx = ctx.WithOptionString("patchline", "pre-release")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Client")
		itestutil.AssertEmbedTitleContains(t, embed, "Pre-release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version server", versionsTestCase(func(t *testing.T) {
		version := "1.6.25"
		setVersion(hytale.Server, hytale.Release, version)

		ctx := NewMockContext(versionsCE).WithOptionSubCommand("server")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Server")
		itestutil.AssertEmbedTitleContains(t, embed, "Release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("/version server patchline=pre-release", versionsTestCase(func(t *testing.T) {
		version := "1.1.0"
		setVersion(hytale.Server, hytale.PreRelease, version)

		ctx := NewMockContext(versionsCE).WithOptionSubCommand("server")
		ctx = ctx.WithOptionString("patchline", "pre-release")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		embeds := ctx.replies[0].Embeds
		assert.Equal(t, 1, len(embeds))
		embed := embeds[0]
		itestutil.AssertEmbedTitleContains(t, embed, "Server")
		itestutil.AssertEmbedTitleContains(t, embed, "Pre-release")
		assert.Contains(t, embed.Description, version)
	}))

	t.Run("invalid patchline", versionsTestCase(func(t *testing.T) {
		ctx := NewMockContext(versionsCE).WithOptionSubCommand("client")
		ctx = ctx.WithOptionString("patchline", "invalid")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "Invalid patchline")
	}))

	t.Run("missing feed", versionsTestCase(func(t *testing.T) {
		ctx := NewMockContext(versionsCE).WithOptionSubCommand("client")
		getCommand("version").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		assert.Contains(t, ctx.replies[0].Content, "Could not retrieve the latest Hytale version")
	}))
}
