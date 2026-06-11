package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/itestutil"
	"github.com/Tisawesomeness/gaia/testutil"
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
	playersCE.DB.Clear()
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

		ctx := NewMockContext(playersCE).WithOptionString("player", "tis")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for tis", embed)
		assert.Contains(t, embed.Description, testUuid)
	}))

	t.Run("/profile Valid UUID", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for tis", embed)
		assert.Contains(t, embed.Description, testUuid)
	}))

	t.Run("/profile Invalid format", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOptionString("player", "-")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`-` is not a valid username or UUID")
	}))

	t.Run("/profile Username too long", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOptionString("player", "veryverylongusername")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`veryverylongusername` is not a valid username or UUID")
	}))

	t.Run("/profile Fetch error", playersTestCase(func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+"errorPlayer", httpmock.NewStringResponder(500, ""))

		ctx := NewMockContext(playersCE).WithOptionString("player", "errorPlayer")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, ":x: An error occurred while contacting Hytale servers.")
	}))

	t.Run("/profile UUID not found", playersTestCase(func(t *testing.T) {
		registerUUID(invalidUuid, "")

		ctx := NewMockContext(playersCE).WithOptionString("player", invalidUuid)
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "There is no player with the UUID")
	}))

	t.Run("/profile Username available", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("nonexistent", hytale.Available)

		ctx := NewMockContext(playersCE).WithOptionString("player", "nonexistent")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for nonexistent", embed)
		assert.Equal(t, "Username available", embed.Description)
	}))

	t.Run("/profile Username reserved", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("reservedUser", hytale.Reserved)

		ctx := NewMockContext(playersCE).WithOptionString("player", "reservedUser")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for reservedUser", embed)
		assert.Equal(t, "Username reserved", embed.Description)
	}))

	t.Run("/profile Username prohibited", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("badword", hytale.Prohibited)

		ctx := NewMockContext(playersCE).WithOptionString("player", "badword")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for badword", embed)
		assert.Equal(t, "Username contains a prohibited word", embed.Description)
	}))

	t.Run("/profile Kratos not configured", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("unknownStatus", hytale.Unknown)

		ctx := NewMockContext(playersCE).WithOptionString("player", "unknownStatus")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Profile for unknownStatus", embed)
		assert.Equal(t, "Username not in use (unknown status)", embed.Description)
	}))

	t.Run("/profile Availability check error", playersTestCase(func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.Availability, httpmock.NewStringResponder(500, ""))

		ctx := NewMockContext(playersCE).WithOptionString("player", "error")
		getCommand("profile").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, ":x: An error occurred while contacting Hytale servers.", reply.Content)
	}))
}

