package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

var (
	authStoreDB       *db.DB
	authStoreTestCase = testutil.MakeTestCase(beforeEachAuthStore, nil)
)

func init() {
	db, err := db.NewDB(config.ValkeyConfig{
		Address:       "127.0.0.1",
		Port:          9999,
		DatabaseIndex: 3,
	})
	if err != nil {
		panic(err)
	}
	authStoreDB = db
}

func teardownAuthStore() {
	authStoreDB.Close()
}

func beforeEachAuthStore() {
	authStoreDB.Clear()
}

// Stores a test OAuth token in the test database that expires after the given duration
func storeOAuthToken(expires time.Duration, authType AuthType) {
	authStoreDB.SetOAuthToken(db.OAuthToken{
		AccessToken:  "test-access-token-stored",
		RefreshToken: "test-refresh-token-stored",
		ExpiresAt:    time.Now().Add(expires),
		AuthType:     authType.String(),
	})
}

// Stores the test profile UUID in the test database
func storeProfile() {
	authStoreDB.SetProfileUUID(testUuid)
}

// Stores a test game session token in the test database that expires after the given duration
func storeGameSession(expires time.Duration, authType AuthType) {
	authStoreDB.SetGameSession(db.GameSessionToken{
		SessionToken: "test-session-token-stored",
		ExpiresAt:    time.Now().Add(expires),
		AuthType:     authType.String(),
	})
}

func sampleConfig(secondsBeforeRefresh int, authType AuthType, uuid string) *config.Config {
	return &config.Config{
		Credentials: config.CredentialsConfig{
			AuthMethod:            authType.String(),
			StartRedirectListener: false,
			ProfileUUID:           uuid,
		},
		Auth: config.AuthConfig{
			OAuthRefreshBuffer:       secondsBeforeRefresh,
			GameSessionRefreshBuffer: secondsBeforeRefresh,
			DeviceAuth:               "https://example.com/oauth/device",
			BrowserAuth:              "https://example.com/oauth2/auth",
			BrowserAuthTimeout:       1,
			RedirectURI:              "https://example.com/redirect",
			Token:                    "https://example.com/oauth/token",
			Profiles:                 "https://example.com/profiles",
			LauncherData:             "https://account-data.example.com/my-account/get-launcher-data?arch=amd64&os=windows",
			CreateGameSession:        "https://example.com/api/sessions",
			RefreshGameSession:       "https://example.com/api/sessions/refresh",
		},
		HTTP: config.HTTPConfig{
			Timeout:      5,
			MaxIdleConns: 10,
		},
	}
}

func assertOAuth(t *testing.T, authStore AuthStore, expectedToken string, expectedType AuthType) {
	authToken, err := authStore.GetOAuthToken()
	assert.NoError(t, err)
	assert.Equal(t, expectedToken, authToken.Token)
	assert.Equal(t, expectedType, authToken.AuthType)
}

func assertSession(t *testing.T, authStore AuthStore, expectedToken string, expectedType AuthType) {
	sessionToken, err := authStore.GetGameSessionToken()
	assert.NoError(t, err)
	assert.Equal(t, expectedToken, sessionToken.Token)
	assert.Equal(t, expectedType, sessionToken.AuthType)
}

