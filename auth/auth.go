package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type AuthType int

const (
	// Authenticated with `hytale-launcher` client ID and `auth:launcher` scope
	Launcher AuthType = iota
	// Authenticated with `hytale-server` client ID and `auth:server` scope
	Server
)

func (at AuthType) ClientID() string {
	switch at {
	case Launcher:
		return "hytale-launcher"
	case Server:
		return "hytale-server"
	default:
		panic("Unknown auth type")
	}
}

func (at AuthType) String() string {
	switch at {
	case Launcher:
		return "launcher"
	case Server:
		return "server"
	default:
		panic("Unknown auth type")
	}
}

type TokenResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	IDToken          string `json:"id_token"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
	// In seconds from request time
	ExpiresIn int `json:"expires_in"`
}

func (t TokenResponse) isSuccess() bool {
	return t.Error == "" && t.AccessToken != ""
}

type DeviceAuthResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	// In seconds from request time
	ExpiresIn int `json:"expires_in"`
	// In seconds
	Interval int `json:"interval"`
}

func defaultDeviceAuthResponse() DeviceAuthResponse {
	return DeviceAuthResponse{
		ExpiresIn: 600,
		Interval:  5,
	}
}

// Starts the OAuth device flow, used for Server auth. This will block until authentication is finished.
// onAuthRequired is called with the verification URL and code to be shown to the user.
func OAuthDeviceFlow(config *config.Config, httpClient *http.Client, onAuthRequired func(DeviceAuthResponse)) (TokenResponse, error) {
	deviceAuthResponse, err := startDeviceAuth(config, httpClient)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to start device auth: %v", err)
	}

	if onAuthRequired != nil {
		onAuthRequired(deviceAuthResponse)
	}

	tokenResponse, err := pollForToken(config, httpClient, deviceAuthResponse)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to poll for token: %v", err)
	}

	return tokenResponse, nil
}

func startDeviceAuth(config *config.Config, httpClient *http.Client) (DeviceAuthResponse, error) {
	params := url.Values{}
	params.Add("client_id", "hytale-server")
	params.Add("scope", "openid offline auth:server")

	resp, err := httpClient.PostForm(config.Auth.DeviceAuth, params)
	if err != nil {
		return DeviceAuthResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return DeviceAuthResponse{}, util.NewBadResponseError("Start device auth", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return DeviceAuthResponse{}, err
	}

	deviceAuthResponse := defaultDeviceAuthResponse()
	err = json.Unmarshal(body, &deviceAuthResponse)
	if err != nil {
		return DeviceAuthResponse{}, err
	}

	return deviceAuthResponse, nil
}

func pollForToken(config *config.Config, httpClient *http.Client, deviceAuthResponse DeviceAuthResponse) (TokenResponse, error) {
	params := url.Values{}
	params.Add("client_id", "hytale-server")
	params.Add("device_code", deviceAuthResponse.DeviceCode)
	params.Add("grant_type", "urn:ietf:params:oauth:grant-type:device_code")

	ticker := time.NewTicker(time.Duration(deviceAuthResponse.Interval) * time.Second)
	defer ticker.Stop()

	timeout := time.After(time.Duration(deviceAuthResponse.ExpiresIn) * time.Second)

	for {
		select {
		case <-ticker.C:
			resp, err := httpClient.PostForm(config.Auth.Token, params)
			if err != nil {
				return TokenResponse{}, err
			}
			defer resp.Body.Close()

			if (resp.StatusCode < 200 || resp.StatusCode >= 300) && resp.StatusCode != 400 {
				return TokenResponse{}, util.NewBadResponseError("Poll for token", resp)
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return TokenResponse{}, err
			}

			var tokenResponse TokenResponse
			err = json.Unmarshal(body, &tokenResponse)
			if err != nil {
				return TokenResponse{}, err
			}

			if tokenResponse.Error == "" {
				return tokenResponse, nil
			}
			if tokenResponse.Error != "authorization_pending" {
				return TokenResponse{}, fmt.Errorf("error polling for token: `%s`: %s", tokenResponse.Error, tokenResponse.ErrorDescription)
			}

		case <-timeout:
			return TokenResponse{}, fmt.Errorf("timeout waiting for token")
		}
	}
}

type RedirectParams struct {
	Code  string
	State string
}

type c struct {
	params RedirectParams
	err    error
}

// Starts the OAuth browser flow, used for Launcher auth. This will block until authentication is finished.
//
// getRedirectFromUser is called with the auth URL to be shown to the user, and must return the code/state parameters extracted from a browser redirect.
// You can either start a local web server and intercept the redirect, or ask the user to paste the redirect URL.
//
// port is the port the user will be redirected to. If using a local web server, this is the port of the web server.
// If not using a local web server, port should be an arbitrary unused port.
func OAuthBrowserFlow(config *config.Config, httpClient *http.Client, port uint16, getRedirectFromUser func(string) (RedirectParams, error)) (TokenResponse, error) {
	state, err := generateRandomString(32)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to generate state: %w", err)
	}
	encodedState := encodeStateWithPort(state, port)

	codeVerifier, err := generateRandomString(64)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to generate code verifier: %w", err)
	}
	codeChallenge := calculateCodeChallenge(codeVerifier)

	authUrl := buildAuthStateURL(config, encodedState, codeChallenge)

	resultChan := make(chan c, 1)
	go func() {
		params, err := getRedirectFromUser(authUrl)
		resultChan <- c{params, err}
	}()

	timeout := time.After(time.Duration(config.Auth.BrowserAuthTimeout) * time.Second)
	select {
	case result := <-resultChan:
		if result.err != nil {
			return TokenResponse{}, fmt.Errorf("getQueryFromRedirect failed: %w", result.err)
		}
		clientRedirectQuery := result.params

		if clientRedirectQuery.State != state {
			return TokenResponse{}, fmt.Errorf("invalid OAuth state parameter in redirect")
		}
		code := clientRedirectQuery.Code
		if code == "" {
			return TokenResponse{}, errors.New("authorization code missing in redirect query parameters")
		}

		tokenResp, err := exchangeCodeForToken(config, httpClient, code, codeVerifier)
		if err != nil {
			return TokenResponse{}, fmt.Errorf("token exchange failed: %w", err)
		}
		return tokenResp, nil
	case <-timeout:
		return TokenResponse{}, fmt.Errorf("timeout waiting for browser auth after %d seconds", config.Auth.BrowserAuthTimeout)
	}
}

func generateRandomString(len int) (string, error) {
	bytes := make([]byte, len)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

type stateWithPort struct {
	State string `json:"state"`
	Port  uint16 `json:"port"`
}

func encodeStateWithPort(state string, port uint16) string {
	s := stateWithPort{
		State: state,
		Port:  port,
	}
	bytes, _ := json.Marshal(s)
	return base64.RawURLEncoding.EncodeToString(bytes)
}

func calculateCodeChallenge(codeVerifier string) string {
	hash := sha256.Sum256([]byte(codeVerifier))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func buildAuthStateURL(config *config.Config, state string, codeChallenge string) string {
	params := url.Values{}
	params.Add("response_type", "code")
	params.Add("client_id", "hytale-launcher")
	params.Add("redirect_uri", config.Auth.RedirectURI)
	params.Add("scope", "openid offline auth:launcher")
	params.Add("state", state)
	params.Add("code_challenge", codeChallenge)
	params.Add("code_challenge_method", "S256")
	return fmt.Sprintf("%s?%s", config.Auth.BrowserAuth, params.Encode())
}

func exchangeCodeForToken(config *config.Config, httpClient *http.Client, code string, verifier string) (TokenResponse, error) {
	form := url.Values{}
	form.Add("grant_type", "authorization_code")
	form.Add("client_id", "hytale-launcher")
	form.Add("code", code)
	form.Add("redirect_uri", config.Auth.RedirectURI)
	form.Add("code_verifier", verifier)

	req, err := http.NewRequest("POST", config.Auth.Token, strings.NewReader(form.Encode()))
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpClient.Do(req)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to execute token request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	body := string(bodyBytes)

	if resp.StatusCode != http.StatusOK {
		return TokenResponse{}, fmt.Errorf("token exchange failed with HTTP status %d: %s", resp.StatusCode, body)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal([]byte(body), &tokenResp); err != nil {
		return TokenResponse{}, fmt.Errorf("failed to parse token response JSON: %w", err)
	}
	return tokenResp, nil
}

func OAuthRefresh(config *config.Config, httpClient *http.Client, oauthRefreshToken string, authType AuthType) (TokenResponse, error) {
	params := url.Values{}
	params.Add("client_id", authType.ClientID())
	params.Add("refresh_token", oauthRefreshToken)
	params.Add("grant_type", "refresh_token")

	resp, err := httpClient.PostForm(config.Auth.Token, params)
	if err != nil {
		return TokenResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return TokenResponse{}, util.NewBadResponseError("OAuth refresh", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return TokenResponse{}, err
	}

	var tokenResponse TokenResponse
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return TokenResponse{}, err
	}

	if !tokenResponse.isSuccess() {
		return TokenResponse{}, fmt.Errorf("failed to refresh token: %s", tokenResponse.Error)
	}

	return tokenResponse, nil
}
