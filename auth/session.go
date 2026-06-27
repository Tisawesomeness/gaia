package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

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
