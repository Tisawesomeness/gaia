package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/valkey-io/valkey-go"
)

type CommandContext struct {
	config       *Config
	valkeyClient *valkey.Client
	hytale       *HytaleAPI
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

func main() {
	config, err := LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	valkeyClient, err := initValkey(&config)
	if err != nil {
		log.Fatalf("Error creating Valkey client: %v", err)
	}
	defer valkeyClient.Close()
	log.Println("Connected to valkey")

	api, err := NewHytaleAPI(&valkeyClient, &config)
	if err != nil {
		log.Fatalf("Error creating Hytale API: %v", err)
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
	log.Println("Bot authenticated")

	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		if i.Type != discordgo.InteractionApplicationCommand {
			return
		}
		commandName := i.ApplicationCommandData().Name
		for _, command := range commands {
			if commandName == command.discord.Name {
				command.handler(s, i, &CommandContext{
					&config,
					&valkeyClient,
					api,
				})
				return
			}
		}
	})

	err = session.Open()
	if err != nil {
		log.Fatalf("Could not open session: %v", err)
	}
	defer session.Close()
	log.Println("Bot started")

	err = initCommands(session)
	if err != nil {
		log.Fatalf("Error while registering commands: %v", err)
	}

	go pollAPIs(session, &config, &valkeyClient, api)

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

func initCommands(session *discordgo.Session) error {
	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command.discord)
		if err != nil {
			return fmt.Errorf("Could not deploy '%v' command: %v", command.discord.Name, err)
		}
	}
	return nil
}

func pollAPIs(s *discordgo.Session, config *Config, client *valkey.Client, api *HytaleAPI) {
	ticker := time.NewTicker(time.Duration(config.API.Interval) * time.Second)
	defer ticker.Stop()

	poll(api, s, client)
	for {
		select {
		case <-ticker.C:
			poll(api, s, client)
		}
	}
}

func poll(api *HytaleAPI, s *discordgo.Session, client *valkey.Client) {
	log.Println("Polling APIs...")
	err := api.PollLauncherRelease()
	if err != nil {
		log.Printf("Error while polling launcher release: %v", err)
		return
	}
	err = notifyLauncherReleaseFeeds(s, client, api)
	if err != nil {
		log.Printf("Error while notifying channels: %v", err)
	}
}
