package cmd

import (
	"fmt"
	"iter"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/testutil/atestutil"
	"github.com/Tisawesomeness/gaia/testutil/itestutil"
	"github.com/bwmarrin/discordgo"
	"github.com/jarcoal/httpmock"
)

type MockExecutor struct {
	*CommandExecutor
}

// Creates a mock command executor.
//
// The provided config (or a new config if nil passed) will be modified with:
// - Test DB parameters
// - URLs for feeds and profile lookups
// - Disabled breakers
//
// If no database is provided, this will start the test DB.
// Be sure to clear the database before each test and cleanup when done.
//
// All feeds are initialized to default values defined in [itestutil.RegisterFeedResponders].
//
// Creates an auth store with launcher auth.
func InitMockExecutor(c *config.Config, db *db.DB) (*MockExecutor, error) {
	return initMockExecutor(c, db, false)
}

// Like [InitMockExecutor], but creates [MockFeeds] initialized to no feeds present.
func InitMockExecutorWithMockFeeds(c *config.Config, db *db.DB) (*MockExecutor, error) {
	return initMockExecutor(c, db, true)
}

func initMockExecutor(c *config.Config, d *db.DB, mockFeeds bool) (*MockExecutor, error) {
	if c == nil {
		c = &config.Config{}
	}

	c.Valkey.Address = "127.0.0.1"
	c.Valkey.Port = 9999
	c.Valkey.DatabaseIndex = 2

	c.Discord.InteractionCleanupInterval = 1_000_000
	c.Discord.InteractionExpiryTime = 1_000_000

	c.Feeds.Patchlines = "https://account-data.example.com/my-account/get-patchlines"
	c.Feeds.LauncherRelease = "https://launcher.example.com/version/release/launcher.json"
	c.Feeds.LauncherArticles = "https://launcher.example.com/launcher-feed/release/feed.json"
	c.Feeds.GameVersion = "https://account-data.example.com/game-assets/version/"
	c.Feeds.MavenRepo = "https://maven.example.com"
	c.Feeds.MavenGroup = "com.hypixel.hytale"
	c.Feeds.MavenArtifact = "Server"

	c.Profile.ByUUID = "https://account-data.example.com/profile/uuid/"
	c.Profile.ByUsername = "https://account-data.example.com/profile/username/"
	c.Profile.Hyvatar = "https://hyvatar.example.com/"

	c.Auth.Breaker.Enabled = false
	c.Auth.LauncherData = "https://account-data.example.com/my-account/get-launcher-data?arch=amd64&os=windows"

	if d == nil {
		newDB, err := db.NewDB(c.Valkey)
		if err != nil {
			return nil, err
		}
		d = newDB
	}
	d.Clear()

	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)

	authStore := atestutil.NewTestAuthStore(auth.Launcher)

	itestutil.RegisterFeedResponders(c)
	var feeds hytale.HytaleFeeds
	if mockFeeds {
		feeds = newMockFeeds()
	} else {
		newFeeds, err := hytale.NewHytaleFeeds(c, d, http, authStore)
		if err != nil {
			return nil, err
		}
		feeds = newFeeds
	}
	bootTime := time.Now()

	ce := NewCommandExecutor(c, d, http, authStore, feeds, "0.1.0", &bootTime)
	return &MockExecutor{
		CommandExecutor: &ce,
	}, nil
}

func (ce *MockExecutor) WithPatchlinesFeed(feed *hytale.PatchlinesFeed) *MockExecutor {
	ce.HytaleFeeds.(*MockFeeds).patchlinesFeed = feed
	return ce
}

func (ce *MockExecutor) WithGameFeed(feed *hytale.GameFeed, patchline string) *MockExecutor {
	ce.HytaleFeeds.(*MockFeeds).gameFeeds[patchline] = feed
	return ce
}

func (ce *MockExecutor) WithMavenFeed(feed *hytale.MavenFeed, patchline string) *MockExecutor {
	ce.HytaleFeeds.(*MockFeeds).mavenFeeds[patchline] = feed
	return ce
}

func (ce *MockExecutor) WithLauncherReleaseFeed(feed *hytale.LauncherReleaseFeed) *MockExecutor {
	ce.HytaleFeeds.(*MockFeeds).launcherReleaseFeed = feed
	return ce
}

func (ce *MockExecutor) WithLauncherPostFeed(feed *hytale.LauncherPostFeed) *MockExecutor {
	ce.HytaleFeeds.(*MockFeeds).launcherPostFeed = feed
	return ce
}

