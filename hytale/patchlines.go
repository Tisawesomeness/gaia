package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type BuildDetails struct {
	BuildVersion string `json:"buildVersion"`
	// Unknown format
	ExpiresAt int `json:"expiresAt"`
}

type Profile struct {
	CreatedAt    time.Time `json:"createdAt"`
	Entitlements []string  `json:"entitlements"`
	Username     string    `json:"username"`
	UUID         string    `json:"uuid"`
}

type LauncherData struct {
	// Map of patchline ID to build data
	PatchLines map[string]BuildDetails `json:"patchlines"`
	// List of profiles associated with the currently logged-in account
	Profiles []Profile `json:"profiles"`
}

func GetLauncherData(config *config.Config, httpClient *http.Client, oauthAccessToken string) (*LauncherData, error) {
	if oauthAccessToken == "" {
		return nil, errors.New("Oauth access token is required")
	}

	req, err := http.NewRequest("GET", config.Feeds.LauncherData, nil)
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
