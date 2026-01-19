package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

type CommandContext struct {
	Config      config.Config
	DB          db.DB
	HTTP        http.Client
	AuthStore   auth.AuthStore
	HytaleFeeds hytale.HytaleFeeds
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
		// Handle button interactions
		customID := i.MessageComponentData().CustomID
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
