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
	Skin     string `json:"skin"`
	UUID     string `json:"uuid"`
	Username string `json:"username"`
}

type PublicGameProfile struct {
	Skin     map[string]*string
	UUID     string
	Username string
}

func FetchProfileFromUUID(uuid string, config config.Config, httpClient http.Client, authStore auth.AuthStore) (PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUUID+uuid, authStore, httpClient)
}

func FetchProfileFromUsername(username string, config config.Config, httpClient http.Client, authStore auth.AuthStore) (PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUsername+username, authStore, httpClient)
}

func fetchProfile(url string, authStore auth.AuthStore, httpClient http.Client) (PublicGameProfile, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return PublicGameProfile{}, err
	}

	sessionToken, err := authStore.GetGameSessionToken()
	if err != nil {
		return PublicGameProfile{}, err
	}
	req.Header.Add("Authorization", "Bearer "+sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return PublicGameProfile{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("Error reading response body: %v\n", err)
		return PublicGameProfile{}, err
	}
	println(string(body))

	var profile publicGameProfileResponse
	err = json.Unmarshal(util.TrimBOM(body), &profile)

	var skinData map[string]any
	err = json.Unmarshal([]byte(profile.Skin), &skinData)
	if err != nil {
		return PublicGameProfile{}, err
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

	return PublicGameProfile{
		Skin:     skin,
		UUID:     profile.UUID,
		Username: profile.Username,
	}, nil
}
