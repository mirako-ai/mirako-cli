package voice

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [profile-id]",
	Short: "Delete a voice profile by its unique ID",
	Long:  `Delete a custom voice profile by its unique ID. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	Run:   runDelete,
}

var forceDelete bool

func runDelete(cmd *cobra.Command, args []string) {
	profileID := args[0]

	if !forceDelete {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to delete voice profile %s? This action cannot be undone.", profileID),
			Default: false,
		}
		if err := survey.AskOne(prompt, &confirm); err != nil {
			fmt.Printf("Error getting confirmation: %v\n", err)
			os.Exit(1)
		}
		if !confirm {
			fmt.Println("Deletion cancelled")
			return
		}
	}

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

	_, err = client.DeleteVoiceProfile(cmd.Context(), profileID)
	if err != nil {
		fmt.Printf("Error deleting voice profile: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted voice profile: %s\n", profileID)
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Skip confirmation prompt")
}