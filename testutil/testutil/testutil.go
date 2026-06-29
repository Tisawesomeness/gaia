// Unit test utilities. Must not import anything other than config to prevent import loops.
package testutil

import (
	_ "embed"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
)

type TestCaseFunc func(func(*testing.T)) func(*testing.T)

func MakeTestCase(beforeEach func(), afterEach func()) TestCaseFunc {
	return func(test func(*testing.T)) func(*testing.T) {
		return func(t *testing.T) {
			if beforeEach != nil {
				beforeEach()
			}
			test(t)
			if afterEach != nil {
				afterEach()
			}
		}
	}
}

// Creates a [httpmock.Responder] that attaches the [http.Request] to the returned [http.Response].
// Required so `resp.Request.URL` does not panic.
func WithRequest(r httpmock.Responder) httpmock.Responder {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := r(req)
		if err != nil {
			return nil, err
		}
		resp.Request = req
		return resp, nil
	}
}

var (
	//go:embed sample/launcher.json
	SampleLauncherRelease string
	//go:embed sample/feed.json
	SampleArticlesFeed string

	//go:embed sample/release_url.json
	SampleReleaseURL string
	//go:embed sample/pre_release_url.json
	SamplePreReleaseURL string
	ReleaseURL          = "https://example-game-assets-release.r2.cloudflarestorage.com/version/release.json"
	PreReleaseURL       = "https://example-game-assets-release.r2.cloudflarestorage.com/version/pre-release.json"

	//go:embed sample/release.json
	SampleRelease string
	//go:embed sample/pre-release.json
	SamplePreRelease string

	//go:embed sample/maven-metadata.xml
	SampleMaven string

	//go:embed sample/profile.json
	SampleProfileResponse string
	//go:embed sample/profile_cape.json
	SampleProfileResponseCape string
	//go:embed sample/profile_extra.json
	SampleProfileResponseExtra string
	//go:embed sample/profile_no_skin.json
	SampleProfileResponseNoSkin string

	//go:embed sample/get-launcher-data.json
	SampleLauncherData string
	//go:embed sample/patchlines.json
	SamplePatchlinesResponse string
	//go:embed sample/patchlines_expiry.json
	SamplePatchlinesResponseExpiry string

	//go:embed sample/server.json
	SampleServerResponse string
	//go:embed sample/server_unknown.json
	SampleServerResponseUnknown string
)
