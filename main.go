package main

import (
	"errors"
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

	gameSession, err := initAuth(config, httpClient)
	if err != nil {
		log.Fatalf("Auth error: %v", err)
	}
	log.Println("Created game session, expires at: " + gameSession.ExpiresAt)

	feeds, err := hytale.NewHytaleFeeds(config, *database, *httpClient)
	if err != nil {
		log.Fatalf("Error creating Hytale feeds: %v", err)
	}

	session, err := discordgo.New("Bot " + config.Token)
	if err != nil {
		log.Fatalf("Error starting bot: %v", err)
	}
	log.Println("Bot authenticated")

	ctx := &cmd.CommandContext{
		Config:      config,
		DB:          *database,
		HTTP:        *httpClient,
		HytaleFeeds: *feeds,
	}
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		cmd.HandleInteractionCreate(s, i, ctx)
	})

	err = session.Open()
	if err != nil {
		log.Fatalf("Could not open session: %v", err)
	}
	defer session.Close()
	log.Println("Bot started")

	err = cmd.InitCommands(session, config)
	if err != nil {
		log.Fatalf("Error while registering commands: %v", err)
	}

	go pollFeeds(session, config, *feeds)

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

func initAuth(config config.Config, httpClient *http.Client) (*auth.GameSessionResponse, error) {
	tokenResponse, err := auth.OAuthFlow(config, httpClient)
	if err != nil {
		return nil, err
	}
	profiles, err := auth.GetAccountProfiles(tokenResponse.AccessToken, config, httpClient)
	if err != nil {
		return nil, err
	}
	if len(profiles.Profiles) <= 0 {
		return nil, errors.New("No profiles found!")
	}
	log.Println("Found profiles:")
	for _, profile := range profiles.Profiles {
		log.Printf("%s - %s", profile.UUID, profile.Username)
	}
	uuid := profiles.Profiles[0].UUID
	log.Println("Using profile " + uuid)
	session, err := auth.CreateGameSession(tokenResponse.AccessToken, uuid, config, httpClient)
	if err != nil {
		return nil, err
	}
	return &session, err
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
