// Integration test utilities. May import anything except commands, but will cause import loops in those packages.
package itestutil

import (
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/testutil"
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