type MockGuildData struct {
	channelIDs []string
}

type CommandContextMock struct {
	*commandContext

	id       string
	guild    *MockGuildData
	customID string
	options  OptionsMap

	replies []*discordgo.InteractionResponseData
	edits   []*discordgo.InteractionResponseData
}

const (
	TEST_USER_ID  = "123456789012345678"
	TEST_GUILD_ID = "876543210987654321"
)

func (ctx *CommandContextMock) Options() OptionsMap {
	return ctx.options
}

func (ctx *CommandContextMock) User() *discordgo.User {
	return &discordgo.User{
		ID:            TEST_USER_ID,
		Username:      "testuser",
		Discriminator: "0",
	}
}

func (ctx *CommandContextMock) UserCanExecute(command *discordgo.ApplicationCommand) bool {
	return true
}

func (ctx *CommandContextMock) GuildID() string {
	if ctx.guild != nil {
		return TEST_GUILD_ID
	} else {
		return ""
	}
}

func (ctx *CommandContextMock) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	if ctx.guild != nil {
		result := make([]*discordgo.Channel, len(ctx.guild.channelIDs))
		for i, channelID := range ctx.guild.channelIDs {
			result[i] = &discordgo.Channel{
				ID:      channelID,
				GuildID: guildID,
			}
		}
		return result, nil
	} else {
		return []*discordgo.Channel{}, nil
	}
}

func (ctx *CommandContextMock) InteractionID() string {
	return ctx.id
}

func (ctx *CommandContextMock) CustomID() *CustomID {
	return parseCustomID(ctx.customID)
}

func (ctx *CommandContextMock) NewInteraction(id string, state any) {
	interactionSessions[id] = &InteractionSession{
		State:    state,
		UserID:   ctx.User().ID,
		LastUsed: time.Now(),
	}
}

func (ctx *CommandContextMock) DeferReply() {
	ctx.hasDeferred = true
}

func (ctx *CommandContextMock) Reply(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
	})
}

func (ctx *CommandContextMock) ReplyEphemeral(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func (ctx *CommandContextMock) ReplyEmbed(embed *discordgo.MessageEmbed) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (ctx *CommandContextMock) ReplyWarn(content string) {
	ctx.ReplyEphemeral(":warning: " + content)
}

func (ctx *CommandContextMock) ReplyExternalError(content string) {
	ctx.ReplyEphemeral(":x: " + content)
}

func (ctx *CommandContextMock) ReplyError(message string, err error) {
	ctx.ReplyEphemeral(":boom: " + message)
}

func (ctx *CommandContextMock) ReplyComplex(data *discordgo.InteractionResponseData) {
	ctx.replies = append(ctx.replies, data)
}

func (ctx *CommandContextMock) Edit(data *discordgo.InteractionResponseData) {
	ctx.edits = append(ctx.edits, data)
}

// NewMockContext creates a new mock command context with the given command executor.
//
// The command will be called outside of a guild with no arguments by default. Use WithOptions() and WithGuild() to override.
//
// BEWARE: `Session()` and `Interaction()` are both nil, which causes Discord-specific functionality to fail.
func NewMockContext(ce *MockExecutor) *CommandContextMock {
	return &CommandContextMock{
		id:             randomID(),
		options:        make(OptionsMap),
		commandContext: ce.newCommandContext(nil, nil),
	}
}

// Selects the subcommand `name`
func (ctx *CommandContextMock) WithOptionSubCommand(name string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name: name,
		Type: discordgo.ApplicationCommandOptionSubCommand,
	}
	return ctx
}

// Selects the subcommand group `name`
func (ctx *CommandContextMock) WithOptionSubCommandGroup(name string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name: name,
		Type: discordgo.ApplicationCommandOptionSubCommandGroup,
	}
	return ctx
}

// Selects a string option. Also used for choice/enum options.
func (ctx *CommandContextMock) WithOptionString(name string, value string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionString,
		Value: value,
	}
	return ctx
}

// Selects an integer option.
func (ctx *CommandContextMock) WithOptionInteger(name string, value int64) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionInteger,
		Value: float64(value), // discordgo library expects float64 since that is what the json deserializes to
	}
	return ctx
}

// Selects a boolean option.
func (ctx *CommandContextMock) WithOptionBoolean(name string, value bool) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionBoolean,
		Value: value,
	}
	return ctx
}

