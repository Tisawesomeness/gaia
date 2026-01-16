package main

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
	API struct {
		Endpoint string `json:"endpoint"`
		Interval int    `json:"interval"` // Interval in seconds
	} `json:"api"`
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
