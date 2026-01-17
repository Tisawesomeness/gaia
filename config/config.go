package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	Token  string `json:"token"`
	Valkey struct {
		Address  string `json:"address"`
		Port     int    `json:"port"`
		Password string `json:"password"`
	} `json:"valkey"`
	Feeds struct {
		Interval        int    `json:"interval"` // in seconds
		LauncherRelease string `json:"launcher_release"`
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
		config.Token = envToken
	}

	return config, nil
}
