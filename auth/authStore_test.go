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
			RenewalBufferSeconds: 86400,
			AccountsBackend:      "http://mock.kratos",
		},
		Auth: config.AuthConfig{
			OAuthRefreshBuffer:       300,
			GameSessionRefreshBuffer: 300,
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
		registerOAuthSuccess(config)
		registerProfilesSuccess(config)
		registerGameSessionSuccess(config)

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
	}))
}
