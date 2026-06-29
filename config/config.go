package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/tidwall/jsonc"
)

type BreakerConfig struct {
	Enabled             bool    `json:"enabled"`
	MaxHalfOpenRequests uint32  `json:"max_half_open_requests"`
	ResetInterval       int     `json:"reset_interval"`
	Timeout             int     `json:"timeout"`
	FailureRatio        float64 `json:"failure_ratio"`
}

type CredentialsConfig struct {
	DiscordToken          string `json:"discord_token"`
	AuthMethod            string `json:"auth_method"`
	StartRedirectListener bool   `json:"start_redirect_listener"`
	ProfileUUID           string `json:"profile_uuid"`
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

type DiscordConfig struct {
	InteractionExpiryTime      int `json:"interaction_expiry_time"`
	InteractionCleanupInterval int `json:"interaction_cleanup_interval"`
}

type AuthConfig struct {
	OAuthRefreshBuffer       int           `json:"oauth_refresh_buffer"`
	GameSessionRefreshBuffer int           `json:"game_session_refresh_buffer"`
	Breaker                  BreakerConfig `json:"breaker"`
	DeviceAuth               string        `json:"device_auth"`
	DeviceAuthTimeout        int           `json:"device_auth_timeout"`
	BrowserAuth              string        `json:"browser_auth"`
	BrowserAuthTimeout       int           `json:"browser_auth_timeout"`
	RedirectURI              string        `json:"redirect_uri"`
	Token                    string        `json:"token"`
	Profiles                 string        `json:"profiles"`
	LauncherData             string        `json:"launcher_data"`
	CreateGameSession        string        `json:"create_game_session"`
	RefreshGameSession       string        `json:"refresh_game_session"`
}

type ProfileConfig struct {
	ByUUID         string `json:"by_uuid"`
	ByUsername     string `json:"by_username"`
	Hyvatar        string `json:"hyvatar"`
	ProfileWebsite string `json:"profile_website"`
}

type ServersConfig struct {
	Discovery string `json:"discovery"`
}

type FeedsConfig struct {
	PollOnStartup      bool   `json:"poll_on_startup"`
	NotifyOnStartup    bool   `json:"notify_on_startup"`
	Interval           int    `json:"interval"`
	Patchlines         string `json:"patchlines"`
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
	Discord                 DiscordConfig     `json:"discord"`
	Auth                    AuthConfig        `json:"auth"`
	Profile                 ProfileConfig     `json:"profile"`
	Servers                 ServersConfig     `json:"servers"`
	Feeds                   FeedsConfig       `json:"feeds"`
	HTTP                    HTTPConfig        `json:"http"`
}

var defaultConfig = Config{
	IsSelfHosted: true,
}

func LoadConfig() (Config, error) {
	configFile, err := readConfig()
	if err != nil {
		return Config{}, err
	}
	config := defaultConfig
	err = json.Unmarshal(jsonc.ToJSON(configFile), &config)
	if err != nil {
		return Config{}, err
	}

	if envToken, exists := os.LookupEnv("GAIA_DISCORD_TOKEN"); exists {
		log.Println("Overridden discord token from env vars")
		config.Credentials.DiscordToken = envToken
	}
	if envToken, exists := os.LookupEnv("GAIA_PROFILE_UUID"); exists {
		log.Println("Overridden profile UUID from env vars")
		config.Credentials.ProfileUUID = envToken
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
	if config.Credentials.AuthMethod != "launcher" && config.Credentials.AuthMethod != "server" {
		return Config{}, fmt.Errorf("Auth method must be \"launcher\" or \"server\" but was \"%s\"", config.Credentials.AuthMethod)
	}

	return config, nil
}

func readConfig() ([]byte, error) {
	configFile, err1 := os.ReadFile("config.jsonc")
	if err1 == nil {
		return configFile, nil
	}
	configFile, err2 := os.ReadFile("config.json")
	if err2 == nil {
		return configFile, nil
	}
	return nil, fmt.Errorf("Could not read either config.jsonc or config.json: %v, %v", err1, err2)
}
