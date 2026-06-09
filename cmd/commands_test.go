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
// This will start the test DB. Be sure to clear the database before each test and cleanup when done.
func InitMockExecutor() (*CommandExecutor, error) {
	config := &config.Config{
		Valkey: config.ValkeyConfig{
			Address: "127.0.0.1",
			Port:    9999,
		},
		Feeds: config.FeedsConfig{
			LauncherRelease:  "https://launcher.example.com/version/release/launcher.json",
			LauncherArticles: "https://launcher.example.com/launcher-feed/release/feed.json",
			GameVersion:      "https://account-data.example.com/game-assets/version/",
			MavenRepo:        "https://maven.example.com",
			MavenGroup:       "com.hypixel.hytale",
			MavenArtifact:    "Server",
		},
		Profile: config.ProfileConfig{
			ByUUID:       "https://account-data.example.com/profile/uuid/",
			ByUsername:   "https://account-data.example.com/profile/username/",
			Availability: "https://accounts.example.com/api/account/username-reservations/availability",
			Hyvatar:      "https://hyvatar.example.com/",
		},
		Auth: config.AuthConfig{
			Breaker: config.BreakerConfig{
				Enabled: false,
			},
		},
		Kratos: config.KratosConfig{
			Breaker: config.BreakerConfig{
				Enabled: false,
			},
		},
	}

	db, err := db.NewDB(config.Valkey)
	if err != nil {
		return nil, err
	}
	db.ClearAll()

	http := &http.Client{
		Timeout: time.Duration(10) * time.Second,
	}
	httpmock.ActivateNonDefault(http)
	itestutil.RegisterFeedResponders(config)

	authStore := testutil.NewTestAuthStore(http)
	feeds, err := hytale.NewHytaleFeeds(config, db, http, authStore)
	if err != nil {
		return nil, err
	}
	bootTime := time.Now()

	ce := NewCommandExecutor(config, db, http, authStore, feeds, "0.1.0", &bootTime)
	return &ce, nil
}

type MockGuildData struct {
	channelIDs []string
}

type CommandContextMock struct {
	*commandContext
	id      string
	guild   *MockGuildData
	options OptionsMap
	replies []*discordgo.InteractionResponseData
}

const (
	TEST_USER_ID  = "123456789012345678"
	TEST_GUILD_ID = "876543210987654321"
)

func (ctx *CommandContextMock) InteractionID() string {
	return ctx.id
}

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
	panic(fmt.Errorf("%s: %v", message, err))
}

func (ctx *CommandContextMock) ReplyComplex(data *discordgo.InteractionResponseData) {
	ctx.replies = append(ctx.replies, data)
}

// NewMockContext creates a new mock command context with the given command executor.
//
// The command will be called outside of a guild with no arguments by default. Use WithOptions() and WithGuild() to override.
//
// BEWARE: `Session()` and `Interaction()` are both nil, which causes commands that rely on Discord-specific functionality
// (such as pagination) to fail.
func NewMockContext(ce *CommandExecutor) *CommandContextMock {
	return &CommandContextMock{
		id:             randomID(),
		options:        make(OptionsMap),
		commandContext: ce.newCommandContext(nil, nil),
	}
}

// Sets the arguments/options the command was called with.
func (c *CommandContextMock) WithOptions(options OptionsMap) *CommandContextMock {
	if options == nil {
		panic("options must not be nil")
	} else {
		c.options = options
	}
	return c
}

// Causes the command to be run in a guild with the provided channel IDs.
func (c *CommandContextMock) WithGuild(channelIDs ...string) *CommandContextMock {
	if len(channelIDs) == 0 {
		panic("channelIDs cannot be empty")
	}
	c.guild = &MockGuildData{
		channelIDs: channelIDs,
	}
	return c
}

func randomID() string {
	return fmt.Sprint(100_000_000_000_000_000 + rand.Int64N(900_000_000_000_000_000))
}
