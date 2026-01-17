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
	LauncherReleaseFeedID      = "launcher_release"
	LauncherReleaseFeedDisplay = "New Versions"
	LauncherPostFeedID         = "launcher_post"
	LauncherPostFeedDisplay    = "Launcher Posts"
)

type Feed interface {
	GetID() string
	GetDisplayName() string
	BuildMessage() string
	GetVersion() string
}

type LauncherReleaseFeed struct {
	Release *HytaleRelease
}

func (f *LauncherReleaseFeed) GetID() string {
	return LauncherReleaseFeedID
}

func (f *LauncherReleaseFeed) GetDisplayName() string {
	return LauncherReleaseFeedDisplay
}

func (f *LauncherReleaseFeed) BuildMessage() string {
	return "new version: " + f.Release.Version
}

func (f *LauncherReleaseFeed) GetVersion() string {
	return f.Release.Version
}

type LauncherPostFeed struct {
	Articles *ArticleFeed
}

func (f *LauncherPostFeed) GetID() string {
	return LauncherPostFeedID
}

func (f *LauncherPostFeed) GetDisplayName() string {
	return LauncherPostFeedDisplay
}

func (f *LauncherPostFeed) BuildMessage() string {
	if len(f.Articles.Articles) <= 0 {
		return ""
	}
	return "new post: " + f.Articles.Articles[0].Title
}

func (f *LauncherPostFeed) GetVersion() string {
	if len(f.Articles.Articles) <= 0 {
		return ""
	}
	return f.Articles.Articles[0].DestURL
}

type HytaleFeeds struct {
	Feeds  map[string]Feed
	config config.Config
	db     db.DB
	http   http.Client
}

func NewHytaleFeeds(config config.Config, db db.DB, http http.Client) (*HytaleFeeds, error) {
	feeds := &HytaleFeeds{
		config: config,
		db:     db,
		http:   http,
		Feeds:  make(map[string]Feed),
	}

	// Initialize feeds
	err := feeds.initializeFeeds()
	if err != nil {
		return nil, err
	}

	if len(feeds.Feeds) == 0 {
		log.Println("Feeds have not been stored yet, fetching...")
		err = feeds.Poll()
		if err != nil {
			return nil, err
		}
	}

	if len(feeds.Feeds) == 0 {
		return nil, errors.New("feed state was not initialized")
	}
	return feeds, nil
}

func (feeds *HytaleFeeds) initializeFeeds() error {
	// Initialize launcher release feed
	release, err := getStoredLauncherRelease(feeds.db)
	if err != nil {
		return err
	}
	if release != nil {
		feeds.Feeds[LauncherReleaseFeedID] = &LauncherReleaseFeed{Release: release}
	}

	// Initialize articles feed
	articles, err := getStoredArticles(feeds.db)
	if err != nil {
		return err
	}
	if articles != nil {
		feeds.Feeds[LauncherPostFeedID] = &LauncherPostFeed{Articles: articles}
	}

	return nil
}

func (feeds *HytaleFeeds) Poll() error {
	// Handle launcher release
	release, err := feeds.fetchLauncherRelease()
	if err != nil {
		return err
	}
	releaseStr, _ := json.Marshal(release)
	err = feeds.db.SetLatestPost(LauncherReleaseFeedID, string(releaseStr))
	if err != nil {
		return err
	}

	// Update or add launcher release feed
	feeds.updateOrAddFeed(&LauncherReleaseFeed{Release: release})

	// Handle articles
	articles, err := feeds.fetchArticles()
	if err != nil {
		return err
	}
	articlesStr, _ := json.Marshal(articles)
	err = feeds.db.SetLatestPost(LauncherPostFeedID, string(articlesStr))
	if err != nil {
		return err
	}

	// Update or add articles feed
	feeds.updateOrAddFeed(&LauncherPostFeed{Articles: articles})

	return nil
}

func (feeds *HytaleFeeds) updateOrAddFeed(newFeed Feed) {
	feeds.Feeds[newFeed.GetID()] = newFeed
}

func (feeds HytaleFeeds) NotifyFeeds(s *discordgo.Session) error {
	for feedID, feed := range feeds.Feeds {
		subs, err := feeds.db.GetSubscriptions(feedID)
		if err != nil {
			return err
		}

		for channelId, lastKnownVersion := range subs {
			if lastKnownVersion != feed.GetVersion() {
				_, err = s.Channel(channelId)
				if err != nil {
					log.Printf("Error accessing channel, removing: %v", err)
					feeds.removeAllSubscriptions(channelId)
				} else {
					message := feed.BuildMessage()
					if message == "" {
						continue
					}
					_, err = s.ChannelMessageSend(channelId, message)
					if err != nil {
						log.Printf("Cannot send feed update: %v", err)
						continue
					}
					feeds.db.AddOrUpdateSubscription(feedID, channelId, feed.GetVersion())
				}
			}
		}
	}
	return nil
}

func getStoredLauncherRelease(db db.DB) (*HytaleRelease, error) {
	raw, err := db.GetLatestPost(LauncherReleaseFeedID)
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
	raw, err := db.GetLatestPost(LauncherPostFeedID)
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

func (feeds HytaleFeeds) removeAllSubscriptions(channelId string) {
	for feedID, _ := range feeds.Feeds {
		feeds.db.RemoveSubscription(feedID, channelId)
	}
}