// Selects a user option with the given ID.
func (ctx *CommandContextMock) WithOptionUser(name string, userID string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionUser,
		Value: userID,
	}
	return ctx
}

// Selects a channel option with the given ID.
func (ctx *CommandContextMock) WithOptionChannel(name string, channelID string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionChannel,
		Value: channelID,
	}
	return ctx
}

// Selects a role option with the given ID.
func (ctx *CommandContextMock) WithOptionRole(name string, roleID string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionRole,
		Value: roleID,
	}
	return ctx
}

// Selects a user/role option with the given ID.
func (ctx *CommandContextMock) WithOptionMentionable(name string, mentionableID string) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionMentionable,
		Value: mentionableID,
	}
	return ctx
}

// Selects a floating-point option.
func (ctx *CommandContextMock) WithOptionNumber(name string, value float64) *CommandContextMock {
	ctx.options[name] = &discordgo.ApplicationCommandInteractionDataOption{
		Name:  name,
		Type:  discordgo.ApplicationCommandOptionNumber,
		Value: value,
	}
	return ctx
}

// Causes the command to be run in a guild with the provided channel IDs.
func (ctx *CommandContextMock) WithGuild(channelIDs ...string) *CommandContextMock {
	if len(channelIDs) == 0 {
		panic("channelIDs cannot be empty")
	}
	ctx.guild = &MockGuildData{
		channelIDs: channelIDs,
	}
	return ctx
}

// Simulates clicking a component with the given ID. Required when calling an interaction handler.
//
//	ctx := NewMockContext(ce)
//	ctx.RunCommand("example") // Run /example
//	button := // Extract button from ctx.replies
//
//	componentID := button.CustomID
//	ctx2 := NewMockContext(ce).WithComponent(componentID)
//	ctx2.RunInteraction() // Interact with component
func (ctx *CommandContextMock) WithComponent(componentID string) *CommandContextMock {
	ctx.customID = componentID
	return ctx
}

func randomID() string {
	return fmt.Sprint(100_000_000_000_000_000 + rand.Int64N(900_000_000_000_000_000))
}

func (ctx *CommandContextMock) RunCommand(name string) {
	handleCommand(ctx, name)
}

func (ctx *CommandContextMock) RunInteraction() {
	if ctx.customID == "" {
		panic("Custom ID cannot be empty")
	}
	handleInteraction(ctx)
}

type MockFeeds struct {
	patchlinesFeed      *hytale.PatchlinesFeed
	gameFeeds           map[string]*hytale.GameFeed
	mavenFeeds          map[string]*hytale.MavenFeed
	launcherReleaseFeed *hytale.LauncherReleaseFeed
	launcherPostFeed    *hytale.LauncherPostFeed
}

func newMockFeeds() *MockFeeds {
	return &MockFeeds{
		gameFeeds:  make(map[string]*hytale.GameFeed),
		mavenFeeds: make(map[string]*hytale.MavenFeed),
	}
}

func (feeds *MockFeeds) GetPatchlinesFeed() (*hytale.PatchlinesFeed, bool) {
	return feeds.patchlinesFeed, feeds.patchlinesFeed != nil
}

func (feeds *MockFeeds) GetGameFeed(patchline string) (*hytale.GameFeed, bool) {
	feed, ok := feeds.gameFeeds[patchline]
	return feed, ok
}

func (feeds *MockFeeds) GetMavenFeed(patchline string) (*hytale.MavenFeed, bool) {
	feed, ok := feeds.mavenFeeds[patchline]
	return feed, ok
}

func (feeds *MockFeeds) GetLauncherReleaseFeed() (*hytale.LauncherReleaseFeed, bool) {
	return feeds.launcherReleaseFeed, feeds.launcherReleaseFeed != nil
}

func (feeds *MockFeeds) GetLauncherPostFeed() (*hytale.LauncherPostFeed, bool) {
	return feeds.launcherPostFeed, feeds.launcherPostFeed != nil
}

func (feeds *MockFeeds) Feeds() iter.Seq[hytale.Feed] {
	return func(yield func(hytale.Feed) bool) {
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

func (feeds *MockFeeds) Poll() {
	// no-op
}

func (feeds *MockFeeds) NotifyFeeds(s *discordgo.Session) {
	// no-op
}

func (feeds *MockFeeds) RemoveAllSubscriptions(targetID string) {
	// no-op
}
