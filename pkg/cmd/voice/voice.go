package voice

import (
	"fmt"
	"strings"

	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-cli/pkg/ui"
	"github.com/spf13/cobra"
)

const voiceCloningRedesignMessage = "Voice cloning feature is being redesigned and will be available soon"

// Language code to friendly label mapping
var languageLabels = map[string]string{
	"en":  "English",
	"yue": "Cantonese",
	"zh":  "Mandarin",
}

// formatLanguages converts language codes to friendly labels
func formatLanguages(languages *[]string) string {
	if languages == nil || len(*languages) == 0 {
		return ""
	}

	labels := make([]string, len(*languages))
	for i, lang := range *languages {
		if label, ok := languageLabels[lang]; ok {
			labels[i] = label
		} else {
			labels[i] = lang // fallback to original code if no mapping found
		}
	}

	return strings.Join(labels, ", ")
}

func NewVoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage voice profiles",
		Long:  `Manage premade and custom voice profiles for TTS`,
	}

	cmd.AddCommand(newListProfilesCmd())
	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newCloneVoiceCmd())
	cmd.AddCommand(viewCmd)
	cmd.AddCommand(deleteCmd)

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

	cfg, err := util.GetConfig(cmd)
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
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to list voice profiles: %w", err)
	}

	if resp == nil || resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No voice profiles found")
		return nil
	}

	t := ui.NewVoiceProfileTable(cmd.OutOrStdout())
	for _, profile := range *resp.Data {
		name := ""
		if profile.Name != nil {
			name = *profile.Name
		}
		description := ""
		if profile.Description != nil {
			description = *profile.Description
		}
		languages := formatLanguages(profile.Languages)
		t.AddRow([]interface{}{
			profile.Id,
			name,
			description,
			languages,
		})
	}
	t.Flush()
	return nil
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List custom voice profiles",
		Long:  `List your custom (cloned) voice profiles for TTS`,
		RunE:  runListCustomProfiles,
	}
}

func runListCustomProfiles(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListVoiceProfiles(ctx)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to list voice profiles: %w", err)
	}

	if resp == nil || resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No custom voice profiles found")
		return nil
	}

	t := ui.NewVoiceProfileTable(cmd.OutOrStdout())
	for _, profile := range *resp.Data {
		name := ""
		if profile.Name != nil {
			name = *profile.Name
		}
		description := ""
		if profile.Description != nil {
			description = *profile.Description
		}
		languages := formatLanguages(profile.Languages)
		t.AddRow([]interface{}{
			profile.Id,
			name,
			description,
			languages,
		})
	}
	t.Flush()
	return nil
}

func newCloneVoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Voice cloning is being redesigned",
		Long:  voiceCloningRedesignMessage,
		RunE:  runCloneVoice,
	}

	cmd.Flags().StringP("name", "n", "", "Ignored legacy flag")
	cmd.Flags().StringP("audio-dir", "a", "", "Ignored legacy flag")
	cmd.Flags().StringP("annotations", "t", "", "Ignored legacy flag")
	cmd.Flags().StringP("poll-interval", "p", "10", "Ignored legacy flag")
	cmd.Flags().StringP("clean-data", "c", "false", "Ignored legacy flag")
	cmd.Flags().Lookup("clean-data").NoOptDefVal = "true"
	cmd.Flags().StringP("description", "d", "", "Ignored legacy flag")

	for _, name := range []string{"name", "audio-dir", "annotations", "poll-interval", "clean-data", "description"} {
		cmd.Flags().MarkHidden(name)
	}

	return cmd
}

func runCloneVoice(cmd *cobra.Command, args []string) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), voiceCloningRedesignMessage)
	return err
}
