package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/cmd"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/bwmarrin/discordgo"
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	database, err := db.NewDB(config)
	if err != nil {
		log.Fatalf("Error creating Valkey client: %v", err)
	}
	defer database.Close()
	log.Println("Connected to valkey")

	httpClient := initHTTP(config)

	authStore, err := auth.NewAuthStore(config, *database, httpClient)
	if err != nil {
		log.Fatalf("Could not create auth store: %v", err)
	}

	feeds, err := hytale.NewHytaleFeeds(&config, database, httpClient, authStore)
	if err != nil {
		log.Fatalf("Error creating Hytale feeds: %v", err)
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
	log.Println("Bot authenticated")

	bootTime := time.Now()
	commandExecutor := cmd.NewCommandExecutor(&config, database, httpClient, authStore, feeds, &bootTime)
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		commandExecutor.HandleInteractionCreate(s, i)
	})

	err = session.Open()
	if err != nil {
		log.Fatalf("Could not open session: %v", err)
	}
	defer session.Close()
	log.Println("Opened Discord session")

	err = cmd.InitCommands(session, config)
	if err != nil {
		log.Fatalf("Error while registering commands: %v", err)
	}
	log.Println("Commands created")

	go pollFeeds(session, config, *feeds)
	log.Println("Bot finished init")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("Bot shut down")
}

func initHTTP(config config.Config) *http.Client {
	tr := &http.Transport{
		MaxIdleConns: config.HTTP.MaxIdleConns,
	}
	return &http.Client{
		Transport: tr,
		Timeout:   time.Duration(config.HTTP.Timeout) * time.Second,
	}
}

func pollFeeds(s *discordgo.Session, config config.Config, feeds hytale.HytaleFeeds) {
	ticker := time.NewTicker(time.Duration(config.Feeds.Interval) * time.Second)
	defer ticker.Stop()

	if config.Feeds.PollOnStartup {
		poll(s, feeds)
	}
	for range ticker.C {
		poll(s, feeds)
	}
}

func poll(s *discordgo.Session, feeds hytale.HytaleFeeds) {
	log.Println("Polling feeds...")
	err := feeds.Poll()
	if err != nil {
		log.Printf("Error while polling launcher release: %v", err)
		return
	}
	err = feeds.NotifyFeeds(s)
	if err != nil {
		log.Printf("Error while notifying channels: %v", err)
	}
}