func TestSkin(t *testing.T) {
	t.Cleanup(teardownPlayers)
	config := playersCE.Config

	t.Run("/skin Valid UUID with skin", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Skin details for tis", embed)
		assert.Regexp(t, `.*full/tis.*`, embed.Image.URL)
		assert.NotEmpty(t, embed.Fields)
		head := itestutil.FindField(embed, "Head")
		assert.NotNil(t, head)
		assert.Contains(t, head.Value, "Haircut: `SuperSlickback.BlondCaramel`")
		assert.Contains(t, head.Value, "Eyes: `Almond_Eyes.Grey`")
		assert.Contains(t, head.Value, "Facial Hair: (none)")
		underwear := itestutil.FindField(embed, "General")
		assert.NotNil(t, underwear)
		assert.Contains(t, underwear.Value, "Underwear: `Suit.Blue`")
		assert.Contains(t, underwear.Value, "Body Characteristic: `Muscular.10`")
		assert.Contains(t, underwear.Value, "Face: `Face_Almond_Eyes`")
		assert.Contains(t, underwear.Value, "Mouth: `Mouth_Default`")
		assert.Contains(t, underwear.Value, "Ears: `Elf_Ears`")
	}))

	t.Run("/skin Valid UUID without skin", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponseNoSkin)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Skin details for tis", embed)
		assert.NotEmpty(t, embed.Image.URL)
		assert.Equal(t, "(no skin)", embed.Description)
		assert.Empty(t, embed.Fields)
	}))

	t.Run("/skin Valid Username with skin", playersTestCase(func(t *testing.T) {
		registerUsername("tis", testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", "tis")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Skin details for tis", embed)
		assert.Regexp(t, `.*full/tis.*`, embed.Image.URL)
		head := itestutil.FindField(embed, "Head")
		assert.NotNil(t, head)
		assert.Contains(t, head.Value, "Haircut: `SuperSlickback.BlondCaramel`")
	}))

	t.Run("/skin Valid Username with skin and unknown cosmetic", playersTestCase(func(t *testing.T) {
		registerUsername("tis", testutil.SampleProfileResponseExtra)

		ctx := NewMockContext(playersCE).WithOptionString("player", "tis")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		itestutil.AssertEmbedTitle(t, "Skin details for tis", embed)
		assert.Regexp(t, `.*full/tis.*`, embed.Image.URL)
		extra := itestutil.FindField(embed, "Extra")
		assert.NotNil(t, extra)
		assert.Contains(t, extra.Value, "Booster: `RocketBoosters`")
	}))

	t.Run("/skin Invalid format", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOptionString("player", "-")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`-` is not a valid username or UUID")
	}))

	t.Run("/skin Username too long", playersTestCase(func(t *testing.T) {
		ctx := NewMockContext(playersCE).WithOptionString("player", "veryverylongusername")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "`veryverylongusername` is not a valid username or UUID")
	}))

	t.Run("/skin Fetch error", playersTestCase(func(t *testing.T) {
		httpmock.RegisterResponder("GET", config.Profile.ByUsername+"errorPlayer", httpmock.NewStringResponder(500, ""))

		ctx := NewMockContext(playersCE).WithOptionString("player", "errorPlayer")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, ":x: An error occurred while contacting Hytale servers.")
	}))

	t.Run("/skin UUID not found", playersTestCase(func(t *testing.T) {
		registerUUID(invalidUuid, "")

		ctx := NewMockContext(playersCE).WithOptionString("player", invalidUuid)
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "There is no player with the UUID")
	}))

	t.Run("/skin Username not found", playersTestCase(func(t *testing.T) {
		registerUsernameUnused("doesnotexist", hytale.Available)

		ctx := NewMockContext(playersCE).WithOptionString("player", "doesnotexist")
		getCommand("skin").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Contains(t, reply.Content, "There is no player with the username")
	}))
}

func TestRender(t *testing.T) {
	t.Cleanup(teardownPlayers)

	t.Run("/head Valid UUID default size", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("head").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
	}))

	t.Run("/head Valid UUID with size and rotate", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid).WithOptionInteger("size", 2048).WithOptionInteger("rotate", 90)
		getCommand("head").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		assert.Contains(t, embed.Image.URL, "size=2048")
		assert.Contains(t, embed.Image.URL, "rotate=90")
	}))

	t.Run("/head Valid UUID with only size", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid).WithOptionInteger("size", 1024)
		getCommand("head").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		assert.Contains(t, embed.Image.URL, "size=1024")
	}))

	t.Run("/head Valid UUID with only rotate", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid).WithOptionInteger("rotate", 180)
		getCommand("head").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
		embed := reply.Embeds[0]
		assert.Contains(t, embed.Image.URL, "rotate=180")
	}))

	t.Run("/body Valid UUID default size", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("body").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
	}))

	t.Run("/cape Valid UUID with cape, default size", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponseCape)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("cape").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Equal(t, 1, len(reply.Embeds))
	}))

	t.Run("/cape - No capes!", playersTestCase(func(t *testing.T) {
		registerUUID(testUuid, testutil.SampleProfileResponse)

		ctx := NewMockContext(playersCE).WithOptionString("player", testUuid)
		getCommand("cape").handler(ctx)

		assert.Equal(t, 1, len(ctx.replies))
		reply := ctx.replies[0]
		assert.Empty(t, reply.Embeds)
		assert.Equal(t, "That player does not have a cape.", reply.Content)
	}))
}
