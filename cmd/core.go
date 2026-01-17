package cmd

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var HelpCommand = &discordgo.ApplicationCommand{
	Name:        "help",
	Description: "List all commands",
}

func helpCommand(s *discordgo.Session, i *discordgo.InteractionCreate, ctx *CommandContext) {
	var description strings.Builder
	for _, command := range commands {
		description.WriteString(fmt.Sprintf("`/%s` - %s\n", command.discord.Name, command.discord.Description))
	}

	embed := &discordgo.MessageEmbed{
		Title:       "Gaia Help",
		Description: description.String(),
		Color:       0x0000ff,
	}

	s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Embeds: []*discordgo.MessageEmbed{embed},
		},
	})
}