func TestAuthStore(t *testing.T) {
	// These tests can be LONG because they have to wait for OAuth polling and refreshing,
	// consider setting a timeout greater than 30s
	if testing.Short() {
		t.Skip("skipping long tests")
	}
	t.Cleanup(teardownAuthStore)

	httpClient := &http.Client{
		Timeout: time.Duration(5) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	// No tests that invoke the OAuth browser flow are included since
	// it is highly dependent on manual user input

	t.Run("Full happy path", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		// No DB values, all endpoints succeed
		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config, Server)
		registerGameSessionSuccess(config, time.Second*7)

		// Expect both tokens to be returned
		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token", Server)
		assertSession(t, authStore, "test-session-token", Server)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		registerGameSessionRefreshSuccess(t, config, time.Hour)
		time.Sleep(time.Second * 3)

		// Expect both tokens to be updated
		assertOAuth(t, authStore, "test-access-token-refreshed", Server)
		assertSession(t, authStore, "test-session-token-refreshed", Server)
	}))

	t.Run("If config set profile UUID, profile lookup is skipped", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "c97da2da-f703-48cd-a1fa-e22a8e7e8588")

		// No DB values, all endpoints succeed
		registerOAuthSuccess(config, time.Second*7)
		registerGameSessionSuccess(config, time.Second*7)

		// Expect both tokens to be returned
		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token", Server)
		assertSession(t, authStore, "test-session-token", Server)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		registerGameSessionRefreshSuccess(t, config, time.Hour)
		time.Sleep(time.Second * 3)

		// Expect both tokens to be updated
		assertOAuth(t, authStore, "test-access-token-refreshed", Server)
		assertSession(t, authStore, "test-session-token-refreshed", Server)
	}))

	t.Run("OAuth fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		registerOAuthFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Profile fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesFailure(config, Server)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Game session fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config, Server)
		registerGameSessionFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Restore all values from DB", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		storeGameSession(time.Hour, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Server)
		assertSession(t, authStore, "test-session-token-stored", Server)
	}))

	t.Run("Restore all values from DB, launcher auth", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Launcher, "")

		storeOAuthToken(time.Hour, Launcher)
		storeProfile()
		storeGameSession(time.Hour, Launcher)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Launcher)
		assertSession(t, authStore, "test-session-token-stored", Launcher)
	}))

	t.Run("OAuth refreshes if DB loads near-expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Second, Server)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		storeProfile()
		storeGameSession(time.Hour, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-refreshed", Server)
		assertSession(t, authStore, "test-session-token-stored", Server)
	}))

	t.Run("OAuth gets new token if DB loads expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(-time.Second, Server)
		registerOAuthSuccess(config, time.Hour)
		storeProfile()
		storeGameSession(time.Hour, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token", Server)
		assertSession(t, authStore, "test-session-token-stored", Server)
	}))

	t.Run("Game session refreshes if DB loads near-expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		storeGameSession(time.Second, Server)
		registerGameSessionRefreshSuccess(t, config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Server)
		assertSession(t, authStore, "test-session-token-refreshed", Server)
	}))

	t.Run("Game session gets new token if DB loads expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		storeGameSession(-time.Second, Server)
		registerGameSessionSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Server)
		assertSession(t, authStore, "test-session-token", Server)
	}))

	t.Run("Creates a new game session if no profile stored", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		// do not store profile
		registerProfilesSuccess(config, Server)
		storeGameSession(time.Hour, Server)
		registerGameSessionSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Server)
		assertSession(t, authStore, "test-session-token", Server)
	}))

	t.Run("unexpired OAuth token is OK even after refresh fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Second*7, Server)
		storeProfile()
		storeGameSession(time.Hour, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshFailure(config)
		time.Sleep(time.Second * 3)

		assertOAuth(t, authStore, "test-access-token-stored", Server)
	}))

	t.Run("game session refresh fail falls back to oauth then creates a new session", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		storeGameSession(time.Second*7, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerGameSessionRefreshFailure(config)
		registerGameSessionSuccess(config, time.Hour)
		time.Sleep(time.Second * 3)

		assertSession(t, authStore, "test-session-token", Server)
	}))

	t.Run("expired OAuth token returns error", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(1, Server, "")

		storeOAuthToken(time.Second*2, Server)
		storeProfile()
		storeGameSession(time.Hour, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assert.NotNil(t, authStore)

		// Wait for refresh and expiry (2s-3s = -1s)
		registerOAuthRefreshFailure(config)
		time.Sleep(time.Second * 3)

		_, err = authStore.GetOAuthToken()
		assert.Error(t, err)
	}))

	t.Run("expired game session token returns error", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(1, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		storeGameSession(time.Second*2, Server)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh and expiry (2s-3s = -1s)
		registerGameSessionRefreshFailure(config)
		registerGameSessionFailure(config)
		time.Sleep(time.Second * 3)

		_, err = authStore.GetGameSessionToken()
		assert.Error(t, err)
	}))

	t.Run("ignores stored OAuth token with mismatched auth type", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		// Store token with different auth type
		storeOAuthToken(time.Hour, Launcher)
		storeProfile()
		storeGameSession(time.Hour, Server)

		// Expect new token fetch because auth type mismatched
		registerOAuthSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token", Server)
		assertSession(t, authStore, "test-session-token-stored", Server)
	}))

	t.Run("ignores stored game session token with mismatched auth type", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, Server, "")

		storeOAuthToken(time.Hour, Server)
		storeProfile()
		// Store game session with different auth type
		storeGameSession(time.Hour, Launcher)

		// Expect new game session fetch because auth type mismatched
		registerGameSessionSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertOAuth(t, authStore, "test-access-token-stored", Server)
		assertSession(t, authStore, "test-session-token", Server)
	}))
}
