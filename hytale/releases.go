package hytale

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

type Side int

const (
	Client Side = iota
	Server
)

type GameReleaseVersion struct {
	Version string `json:"version"`
}

type gameReleaseResponse struct {
	Url string `json:"url"`
}

type GameFeed struct {
	Version   *GameReleaseVersion
	Patchline string
}

func (f GameFeed) GetID() string {
	return GameFeedType.ID() + "_" + f.Patchline
}

func (f GameFeed) GetType() FeedType {
	return GameFeedType
}

func (f GameFeed) GetDisplay() string {
	if f.Patchline == "release" {
		return GameFeedType.Display()
	}
	return fmt.Sprintf("%s (`%s`)", GameFeedType.Display(), f.Patchline)
}

func (f GameFeed) BuildMessage(config *config.Config) *FeedMessage {
	return f.buildMessage(false)
}
func (f GameFeed) BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage {
	return f.buildMessage(true)
}

func (f GameFeed) buildMessage(isNews bool) *FeedMessage {
	var adjective string
	if isNews {
		adjective = "New"
	} else {
		adjective = "Latest"
	}
	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       fmt.Sprintf("%s Hytale Client %s", adjective, DisplayPatchline(f.Patchline)),
				Description: fmt.Sprintf("`%s`", f.GetVersion()),
				Color:       0x0000FF,
			},
		},
	}
}

func (f GameFeed) GetVersion() string {
	if f.Version == nil {
		return ""
	}
	return f.Version.Version
}

func (f GameFeed) content() (string, error) {
	contentBytes, err := json.Marshal(f.Version)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func deserializeGame(data []byte, patchline string) (*GameFeed, error) {
	var release GameReleaseVersion
	err := json.Unmarshal(data, &release)
	return &GameFeed{
		Version:   &release,
		Patchline: patchline,
	}, err
}

func fetchGameReleaseUrl(config *config.Config, httpClient *http.Client, authStore auth.AuthStore, patchline string) (string, error) {
	token, err := authStore.GetOAuthToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("%s%s.json", config.Feeds.GameVersion, patchline), nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token.Token))

	resp, err := httpClient.Do(req)
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

func fetchGame(config *config.Config, httpClient *http.Client, authStore auth.AuthStore, patchline string) (*GameFeed, error) {
	url, err := fetchGameReleaseUrl(config, httpClient, authStore, patchline)
	if err != nil {
		return nil, err
	}

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s version", patchline), resp)
	}

	var release GameReleaseVersion
	err = json.NewDecoder(resp.Body).Decode(&release)
	return &GameFeed{
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
	Patchline string
}

func (f MavenFeed) GetID() string {
	return MavenFeedType.ID() + "_" + f.Patchline
}

func (f MavenFeed) GetType() FeedType {
	return MavenFeedType
}

func (f MavenFeed) GetDisplay() string {
	if f.Patchline == "release" {
		return MavenFeedType.Display()
	}
	return fmt.Sprintf("%s (`%s`)", MavenFeedType.Display(), f.Patchline)
}

func downloadUrl(config *config.Config, version string, patchline string) string {
	return fmt.Sprintf("%s/%s/%s/%s/%s/%s-%s.jar",
		config.Feeds.MavenRepo,
		patchline,
		strings.ReplaceAll(config.Feeds.MavenGroup, ".", "/"),
		config.Feeds.MavenArtifact,
		version,
		config.Feeds.MavenArtifact,
		version,
	)
}

func (f MavenFeed) BuildMessage(config *config.Config) *FeedMessage {
	return f.buildMessage(config, false)
}
func (f MavenFeed) BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage {
	return f.buildMessage(config, true)
}

func (f MavenFeed) buildMessage(config *config.Config, isNews bool) *FeedMessage {
	// If announcing a new release, only need to include the new release in the embed
	if isNews {
		url := downloadUrl(config, f.Version.Latest, f.Patchline)

		return &FeedMessage{
			Embeds: []*discordgo.MessageEmbed{
				{
					Title:       fmt.Sprintf("New Hytale Server %s", DisplayPatchline(f.Patchline)),
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
				Title:       fmt.Sprintf("Hytale Server %ss", DisplayPatchline(f.Patchline)),
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

func deserializeMaven(data []byte, patchline string) (*MavenFeed, error) {
	var versioning MavenVersioning
	err := xml.Unmarshal(data, &versioning)
	return &MavenFeed{
		Version:   &versioning,
		Patchline: patchline,
	}, err
}

func MavenMetadataUrl(config config.FeedsConfig, patchline string) string {
	return fmt.Sprintf("%s/%s/%s/%s/maven-metadata.xml",
		config.MavenRepo,
		patchline,
		strings.ReplaceAll(config.MavenGroup, ".", "/"),
		config.MavenArtifact,
	)
}

func fetchMaven(config *config.Config, httpClient *http.Client, patchline string) (*MavenFeed, error) {
	metadataUrl := MavenMetadataUrl(config.Feeds, patchline)

	resp, err := httpClient.Get(metadataUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError(fmt.Sprintf("Fetch %s maven version", patchline), resp)
	}

	var mavenResp mavenResponse
	err = xml.NewDecoder(resp.Body).Decode(&mavenResp)
	return &MavenFeed{
		Version:   &mavenResp.Versioning,
		Patchline: patchline,
	}, err
}
