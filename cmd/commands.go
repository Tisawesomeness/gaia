package cmd

import (
	"fmt"
	"net/http"

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

var commands = []*Command{
	{SubscribeCommand, subscribeCommand},
	{ListCommand, listCommand},
	{UnsubscribeCommand, unsubscribeCommand},
}

func HandleInteractionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}
	commandName := i.ApplicationCommandData().Name
	for _, command := range commands {
		if commandName == command.discord.Name {
			command.handler(s, i, ctx)
			return
		}
	}
}

func InitCommands(session *discordgo.Session) error {
	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command.discord)
		if err != nil {
			return fmt.Errorf("Could not deploy '%v' command: %v", command.discord.Name, err)
		}
	}
	return nil
}
