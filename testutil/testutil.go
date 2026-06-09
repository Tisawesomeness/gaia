// Unit test utilities. Must not import anything other than config to prevent import loops.
package testutil

import (
	_ "embed"
	"net/http"
	"testing"
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

type TestAuthStore struct {
	OAuthToken       string
	GameSessionToken string
	HTTPClient       *http.Client
}

// GetGameSessionToken implements [AuthStore].
func (t TestAuthStore) GetGameSessionToken() (string, error) {
	return t.OAuthToken, nil
}

// GetOAuthToken implements [AuthStore].
func (t TestAuthStore) GetOAuthToken() (string, error) {
	return t.GameSessionToken, nil
}

// GetKratosClient implements [AuthStore].
func (t TestAuthStore) GetKratosClient() (*http.Client, bool) {
	return t.HTTPClient, t.HTTPClient != nil
}

func NewTestAuthStore(http *http.Client) TestAuthStore {
	return TestAuthStore{
		OAuthToken:       "sample",
		GameSessionToken: "sample",
		HTTPClient:       http,
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

	//go:embed sample/tis.json
	SampleProfileResponse string
)
