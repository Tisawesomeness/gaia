package config

import (
	"encoding/json"
	"log"
	"os"
)

type BreakerConfig struct {
	Enabled             bool    `json:"enabled"`
	MaxHalfOpenRequests uint32  `json:"max_half_open_requests"`
	ResetInterval       int     `json:"reset_interval"`
	Timeout             int     `json:"timeout"`
	FailureRatio        float64 `json:"failure_ratio"`
}

type Config struct {
	Token        string `json:"token"`
	TestServer   string `json:"test_server"`
	IsSelfHosted bool   `json:"is_self_hosted"`
	Branding     struct {
		Author          string `json:"author"`
		AuthorTag       string `json:"author_tag"`
		Invite          string `json:"invite"`
		HelpServer      string `json:"help_server"`
		Website         string `json:"website"`
		Github          string `json:"github"`
		Issues          string `json:"issues"`
		HostingProvider string `json:"hosting_provider"`
		HostingWebsite  string `json:"hosting_website"`
		Terms           string `json:"terms"`
		Privacy         string `json:"privacy"`
	} `json:"branding"`
	Valkey struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Password string `json:"password"`
	} `json:"valkey"`
	Kratos struct {
		SessionCookie string        `json:"initial_session_cookie"`
		Breaker       BreakerConfig `json:"breaker"`
	} `json:"kratos"`
	Auth struct {
		OAuthRefreshBuffer       int           `json:"oauth_refresh_buffer"`
		GameSessionRefreshBuffer int           `json:"game_session_refresh_buffer"`
		Breaker                  BreakerConfig `json:"breaker"`
		ClientID                 string        `json:"client_id"`
		Scope                    string        `json:"scope"`
		DeviceAuth               string        `json:"device_auth"`
		Token                    string        `json:"token"`
		Profiles                 string        `json:"profiles"`
		CreateGameSession        string        `json:"create_game_session"`
		RefreshGameSession       string        `json:"refresh_game_session"`
	} `json:"auth"`
	Profile struct {
		ByUUID       string `json:"by_uuid"`
		ByUsername   string `json:"by_username"`
		Availability string `json:"availability"`
	} `json:"profile"`
	Feeds struct {
		PollOnStartup      bool   `json:"poll_on_startup"`
		Interval           int    `json:"interval"`
		LauncherRelease    string `json:"launcher_release"`
		LauncherArticles   string `json:"launcher_articles"`
		ArticleImagePrefix string `json:"article_image_prefix"`
	} `json:"feeds"`
	HTTP struct {
		Timeout      int `json:"timeout"`
		MaxIdleConns int `json:"max_idle_conns"`
	} `json:"http"`
}

var defaultConfig = Config{
	IsSelfHosted: true,
}

func LoadConfig() (Config, error) {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		return Config{}, err
	}
	config := defaultConfig
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
