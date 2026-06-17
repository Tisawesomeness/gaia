package auth

import (
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

	sampleTokenErrorResponse = `{
		"error": "access_denied",
		"error_description": "User denied access"
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
		"expiresAt": "2024-01-01T00:00:00Z"
	}`

	testUuid = "99c08079-3875-4aec-b329-34c4c88edc5a"
)

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

	t.Run("StartDeviceAuth_Success", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))

		resp, err := startDeviceAuth(config, httpClient)
		if err != nil {
			t.Fatalf("startDeviceAuth failed: %v", err)
		}

		if resp.DeviceCode != "test-device-code" {
			t.Errorf("expected DeviceCode=test-device-code, got %s", resp.DeviceCode)
		}
		if resp.UserCode != "ABCD" {
			t.Errorf("expected UserCode=ABCD, got %s", resp.UserCode)
		}
	})

	t.Run("StartDeviceAuth_BadResponse", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(400, ""))

		_, err := startDeviceAuth(config, httpClient)
		if err == nil {
			t.Fatalf("expected error for bad response")
		}
	})

	t.Run("StartDeviceAuth_ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(500, ""))

		_, err := startDeviceAuth(config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("PollForToken_Success", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.Token,
			httpmock.NewStringResponder(200, sampleTokenErrorResponse))
		httpmock.RegisterResponder("POST", config.Auth.Token,
			httpmock.NewStringResponder(200, sampleTokenResponse))

		response := defaultDeviceAuthResponse()

		token, err := pollForToken(response, config, httpClient)
		if err != nil {
			t.Fatalf("pollForToken failed: %v", err)
		}

		if token.AccessToken != "test-access-token" {
			t.Errorf("expected AccessToken=test-access-token, got %s", token.AccessToken)
		}
	})

	t.Run("PollForToken_400Then401", func(t *testing.T) {
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

		response := defaultDeviceAuthResponse()
		response.Interval = 1 // Faster polling for tests

		_, err := pollForToken(response, config, httpClient)
		if err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("PollForToken_500ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.Token,
			httpmock.NewStringResponder(500, ""))

		response := defaultDeviceAuthResponse()

		_, err := pollForToken(response, config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("PollForToken_Timeout", func(t *testing.T) {
		// Simulate authorization_pending forever to trigger timeout
		httpmock.RegisterResponder("POST", config.Auth.Token,
			httpmock.NewStringResponder(200, `{
				"error": "authorization_pending",
				"error_description": "Please wait before polling"
			}`))

		response := defaultDeviceAuthResponse()
		response.Interval = 1
		response.ExpiresIn = 3

		_, err := pollForToken(response, config, httpClient)
		if err == nil {
			t.Fatalf("expected timeout error")
		}
	})

	t.Run("OAuthRefresh_Success", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, sampleTokenResponse))

		token, err := OAuthRefresh("test-refresh-token", config, httpClient)
		if err != nil {
			t.Fatalf("OAuthRefresh failed: %v", err)
		}

		if token.AccessToken != "test-access-token" {
			t.Errorf("expected AccessToken=test-access-token, got %s", token.AccessToken)
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
		httpmock.RegisterResponder("GET", config.Auth.Profiles,
			httpmock.NewStringResponder(200, sampleLauncherResponse))

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
		httpmock.RegisterResponder("GET", config.Auth.Profiles,
			httpmock.NewStringResponder(401, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})

	t.Run("GetAccountProfiles_500", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.Profiles,
			httpmock.NewStringResponder(500, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("CreateGameSession_Success", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.CreateGameSession,
			httpmock.NewStringResponder(200, sampleGameSessionResponse))

		session, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err != nil {
			t.Fatalf("CreateGameSession failed: %v", err)
		}

		if session.SessionToken != "test-session-token" {
			t.Errorf("expected SessionToken=test-session-token, got %s", session.SessionToken)
		}
	})

	t.Run("CreateGameSession_401", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.CreateGameSession,
			httpmock.NewStringResponder(401, ""))

		_, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})

	t.Run("CreateGameSession_ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.CreateGameSession,
			httpmock.NewStringResponder(500, ""))

		_, err := CreateGameSession("Bearer test-access-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	t.Run("RefreshGameSession_Success", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession,
			httpmock.NewStringResponder(200, sampleGameSessionResponse))

		session, err := RefreshGameSession("Bearer test-session-token", testUuid, config, httpClient)
		if err != nil {
			t.Fatalf("RefreshGameSession failed: %v", err)
		}

		if session.SessionToken != "test-session-token" {
			t.Errorf("expected SessionToken=test-session-token, got %s", session.SessionToken)
		}
	})

	t.Run("RefreshGameSession_401", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.RefreshGameSession,
			httpmock.NewStringResponder(401, ""))

		_, err := RefreshGameSession("Bearer test-session-token", testUuid, config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})
}
