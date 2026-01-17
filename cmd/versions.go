package cmd

import (
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	VersionCommand = &discordgo.ApplicationCommand{
		Name:        "version",
		Description: "Get the latest Hytale version",
	}

	ArticlesCommand = &discordgo.ApplicationCommand{
		Name:        "articles",
		Description: "Get the latest Hytale article",
	}
)

func versionCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	// Get the launcher release feed
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherReleaseFeedID]
	if !exists {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Could not retrieve the latest Hytale version.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Get the version from the feed
	launcherReleaseFeed, ok := feed.(*hytale.LauncherReleaseFeed)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Could not retrieve the latest Hytale version.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Respond with the embed
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{launcherReleaseFeed.BuildMessage(s, ctx.Config)},
		},
	})
}

func articlesCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	// Get the launcher post feed
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherPostFeedID]
	if !exists {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Could not retrieve the latest Hytale article.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Get the articles from the feed
	launcherPostFeed, ok := feed.(*hytale.LauncherPostFeed)
	if !ok {
		s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Could not retrieve the latest Hytale article.",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	// Respond with the embed
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{launcherPostFeed.BuildMessage(s, ctx.Config)},
		},
	})
}
