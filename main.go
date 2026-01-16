package main

import (
	"log"
	"os"
	"os/signal"

	"github.com/bwmarrin/discordgo"
	"github.com/valkey-io/valkey-go"
)

func interactionCreate(s *discordgo.Session, i *discordgo.InteractionCreate, client *valkey.Client) {
	if i.Type != discordgo.InteractionApplicationCommand {
		return
	}

	if i.ApplicationCommandData().Name == "subscribe" {
		subscribeCommand(i, s, client)
	}
}

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}
	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
	log.Println("Bot authenticated")

	// Initialize Valkey client
	client, err := valkey.NewClient(valkey.ClientOption{InitAddress: []string{"127.0.0.1:6379"}})
	if err != nil {
		log.Fatalf("Error creating Valkey client: %v", err)
	}
	defer client.Close()
	log.Println("Connected to valkey")

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		interactionCreate(s, i, &client)
	})

	err = session.Open()
	if err != nil {
		log.Fatalf("Could not open session: %v", err)
	}
	defer session.Close()
	log.Println("Bot started")

	// Register the subscribe command
	_, err = session.ApplicationCommandCreate(session.State.User.ID, "", SubscribeCommand)
	if err != nil {
		log.Fatalf("Could not create subscribe command: %v", err)
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Bot shut down")
}
