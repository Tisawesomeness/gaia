package hytale

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Tisawesomeness/gaia/config"
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
	return LauncherReleaseFeedType
}

func (f LauncherReleaseFeed) BuildMessage(config *config.Config) *FeedMessage {
	return f.buildMessage(config, false)
}
func (f LauncherReleaseFeed) BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage {
	return f.buildMessage(config, true)
}

func (f LauncherReleaseFeed) buildMessage(config *config.Config, isNews bool) *FeedMessage {
	var title string
	if isNews {
		title = "New Hytale Launcher Version"
	} else {
		title = "Latest Hytale Launcher Version"
	}
	embed := &discordgo.MessageEmbed{
		Title:       title,
		Description: fmt.Sprintf("`%s`", f.GetVersion()),
		Color:       0x0000FF,
	}

	if f.Release != nil && f.Release.DownloadURLs != nil {
		fields := []*discordgo.MessageEmbedField{}
		for platform, urls := range f.Release.DownloadURLs {
			for arch, downloadURL := range urls {
				platformName := util.CapitalizeFirstLetter(platform)
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

	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{embed},
	}
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

func deserializeLauncherRelease(data []byte) (Feed, error) {
	var release LauncherRelease
	err := json.Unmarshal(data, &release)
	return LauncherReleaseFeed{Release: &release}, err
}

func fetchLauncherRelease(feeds *HytaleFeeds) (Feed, error) {
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

func (a *Article) BuildMessage(config *config.Config) *FeedMessage {
	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       a.Title,
				URL:         a.DestURL,
				Description: a.Description,
				Image:       &discordgo.MessageEmbedImage{URL: config.Feeds.ArticleImagePrefix + a.ImageURL},
				Color:       0x0000FF,
			},
		},
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

func (f LauncherPostFeed) BuildMessage(config *config.Config) *FeedMessage {
	return f.BuildSubscriberMessage(config, nil)
}
func (f LauncherPostFeed) BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage {
	if len(f.Articles.Articles) <= 0 {
		return &FeedMessage{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       "Hytale Articles",
					Description: "No articles yet...",
					Color:       0xFF0000,
				},
			},
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

func deserializeArticles(data []byte) (Feed, error) {
	var articles ArticleList
	err := json.Unmarshal(data, &articles)
	return LauncherPostFeed{Articles: &articles}, err
}

func fetchArticles(feeds *HytaleFeeds) (Feed, error) {
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
