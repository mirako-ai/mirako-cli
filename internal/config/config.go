package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type InteractiveProfile struct {
	AvatarID       string  `mapstructure:"avatar_id" yaml:"avatar_id"`
	Model          string  `mapstructure:"model" yaml:"model"`
	LLMModel       string  `mapstructure:"llm_model" yaml:"llm_model"`
	VoiceProfileID string  `mapstructure:"voice_profile_id" yaml:"voice_profile_id"`
	Instruction    string  `mapstructure:"instruction" yaml:"instruction"`
	Tools          string  `mapstructure:"tools" yaml:"tools"`
	IdleTimeout    int64   `mapstructure:"idle_timeout" yaml:"idle_timeout"`
}

type Config struct {
	APIToken            string                        `mapstructure:"api_token" yaml:"api_token"`
	APIURL              string                        `mapstructure:"api_url" yaml:"api_url"`
	DefaultModel        string                        `mapstructure:"default_model" yaml:"default_model"`
	DefaultVoice        string                        `mapstructure:"default_voice" yaml:"default_voice"`
	DefaultSavePath     string                        `mapstructure:"default_save_path" yaml:"default_save_path"`
	InteractiveProfiles map[string]InteractiveProfile `mapstructure:"interactive_profiles" yaml:"interactive_profiles"`
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
		APIURL:              "https://mirako.co",
		DefaultModel:        "metis-2.5",
		DefaultVoice:        "",
		DefaultSavePath:     ".",
		InteractiveProfiles: map[string]InteractiveProfile{
			"default": {
				Model:       "metis-2.5",
				IdleTimeout: 15,
			},
		},
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
	if cfg.DefaultVoice != "" {
		viper.SetDefault("default_voice", cfg.DefaultVoice)
	}

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
	if c.DefaultVoice != "" {
		viper.Set("default_voice", c.DefaultVoice)
	}
	viper.Set("default_save_path", c.DefaultSavePath)
	viper.Set("interactive_profiles", c.InteractiveProfiles)

	if err := viper.WriteConfigAs(ConfigFile); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (c *Config) IsAuthenticated() bool {
	return c.APIToken != ""
}
