package hytale

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
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
