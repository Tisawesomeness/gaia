package main

import (
	"github.com/bwmarrin/discordgo"
)

func main() {
	config := LoadConfig()
	_, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		panic(err)
	}
}
