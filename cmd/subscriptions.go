package cmd

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	SubscribeCommand = &discordgo.ApplicationCommand{
		Name:        "subscribe",
		Description: "Subscribe to Hytale updates",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "What to subscribe to",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  hytale.LauncherReleaseFeedDisplay,
						Value: hytale.LauncherReleaseFeedID,
					},
					{
						Name:  hytale.LauncherPostFeedDisplay,
						Value: hytale.LauncherPostFeedID,
					},
				},
			},
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Channel where updates will be posted",
				Required:    true,
			},
		},
	}

	ListCommand = &discordgo.ApplicationCommand{
		Name:        "list-subscriptions",
		Description: "List all subscriptions",
	}

	UnsubscribeCommand = &discordgo.ApplicationCommand{
		Name:        "unsubscribe",
		Description: "Unsubscribe from Hytale updates",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionChannel,
				Name:        "channel",
				Description: "Which channel to unsubscribe (defaults to all channels)",
				Required:    false,
			},
		},
	}
)

func subscribeCommand(ctx *CommandContext) {
	options := ctx.Options()

	subType := options["type"].StringValue()
	channel := options["channel"].ChannelValue(ctx.Session)

	// Get the feed from the HytaleFeeds map
	feed, exists := ctx.HytaleFeeds.Feeds[subType]
	if !exists {
		ctx.ReplyWarn("Invalid feed type.")
		return
	}

	err := ctx.DB.AddOrUpdateSubscription(feed.GetID(), channel.ID, feed.GetVersion())
	if err != nil {
		ctx.ReplyError(fmt.Errorf("Error while trying to save subscription: %w", err))
	} else {
		ctx.Reply(fmt.Sprintf("Subscribed %s to %s", channel.Mention(), feed.GetDisplayName()))
	}
}

func listCommand(ctx *CommandContext) {
	guildID := ctx.Interaction.GuildID
	if guildID == "" {
		ctx.ReplyWarn("This command can only be used in a guild.")
		return
	}

	channels, err := ctx.Session.GuildChannels(guildID)
	if err != nil {
		ctx.ReplyError(fmt.Errorf("Error while trying fetch guild channels: %w", err))
		return
	}

	// Sort channels by name
	sort.Slice(channels, func(i, j int) bool {
		return channels[i].Name < channels[j].Name
	})

	channelMap := make(map[string]*discordgo.Channel)
	for _, channel := range channels {
		channelMap[channel.ID] = channel
	}

	channelSubscriptions := make(map[string][]string)
	for feedID := range ctx.HytaleFeeds.Feeds {
		members, err := ctx.DB.GetSubscriptions(feedID)
		if err != nil {
			log.Printf("Error fetching subscriptions for %s: %v", feedID, err)
			continue
		}

		for member := range members {
			if channel, exists := channelMap[member]; exists {
				channelSubscriptions[channel.ID] = append(channelSubscriptions[channel.ID], feedID)
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
				subscriptionNames := make([]string, len(subscriptions))
				for i, sub := range subscriptions {
					if feed, exists := ctx.HytaleFeeds.Feeds[sub]; exists {
						subscriptionNames[i] = feed.GetDisplayName()
					}
				}
				description = append(description, "- "+channel.Mention()+" - **"+strings.Join(subscriptionNames, ", ")+"**")
			}
		}
		embed.Description = strings.Join(description, "\n")
	}

	ctx.ReplyEmbed(embed)
}

func unsubscribeCommand(ctx *CommandContext) {
	guildID := ctx.Interaction.GuildID
	if guildID == "" {
		ctx.ReplyWarn("This command can only be used in a guild.")
		return
	}

	options := ctx.Options()

	var channelID string
	if channelOpt, exists := options["channel"]; exists {
		channel := channelOpt.ChannelValue(ctx.Session)
		channelID = channel.ID
	}

	if channelID != "" {
		// Unsubscribe specific channel from all feeds
		for feedID, _ := range ctx.HytaleFeeds.Feeds {
			if err := ctx.DB.RemoveSubscription(feedID, channelID); err != nil {
				log.Printf("Error removing subscription for %s: %v", feedID, err)
			}
		}
		channel := options["channel"].ChannelValue(ctx.Session)
		ctx.Reply("Unsubscribed all feeds from channel: " + channel.Mention())
	} else {
		// Unsubscribe all channels in the guild from all feeds
		channels, err := ctx.Session.GuildChannels(guildID)
		if err != nil {
			ctx.ReplyError(fmt.Errorf("Error while fetching guild channels: %w", err))
			return
		}

		for _, channel := range channels {
			for feedID, _ := range ctx.HytaleFeeds.Feeds {
				if err := ctx.DB.RemoveSubscription(feedID, channel.ID); err != nil {
					log.Printf("Error removing subscription for %s in channel %s: %v", feedID, channel.ID, err)
				}
			}
		}

		ctx.Reply("Unsubscribed all feeds in this guild.")
	}
}
