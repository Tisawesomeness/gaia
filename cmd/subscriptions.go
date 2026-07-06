package cmd

import (
	"fmt"
	"log"
	"slices"
	"sort"
	"strings"

	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	feedTypeChoices = []*discordgo.ApplicationCommandOptionChoice{
		{
			Name:  hytale.GameFeedType.Display(),
			Value: hytale.GameFeedType.ID(),
		},
		{
			Name:  hytale.MavenFeedType.Display(),
			Value: hytale.MavenFeedType.ID(),
		},
		{
			Name:  hytale.LauncherReleaseFeedType.Display(),
			Value: hytale.LauncherReleaseFeedType.ID(),
		},
		{
			Name:  hytale.LauncherPostFeedType.Display(),
			Value: hytale.LauncherPostFeedType.ID(),
		},
		{
			Name:  hytale.PatchlinesFeedType.Display(),
			Value: hytale.PatchlinesFeedType.ID(),
		},
	}
	patchlineOptionSubscriptions = &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "patchline",
		Description: "The patchline to subscribe to (for new client/server releases)",
	}
	manageSubscriptionsPermissions = int64(discordgo.PermissionManageWebhooks)

	SubscribeCommand = &discordgo.ApplicationCommand{
		Name:        "subscribe",
		Description: "Subscribe to Hytale updates",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
		},
		DefaultMemberPermissions: &manageSubscriptionsPermissions,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "What to subscribe to",
				Required:    true,
				Choices:     feedTypeChoices,
			},
			patchlineOptionSubscriptions,
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel where updates will be posted",
				Required:    true,
			},
			{
				Type:        discordgo.ApplicationCommandOptionRole,
				Name:        "role",
				Description: "Role to ping",
			},
		},
	}

	SubscribeDMCommand = &discordgo.ApplicationCommand{
		Name:        "subscribe-dm",
		Description: "Subscribe to Hytale updates in DMs",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextBotDM,
		},
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "What to subscribe to",
				Required:    true,
				Choices:     feedTypeChoices,
			},
			patchlineOptionSubscriptions,
		},
	}

	ListCommand = &discordgo.ApplicationCommand{
		Name:        "list-subscriptions",
		Description: "List all subscriptions",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
		},
		DefaultMemberPermissions: &manageSubscriptionsPermissions,
	}

	UnsubscribeCommand = &discordgo.ApplicationCommand{
		Name:        "unsubscribe",
		Description: "Unsubscribe from Hytale updates",
		Contexts: &[]discordgo.InteractionContextType{
			discordgo.InteractionContextGuild,
			discordgo.InteractionContextBotDM,
		},
		DefaultMemberPermissions: &manageSubscriptionsPermissions,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Which channel to unsubscribe (defaults to all channels)",
			},
		},
	}
)

func getFeed(feeds hytale.HytaleFeeds, feedType hytale.FeedType, patchline string) (hytale.Feed, bool) {
	switch feedType {
	case hytale.PatchlinesFeedType:
		return feeds.GetPatchlinesFeed()
	case hytale.GameFeedType:
		return feeds.GetGameFeed(patchline)
	case hytale.MavenFeedType:
		return feeds.GetMavenFeed(patchline)
	case hytale.LauncherReleaseFeedType:
		return feeds.GetLauncherReleaseFeed()
	case hytale.LauncherPostFeedType:
		return feeds.GetLauncherPostFeed()
	default:
		panic("unknown feed type")
	}
}

