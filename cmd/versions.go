package cmd

import (
	"errors"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

// ArticleInteraction tracks the state of an article browsing session
type ArticleInteraction struct {
	CurrentIndex int
	Articles     []*hytale.Article
	LastUsed     time.Time // Track the last time the interaction was used
}

// Global map to track article interactions
var articleInteractions = make(map[string]*ArticleInteraction)

// CleanupInterval defines how often to clean up old interactions
const CleanupInterval = 5 * time.Minute

// InteractionExpiry defines how long an interaction can be inactive before being removed
const InteractionExpiry = 30 * time.Minute

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

// StartCleanup starts a background goroutine to periodically clean up old interactions
func StartCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	go func() {
		for range ticker.C {
			cleanupOldInteractions()
		}
	}()
}

// cleanupOldInteractions removes interactions that have expired
func cleanupOldInteractions() {
	now := time.Now()
	for id, interaction := range articleInteractions {
		if now.Sub(interaction.LastUsed) > InteractionExpiry {
			delete(articleInteractions, id)
		}
	}
}

func versionCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	// Get the launcher release feed
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherReleaseFeedID]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale Launcher version."))
		return
	}

	// Get the version from the feed
	launcherReleaseFeed, ok := feed.(*hytale.LauncherReleaseFeed)
	if !ok {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale Launcher version."))
		return
	}

	// Respond with the embed
	ctx.ReplyEmbed(launcherReleaseFeed.BuildMessage(s, ctx.Config))
}

func articlesCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	// Get the launcher post feed
	feed, exists := ctx.HytaleFeeds.Feeds[hytale.LauncherPostFeedID]
	if !exists {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale article."))
		return
	}

	// Get the articles from the feed
	launcherPostFeed, ok := feed.(*hytale.LauncherPostFeed)
	if !ok {
		ctx.ReplyError(errors.New("Could not retrieve the latest Hytale article."))
		return
	}
	// Get all articles
	articles := launcherPostFeed.Articles.Articles
	if len(articles) == 0 {
		ctx.ReplyEphemeral("No articles found.")
		return
	}

	// Store the interaction and current article index
	interactionID := i.Interaction.ID
	articleInteractions[interactionID] = &ArticleInteraction{
		CurrentIndex: 0,
		Articles:     articles,
		LastUsed:     time.Now(),
	}

	// Build the message embed for the latest article
	latestArticle := articles[0]
	embed := latestArticle.BuildMessage(s, ctx.Config)

	// Add buttons
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

	// Respond with the embed and buttons
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

// HandleArticleButton handles button clicks for navigating articles
func HandleArticleButton(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	customID := i.MessageComponentData().CustomID
	interactionID := strings.TrimPrefix(customID, "article_back_")
	interactionID = strings.TrimPrefix(interactionID, "article_forward_")

	// Get the interaction data
	interaction, exists := articleInteractions[interactionID]
	if !exists {
		ctx.ReplyEphemeral("This interaction has expired.")
		return
	}

	// Update the last used time
	interaction.LastUsed = time.Now()

	// Update the current index based on the button clicked
	if strings.HasPrefix(customID, "article_back_") {
		interaction.CurrentIndex++
	} else if strings.HasPrefix(customID, "article_forward_") {
		interaction.CurrentIndex--
	}

	// Get the current article
	currentArticle := interaction.Articles[interaction.CurrentIndex]

	// Build the message embed for the current article
	embed := currentArticle.BuildMessage(s, ctx.Config)

	// Update the buttons
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
		},
	})
}
