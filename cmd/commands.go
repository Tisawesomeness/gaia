package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
	"github.com/sony/gobreaker"
)

type CommandHandler struct {
	Config      *config.Config
	DB          *db.DB
	HTTP        *http.Client
	AuthStore   *auth.AuthStore
	HytaleFeeds *hytale.HytaleFeeds
	Breakers    *Breakers
}

type Breakers struct {
	HytaleSession *gobreaker.CircuitBreaker
	KratosSession *gobreaker.CircuitBreaker
}

func NewCommandHandler(config *config.Config, db *db.DB, httpClient *http.Client, authStore *auth.AuthStore, hytaleFeeds *hytale.HytaleFeeds) CommandHandler {
	return CommandHandler{
		Config:      config,
		DB:          db,
		HTTP:        httpClient,
		AuthStore:   authStore,
		HytaleFeeds: hytaleFeeds,
		Breakers: &Breakers{
			HytaleSession: makeBreaker("HytaleSession", config.Auth.Breaker),
			KratosSession: makeBreaker("KratosSession", config.Kratos.Breaker),
		},
	}
}

func makeBreaker(name string, config config.BreakerConfig) *gobreaker.CircuitBreaker {
	return gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        name,
		MaxRequests: config.MaxHalfOpenRequests,
		Interval:    time.Duration(config.ResetInterval) * time.Second,
		Timeout:     time.Duration(config.Timeout) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return config.Enabled && counts.Requests >= config.MaxHalfOpenRequests && failureRatio >= config.FailureRatio
		},

		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Printf("Circuit breaker %s %s -> %s", name, from.String(), to.String())
		},
		IsSuccessful: func(err error) bool {
			var userErr *UserError
			return errors.As(err, &userErr)
		},
	})
}

type CommandContext struct {
	Config      *config.Config
	DB          *db.DB
	HTTP        *http.Client
	AuthStore   *auth.AuthStore
	HytaleFeeds *hytale.HytaleFeeds
	Breakers    *Breakers

	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	hasDeferred bool
}

func (ch CommandHandler) newCommandContext(s *discordgo.Session, i *discordgo.InteractionCreate) *CommandContext {
	return &CommandContext{
		Config:      ch.Config,
		DB:          ch.DB,
		HTTP:        ch.HTTP,
		AuthStore:   ch.AuthStore,
		HytaleFeeds: ch.HytaleFeeds,
		Breakers:    ch.Breakers,
		Session:     s,
		Interaction: i,
		hasDeferred: false,
	}
}

func (ctx *CommandContext) User() *discordgo.User {
	if ctx.Interaction.Member != nil {
		return ctx.Interaction.Member.User
	} else {
		return ctx.Interaction.User
	}
}

func (ctx *CommandContext) DeferReply() {
	ctx.hasDeferred = true
	ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

func (ctx *CommandContext) Reply(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
	})
}

func (ctx *CommandContext) ReplyEphemeral(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func (ctx *CommandContext) ReplyEmbed(embed *discordgo.MessageEmbed) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (ctx *CommandContext) ReplyComplex(data *discordgo.InteractionResponseData) {
	if ctx.hasDeferred {
		var attachments []*discordgo.MessageAttachment
		if data.Attachments == nil {
			attachments = []*discordgo.MessageAttachment{}
		} else {
			attachments = *data.Attachments
		}
		ctx.Session.FollowupMessageCreate(ctx.Interaction.Interaction, false, &discordgo.WebhookParams{
			Content:         data.Content,
			Components:      data.Components,
			Embeds:          data.Embeds,
			TTS:             data.TTS,
			Files:           data.Files,
			Attachments:     attachments,
			AllowedMentions: data.AllowedMentions,
			Flags:           data.Flags,
		})
	} else {
		ctx.Session.InteractionRespond(ctx.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})
	}
}

func (ctx *CommandContext) ReplyWarn(content string) {
	ctx.ReplyEphemeral(":warning: " + content)
}

func (ctx *CommandContext) ReplyExternalError(content string) {
	ctx.ReplyEphemeral(":x: " + content)
}

func (ctx *CommandContext) ReplyError(err error) {
	var userErr *UserError
	if errors.As(err, &userErr) {
		ctx.ReplyWarn(userErr.message)
	} else {
		ctx.ReplyEphemeral(":boom: An error occurred: " + err.Error())

		id := ctx.Interaction.ApplicationCommandData().Name
		options := formatCommandOptions(ctx.Interaction.ApplicationCommandData().Options)
		log.Printf("Error in command /%s options %s: %+v", id, options, err)
	}
}

func formatCommandOptions(options []*discordgo.ApplicationCommandInteractionDataOption) string {
	var parts []string
	for _, option := range options {
		value := option.Value
		if option.Type == discordgo.ApplicationCommandOptionSubCommand {
			value = formatCommandOptions(option.Options)
		}
		parts = append(parts, fmt.Sprintf("%s=%v", option.Name, value))
	}
	return strings.Join(parts, ", ")
}

type UserError struct {
	message string
}

func (ce UserError) Error() string {
	return ce.message
}

func NewUserError(message string) UserError {
	return UserError{message: message}
}

type Category struct {
	name     string
	commands []*Command
}

type Command struct {
	discord *discordgo.ApplicationCommand
	handler func(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext)
}

var categories []*Category

func init() {
	categories = []*Category{
		{"Core", []*Command{
			{HelpCommand, helpCommand},
			{CreditsCommand, creditsCommand},
		}},
		{"Players", []*Command{
			{ProfileCommand, profileCommand},
			{SkinCommand, skinCommand},
		}},
		{"Updates", []*Command{
			{VersionCommand, versionCommand},
			{ArticlesCommand, articlesCommand},
			{SubscribeCommand, subscribeCommand},
			{ListCommand, listCommand},
			{UnsubscribeCommand, unsubscribeCommand},
		}},
	}
}

func (ch CommandHandler) HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := ch.newCommandContext(s, i)

	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		// https://www.youtube.com/watch?v=bLHL75H_VEM
		defer func() {
			if r := recover(); r != nil {
				id := i.ApplicationCommandData().Name
				options := formatCommandOptions(i.ApplicationCommandData().Options)
				log.Printf("Panic in command /%s options %s:\n%v", id, options, r)
			}
		}()

		commandName := strings.TrimPrefix(i.ApplicationCommandData().Name, "test-")
		for _, category := range categories {
			for _, command := range category.commands {
				if commandName == command.discord.Name {
					command.handler(s, i, ctx)
					return
				}
			}
		}

	case discordgo.InteractionMessageComponent:
		customID := i.MessageComponentData().CustomID

		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in interaction %s:\n%v", customID, r)
			}
		}()

		if isArticleInteraction(customID) {
			HandleArticleButton(s, i, ctx)
		}
	}
}

func InitCommands(session *discordgo.Session, config config.Config) error {
	for _, category := range categories {
		for _, command := range category.commands {
			// Register global commands
			_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command.discord)
			if err != nil {
				return fmt.Errorf("Could not deploy global '%v' command: %v", command.discord.Name, err)
			}

			if config.TestServer == "" {
				continue
			}
			// Register guild commands with the test- prefix
			guildCommand := *command.discord
			guildCommand.Name = "test-" + guildCommand.Name
			_, err = session.ApplicationCommandCreate(session.State.User.ID, config.TestServer, &guildCommand)
			if err != nil {
				return fmt.Errorf("Could not deploy guild '%v' command: %v", guildCommand.Name, err)
			}
		}
	}
	StartCleanup()
	return nil
}
