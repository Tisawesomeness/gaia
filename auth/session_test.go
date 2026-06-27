package auth

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/jarcoal/httpmock"
)

var (
	sampleLauncherResponse = `{
		"owner": "owner",
		"profiles": [
			{
				"uuid": "99c08079-3875-4aec-b329-34c4c88edc5a",
				"username": "test-user"
			}
		]
	}`

	sampleGameSessionResponse = `{
		"sessionToken": "test-session-token",
		"identityToken": "test-identity-token",
		"expiresAt": "` + time.Now().Add(time.Hour).Format(time.RFC3339Nano) + `"
	}`

	testUuid = "99c08079-3875-4aec-b329-34c4c88edc5a"
)

func registerProfilesSuccess(config *config.Config) {
	httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(200, sampleLauncherResponse))
}
func registerProfilesFailure(config *config.Config) {
	httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(500, ""))
}

// Causes the mock game session endpoint to return a token that expires in the given duration
func registerGameSessionSuccess(config *config.Config, expires time.Duration) {
	expiresAtString := time.Now().Add(expires).Format(time.RFC3339Nano)

	sampleGameSessionResponse = fmt.Sprintf(`{
		"sessionToken": "test-session-token",
		"identityToken": "test-identity-token",
		"expiresAt": "%s"
	}`, expiresAtString)
	httpmock.RegisterResponder("POST", config.Auth.CreateGameSession, httpmock.NewStringResponder(200, sampleGameSessionResponse))
}
func registerGameSessionFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.CreateGameSession, httpmock.NewStringResponder(500, ""))
}

// Causes the mock game session endpoint to return a token that expires in the given duration
// and fails on subsequent refreshes.
func registerGameSessionRefreshSuccess(t *testing.T, config *config.Config, expires time.Duration) {
	expiresAtString := time.Now().Add(expires).Format(time.RFC3339Nano)

	sampleGameSessionResponse = fmt.Sprintf(`{
		"sessionToken": "test-session-token-refreshed",
		"identityToken": "test-identity-token-2",
		"expiresAt": "%s"
	}`, expiresAtString)
	httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession, httpmock.NewStringResponder(200, sampleGameSessionResponse).Once(t.Log))
}
func registerGameSessionRefreshFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession, httpmock.NewStringResponder(500, ""))
}

func TestSession(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long tests")
	}

	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Auth: config.AuthConfig{
			Profiles:           "https://example.com/profiles",
			CreateGameSession:  "https://example.com/api/sessions",
			RefreshGameSession: "https://example.com/api/sessions/refresh",
		},
	}

	t.Run("GetAccountProfiles_Success", func(t *testing.T) {
		registerProfilesSuccess(config)

		result, err := GetAccountProfiles("Bearer test-access-token", config, httpClient)
		if err != nil {
			t.Fatalf("GetAccountProfiles failed: %v", err)
		}

		if result.Owner != "owner" {
			t.Errorf("expected Owner=owner, got %s", result.Owner)
		}
		if len(result.Profiles) != 1 {
			t.Errorf("expected 1 profile, got %d", len(result.Profiles))
		}
	})

	t.Run("GetAccountProfiles_401", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(401, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})

	t.Run("GetAccountProfiles_500", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(500, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("CreateGameSession_Success", func(t *testing.T) {
		registerGameSessionSuccess(config, time.Hour)

		session, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err != nil {
			t.Fatalf("CreateGameSession failed: %v", err)
		}

		if session.SessionToken != "test-session-token" {
			t.Errorf("expected SessionToken=test-session-token, got %s", session.SessionToken)
		}
	})

	t.Run("CreateGameSession_401", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.CreateGameSession, httpmock.NewStringResponder(401, ""))

		_, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})

	t.Run("CreateGameSession_ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.CreateGameSession, httpmock.NewStringResponder(500, ""))

		_, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("RefreshGameSession_Success", func(t *testing.T) {
		registerGameSessionRefreshSuccess(t, config, time.Hour)

		session, err := RefreshGameSession("Bearer test-session-token", testUuid, config, httpClient)
		if err != nil {
			t.Fatalf("RefreshGameSession failed: %v", err)
		}

		if session.SessionToken != "test-session-token-refreshed" {
			t.Errorf("expected SessionToken=test-session-token-refreshed, got %s", session.SessionToken)
		}
	})

	t.Run("RefreshGameSession_401", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession, httpmock.NewStringResponder(401, ""))

		_, err := RefreshGameSession("Bearer test-session-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})
}
