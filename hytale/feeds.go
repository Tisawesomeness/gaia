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

type FeedType int

const (
	GameReleaseFeedType FeedType = iota
	GamePreReleaseFeedType
	LauncherReleaseFeedType
	LauncherPostFeedType
	expectedFeeds = 4
)

func ParseFeedType(feedType string) (FeedType, error) {
	switch feedType {
	case GameReleaseFeedType.ID():
		return GameReleaseFeedType, nil
	case GamePreReleaseFeedType.ID():
		return GamePreReleaseFeedType, nil
	case LauncherReleaseFeedType.ID():
		return LauncherReleaseFeedType, nil
	case LauncherPostFeedType.ID():
		return LauncherPostFeedType, nil
	default:
		return 0, fmt.Errorf("unknown feedType: %s", feedType)
	}
}

func (ft FeedType) ID() string {
	switch ft {
	case GameReleaseFeedType:
		return "game_release"
	case GamePreReleaseFeedType:
		return "game_pre_release"
	case LauncherReleaseFeedType:
		return "launcher_release"
	case LauncherPostFeedType:
		return "launcher_post"
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

func (ft FeedType) Display() string {
	switch ft {
	case GameReleaseFeedType:
		return "New Hytale Releases"
	case GamePreReleaseFeedType:
		return "New Hytale Pre-releases"
	case LauncherReleaseFeedType:
		return "New Launcher Versions"
	case LauncherPostFeedType:
		return "Launcher Articles"
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

// A Hytale release branch (release, pre-release, etc)
type Patchline int

const (
	Release Patchline = iota
	PreRelease
)

var (
	patchlines = []Patchline{Release, PreRelease}
)

func ParsePatchline(patchline string) (Patchline, error) {
	switch patchline {
	case Release.ID():
		return Release, nil
	case PreRelease.ID():
		return PreRelease, nil
	default:
		return 0, fmt.Errorf("unknown patchline: %s", patchline)
	}
}

func (p Patchline) ID() string {
	switch p {
	case Release:
		return "release"
	case PreRelease:
		return "pre-release"
	default:
		panic(fmt.Errorf("unknown state: %d", p))
	}
}

func (p Patchline) Display() string {
	switch p {
	case Release:
		return "Release"
	case PreRelease:
		return "Pre-release"
	default:
		panic(fmt.Errorf("unknown state: %d", p))
	}
}

func (p Patchline) FeedType() FeedType {
	switch p {
	case Release:
		return GameReleaseFeedType
	case PreRelease:
		return GamePreReleaseFeedType
	default:
		panic(fmt.Errorf("unknown state: %d", p))
	}
}

// Represents a feed of content
type Feed interface {
	GetType() FeedType
	// Formats the latest feed content as an embed
	BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed
	// The last version string that was sent to the subscriber
	// If the subscribed content has a new version string, then we know the subscriber should be notified
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

func (f *GameReleaseFeed) GetType() FeedType {
	return f.Patchline.FeedType()
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
	raw, err := db.GetLatestPost(patchline.FeedType().ID())
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

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s.json", feeds.config.Feeds.GameVersion, patchline.ID()), nil)
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
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s version", patchline.ID()), resp)
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

func (f *LauncherReleaseFeed) GetType() FeedType {
	return LauncherPostFeedType
}

func (f *LauncherReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
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
	raw, err := db.GetLatestPost(LauncherReleaseFeedType.ID())
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

func (f *LauncherPostFeed) GetType() FeedType {
	return LauncherPostFeedType
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
	raw, err := db.GetLatestPost(LauncherPostFeedType.ID())
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

// Feed Management

// Keeps all feeds up-to-date by periodically checking for new content and notifying subscribers
type HytaleFeeds struct {
	Feeds     map[FeedType]Feed
	config    *config.Config
	db        *db.DB
	http      *http.Client
	authStore *auth.AuthStore
}

func NewHytaleFeeds(config *config.Config, db *db.DB, http *http.Client, authStore *auth.AuthStore) (*HytaleFeeds, error) {
	feeds := &HytaleFeeds{
		Feeds:     make(map[FeedType]Feed),
		config:    config,
		db:        db,
		http:      http,
		authStore: authStore,
	}

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
	release, err := getStoredLauncherRelease(feeds.db)
	if err != nil {
		return err
	}
	if release != nil {
		feeds.Feeds[LauncherReleaseFeedType] = &LauncherReleaseFeed{Release: release}
	}

	articles, err := getStoredArticles(feeds.db)
	if err != nil {
		return err
	}
	if articles != nil {
		feeds.Feeds[LauncherPostFeedType] = &LauncherPostFeed{Articles: articles}
	}

	for _, patchline := range patchlines {
		gameRelease, err := getStoredGameRelease(patchline, feeds.db)
		if err != nil {
			return err
		}
		if gameRelease != nil {
			feeds.Feeds[patchline.FeedType()] = &GameReleaseFeed{
				Version:   gameRelease,
				Patchline: patchline,
			}
		}
	}

	return nil
}

// Fetches new content for all feeds
func (feeds *HytaleFeeds) Poll() error {
	release, err := feeds.fetchLauncherRelease()
	if err != nil {
		return err
	}
	releaseStr, _ := json.Marshal(release)
	err = feeds.db.SetLatestPost(LauncherReleaseFeedType.ID(), string(releaseStr))
	if err != nil {
		return err
	}
	feeds.updateOrAddFeed(&LauncherReleaseFeed{Release: release})

	articles, err := feeds.fetchArticles()
	if err != nil {
		return err
	}
	articlesStr, _ := json.Marshal(articles)
	err = feeds.db.SetLatestPost(LauncherPostFeedType.ID(), string(articlesStr))
	if err != nil {
		return err
	}
	feeds.updateOrAddFeed(&LauncherPostFeed{Articles: articles})

	for _, patchline := range patchlines {
		gameRelease, err := feeds.fetchGameRelease(patchline)
		if err != nil {
			return err
		}
		gameReleaseStr, _ := json.Marshal(gameRelease)
		err = feeds.db.SetLatestPost(patchline.FeedType().ID(), string(gameReleaseStr))
		if err != nil {
			return err
		}
		feeds.updateOrAddFeed(&GameReleaseFeed{
			Version:   gameRelease,
			Patchline: patchline,
		})
	}

	return nil
}

func (feeds *HytaleFeeds) updateOrAddFeed(newFeed Feed) {
	feeds.Feeds[newFeed.GetType()] = newFeed
}

// Notifies any subscribers if they have not received the latest content
func (feeds HytaleFeeds) NotifyFeeds(s *discordgo.Session) error {
	for feedType, feed := range feeds.Feeds {
		targetIDs, err := feeds.db.GetSubscriptions(feedType.ID())
		if err != nil {
			return err
		}
		for _, targetID := range targetIDs {
			feeds.notify(s, feed, targetID)
		}
	}
	return nil
}

func (feeds HytaleFeeds) notify(s *discordgo.Session, feed Feed, targetID string) {
	sub, err := feeds.db.GetSubscription(feed.GetType().ID(), targetID)
	if err != nil {
		log.Printf("Error getting subscription from db: %v", err)
		return
	}

	// Instead of comparing entire content (formatting can change), compare just the version
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
					return
				}

				feeds.db.AddOrUpdateSubscription(feed.GetType().ID(), targetID, db.GuildSubscription{
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
					return
				}

				message := feed.BuildMessage(feeds.config, true)
				_, err = s.ChannelMessageSendComplex(dm.ID, &discordgo.MessageSend{
					Embeds:          []*discordgo.MessageEmbed{message},
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				})
				if err != nil {
					log.Printf("Cannot send feed update: %v", err)
					return
				}

				feeds.db.AddOrUpdateSubscription(feed.GetType().ID(), targetID, db.UserSubscription{
					Version: feed.GetVersion(),
				})
			}

		default:
			panic("Invalid subscription type")
		}
	}
}

func (feeds HytaleFeeds) removeAllSubscriptions(targetID string) {
	for feedType := range feeds.Feeds {
		feeds.db.RemoveSubscription(feedType.ID(), targetID)
	}
}

func roleMentions(roleIDs []string) string {
	var mentions []string
	for _, id := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", id))
	}
	return strings.Join(mentions, " ")
}
