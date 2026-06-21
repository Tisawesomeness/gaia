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
	sampleDeviceAuthResponse = `{
		"device_code": "test-device-code",
		"user_code": "ABCD",
		"verification_uri": "https://example.com/verify",
		"verification_uri_complete": "https://example.com/verify?code=abc",
		"expires_in": 20,
		"interval": 1
	}`

	sampleTokenResponse = `{
		"access_token": "test-access-token",
		"refresh_token": "test-refresh-token",
		"id_token": "test-id-token",
		"expires_in": 20
	}`

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

// Causes the mock OAuth login endpoints to return a token that expires in the given duration (with second resolution)
func registerOAuthSuccess(config *config.Config, expires time.Duration) {
	httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))

	expiresSeconds := int((expires / time.Second))
	sampleTokenResponse = fmt.Sprintf(`{
		"access_token": "test-access-token",
		"refresh_token": "test-refresh-token",
		"id_token": "test-id-token",
		"expires_in": %d
	}`, expiresSeconds)
	httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, sampleTokenResponse))
}
func registerOAuthFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(500, ""))
}

// Causes the mock OAuth refresh endpoint to return a different token from the login token,
// that expires in the given duration (with second resolution), and fails on subsequent refreshes.
func registerOAuthRefreshSuccess(t *testing.T, config *config.Config, expires time.Duration) {
	expiresSeconds := int((expires / time.Second))
	sampleTokenResponse = fmt.Sprintf(`{
		"access_token": "test-access-token-2",
		"refresh_token": "test-refresh-token-2",
		"id_token": "test-id-token",
		"expires_in": %d
	}`, expiresSeconds)

	httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, sampleTokenResponse).Once(t.Log))
}
func registerTokenFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))
}

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
		"sessionToken": "test-session-token-2",
		"identityToken": "test-identity-token-2",
		"expiresAt": "%s"
	}`, expiresAtString)
	httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession, httpmock.NewStringResponder(200, sampleGameSessionResponse).Once(t.Log))
}
func registerGameSessionRefreshFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession, httpmock.NewStringResponder(500, ""))
}

func TestAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long tests")
	}

	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Auth: config.AuthConfig{
			ClientID:           "test-client-id",
			Scope:              "openid profile",
			DeviceAuth:         "https://example.com/oauth/device",
			Token:              "https://example.com/oauth/token",
			Profiles:           "https://example.com/profiles",
			CreateGameSession:  "https://example.com/api/sessions",
			RefreshGameSession: "https://example.com/api/sessions/refresh",
		},
	}

	t.Run("OAuthDeviceFlow_Success", func(t *testing.T) {
		registerOAuthSuccess(config, time.Hour)

		token, err := OAuthDeviceFlow(config, httpClient)
		if err != nil {
			t.Fatalf("OAuthDeviceFlow failed: %v", err)
		}

		if token.AccessToken != "test-access-token" {
			t.Errorf("expected AccessToken=test-access-token, got %s", token.AccessToken)
		}
	})

	t.Run("OAuthDeviceFlow_BadResponse", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(400, ""))

		_, err := OAuthDeviceFlow(config, httpClient)
		if err == nil {
			t.Fatalf("expected error for bad response")
		}
	})

	t.Run("OAuthDeviceFlow_ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(500, ""))

		_, err := OAuthDeviceFlow(config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("OAuthDeviceFlow_400Then401", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))
		// Create a responder that tracks call count and returns appropriate responses
		responseCount := 0
		httpmock.RegisterResponder("POST", config.Auth.Token, func(req *http.Request) (*http.Response, error) {
			responseCount++
			if responseCount < 3 {
				// Return authorization_pending
				return httpmock.NewStringResponse(400, `{
					"error": "authorization_pending",
					"error_description": "Please wait before polling"
				}`), nil
			}
			// Then return access_denied
			return httpmock.NewStringResponse(401, `{
				"error": "access_denied",
				"error_description": "Access denied"
			}`), nil
		})

		_, err := OAuthDeviceFlow(config, httpClient)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("OAuthDeviceFlow_500ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))

		_, err := OAuthDeviceFlow(config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("OAuthDeviceFlow_Timeout", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))
		// Simulate authorization_pending forever to trigger timeout
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, `{
			"error": "authorization_pending",
			"error_description": "Please wait before polling"
		}`))

		_, err := OAuthDeviceFlow(config, httpClient)
		if err == nil {
			t.Fatalf("expected timeout error")
		}
	})

	t.Run("OAuthRefresh_Success", func(t *testing.T) {
		registerOAuthRefreshSuccess(t, config, time.Hour)

		token, err := OAuthRefresh("test-refresh-token", config, httpClient)
		if err != nil {
			t.Fatalf("OAuthRefresh failed: %v", err)
		}

		if token.AccessToken != "test-access-token-2" {
			t.Errorf("expected AccessToken=test-access-token-2, got %s", token.AccessToken)
		}
	})

	t.Run("OAuthRefresh_400Error", func(t *testing.T) {
		// 400 responses return error before reading body
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(400, `{
			"error": "invalid_grant",
			"error_description": "Refresh token was revoked"
		}`))

		_, err := OAuthRefresh("invalid-refresh-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected error for 400 response")
		}
	})

	t.Run("OAuthRefresh_500", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))

		_, err := OAuthRefresh("test-refresh-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

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

		if session.SessionToken != "test-session-token-2" {
			t.Errorf("expected SessionToken=test-session-token-2, got %s", session.SessionToken)
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
