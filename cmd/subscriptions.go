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

func subscribeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	subType := optionMap["type"].StringValue()
	channel := optionMap["channel"].ChannelValue(s)

	// Get the feed from the HytaleFeeds map
	feed, exists := ctx.HytaleFeeds.Feeds[subType]
	if !exists {
		ctx.ReplyWarn(s, i, "Invalid feed type.")
		return
	}

	err := ctx.DB.AddOrUpdateSubscription(feed.GetID(), channel.ID, feed.GetVersion())
	if err != nil {
		ctx.ReplyError(s, i, fmt.Errorf("Error while trying to save subscription: %w", err))
	} else {
		ctx.Reply(s, i, fmt.Sprintf("Subscribed %s to %s", channel.Mention(), feed.GetDisplayName()))
	}
}

func listCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	guildID := i.GuildID
	if guildID == "" {
		ctx.ReplyWarn(s, i, "This command can only be used in a guild.")
		return
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		ctx.ReplyError(s, i, fmt.Errorf("Error while trying fetch guild channels: %w", err))
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

	ctx.ReplyEmbed(s, i, embed)
}

func unsubscribeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	guildID := i.GuildID
	if guildID == "" {
		ctx.ReplyWarn(s, i, "This command can only be used in a guild.")
		return
	}

	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	var channelID string
	if channelOpt, exists := optionMap["channel"]; exists {
		channel := channelOpt.ChannelValue(s)
		channelID = channel.ID
	}

	if channelID != "" {
		// Unsubscribe specific channel from all feeds
		for feedID, _ := range ctx.HytaleFeeds.Feeds {
			if err := ctx.DB.RemoveSubscription(feedID, channelID); err != nil {
				log.Printf("Error removing subscription for %s: %v", feedID, err)
			}
		}
		channel := optionMap["channel"].ChannelValue(s)
		ctx.Reply(s, i, "Unsubscribed all feeds from channel: "+channel.Mention())
	} else {
		// Unsubscribe all channels in the guild from all feeds
		channels, err := s.GuildChannels(guildID)
		if err != nil {
			ctx.ReplyError(s, i, fmt.Errorf("Error while fetching guild channels: %w", err))
			return
		}

		for _, channel := range channels {
			for feedID, _ := range ctx.HytaleFeeds.Feeds {
				if err := ctx.DB.RemoveSubscription(feedID, channel.ID); err != nil {
					log.Printf("Error removing subscription for %s in channel %s: %v", feedID, channel.ID, err)
				}
			}
		}

		ctx.Reply(s, i, "Unsubscribed all feeds in this guild.")
	}
}
