package hytale

import (
	_ "embed"
	"encoding/xml"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil/atestutil"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

var (
	expectedReleaseUrl    = "https://example-game-assets-release.r2.cloudflarestorage.com/version/release.json"
	expectedPreReleaseUrl = "https://example-game-assets-release.r2.cloudflarestorage.com/version/pre-release.json"

	expectedRelease = &GameFeed{
		Version: &GameReleaseVersion{
			Version: "2026.01.17-4b0f30090",
		},
		Patchline: "release",
	}
	expectedPreRelease = &GameFeed{
		Version: &GameReleaseVersion{
			Version: "2026.01.17-4b0f30090",
		},
		Patchline: "pre-release",
	}
)

func expectedMavenFeed(patchline string) *MavenFeed {
	return &MavenFeed{
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
		Patchline: patchline,
	}
}

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
	authStore := atestutil.NewTestAuthStore(auth.Server)

	var testCases = []struct {
		patchline        string
		display          string
		sampleReleaseURL string
		sampleRelease    string
		sampleMaven      string
		expectedUrl      string
		expectedFeed     *GameFeed
	}{
		{"release", "Release", testutil.SampleReleaseURL, testutil.SampleRelease, testutil.SampleMaven, expectedReleaseUrl, expectedRelease},
		{"pre-release", "Pre Release", testutil.SamplePreReleaseURL, testutil.SamplePreRelease, testutil.SampleMaven, expectedPreReleaseUrl, expectedPreRelease},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("game %s", tt.display), func(t *testing.T) {
			httpmock.RegisterResponder("GET", config.Feeds.GameVersion+tt.patchline+".json", httpmock.NewStringResponder(200, tt.sampleReleaseURL))
			url, err := fetchGameReleaseUrl(config, http, authStore, tt.patchline)
			assert.NoError(t, err)
			assert.Equal(t, url, tt.expectedUrl)

			httpmock.RegisterResponder("GET", url, httpmock.NewStringResponder(200, tt.sampleRelease))
			feed, err := fetchGame(config, http, authStore, tt.patchline)
			assert.NoError(t, err)
			td.Cmp(t, feed, tt.expectedFeed)
		})

		t.Run(fmt.Sprintf("maven %s", tt.display), func(t *testing.T) {
			httpmock.RegisterResponder("GET", MavenMetadataUrl(config.Feeds, tt.patchline), httpmock.NewStringResponder(200, tt.sampleMaven))
			feed, err := fetchMaven(config, http, tt.patchline)
			assert.NoError(t, err)
			td.Cmp(t, feed, expectedMavenFeed(tt.patchline))
		})
	}
}
