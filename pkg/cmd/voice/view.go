package voice

import (
	"fmt"
	"os"

	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/spf13/cobra"
)

var viewCmd = &cobra.Command{
	Use:   "view [profile-id]",
	Short: "Get a specific voice profile by ID",
	Long:  `Get detailed information about a specific voice profile by its unique ID.`,
	Args:  cobra.ExactArgs(1),
	Run:   runView,
}

func runView(cmd *cobra.Command, args []string) {
	profileID := args[0]
	
	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}
	
	client, err := client.New(cfg)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	resp, err := client.GetVoiceProfile(cmd.Context(), profileID)
	if err != nil {
		fmt.Printf("Error getting voice profile: %v\n", err)
		os.Exit(1)
	}

	profile := resp.Data
	if profile.Id == "" {
		fmt.Println("Voice profile not found")
		os.Exit(1)
	}

	fmt.Printf("Voice Profile Details:\n")
	fmt.Printf("  ID: %s\n", profile.Id)
	fmt.Printf("  Name: %s\n", profile.Name)
	fmt.Printf("  Description: %s\n", profile.Description)
	fmt.Printf("  Status: %s\n", profile.Status)
	fmt.Printf("  Created: %s\n", profile.CreatedAt)
	fmt.Printf("  Premade: %t\n", profile.IsPremade)
	if profile.UserId != nil {
		fmt.Printf("  User ID: %s\n", *profile.UserId)
	}
	if profile.SampleClip != nil {
		fmt.Printf("  Sample: %s\n", *profile.SampleClip)
	}
}

func init() {
	// This will be registered in cmd.go
}