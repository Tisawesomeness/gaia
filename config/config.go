package config

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Token      string `json:"token"`
	TestServer string `json:"test_server"`
	Valkey     struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Password string `json:"password"`
	} `json:"valkey"`
	Kratos struct {
		SessionCookie string `json:"initial_session_cookie"`
	} `json:"kratos"`
	Auth struct {
		OAuthRefreshBuffer       int    `json:"oauth_refresh_buffer"`        // in seconds
		GameSessionRefreshBuffer int    `json:"game_session_refresh_buffer"` // in seconds
		ClientID                 string `json:"client_id"`
		Scope                    string `json:"scope"`
		DeviceAuth               string `json:"device_auth"`
		Token                    string `json:"token"`
		Profiles                 string `json:"profiles"`
		CreateGameSession        string `json:"create_game_session"`
		RefreshGameSession       string `json:"refresh_game_session"`
	} `json:"auth"`
	Profile struct {
		ByUUID       string `json:"by_uuid"`
		ByUsername   string `json:"by_username"`
		Availability string `json:"availability"`
	} `json:"profile"`
	Feeds struct {
		PollOnStartup      bool   `json:"poll_on_startup"`
		Interval           int    `json:"interval"` // in seconds
		LauncherRelease    string `json:"launcher_release"`
		LauncherArticles   string `json:"launcher_articles"`
		ArticleImagePrefix string `json:"article_image_prefix"`
	} `json:"feeds"`
	HTTP struct {
		Timeout      int `json:"timeout"` // in seconds
		MaxIdleConns int `json:"max_idle_conns"`
	} `json:"http"`
}

func LoadConfig() (Config, error) {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		return Config{}, err
	}
	var config Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		return Config{}, err
	}

	if envToken, exists := os.LookupEnv("GAIA_DISCORD_TOKEN"); exists {
		log.Println("Overridden discord token from env vars")
		config.Token = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_TEST_SERVER"); exists {
		log.Println("Overridden test server from env vars")
		config.TestServer = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_VALKEY_PASS"); exists {
		log.Println("Overridden valkey password from env vars")
		config.Valkey.Password = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_KRATOS_COOKIE"); exists {
		log.Println("Overridden kratos session cookie from env vars")
		config.Kratos.SessionCookie = envToken
	}

	return config, nil
}
