package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"unicode"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

const (
	GameReleaseFeedID          = "game_release"
	GameReleaseFeedDisplay     = "New Hytale Release"
	LauncherReleaseFeedID      = "launcher_release"
	LauncherReleaseFeedDisplay = "New Launcher Version"
	LauncherPostFeedID         = "launcher_post"
	LauncherPostFeedDisplay    = "Launcher Articles"
	expectedFeeds              = 3
)

type Feed interface {
	GetID() string
	GetDisplayName() string
	BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed
	GetVersion() string
}

// Game Release

type GameReleaseVersion struct {
	Version string `json:"version"`
}

type gameReleaseResponse struct {
	Url string `json:"url"`
}

// Game Release Feed
type GameReleaseFeed struct {
	Version *GameReleaseVersion
}

func (f *GameReleaseFeed) GetID() string {
	return GameReleaseFeedID
}

func (f *GameReleaseFeed) GetDisplayName() string {
	return GameReleaseFeedDisplay
}

func (f *GameReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
	var title string
	if isNews {
		title = "New Hytale Version"
	} else {
		title = "Latest Hytale Version"
	}
	return &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("`%s`", f.GetVersion()),
		Color:       0x00FF00,
	}
}

func (f *GameReleaseFeed) GetVersion() string {
	if f.Version == nil {
		return ""
	}
	return f.Version.Version
}

func getStoredGameRelease(db *db.DB) (*GameReleaseVersion, error) {
	raw, err := db.GetLatestPost(GameReleaseFeedID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var release GameReleaseVersion
	err = json.Unmarshal(raw, &release)
	return &release, err
}

func (feeds HytaleFeeds) fetchGameReleaseUrl() (string, error) {
	token, err := feeds.authStore.GetOAuthToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", feeds.config.Feeds.GameRelease, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

	resp, err := feeds.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", util.NewBadResponseError("Fetch game release url", resp)
	}

	var response gameReleaseResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return "", err
	}

	return response.Url, nil
}

func (feeds HytaleFeeds) fetchGameRelease() (*GameReleaseVersion, error) {
	url, err := feeds.fetchGameReleaseUrl()
	if err != nil {
		return nil, err
	}

	resp, err := feeds.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Fetch game release", resp)
	}

	var release GameReleaseVersion
	err = json.NewDecoder(resp.Body).Decode(&release)
	return &release, err
}

// Launcher Release

type DownloadURL struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

type PlatformDownloadURLs map[string]DownloadURL

type DownloadURLs map[string]PlatformDownloadURLs

type LauncherRelease struct {
	Version      string       `json:"version"`
	DownloadURLs DownloadURLs `json:"download_url"`
}

type LauncherReleaseFeed struct {
	Release *LauncherRelease
}

func (f *LauncherReleaseFeed) GetID() string {
	return LauncherReleaseFeedID
}

func (f *LauncherReleaseFeed) GetDisplayName() string {
	return LauncherReleaseFeedDisplay
}

func (f *LauncherReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
	// Prepare the embed with version and download links
	var title string
	if isNews {
		title = "New Hytale Launcher Version"
	} else {
		title = "Latest Hytale Launcher Version"
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("`%s`", f.GetVersion()),
		Color:       0x00FF00,
	}

	// Add download links for each platform
	if f.Release != nil && f.Release.DownloadURLs != nil {
		fields := []*discordgo.MessageEmbedField{}
		for platform, urls := range f.Release.DownloadURLs {
			for arch, downloadURL := range urls {
				platformName := capitalizeFirstLetter(platform)
				fieldName := fmt.Sprintf("%s (%s)", platformName, arch)
				fields = append(fields, &discordgo.MessageEmbedField{
					Name:   fieldName,
					Value:  fmt.Sprintf("[Download](%s)", downloadURL.URL),
					Inline: false,
				})
			}
		}
		embed.Fields = fields
	}

	return embed
}

