package cmd

import (
	_ "embed"
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

type BotMetadata struct {
	Version  string
	BootTime *time.Time
}

type CommandExecutor struct {
	Config      *config.Config
	DB          *db.DB
	HTTP        *http.Client
	AuthStore   auth.AuthStore
	HytaleFeeds *hytale.HytaleFeeds
	Breakers    *Breakers
	BotMetadata *BotMetadata
}

// Shared circuit breakers based on auth method
// ex: if Hytale session goes down, circuit breaker will gradually retry
type Breakers struct {
	HytaleSession *gobreaker.CircuitBreaker
	KratosSession *gobreaker.CircuitBreaker
}

func NewCommandExecutor(
	config *config.Config,
	db *db.DB,
	httpClient *http.Client,
	authStore auth.AuthStore,
	hytaleFeeds *hytale.HytaleFeeds,
	version string,
	bootTime *time.Time,
) CommandExecutor {
	return CommandExecutor{
		Config:      config,
		DB:          db,
		HTTP:        httpClient,
		AuthStore:   authStore,
		HytaleFeeds: hytaleFeeds,
		Breakers:    makeBreakers(config),
		BotMetadata: &BotMetadata{
			Version:  version,
			BootTime: bootTime,
		},
	}
}

func makeBreakers(config *config.Config) *Breakers {
	return &Breakers{
		HytaleSession: makeBreaker("HytaleSession", config.Auth.Breaker),
		KratosSession: makeBreaker("KratosSession", config.Kratos.Breaker),
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
	})
}

type OptionsMap map[string]*discordgo.ApplicationCommandInteractionDataOption

// Contains utility methods that make commands less error-prone:
//
// - Reply... methods with mentions disabled by default and work whether deferred or not
//
// - User() gets the user without having to nil-check Interaction.User / Interaction.Member
//
// - etc.
type CommandContext interface {
	Config() *config.Config
	DB() *db.DB
	HTTP() *http.Client
	AuthStore() auth.AuthStore
	HytaleFeeds() *hytale.HytaleFeeds
	Breakers() *Breakers
	BotMetadata() *BotMetadata
	// The raw Discord session, prefer using CommandContext methods since this makes the command untestable
	Session() *discordgo.Session
	// The raw Discord event, prefer using CommandContext methods since this makes the command untestable
	Interaction() *discordgo.InteractionCreate

	InteractionID() string
	// Generates a map of option ID to option data
	Options() OptionsMap
	// Gets the user that executed this command, works in both guilds and DMs
	User() *discordgo.User
	// Checks if the user has permission and is in the right context to execute the provided command
	UserCanExecute(command *discordgo.ApplicationCommand) bool
	// The ID of the guild the command was executed in. Empty if not executed in a guild.
	GuildID() string
	// Fetches the channels of the provided guild.
	GuildChannels(guildID string) ([]*discordgo.Channel, error)

	// Signals to Discord that the command was received and a follow-up will come later.
	// After deferring, regular replies (InteractionResponseChannelMessageWithSource) will do nothing.
	// Use the ctx.Reply... methods instead.
	DeferReply()
	Reply(content string)
	ReplyEphemeral(content string)
	ReplyEmbed(embed *discordgo.MessageEmbed)
	ReplyComplex(data *discordgo.InteractionResponseData)
	ReplyWarn(content string)
	ReplyExternalError(content string)
	ReplyError(message string, err error)
}

type commandContext struct {
	config      *config.Config
	db          *db.DB
	http        *http.Client
	authStore   auth.AuthStore
	hytaleFeeds *hytale.HytaleFeeds
	breakers    *Breakers
	botMetadata *BotMetadata

	session     *discordgo.Session
	interaction *discordgo.InteractionCreate
	hasDeferred bool
}

func (ce CommandExecutor) newCommandContext(s *discordgo.Session, i *discordgo.InteractionCreate) *commandContext {
	return &commandContext{
		config:      ce.Config,
		db:          ce.DB,
		http:        ce.HTTP,
		authStore:   ce.AuthStore,
		hytaleFeeds: ce.HytaleFeeds,
		breakers:    ce.Breakers,
		botMetadata: ce.BotMetadata,
		session:     s,
		interaction: i,
		hasDeferred: false,
	}
}

// Implements ICommandContext
func (ctx *commandContext) Config() *config.Config                    { return ctx.config }
func (ctx *commandContext) DB() *db.DB                                { return ctx.db }
func (ctx *commandContext) HTTP() *http.Client                        { return ctx.http }
func (ctx *commandContext) AuthStore() auth.AuthStore                 { return ctx.authStore }
func (ctx *commandContext) HytaleFeeds() *hytale.HytaleFeeds          { return ctx.hytaleFeeds }
func (ctx *commandContext) Breakers() *Breakers                       { return ctx.breakers }
func (ctx *commandContext) BotMetadata() *BotMetadata                 { return ctx.botMetadata }
func (ctx *commandContext) Session() *discordgo.Session               { return ctx.session }
func (ctx *commandContext) Interaction() *discordgo.InteractionCreate { return ctx.interaction }

