package hytale

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

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

func FetchProfileFromUUID(uuid string, config *config.Config, httpClient *http.Client, authStore *auth.AuthStore) (*PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUUID+uuid, authStore, httpClient)
}

func FetchProfileFromUsername(username string, config *config.Config, httpClient *http.Client, authStore *auth.AuthStore) (*PublicGameProfile, error) {
	return fetchProfile(config.Profile.ByUsername+username, authStore, httpClient)
}

func fetchProfile(url string, authStore *auth.AuthStore, httpClient *http.Client) (*PublicGameProfile, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	sessionToken, err := authStore.GetGameSessionToken()
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", "Bearer "+sessionToken)

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

	skin, err := parseSkin(err, profile.Skin)
	if err != nil {
		return nil, err
	}

	return &PublicGameProfile{
		Skin:     skin,
		UUID:     profile.UUID,
		Username: profile.Username,
	}, nil
}

func parseSkin(err error, skinJson *string) (map[string]*string, error) {
	if skinJson == nil {
		return make(map[string]*string), nil
	}

	var skinData map[string]any
	err = json.Unmarshal([]byte(*skinJson), &skinData)
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

type Availability int

const (
	// Available to claim right now
	Available Availability = iota
	// Reserved by another account
	Reserved
	// Reserved by the Hytale team
	HytaleReserved
	// Contains a https://xkcd.com/1963/
	Prohibited
	// Actively in-use
	InUse
	// Unknown response (have to black-box test the API)
	Unknown
)

func CheckAvailability(username string, config *config.Config, httpClient *http.Client) (Availability, error) {
	req, err := http.NewRequest("GET", config.Profile.Availability, nil)
	if err != nil {
		return 0, err
	}
	q := req.URL.Query()
	q.Add("username", username)
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Cookie", fmt.Sprintf("ory_kratos_session=%s", config.Kratos.SessionCookie))

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Hytale returns 400 if username reserved, even though the request is fine
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusBadRequest {
		return 0, util.NewBadResponseError("Check availability", resp)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	responseBody := string(body)
	if strings.Contains(responseBody, "Username is already in use") {
		return InUse, nil
	} else if strings.Contains(responseBody, "Username is already reserved") {
		return Reserved, nil
	} else if strings.Contains(responseBody, "reserved by the Hytale Team") {
		return HytaleReserved, nil
	} else if strings.Contains(responseBody, "prohibited word") {
		return Prohibited, nil
	} else if strings.TrimSpace(responseBody) == "" {
		return Available, nil
	} else {
		log.Println("Unknown username availability response: " + responseBody[:min(50, len(responseBody))])
		return Unknown, nil
	}
}
