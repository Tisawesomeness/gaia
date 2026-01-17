package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/bwmarrin/discordgo"
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

const (
	LauncherReleaseFeed = "launcher_release"
	LauncherPostFeed    = "launcher_post"
)

var (
	Feeds = []string{LauncherReleaseFeed, LauncherPostFeed}
)

func FriendlyFeedName(feedId string) string {
	switch feedId {
	case LauncherReleaseFeed:
		return "New Versions"
	case LauncherPostFeed:
		return "Launcher Posts"
	default:
		return feedId
	}
}

type HytaleFeeds struct {
	LauncherRelease *HytaleRelease
	config          config.Config
	db              db.DB
}

func NewHytaleFeeds(config config.Config, db db.DB) (*HytaleFeeds, error) {
	feeds := &HytaleFeeds{
		config: config,
		db:     db,
	}

	release, err := getStoredLauncherRelease(db)
	if err != nil {
		return nil, err
	}
	if release == nil {
		// If release not in db, Poll() will update our in-memory copy
		err = feeds.Poll()
		if err != nil {
			return nil, err
		}
	} else {
		feeds.LauncherRelease = release
	}

	if feeds.LauncherRelease == nil {
		return nil, errors.New("launcher release was not initialized")
	}
	return feeds, nil
}

func getStoredLauncherRelease(db db.DB) (*HytaleRelease, error) {
	raw, err := db.GetLatestPost(LauncherReleaseFeed)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var release HytaleRelease
	err = json.Unmarshal(raw, &release)
	return &release, err
}

func (feeds *HytaleFeeds) Poll() error {
	release, err := feeds.fetchLauncherRelease()
	if err != nil {
		return err
	}
	releaseStr, _ := json.Marshal(release)
	err = feeds.db.SetLatestPost(LauncherReleaseFeed, string(releaseStr))
	if err != nil {
		return err
	}

	if feeds.LauncherRelease == nil || feeds.LauncherRelease.Version != release.Version {
		feeds.LauncherRelease = release
	}
	return nil
}

func (feeds HytaleFeeds) fetchLauncherRelease() (*HytaleRelease, error) {
	resp, err := http.Get(feeds.config.Feeds.LauncherRelease)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	var release HytaleRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	return &release, err
}

func (feeds HytaleFeeds) NotifyLauncherReleaseFeeds(s *discordgo.Session) error {
	subs, err := feeds.db.GetSubscriptions(LauncherReleaseFeed)
	if err != nil {
		return err
	}
	for channelId, lastKnownVersion := range subs {
		if lastKnownVersion != feeds.LauncherRelease.Version {
			_, err = s.Channel(channelId)
			if err != nil {
				log.Printf("Error accessing channel, removing: %v", err)
				feeds.db.RemoveSubscription(LauncherReleaseFeed, channelId)
			} else {
				_, err = s.ChannelMessageSend(channelId, "new version: "+feeds.LauncherRelease.Version)
				if err != nil {
					log.Printf("Cannot send feed update: %v", err)
				}
			}
		}
	}
	return nil
}
