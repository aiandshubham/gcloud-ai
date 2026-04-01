package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

const defaultModel = "gemini-2.5-pro"

type Config struct {
	GeminiAPIKey   string `yaml:"gemini_api_key"`
	GeminiModel    string `yaml:"gemini_model"`
	DefaultProject string `yaml:"default_project"`
	DefaultRegion  string `yaml:"default_region"`
	DefaultCluster string `yaml:"default_cluster"`
}

var loaded *Config

// Load reads ~/.gai/config.yml if it exists, returns defaults otherwise.
// Result is cached so the file is only read once per invocation.
func Load() *Config {
	if loaded != nil {
		return loaded
	}

	loaded = &Config{
		GeminiModel: defaultModel, // sensible default
	}

	path := os.Getenv("HOME") + "/.gai/config.yml"
	data, err := os.ReadFile(path)
	if err != nil {
		// No config file — that's fine, use defaults
		return loaded
	}

	// Unmarshal but don't fail if some fields are missing
	_ = yaml.Unmarshal(data, loaded)

	// Always ensure model has a value
	if loaded.GeminiModel == "" {
		loaded.GeminiModel = defaultModel
	}

	return loaded
}

// GeminiAPIKey returns the API key using this priority:
//  1. GEMINI_API_KEY env var
//  2. gemini_api_key in ~/.gai/config.yml
func GetGeminiAPIKey() string {
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		return key
	}
	return Load().GeminiAPIKey
}
