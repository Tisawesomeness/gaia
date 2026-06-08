package testutil

import (
	_ "embed"
	"net/http"
)

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
	//go:embed sample/release_url.json
	SampleReleaseURL string
	//go:embed sample/pre_release_url.json
	SamplePreReleaseURL string
	//go:embed sample/release.json
	SampleRelease string
	//go:embed sample/pre-release.json
	SamplePreRelease string
	//go:embed sample/maven-metadata.xml
	SampleMaven string
	//go:embed sample/launcher.json
	SampleLauncherRelease string
	//go:embed sample/feed.json
	SampleArticlesFeed string
	//go:embed sample/tis.json
	SampleProfileResponse string
)
