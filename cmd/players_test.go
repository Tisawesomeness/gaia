package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/itestutil"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

var (
	playersCE       *CommandExecutor
	playersTestCase = testutil.MakeTestCase(beforeEachPlayers, nil)
)

func init() {
	ce, err := InitMockExecutor(nil)
	if err != nil {
		panic(err)
	}
	playersCE = ce
}

func teardownPlayers() {
	playersCE.DB.Close()
}

func beforeEachPlayers() {
	playersCE.DB.ClearAll()
}

const (
	testUuid    = "d798091b-f494-4208-a1ba-e24da5880786"
	invalidUuid = "00000000-0000-0000-0000-000000000000"
)

// Registers a player that will be returned from the mock Hytale API when requested by UUID.
//
// If response blank, that player will not exist.
func registerUUID(uuid string, response string) {
	profileEndpoint := playersCE.Config.Profile.ByUUID + uuid
	if response != "" {
		httpmock.RegisterResponder("GET", profileEndpoint, httpmock.NewStringResponder(200, response))
	} else {
		httpmock.RegisterResponder("GET", profileEndpoint, httpmock.NewStringResponder(404, ""))
	}
}

// Registers a player that will be returned from the mock Hytale API when requested by username.
//
// Response cannot be blank. To register a username that doesn't exist, see registerUsernameUnused()
func registerUsername(username string, response string) {
	if response == "" {
		panic("response cannot be blank, use registerUsernameUnused() instead")
	}
	profileEndpoint := playersCE.Config.Profile.ByUsername + username
	httpmock.RegisterResponder("GET", profileEndpoint, httpmock.NewStringResponder(200, response))

	availabilityEndpoint := playersCE.Config.Profile.Availability
	httpmock.RegisterResponder("GET", availabilityEndpoint, httpmock.NewStringResponder(200, *hytale.InUse.ExpectedResponse()))
}

// Registers a username that will be returned from the mock Hytale API and that username's availability status.
func registerUsernameUnused(username string, availability hytale.Availability) {
	profileEndpoint := playersCE.Config.Profile.ByUsername + username
	httpmock.RegisterResponder("GET", profileEndpoint, httpmock.NewStringResponder(404, ""))

	availabilityEndpoint := playersCE.Config.Profile.Availability
	switch availability {
	case hytale.InUse:
		panic("availability cannot be hytale.InUse, use registerUsername() instead")
	case hytale.Unknown:
		httpmock.RegisterResponder("GET", availabilityEndpoint, httpmock.NewStringResponder(200, "unknown response nobody has seen before"))
	case hytale.Reserved:
		httpmock.RegisterResponder("GET", availabilityEndpoint, httpmock.NewStringResponder(400, *availability.ExpectedResponse()))
	default:
		httpmock.RegisterResponder("GET", availabilityEndpoint, httpmock.NewStringResponder(200, *availability.ExpectedResponse()))
	}
}

func TestProfile(t *testing.T) {
	t.Cleanup(teardownPlayers)
	config := playersCE.Config

	t.Run("/profile Valid Username", playersTestCase(func(t *testing.T) {
		registerUsername("tis", testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "tis")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for tis", embed)
		assert.Contains(t, embed.Description, testUuid)
	}))

	t.Run("/profile Valid UUID", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, testUuid)
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for tis", embed)
		assert.Contains(t, embed.Description, testUuid)
	}))

	t.Run("/profile Invalid format", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "-")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`-` is not a valid username or UUID")
	}))

	t.Run("/profile Username too long", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "veryverylongusername")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`veryverylongusername` is not a valid username or UUID")
	}))

	t.Run("/profile Fetch error", playersTestCase(func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+"errorPlayer", httpmock.NewStringResponder(500, ""))

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "errorPlayer")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, ":x: An error occurred while contacting Hytale servers.")
	}))

	t.Run("/profile UUID not found", playersTestCase(func(t *testing.T) {
		registerUUID(invalidUuid, "")

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, invalidUuid)
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "There is no player with the UUID")
	}))

	t.Run("/profile Username available", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("nonexistent", hytale.Available)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "nonexistent")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for nonexistent", embed)
		assert.Equal(t, "Username available", embed.Description)
	}))

	t.Run("/profile Username reserved", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("reservedUser", hytale.Reserved)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "reservedUser")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for reservedUser", embed)
		assert.Equal(t, "Username reserved", embed.Description)
	}))

	t.Run("/profile Username prohibited", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("badword", hytale.Prohibited)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "badword")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for badword", embed)
		assert.Equal(t, "Username contains a prohibited word", embed.Description)
	}))

	t.Run("/profile Kratos not configured", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("unknownStatus", hytale.Unknown)

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "unknownStatus")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for unknownStatus", embed)
		assert.Equal(t, "Username not in use (unknown status)", embed.Description)
	}))

	t.Run("/profile Availability check error", playersTestCase(func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.Availability, httpmock.NewStringResponder(500, ""))

		ctx := NewMockContext(playersCE).WithOption("player", discordgo.ApplicationCommandOptionString, "error")
		profileCommand(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, ":x: An error occurred while contacting Hytale servers.", reply.Content)
	}))
}
