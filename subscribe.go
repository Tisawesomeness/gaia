package main

import (
	"context"
	"log"
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
)

func subscribeCommand(i *discordgo.InteractionCreate, s *discordgo.Session, client *valkey.Client) {
	options := i.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}

	subType := optionMap["type"].StringValue()
	channel := optionMap["channel"].ChannelValue(s)

	err := addSubscription(client, subType, channel.ID)
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

func listCommand(i *discordgo.InteractionCreate, s *discordgo.Session, client *valkey.Client) {
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

	channelMap := make(map[string]*discordgo.Channel)
	for _, channel := range channels {
		channelMap[channel.ID] = channel
	}

	feedTypes := []string{launcherRelease, launcherFeed}
	channelSubscriptions := make(map[string][]string)

	for _, feedType := range feedTypes {
		members, err := getSubscriptions(client, feedType)
		if err != nil {
			log.Printf("Error fetching subscriptions for %s: %v", feedType, err)
			continue
		}

		for _, member := range members {
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

func addSubscription(client *valkey.Client, subType string, channelId string) error {
	return (*client).Do(context.Background(), (*client).B().Sadd().Key(subType+":subs").Member(channelId).Build()).Error()
}

func getSubscriptions(client *valkey.Client, subType string) ([]string, error) {
	return (*client).Do(context.Background(), (*client).B().Smembers().Key(subType+":subs").Build()).AsStrSlice()
}
