package hytale

import (
	"net/http"
	"testing"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/jarcoal/httpmock"
	"github.com/maxatome/go-testdeep/td"
)

var (
	createdAt, _ = time.Parse(time.RFC3339, "2025-12-13T22:01:18.169026Z")
	expectedData = &LauncherData{
		PatchLines: map[string]BuildDetails{
			"pre-release": {
				BuildVersion: "2026.01.17-a4cc0e7dd",
				ExpiresAt:    0,
			},
			"release": {
				BuildVersion: "2026.01.17-4b0f30090",
				ExpiresAt:    0,
			},
		},
		Profiles: []Profile{
			{
				CreatedAt:    createdAt,
				Entitlements: []string{"game.base"},
				Username:     "tis",
				UUID:         "d798091b-f494-4208-a1ba-e24da5880786",
			},
		},
	}
)

func TestGetLauncherData(t *testing.T) {
	httpClient := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(httpClient)

	config := &config.Config{
		Feeds: config.FeedsConfig{
			LauncherData: "https://account-data.example.com/my-account/get-launcher-data?arch=amd64&os=windows",
		},
	}

	t.Run("success case (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherData, httpmock.NewStringResponder(200, testutil.SampleLauncherData))

		data, err := GetLauncherData(config, httpClient, "sample-token")

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if data == nil {
			t.Fatal("Expected launcher data, got nil")
		}

		td.Cmp(t, data, expectedData)
	})

	t.Run("network failure (401 unauthorized)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherData, httpmock.NewStringResponder(http.StatusUnauthorized, ""))

		_, err := GetLauncherData(config, httpClient, "sample-token")

		if err == nil {
			t.Fatal("Expected an error on 401 unauthorized, got nil")
		}
	})

	t.Run("empty response body schema validation (200 OK)", func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Feeds.LauncherData, httpmock.NewStringResponder(200, `{"patchlines": {}, "profiles": []}`))

		data, err := GetLauncherData(config, httpClient, "sample-token")

		if err != nil {
			t.Fatalf("Expected no error for empty data, got %v", err)
		}

		expectedData := &LauncherData{
			PatchLines: make(map[string]BuildDetails),
			Profiles:   []Profile{},
		}
		td.Cmp(t, data, expectedData)
	})
}
