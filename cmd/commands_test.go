package cmd

import (
	"fmt"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/itestutil"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/jarcoal/httpmock"
)

// Creates a mock command executor.
//
// The provided config (or a new config if nil passed) will be modified with:
// - Test DB parameters
// - URLs for feeds and profile lookups
// - Disabled breakers
//
// This will start the test DB. Be sure to clear the database before each test and cleanup when done.
func InitMockExecutor(c *config.Config) (*CommandExecutor, error) {
	if c == nil {
		c = &config.Config{}
	}

	c.Valkey.Address = "127.0.0.1"
	c.Valkey.Port = 9999
	c.Valkey.DatabaseIndex = 2

	c.Discord.InteractionCleanupInterval = 1_000_000
	c.Discord.InteractionExpiryTime = 1_000_000

	c.Feeds.LauncherRelease = "https://launcher.example.com/version/release/launcher.json"
	c.Feeds.LauncherArticles = "https://launcher.example.com/launcher-feed/release/feed.json"
	c.Feeds.GameVersion = "https://account-data.example.com/game-assets/version/"
	c.Feeds.MavenRepo = "https://maven.example.com"
	c.Feeds.MavenGroup = "com.hypixel.hytale"
	c.Feeds.MavenArtifact = "Server"

	c.Profile.ByUUID = "https://account-data.example.com/profile/uuid/"
	c.Profile.ByUsername = "https://account-data.example.com/profile/username/"
	c.Profile.Availability = "https://accounts.example.com/api/account/username-reservations/availability"
	c.Profile.Hyvatar = "https://hyvatar.example.com/"

	c.Auth.Breaker.Enabled = false
	c.Kratos.Breaker.Enabled = false

	db, err := db.NewDB(c.Valkey)
	if err != nil {
		return nil, err
	}
	db.Clear()

	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)
	itestutil.RegisterFeedResponders(c)

	authStore := testutil.NewTestAuthStore(http)
	feeds, err := hytale.NewHytaleFeeds(c, db, http, authStore)
	if err != nil {
		return nil, err
	}
	bootTime := time.Now()

	ce := NewCommandExecutor(c, db, http, authStore, feeds, "0.1.0", &bootTime)
	return &ce, nil
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
func NewMockContext(ce *CommandExecutor) *CommandContextMock {
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
