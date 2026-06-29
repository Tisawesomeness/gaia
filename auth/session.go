package auth

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type GameProfile struct {
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

func GetAccountProfiles(oauthAccessToken string, authType AuthType, config *config.Config, httpClient *http.Client) ([]GameProfile, error) {
	switch authType {
	case Launcher:
		data, err := GetLauncherData(config, httpClient, oauthAccessToken)
		if err != nil {
			return nil, err
		}
		return data.Profiles, nil
	case Server:
		data, err := getAccountProfiles(oauthAccessToken, config, httpClient)
		if err != nil {
			return nil, err
		}
		return data.Profiles, nil
	default:
		panic("unknown auth type")
	}
}

type accountProfilesResponse struct {
	Owner    string        `json:"owner"`
	Profiles []GameProfile `json:"profiles"`
}

// Server auth only!
func getAccountProfiles(oauthAccessToken string, config *config.Config, httpClient *http.Client) (accountProfilesResponse, error) {
	req, err := http.NewRequest("GET", config.Auth.Profiles, nil)
	if err != nil {
		return accountProfilesResponse{}, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", oauthAccessToken))
	resp, err := httpClient.Do(req)
	if err != nil {
		return accountProfilesResponse{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return accountProfilesResponse{}, util.NewBadResponseError("Get account profiles", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return accountProfilesResponse{}, err
	}

	var launcherDataResponse accountProfilesResponse
	err = json.Unmarshal(body, &launcherDataResponse)
	if err != nil {
		return accountProfilesResponse{}, err
	}

	return launcherDataResponse, nil
}

type BuildDetails struct {
	BuildVersion string `json:"buildVersion"`
	ExpiresAt    int64  `json:"expiresAt"` // unix timestamp, 0 if no expiry
}

type LauncherData struct {
	// Map of patchline ID to build data
	PatchLines map[string]BuildDetails `json:"patchlines"`
	// List of profiles associated with the currently logged-in account
	Profiles []GameProfile `json:"profiles"`
}

// Launcher auth only!
func GetLauncherData(config *config.Config, httpClient *http.Client, oauthAccessToken string) (*LauncherData, error) {
	if oauthAccessToken == "" {
		return nil, errors.New("Oauth access token is required")
	}

	req, err := http.NewRequest("GET", config.Auth.LauncherData, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+oauthAccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Get launcher data", resp)
	}

	var data LauncherData
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("Error reading launcher data body: %w", err)
	}

	return &data, nil
}

type gameSessionRequest struct {
	UUID string `json:"uuid"`
}
type GameSessionResponse struct {
	SessionToken  string    `json:"sessionToken"`
	IdentityToken string    `json:"identityToken"`
	ExpiresAt     time.Time `json:"expiresAt"`
}

func CreateGameSession(oauthAccessToken string, uuid string, config *config.Config, httpClient *http.Client) (GameSessionResponse, error) {
	requestBody, err := json.Marshal(gameSessionRequest{UUID: uuid})
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
