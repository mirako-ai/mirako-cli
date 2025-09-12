package auth

import (
	"fmt"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/spf13/cobra"
)

func NewAuthCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Manage authentication",
		Long:  `Login, logout, and check authentication status`,
	}

	cmd.AddCommand(newLoginCmd())
	cmd.AddCommand(newLogoutCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newLoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Mirako API",
		Long:  `Authenticate with your API token to access Mirako services`,
		RunE:  runLogin,
	}

	cmd.Flags().String("token", "", "API token (optional, can be provided interactively)")

	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	// Check if token is provided via flag
	token, _ := cmd.Flags().GetString("token")
	if token == "" {
		// Interactive prompt for token
		prompt := &survey.Input{
			Message: "API Token:",
			Help:    "Your Mirako API token. You can find it in your dashboard.",
		}
		if err := survey.AskOne(prompt, &token, survey.WithValidator(survey.Required)); err != nil {
			return fmt.Errorf("failed to get token: %w", err)
		}
	}

	// Save token to config
	cfg.APIToken = token
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("✅ Successfully authenticated!")
	return nil
}

func newLogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove authentication",
		Long:  `Remove stored API token and logout`,
		RunE:  runLogout,
	}
}

func runLogout(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	cfg.APIToken = ""
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Println("✅ Successfully logged out!")
	return nil
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check authentication status",
		Long:  `Check if you're currently authenticated`,
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	if cfg.IsAuthenticated() {
		fmt.Println("✅ Authenticated")
		fmt.Printf("   API URL: %s\n", cfg.APIURL)
		fmt.Printf("   Config Path: %s\n", config.ConfigPath)
	} else {
		fmt.Println("❌ Not authenticated")
		fmt.Println("   Run 'mirako auth login' to authenticate")
	}

	return nil
}

