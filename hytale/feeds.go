package hytale

import (
	"fmt"
	"iter"
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
	PatchlinesFeedType FeedType = iota
	GameFeedType
	MavenFeedType
	LauncherReleaseFeedType
	LauncherPostFeedType
)

func ParseFeedType(feedType string) (FeedType, error) {
	switch feedType {
	case PatchlinesFeedType.ID():
		return PatchlinesFeedType, nil
	case GameFeedType.ID():
		return GameFeedType, nil
	case MavenFeedType.ID():
		return MavenFeedType, nil
	case LauncherReleaseFeedType.ID():
		return LauncherReleaseFeedType, nil
	case LauncherPostFeedType.ID():
		return LauncherPostFeedType, nil
	default:
		return 0, fmt.Errorf("unknown feedType: %s", feedType)
	}
}

const (
	patchlinesID      = "patchlines"
	gameIDPrefix      = "game"
	mavenIDPrefix     = "maven"
	launcherReleaseID = "launcher_release"
	launcherPostID    = "launcher_post"
)

func (ft FeedType) ID() string {
	switch ft {
	case PatchlinesFeedType:
		return patchlinesID
	case GameFeedType:
		return gameIDPrefix
	case MavenFeedType:
		return mavenIDPrefix
	case LauncherReleaseFeedType:
		return launcherReleaseID
	case LauncherPostFeedType:
		return launcherPostID
	default:
		panic(fmt.Errorf("unknown state: %d", ft))
	}
}

