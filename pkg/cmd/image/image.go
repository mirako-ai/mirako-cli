package image

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/spf13/cobra"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewImageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "image",
		Short: "Manage images",
		Long:  `Generate and manage AI images`,
	}

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new image",
		Long:  `Generate a new image using AI and save it to disk`,
		RunE:  runGenerate,
	}

	cmd.Flags().StringP("prompt", "p", "", "Prompt for image generation")
	cmd.Flags().StringP("aspect-ratio", "a", "16:9", "Aspect ratio for the image (1:1, 16:9, 2:3, 3:2, 3:4, 4:3, 9:16)")
	cmd.Flags().Int64P("seed", "s", 0, "Seed for reproducible generation (optional)")
	cmd.Flags().StringP("output", "o", "", "Output file path for the generated image (e.g., ./output/image.jpg)")
	cmd.Flags().BoolP("no-save", "n", false, "Skip saving the image to disk")
	cmd.Flags().IntP("poll-interval", "i", 2, "Polling interval in seconds for checking status")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" {
		return fmt.Errorf("prompt is required. Use --prompt flag")
	}

	aspectRatioStr, _ := cmd.Flags().GetString("aspect-ratio")
	aspectRatio := api.AsyncGenerateImageApiRequestBodyAspectRatio(aspectRatioStr)
	outputPath, _ := cmd.Flags().GetString("output")
	noSave, _ := cmd.Flags().GetBool("no-save")
	pollInterval, _ := cmd.Flags().GetInt("poll-interval")

	seed, _ := cmd.Flags().GetInt64("seed")
	var seedPtr *int64
	if seed != 0 {
		seedPtr = &seed
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start generation
	fmt.Printf("🚀 Starting image generation...\n")
	resp, err := client.GenerateImage(ctx, prompt, aspectRatio, seedPtr)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to generate image: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.Data.TaskId
	fmt.Printf("✅ Image generation started!\n")
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
			statusResp, err := client.GetImageStatus(ctx, taskID)
			if err != nil {
				fmt.Print(clearLine) // Clear the spinner line
				if apiErr, ok := errors.IsAPIError(err); ok {
					return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
				}
				return fmt.Errorf("failed to check status: %w", err)
			}

			if statusResp.Data == nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(statusResp.Data.Status)

			if statusResp.Data.Status == api.GenerateTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine) // Clear the spinner line
				fmt.Printf("✅ Generation completed!\n")

				if statusResp.Data.Image != nil {
					if noSave {
						fmt.Printf("📸 Image generated (%d bytes) - skipping save due to --no-save flag\n", len(*statusResp.Data.Image))
						return nil
					}

					// Save the image
					imageData := *statusResp.Data.Image

					// Determine output path
					if outputPath == "" {
						now := time.Now()
						timestamp := fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/1000000)
						defaultFilename := fmt.Sprintf("image_%s.jpg", timestamp)
						outputPath = filepath.Join(cfg.DefaultSavePath, defaultFilename)
					}

					// Ensure .jpg extension
					if !strings.HasSuffix(strings.ToLower(outputPath), ".jpg") && !strings.HasSuffix(strings.ToLower(outputPath), ".jpeg") {
						outputPath += ".jpg"
					}

					// Decode base64 image
					// Remove data URL prefix if present
					if strings.HasPrefix(imageData, "data:image") {
						commaIndex := strings.Index(imageData, ",")
						if commaIndex != -1 {
							imageData = imageData[commaIndex+1:]
						}
					}

					decodedImage, err := base64.StdEncoding.DecodeString(imageData)
					if err != nil {
						return fmt.Errorf("failed to decode image data: %w", err)
					}

					// Create directory if it doesn't exist
					dir := filepath.Dir(outputPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}

					// Save the file
					if err := os.WriteFile(outputPath, decodedImage, 0644); err != nil {
						return fmt.Errorf("failed to save image: %w", err)
					}

					fmt.Printf("💾 Image saved to: %s\n", outputPath)
				}

				return nil
			} else if statusResp.Data.Status == api.GenerateTaskOutputStatusFAILED ||
				statusResp.Data.Status == api.GenerateTaskOutputStatusCANCELED ||
				statusResp.Data.Status == api.GenerateTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("image generation failed with status: %s", statusResp.Data.Status)
			}
			// Update status but don't draw here - spinner ticker handles animation
		case <-spinnerTicker.C:
			// Update spinner animation smoothly
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Status: %s", frame, currentStatus)
			spinnerIndex++
		}
	}
}

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [task-id]",
		Short: "Check image generation status",
		Long:  `Check the status of an image generation task`,
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	taskID := args[0]

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GetImageStatus(ctx, taskID)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to get status: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("Task ID: %s\n", resp.Data.TaskId)

	if resp.Data.Image != nil {
		fmt.Printf("✅ Image generated successfully!\n")
		fmt.Printf("   Image: %d bytes\n", len(*resp.Data.Image))

		// Ask user if they want to save the image
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nWould you like to save the generated image? (Y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			// Generate default filename
			defaultFilename := fmt.Sprintf("image_%s.jpg", taskID)
			defaultPath := filepath.Join(cfg.DefaultSavePath, defaultFilename)

			// Ask for save location
			fmt.Printf("Enter save path [%s]: ", defaultPath)
			savePath, _ := reader.ReadString('\n')
			savePath = strings.TrimSpace(savePath)

			if savePath == "" {
				savePath = defaultPath
			}

			// Ensure the path ends with .jpg
			if !strings.HasSuffix(strings.ToLower(savePath), ".jpg") && !strings.HasSuffix(strings.ToLower(savePath), ".jpeg") {
				savePath += ".jpg"
			}

			// Decode base64 image
			imageData := *resp.Data.Image
			// Remove data URL prefix if present
			if strings.HasPrefix(imageData, "data:image") {
				commaIndex := strings.Index(imageData, ",")
				if commaIndex != -1 {
					imageData = imageData[commaIndex+1:]
				}
			}

			decodedImage, err := base64.StdEncoding.DecodeString(imageData)
			if err != nil {
				return fmt.Errorf("failed to decode image data: %w", err)
			}

			// Create directory if it doesn't exist
			dir := filepath.Dir(savePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			// Save the file
			if err := os.WriteFile(savePath, decodedImage, 0644); err != nil {
				return fmt.Errorf("failed to save image: %w", err)
			}

			fmt.Printf("✅ Image saved to: %s\n", savePath)
		} else {
			fmt.Println("Image not saved.")
		}
	}

	return nil
}
