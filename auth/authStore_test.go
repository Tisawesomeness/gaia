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

func sampleConfig(kratosEnabled bool) *config.Config {
	var creds config.CredentialsConfig
	if kratosEnabled {
		creds = config.CredentialsConfig{
			HytaleEmail:     "test@example.com",
			HytalePassword:  "test123",
			Hytale2FASecret: "T7HK43UPHIYJGWGDYFGBQKGHKUT47G4Z",
		}
	}
	return &config.Config{
		Credentials: creds,
		Kratos: config.KratosConfig{
			RenewalBufferSeconds: 5, // Schedule refresh for 5 seconds before expiry
			AccountsBackend:      "http://mock.kratos",
		},
		Auth: config.AuthConfig{
			OAuthRefreshBuffer:       5, // Schedule refresh for 5 seconds before expiry
			GameSessionRefreshBuffer: 5, // Schedule refresh for 5 seconds before expiry
			ClientID:                 "test-client-id",
			Scope:                    "openid profile",
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

	t.Run("Full happy path (Kratos disabled)", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(false)

		// No DB values, all endpoints succeed
		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config)
		registerGameSessionSuccess(config, time.Second*7)

		// Expect both tokens to be returned
		authStore, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.NoError(t, err)
		authToken, err := authStore.GetOAuthToken()
		assert.NoError(t, err)
		assert.NotEmpty(t, authToken)
		sessionToken, err := authStore.GetGameSessionToken()
		assert.NoError(t, err)
		assert.NotEmpty(t, sessionToken)
		_, exists := authStore.GetKratosClient()
		assert.False(t, exists)

		// Wait for refresh (7s-3s = 4s, less than the 5s needed for renewal)
		registerOAuthRefreshSuccess(t, config, time.Hour)
		registerGameSessionRefreshSuccess(t, config, time.Hour)
		time.Sleep(time.Second * 3)

		// Expect both tokens to be updated
		authToken2, err := authStore.GetOAuthToken()
		assert.NoError(t, err)
		assert.NotEmpty(t, authToken)
		assert.NotEqual(t, authToken, authToken2)
		sessionToken2, err := authStore.GetGameSessionToken()
		assert.NoError(t, err)
		assert.NotEmpty(t, sessionToken)
		assert.NotEqual(t, sessionToken, sessionToken2)
	}))

	t.Run("OAuth fail (Kratos disabled)", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(false)

		registerOAuthFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Profile fail (Kratos disabled)", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(false)

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))

	t.Run("Game session fail (Kratos disabled)", authStoreTestCase(func(t *testing.T) {
		config := sampleConfig(false)

		registerOAuthSuccess(config, time.Second*7)
		registerProfilesSuccess(config)
		registerGameSessionFailure(config)

		_, err := NewAuthStore(config, authStoreDB, httpClient)
		assert.Error(t, err)
	}))
}
