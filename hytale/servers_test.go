package hytale

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil/atestutil"
	"github.com/Tisawesomeness/gaia/testutil/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
	"github.com/stretchr/testify/assert"
)

var (
	expectedDiscoveryResponse = []*ServerListing{
		{
			Audience:       AudienceEveryone,
			CreatedAt:      time.Date(2026, time.May, 4, 19, 5, 26, 609484087, time.UTC),
			Description:    "FULL LOOT PVP\n\nMASSIVE DUNGEONS",
			Favorites:      80,
			Host:           "play.darktale.org",
			Likes:          61,
			Name:           "Darktale",
			OwnerProfileId: "f46a5cee-1e0e-45a1-a7dc-83a10b0df2fc",
			Port:           5520,
			Regions: []Region{
				RegionNAEast,
				RegionNAWest,
				RegionEUWest,
				RegionEUCentral,
				RegionEUEast,
			},
			ServerType: ServerTypePvP,
			ServerUUID: "625ade47-aab1-4e83-a628-1fcb5ed62285",
		},
	}
	expectedDiscoveryResponseUnknown = []*ServerListing{
		{
			Audience:       AudienceUnknown,
			CreatedAt:      time.Date(2026, time.May, 4, 19, 5, 26, 609484087, time.UTC),
			Description:    "FULL LOOT PVP\n\nMASSIVE DUNGEONS",
			Favorites:      80,
			Host:           "play.darktale.org",
			Likes:          61,
			Name:           "Darktale",
			OwnerProfileId: "f46a5cee-1e0e-45a1-a7dc-83a10b0df2fc",
			Port:           5520,
			Regions: []Region{
				RegionUnknown,
			},
			ServerType: ServerTypeOther,
			ServerUUID: "625ade47-aab1-4e83-a628-1fcb5ed62285",
		},
	}
)

func TestServerDiscovery(t *testing.T) {
	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Servers: config.ServersConfig{
			Discovery: "https://server-discovery.example.com/servers/listings",
		},
	}

	authStore := atestutil.NewTestAuthStore(auth.Launcher)

	t.Run("success case (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Servers.Discovery, httpmock.NewStringResponder(200, testutil.SampleServerResponse))

		servers, err := GetServers(config, httpClient, authStore)

		assert.NoError(t, err)
		assert.NotNil(t, servers)
		td.Cmp(t, servers, expectedDiscoveryResponse)
	})

	t.Run("unknown audience/region/serverType (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Servers.Discovery, httpmock.NewStringResponder(200, testutil.SampleServerResponseUnknown))

		servers, err := GetServers(config, httpClient, authStore)

		assert.NoError(t, err)
		assert.NotNil(t, servers)
		td.Cmp(t, servers, expectedDiscoveryResponseUnknown)
	})

	t.Run("empty array is parsed correctly (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Servers.Discovery, httpmock.NewStringResponder(200, "[]"))

		servers, err := GetServers(config, httpClient, authStore)

		assert.NoError(t, err)
		assert.NotNil(t, servers)
		assert.Empty(t, servers)
	})

	t.Run("HTTP 500 error returns an error to caller", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Servers.Discovery, httpmock.NewStringResponder(500, "Internal Server Error"))

		servers, err := GetServers(config, httpClient, authStore)

		assert.Error(t, err)
		assert.Nil(t, servers)
	})

	t.Run("Errors if no auth:launcher scope", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Servers.Discovery, httpmock.NewStringResponder(200, testutil.SampleServerResponse))

		serverAuthStore := atestutil.NewTestAuthStore(auth.Server)
		servers, err := GetServers(config, httpClient, serverAuthStore)

		assert.Error(t, err)
		assert.Nil(t, servers)
	})
}
