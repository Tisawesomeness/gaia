package hytale

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

var (
	expectedLauncherRelease = LauncherReleaseFeed{
		Release: &LauncherRelease{
			Version: "2026.01.12-e43ec47",
			DownloadURLs: DownloadURLs{
				"linux": {
					"amd64": {
						URL:    "https://launcher.hytale.com/builds/release/linux/amd64/hytale-launcher-2026.01.12-e43ec47.zip",
						SHA256: "e3ff4eca14932ef7051dad5f1a9a646aca72b4b62f7ea263faa6f92fc03b76ab",
					},
				},
				"darwin": {
					"arm64": {
						URL:    "https://launcher.hytale.com/builds/release/darwin/arm64/hytale-launcher-2026.01.12-e43ec47.zip",
						SHA256: "856161996eeced29477c0073e0e3fb2b3809c24302ea09e766713ca9c9f18d6a",
					},
				},
				"windows": {
					"amd64": {
						URL:    "https://launcher.hytale.com/builds/release/windows/amd64/hytale-launcher-2026.01.12-e43ec47.zip",
						SHA256: "369ccbb9a8a620f1338b2bc3cdb3be3e39d6304863bb0a2c596fd57b834dccb0",
					},
				},
			},
		},
	}
	expectedArticlesFeed = LauncherPostFeed{
		Articles: &ArticleList{
			Articles: []*Article{
				{
					Title:       "Hotfix Notes",
					DestURL:     "https://hytale.com/news/2026/1/hotfixes-january-2026",
					Description: "Jan 15 - New fixes and improvements, check out the patch notes for more details. Check back here for more updates!",
					ImageURL:    "images/patches.png",
				},
				{
					Title:       "Hytale is finally here!",
					DestURL:     "https://hytale.com/news/2026/1/hytale-is-finally-here",
					Description: "The moment has arrived, Hytale is releasing into Early Access today!",
					ImageURL:    "images/launch.png",
				},
			},
		},
	}
)

func TestLauncher(t *testing.T) {
	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)

	config := &config.Config{
		Feeds: config.FeedsConfig{
			LauncherRelease:  "https://launcher.example.com/version/release/launcher.json",
			LauncherArticles: "https://launcher.example.com/launcher-feed/release/feed.json",
		},
	}
	authStore := auth.NewSimpleAuthStore(auth.Server)
	feeds := &HytaleFeeds{
		config:    config,
		http:      http,
		authStore: authStore,
	}

	t.Run("launcher release", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherRelease, httpmock.NewStringResponder(200, testutil.SampleLauncherRelease))
		feed, err := fetchLauncherRelease(feeds)
		assert.NoError(t, err)
		td.Cmp(t, feed, expectedLauncherRelease)
	})

	t.Run("launcher articles", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherArticles, httpmock.NewStringResponder(200, testutil.SampleArticlesFeed))
		feed, err := fetchArticles(feeds)
		assert.NoError(t, err)
		td.Cmp(t, feed, expectedArticlesFeed)
	})

	t.Run("launcher release error status", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherRelease, httpmock.NewStringResponder(400, ""))

		feed, err := fetchLauncherRelease(feeds)
		assert.Error(t, err)
		assert.Nil(t, feed)
		assert.Contains(t, err.Error(), "Fetch launcher release")
	})

	t.Run("launcher articles error status", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherArticles, httpmock.NewStringResponder(400, ""))

		feed, err := fetchArticles(feeds)
		assert.Error(t, err)
		assert.Nil(t, feed)
		assert.Contains(t, err.Error(), "Fetch articles")
	})
}
