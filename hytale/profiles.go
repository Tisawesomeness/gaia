package hytale

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type publicGameProfileResponse struct {
	// Can be absent!
	Skin     *string `json:"skin"`
	UUID     string  `json:"uuid"`
	Username string  `json:"username"`
}

type PublicGameProfile struct {
	// May be empty
	Skin     map[string]*string
	UUID     string
	Username string
}

func FetchProfileFromUUID(config *config.Config, httpClient *http.Client, authStore auth.AuthStore, uuid string) (*PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUUID+uuid, httpClient, authStore)
}

func FetchProfileFromUsername(config *config.Config, httpClient *http.Client, authStore auth.AuthStore, username string) (*PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUsername+username, httpClient, authStore)
}

func fetchProfile(url string, httpClient *http.Client, authStore auth.AuthStore) (*PublicGameProfile, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	sessionToken, err := authStore.GetGameSessionToken()
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+sessionToken.Token)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Fetch profile", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return nil, err
	}

	var profile publicGameProfileResponse
	err = json.Unmarshal(body, &profile)
	if err != nil {
		return nil, err
	}

	skin, err := parseSkin(profile.Skin)
	if err != nil {
		return nil, err
	}

	return &PublicGameProfile{
		Skin:     skin,
		UUID:     profile.UUID,
		Username: profile.Username,
	}, nil
}

func parseSkin(skinJson *string) (map[string]*string, error) {
	if skinJson == nil {
		return make(map[string]*string), nil
	}

	var skinData map[string]any
	err := json.Unmarshal([]byte(*skinJson), &skinData)
	if err != nil {
		return nil, err
	}

	skin := make(map[string]*string)
	for key, value := range skinData {
		if value == nil {
			skin[key] = nil
		} else {
			stringValue := fmt.Sprintf("%v", value)
			skin[key] = &stringValue
		}
	}
	return skin, nil
}
