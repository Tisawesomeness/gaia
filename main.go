package main

import (
	"github.com/bwmarrin/discordgo"
)

func main() {
	_, err := discordgo.New("Bot " + "")
	if err != nil {
		panic(err)
	}
}
