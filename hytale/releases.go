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
