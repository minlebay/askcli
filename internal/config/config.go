package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	Gemini GeminiConfig `mapstructure:"gemini"`
	Prompt PromptConfig `mapstructure:"prompt"`
	Alias  AliasConfig  `mapstructure:"alias"`
	UI     UIConfig     `mapstructure:"ui"`
}

type GeminiConfig struct {
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

type PromptConfig struct {
	Template      string `mapstructure:"template"`
	SystemMessage string `mapstructure:"system_message"`
}

type AliasConfig struct {
	Name    string `mapstructure:"name"`
	Enabled bool   `mapstructure:"enabled"`
}

type UIConfig struct {
	ShowTokens bool   `mapstructure:"show_tokens"`
	CopyHint   string `mapstructure:"copy_hint"`
}

var cfg *Config

// Load loads configuration from the config file (optional) and environment.
// The API key is not part of the configuration: it lives in KWallet.
func Load() (*Config, error) {
	if cfg != nil {
		return cfg, nil
	}

	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// Config search paths
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPaths := []string{
		filepath.Join(homeDir, ".config", "ask"),
		homeDir,
	}

	for _, path := range configPaths {
		viper.AddConfigPath(path)
	}

	// The API key is not read from the environment or the config file: it
	// lives in KWallet. ASK_GEMINI_MODEL remains available for model choice.
	viper.BindEnv("gemini.model", "ASK_GEMINI_MODEL")

	// Default values
	setDefaults()

	// Read config (optional if file doesn't exist)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("error reading config: %w", err)
		}
		// File not found - use default values and environment variables
	}

	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error parsing config: %w", err)
	}

	return cfg, nil
}

func setDefaults() {
	viper.SetDefault("gemini.model", "gemini-3.5-flash-lite")
	viper.SetDefault("prompt.template", "ask > ")
	viper.SetDefault("prompt.system_message", "You are a Linux terminal assistant. Answer only with terminal commands without additional explanations. If the question is not about terminal commands, answer briefly.")
	viper.SetDefault("alias.name", "ask")
	viper.SetDefault("alias.enabled", true)
	viper.SetDefault("ui.show_tokens", true)
	viper.SetDefault("ui.copy_hint", "[C] copy, [Q] quit")
}

// Get returns the loaded configuration
func Get() *Config {
	return cfg
}
