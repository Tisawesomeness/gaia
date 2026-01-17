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

type Article struct {
	Title       string `json:"title"`
	DestURL     string `json:"dest_url"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

type ArticleFeed struct {
	Articles []*Article `json:"articles"`
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
	Articles        *ArticleFeed
	config          config.Config
	db              db.DB
	http            http.Client
}

func NewHytaleFeeds(config config.Config, db db.DB, http http.Client) (*HytaleFeeds, error) {
	feeds := &HytaleFeeds{
		config: config,
		db:     db,
		http:   http,
	}

	// Initialize release
	release, err := getStoredLauncherRelease(db)
	if err != nil {
		return nil, err
	}

	// Initialize articles
	articles, err := getStoredArticles(db)
	if err != nil {
		return nil, err
	}

	if release == nil || articles == nil {
		// If db is missing values, Poll() will update our in-memory copy
		log.Println("A feed has not been stored yet, fetching...")
		err = feeds.Poll()
		if err != nil {
			return nil, err
		}
	} else {
		feeds.LauncherRelease = release
		feeds.Articles = articles
	}

	if feeds.LauncherRelease == nil || feeds.Articles == nil {
		return nil, errors.New("feed state was not initialized")
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

func getStoredArticles(db db.DB) (*ArticleFeed, error) {
	raw, err := db.GetLatestPost(LauncherPostFeed)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var articles ArticleFeed
	err = json.Unmarshal(raw, &articles)
	return &articles, err
}

func (feeds *HytaleFeeds) Poll() error {
	// Handle launcher release
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

	// Handle articles
	articles, err := feeds.fetchArticles()
	if err != nil {
		return err
	}
	articlesStr, _ := json.Marshal(articles)
	err = feeds.db.SetLatestPost(LauncherPostFeed, string(articlesStr))
	if err != nil {
		return err
	}

	if feeds.Articles == nil || latestArticleUrl(*feeds.Articles) != latestArticleUrl(*articles) {
		feeds.Articles = articles
	}

	return nil
}

func latestArticleUrl(a ArticleFeed) string {
	if len(a.Articles) <= 0 {
		return ""
	}
	return a.Articles[0].DestURL
}

func (feeds HytaleFeeds) fetchLauncherRelease() (*HytaleRelease, error) {
	resp, err := feeds.http.Get(feeds.config.Feeds.LauncherRelease)
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

func (feeds HytaleFeeds) fetchArticles() (*ArticleFeed, error) {
	resp, err := feeds.http.Get(feeds.config.Feeds.LauncherArticles)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}

	var articles ArticleFeed
	err = json.NewDecoder(resp.Body).Decode(&articles)
	return &articles, err
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

	subs, err = feeds.db.GetSubscriptions(LauncherPostFeed)
	if err != nil {
		return err
	}
	for channelId, lastKnownVersion := range subs {
		if lastKnownVersion != latestArticleUrl(*feeds.Articles) {
			_, err = s.Channel(channelId)
			if err != nil {
				log.Printf("Error accessing channel, removing: %v", err)
				feeds.db.RemoveSubscription(LauncherReleaseFeed, channelId)
			} else {
				if len(feeds.Articles.Articles) <= 0 {
					continue
				}
				_, err = s.ChannelMessageSend(channelId, "new post: "+feeds.Articles.Articles[0].Title)
				if err != nil {
					log.Printf("Cannot send feed update: %v", err)
				}
			}
		}
	}

	return nil
}
