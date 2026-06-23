package hytale

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
)

type Audience int

const (
	AudienceEveryone = iota
	AudienceTeen
	AudienceMature
	// Placeholder for unknown responses, cannot be used as a filter
	AudienceUnknown
)

type ServerType int

const (
	ServerTypeSurvival = iota
	ServerTypeAdventureRPG
	ServerTypeCreative
	ServerTypePvP
	ServerTypeMinigames
	ServerTypeRoleplay
	ServerTypeSocial
	ServerTypeSandbox
	// Also represents unknown responses
	ServerTypeOther
)

type Region int

const (
	RegionNAEast = iota
	RegionNAWest
	RegionSouthAmerica
	RegionEUWest
	RegionEUCentral
	RegionEUEast
	RegionMiddleEast
	RegionAsiaEast
	RegionAsiaSoutheast
	RegionOceania
	// Placeholder for unknown responses, cannot be used as a filter
	RegionUnknown
)

type ServerListing struct {
	Audience       Audience   `json:"audience"`
	CreatedAt      time.Time  `json:"createdAt"`
	Description    string     `json:"description"`
	Favorites      int        `json:"favorites"`
	Host           string     `json:"host"`
	Likes          int        `json:"likes"`
	Name           string     `json:"name"`
	OwnerProfileId string     `json:"ownerProfileId"`
	Port           uint16     `json:"port"`
	Regions        []Region   `json:"regions"` // In ascending order
	ServerType     ServerType `json:"serverType"`
	ServerUUID     string     `json:"uuid"`
}

func GetServers(config *config.Config, httpClient *http.Client, authStore auth.AuthStore) ([]*ServerListing, error) {
	sessionToken, err := authStore.GetGameSessionToken()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", config.Servers.Discovery, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Get servers", resp)
	}

	var servers []*ServerListing
	err = json.NewDecoder(resp.Body).Decode(&servers)
	if err != nil {
		return nil, fmt.Errorf("Error reading server response body: %w", err)
	}

	for _, server := range servers {
		server.Audience = util.Coerce(server.Audience, AudienceEveryone, AudienceTeen, AudienceUnknown)
		server.ServerType = util.Coerce(server.ServerType, ServerTypeSurvival, ServerTypeOther, ServerTypeOther)
		for i, region := range server.Regions {
			server.Regions[i] = util.Coerce(region, RegionNAEast, RegionOceania, RegionUnknown)
		}
		sort.Slice(server.Regions, func(i, j int) bool {
			return i < j
		})
	}

	return servers, nil
}
