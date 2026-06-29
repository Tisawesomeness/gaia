package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

// Fetches all patchlines from the Hytale API, mapped to their expiration time (or nil if no expiry).
func GetPatchlines(config *config.Config, httpClient *http.Client, authStore auth.AuthStore) (map[string]*time.Time, error) {
	token, err := authStore.GetOAuthToken()
	if err != nil {
		return nil, err
	}

	if token.AuthType == auth.Server {
		patchlines, err := getPatchlines(config, httpClient, token.Token)
		if err != nil {
			return nil, err
		}
		result := make(map[string]*time.Time, len(patchlines))
		for _, p := range patchlines {
			var t *time.Time
			if p.ExpiresAt > 0 {
				parsed := time.Unix(p.ExpiresAt, 0)
				t = &parsed
			}
			result[p.Name] = t
		}
		return result, nil

	} else {
		data, err := auth.GetLauncherData(config, httpClient, token.Token)
		if err != nil {
			return nil, err
		}
		result := make(map[string]*time.Time, len(data.PatchLines))
		for name, build := range data.PatchLines {
			var t *time.Time
			if build.ExpiresAt > 0 {
				parsed := time.Unix(build.ExpiresAt, 0)
				t = &parsed
			}
			result[name] = t
		}
		return result, nil
	}
}

type patchlineResponses struct {
	Patchlines []*patchlineResponse `json:"patchlines"`
}

type patchlineResponse struct {
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expiresAt"` // unix timestamp
}

// Server auth only!
func getPatchlines(config *config.Config, httpClient *http.Client, oauthAccessToken string) ([]*patchlineResponse, error) {
	if oauthAccessToken == "" {
		return nil, errors.New("Oauth access token is required")
	}

	req, err := http.NewRequest("GET", config.Feeds.Patchlines, nil)
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

	var data patchlineResponses
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("Error reading launcher data body: %w", err)
	}

	return data.Patchlines, nil
}
