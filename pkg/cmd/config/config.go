package config

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
)

func NewConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
		Long:  `View and modify configuration settings`,
	}

	cmd.AddCommand(newSetCmd())
	cmd.AddCommand(newGetCmd())
	cmd.AddCommand(newListCmd())

	return cmd
}

func newSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set [key] [value]",
		Short: "Set a configuration value",
		Long:  `Set a configuration value in the config file`,
		Args:  cobra.ExactArgs(2),
		RunE:  runSet,
	}
}

func runSet(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	key := strings.ToLower(args[0])
	value := args[1]

	switch key {
	case "api-token":
		cfg.APIToken = value
	case "api-url":
		cfg.APIURL = value
	case "default-model":
		cfg.DefaultModel = value
	case "default-voice":
		cfg.DefaultVoice = value
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("✅ Set %s = %s\n", key, value)
	return nil
}

func newGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Get a configuration value",
		Long:  `Get a configuration value from the config file`,
		Args:  cobra.ExactArgs(1),
		RunE:  runGet,
	}
}

func runGet(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	key := strings.ToLower(args[0])

	switch key {
	case "api-token":
		if cfg.APIToken == "" {
			fmt.Println("(not set)")
		} else {
			fmt.Println("***") // Don't print actual token
		}
	case "api-url":
		fmt.Println(cfg.APIURL)
	case "default-model":
		fmt.Println(cfg.DefaultModel)
	case "default-voice":
		fmt.Println(cfg.DefaultVoice)
	default:
		return fmt.Errorf("unknown configuration key: %s", key)
	}

	return nil
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all configuration values",
		Long:  `List all configuration values from the config file`,
		RunE:  runList,
	}
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	fmt.Println("Configuration:")
	fmt.Printf("  api-url: %s\n", cfg.APIURL)
	fmt.Printf("  api-token: %s\n", formatToken(cfg.APIToken))
	fmt.Printf("  default-model: %s\n", cfg.DefaultModel)
	fmt.Printf("  default-voice: %s\n", cfg.DefaultVoice)

	return nil
}

func formatToken(token string) string {
	if token == "" {
		return "(not set)"
	}
	return "***"
}