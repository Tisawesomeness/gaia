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
	fields := []*discordgo.MessageEmbedField{}
	for _, category := range categories {
		var description strings.Builder
		for _, command := range category.commands {
			fmt.Fprintf(&description, "`/%s` - %s\n", command.discord.Name, command.discord.Description)
		}
		field := &discordgo.MessageEmbedField{
			Name:   category.name,
			Value:  description.String(),
			Inline: false,
		}
		fields = append(fields, field)
	}

	embed := &discordgo.MessageEmbed{
		Title:  "Gaia Help",
		Color:  0x0000ff,
		Fields: fields,
	}

	ctx.ReplyEmbed(s, i, embed)
}
