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

const (
	CleanupInterval   = 5 * time.Minute
	InteractionExpiry = 30 * time.Minute
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

type InteractionType struct {
	prefix  string
	handler func(ctx CommandContext, state any)
}

type InteractionSession struct {
	State    any
	UserID   string
	LastUsed time.Time
}

var (
	categories          []*Category
	interactionTypes    []*InteractionType
	interactionSessions map[string]*InteractionSession
)

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
	interactionTypes = []*InteractionType{
		{"article", handleArticleButton},
	}
	interactionSessions = make(map[string]*InteractionSession)
}

func getCommand(name string) *Command {
	commandName := strings.TrimPrefix(name, "test-")
	for _, category := range categories {
		for _, command := range category.commands {
			if commandName == command.discord.Name {
				return command
			}
		}
	}
	return nil
}

func getInteractionType(interactionType string) *InteractionType {
	for _, interaction := range interactionTypes {
		if interaction.prefix == interactionType {
			return interaction
		}
	}
	return nil
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

		ctx := ce.newCommandContext(s, i)
		handleCommand(ctx, i.ApplicationCommandData().ID)

	case discordgo.InteractionMessageComponent:
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Panic in interaction %s:\n%v", i.MessageComponentData().CustomID, r)
			}
		}()

		ctx := ce.newCommandContext(s, i)
		handleInteraction(ctx)
	}
}

func handleCommand(ctx CommandContext, name string) {
	command := getCommand(name)
	if command != nil {
		if ctx.UserCanExecute(command.discord) {
			command.handler(ctx)
		}
	}
}

func handleInteraction(ctx CommandContext) {
	customID := ctx.CustomID()
	interactionType := getInteractionType(customID.InteractionType)
	if interactionType != nil {
		interactionSession := interactionSessions[customID.SessionID]
		if interactionSession != nil {
			if ctx.User().ID != interactionSession.UserID {
				ctx.ReplyEphemeral("You cannot interact with someone else's menu.")
				return
			}
			if time.Now().After(interactionSession.LastUsed.Add(InteractionExpiry)) {
				ctx.ReplyEphemeral("That menu has expired.")
				return
			}
			interactionSession.LastUsed = time.Now()
			interactionType.handler(ctx, interactionSession.State)
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
	startInteractionCleanup()
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

func startInteractionCleanup() {
	ticker := time.NewTicker(CleanupInterval)
	go func() {
		for range ticker.C {
			cleanupOldInteractions()
		}
	}()
}

func cleanupOldInteractions() {
	now := time.Now()
	for id, interaction := range interactionSessions {
		if now.Sub(interaction.LastUsed) > InteractionExpiry {
			delete(interactionSessions, id)
		}
	}
}
