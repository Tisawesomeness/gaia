package auth

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
)

var (
	sampleAccountDataResponse = `{
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

	expectedLauncherData = &LauncherData{
		PatchLines: map[string]BuildDetails{
			"pre-release": {
				BuildVersion: "2026.01.17-a4cc0e7dd",
				ExpiresAt:    0,
			},
			"release": {
				BuildVersion: "2026.01.17-4b0f30090",
				ExpiresAt:    0,
			},
		},
		Profiles: []GameProfile{
			{
				Username: "tis",
				UUID:     "d798091b-f494-4208-a1ba-e24da5880786",
			},
		},
	}
)

func registerProfilesSuccess(config *config.Config, authType AuthType) {
	switch authType {
	case Server:
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(200, sampleAccountDataResponse))
	case Launcher:
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(200, testutil.SampleLauncherData))
	default:
		panic("unknown auth type")
	}
}
func registerProfilesFailure(config *config.Config, authType AuthType) {
	switch authType {
	case Server:
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(500, ""))
	case Launcher:
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(500, ""))
	default:
		panic("unknown auth type")
	}
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
			LauncherData:       "https://account-data.example.com/my-account/get-launcher-data?arch=amd64&os=windows",
			CreateGameSession:  "https://example.com/api/sessions",
			RefreshGameSession: "https://example.com/api/sessions/refresh",
		},
	}

	// Account profiles

	t.Run("GetAccountProfiles_Success", func(t *testing.T) {
		registerProfilesSuccess(config, Server)

		profiles, err := GetAccountProfiles("Bearer test-access-token", Server, config, httpClient)
		if err != nil {
			t.Fatalf("GetAccountProfiles failed: %v", err)
		}

		if len(profiles) != 1 {
			t.Errorf("expected 1 profile, got %d", len(profiles))
		}
	})

	t.Run("GetAccountProfiles_SuccessLauncher", func(t *testing.T) {
		registerProfilesSuccess(config, Launcher)

		profiles, err := GetAccountProfiles("Bearer test-access-token", Launcher, config, httpClient)
		if err != nil {
			t.Fatalf("GetAccountProfiles failed: %v", err)
		}

		if len(profiles) != 1 {
			t.Errorf("expected 1 profile, got %d", len(profiles))
		}
	})

	t.Run("GetAccountProfiles_401", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(401, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", Server, config, httpClient)
		if err == nil {
			t.Fatalf("expected 401 error")
		}
	})

	t.Run("GetAccountProfiles_500", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.Profiles, httpmock.NewStringResponder(500, ""))

		_, err := GetAccountProfiles("Bearer test-access-token", Server, config, httpClient)
		if err == nil {
			t.Fatalf("expected error for server error")
		}
	})

	// Launcher data

	t.Run("success case (200 OK)", func(t *testing.T) {
		registerProfilesSuccess(config, Launcher)

		data, err := GetLauncherData(config, httpClient, "sample-token")

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if data == nil {
			t.Fatal("Expected launcher data, got nil")
		}

		td.Cmp(t, data, expectedLauncherData)
	})

	t.Run("network failure (401 unauthorized)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(http.StatusUnauthorized, ""))

		_, err := GetLauncherData(config, httpClient, "sample-token")

		if err == nil {
			t.Fatal("Expected an error on 401 unauthorized, got nil")
		}
	})

	t.Run("empty response body schema validation (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Auth.LauncherData, httpmock.NewStringResponder(200, `{"patchlines": {}, "profiles": []}`))

		data, err := GetLauncherData(config, httpClient, "sample-token")

		if err != nil {
			t.Fatalf("Expected no error for empty data, got %v", err)
		}

		expectedLauncherData := &LauncherData{
			PatchLines: make(map[string]BuildDetails),
			Profiles:   []GameProfile{},
		}
		td.Cmp(t, data, expectedLauncherData)
	})

	// Create game session

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

	// Refresh game session

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