func subscribeCommand(ctx CommandContext) {
	options := ctx.Options()

	subType := options["type"].StringValue()
	feedType, err := hytale.ParseFeedType(subType)
	if err != nil {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	var patchline string
	if feedType == hytale.GameFeedType || feedType == hytale.MavenFeedType {
		option, exists := options["patchline"]
		patchlineInput := "release"
		if exists {
			patchlineInput = option.StringValue()
		}

		if patchlineInput == "release" {
			patchline = "release"
		} else {
			patchlineFeed, exists := ctx.HytaleFeeds().GetPatchlinesFeed()
			if !exists {
				ctx.ReplyError("Could not retrieve Hytale patchlines.", nil)
				return
			}
			patchline = hytale.ClosestPatchline(patchlineInput, patchlineFeed.Patchlines)
			if patchline == "" {
				ctx.ReplyWarn("Patchline must be one of: " + displayPatchlineList(patchlineFeed.Patchlines))
				return
			}
		}

	} else {
		_, exists := options["patchline"]
		if exists {
			ctx.ReplyWarn(feedType.Display() + " subscriptions do not have a patchline")
			return
		}
	}

	feed, exists := getFeed(ctx.HytaleFeeds(), feedType, patchline)
	if !exists {
		ctx.ReplyError("Could not retrieve subscription data.", nil)
		return
	}

	channel := options["channel"].ChannelValue(nil)
	var role *discordgo.Role
	roleOption, exists := options["role"]
	if exists {
		role = roleOption.RoleValue(nil, "")
	}

	roles := []string{}
	if role != nil {
		roles = append(roles, role.ID)
	}
	err = ctx.DB().AddOrUpdateSubscription(feed.GetID(), channel.ID, db.GuildSubscription{
		Version: feed.GetVersion(),
		Roles:   roles,
	})
	if err != nil {
		ctx.ReplyError("Error while trying to save subscription", err)
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed %s to %s", channel.Mention(), feed.GetDisplay()))
	}
}

func subscribeDMCommand(ctx CommandContext) {
	options := ctx.Options()

	subType := options["type"].StringValue()
	feedType, err := hytale.ParseFeedType(subType)
	if err != nil {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	var patchline string
	if feedType == hytale.GameFeedType || feedType == hytale.MavenFeedType {
		option, exists := options["patchline"]
		patchlineInput := "release"
		if exists {
			patchlineInput = option.StringValue()
		}

		if patchlineInput == "release" {
			patchline = "release"
		} else {
			patchlineFeed, exists := ctx.HytaleFeeds().GetPatchlinesFeed()
			if !exists {
				ctx.ReplyError("Could not retrieve Hytale patchlines.", nil)
				return
			}
			patchline = hytale.ClosestPatchline(patchlineInput, patchlineFeed.Patchlines)
			if patchline == "" {
				ctx.ReplyWarn("Patchline must be one of: " + displayPatchlineList(patchlineFeed.Patchlines))
				return
			}
		}

	} else {
		_, exists := options["patchline"]
		if exists {
			ctx.ReplyWarn(feedType.Display() + " subscriptions do not have a patchline")
			return
		}
	}

	feed, exists := getFeed(ctx.HytaleFeeds(), feedType, patchline)
	if !exists {
		ctx.ReplyError("Could not retrieve subscription data.", nil)
		return
	}

	err = ctx.DB().AddOrUpdateSubscription(feed.GetID(), ctx.User().ID, db.UserSubscription{
		Version: feed.GetVersion(),
	})
	if err != nil {
		ctx.ReplyError("Error while trying to save subscription", err)
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed to %s", feed.GetDisplay()))
	}
}

func listCommand(ctx CommandContext) {
	guildID := ctx.GuildID()
	if guildID == "" {
		userSubscriptions := []hytale.Feed{}
		for feed := range ctx.HytaleFeeds().Feeds() {
			targetIDs, err := ctx.DB().GetSubscriptions(feed.GetID())
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feed.GetID(), err)
				continue
			}

			if slices.Contains(targetIDs, ctx.User().ID) {
				userSubscriptions = append(userSubscriptions, feed)
			}
		}

		var embed *discordgo.MessageEmbed
		if len(userSubscriptions) == 0 {
			embed = &discordgo.MessageEmbed{
				Title:       "Active Subscriptions",
				Description: "(none yet)",
				Color:       0xFF0000,
			}
		} else {
			embed = &discordgo.MessageEmbed{
				Title: "Active Subscriptions",
				Color: 0x00FF00,
			}

			subscriptionNames := []string{}
			for _, feed := range userSubscriptions {
				subscriptionNames = append(subscriptionNames, "- "+feed.GetDisplay())
			}
			embed.Description = strings.Join(subscriptionNames, "\n")
		}

		ctx.ReplyEmbed(embed)
	} else {
		channels, err := ctx.GuildChannels(guildID)
		if err != nil {
			ctx.ReplyError("Error while fetching guild channels", err)
			return
		}

		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Name < channels[j].Name
		})

		channelMap := make(map[string]*discordgo.Channel)
		for _, channel := range channels {
			channelMap[channel.ID] = channel
		}

		channelSubscriptions := make(map[string][]hytale.Feed)
		for feed := range ctx.HytaleFeeds().Feeds() {
			tagetIDs, err := ctx.DB().GetSubscriptions(feed.GetID())
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feed.GetID(), err)
				continue
			}

			for _, targetID := range tagetIDs {
				sub, err := ctx.DB().GetSubscription(feed.GetID(), targetID)
				if err != nil {
					log.Printf("Error fetching subscription for %s %s: %v", feed.GetID(), targetID, err)
				}
				_, ok := sub.(db.GuildSubscription)
				if ok {
					if channel, exists := channelMap[targetID]; exists {
						channelSubscriptions[channel.ID] = append(channelSubscriptions[channel.ID], feed)
					}
				}
			}
		}

		var embed *discordgo.MessageEmbed
		if len(channelSubscriptions) == 0 {
			embed = &discordgo.MessageEmbed{
				Title:       "Active Subscriptions",
				Description: "(none yet)",
				Color:       0xFF0000,
			}
		} else {
			embed = &discordgo.MessageEmbed{
				Title:       "Active Subscriptions",
				Description: "List of all subscriptions in this guild:",
				Color:       0x00FF00,
			}

			description := []string{}
			for channelID, feeds := range channelSubscriptions {
				if channel, exists := channelMap[channelID]; exists {
					subscriptionNames := []string{}
					for _, feed := range feeds {
						subscriptionNames = append(subscriptionNames, feed.GetDisplay())
					}
					description = append(description, "- "+channel.Mention()+" - **"+strings.Join(subscriptionNames, ", ")+"**")
				}
			}
			embed.Description = strings.Join(description, "\n")
		}

		ctx.ReplyEmbed(embed)
	}
}

func unsubscribeCommand(ctx CommandContext) {
	guildID := ctx.GuildID()
	options := ctx.Options()

	if guildID == "" {
		ctx.HytaleFeeds().RemoveAllSubscriptions(ctx.User().ID)
		ctx.Reply("Unsubscribed all feeds.")
	} else {
		var channelID string
		if channelOpt, exists := options["channel"]; exists {
			channelID = channelOpt.ChannelValue(nil).ID
		}

		if channelID != "" {
			ctx.HytaleFeeds().RemoveAllSubscriptions(channelID)
			channel := options["channel"].ChannelValue(nil)
			ctx.Reply("Unsubscribed all feeds from channel: " + channel.Mention())
		} else {
			channels, err := ctx.GuildChannels(guildID)
			if err != nil {
				ctx.ReplyError("Error while fetching guild channels", err)
				return
			}

			for _, channel := range channels {
				ctx.HytaleFeeds().RemoveAllSubscriptions(channel.ID)
			}

			ctx.Reply("Unsubscribed all feeds in this guild.")
		}
	}
}
