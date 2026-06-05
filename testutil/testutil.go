package testutil

import (
	"net/http"

	"github.com/Tisawesomeness/gaia/auth"
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

func NewTestAuthStore(http *http.Client) auth.AuthStore {
	return TestAuthStore{
		OAuthToken:       "sample",
		GameSessionToken: "sample",
		HTTPClient:       http,
	}
}
