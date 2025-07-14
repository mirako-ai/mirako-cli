package voice

import (
	"fmt"
	"text/tabwriter"

	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/spf13/cobra"
)

func NewVoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage voice services",
		Long:  `Manage voice profiles and voice cloning services`,
	}

	cmd.AddCommand(newListProfilesCmd())

	return cmd
}

func newListProfilesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "premade",
		Short: "List premade voice profiles",
		Long:  `List available premade voice profiles for TTS`,
		RunE:  runListProfiles,
	}
}

func runListProfiles(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListPremadeProfiles(ctx)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf(apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to list voice profiles: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil || len(*resp.JSON200.Data) == 0 {
		fmt.Println("No voice profiles found")
		return nil
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tDESCRIPTION")

	for _, profile := range *resp.JSON200.Data {
		name := ""
		if profile.Name != nil {
			name = *profile.Name
		}
		description := ""
		if profile.Description != nil {
			description = *profile.Description
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n",
			profile.Id,
			name,
			description,
		)
	}

	w.Flush()
	return nil
}