func (ctx *commandContext) InteractionID() string {
	return ctx.Interaction().Interaction.ID
}

func (ctx *commandContext) Options() OptionsMap {
	options := ctx.Interaction().ApplicationCommandData().Options
	optionMap := make(OptionsMap, len(options))
	for _, opt := range options {
		optionMap[opt.Name] = opt
	}
	return optionMap
}

func (ctx *commandContext) User() *discordgo.User {
	if ctx.Interaction().Member != nil {
		return ctx.Interaction().Member.User
	} else {
		return ctx.Interaction().User
	}
}

func (ctx *commandContext) UserCanExecute(command *discordgo.ApplicationCommand) bool {
	if command.Contexts != nil && !slices.Contains(*command.Contexts, ctx.Interaction().Context) {
		return false
	}
	if command.DefaultMemberPermissions == nil {
		return true
	}
	if ctx.Interaction().Member != nil {
		return (ctx.Interaction().Member.Permissions & *command.DefaultMemberPermissions) == *command.DefaultMemberPermissions
	} else {
		return true
	}
}

func (ctx *commandContext) GuildID() string {
	return ctx.Interaction().GuildID
}

func (ctx *commandContext) GuildChannels(guildID string) ([]*discordgo.Channel, error) {
	return ctx.Session().GuildChannels(guildID)
}

func (ctx *commandContext) DeferReply() {
	ctx.hasDeferred = true
	ctx.Session().InteractionRespond(ctx.Interaction().Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

func (ctx *commandContext) Reply(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
	})
}

func (ctx *commandContext) ReplyEphemeral(content string) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Content: content,
		Flags:   discordgo.MessageFlagsEphemeral,
	})
}

func (ctx *commandContext) ReplyEmbed(embed *discordgo.MessageEmbed) {
	ctx.ReplyComplex(&discordgo.InteractionResponseData{
		Embeds: []*discordgo.MessageEmbed{embed},
	})
}

func (ctx *commandContext) ReplyWarn(content string) {
	ctx.ReplyEphemeral(":warning: " + content)
}

func (ctx *commandContext) ReplyExternalError(content string) {
	ctx.ReplyEphemeral(":x: " + content)
}

func (ctx *commandContext) ReplyError(message string, err error) {
	ctx.ReplyEphemeral(":boom: " + message)
	if err != nil {
		ctx.reportError(err)
	}
}

func (ctx *commandContext) ReplyComplex(data *discordgo.InteractionResponseData) {
	// Default to no mentions allowed
	if data.AllowedMentions == nil {
		data.AllowedMentions = &discordgo.MessageAllowedMentions{}
	}

	for _, embed := range data.Embeds {
		if embed.Footer == nil {
			embed.Footer = &discordgo.MessageEmbedFooter{
				Text: "Gaia " + ctx.BotMetadata().Version,
			}
		}
	}

	var err error
	if ctx.hasDeferred {
		var attachments []*discordgo.MessageAttachment
		if data.Attachments == nil {
			attachments = []*discordgo.MessageAttachment{}
		} else {
			attachments = *data.Attachments
		}

		_, err = ctx.Session().FollowupMessageCreate(ctx.Interaction().Interaction, false, &discordgo.WebhookParams{
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
		err = ctx.Session().InteractionRespond(ctx.Interaction().Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: data,
		})
	}

	if err != nil {
		ctx.reportError(err)
	}
}

func (ctx *commandContext) reportError(err error) {
	id := ctx.Interaction().ApplicationCommandData().Name
	options := formatCommandOptions(ctx.Interaction().ApplicationCommandData().Options)
	log.Printf("Error in command /%s options %s:\n%+v", id, options, err)
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

type Category struct {
	name     string
	commands []*Command
}

type Command struct {
	discord *discordgo.ApplicationCommand
	handler func(ctx CommandContext)
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
			NewRenderCommand("head", "Get an image of a Hytale player's head", hytale.HeadRender),
			NewRenderCommand("body", "Get an image of a Hytale player's body", hytale.FullBodyRender),
			NewRenderCommand("cape", "Get an image of a Hytale player's cape", hytale.CapeRender),
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
		{"Developer", []*Command{
			{MavenCommand, mavenCommand},
			{GradleCommand, gradleCommand},
		}},
	}
}

func (ce CommandExecutor) HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate) {
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
					ctx := ce.newCommandContext(s, i)
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
			ctx := ce.newCommandContext(s, i)
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
		log.Println("Commands created")
	}
	StartInteractionCleanup()
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

			// Global commands can take some time to deploy
			// Registering guild commands in a test server is instant and great for prototyping
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
