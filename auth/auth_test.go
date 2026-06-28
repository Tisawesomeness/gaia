package auth

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
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
		"access_token": "test-access-token-refreshed",
		"refresh_token": "test-refresh-token-refreshed",
		"id_token": "test-id-token",
		"expires_in": %d
	}`, expiresSeconds)

	httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, sampleTokenResponse).Once(t.Log))
}
func registerOAuthRefreshFailure(config *config.Config) {
	httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))
}

func TestDeviceAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping long tests")
	}

	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Auth: config.AuthConfig{
			DeviceAuth: "https://example.com/oauth/device",
			Token:      "https://example.com/oauth/token",
		},
	}

	t.Run("OAuthDeviceFlow_Success", func(t *testing.T) {
		registerOAuthSuccess(config, time.Hour)

		token, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.NoError(t, err)
		assert.Equal(t, "test-access-token", token.AccessToken)
	})

	t.Run("OAuthDeviceFlow_BadResponse", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(400, ""))

		_, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.Error(t, err)
	})

	t.Run("OAuthDeviceFlow_ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(500, ""))

		_, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.Error(t, err)
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

		_, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.Error(t, err)
	})

	t.Run("OAuthDeviceFlow_500ServerError", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))

		_, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.Error(t, err)
	})

	t.Run("OAuthDeviceFlow_Timeout", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.DeviceAuth, httpmock.NewStringResponder(200, sampleDeviceAuthResponse))
		// Simulate authorization_pending forever to trigger timeout
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(200, `{
			"error": "authorization_pending",
			"error_description": "Please wait before polling"
		}`))

		_, err := OAuthDeviceFlow(config, httpClient, nil)
		assert.Error(t, err)
	})

	t.Run("OAuthRefresh_SuccessServer", func(t *testing.T) {
		registerOAuthRefreshSuccess(t, config, time.Hour)

		token, err := OAuthRefresh(config, httpClient, "test-refresh-token", Server)
		assert.NoError(t, err)
		assert.Equal(t, "test-access-token-refreshed", token.AccessToken)
	})

	t.Run("OAuthRefresh_SuccessLauncher", func(t *testing.T) {
		registerOAuthRefreshSuccess(t, config, time.Hour)

		token, err := OAuthRefresh(config, httpClient, "test-refresh-token", Launcher)
		assert.NoError(t, err)
		assert.Equal(t, "test-access-token-refreshed", token.AccessToken)
	})

	t.Run("OAuthRefresh_400Error", func(t *testing.T) {
		// 400 responses return error before reading body
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(400, `{
			"error": "invalid_grant",
			"error_description": "Refresh token was revoked"
		}`))

		_, err := OAuthRefresh(config, httpClient, "invalid-refresh-token", Server)
		assert.Error(t, err)
	})

	t.Run("OAuthRefresh_500", func(t *testing.T) {
		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(500, ""))

		_, err := OAuthRefresh(config, httpClient, "test-refresh-token", Server)
		assert.Error(t, err)
	})
}

func TestBrowserAuth(t *testing.T) {
	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Auth: config.AuthConfig{
			BrowserAuth:        "https://oauth.example.com/auth",
			BrowserAuthTimeout: 1,
			RedirectURI:        "https://example.com/redirect",
			Token:              "https://oauth.example.com/token",
		},
	}

	t.Run("OAuthBrowserFlow_Success", func(t *testing.T) {
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			parsed, err := url.Parse(authUrl)
			if err != nil {
				return RedirectParams{}, err
			}
			encodedState := parsed.Query().Get("state")
			if encodedState == "" {
				return RedirectParams{}, errors.New("state missing")
			}

			decodedState, err := base64.RawURLEncoding.DecodeString(encodedState)
			if err != nil {
				return RedirectParams{}, fmt.Errorf("decode error: %w", err)
			}

			var statePort stateWithPort
			if err := json.Unmarshal(decodedState, &statePort); err != nil {
				return RedirectParams{}, fmt.Errorf("unmarshal error: %w", err)
			}

			return RedirectParams{
				Code:  "test-code",
				State: statePort.State,
			}, nil
		}

		registerOAuthSuccess(config, time.Minute)

		token, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.NoError(t, err)
		assert.True(t, token.isSuccess())
	})

	t.Run("OAuthBrowserFlow_GetRedirectError", func(t *testing.T) {
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			return RedirectParams{}, errors.New("malformed url")
		}

		_, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.Error(t, err)
	})

	t.Run("OAuthBrowserFlow_StateMismatch", func(t *testing.T) {
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			return RedirectParams{
				Code:  "test-code",
				State: "wrong-state",
			}, nil
		}

		_, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.Error(t, err)
	})

	t.Run("OAuthBrowserFlow_EmptyCode", func(t *testing.T) {
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			parsed, _ := url.Parse(authUrl)
			encodedState := parsed.Query().Get("state")
			decodedState, _ := base64.RawURLEncoding.DecodeString(encodedState)
			var statePort stateWithPort
			json.Unmarshal(decodedState, &statePort)
			return RedirectParams{Code: "", State: statePort.State}, nil
		}

		_, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.Error(t, err)
	})

	t.Run("OAuthBrowserFlow_TokenEndpointFailure", func(t *testing.T) {
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			parsed, _ := url.Parse(authUrl)
			encodedState := parsed.Query().Get("state")
			decodedState, _ := base64.RawURLEncoding.DecodeString(encodedState)
			var statePort stateWithPort
			json.Unmarshal(decodedState, &statePort)
			return RedirectParams{Code: "test-code", State: statePort.State}, nil
		}

		httpmock.RegisterResponder("POST", config.Auth.Token, httpmock.NewStringResponder(401, `{"error": "access_denied"}`))

		_, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.Error(t, err)
	})

	t.Run("OAuthBrowserFlow_Timeout", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping long tests")
		}

		// Mock function that delays for 2 seconds (longer than 1s timeout)
		mockGetRedirect := func(authUrl string) (RedirectParams, error) {
			time.Sleep(2 * time.Second)
			return RedirectParams{}, errors.New("timeout not triggered")
		}

		_, err := OAuthBrowserFlow(config, httpClient, 8080, mockGetRedirect)
		assert.ErrorContains(t, err, "timeout waiting for browser auth")
	})
}
