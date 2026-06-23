package main

import (
	_ "embed"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/Tisawesomeness/gaia/auth"
	"github.com/Tisawesomeness/gaia/cmd"
	"github.com/Tisawesomeness/gaia/config"
	"github.com/Tisawesomeness/gaia/db"
	"github.com/Tisawesomeness/gaia/hytale"
	"github.com/Tisawesomeness/gaia/util"
	"github.com/bwmarrin/discordgo"
)

var (
	//go:embed version.txt
	versionRaw string
	version    = strings.TrimSpace(versionRaw)
)

func main() {
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	err = os.MkdirAll("./data", 0755)
	if err != nil {
		log.Fatalf("Could not create data directory: %v", err)
	}
	database, err := db.NewDB(config.Valkey)
	if err != nil {
		log.Fatalf("Error creating Valkey client: %v", err)
	}
	defer database.Close()
	log.Println("Connected to valkey")

	httpClient := initHTTP(config)

	authStore, err := auth.NewAuthStore(&config, database, httpClient)
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Could not create auth store: %v", err)
		os.Exit(1)
	}

	feeds, err := hytale.NewHytaleFeeds(&config, database, httpClient, authStore)
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Error creating Hytale feeds: %v", err)
		os.Exit(1)
	}

	session, err := discordgo.New("Bot " + config.Credentials.DiscordToken)
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Error starting bot: %v", err)
		os.Exit(1)
	}
	log.Println("Bot authenticated")

	bootTime := time.Now()
	commandExecutor := cmd.NewCommandExecutor(&config, database, httpClient, authStore, feeds, version, &bootTime)
	session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		commandExecutor.HandleInteractionCreate(s, i)
	})

	err = session.Open()
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Could not open session: %v", err)
		os.Exit(1)
	}
	defer session.Close()
	log.Println("Opened Discord session")

	err = session.UpdateGameStatus(0, config.Playing)
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Error setting bot activity: %v", err)
		os.Exit(1)
	}

	err = cmd.InitCommands(session, &config)
	if err != nil {
		util.DiscordLogf(&config, httpClient, "Error while registering commands: %v", err)
		os.Exit(1)
	}

	go pollFeeds(session, config, *feeds)
	util.DiscordLog(&config, httpClient, "Bot finished init")

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
		feeds.Poll()
	}
	if config.Feeds.NotifyOnStartup {
		feeds.NotifyFeeds(s)
	}
	for range ticker.C {
		feeds.Poll()
		feeds.NotifyFeeds(s)
	}
}
