package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

var (
	patchlineOptionVersions = &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "patchline",
		Description: "The release channel",
	}

	VersionCommand = &discordgo.ApplicationCommand{
		Name:        "version",
		Description: "Get the latest Hytale versions",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "server",
				Description: "List Hytale server versions",
				Options:     []*discordgo.ApplicationCommandOption{patchlineOptionVersions},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "client",
				Description: "Get the latest Hytale client version",
				Options:     []*discordgo.ApplicationCommandOption{patchlineOptionVersions},
			},
		},
	}

	LauncherCommand = &discordgo.ApplicationCommand{
		Name:        "launcher",
		Description: "Get the latest Hytale Launcher version",
	}

	ArticlesCommand = &discordgo.ApplicationCommand{
		Name:        "articles",
		Description: "Get the latest Hytale article",
	}
)

func displayPatchlineList(patchlines map[string]*time.Time) string {
	var parts []string
	for patchline, _ := range patchlines {
		parts = append(parts, fmt.Sprintf("`%s`", patchline))
	}
	return strings.Join(parts, ", ")
}

func versionCommand(ctx CommandContext) {
	options := ctx.Options()

	var side hytale.Side
	if _, exists := options["client"]; exists {
		side = hytale.Client
	} else if _, exists := options["server"]; exists {
		side = hytale.Server
	} else {
		ctx.ReplyWarn("Invalid side")
		return
	}

	option, exists := options["patchline"]
	patchlineInput := "release"
	if exists {
		patchlineInput = option.StringValue()
	}

	var patchline string
	if patchlineInput == "release" {
		patchline = "release"
	} else {
		patchlineFeed, exists := ctx.HytaleFeeds().GetPatchlinesFeed()
		if !exists {
			ctx.ReplyError("Could not retrieve Hytale patchlines.", nil)
			return
		}
		patchline = hytale.ClosestPatchline(patchlineInput, patchlineFeed.Patchlines)
		if patchline == "" {
			ctx.ReplyWarn("Patchline must be one of: " + displayPatchlineList(patchlineFeed.Patchlines))
			return
		}
	}

	var feed hytale.Feed
	if side == hytale.Client {
		feed, exists = ctx.HytaleFeeds().GetGameFeed(patchline)
	} else {
		feed, exists = ctx.HytaleFeeds().GetMavenFeed(patchline)
	}
	if !exists {
		ctx.ReplyError("Could not retrieve the latest Hytale version.", nil)
		return
	}

	message := feed.BuildMessage(ctx.Config())
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

func launcherCommand(ctx CommandContext) {
	feed, exists := ctx.HytaleFeeds().GetLauncherReleaseFeed()
	if !exists {
		ctx.ReplyError("Could not retrieve the latest Hytale Launcher version.", nil)
		return
	}

	message := feed.BuildMessage(ctx.Config())
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

// Article browsing session state
type ArticleState struct {
	CurrentIndex int
	Articles     []*hytale.Article
}

func articlesCommand(ctx CommandContext) {
	feed, exists := ctx.HytaleFeeds().GetLauncherPostFeed()
	if !exists {
		ctx.ReplyError("Could not retrieve the latest Hytale article.", nil)
		return
	}

	articles := feed.Articles.Articles
	if len(articles) == 0 {
		ctx.ReplyEphemeral("No articles found.")
		return
	}

	interactionID := ctx.InteractionID()
	ctx.NewInteraction(interactionID, &ArticleState{
		CurrentIndex: 0,
		Articles:     articles,
	})

	latestArticle := articles[0]
	message := latestArticle.BuildMessage(ctx.Config())

	customID := CustomID{
		InteractionType: "article",
		Action:          "",
		SessionID:       interactionID,
	}

	// Note: going *back* in time means going *forward* in the articles array
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "Back",
			Style:    discordgo.PrimaryButton,
			CustomID: customID.WithAction("back").String(),
			Disabled: len(articles) <= 1, // Disable if there's only one article
		},
		discordgo.Button{
			Label:    "Forward",
			Style:    discordgo.PrimaryButton,
			CustomID: customID.WithAction("forward").String(),
			Disabled: true, // Disable if it's the first article
		},
	}

	paginateMessage(message, buttons)
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

func handleArticleButton(ctx CommandContext, state any) {
	interaction := state.(*ArticleState)
	customID := ctx.CustomID()
	switch customID.Action {
	case "back":
		if interaction.CurrentIndex == len(interaction.Articles)-1 {
			return // ignore invalid button press
		}
		interaction.CurrentIndex++
	case "forward":
		if interaction.CurrentIndex == 0 {
			return // ignore invalid button press
		}
		interaction.CurrentIndex--
	default:
		return // ignore unknown action
	}

	currentArticle := interaction.Articles[interaction.CurrentIndex]

	message := currentArticle.BuildMessage(ctx.Config())

	// Note: going *back* in time means going *forward* in the articles array
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "Back",
			Style:    discordgo.PrimaryButton,
			CustomID: customID.WithAction("back").String(),
			Disabled: interaction.CurrentIndex == len(interaction.Articles)-1,
		},
		discordgo.Button{
			Label:    "Forward",
			Style:    discordgo.PrimaryButton,
			CustomID: customID.WithAction("forward").String(),
			Disabled: interaction.CurrentIndex == 0,
		},
	}

	// Edit the original message
	paginateMessage(message, buttons)
	ctx.Edit(&discordgo.InteractionResponseData{
		Embeds:          message.Embeds,
		Components:      message.Components,
		AllowedMentions: &discordgo.MessageAllowedMentions{},
	})
}

// Mutates message
func paginateMessage(message *hytale.FeedMessage, buttons []discordgo.MessageComponent) {
	if len(message.Components) == 0 {
		message.Components = []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: buttons,
			},
		}
	} else if actionsRow, ok := message.Components[0].(discordgo.ActionsRow); ok {
		message.Components[0] = discordgo.ActionsRow{
			Components: append(buttons, actionsRow.Components...),
		}
	} else {
		message.Components = append([]discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: buttons,
			},
		}, message.Components...)
	}
}
