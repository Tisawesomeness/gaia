package hytale

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

// A Hytale release branch (release, pre-release, etc)
type Patchline int

const (
	Release Patchline = iota
	PreRelease
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

// Patchline ID is the ID in Hytale's format (with dashes instead of underscores).
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

type GameReleaseVersion struct {
	Version string `json:"version"`
}

type gameReleaseResponse struct {
	Url string `json:"url"`
}

type GameReleaseFeed struct {
	Version   *GameReleaseVersion
	Patchline Patchline
}

func (f GameReleaseFeed) GetType() FeedType {
	return f.Patchline.FeedType()
}

func (f GameReleaseFeed) BuildMessage(config *config.Config, isNews bool) *discordgo.MessageEmbed {
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

func (f GameReleaseFeed) GetVersion() string {
	if f.Version == nil {
		return ""
	}
	return f.Version.Version
}

func (f GameReleaseFeed) content() (string, error) {
	contentBytes, err := json.Marshal(f.Version)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func getStoredGameRelease(patchline Patchline, db *db.DB) (Feed, error) {
	raw, err := db.GetLatestPost(patchline.FeedType().ID())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var release GameReleaseVersion
	err = json.Unmarshal(raw, &release)
	return GameReleaseFeed{
		Version:   &release,
		Patchline: patchline,
	}, err
}

func (feed GameReleaseFeed) fetchGameReleaseUrl(feeds *HytaleFeeds) (string, error) {
	token, err := feeds.authStore.GetOAuthToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s.json", feeds.config.Feeds.GameVersion, feed.Patchline.ID()), nil)
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

func (feed GameReleaseFeed) fetch(feeds *HytaleFeeds) (Feed, error) {
	url, err := feed.fetchGameReleaseUrl(feeds)
	if err != nil {
		return nil, err
	}

	resp, err := feeds.http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s version", feed.Patchline.ID()), resp)
	}

	var release GameReleaseVersion
	err = json.NewDecoder(resp.Body).Decode(&release)
	return GameReleaseFeed{
		Version:   &release,
		Patchline: feed.Patchline,
	}, err
}
