package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/bwmarrin/discordgo"
)

const (
	websiteOriginal    = "https://tis.codes/gaia"
	helpServerOriginal = "https://minecord.github.io/support"
	githubOriginal     = "https://github.com/Tisawesomeness/gaia"
	donateOriginal     = "https://ko-fi.com/tis_awesomeness"
)

var (
	HelpCommand = &discordgo.ApplicationCommand{
		Name:        "help",
		Description: "List all commands",
	}
	CreditsCommand = &discordgo.ApplicationCommand{
		Name:        "credits",
		Description: "See who made the bot possible",
	}

	developersValue = "[Tis](https://tis.codes) - Main Dev"
	apisValue       = "Discord API Wrapper - [discordgo](https://github.com/bwmarrin/discordgo)\n" +
		"Hytale APIs - [Hytale Team](https://hytale.com/)"
	donateLine = fmt.Sprintf(":sparkles: **[Donate](%s)** :sparkles: to support development", donateOriginal)
)

func helpCommand(ctx *CommandContext) {
	fields := []*discordgo.MessageEmbedField{}
	for _, category := range categories {
		var description strings.Builder
		for _, command := range category.commands {
			if command.discord.Contexts == nil || slices.Contains(*command.discord.Contexts, ctx.Interaction.Context) {
				fmt.Fprintf(&description, "`/%s` - %s\n", command.discord.Name, command.discord.Description)
			}
		}
		field := &discordgo.MessageEmbedField{
			Name:   category.name,
			Value:  description.String(),
			Inline: false,
		}
		fields = append(fields, field)
	}

	embed := &discordgo.MessageEmbed{
		Title:  "Gaia Help",
		Color:  0x0000ff,
		Fields: fields,
	}

	ctx.ReplyEmbed(embed)
}

func creditsCommand(ctx *CommandContext) {
	embed := &discordgo.MessageEmbed{
		Title:       "Gaia Credits",
		Description: "Thanks to all these great people who helped to make the bot possible. :heart:",
		Color:       0x00FF00,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Developers",
				Value:  developersValue,
				Inline: true,
			},
			{
				Name:   "Contribute",
				Value:  buildContributeString(ctx.Config),
				Inline: true,
			},
			{
				Name:   "APIs Used",
				Value:  apisValue,
				Inline: false,
			},
			buildHostingField(ctx.Config),
			{
				Name:   "Special Thanks",
				Value:  fmt.Sprintf("%s for using Gaia!", ctx.User().Mention()),
				Inline: false,
			},
		},
	}
	ctx.ReplyEmbed(embed)
}

func buildContributeString(config *config.Config) string {
	base := fmt.Sprintf("[Contribute on GitHub](%s)\n[Suggest features here](%s)", config.Branding.Github, config.Branding.Issues)
	if config.IsSelfHosted {
		return base
	} else {
		return fmt.Sprintf("%s\n%s", donateLine, base)
	}
}

func buildHostingField(config *config.Config) *discordgo.MessageEmbedField {
	if config.IsSelfHosted {
		return &discordgo.MessageEmbedField{
			Name: "Self-Hosting",
			Value: fmt.Sprintf("This bot is self-hosted by **%s**\nOriginal [Website](%s) [Help Server](%s) [GitHub](%s)\n%s",
				config.Branding.Author,
				websiteOriginal,
				helpServerOriginal,
				githubOriginal,
				donateLine),
			Inline: false,
		}
	} else {
		return &discordgo.MessageEmbedField{
			Name: "Hosting",
			Value: fmt.Sprintf("The public bot is proudly hosted by [%s](%s)",
				config.Branding.HostingProvider,
				config.Branding.HostingWebsite),
			Inline: false,
		}
	}
}
