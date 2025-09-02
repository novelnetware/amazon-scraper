package config

import (
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

// ProviderConfig holds the configuration for a single AI provider.
type ProviderConfig struct {
	Name   string `yaml:"name"`
	ApiURL string `yaml:"api_url"`
	ApiKey string `yaml:"api_key"`
	Model  string `yaml:"model"`
}

// ScraperConfig holds general scraper settings.
type ScraperConfig struct {
	Workers  string `yaml:"workers"`
	Headless bool   `yaml:"headless"`
}

// AmazonConfig holds settings specific to Amazon.
type AmazonConfig struct {
	BaseURL string `yaml:"base_url"`
}

// Config is the complete structure for the config.yml file.
type Config struct {
	Scraper    ScraperConfig `yaml:"scraper"`
	Amazon     AmazonConfig  `yaml:"amazon"`
	Translator struct {
		PrimaryProvider   string           `yaml:"primary_provider"`
		FallbackProviders []string         `yaml:"fallback_providers"`
		Providers         []ProviderConfig `yaml:"providers"`
	} `yaml:"translator"`
	Server struct {
		ApiKey string `yaml:"api_key"`
	} `yaml:"server"`
}

// LoadConfig remains the same
func LoadConfig(filepath string) *Config {
	data, err := os.ReadFile(filepath)
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		log.Fatalf("Error unmarshalling config YAML: %v", err)
	}
	return &cfg
}
