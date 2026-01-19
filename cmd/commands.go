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

type Breakers struct {
	HytaleSession *gobreaker.CircuitBreaker
	KratosSession *gobreaker.CircuitBreaker
}

type CommandContext struct {
	Config      config.Config
	DB          db.DB
	HTTP        http.Client
	AuthStore   auth.AuthStore
	HytaleFeeds hytale.HytaleFeeds
	Breakers    Breakers
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

func NewCommandContext(config config.Config, db db.DB, httpClient http.Client, authStore auth.AuthStore, hytaleFeeds hytale.HytaleFeeds) *CommandContext {
	return &CommandContext{
		Config:      config,
		DB:          db,
		HTTP:        httpClient,
		AuthStore:   authStore,
		HytaleFeeds: hytaleFeeds,
		Breakers: Breakers{
			HytaleSession: makeBreaker("HytaleSession", config.Auth.Breaker),
			KratosSession: makeBreaker("KratosSession", config.Kratos.Breaker),
		},
	}
}

func (ctx CommandContext) Reply(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	ctx.ReplyComplex(s, i, &discordgo.InteractionResponseData{
		Content: content,
	})
}

func (ctx CommandContext) ReplyEphemeral(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	ctx.ReplyComplex(s, i, &discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func (ctx CommandContext) ReplyEmbed(s *discordgo.Session, i *discordgo.InteractionCreate, embed *discordgo.MessageEmbed) {
	ctx.ReplyComplex(s, i, &discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (ctx CommandContext) ReplyComplex(s *discordgo.Session, i *discordgo.InteractionCreate, data *discordgo.InteractionResponseData) {
	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: data,
	})
}

func (ctx CommandContext) ReplyWarn(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	ctx.ReplyEphemeral(s, i, ":warning: "+content)
}

func (ctx CommandContext) ReplyExternalError(s *discordgo.Session, i *discordgo.InteractionCreate, content string) {
	ctx.ReplyEphemeral(s, i, ":x: "+content)
}

func (ctx CommandContext) ReplyError(s *discordgo.Session, i *discordgo.InteractionCreate, err error) {
	var userErr *UserError
	if errors.As(err, &userErr) {
		ctx.ReplyWarn(s, i, userErr.message)
	} else {
		ctx.ReplyEphemeral(s, i, ":boom: An error occurred: "+err.Error())

		id := i.ApplicationCommandData().Name
		options := formatCommandOptions(i.ApplicationCommandData().Options)
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

func HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
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
