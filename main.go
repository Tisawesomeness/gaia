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

func initCommands(session *discordgo.Session) error {
	for _, command := range commands {
		_, err := session.ApplicationCommandCreate(session.State.User.ID, "", command.discord)
		if err != nil {
			return fmt.Errorf("Could not deploy '%v' command: %v", command.discord.Name, err)
		}
	}
	return nil
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
