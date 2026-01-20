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
			Name:  hytale.GameReleaseFeedDisplay,
			Value: hytale.GameReleaseFeedID,
		},
		{
			Name:  hytale.GamePreReleaseFeedDisplay,
			Value: hytale.GamePreReleaseFeedID,
		},
		{
			Name:  hytale.LauncherReleaseFeedDisplay,
			Value: hytale.LauncherReleaseFeedID,
		},
		{
			Name:  hytale.LauncherPostFeedDisplay,
			Value: hytale.LauncherPostFeedID,
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

func subscribeCommand(ctx *CommandContext) {
	options := ctx.Options()

	subType := options["type"].StringValue()
	channel := options["channel"].ChannelValue(nil)
	var role *discordgo.Role
	roleOption, exists := options["role"]
	if exists {
		role = roleOption.RoleValue(nil, "")
	}

	feed, exists := ctx.HytaleFeeds.Feeds[subType]
	if !exists {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	roles := []string{}
	if role != nil {
		roles = append(roles, role.ID)
	}
	err := ctx.DB.AddOrUpdateSubscription(feed.GetID(), channel.ID, db.GuildSubscription{
		Version: feed.GetVersion(),
		Roles:   roles,
	})
	if err != nil {
		ctx.ReplyError(fmt.Errorf("Error while trying to save subscription: %w", err))
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed %s to %s", channel.Mention(), feed.GetDisplayName()))
	}
}

func subscribeDMCommand(ctx *CommandContext) {
	options := ctx.Options()
	subType := options["type"].StringValue()

	feed, exists := ctx.HytaleFeeds.Feeds[subType]
	if !exists {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	err := ctx.DB.AddOrUpdateSubscription(feed.GetID(), ctx.User().ID, db.UserSubscription{
		Version: feed.GetVersion(),
	})
	if err != nil {
		ctx.ReplyError(fmt.Errorf("Error while trying to save subscription: %w", err))
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed to %s", feed.GetDisplayName()))
	}
}

func listCommand(ctx *CommandContext) {
	guildID := ctx.Interaction.GuildID
	if guildID == "" {
		userSubscriptions := make(map[string]string)
		for feedID := range ctx.HytaleFeeds.Feeds {
			targetIDs, err := ctx.DB.GetSubscriptions(feedID)
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feedID, err)
				continue
			}

			for _, targetID := range targetIDs {
				if targetID == ctx.User().ID {
					if feed, exists := ctx.HytaleFeeds.Feeds[feedID]; exists {
						userSubscriptions[feedID] = feed.GetDisplayName()
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
		channels, err := ctx.Session.GuildChannels(guildID)
		if err != nil {
			ctx.ReplyError(fmt.Errorf("Error while trying fetch guild channels: %w", err))
			return
		}

		sort.Slice(channels, func(i, j int) bool {
			return channels[i].Name < channels[j].Name
		})

		channelMap := make(map[string]*discordgo.Channel)
		for _, channel := range channels {
			channelMap[channel.ID] = channel
		}

		channelSubscriptions := make(map[string][]string)
		for feedID := range ctx.HytaleFeeds.Feeds {
			tagetIDs, err := ctx.DB.GetSubscriptions(feedID)
			if err != nil {
				log.Printf("Error fetching subscriptions for %s: %v", feedID, err)
				continue
			}

			for _, targetID := range tagetIDs {
				sub, err := ctx.DB.GetSubscription(feedID, targetID)
				if err != nil {
					log.Printf("Error fetching subscription for %s %s: %v", feedID, targetID, err)
				}
				_, ok := sub.(db.GuildSubscription)
				if ok {
					if channel, exists := channelMap[targetID]; exists {
						channelSubscriptions[channel.ID] = append(channelSubscriptions[channel.ID], feedID)
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
						if feed, exists := ctx.HytaleFeeds.Feeds[sub]; exists {
							subscriptionNames = append(subscriptionNames, feed.GetDisplayName())
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

func unsubscribeCommand(ctx *CommandContext) {
	guildID := ctx.Interaction.GuildID
	options := ctx.Options()

	if guildID == "" {
		for feedID := range ctx.HytaleFeeds.Feeds {
			if err := ctx.DB.RemoveSubscription(feedID, ctx.Interaction.User.ID); err != nil {
				log.Printf("Error removing subscription for %s: %v", feedID, err)
			}
		}
		ctx.Reply("Unsubscribed all feeds.")
	} else {
		var channelID string
		if channelOpt, exists := options["channel"]; exists {
			channel := channelOpt.ChannelValue(ctx.Session)
			channelID = channel.ID
		}

		if channelID != "" {
			for feedID := range ctx.HytaleFeeds.Feeds {
				if err := ctx.DB.RemoveSubscription(feedID, channelID); err != nil {
					log.Printf("Error removing subscription for %s: %v", feedID, err)
				}
			}
			channel := options["channel"].ChannelValue(ctx.Session)
			ctx.Reply("Unsubscribed all feeds from channel: " + channel.Mention())
		} else {
			channels, err := ctx.Session.GuildChannels(guildID)
			if err != nil {
				ctx.ReplyError(fmt.Errorf("Error while fetching guild channels: %w", err))
				return
			}

			for _, channel := range channels {
				for feedID := range ctx.HytaleFeeds.Feeds {
					if err := ctx.DB.RemoveSubscription(feedID, channel.ID); err != nil {
						log.Printf("Error removing subscription for %s in channel %s: %v", feedID, channel.ID, err)
					}
				}
			}

			ctx.Reply("Unsubscribed all feeds in this guild.")
		}
	}
}
