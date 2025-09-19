package root

import (
	"fmt"
	"os"

	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/auth"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/avatar"
	configcmd "github.com/mirako-ai/mirako-cli/pkg/cmd/config"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/image"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/interactive"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/speech"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/video"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/voice"
	"github.com/spf13/cobra"
)

// These will be set by ldflags during build
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "mirako",
	Short: "Mirako CLI - Command line interface for Mirako AI services",
	Long: `Mirako CLI provides a command-line interface for Mirako AI services.

It allows you to:
- Create and manage AI avatars
- Start interactive sessions
- Generate images and videos
- Use speech-to-text and text-to-speech services
- Clone and manage voice profiles

For more information, visit: https://mirako.ai`,
	SilenceUsage: true,
	Version:      getVersionString(),
}

func Execute() error {
	return rootCmd.Execute()
}

func printVersion() {
	fmt.Printf("Mirako CLI %s\n", Version)
	fmt.Printf("Commit: %s\n", Commit)
	fmt.Printf("Built: %s\n", Date)
}

func getVersionString() string {
	return fmt.Sprintf("Mirako CLI %s\nCommit: %s\nBuilt: %s", Version, Commit, Date)
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version, commit hash, and build date information",
		Run: func(cmd *cobra.Command, args []string) {
			printVersion()
		},
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode")
	rootCmd.PersistentFlags().String("api-token", "", "API token for authentication")
	rootCmd.PersistentFlags().String("api-url", "", "API URL (default https://mirako.co)")

	// Add subcommands
	rootCmd.AddCommand(newVersionCmd())
	rootCmd.AddCommand(auth.NewAuthCmd())
	rootCmd.AddCommand(avatar.NewAvatarCmd())
	rootCmd.AddCommand(configcmd.NewConfigCmd())
	rootCmd.AddCommand(image.NewImageCmd())
	rootCmd.AddCommand(interactive.NewInteractiveCmd())
	rootCmd.AddCommand(speech.NewSpeechCmd())
	rootCmd.AddCommand(video.NewVideoCmd())
	rootCmd.AddCommand(voice.NewVoiceCmd())
}

func initConfig() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Override with command line flags
	apiToken, _ := rootCmd.Flags().GetString("api-token")
	if apiToken != "" {
		cfg.APIToken = apiToken
	}

	apiURL, _ := rootCmd.Flags().GetString("api-url")
	if apiURL != "" {
		cfg.APIURL = apiURL
	}
}