func capitalizeFirstLetter(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func (f *LauncherReleaseFeed) GetVersion() string {
	return f.Release.Version
}

func getStoredLauncherRelease(db *db.DB) (*LauncherRelease, error) {
	raw, err := db.GetLatestPost(LauncherReleaseFeedID)
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var release LauncherRelease
	err = json.Unmarshal(raw, &release)
	return &release, err
}

func (feeds HytaleFeeds) fetchLauncherRelease() (*LauncherRelease, error) {
	resp, err := feeds.http.Get(feeds.config.Feeds.LauncherRelease)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Fetch launcher release", resp)
	}

	var release LauncherRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	return &release, err
}

// Articles

type Article struct {
	Title       string `json:"title"`
	DestURL     string `json:"dest_url"`
	Description string `json:"description"`
	ImageURL    string `json:"image_url"`
}

func (a *Article) BuildMessage(config *config.Config) *discordgo.MessageEmbed {
	return &discordgo.MessageEmbed{
		Title:       a.Title,
		URL:         a.DestURL,
		Description: a.Description,
		Image:       &discordgo.MessageEmbedImage{URL: config.Feeds.ArticleImagePrefix + a.ImageURL},
		Color:       0x00FF00,
	}
}

type ArticleFeed struct {
	Articles []*Article `json:"articles"`
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

func (f *LauncherPostFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
	if len(f.Articles.Articles) <= 0 {
		return &discordgo.MessageEmbed{
			Title:       "Hytale Articles",
			Description: "No articles yet...",
			Color:       0xFF0000,
		}
	}
	latestArticle := f.Articles.Articles[0]
	return latestArticle.BuildMessage(config)
}

func (f *LauncherPostFeed) GetVersion() string {
	if len(f.Articles.Articles) <= 0 {
		return ""
	}
	return f.Articles.Articles[0].DestURL
}

func getStoredArticles(db *db.DB) (*ArticleFeed, error) {
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

func (feeds HytaleFeeds) fetchArticles() (*ArticleFeed, error) {
	resp, err := feeds.http.Get(feeds.config.Feeds.LauncherArticles)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Fetch articles", resp)
	}

	var articles ArticleFeed
	err = json.NewDecoder(resp.Body).Decode(&articles)
	return &articles, err
}

// Management

type HytaleFeeds struct {
	Feeds     map[string]Feed
	config    *config.Config
	db        *db.DB
	http      *http.Client
	authStore *auth.AuthStore
}

func NewHytaleFeeds(config *config.Config, db *db.DB, http *http.Client, authStore *auth.AuthStore) (*HytaleFeeds, error) {
	feeds := &HytaleFeeds{
		Feeds:     make(map[string]Feed),
		config:    config,
		db:        db,
		http:      http,
		authStore: authStore,
	}

	// Initialize feeds
	err := feeds.initializeFeeds()
	if err != nil {
		return nil, err
	}

	if len(feeds.Feeds) < expectedFeeds {
		log.Println("Feeds have not been stored yet, fetching...")
		err = feeds.Poll()
		if err != nil {
			return nil, err
		}
		if len(feeds.Feeds) < expectedFeeds {
			return nil, errors.New("feed state was not initialized")
		}
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

	// Initialize game release feed
	gameRelease, err := getStoredGameRelease(feeds.db)
	if err != nil {
		return err
	}
	if gameRelease != nil {
		feeds.Feeds[GameReleaseFeedID] = &GameReleaseFeed{Version: gameRelease}
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

	// Handle game release
	gameRelease, err := feeds.fetchGameRelease()
	if err != nil {
		return err
	}
	gameReleaseStr, _ := json.Marshal(gameRelease)
	err = feeds.db.SetLatestPost(GameReleaseFeedID, string(gameReleaseStr))
	if err != nil {
		return err
	}

	// Update or add game release feed
	feeds.updateOrAddFeed(&GameReleaseFeed{Version: gameRelease})

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
					message := feed.BuildMessage(feeds.config, true)
					_, err = s.ChannelMessageSendEmbed(channelId, message)
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

func (feeds HytaleFeeds) removeAllSubscriptions(channelId string) {
	for feedID := range feeds.Feeds {
		feeds.db.RemoveSubscription(feedID, channelId)
	}
}
