package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/testutil"
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
func storeOAuthToken(expires time.Duration) {
	authStoreDB.SetOAuthToken(db.OAuthToken{
		AccessToken:  "test-access-token-stored",
		RefreshToken: "test-refresh-token-stored",
		ExpiresAt:    time.Now().Add(expires),
	})
}

// Stores the test profile UUID in the test database
func storeProfile() {
	authStoreDB.SetProfileUUID(testUuid)
}

// Stores a test game session token in the test database that expires after the given duration
func storeGameSession(expires time.Duration) {
	authStoreDB.SetGameSession(db.GameSessionToken{
		SessionToken: "test-session-token-stored",
		ExpiresAt:    time.Now().Add(expires),
	})
}

func sampleConfig(secondsBeforeRefresh int, uuid string) *config.Config {
	return &config.Config{
		Credentials: config.CredentialsConfig{
			ProfileUUID: uuid,
		},
		Auth: config.AuthConfig{
			OAuthRefreshBuffer:       secondsBeforeRefresh,
			GameSessionRefreshBuffer: secondsBeforeRefresh,
			DeviceAuth:               "https://example.com/oauth/device",
			Token:                    "https://example.com/oauth/token",
			Profiles:                 "https://example.com/profiles",
			CreateGameSession:        "https://example.com/api/sessions",
			RefreshGameSession:       "https://example.com/api/sessions/refresh",
		},
		HTTP: config.HTTPConfig{
			Timeout:      5,
			MaxIdleConns: 10,
		},
	}
}

func assertTokens(t *testing.T, expectedAuthToken string, expectedSessionToken string, authStore AuthStore) {
	authToken, err := authStore.GetOAuthToken()
	assert.NoError(t, err)
	assert.Equal(t, expectedAuthToken, authToken)
	sessionToken, err := authStore.GetGameSessionToken()
	assert.NoError(t, err)
	assert.Equal(t, expectedSessionToken, sessionToken)
}

func AuthStoreTests(t *testing.T) {
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

	t.Run("Full happy path", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		// No DB values, all endpoints succeed
		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config, Server)
		registerGameSessionSuccess(config, time.Second*7)

		// Expect both tokens to be returned
		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token", "test-session-token", authStore)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		registerGameSessionRefreshSuccess(t, config, time.Hour)
		time.Sleep(time.Second * 3)

		// Expect both tokens to be updated
		assertTokens(t, "test-access-token-refreshed", "test-session-token-refreshed", authStore)
	}))

	t.Run("If config set profile UUID, profile lookup is skipped", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "c97da2da-f703-48cd-a1fa-e22a8e7e8588")

		// No DB values, all endpoints succeed
		registerOAuthSuccess(config, time.Second*7)
		registerGameSessionSuccess(config, time.Second*7)

		// Expect both tokens to be returned
		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token", "test-session-token", authStore)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		registerGameSessionRefreshSuccess(t, config, time.Hour)
		time.Sleep(time.Second * 3)

		// Expect both tokens to be updated
		assertTokens(t, "test-access-token-refreshed", "test-session-token-refreshed", authStore)
	}))

	t.Run("OAuth fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		registerOAuthFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Profile fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesFailure(config, Server)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Game session fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config, Server)
		registerGameSessionFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Restore all values from DB", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Hour)
		storeProfile()
		storeGameSession(time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token-stored", "test-session-token-stored", authStore)
	}))

	t.Run("OAuth refreshes if DB loads near-expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Second)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		storeProfile()
		storeGameSession(time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token-refreshed", "test-session-token-stored", authStore)
	}))

	t.Run("OAuth gets new token if DB loads expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(-time.Second)
		registerOAuthSuccess(config, time.Hour)
		storeProfile()
		storeGameSession(time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token", "test-session-token-stored", authStore)
	}))

	t.Run("Game session refreshes if DB loads near-expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Hour)
		storeProfile()
		storeGameSession(time.Second)
		registerGameSessionRefreshSuccess(t, config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token-stored", "test-session-token-refreshed", authStore)
	}))

	t.Run("Game session gets new token if DB loads expired token", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Hour)
		storeProfile()
		storeGameSession(-time.Second)
		registerGameSessionSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token-stored", "test-session-token", authStore)
	}))

	t.Run("Creates a new game session if no profile stored", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Hour)
		// do not store profile
		registerProfilesSuccess(config, Server)
		storeGameSession(time.Hour)
		registerGameSessionSuccess(config, time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		assertTokens(t, "test-access-token-stored", "test-session-token", authStore)
	}))

	t.Run("unexpired OAuth token is OK even after refresh fail", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Second * 7)
		storeProfile()
		storeGameSession(time.Hour)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshFailure(config)
		time.Sleep(time.Second * 3)

		authToken, err := authStore.GetOAuthToken()
		assert.NoError(t, err)
		assert.Equal(t, "test-access-token-stored", authToken)
	}))

	t.Run("game session refresh fail falls back to oauth then creates a new session", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(5, "")

		storeOAuthToken(time.Hour)
		storeProfile()
		storeGameSession(time.Second * 7)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerGameSessionRefreshFailure(config)
		registerGameSessionSuccess(config, time.Hour)
		time.Sleep(time.Second * 3)

		sessionToken, err := authStore.GetGameSessionToken()
		assert.NoError(t, err)
		assert.Equal(t, "test-session-token", sessionToken)
	}))

	t.Run("expired OAuth token returns error", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(1, "")

		storeOAuthToken(time.Second * 2)
		storeProfile()
		storeGameSession(time.Hour)

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
		config := sampleConfig(1, "")

		storeOAuthToken(time.Hour)
		storeProfile()
		storeGameSession(time.Second * 2)

		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)

		// Wait for refresh and expiry (2s-3s = -1s)
		registerGameSessionRefreshFailure(config)
		registerGameSessionFailure(config)
		time.Sleep(time.Second * 3)

		_, err = authStore.GetGameSessionToken()
		assert.Error(t, err)
	}))
}
