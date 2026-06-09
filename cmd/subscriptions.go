package cmd

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	feedChoices = []*discordgo.ApplicationCommandOptionChoice{
		{
			Name:  hytale.GameReleaseFeedType.Display(),
			Value: hytale.GameReleaseFeedType.ID(),
		},
		{
			Name:  hytale.GamePreReleaseFeedType.Display(),
			Value: hytale.GamePreReleaseFeedType.ID(),
		},
		{
			Name:  hytale.MavenReleaseFeedType.Display(),
			Value: hytale.MavenReleaseFeedType.ID(),
		},
		{
			Name:  hytale.MavenPreReleaseFeedType.Display(),
			Value: hytale.MavenPreReleaseFeedType.ID(),
		},
		{
			Name:  hytale.LauncherReleaseFeedType.Display(),
			Value: hytale.LauncherReleaseFeedType.ID(),
		},
		{
			Name:  hytale.LauncherPostFeedType.Display(),
			Value: hytale.LauncherPostFeedType.ID(),
		},
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
				Choices:     feedChoices,
			},
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
				Choices:     feedChoices,
			},
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

func subscribeCommand(ctx CommandContext) {
	options := ctx.Options()

	subType := options["type"].StringValue()
	channel := options["channel"].ChannelValue(nil)
	var role *discordgo.Role
	roleOption, exists := options["role"]
	if exists {
		role = roleOption.RoleValue(nil, "")
	}

	feedType, err := hytale.ParseFeedType(subType)
	if err != nil {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}
	feed, exists := ctx.HytaleFeeds().Feeds[feedType]
	if !exists {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	roles := []string{}
	if role != nil {
		roles = append(roles, role.ID)
	}
	err = ctx.DB().AddOrUpdateSubscription(feedType.ID(), channel.ID, db.GuildSubscription{
		Version: feed.GetVersion(),
		Roles:   roles,
	})
	if err != nil {
		ctx.ReplyError("Error while trying to save subscription", err)
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed %s to %s", channel.Mention(), feedType.Display()))
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
	feed, exists := ctx.HytaleFeeds().Feeds[feedType]
	if !exists {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	err = ctx.DB().AddOrUpdateSubscription(feedType.ID(), ctx.User().ID, db.UserSubscription{
		Version: feed.GetVersion(),
	})
	if err != nil {
		ctx.ReplyError("Error while trying to save subscription", err)
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed to %s", feedType.Display()))
	}
}

func listCommand(ctx CommandContext) {
	guildID := ctx.GuildID()
	if guildID == "" {
		userSubscriptions := make(map[hytale.FeedType]string)
		for feedType := range ctx.HytaleFeeds().Feeds {
			targetIDs, err := ctx.DB().GetSubscriptions(feedType.ID())
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feedType.ID(), err)
				continue
			}

			for _, targetID := range targetIDs {
				if targetID == ctx.User().ID {
					if _, exists := ctx.HytaleFeeds().Feeds[feedType]; exists {
						userSubscriptions[feedType] = feedType.Display()
					}
				}
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
			for _, displayName := range userSubscriptions {
				subscriptionNames = append(subscriptionNames, "- "+displayName)
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

		channelSubscriptions := make(map[string][]hytale.FeedType)
		for feedType := range ctx.HytaleFeeds().Feeds {
			tagetIDs, err := ctx.DB().GetSubscriptions(feedType.ID())
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feedType.ID(), err)
				continue
			}

			for _, targetID := range tagetIDs {
				sub, err := ctx.DB().GetSubscription(feedType.ID(), targetID)
				if err != nil {
					log.Printf("Error fetching subscription for %s %s: %v", feedType.ID(), targetID, err)
				}
				_, ok := sub.(db.GuildSubscription)
				if ok {
					if channel, exists := channelMap[targetID]; exists {
						channelSubscriptions[channel.ID] = append(channelSubscriptions[channel.ID], feedType)
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
			for channelID, subscriptions := range channelSubscriptions {
				if channel, exists := channelMap[channelID]; exists {
					subscriptionNames := []string{}
					for _, sub := range subscriptions {
						if feed, exists := ctx.HytaleFeeds().Feeds[sub]; exists {
							subscriptionNames = append(subscriptionNames, feed.GetType().Display())
						}
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
		for feedType := range ctx.HytaleFeeds().Feeds {
			if err := ctx.DB().RemoveSubscription(feedType.ID(), ctx.User().ID); err != nil {
				log.Printf("Error removing subscription for %s: %v", feedType.ID(), err)
			}
		}
		ctx.Reply("Unsubscribed all feeds.")
	} else {
		var channelID string
		if channelOpt, exists := options["channel"]; exists {
			channelID = channelOpt.ChannelValue(nil).ID
		}

		if channelID != "" {
			for feedType := range ctx.HytaleFeeds().Feeds {
				if err := ctx.DB().RemoveSubscription(feedType.ID(), channelID); err != nil {
					log.Printf("Error removing subscription for %s: %v", feedType.ID(), err)
				}
			}
			channel := options["channel"].ChannelValue(nil)
			ctx.Reply("Unsubscribed all feeds from channel: " + channel.Mention())
		} else {
			channels, err := ctx.GuildChannels(guildID)
			if err != nil {
				ctx.ReplyError("Error while fetching guild channels", err)
				return
			}

			for _, channel := range channels {
				for feedType := range ctx.HytaleFeeds().Feeds {
					if err := ctx.DB().RemoveSubscription(feedType.ID(), channel.ID); err != nil {
						log.Printf("Error removing subscription for %s in channel %s: %v", feedType.ID(), channel.ID, err)
					}
				}
			}

			ctx.Reply("Unsubscribed all feeds in this guild.")
		}
	}
}
