// Integration test utilities. May import anything except commands, but will cause import loops in those packages.
package itestutil

import (
	"io"
	"strings"
	"testing"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/jarcoal/httpmock"
)

func RegisterFeedResponders(config *config.Config) error {
	httpmock.RegisterResponder("GET", config.Feeds.LauncherRelease, httpmock.NewStringResponder(200, testutil.SampleLauncherRelease))
	httpmock.RegisterResponder("GET", config.Feeds.LauncherArticles, httpmock.NewStringResponder(200, testutil.SampleArticlesFeed))

	var patchlines = []struct {
		patchline        hytale.Patchline
		sampleReleaseURL string
		releaseURL       string
		sampleRelease    string
		sampleMaven      string
	}{
		{hytale.Release, testutil.SampleReleaseURL, testutil.ReleaseURL, testutil.SampleRelease, testutil.SampleMaven},
		{hytale.PreRelease, testutil.SamplePreReleaseURL, testutil.PreReleaseURL, testutil.SamplePreRelease, testutil.SampleMaven},
	}

	for _, tt := range patchlines {
		httpmock.RegisterResponder("GET", config.Feeds.GameVersion+tt.patchline.ID()+".json", httpmock.NewStringResponder(200, tt.sampleReleaseURL))
		httpmock.RegisterResponder("GET", tt.releaseURL, httpmock.NewStringResponder(200, tt.sampleRelease))
		httpmock.RegisterResponder("GET", hytale.MavenMetadataUrl(tt.patchline, config.Feeds), httpmock.NewStringResponder(200, tt.sampleMaven))
	}
	return nil
}

func FindField(embed *discordgo.MessageEmbed, name string) *discordgo.MessageEmbedField {
	for _, field := range embed.Fields {
		if field.Name == name {
			return field
		}
	}
	return nil
}

// Finds and returns the main text content of the response. The "main text content" is:
//   - The `Content` string
//   - The description of any `Embeds`
//   - The contents of any `Files`
//   - (attachments are not supported)
//
// If text content is found in more than one place, they are joined with newlines.
func ExtractMainContent(data *discordgo.InteractionResponseData) string {
	var result []string

	if data.Content != "" {
		result = append(result, data.Content)
	}

	for _, embed := range data.Embeds {
		if embed.Description != "" {
			result = append(result, embed.Description)
		}
	}

	for _, file := range data.Files {
		buf, err := io.ReadAll(file.Reader)
		if err != nil {
			result = append(result, string(buf))
		}
	}

	return strings.Join(result, "\n")
}

// Asserts that **either** the embed's author name or title matches the expected string.
func AssertEmbedTitle(t *testing.T, expected string, actual *discordgo.MessageEmbed) {
	if actual == nil {
		t.Fatal("embed is nil")
	}
	if actual.Title == expected {
		return
	}
	if actual.Author != nil {
		authorName := actual.Author.Name
		if authorName != expected {
			t.Fatalf("embed title `%q`, but is `%q` (or author name `%q`)", expected, actual.Title, authorName)
		}
	} else {
		t.Fatalf("embed title `%q`, but is `%q` (or author name nil)", expected, actual.Title)
	}
}
