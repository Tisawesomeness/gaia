package config

import (
	"encoding/json"
	"errors"
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

type CredentialsConfig struct {
	DiscordToken    string `json:"discord_token"`
	HytaleEmail     string `json:"hytale_email"`
	HytalePassword  string `json:"hytale_password"`
	Hytale2FASecret string `json:"hytale_2fa_secret"`
}

type ValkeyConfig struct {
	Address       string `json:"address"`
	Port          int    `json:"port"`
	Password      string `json:"password"`
	DatabaseIndex int    `json:"database_index"`
}

type BrandingConfig struct {
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
}

type KratosConfig struct {
	RenewalBufferSeconds int           `json:"renewal_buffer_seconds"`
	Breaker              BreakerConfig `json:"breaker"`
	AccountsBackend      string        `json:"accounts_backend"`
}

type AuthConfig struct {
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
}

type ProfileConfig struct {
	ByUUID         string `json:"by_uuid"`
	ByUsername     string `json:"by_username"`
	Availability   string `json:"availability"`
	Hyvatar        string `json:"hyvatar"`
	ProfileWebsite string `json:"profile_website"`
}

type FeedsConfig struct {
	PollOnStartup      bool   `json:"poll_on_startup"`
	NotifyOnStartup    bool   `json:"notify_on_startup"`
	Interval           int    `json:"interval"`
	GameVersion        string `json:"game_version"`
	LauncherRelease    string `json:"launcher_release"`
	LauncherArticles   string `json:"launcher_articles"`
	ArticleImagePrefix string `json:"article_image_prefix"`
	MavenRepo          string `json:"maven_repo"`
	MavenGroup         string `json:"maven_group"`
	MavenArtifact      string `json:"maven_artifact"`
}

type HTTPConfig struct {
	Timeout      int `json:"timeout"`
	MaxIdleConns int `json:"max_idle_conns"`
}

type Config struct {
	Credentials             CredentialsConfig `json:"credentials"`
	Valkey                  ValkeyConfig      `json:"valkey"`
	TestServer              string            `json:"test_server"`
	LogWebhook              string            `json:"log_webhook"`
	IsSelfHosted            bool              `json:"is_self_hosted"`
	CreateCommandsOnStartup bool              `json:"create_commands_on_startup"`
	Playing                 string            `json:"playing"`
	Branding                BrandingConfig    `json:"branding"`
	Kratos                  KratosConfig      `json:"kratos"`
	Auth                    AuthConfig        `json:"auth"`
	Profile                 ProfileConfig     `json:"profile"`
	Feeds                   FeedsConfig       `json:"feeds"`
	HTTP                    HTTPConfig        `json:"http"`
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
		config.Credentials.DiscordToken = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_HYTALE_EMAIL"); exists {
		log.Println("Overridden Hytale email from env vars")
		config.Credentials.HytaleEmail = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_HYTALE_PASSWORD"); exists {
		log.Println("Overridden Hytale password from env vars")
		config.Credentials.HytalePassword = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_HYTALE_2FA_SECRET"); exists {
		log.Println("Overridden Hytale 2FA secret from env vars")
		config.Credentials.Hytale2FASecret = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_VALKEY_PASS"); exists {
		log.Println("Overridden valkey password from env vars")
		config.Valkey.Password = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_TEST_SERVER"); exists {
		log.Println("Overridden test server from env vars")
		config.TestServer = envToken
	}

	if config.Credentials.DiscordToken == "YOUR_DISCORD_TOKEN" || config.Credentials.DiscordToken == "" {
		return Config{}, errors.New("Must provide Discord token")
	}
	if config.Credentials.HytaleEmail == "YOUR_HYTALE_EMAIL" || config.Credentials.HytaleEmail == "" ||
		config.Credentials.HytalePassword == "YOUR_HYTALE_PASSWORD" || config.Credentials.HytalePassword == "" ||
		config.Credentials.Hytale2FASecret == "YOUR_HYTALE_2FA_SECRET" || config.Credentials.Hytale2FASecret == "" {
		log.Println("Either Hytale username, password, or 2FA secret were left unset, username availability will not work")
		config.Credentials.HytaleEmail = ""
		config.Credentials.HytalePassword = ""
		config.Credentials.Hytale2FASecret = ""
	}

	return config, nil
}
