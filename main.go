package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

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

	client, err := initValkey(&config)
	if err != nil {
		log.Fatalf("Error creating Valkey client: %v", err)
	}
	defer client.Close()
	log.Println("Connected to valkey")

	api, err := NewHytaleAPI(&client, &config)
	if err != nil {
		log.Fatalf("Error creating Hytale API: %v", err)
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
	log.Println("Bot authenticated")

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		interactionCreate(s, i, &client)
	})

	err = session.Open()
	if err != nil {
		log.Fatalf("Could not open session: %v", err)
	}
	defer session.Close()
	log.Println("Bot started")

	_, err = session.ApplicationCommandCreate(session.State.User.ID, "", SubscribeCommand)
	if err != nil {
		log.Fatalf("Could not create subscribe command: %v", err)
	}

	go pollAPIs(api, &config)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Bot shut down")
}

func initValkey(config *Config) (valkey.Client, error) {
	options := valkey.ClientOption{
		InitAddress: []string{config.Valkey.Address + ":" + strconv.Itoa(config.Valkey.Port)},
	}
	if config.Valkey.Password != "" {
		options.Password = config.Valkey.Password
	}
	return valkey.NewClient(options)
}

func pollAPIs(api *HytaleAPI, config *Config) {
	ticker := time.NewTicker(time.Duration(config.API.Interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Polling APIs...")
			err := api.PollLauncherRelease()
			if err != nil {
				log.Printf("Error while polling launcher release: %v", err)
			}
		}
	}
}
