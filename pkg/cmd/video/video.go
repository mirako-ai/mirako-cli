package video

import (
	"encoding/base64"
	"fmt"
	"os"
	"time"

	"github.com/mirako-ai/mirako-cli/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/spf13/cobra"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewVideoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video",
		Short: "Manage videos",
		Long:  `Generate and manage AI videos including talking avatars`,
	}

	cmd.AddCommand(newGenerateTalkingAvatarCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newGenerateTalkingAvatarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate-talking",
		Short: "Generate a talking avatar video",
		Long:  `Generate a talking avatar video using AI and save it to disk`,
		RunE:  runGenerateTalkingAvatar,
	}

	cmd.Flags().StringP("audio", "a", "", "Path to the audio file for speech")
	cmd.Flags().StringP("image", "i", "", "Path to the image file for avatar face")
	cmd.Flags().StringP("output", "o", "", "Output file path for the generated video (e.g., ./output/video.mp4)")
	cmd.Flags().BoolP("no-save", "n", false, "Skip saving the video to disk")
	cmd.Flags().IntP("poll-interval", "p", 2, "Polling interval in seconds for checking status")

	return cmd
}

func runGenerateTalkingAvatar(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	audioPath, _ := cmd.Flags().GetString("audio")
	if audioPath == "" {
		return fmt.Errorf("audio path is required. Use --audio flag")
	}

	imagePath, _ := cmd.Flags().GetString("image")
	if imagePath == "" {
		return fmt.Errorf("image path is required. Use --image flag")
	}

	noSave, _ := cmd.Flags().GetBool("no-save")
	pollInterval, _ := cmd.Flags().GetInt("poll-interval")

	// Read and encode the audio file
	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}
	audioBase64 := base64.StdEncoding.EncodeToString(audioData)

	// Read and encode the image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start generation
	fmt.Printf("🚀 Starting talking avatar video generation...\n")
	resp, err := client.GenerateTalkingAvatar(ctx, audioBase64, imageBase64)
	if err != nil {
		return fmt.Errorf("failed to generate talking avatar video: %w", err)
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.JSON200.Data.TaskId
	fmt.Printf("✅ Talking avatar video generation started!\n")
	fmt.Printf("   Task ID: %s\n", taskID)

	// Poll for status until complete
	fmt.Printf("⏳ Waiting for generation to complete...\n")

	// Use separate tickers for polling and spinner animation
	pollTicker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	spinnerTicker := time.NewTicker(100 * time.Millisecond) // Smooth spinner animation
	defer pollTicker.Stop()
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	currentStatus := "PROCESSING" // Initial status
	clearLine := "\r\033[K"       // ANSI escape codes to clear the line

	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine) // Clear the spinner line
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-pollTicker.C:
			statusResp, err := client.GetTalkingAvatarStatus(ctx, taskID)
			if err != nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("failed to check status: %w", err)
			}

			if statusResp.JSON200 == nil || statusResp.JSON200.Data == nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(statusResp.JSON200.Data.Status)

			if statusResp.JSON200.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine) // Clear the spinner line
				fmt.Printf("✅ Generation completed!\n")

				if statusResp.JSON200.Data.FileUrl != nil {
					if noSave {
						fmt.Printf("🎥 Video generated - URL: %s\n", *statusResp.JSON200.Data.FileUrl)
						return nil
					}

					// Note: FileUrl is a URL, not base64 data
					fmt.Printf("🎥 Video generated successfully!\n")
					fmt.Printf("   Video URL: %s\n", *statusResp.JSON200.Data.FileUrl)
					if statusResp.JSON200.Data.OutputDuration != nil {
						fmt.Printf("   Duration: %.2f seconds\n", *statusResp.JSON200.Data.OutputDuration)
					}
				}

				return nil
			} else if statusResp.JSON200.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusFAILED || statusResp.JSON200.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCANCELED || statusResp.JSON200.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("talking avatar video generation failed with status: %s", statusResp.JSON200.Data.Status)
			}
			// Update status but don't draw here - spinner ticker handles animation
		case <-spinnerTicker.C:
			// Update spinner animation smoothly
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r%s Status: %s", frame, currentStatus)
			spinnerIndex++
		}
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Check talking avatar generation status",
		Long:  `Check the status of a talking avatar generation task`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	taskID := args[0]

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GetTalkingAvatarStatus(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("Task ID: %s\n", resp.JSON200.Data.TaskId)

	if resp.JSON200.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCOMPLETED {
		fmt.Printf("✅ Talking avatar video generated successfully!\n")
		if resp.JSON200.Data.FileUrl != nil {
			fmt.Printf("   Video URL: %s\n", *resp.JSON200.Data.FileUrl)
		}
		if resp.JSON200.Data.OutputDuration != nil {
			fmt.Printf("   Duration: %.2f seconds\n", *resp.JSON200.Data.OutputDuration)
		}
	}

	return nil
}

