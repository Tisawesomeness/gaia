package main

import (
	"context"
	"log"
	"sort"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/valkey-io/valkey-go"
)

const (
	launcherRelease = "launcher_release"
	launcherFeed    = "launcher_feed"
)

func friendlyFeedName(feedId string) string {
	switch feedId {
	case launcherRelease:
		return "New Versions"
	case launcherFeed:
		return "Launcher Posts"
	default:
		return feedId
	}
}

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
						Name:  friendlyFeedName(launcherRelease),
						Value: launcherRelease,
					},
					{
						Name:  friendlyFeedName(launcherFeed),
						Value: launcherFeed,
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
		Description: "List all subscriptions for channels in your guild",
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

	err := addSubscription(ctx.valkeyClient, subType, channel.ID, ctx.hytale.LauncherRelease.Version)
	var response discordgo.InteractionResponseData
	if err != nil {
		log.Printf("Error saving subscription to Valkey: %v", err)
		response = discordgo.InteractionResponseData{
			Content: "An error occurred while trying to save your subscription",
			Flags:   discordgo.MessageFlagsEphemeral,
		}
	} else {
		response = discordgo.InteractionResponseData{
			Content: "Subscribed to " + friendlyFeedName(subType) + " channel: " + channel.Mention(),
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &response,
	})
}

func listCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	guildID := i.GuildID
	if guildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command can only be used in a guild.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		log.Printf("Error fetching guild channels: %v", err)
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "An error occurred while fetching guild channels.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
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

	feedTypes := []string{launcherRelease, launcherFeed}
	channelSubscriptions := make(map[string][]string)

	for _, feedType := range feedTypes {
		members, err := getSubscriptions(ctx.valkeyClient, feedType)
		if err != nil {
			log.Printf("Error fetching subscriptions for %s: %v", feedType, err)
			continue
		}

		for member := range members {
			if channel, exists := channelMap[member]; exists {
				channelSubscriptions[channel.ID] = append(channelSubscriptions[channel.ID], feedType)
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
					subscriptionNames[i] = friendlyFeedName(sub)
				}
				description = append(description, "- "+channel.Mention()+" - **"+strings.Join(subscriptionNames, ", ")+"**")
			}
		}
		embed.Description = strings.Join(description, "\n")
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}

func unsubscribeCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	guildID := i.GuildID
	if guildID == "" {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "This command can only be used in a guild.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
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

	feedTypes := []string{launcherRelease, launcherFeed}
	var response discordgo.InteractionResponseData

	if channelID != "" {
		// Unsubscribe specific channel from all feeds
		for _, feedType := range feedTypes {
			if err := removeSubscription(ctx.valkeyClient, feedType, channelID); err != nil {
				log.Printf("Error removing subscription for %s: %v", feedType, err)
			}
		}
		channel := optionMap["channel"].ChannelValue(s)
		response = discordgo.InteractionResponseData{
			Content: "Unsubscribed all feeds from channel: " + channel.Mention(),
		}
	} else {
		// Unsubscribe all channels in the guild from all feeds
		channels, err := s.GuildChannels(guildID)
		if err != nil {
			log.Printf("Error fetching guild channels: %v", err)
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "An error occurred while fetching guild channels.",
					Flags:   discordgo.MessageFlagsEphemeral,
				},
			})
			return
		}

		for _, channel := range channels {
			for _, feedType := range feedTypes {
				if err := removeSubscription(ctx.valkeyClient, feedType, channel.ID); err != nil {
					log.Printf("Error removing subscription for %s in channel %s: %v", feedType, channel.ID, err)
				}
			}
		}

		response = discordgo.InteractionResponseData{
			Content: "Unsubscribed all feeds in this guild.",
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &response,
	})
}

func notifyLauncherReleaseFeeds(s *discordgo.Session, client *valkey.Client, api *HytaleAPI) error {
	subs, err := getSubscriptions(client, launcherRelease)
	if err != nil {
		return err
	}
	for channelId, lastKnownVersion := range subs {
		if lastKnownVersion != api.LauncherRelease.Version {
			_, err = s.Channel(channelId)
			if err != nil {
				log.Printf("Error accessing channel, removing: %v", err)
				removeSubscription(client, launcherRelease, channelId)
			} else {
				_, err = s.ChannelMessageSend(channelId, "new version: "+api.LauncherRelease.Version)
				if err != nil {
					log.Printf("Cannot send feed update: %v", err)
				}
			}
		}
	}
	return nil
}

func addSubscription(client *valkey.Client, subType string, channelId string, currentVersion string) error {
	command := (*client).B().Hset().Key(subType+":subs").FieldValue().FieldValue(channelId, currentVersion).Build()
	return (*client).Do(context.Background(), command).Error()
}

// Returns a mapping of channel ID to last notified version
func getSubscriptions(client *valkey.Client, subType string) (map[string]string, error) {
	command := (*client).B().Hgetall().Key(subType + ":subs").Build()
	return (*client).Do(context.Background(), command).AsStrMap()
}

func removeSubscription(client *valkey.Client, subType string, channelId string) error {
	command := (*client).B().Hdel().Key(subType + ":subs").Field(channelId).Build()
	return (*client).Do(context.Background(), command).Error()
}
