package hytale

import (
	_ "embed"
	"encoding/xml"
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

var (
	//go:embed sample/release_url.json
	sampleReleaseUrl   string
	expectedReleaseUrl = "https://example-game-assets-release.r2.cloudflarestorage.com/version/release.json"

	//go:embed sample/release.json
	sampleRelease   string
	expectedRelease = GameReleaseFeed{
		Version: &GameReleaseVersion{
			Version: "2026.01.17-4b0f30090",
		},
		Patchline: Release,
	}

	//go:embed sample/maven-metadata.xml
	sampleMavenRelease   string
	expectedMavenRelease = MavenFeed{
		Version: &MavenVersioning{
			XMLName: xml.Name{
				Local: "versioning",
			},
			Latest: "0.5.4",
			Versions: []MavenVersion{
				{
					Version: "0.5.0",
				},
				{
					Version: "0.5.1",
				},
				{
					Version: "0.5.2",
				},
				{
					Version: "0.5.3",
				},
				{
					Version: "0.5.4",
				},
			},
			LastUpdated: "20260605155755",
		},
		Patchline: Release,
	}
)

func TestReleases(t *testing.T) {
	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)

	config := &config.Config{
		Feeds: config.FeedsConfig{
			GameVersion:   "https://account-data.example.com/game-assets/version/",
			MavenRepo:     "https://maven.example.com",
			MavenGroup:    "com.hypixel.hytale",
			MavenArtifact: "Server",
		},
	}
	authStore := testutil.NewTestAuthStore(http)
	feeds := &HytaleFeeds{
		config:    config,
		http:      http,
		authStore: authStore,
	}

	t.Run("game release", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.GameVersion+"release.json", httpmock.NewStringResponder(200, sampleReleaseUrl))
		url, err := fetchGameReleaseUrl(Release, feeds)
		assert.NoError(t, err)
		assert.Equal(t, url, expectedReleaseUrl)

		httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, sampleRelease))
		feed, err := fetchGameRelease(Release, feeds)
		assert.NoError(t, err)
		td.Cmp(t, feed, expectedRelease)
	})

	t.Run("maven release", func(t *testing.T) {
		httpmock.RegisterResponder("GET", mavenMetadataUrl(Release, config.Feeds), httpmock.NewStringResponder(200, sampleMavenRelease))
		feed, err := fetchMavenRelease(Release, feeds)
		assert.NoError(t, err)
		td.Cmp(t, feed, expectedMavenRelease)
	})
}
