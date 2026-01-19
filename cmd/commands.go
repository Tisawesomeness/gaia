package cmd

import (
	"errors"
	"fmt"
	"log"
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

func (ctx CommandContext) ReplyError(s *discordgo.Session, i *discordgo.InteractionCreate, err error) {
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		ctx.ReplyWarn(s, i, cmdErr.message)
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

type CommandError struct {
	message string
}

func (ce CommandError) Error() string {
	return ce.message
}

func NewCommandError(message string) CommandError {
	return CommandError{message: message}
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
