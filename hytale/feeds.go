package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
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
	GamePreReleaseFeedID       = "game_pre_release"
	GamePreReleaseFeedDisplay  = "New Hytale Pre-release"
	LauncherReleaseFeedID      = "launcher_release"
	LauncherReleaseFeedDisplay = "New Launcher Version"
	LauncherPostFeedID         = "launcher_post"
	LauncherPostFeedDisplay    = "Launcher Articles"
	expectedFeeds              = 4
)

type Patchline string

const (
	Release    Patchline = "release"
	PreRelease Patchline = "pre-release"
)

var (
	patchlines = []Patchline{Release, PreRelease}
)

func ParsePatchline(patchline string) (Patchline, error) {
	switch patchline {
	case string(Release):
		return Release, nil
	case string(PreRelease):
		return PreRelease, nil
	default:
		return "", fmt.Errorf("unknown patchline: %s", patchline)
	}
}

func (p Patchline) Display() string {
	switch p {
	case Release:
		return "Release"
	case PreRelease:
		return "Pre-release"
	default:
		panic(fmt.Errorf("unknown state: %s", p))
	}
}

func (p Patchline) FeedID() string {
	switch p {
	case Release:
		return GameReleaseFeedID
	case PreRelease:
		return GamePreReleaseFeedID
	default:
		panic(fmt.Errorf("unknown state: %s", p))
	}
}

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

// Game Release Feed, includes pre-release
type GameReleaseFeed struct {
	Version   *GameReleaseVersion
	Patchline Patchline
}

func (f *GameReleaseFeed) GetID() string {
	return f.Patchline.FeedID()
}

func (f *GameReleaseFeed) GetDisplayName() string {
	return "New Hytale " + f.Patchline.Display()
}

func (f *GameReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
	var adjective string
	if isNews {
		adjective = "New"
	} else {
		adjective = "Latest"
	}
	return &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%s Hytale %s", adjective, f.Patchline.Display()),
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

func getStoredGameRelease(patchline Patchline, db *db.DB) (*GameReleaseVersion, error) {
	raw, err := db.GetLatestPost(patchline.FeedID())
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

func (feeds HytaleFeeds) fetchGameReleaseUrl(patchline Patchline) (string, error) {
	token, err := feeds.authStore.GetOAuthToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s.json", feeds.config.Feeds.GameVersion, patchline), nil)
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

func (feeds HytaleFeeds) fetchGameRelease(patchline Patchline) (*GameReleaseVersion, error) {
	url, err := feeds.fetchGameReleaseUrl(patchline)
	if err != nil {
		return nil, err
	}

	resp, err := feeds.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s version", patchline), resp)
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
	for _, patchline := range patchlines {
		gameRelease, err := getStoredGameRelease(patchline, feeds.db)
		if err != nil {
			return err
		}
		if gameRelease != nil {
			feeds.Feeds[patchline.FeedID()] = &GameReleaseFeed{
				Version:   gameRelease,
				Patchline: patchline,
			}
		}
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
	for _, patchline := range patchlines {
		gameRelease, err := feeds.fetchGameRelease(patchline)
		if err != nil {
			return err
		}
		gameReleaseStr, _ := json.Marshal(gameRelease)
		err = feeds.db.SetLatestPost(patchline.FeedID(), string(gameReleaseStr))
		if err != nil {
			return err
		}

		// Update or add game release feed
		feeds.updateOrAddFeed(&GameReleaseFeed{
			Version:   gameRelease,
			Patchline: patchline,
		})
	}

	return nil
}

func (feeds *HytaleFeeds) updateOrAddFeed(newFeed Feed) {
	feeds.Feeds[newFeed.GetID()] = newFeed
}

func (feeds HytaleFeeds) NotifyFeeds(s *discordgo.Session) error {
	for feedID, feed := range feeds.Feeds {
		targetIDs, err := feeds.db.GetSubscriptions(feedID)
		if err != nil {
			return err
		}

		for _, targetID := range targetIDs {
			sub, err := feeds.db.GetSubscription(feedID, targetID)
			if err != nil {
				log.Printf("Error getting subscription from db: %v", err)
				continue
			}

			if sub.CurrentVersion() != feed.GetVersion() {
				switch sub := sub.(type) {
				case db.GuildSubscription:
					_, err = s.Channel(targetID)
					if err != nil {
						log.Printf("Error accessing channel, removing: %v", err)
						feeds.removeAllSubscriptions(targetID)
					} else {
						message := feed.BuildMessage(feeds.config, true)
						_, err = s.ChannelMessageSendComplex(targetID, &discordgo.MessageSend{
							Content: roleMentions(sub.Roles),
							Embeds:  []*discordgo.MessageEmbed{message},
							AllowedMentions: &discordgo.MessageAllowedMentions{
								Roles: sub.Roles,
							},
						})
						if err != nil {
							log.Printf("Cannot send feed update: %v", err)
							continue
						}

						feeds.db.AddOrUpdateSubscription(feedID, targetID, db.GuildSubscription{
							Version: feed.GetVersion(),
							Roles:   sub.Roles,
						})
					}

				case db.UserSubscription:
					_, err = s.User(targetID)
					if err != nil {
						log.Printf("Error accessing user, removing: %v", err)
						feeds.removeAllSubscriptions(targetID)
					} else {
						dm, err := s.UserChannelCreate(targetID)
						if err != nil {
							log.Printf("Cannot open DM: %v", err)
							continue
						}

						message := feed.BuildMessage(feeds.config, true)
						_, err = s.ChannelMessageSendEmbed(dm.ID, message)
						if err != nil {
							log.Printf("Cannot send feed update: %v", err)
							continue
						}

						feeds.db.AddOrUpdateSubscription(feedID, targetID, db.UserSubscription{
							Version: feed.GetVersion(),
						})
					}

				default:
					panic("Invalid subscription type")
				}
			}
		}
	}
	return nil
}

func (feeds HytaleFeeds) removeAllSubscriptions(targetID string) {
	for feedID := range feeds.Feeds {
		feeds.db.RemoveSubscription(feedID, targetID)
	}
}

func roleMentions(roleIDs []string) string {
	var mentions []string
	for _, id := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", id))
	}
	return strings.Join(mentions, " ")
}
