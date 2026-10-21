package hytale

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

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

type Side int

const (
	Client Side = iota
	Server
)

func GetFeedType(patchline Patchline, side Side) FeedType {
	switch patchline {
	case Release:
		switch side {
		case Client:
			return GameReleaseFeedType
		case Server:
			return MavenReleaseFeedType
		}
	case PreRelease:
		switch side {
		case Client:
			return GamePreReleaseFeedType
		case Server:
			return MavenPreReleaseFeedType
		}
	}
	panic(fmt.Errorf("unknown patchline %d or side %d", patchline, side))
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
	return GetFeedType(f.Patchline, Client)
}

func (f GameReleaseFeed) BuildMessage(config *config.Config, isNews bool) *FeedMessage {
	var adjective string
	if isNews {
		adjective = "New"
	} else {
		adjective = "Latest"
	}
	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       fmt.Sprintf("%s Hytale Client %s", adjective, f.Patchline.Display()),
				Description: fmt.Sprintf("`%s`", f.GetVersion()),
				Color:       0x0000FF,
			},
		},
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
	raw, err := db.GetLatestPost(GetFeedType(patchline, Client).ID())
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

func fetchGameReleaseUrl(patchline Patchline, feeds *HytaleFeeds) (string, error) {
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

func fetchGameRelease(patchline Patchline, feeds *HytaleFeeds) (Feed, error) {
	url, err := fetchGameReleaseUrl(patchline, feeds)
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
	return GameReleaseFeed{
		Version:   &release,
		Patchline: patchline,
	}, err
}

type MavenVersion struct {
	Version string `xml:",chardata"`
}

type MavenVersioning struct {
	XMLName     xml.Name       `xml:"versioning"`
	Latest      string         `xml:"latest"`
	Versions    []MavenVersion `xml:"versions>version"`
	LastUpdated string         `xml:"lastUpdated"`
}

type mavenResponse struct {
	XMLName    xml.Name        `xml:"metadata"`
	Versioning MavenVersioning `xml:"versioning"`
}

type MavenFeed struct {
	Version   *MavenVersioning
	Patchline Patchline
}

func (f MavenFeed) GetType() FeedType {
	return GetFeedType(f.Patchline, Server)
}

func downloadUrl(config *config.Config, version string, patchline Patchline) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s-%s.jar",
		config.Feeds.MavenRepo,
		patchline.ID(),
		strings.ReplaceAll(config.Feeds.MavenGroup, ".", "/"),
		config.Feeds.MavenArtifact,
		version,
		config.Feeds.MavenArtifact,
		version,
	)
}

func (f MavenFeed) BuildMessage(config *config.Config, isNews bool) *FeedMessage {
	// If announcing a new release, only need to include the new release in the embed
	if isNews {
		url := downloadUrl(config, f.Version.Latest, f.Patchline)

		return &FeedMessage{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       fmt.Sprintf("New Hytale Server %s", f.Patchline.Display()),
					Description: fmt.Sprintf("`%s`", f.Version.Latest),
					Color:       0x0000FF,
				},
			},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.Button{
							Label: "Download",
							Style: discordgo.LinkButton,
							URL:   url,
						},
					},
				},
			},
		}
	}

	// Slice goes from oldest to newest, iterate in reverse so latest is at the top
	descriptionLines := []string{}
	for i := len(f.Version.Versions) - 1; i >= 0; i-- {
		version := f.Version.Versions[i]
		line := fmt.Sprintf("- `%s`- [Download](%s)", version.Version, downloadUrl(config, version.Version, f.Patchline))
		descriptionLines = append(descriptionLines, line)
	}
	description := strings.Join(descriptionLines, "\n")

	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       fmt.Sprintf("Hytale Server %ss", f.Patchline.Display()),
				Description: description,
				Color:       0x0000FF,
			},
		},
	}
}

func (f MavenFeed) GetVersion() string {
	if f.Version == nil {
		return ""
	}
	return f.Version.Latest
}

func (f MavenFeed) content() (string, error) {
	contentBytes, err := xml.Marshal(f.Version)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func getStoredMavenRelease(patchline Patchline, db *db.DB) (Feed, error) {
	raw, err := db.GetLatestPost(GetFeedType(patchline, Server).ID())
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	var versioning MavenVersioning
	err = xml.Unmarshal(raw, &versioning)
	return MavenFeed{
		Version:   &versioning,
		Patchline: patchline,
	}, err
}

func fetchMavenRelease(patchline Patchline, feeds *HytaleFeeds) (Feed, error) {
	config := feeds.config.Feeds
	metadataUrl := fmt.Sprintf("%s/%s/%s/%s/maven-metadata.xml",
		config.MavenRepo,
		patchline.ID(),
		strings.ReplaceAll(config.MavenGroup, ".", "/"),
		config.MavenArtifact,
	)

	resp, err := feeds.http.Get(metadataUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s maven version", patchline.ID()), resp)
	}

	var mavenResp mavenResponse
	err = xml.NewDecoder(resp.Body).Decode(&mavenResp)
	return MavenFeed{
		Version:   &mavenResp.Versioning,
		Patchline: patchline,
	}, err
}
