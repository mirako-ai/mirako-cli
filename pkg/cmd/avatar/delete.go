package avatar

import (
	"fmt"
	"os"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [avatar-id]",
	Short: "Delete an avatar by its unique ID",
	Long:  `Delete an avatar by its unique ID. This action cannot be undone.`,
	Args:  cobra.ExactArgs(1),
	Run:   runDelete,
}

var forceDelete bool

func runDelete(cmd *cobra.Command, args []string) {
	avatarID := args[0]

	if !forceDelete {
		confirm := false
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Are you sure you want to delete avatar %s? This action cannot be undone.", avatarID),
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

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	client, err := client.New(cfg)
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	_, err = client.DeleteAvatar(cmd.Context(), avatarID)
	if err != nil {
		fmt.Printf("Error deleting avatar: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Successfully deleted avatar: %s\n", avatarID)
}

func init() {
	deleteCmd.Flags().BoolVarP(&forceDelete, "force", "f", false, "Skip confirmation prompt")
}