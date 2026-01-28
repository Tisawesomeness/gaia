package cmd

import (
	"errors"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

const (
	CleanupInterval   = 5 * time.Minute
	InteractionExpiry = 30 * time.Minute
)

var (
	patchlineOption = &discordgo.ApplicationCommandOption{
		Type:        discordgo.ApplicationCommandOptionString,
		Name:        "patchline",
		Description: "The release channel",
		Choices: []*discordgo.ApplicationCommandOptionChoice{
			{
				Name:  hytale.Release.Display(),
				Value: hytale.Release.ID(),
			},
			{
				Name:  hytale.PreRelease.Display(),
				Value: hytale.PreRelease.ID(),
			},
		},
	}

	VersionCommand = &discordgo.ApplicationCommand{
		Name:        "version",
		Description: "Get the latest Hytale versions",
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "server",
				Description: "List Hytale server versions",
				Options:     []*discordgo.ApplicationCommandOption{patchlineOption},
			},
			{
				Type:        discordgo.ApplicationCommandOptionSubCommand,
				Name:        "client",
				Description: "Get the latest Hytale client version",
				Options:     []*discordgo.ApplicationCommandOption{patchlineOption},
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

func StartInteractionCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	go func() {
		for range ticker.C {
			cleanupOldInteractions()
		}
	}()
}

func cleanupOldInteractions() {
	now := time.Now()
	for id, interaction := range articleInteractions {
		if now.Sub(interaction.LastUsed) > InteractionExpiry {
			delete(articleInteractions, id)
		}
	}
}

func versionCommand(ctx *CommandContext) {
	options := ctx.Options()

	var side hytale.Side
	if _, exists := options["client"]; exists {
		side = hytale.Client
	} else if _, exists := options["server"]; exists {
		side = hytale.Server
	} else {
		ctx.ReplyWarn("Invalid side")
	}

	option, exists := options["patchline"]
	var patchlineValue string
	if exists {
		patchlineValue = option.StringValue()
	} else {
		patchlineValue = "release"
	}

	patchline, err := hytale.ParsePatchline(patchlineValue)
	if err != nil {
		ctx.ReplyWarn("Invalid patchline")
	}

	feed, exists := ctx.HytaleFeeds.Feeds[hytale.GetFeedType(patchline, side)]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale version."))
		return
	}

	message := feed.BuildMessage(ctx.Config, false)
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

func launcherCommand(ctx *CommandContext) {
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherReleaseFeedType]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale Launcher version."))
		return
	}

	message := feed.BuildMessage(ctx.Config, false)
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

// Article browsing session state
type ArticleInteraction struct {
	CurrentIndex int
	Articles     []*hytale.Article
	LastUsed     time.Time
}

var articleInteractions = make(map[string]*ArticleInteraction)

func articlesCommand(ctx *CommandContext) {
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherPostFeedType]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale article."))
		return
	}

	launcherPostFeed, ok := feed.(hytale.LauncherPostFeed)
	if !ok {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale article."))
		return
	}
	articles := launcherPostFeed.Articles.Articles
	if len(articles) == 0 {
		ctx.ReplyEphemeral("No articles found.")
		return
	}

	interactionID := ctx.Interaction.Interaction.ID
	articleInteractions[interactionID] = &ArticleInteraction{
		CurrentIndex: 0,
		Articles:     articles,
		LastUsed:     time.Now(),
	}

	latestArticle := articles[0]
	message := latestArticle.BuildMessage(ctx.Config)

	// Note: going *back* in time means going *forward* in the articles array
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "Back",
			Style:    discordgo.PrimaryButton,
			CustomID: "article_back_" + interactionID,
			Disabled: len(articles) <= 1, // Disable if there's only one article
		},
		discordgo.Button{
			Label:    "Forward",
			Style:    discordgo.PrimaryButton,
			CustomID: "article_forward_" + interactionID,
			Disabled: true, // Disable if it's the first article
		},
	}

	paginateMessage(message, buttons)
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds:     message.Embeds,
		Components: message.Components,
	})
}

func isArticleInteraction(customID string) bool {
	return strings.HasPrefix(customID, "article_back_") || strings.HasPrefix(customID, "article_forward_")
}

func HandleArticleButton(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	customID := i.MessageComponentData().CustomID
	interactionID := strings.TrimPrefix(customID, "article_back_")
	interactionID = strings.TrimPrefix(interactionID, "article_forward_")

	interaction, exists := articleInteractions[interactionID]
	if !exists {
		ctx.ReplyEphemeral("This interaction has expired.")
		return
	}

	interaction.LastUsed = time.Now()

	if strings.HasPrefix(customID, "article_back_") {
		interaction.CurrentIndex++
	} else if strings.HasPrefix(customID, "article_forward_") {
		interaction.CurrentIndex--
	}

	currentArticle := interaction.Articles[interaction.CurrentIndex]

	message := currentArticle.BuildMessage(ctx.Config)

	// Note: going *back* in time means going *forward* in the articles array
	buttons := []discordgo.MessageComponent{
		discordgo.Button{
			Label:    "Back",
			Style:    discordgo.PrimaryButton,
			CustomID: "article_back_" + interactionID,
			Disabled: interaction.CurrentIndex == len(interaction.Articles)-1,
		},
		discordgo.Button{
			Label:    "Forward",
			Style:    discordgo.PrimaryButton,
			CustomID: "article_forward_" + interactionID,
			Disabled: interaction.CurrentIndex == 0,
		},
	}

	// Edit the original message
	paginateMessage(message, buttons)
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:          message.Embeds,
			Components:      message.Components,
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
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
