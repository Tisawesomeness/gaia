package cmd

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
	"github.com/sony/gobreaker"
)

type CommandExecutor struct {
	Config      *config.Config
	DB          *db.DB
	HTTP        *http.Client
	AuthStore   *auth.AuthStore
	HytaleFeeds *hytale.HytaleFeeds
	BootTime    *time.Time
	Breakers    *Breakers
}

type Breakers struct {
	HytaleSession *gobreaker.CircuitBreaker
	KratosSession *gobreaker.CircuitBreaker
}

func NewCommandExecutor(config *config.Config, db *db.DB, httpClient *http.Client, authStore *auth.AuthStore, hytaleFeeds *hytale.HytaleFeeds, bootTime *time.Time) CommandExecutor {
	return CommandExecutor{
		Config:      config,
		DB:          db,
		HTTP:        httpClient,
		AuthStore:   authStore,
		HytaleFeeds: hytaleFeeds,
		BootTime:    bootTime,
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
	BootTime    *time.Time
	Breakers    *Breakers

	Session     *discordgo.Session
	Interaction *discordgo.InteractionCreate
	hasDeferred bool
}

func (ce CommandExecutor) newCommandContext(s *discordgo.Session, i *discordgo.InteractionCreate) *CommandContext {
	return &CommandContext{
		Config:      ce.Config,
		DB:          ce.DB,
		HTTP:        ce.HTTP,
		AuthStore:   ce.AuthStore,
		HytaleFeeds: ce.HytaleFeeds,
		Breakers:    ce.Breakers,
		BootTime:    ce.BootTime,
		Session:     s,
		Interaction: i,
		hasDeferred: false,
	}
}

func (ctx *CommandContext) Options() map[string]*discordgo.ApplicationCommandInteractionDataOption {
	options := ctx.Interaction.ApplicationCommandData().Options
	optionMap := make(map[string]*discordgo.ApplicationCommandInteractionDataOption, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	return optionMap
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
		// Default to no mentions allowed
		if data.AllowedMentions == nil {
			data.AllowedMentions = &discordgo.MessageAllowedMentions{}
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
	handler func(ctx *CommandContext)
}

var categories []*Category

func init() {
	categories = []*Category{
		{"Core", []*Command{
			{HelpCommand, helpCommand},
			{InfoCommand, infoCommand},
			{CreditsCommand, creditsCommand},
		}},
		{"Players", []*Command{
			{ProfileCommand, profileCommand},
			{SkinCommand, skinCommand},
		}},
		{"Updates", []*Command{
			{VersionCommand, versionCommand},
			{LauncherCommand, launcherCommand},
			{ArticlesCommand, articlesCommand},
			{SubscribeCommand, subscribeCommand},
			{SubscribeDMCommand, subscribeDMCommand},
			{ListCommand, listCommand},
			{UnsubscribeCommand, unsubscribeCommand},
		}},
	}
}

func (ce CommandExecutor) HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
	ctx := ce.newCommandContext(s, i)

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
					command.handler(ctx)
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
	if config.CreateCommandsOnStartup {
		err := deployCommands(session, config)
		if err != nil {
			return err
		}
	}
	StartCleanup()
	return nil
}

func deployCommands(session *discordgo.Session, config config.Config) error {
	var commands []*discordgo.ApplicationCommand
	var testCommands []*discordgo.ApplicationCommand

	for _, category := range categories {
		for _, command := range category.commands {
			commands = append(commands, command.discord)

			contexts := command.discord.Contexts
			if config.TestServer == "" || (contexts != nil && !slices.Contains(*contexts, discordgo.InteractionContextGuild)) {
				continue
			}

			guildCommand := *command.discord
			guildCommand.Name = "test-" + guildCommand.Name
			testCommands = append(testCommands, &guildCommand)
		}
	}

	if len(commands) > 0 {
		_, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, "", commands)
		if err != nil {
			return fmt.Errorf("Could not deploy global commands: %v", err)
		}
	}

	if len(testCommands) > 0 && config.TestServer != "" {
		_, err := session.ApplicationCommandBulkOverwrite(session.State.User.ID, config.TestServer, testCommands)
		if err != nil {
			return fmt.Errorf("Could not deploy guild commands: %v", err)
		}
	}
	return nil
}
