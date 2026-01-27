package hytale

import (
	"encoding/json"
	"fmt"
	"net/http"
	"unicode"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

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

func (f LauncherReleaseFeed) GetType() FeedType {
	return LauncherPostFeedType
}

func (f LauncherReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
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

func (f LauncherReleaseFeed) GetVersion() string {
	return f.Release.Version
}

func (f LauncherReleaseFeed) content() (string, error) {
	contentBytes, err := json.Marshal(f.Release)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func getStoredLauncherRelease(db *db.DB) (Feed, error) {
	raw, err := db.GetLatestPost(LauncherReleaseFeedType.ID())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var release LauncherRelease
	err = json.Unmarshal(raw, &release)
	return LauncherReleaseFeed{Release: &release}, err
}

func (LauncherReleaseFeed) fetch(feeds *HytaleFeeds) (Feed, error) {
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
	return LauncherReleaseFeed{Release: &release}, err
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

type ArticleList struct {
	Articles []*Article `json:"articles"`
}

type LauncherPostFeed struct {
	Articles *ArticleList
}

func (f LauncherPostFeed) GetType() FeedType {
	return LauncherPostFeedType
}

func (f LauncherPostFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
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

func (f LauncherPostFeed) GetVersion() string {
	if len(f.Articles.Articles) <= 0 {
		return ""
	}
	return f.Articles.Articles[0].DestURL
}

func (f LauncherPostFeed) content() (string, error) {
	contentBytes, err := json.Marshal(f.Articles)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func getStoredArticles(db *db.DB) (Feed, error) {
	raw, err := db.GetLatestPost(LauncherPostFeedType.ID())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var articles ArticleList
	err = json.Unmarshal(raw, &articles)
	return LauncherPostFeed{Articles: &articles}, err
}

func (LauncherPostFeed) fetch(feeds *HytaleFeeds) (Feed, error) {
	resp, err := feeds.http.Get(feeds.config.Feeds.LauncherArticles)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Fetch articles", resp)
	}

	var articles ArticleList
	err = json.NewDecoder(resp.Body).Decode(&articles)
	return LauncherPostFeed{Articles: &articles}, err
}
