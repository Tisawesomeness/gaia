package cmd

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/bwmarrin/discordgo"
)

const (
	websiteOriginal    = "https://tis.codes/gaia"
	helpServerOriginal = "https://tis.codes/gaia-support"
	githubOriginal     = "https://github.com/Tisawesomeness/gaia"
	donateOriginal     = "https://ko-fi.com/tis_awesomeness"
)

var (
	HelpCommand = &discordgo.ApplicationCommand{
		Name:        "help",
		Description: "List all commands",
	}
	InfoCommand = &discordgo.ApplicationCommand{
		Name:        "info",
		Description: "Get information and stats about the bot",
	}
	CreditsCommand = &discordgo.ApplicationCommand{
		Name:        "credits",
		Description: "See who made the bot possible",
	}

	developersValue = "[Tis](https://tis.codes) - Main Dev"
	apisValue       = "Discord API Wrapper - [discordgo](https://github.com/bwmarrin/discordgo)\n" +
		"Hytale APIs - [Hytale Team](https://hytale.com/)\n" +
		"Skin Renders - [Hyvatar](https://hyvatar.io/)"
	donateLine = fmt.Sprintf(":sparkles: **[Donate](%s)** :sparkles: to support development", donateOriginal)
)

func helpCommand(ctx *CommandContext) {
	fields := []*discordgo.MessageEmbedField{}
	for _, category := range categories {
		var description strings.Builder
		for _, command := range category.commands {
			if userCanExecute(ctx, command.discord) {
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

func userCanExecute(ctx *CommandContext, command *discordgo.ApplicationCommand) bool {
	if command.Contexts != nil && !slices.Contains(*command.Contexts, ctx.Interaction.Context) {
		return false
	}
	if command.DefaultMemberPermissions == nil {
		return true
	}
	if ctx.Interaction.Member != nil {
		return (ctx.Interaction.Member.Permissions & *command.DefaultMemberPermissions) == *command.DefaultMemberPermissions
	} else {
		return true
	}
}

func infoCommand(ctx *CommandContext) {
	fields := []*discordgo.MessageEmbedField{
		{
			Name:   "Author",
			Value:  "[Tis](https://tis.codes)",
			Inline: true,
		},
	}

	if ctx.Config.IsSelfHosted {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Self-Hoster",
			Value:  fmt.Sprintf("%s", ctx.Config.Branding.Author),
			Inline: true,
		})
	}

	fields = append(fields, &discordgo.MessageEmbedField{
		Name:   "Version",
		Value:  fmt.Sprintf("`%s`", ctx.BotMetadata.Version),
		Inline: true,
	})

	if !ctx.Config.IsSelfHosted {
		fields = append(fields, &discordgo.MessageEmbedField{
			Name:   "Shard",
			Value:  "1/1",
			Inline: true,
		})
	}

	fields = append(fields, []*discordgo.MessageEmbedField{
		{
			Name:   "Guilds",
			Value:  fmt.Sprintf("%d", len(ctx.Session.State.Guilds)),
			Inline: true,
		},
		{
			Name:   "Uptime",
			Value:  formatUptime(*ctx.BotMetadata.BootTime),
			Inline: true,
		},
		{
			Name:   "Ping",
			Value:  fmt.Sprintf("%dms", ctx.Session.HeartbeatLatency().Milliseconds()),
			Inline: true,
		},
		{
			Name: "Links",
			Value: fmt.Sprintf("[INVITE](%s) | [SUPPORT](%s) | [WEBSITE](%s) | [GITHUB](%s)",
				ctx.Config.Branding.Invite,
				ctx.Config.Branding.HelpServer,
				ctx.Config.Branding.Website,
				ctx.Config.Branding.Github),
			Inline: false,
		},
	}...)

	if !ctx.Config.IsSelfHosted {
		fields = append(fields, []*discordgo.MessageEmbedField{
			{
				Name: "Legal",
				Value: fmt.Sprintf("[TERMS](%s) | [PRIVACY](%s)",
					ctx.Config.Branding.Terms,
					ctx.Config.Branding.Privacy),
				Inline: false,
			},
			{
				Name:   "Donate",
				Value:  donateLine,
				Inline: false,
			},
		}...)
	}

	embed := &discordgo.MessageEmbed{
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
		Footer: &discordgo.MessageEmbedFooter{
			Text: "Not affiliated with Hytale or Hypixel Studios.",
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

func formatUptime(bootTime time.Time) string {
	duration := time.Since(bootTime)
	days := int(duration.Hours() / 24)
	hours := int(duration.Hours()) % 24
	minutes := int(duration.Minutes()) % 60
	seconds := int(duration.Seconds()) % 60

	var parts []string
	if int(duration.Hours()/24) > 0 {
		parts = append(parts, fmt.Sprintf("%dd", days))
	}
	if int(duration.Hours()) > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if int(duration.Minutes()) > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	parts = append(parts, fmt.Sprintf("%ds", seconds))

	return strings.Join(parts, "")
}
