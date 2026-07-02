package hytale

import (
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
	MavenReleaseFeedType
	MavenPreReleaseFeedType
	LauncherReleaseFeedType
	LauncherPostFeedType
	PatchlinesFeedType
)

var (
	feedTypes = []FeedType{
		GameReleaseFeedType,
		GamePreReleaseFeedType,
		MavenReleaseFeedType,
		MavenPreReleaseFeedType,
		LauncherReleaseFeedType,
		LauncherPostFeedType,
		PatchlinesFeedType,
	}
)

func ParseFeedType(feedType string) (FeedType, error) {
	switch feedType {
	case GameReleaseFeedType.ID():
		return GameReleaseFeedType, nil
	case GamePreReleaseFeedType.ID():
		return GamePreReleaseFeedType, nil
	case MavenReleaseFeedType.ID():
		return MavenReleaseFeedType, nil
	case MavenPreReleaseFeedType.ID():
		return MavenPreReleaseFeedType, nil
	case LauncherReleaseFeedType.ID():
		return LauncherReleaseFeedType, nil
	case LauncherPostFeedType.ID():
		return LauncherPostFeedType, nil
	case PatchlinesFeedType.ID():
		return PatchlinesFeedType, nil
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
	case MavenReleaseFeedType:
		return "maven_release"
	case MavenPreReleaseFeedType:
		return "maven_pre_release"
	case LauncherReleaseFeedType:
		return "launcher_release"
	case LauncherPostFeedType:
		return "launcher_post"
	case PatchlinesFeedType:
		return "patchlines"
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

func (ft FeedType) Display() string {
	switch ft {
	case GameReleaseFeedType:
		return "New Client Releases"
	case GamePreReleaseFeedType:
		return "New Client Pre-releases"
	case MavenReleaseFeedType:
		return "New Server Releases"
	case MavenPreReleaseFeedType:
		return "New Server Pre-releases"
	case LauncherReleaseFeedType:
		return "New Launcher Versions"
	case LauncherPostFeedType:
		return "New Launcher Articles"
	case PatchlinesFeedType:
		return "Added/Removed Patchlines"
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

// Gets the feed currently stored in the database
// May return nil!
func (ft FeedType) getStored(db *db.DB, previous bool) (Feed, error) {
	var raw []byte
	var err error
	if previous {
		raw, err = db.GetPreviousPost(ft.ID())
	} else {
		raw, err = db.GetLatestPost(ft.ID())
	}
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}

	switch ft {
	case GameReleaseFeedType:
		return deserializeGameRelease(raw, Release)
	case GamePreReleaseFeedType:
		return deserializeGameRelease(raw, PreRelease)
	case MavenReleaseFeedType:
		return deserializeMavenRelease(raw, Release)
	case MavenPreReleaseFeedType:
		return deserializeMavenRelease(raw, PreRelease)
	case LauncherReleaseFeedType:
		return deserializeLauncherRelease(raw)
	case LauncherPostFeedType:
		return deserializeArticles(raw)
	case PatchlinesFeedType:
		return deserializePatchlines(raw)
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

func (ft FeedType) fetch(feeds *HytaleFeeds) (Feed, error) {
	switch ft {
	case GameReleaseFeedType:
		return fetchGameRelease(feeds, Release)
	case GamePreReleaseFeedType:
		return fetchGameRelease(feeds, PreRelease)
	case MavenReleaseFeedType:
		return fetchMavenRelease(feeds, Release)
	case MavenPreReleaseFeedType:
		return fetchMavenRelease(feeds, PreRelease)
	case LauncherReleaseFeedType:
		return fetchLauncherRelease(feeds)
	case LauncherPostFeedType:
		return fetchArticles(feeds)
	case PatchlinesFeedType:
		return fetchPatchlines(feeds)
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

type FeedMessage struct {
	Embeds     []*discordgo.MessageEmbed
	Components []discordgo.MessageComponent
}

// Represents a feed of content
type Feed interface {
	GetType() FeedType
	// Formats the latest feed content as an embed
	BuildMessage(config *config.Config) *FeedMessage
	// Formats the latest feed content as an embed to be shown to subscribers.
	// `previous` is the feed's previous content, or nil if no previous content exists.
	BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage
	// The last version string that was sent to the subscriber
	// If the subscribed content has a new version string, then we know the subscriber should be notified
	GetVersion() string
	// Gets the feed's current raw content, as a string
	content() (string, error)
}

// Keeps all feeds up-to-date by periodically checking for new content and notifying subscribers
type HytaleFeeds struct {
	Feeds     map[FeedType]Feed
	config    *config.Config
	db        *db.DB
	http      *http.Client
	authStore auth.AuthStore
}

func NewHytaleFeeds(config *config.Config, db *db.DB, http *http.Client, authStore auth.AuthStore) (*HytaleFeeds, error) {
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

	expectedFeeds := len(feedTypes)
	if len(feeds.Feeds) < expectedFeeds {
		log.Println("Feeds have not been stored yet, fetching...")
		feeds.Poll()
		if len(feeds.Feeds) < expectedFeeds {
			return nil, errors.New("feed state was not initialized")
		}
	}

	return feeds, nil
}

func (feeds *HytaleFeeds) initializeFeeds() error {
	for _, feedType := range feedTypes {
		feed, err := feedType.getStored(feeds.db, false)
		if err != nil {
			return err
		}
		if feed != nil {
			feeds.Feeds[feedType] = feed
		}
	}
	return nil
}

func (feeds *HytaleFeeds) Poll() {
	for _, feedType := range feedTypes {
		newFeed, err := feedType.fetch(feeds)
		if err != nil {
			log.Printf("Error fetching feed %s: %v", feedType.ID(), err)
			continue
		}
		content, _ := newFeed.content()
		err = feeds.db.SetLatestPost(feedType.ID(), content)
		if err != nil {
			log.Printf("Error setting latest post for feed %s: %v", feedType.ID(), err)
			continue
		}
		feeds.updateOrAddFeed(newFeed)
	}
}

func (feeds *HytaleFeeds) updateOrAddFeed(newFeed Feed) {
	feedType := newFeed.GetType()
	if oldFeed, exists := feeds.Feeds[feedType]; exists {
		content, _ := oldFeed.content()
		err := feeds.db.SetPreviousPost(feedType.ID(), content)
		if err != nil {
			log.Printf("Error setting previous post for feed %s: %v", feedType.ID(), err)
		}
	}
	feeds.Feeds[feedType] = newFeed
}

// Notifies any subscribers if they have not received the latest content
func (feeds HytaleFeeds) NotifyFeeds(s *discordgo.Session) {
	for feedType, feed := range feeds.Feeds {
		targetIDs, err := feeds.db.GetSubscriptions(feedType.ID())
		if err != nil {
			log.Printf("Error getting target IDs for feed %s: %v", feedType.ID(), err)
			continue
		}
		for _, targetID := range targetIDs {
			feeds.notify(s, feed, targetID)
		}
	}
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
				previous, err := feed.GetType().getStored(feeds.db, true)
				if err != nil {
					log.Printf("Cannot get previous feed: %v", err)
				}

				message := feed.BuildSubscriberMessage(feeds.config, previous)
				_, err = s.ChannelMessageSendComplex(targetID, &discordgo.MessageSend{
					Content:    roleMentions(sub.Roles),
					Embeds:     message.Embeds,
					Components: message.Components,
					AllowedMentions: &discordgo.MessageAllowedMentions{
						Roles: sub.Roles,
					},
				})
				if err != nil {
					log.Printf("Cannot send feed update (guild): %v", err)
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

				previous, err := feed.GetType().getStored(feeds.db, true)
				if err != nil {
					log.Printf("Cannot get previous feed: %v", err)
				}

				message := feed.BuildSubscriberMessage(feeds.config, previous)
				_, err = s.ChannelMessageSendComplex(dm.ID, &discordgo.MessageSend{
					Embeds:          message.Embeds,
					Components:      message.Components,
					AllowedMentions: &discordgo.MessageAllowedMentions{},
				})
				if err != nil {
					log.Printf("Cannot send feed update (user): %v", err)
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
