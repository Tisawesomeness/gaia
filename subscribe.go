package main

import (
	"context"
	"log"

	"github.com/bwmarrin/discordgo"
	"github.com/valkey-io/valkey-go"
)

var (
	SubscribeCommand = &discordgo.ApplicationCommand{
		Name:        "subscribe",
		Description: "Subscribe to Hytale updates",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        "type",
				Description: "Type of subscription",
				Required:    true,
				Choices: []*discordgo.ApplicationCommandOptionChoice{
					{
						Name:  "Server",
						Value: "server",
					},
					{
						Name:  "Client",
						Value: "client",
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
			Content: "Subscribed to " + subType + " channel: " + channel.Mention(),
		}
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &response,
	})
}

func addSubscription(client *valkey.Client, subType string, channelId string) error {
	return (*client).Do(context.Background(), (*client).B().Sadd().Key("subscription:"+subType).Member(channelId).Build()).Error()
}
