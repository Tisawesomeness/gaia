package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

type CommandContext struct {
	Config      config.Config
	DB          db.DB
	HTTP        http.Client
	HytaleFeeds hytale.HytaleFeeds
}

type Command struct {
	discord *discordgo.ApplicationCommand
	handler func(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext)
}

var commands []*Command

func init() {
	commands = []*Command{
		{HelpCommand, helpCommand},
		{VersionCommand, versionCommand},
		{ArticlesCommand, articlesCommand},
		{SubscribeCommand, subscribeCommand},
		{ListCommand, listCommand},
		{UnsubscribeCommand, unsubscribeCommand},
	}
}

func HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	commandName := strings.TrimPrefix(i.ApplicationCommandData().Name, "test-")
	for _, command := range commands {
		if commandName == command.discord.Name {
			command.handler(s, i, ctx)
			return
		}
	}
}

func InitCommands(session *discordgo.Session, config config.Config) error {
	// Register global commands
	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command.discord)
		if err != nil {
			return fmt.Errorf("Could not deploy global '%v' command: %v", command.discord.Name, err)
		}
	}

	// Register guild commands with the test- prefix
	guildID := config.TestServer
	if guildID == "" {
		return nil
	}

	for _, command := range commands {
		guildCommand := *command.discord
		guildCommand.Name = "test-" + guildCommand.Name
		_, err := session.ApplicationCommandCreate(session.State.User.ID, guildID, &guildCommand)
		if err != nil {
			return fmt.Errorf("Could not deploy guild '%v' command: %v", guildCommand.Name, err)
		}
	}
	return nil
}
