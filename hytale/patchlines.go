package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

// Fetches all patchlines from the Hytale API, mapped to their expiration time (or nil if no expiry).
func GetPatchlines(config *config.Config, httpClient *http.Client, authStore auth.AuthStore) (map[string]*time.Time, error) {
	token, err := authStore.GetOAuthToken()
	if err != nil {
		return nil, err
	}

	if token.AuthType == auth.Server {
		patchlines, err := getPatchlines(config, httpClient, token.Token)
		if err != nil {
			return nil, err
		}
		result := make(map[string]*time.Time, len(patchlines))
		for _, p := range patchlines {
			var t *time.Time
			if p.ExpiresAt > 0 {
				parsed := time.Unix(p.ExpiresAt, 0)
				t = &parsed
			}
			result[p.Name] = t
		}
		return result, nil

	} else {
		data, err := auth.GetLauncherData(config, httpClient, token.Token)
		if err != nil {
			return nil, err
		}
		result := make(map[string]*time.Time, len(data.PatchLines))
		for name, build := range data.PatchLines {
			var t *time.Time
			if build.ExpiresAt > 0 {
				parsed := time.Unix(build.ExpiresAt, 0)
				t = &parsed
			}
			result[name] = t
		}
		return result, nil
	}
}

type patchlineResponses struct {
	Patchlines []*patchlineResponse `json:"patchlines"`
}

type patchlineResponse struct {
	Name      string `json:"name"`
	ExpiresAt int64  `json:"expiresAt"` // unix timestamp
}

// Server auth only!
func getPatchlines(config *config.Config, httpClient *http.Client, oauthAccessToken string) ([]*patchlineResponse, error) {
	if oauthAccessToken == "" {
		return nil, errors.New("Oauth access token is required")
	}

	req, err := http.NewRequest("GET", config.Feeds.Patchlines, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+oauthAccessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, util.NewBadResponseError("Get launcher data", resp)
	}

	var data patchlineResponses
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, fmt.Errorf("Error reading launcher data body: %w", err)
	}

	return data.Patchlines, nil
}

type PatchlinesFeed struct {
	Patchlines map[string]*time.Time
}

func (f PatchlinesFeed) GetType() FeedType {
	return PatchlinesFeedType
}

func (f PatchlinesFeed) BuildMessage(config *config.Config) *FeedMessage {
	return f.BuildSubscriberMessage(config, nil)
}

func (f PatchlinesFeed) BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage {
	var previousPatchlines map[string]*time.Time
	previousFeed, ok := previous.(PatchlinesFeed)
	if ok {
		previousPatchlines = previousFeed.Patchlines
	}

	added := make(map[string]bool)
	removed := make(map[string]bool)
	currentPatchlines := f.Patchlines
	if previousPatchlines != nil {
		for name := range currentPatchlines {
			if _, exists := previousPatchlines[name]; !exists {
				added[name] = true
			}
		}
		for name := range previousPatchlines {
			if _, exists := currentPatchlines[name]; !exists {
				removed[name] = true
			}
		}
	}

	var description string
	var fields []*discordgo.MessageEmbedField
	patchlineCount := len(currentPatchlines) + len(removed)
	if patchlineCount == 0 {
		description = "(no patchlines found)"

	} else if patchlineCount < 10 {
		for id, expiry := range currentPatchlines {
			var name string
			if added[id] {
				name = fmt.Sprintf("`%s` (**NEW**)", id)
			} else {
				name = fmt.Sprintf("`%s`", id)
			}
			var value string
			if expiry != nil {
				value = fmt.Sprintf("Expires <t:%d:R>", expiry.Unix())
			} else {
				value = "(no expiry)"
			}
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   name,
				Value:  value,
				Inline: false,
			})
		}
		for id := range removed {
			fields = append(fields, &discordgo.MessageEmbedField{
				Name:   fmt.Sprintf("`%s`", id),
				Value:  "(removed)",
				Inline: false,
			})
		}

	} else {
		var lines []string
		for id, expiry := range currentPatchlines {
			if added[id] {
				if expiry != nil {
					lines = append(lines, fmt.Sprintf("`%s` (**NEW**): Expires <t:%d:R>", id, expiry.Unix()))
				} else {
					lines = append(lines, fmt.Sprintf("`%s` (**NEW**): (no expiry)", id))
				}
			} else {
				if expiry != nil {
					lines = append(lines, fmt.Sprintf("`%s`: Expires <t:%d:R>", id, expiry.Unix()))
				} else {
					lines = append(lines, fmt.Sprintf("`%s`: (no expiry)", id))
				}
			}
		}
		for id := range removed {
			if _, exists := currentPatchlines[id]; !exists {
				lines = append(lines, fmt.Sprintf("`%s`: (removed)", id))
			}
		}
		description = strings.Join(lines, "\n")
	}

	return &FeedMessage{
		Embeds: []*discordgo.MessageEmbed{
			{
				Title:       "Hytale Patchlines",
				Description: description,
				Color:       0x0000FF,
				Fields:      fields,
			},
		},
		Components: []discordgo.MessageComponent{},
	}
}

func (f PatchlinesFeed) GetVersion() string {
	var lines []string
	for name, expiry := range f.Patchlines {
		if expiry != nil {
			lines = append(lines, fmt.Sprintf("%s:%d", name, expiry.Unix()))
		} else {
			lines = append(lines, fmt.Sprintf("%s:%d", name, expiry.Unix()))
		}
	}
	return strings.Join(lines, ";")
}

func (f PatchlinesFeed) content() (string, error) {
	contentBytes, err := json.Marshal(f.Patchlines)
	if err != nil {
		return "", err
	}
	return string(contentBytes), nil
}

func deserializePatchlines(data []byte) (Feed, error) {
	var patchlines map[string]*time.Time
	if err := json.Unmarshal(data, &patchlines); err != nil {
		return nil, err
	}
	return PatchlinesFeed{Patchlines: patchlines}, nil
}

func fetchPatchlines(feeds *HytaleFeeds) (Feed, error) {
	patchlines, err := GetPatchlines(feeds.config, feeds.http, feeds.authStore)
	if err != nil {
		return nil, err
	}
	return PatchlinesFeed{Patchlines: patchlines}, nil
}
