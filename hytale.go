package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/valkey-io/valkey-go"
)

// DownloadURL represents the download URL structure for a specific architecture.
type DownloadURL struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// PlatformDownloadURLs represents the download URLs for a specific platform.
// Using a map to dynamically capture any architecture.
type PlatformDownloadURLs map[string]*DownloadURL

// DownloadURLs represents the download URLs for all platforms.
// Using a map to dynamically capture any platform.
type DownloadURLs map[string]PlatformDownloadURLs

// HytaleRelease represents the entire JSON structure.
type HytaleRelease struct {
	Version      string       `json:"version"`
	DownloadURLs DownloadURLs `json:"download_url"`
}

type HytaleAPI struct {
	client          *valkey.Client
	config          *Config
	launcherRelease *HytaleRelease
}

func NewHytaleAPI(client *valkey.Client, config *Config) (*HytaleAPI, error) {
	api := &HytaleAPI{
		client: client,
		config: config,
	}

	release, err := getStoredLauncherRelease(client)
	if err != nil {
		return nil, err
	}
	api.launcherRelease = release

	return api, nil
}

func getStoredLauncherRelease(client *valkey.Client) (*HytaleRelease, error) {
	resp := (*client).Do(context.Background(), (*client).B().Get().Key("launcher_release:latest").Build())
	err := resp.Error()
	if err != nil {
		if valkey.IsValkeyNil(err) {
			return nil, nil
		}
		return nil, err
	}
	raw, err := resp.AsBytes()
	if err != nil {
		return nil, err
	}

	var release HytaleRelease
	err = json.Unmarshal(raw, &release)
	return &release, err
}

func (h HytaleAPI) PollLauncherRelease() error {
	release, err := h.fetchLauncherRelease()
	if err != nil {
		return err
	}
	releaseStr, _ := json.Marshal(release)
	err = (*h.client).Do(context.Background(), (*h.client).B().Set().Key("launcher_release:latest").Value(string(releaseStr)).Build()).Error()
	if err != nil {
		return err
	}

	if h.launcherRelease != nil && h.launcherRelease.Version != release.Version {
		println("new version! " + h.launcherRelease.Version + " -> " + release.Version)
	}
	return nil
}

func (h HytaleAPI) fetchLauncherRelease() (*HytaleRelease, error) {
	resp, err := http.Get(h.config.API.Endpoint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	var release HytaleRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	release.Version = "sample text"
	return &release, err
}
