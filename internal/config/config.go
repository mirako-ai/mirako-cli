package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

type InteractiveProfile struct {
	AvatarID       string `mapstructure:"avatar_id" yaml:"avatar_id"`
	Model          string `mapstructure:"model" yaml:"model"`
	LLMModel       string `mapstructure:"llm_model" yaml:"llm_model"`
	VoiceProfileID string `mapstructure:"voice_profile_id" yaml:"voice_profile_id"`
	Instruction    string `mapstructure:"instruction" yaml:"instruction"`
	Tools          string `mapstructure:"tools" yaml:"tools"`
	IdleTimeout    int64  `mapstructure:"idle_timeout" yaml:"idle_timeout"`
}

type Config struct {
	APIToken            string                        `mapstructure:"api_token" yaml:"api_token"`
	APIURL              string                        `mapstructure:"api_url" yaml:"api_url"`
	DefaultVoice        string                        `mapstructure:"default_voice" yaml:"default_voice"`
	DefaultSavePath     string                        `mapstructure:"default_save_path" yaml:"default_save_path"`
	InteractiveProfiles map[string]InteractiveProfile `mapstructure:"interactive_profiles" yaml:"interactive_profiles"`
}

var (
	ConfigPath              string // Path of config path used in this session. Contains value overriden by env var
	DefaultConfigFileName   string = "config.yml"
	DefaultLLMModel         string = "gemini-2.0-flash"
	DefaultInteractiveModel string = "metis-2.5"
)

func DefaultUserConfigDirPath() string {
	home, err := homedir.Dir()
	if err != nil {
		panic(err)
	}
	return filepath.Join(home, ".mirako")
}

func Load() (*Config, error) {

	cfg := &Config{
		APIURL:              "https://mirako.co",
		DefaultVoice:        "",
		DefaultSavePath:     ".",
		InteractiveProfiles: map[string]InteractiveProfile{},
	}

	// Configure viper
	viper.SetConfigName("config")
	viper.SetConfigType("yml")
	if envConfigPath := os.Getenv("MIRAKO_CONFIG_PATH"); envConfigPath != "" {
		ConfigPath = envConfigPath
	} else {
		ConfigPath = DefaultUserConfigDirPath()
	}
	viper.AddConfigPath(ConfigPath)

	// ENV started with MIRAKO_ will be automatically mapped to config keys
	viper.SetEnvPrefix("MIRAKO")
	viper.AutomaticEnv()

	// Set defaults
	viper.SetDefault("api_url", cfg.APIURL)
	if cfg.DefaultVoice != "" {
		viper.SetDefault("default_voice", cfg.DefaultVoice)
	}

	// Check if config file exists before trying to read it
	if _, err := os.Stat(filepath.Join(ConfigPath, DefaultConfigFileName)); os.IsNotExist(err) {
		// First run

		// add default interactive profile upon first config file write
		cfg.InteractiveProfiles["default"] = InteractiveProfile{
			Model:       "metis-2.5",
			IdleTimeout: 15,
		}

		if err := os.MkdirAll(filepath.Dir(ConfigPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}
		// write default config file to ConfigPath
		if err = cfg.Save(); err != nil {
			return nil, fmt.Errorf("failed to create default config file: %w", err)
		}
	} else {
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file at %s: %w", ConfigPath, err)
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
	if c.DefaultVoice != "" {
		viper.Set("default_voice", c.DefaultVoice)
	}
	viper.Set("default_save_path", c.DefaultSavePath)
	viper.Set("interactive_profiles", c.InteractiveProfiles)

	if err := viper.WriteConfigAs(filepath.Join(ConfigPath, DefaultConfigFileName)); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func (c *Config) IsAuthenticated() bool {
	return c.APIToken != ""
}
