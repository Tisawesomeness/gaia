package cmd

import (
	"testing"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/itestutil"
	"github.com/Tisawesomeness/gaia/testutil"
	"github.com/bwmarrin/discordgo"
	"github.com/stretchr/testify/assert"
)

var (
	subscriptionsCE       *CommandExecutor
	subscriptionsTestCase = testutil.MakeTestCase(beforeEachSubscriptions, nil)
)

func init() {
	ce, err := InitMockExecutor(nil)
	if err != nil {
		panic(err)
	}
	subscriptionsCE = ce
}

func teardownSubscriptions() {
	subscriptionsCE.DB.Close()
}

func beforeEachSubscriptions() {
	subscriptionsCE.DB.Clear()
}

func extractContent(reply *discordgo.InteractionResponseData) string {
	if reply == nil {
		return ""
	}
	return itestutil.ExtractMainContent(reply)
}

func TestSubscriptionsCommands(t *testing.T) {
	t.Cleanup(teardownSubscriptions)

	channelIDs := []string{"983157098887027849", "548167422780907907"}
	roleID := "141193494241241025"

	t.Run("/subscribe game-release adds entry to list", subscriptionsTestCase(func(t *testing.T) {
		ctx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0]).WithOptionRole("role", roleID)

		ctx.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx.replies[0]), "Subscribed")

		// Verify subscription appears in list
		listCtx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx.RunCommand("list-subscriptions")

		assert.Contains(t, extractContent(listCtx.replies[0]), hytale.GameReleaseFeedType.Display())
	}))

	t.Run("/subscribe maven-release adds DM subscription", subscriptionsTestCase(func(t *testing.T) {
		ctx := NewMockContext(subscriptionsCE).WithOptionString("type", hytale.MavenReleaseFeedType.ID())

		ctx.RunCommand("subscribe-dm")
		assert.Contains(t, extractContent(ctx.replies[0]), "Subscribed")

		// Verify DM subscription appears in list
		listCtx := NewMockContext(subscriptionsCE)
		listCtx.RunCommand("list-subscriptions")

		assert.Contains(t, extractContent(listCtx.replies[0]), hytale.MavenReleaseFeedType.Display())
	}))

	t.Run("/subscribe game-release and maven-release adds both to list", subscriptionsTestCase(func(t *testing.T) {
		// Subscribe to game-release in channel1
		ctx1 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0])
		ctx1.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx1.replies[0]), "Subscribed")

		// Subscribe to maven-release in channel2
		ctx2 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.MavenReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[1])
		ctx2.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx2.replies[0]), "Subscribed")

		// List subscriptions
		listCtx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx.RunCommand("list-subscriptions")

		assert.Contains(t, extractContent(listCtx.replies[0]), hytale.GameReleaseFeedType.Display())
		assert.Contains(t, extractContent(listCtx.replies[0]), hytale.MavenReleaseFeedType.Display())
	}))

	t.Run("/subscribe twice for same channel updates version", subscriptionsTestCase(func(t *testing.T) {
		// First subscription at 0.1.0
		ctx1 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0])
		ctx1.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx1.replies[0]), "Subscribed")

		// Second subscription at 0.2.0 (simulated by calling again)
		_ = NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0])
		ctx1.RunCommand("subscribe")

		// List subscriptions - should still have entry
		listCtx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx.RunCommand("list-subscriptions")
		assert.Contains(t, extractContent(listCtx.replies[0]), hytale.GameReleaseFeedType.Display())
	}))

	t.Run("/unsubscribe without args clears all guild subscriptions", subscriptionsTestCase(func(t *testing.T) {
		// Subscribe in guild channel1
		ctx1 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0])
		ctx1.RunCommand("subscribe")

		// Subscribe in DM
		ctx2 := NewMockContext(subscriptionsCE).WithOptionString("type", hytale.MavenReleaseFeedType.ID())
		ctx2.RunCommand("subscribe-dm")

		// Subscribe with role in channel2
		ctx3 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.MavenReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[1]).WithOptionRole("role", roleID)
		ctx3.RunCommand("subscribe")

		// List should show subscriptions
		listCtx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx.RunCommand("list-subscriptions")
		content := extractContent(listCtx.replies[0])
		assert.Contains(t, content, hytale.GameReleaseFeedType.Display())

		// Unsubscribe (no args - should remove all)
		ctx4 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		ctx4.RunCommand("unsubscribe")
		assert.Contains(t, extractContent(ctx4.replies[0]), "Unsubscribed")

		// List should show empty now
		listCtx2 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx2.RunCommand("list-subscriptions")
		assert.Contains(t, extractContent(listCtx2.replies[0]), "(none yet)")
	}))

	t.Run("/unsubscribe with channel removes from that channel only", subscriptionsTestCase(func(t *testing.T) {
		// Subscribe in channel1
		ctx1 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.GameReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[0])
		ctx1.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx1.replies[0]), "Subscribed")

		// Subscribe in channel2
		ctx2 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", hytale.MavenReleaseFeedType.ID()).WithOptionChannel("channel", channelIDs[1])
		ctx2.RunCommand("subscribe")
		assert.Contains(t, extractContent(ctx2.replies[0]), "Subscribed")

		// Unsubscribe only channel1
		ctx3 := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionChannel("channel", channelIDs[0])
		ctx3.RunCommand("unsubscribe")
		assert.Contains(t, extractContent(ctx3.replies[0]), "Unsubscribed all feeds from channel")

		// List should show only channel2 subscription
		listCtx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...)
		listCtx.RunCommand("list-subscriptions")
		content := extractContent(listCtx.replies[0])
		assert.NotContains(t, content, channelIDs[0])
		assert.Contains(t, content, channelIDs[1])
		assert.Contains(t, content, hytale.MavenReleaseFeedType.Display())
	}))

	t.Run("/subscribe invalid type shows warning", subscriptionsTestCase(func(t *testing.T) {
		ctx := NewMockContext(subscriptionsCE).WithGuild(channelIDs...).WithOptionString("type", "invalid-type").WithOptionChannel("channel", channelIDs[0])

		ctx.RunCommand("subscribe")

		assert.Contains(t, extractContent(ctx.replies[0]), "Invalid feed type")
	}))
}
