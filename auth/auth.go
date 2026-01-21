package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	Error        string `json:"error"`
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

type LauncherDataResponse struct {
	Owner    string        `json:"owner"`
	Profiles []GameProfile `json:"profiles"`
}

type GameProfile struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

type GameSessionRequest struct {
	UUID string `json:"uuid"`
}
type GameSessionResponse struct {
	SessionToken  string `json:"sessionToken"`
	IdentityToken string `json:"identityToken"`
	// In RFC3339Nano format
	ExpiresAt string `json:"expiresAt"`
}

// Starts the OAuth device flow, printing a log message asking the user to visit the verification link
// and enter a code. This will block until authentication is finished.
func OAuthDeviceFlow(config *config.Config, httpClient *http.Client) (TokenResponse, error) {
	deviceAuthResponse, err := startDeviceAuth(config, httpClient)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to start device auth: %v", err)
	}

	log.Println("===================================")
	log.Println("===== Authentication Required =====")
	log.Printf("Visit: %s", deviceAuthResponse.VerificationURI)
	fmt.Printf("Enter code: %s", deviceAuthResponse.UserCode)
	log.Println("===================================")

	tokenResponse, err := pollForToken(deviceAuthResponse, config, httpClient)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("failed to poll for token: %v", err)
	}

	return tokenResponse, nil
}

func startDeviceAuth(config *config.Config, httpClient *http.Client) (DeviceAuthResponse, error) {
	params := url.Values{}
	params.Add("client_id", config.Auth.ClientID)
	params.Add("scope", config.Auth.Scope)

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

func pollForToken(deviceAuthResponse DeviceAuthResponse, config *config.Config, httpClient *http.Client) (TokenResponse, error) {
	params := url.Values{}
	params.Add("client_id", config.Auth.ClientID)
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

		case <-timeout:
			return TokenResponse{}, fmt.Errorf("timeout waiting for token")
		}
	}
}

func OAuthRefresh(oauthRefreshToken string, config *config.Config, httpClient *http.Client) (TokenResponse, error) {
	params := url.Values{}
	params.Add("client_id", config.Auth.ClientID)
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

func GetAccountProfiles(oauthAccessToken string, config *config.Config, httpClient *http.Client) (LauncherDataResponse, error) {
	req, err := http.NewRequest("GET", config.Auth.Profiles, nil)
	if err != nil {
		return LauncherDataResponse{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", oauthAccessToken))
	resp, err := httpClient.Do(req)
	if err != nil {
		return LauncherDataResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LauncherDataResponse{}, util.NewBadResponseError("Get account profiles", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return LauncherDataResponse{}, err
	}

	var launcherDataResponse LauncherDataResponse
	err = json.Unmarshal(body, &launcherDataResponse)
	if err != nil {
		return LauncherDataResponse{}, err
	}

	return launcherDataResponse, nil
}

func CreateGameSession(oauthAccessToken string, uuid string, config *config.Config, httpClient *http.Client) (GameSessionResponse, error) {
	requestBody, err := json.Marshal(GameSessionRequest{UUID: uuid})
	if err != nil {
		return GameSessionResponse{}, err
	}

	req, err := http.NewRequest("POST", config.Auth.CreateGameSession, bytes.NewBuffer(requestBody))
	if err != nil {
		return GameSessionResponse{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", oauthAccessToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return GameSessionResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GameSessionResponse{}, util.NewBadResponseError("Create game session", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GameSessionResponse{}, err
	}

	var gameSessionResponse GameSessionResponse
	err = json.Unmarshal(body, &gameSessionResponse)
	if err != nil {
		return GameSessionResponse{}, err
	}

	return gameSessionResponse, nil
}

func RefreshGameSession(gameSessionToken string, uuid string, config *config.Config, httpClient *http.Client) (GameSessionResponse, error) {
	req, err := http.NewRequest("POST", config.Auth.RefreshGameSession, nil)
	if err != nil {
		return GameSessionResponse{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", gameSessionToken))
	req.Header.Add("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return GameSessionResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return GameSessionResponse{}, util.NewBadResponseError("Refresh game session", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return GameSessionResponse{}, err
	}

	var gameSessionResponse GameSessionResponse
	err = json.Unmarshal(body, &gameSessionResponse)
	if err != nil {
		return GameSessionResponse{}, err
	}

	return gameSessionResponse, nil
}
