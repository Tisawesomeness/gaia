package main

import (
	"encoding/json"
	"log"
	"os"
)

type Config struct {
	Token string `json:"token"`
}

func LoadConfig() Config {
	var config Config
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}
	return config
}
