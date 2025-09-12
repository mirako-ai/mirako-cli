package util

import (
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/spf13/cobra"
)

// GetConfig loads configuration and applies flag overrides
func GetConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Apply flag overrides (similar to root.go)
	if cmd.Flags().Changed("api-token") {
		apiToken, _ := cmd.Flags().GetString("api-token")
		cfg.APIToken = apiToken
	}

	if cmd.Flags().Changed("api-url") {
		apiURL, _ := cmd.Flags().GetString("api-url")
		cfg.APIURL = apiURL
	}

	return cfg, nil
}