func (ft FeedType) Display() string {
	switch ft {
	case PatchlinesFeedType:
		return "Added/Removed Patchlines"
	case GameFeedType:
		return "New Client Releases"
	case MavenFeedType:
		return "New Server Releases"
	case LauncherReleaseFeedType:
		return "New Launcher Versions"
	case LauncherPostFeedType:
		return "New Launcher Articles"
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
	// A unique ID representing this feed.
	GetID() string
	// The feed's type. Multiple feeds can share the same FeedType.
	GetType() FeedType
	// The feed's display name as shown in the subscription list.
	GetDisplay() string
	// Formats the latest feed content as an embed.
	BuildMessage(config *config.Config) *FeedMessage
	// Formats the latest feed content as an embed to be shown to subscribers.
	// `previous` is the feed's previous content, or nil if no previous content exists.
	BuildSubscriberMessage(config *config.Config, previous Feed) *FeedMessage
	// The last version string that was sent to the subscriber.
	// If the subscribed content has a new version string, then we know the subscriber should be notified.
	GetVersion() string
	// Gets the feed's current raw content, as a string.
	content() (string, error)
}

// Keeps all feeds up-to-date by periodically checking for new content and notifying subscribers
type HytaleFeeds interface {
	GetPatchlinesFeed() (*PatchlinesFeed, bool)
	GetGameFeed(patchline string) (*GameFeed, bool)
	GetMavenFeed(patchline string) (*MavenFeed, bool)
	GetLauncherReleaseFeed() (*LauncherReleaseFeed, bool)
	GetLauncherPostFeed() (*LauncherPostFeed, bool)
	Feeds() iter.Seq[Feed]

	// Fetches all feeds' latest content
	Poll()
	// Notifies any subscribers if they have not received the latest content
	NotifyFeeds(s *discordgo.Session)
	// Removes all subscriptions for the provided user/channel ID
	RemoveAllSubscriptions(targetID string)
}

type hytaleFeeds struct {
	patchlinesFeed      *PatchlinesFeed
	gameFeeds           map[string]*GameFeed
	mavenFeeds          map[string]*MavenFeed
	launcherReleaseFeed *LauncherReleaseFeed
	launcherPostFeed    *LauncherPostFeed

	config    *config.Config
	db        *db.DB
	http      *http.Client
	authStore auth.AuthStore
}

// Creates a new feeds instance, restoring from database and fetching any that couldn't be restored.
func NewHytaleFeeds(config *config.Config, db *db.DB, http *http.Client, authStore auth.AuthStore) (HytaleFeeds, error) {
	feeds := &hytaleFeeds{
		gameFeeds:  make(map[string]*GameFeed),
		mavenFeeds: make(map[string]*MavenFeed),

		config:    config,
		db:        db,
		http:      http,
		authStore: authStore,
	}

	allGood := feeds.initializeFeeds()
	if !allGood {
		log.Println("Not all feeds stored yet, fetching...")
		feeds.Poll()
	}

	return feeds, nil
}

func (feeds *hytaleFeeds) initializeFeeds() bool {
	allGood := true

	// Get patchlines first - this decides what feeds are possible later
	patchlinesFeed, err := getStored(patchlinesID, false, feeds.db, deserializePatchlines)
	if err != nil {
		log.Printf("Error getting stored %s feed: %v", patchlinesID, err)
		allGood = false
	} else {
		// Fetch if no patchlines stored in database
		if patchlinesFeed == nil {
			newPatchlinesFeed, err := pollFeed(patchlinesID, feeds.patchlinesFeed, feeds.db, func() (*PatchlinesFeed, error) {
				return fetchPatchlines(feeds.config, feeds.http, feeds.authStore)
			})
			if err != nil {
				log.Printf("%v", err)
				allGood = false
			} else {
				feeds.patchlinesFeed = newPatchlinesFeed
			}
		} else {
			feeds.patchlinesFeed = patchlinesFeed
		}

		if feeds.patchlinesFeed != nil {
			for patchline := range feeds.patchlinesFeed.Patchlines {
				normalizedPatchline := strings.ReplaceAll(patchline, "-", "_")

				gameID := gameIDPrefix + "_" + normalizedPatchline
				gameFeed, err := getStored(gameID, false, feeds.db, func(data []byte) (*GameFeed, error) {
					return deserializeGame(data, patchline)
				})
				if err != nil {
					log.Printf("Error getting stored %s feed: %v", gameID, err)
					allGood = false
				} else if gameFeed != nil {
					feeds.gameFeeds[patchline] = gameFeed
				} else {
					allGood = false
				}

				mavenID := mavenIDPrefix + "_" + normalizedPatchline
				mavenFeed, err := getStored(mavenID, false, feeds.db, func(data []byte) (*MavenFeed, error) {
					return deserializeMaven(data, patchline)
				})
				if err != nil {
					log.Printf("Error getting stored %s feed: %v", mavenID, err)
					allGood = false
				} else if mavenFeed != nil {
					feeds.mavenFeeds[patchline] = mavenFeed
				} else {
					allGood = false
				}

			}
		}
	}

	launcherReleaseFeed, err := getStored(launcherReleaseID, false, feeds.db, deserializeLauncherRelease)
	if err != nil {
		log.Printf("Error getting stored launcher release feed: %v", err)
		allGood = false
	} else if launcherReleaseFeed != nil {
		feeds.launcherReleaseFeed = launcherReleaseFeed
	} else {
		allGood = false
	}

	launcherPostFeed, err := getStored(launcherPostID, false, feeds.db, deserializeArticles)
	if err != nil {
		log.Printf("Error getting stored launcher post feed: %v", err)
		allGood = false
	} else if launcherPostFeed != nil {
		feeds.launcherPostFeed = launcherPostFeed
	} else {
		allGood = false
	}

	return allGood
}

// May return nil!
func getStored[T Feed](feedID string, previous bool, db *db.DB, deserializeFunc func([]byte) (*T, error)) (*T, error) {
	var raw []byte
	var err error
	if previous {
		raw, err = db.GetPreviousPost(feedID)
	} else {
		raw, err = db.GetLatestPost(feedID)
	}
	if err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return deserializeFunc(raw)
}

func (feeds *hytaleFeeds) GetPatchlinesFeed() (*PatchlinesFeed, bool) {
	return feeds.patchlinesFeed, feeds.patchlinesFeed != nil
}

func (feeds *hytaleFeeds) GetGameFeed(patchline string) (*GameFeed, bool) {
	feed, ok := feeds.gameFeeds[patchline]
	return feed, ok
}

func (feeds *hytaleFeeds) GetMavenFeed(patchline string) (*MavenFeed, bool) {
	feed, ok := feeds.mavenFeeds[patchline]
	return feed, ok
}

func (feeds *hytaleFeeds) GetLauncherReleaseFeed() (*LauncherReleaseFeed, bool) {
	return feeds.launcherReleaseFeed, feeds.launcherReleaseFeed != nil
}

func (feeds *hytaleFeeds) GetLauncherPostFeed() (*LauncherPostFeed, bool) {
	return feeds.launcherPostFeed, feeds.launcherPostFeed != nil
}

func (feeds *hytaleFeeds) Feeds() iter.Seq[Feed] {
	return func(yield func(Feed) bool) {
		if feeds.patchlinesFeed != nil {
			if !yield(feeds.patchlinesFeed) {
				return
			}
		}
		for _, feed := range feeds.gameFeeds {
			if !yield(feed) {
				return
			}
		}
		for _, feed := range feeds.mavenFeeds {
			if !yield(feed) {
				return
			}
		}
		if feeds.launcherReleaseFeed != nil {
			if !yield(feeds.launcherReleaseFeed) {
				return
			}
		}
		if feeds.launcherPostFeed != nil {
			if !yield(feeds.launcherPostFeed) {
				return
			}
		}
	}
}

func (feeds *hytaleFeeds) Poll() {
	patchlinesFeed, err := pollFeed(patchlinesID, feeds.patchlinesFeed, feeds.db, func() (*PatchlinesFeed, error) {
		return fetchPatchlines(feeds.config, feeds.http, feeds.authStore)
	})
	if err != nil {
		log.Printf("%v", err)
	} else {
		feeds.patchlinesFeed = patchlinesFeed
	}

	patchlinesFeed = feeds.patchlinesFeed
	if patchlinesFeed != nil {
		for patchline := range patchlinesFeed.Patchlines {
			normalizedPatchline := strings.ReplaceAll(patchline, "-", "_")

			gameID := gameIDPrefix + "_" + normalizedPatchline
			gameFeed, err := pollFeed(gameID, feeds.gameFeeds[patchline], feeds.db, func() (*GameFeed, error) {
				return fetchGame(feeds.config, feeds.http, feeds.authStore, patchline)
			})
			if err != nil {
				log.Printf("%v", err)
			} else if gameFeed != nil {
				feeds.gameFeeds[patchline] = gameFeed
			}

			mavenID := mavenIDPrefix + "_" + normalizedPatchline
			mavenFeed, err := pollFeed(mavenID, feeds.mavenFeeds[patchline], feeds.db, func() (*MavenFeed, error) {
				return fetchMaven(feeds.config, feeds.http, patchline)
			})
			if err != nil {
				log.Printf("%v", err)
			} else if mavenFeed != nil {
				feeds.mavenFeeds[patchline] = mavenFeed
			}

		}
	}

	launcherReleaseFeed, err := pollFeed(launcherReleaseID, feeds.launcherReleaseFeed, feeds.db, func() (*LauncherReleaseFeed, error) {
		return fetchLauncherRelease(feeds.config, feeds.http)
	})
	if err != nil {
		log.Printf("%v", err)
	} else if launcherReleaseFeed != nil {
		feeds.launcherReleaseFeed = launcherReleaseFeed
	}

	launcherPostFeed, err := pollFeed(launcherPostID, feeds.launcherPostFeed, feeds.db, func() (*LauncherPostFeed, error) {
		return fetchArticles(feeds.config, feeds.http)
	})
	if err != nil {
		log.Printf("%v", err)
	} else if launcherPostFeed != nil {
		feeds.launcherPostFeed = launcherPostFeed
	}
}

func pollFeed[T Feed](feedID string, oldFeed *T, db *db.DB, fetchFunc func() (*T, error)) (*T, error) {
	newFeed, err := fetchFunc()
	if err != nil {
		return nil, fmt.Errorf("Error fetching feed %s: %v", feedID, err)
	}
	content, err := (*newFeed).content()
	if err != nil {
		return nil, fmt.Errorf("Error getting content for feed %s: %v", feedID, err)
	}
	err = db.SetLatestPost(feedID, content)
	if err != nil {
		return nil, fmt.Errorf("Error setting latest post for feed %s: %v", feedID, err)
	}
	if oldFeed != nil {
		content, _ := (*oldFeed).content()
		err := db.SetPreviousPost(feedID, content)
		if err != nil {
			log.Printf("Error setting previous post for feed %s: %v", feedID, err)
		}
	}
	return newFeed, err
}

func (feeds *hytaleFeeds) NotifyFeeds(s *discordgo.Session) {
	if feeds.patchlinesFeed != nil {
		prevPatchlines, err := getStored(patchlinesID, true, feeds.db, deserializePatchlines)
		if err != nil {
			log.Printf("Error getting previous stored %s feed: %v", patchlinesID, err)
		}
		feeds.notifyFeed(s, feeds.patchlinesFeed, prevPatchlines)
	}

	for patchline, gameFeed := range feeds.gameFeeds {
		gameID := gameFeed.GetID()
		prevGameFeed, err := getStored(gameID, true, feeds.db, func(data []byte) (*GameFeed, error) {
			return deserializeGame(data, patchline)
		})
		if err != nil {
			log.Printf("Error getting previous stored %s feed: %v", gameID, err)
		}
		feeds.notifyFeed(s, gameFeed, prevGameFeed)
	}

	for patchline, mavenFeed := range feeds.mavenFeeds {
		mavenID := mavenFeed.GetID()
		prevMavenFeed, err := getStored(mavenID, true, feeds.db, func(data []byte) (*MavenFeed, error) {
			return deserializeMaven(data, patchline)
		})
		if err != nil {
			log.Printf("Error getting previous stored %s feed: %v", mavenID, err)
		}
		feeds.notifyFeed(s, mavenFeed, prevMavenFeed)
	}

	if feeds.launcherReleaseFeed != nil {
		prevLauncherRelease, err := getStored(launcherReleaseID, true, feeds.db, deserializeLauncherRelease)
		if err != nil {
			log.Printf("Error getting previous stored %s feed: %v", launcherReleaseID, err)
		}
		feeds.notifyFeed(s, feeds.launcherReleaseFeed, prevLauncherRelease)
	}

	if feeds.launcherPostFeed != nil {
		prevLauncherPost, err := getStored(launcherPostID, true, feeds.db, deserializeArticles)
		if err != nil {
			log.Printf("Error getting previous stored %s feed: %v", launcherPostID, err)
		}
		feeds.notifyFeed(s, feeds.launcherPostFeed, prevLauncherPost)
	}
}

func (feeds *hytaleFeeds) notifyFeed(s *discordgo.Session, feed Feed, previous Feed) {
	feedID := feed.GetID()
	targetIDs, err := feeds.db.GetSubscriptions(feedID)
	if err != nil {
		log.Printf("Error getting target IDs for feed %s: %v", feedID, err)
		return
	}
	for _, targetID := range targetIDs {
		feeds.notify(s, feed, previous, targetID)
	}
}

func (feeds *hytaleFeeds) notify(s *discordgo.Session, feed Feed, previous Feed, targetID string) {
	sub, err := feeds.db.GetSubscription(feed.GetID(), targetID)
	if err != nil {
		log.Printf("Error getting subscription from db: %v", err)
		return
	}
	if sub == nil {
		return
	}

	// Instead of comparing entire content (formatting can change), compare just the version
	if sub.CurrentVersion() != feed.GetVersion() {
		switch sub := sub.(type) {
		case db.GuildSubscription:
			_, err = s.Channel(targetID)
			if err != nil {
				log.Printf("Error accessing channel, removing: %v", err)
				feeds.RemoveAllSubscriptions(targetID)
			} else {
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

				feeds.db.AddOrUpdateSubscription(feed.GetID(), targetID, db.GuildSubscription{
					Version: feed.GetVersion(),
					Roles:   sub.Roles,
				})
			}

		case db.UserSubscription:
			_, err = s.User(targetID)
			if err != nil {
				log.Printf("Error accessing user, removing: %v", err)
				feeds.RemoveAllSubscriptions(targetID)
			} else {
				dm, err := s.UserChannelCreate(targetID)
				if err != nil {
					log.Printf("Cannot open DM: %v", err)
					return
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

				feeds.db.AddOrUpdateSubscription(feed.GetID(), targetID, db.UserSubscription{
					Version: feed.GetVersion(),
				})
			}

		default:
			panic("Invalid subscription type")
		}
	}
}

func (feeds *hytaleFeeds) RemoveAllSubscriptions(targetID string) {
	for feed := range feeds.Feeds() {
		feeds.db.RemoveSubscription(feed.GetID(), targetID)
	}
}

func roleMentions(roleIDs []string) string {
	var mentions []string
	for _, id := range roleIDs {
		mentions = append(mentions, fmt.Sprintf("<@&%s>", id))
	}
	return strings.Join(mentions, " ")
}
