package voice

import (
	"fmt"
	"os"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-cli/pkg/ui"
	"github.com/spf13/cobra"
)

func NewVoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "voice",
		Short: "Manage voice services",
		Long:  `Manage voice profiles and voice cloning services`,
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
		t.AddRow([]interface{}{
			profile.Id,
			name,
			description,
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
		t.AddRow([]interface{}{
			profile.Id,
			name,
			description,
		})
	}
	t.Flush()
	return nil
}

func newCloneVoiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clone",
		Short: "Clone a voice from audio samples",
		Long: `Clone a voice by providing audio samples and annotations.

This command creates a new custom voice profile by training on provided audio samples.
The process is asynchronous and may take significant time to complete.

Required files:
- Audio samples: At least 6 .wav files in the specified directory
- Annotations: text file with training annotations

Example usage:
  mirako voice clone --name "My Voice" --audio-dir ./samples/ --annotations ./annotations.txt
  mirako voice clone --name "My Voice" --audio-dir ./samples/ --annotations ./annotations.txt --clean-data

The command will:
1. Scan the audio directory for .wav files
2. Upload files and start training
3. Poll status until completion
4. Display the new voice profile ID`,
		RunE: runCloneVoice,
	}

	cmd.Flags().StringP("name", "n", "", "Name for the new voice profile (3-64 characters)")
	cmd.Flags().StringP("audio-dir", "a", "", "Directory containing .wav audio sample files")
	cmd.Flags().StringP("annotations", "t", "", "Path to annotation file")
	cmd.Flags().IntP("poll-interval", "p", 10, "Polling interval in seconds for checking status")
	cmd.Flags().BoolP("clean-data", "c", false, "Enable de-noise processing (default: false)")

	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("audio-dir")
	cmd.MarkFlagRequired("annotations")

	return cmd
}

func runCloneVoice(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	name, _ := cmd.Flags().GetString("name")
	audioDir, _ := cmd.Flags().GetString("audio-dir")
	annotations, _ := cmd.Flags().GetString("annotations")
	pollInterval, _ := cmd.Flags().GetInt("poll-interval")
	cleanData, _ := cmd.Flags().GetBool("clean-data")

	// Validate name length
	if len(name) < 3 || len(name) > 64 {
		return fmt.Errorf("name must be between 3 and 64 characters")
	}

	// Validate directories and files exist
	if _, err := os.Stat(audioDir); os.IsNotExist(err) {
		return fmt.Errorf("audio directory does not exist: %s", audioDir)
	}
	if _, err := os.Stat(annotations); os.IsNotExist(err) {
		return fmt.Errorf("annotations file does not exist: %s", annotations)
	}

	// Scan audio files and show count
	audioFiles, err := client.ScanAudioFiles(audioDir)
	if err != nil {
		return fmt.Errorf("failed to scan audio files: %w", err)
	}

	if len(audioFiles) < 6 {
		return fmt.Errorf("at least 6 .wav files are required for voice cloning. Found: %d", len(audioFiles))
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start voice cloning
	fmt.Printf("🎤 Starting voice cloning...\n")
	fmt.Printf("   Name: %s\n", name)
	fmt.Printf("   Audio directory: %s\n", audioDir)
	fmt.Printf("   Annotations file: %s\n", annotations)
	fmt.Printf("   Found %d .wav files\n", len(audioFiles))
	fmt.Printf("   Clean data: %t\n", cleanData)

	resp, err := client.CloneVoice(ctx, name, audioDir, annotations, cleanData)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to start voice cloning: %w", err)
	}

	if resp == nil || resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.Data.TaskId
	fmt.Printf("✅ Voice cloning started!\n")
	fmt.Printf("   Task ID: %s\n", taskID)

	// Poll for status until complete
	fmt.Printf("⏳ Waiting for training to complete...\n")

	// Use separate tickers for polling and spinner animation
	pollTicker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	defer pollTicker.Stop()
	defer spinnerTicker.Stop()

	spinnerFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIndex := 0
	currentStatus := "IN_QUEUE"
	clearLine := "\r\033[K"

	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine)
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-pollTicker.C:
			statusResp, err := client.GetVoiceCloneStatus(ctx, taskID)
			if err != nil {
				fmt.Print(clearLine)
				if apiErr, ok := errors.IsAPIError(err); ok {
					return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
				}
				return fmt.Errorf("failed to check status: %w", err)
			}

			if statusResp == nil || statusResp.Data == nil {
				fmt.Print(clearLine)
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(statusResp.Data.Status)

			if statusResp.Data.Status == api.FinetuningTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine)
				fmt.Printf("✅ Voice cloning completed!\n")
				fmt.Printf("   Profile ID: %s\n", *statusResp.Data.ProfileId)
				fmt.Printf("   Task completed successfully\n")
				return nil
			} else if statusResp.Data.Status == api.FinetuningTaskOutputStatusFAILED ||
				statusResp.Data.Status == api.FinetuningTaskOutputStatusCANCELED ||
				statusResp.Data.Status == api.FinetuningTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine)
				if statusResp.Data.Error != nil && *statusResp.Data.Error != "" {
					return fmt.Errorf("voice cloning failed: %s", *statusResp.Data.Error)
				}
				return fmt.Errorf("voice cloning failed with status: %s", statusResp.Data.Status)
			}
			// Continue polling for other statuses
		case <-spinnerTicker.C:
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Status: %s", frame, currentStatus)
			spinnerIndex++
		}
	}
}