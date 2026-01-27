package cmd

import (
	"errors"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

// Article browsing session state
type ArticleInteraction struct {
	CurrentIndex int
	Articles     []*hytale.Article
	LastUsed     time.Time
}

var articleInteractions = make(map[string]*ArticleInteraction)

const (
	CleanupInterval   = 5 * time.Minute
	InteractionExpiry = 30 * time.Minute
)

var (
	VersionCommand = &discordgo.ApplicationCommand{
		Name:        "version",
		Description: "Get the latest Hytale version",
		Options: []*discordgo.ApplicationCommandOption{
			{
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

func StartArticleInteractionCleanup() {
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

	feed, exists := ctx.HytaleFeeds.Feeds[patchline.FeedType()]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale version."))
		return
	}

	gameReleaseFeed, ok := feed.(hytale.GameReleaseFeed)
	if !ok {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale version."))
		return
	}

	ctx.ReplyEmbed(gameReleaseFeed.BuildMessage(ctx.Config, false))
}

func launcherCommand(ctx *CommandContext) {
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherReleaseFeedType]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale Launcher version."))
		return
	}

	launcherReleaseFeed, ok := feed.(hytale.LauncherReleaseFeed)
	if !ok {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale Launcher version."))
		return
	}

	ctx.ReplyEmbed(launcherReleaseFeed.BuildMessage(ctx.Config, false))
}

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
	embed := latestArticle.BuildMessage(ctx.Config)

	// Note: going *back* in time means going *forward* in the articles array
	backButton := discordgo.Button{
		Label:    "Back",
		Style:    discordgo.SecondaryButton,
		CustomID: "article_back_" + interactionID,
		Disabled: len(articles) <= 1, // Disable if there's only one article
	}

	forwardButton := discordgo.Button{
		Label:    "Forward",
		Style:    discordgo.SecondaryButton,
		CustomID: "article_forward_" + interactionID,
		Disabled: true, // Disable if it's the first article
	}

	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{backButton, forwardButton},
			},
		},
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

	embed := currentArticle.BuildMessage(ctx.Config)

	// Note: going *back* in time means going *forward* in the articles array
	backButton := discordgo.Button{
		Label:    "Back",
		Style:    discordgo.SecondaryButton,
		CustomID: "article_back_" + interactionID,
		Disabled: interaction.CurrentIndex == len(interaction.Articles)-1,
	}

	forwardButton := discordgo.Button{
		Label:    "Forward",
		Style:    discordgo.SecondaryButton,
		CustomID: "article_forward_" + interactionID,
		Disabled: interaction.CurrentIndex == 0,
	}

	// Edit the original message
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{backButton, forwardButton},
				},
			},
			AllowedMentions: &discordgo.MessageAllowedMentions{},
		},
	})
}
