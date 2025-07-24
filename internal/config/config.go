package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type Config struct {
	APIToken        string `mapstructure:"api_token"`
	APIURL          string `mapstructure:"api_url"`
	DefaultModel    string `mapstructure:"default_model"`
	DefaultVoice    string `mapstructure:"default_voice"`
	DefaultSavePath string `mapstructure:"default_save_path"`
}

var (
	ConfigDir  string
	ConfigFile string
)

func init() {
	home, err := homedir.Dir()
	if err != nil {
		panic(err)
	}

	ConfigDir = filepath.Join(home, ".mirako")
	ConfigFile = filepath.Join(ConfigDir, "config.yml")
}

func Load() (*Config, error) {
	cfg := &Config{
		APIURL:          "https://mirako.co",
		DefaultModel:    "metis-2.5",
		DefaultVoice:    "mira-korner",
		DefaultSavePath: ".",
	}

	// Create config directory if it doesn't exist
	if err := os.MkdirAll(ConfigDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Configure viper
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	viper.AddConfigPath(ConfigDir)
	viper.SetEnvPrefix("MIRAKO")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("api_url", cfg.APIURL)
	viper.SetDefault("default_model", cfg.DefaultModel)
	viper.SetDefault("default_voice", cfg.DefaultVoice)

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	if err := viper.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save() error {
	viper.Set("api_token", c.APIToken)
	viper.Set("api_url", c.APIURL)
	viper.Set("default_model", c.DefaultModel)
	viper.Set("default_voice", c.DefaultVoice)
	viper.Set("default_save_path", c.DefaultSavePath)

	if err := viper.WriteConfigAs(ConfigFile); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (c *Config) IsAuthenticated() bool {
	return c.APIToken != ""
}

