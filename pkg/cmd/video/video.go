package video

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type VideoModel string

const (
	VideoModelTalkingAvatar VideoModel = "talking_avatar"
	VideoModelMotion        VideoModel = "motion"
)

func (m VideoModel) String() string {
	return string(m)
}

func (m VideoModel) IsValid() bool {
	switch m {
	case VideoModelTalkingAvatar, VideoModelMotion:
		return true
	default:
		return false
	}
}

func GetSupportedModels() []VideoModel {
	return []VideoModel{VideoModelTalkingAvatar, VideoModelMotion}
}

func GetSupportedModelsString() string {
	models := GetSupportedModels()
	modelStrs := make([]string, len(models))
	for i, m := range models {
		modelStrs[i] = m.String()
	}
	return strings.Join(modelStrs, ", ")
}

func NewVideoCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "video",
		Short: "Manage videos",
		Long:  `Generate and manage AI videos including talking avatars`,
	}

	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a video",
		Long:  `Generate AI videos using various models`,
		RunE:  runGenerate,
	}

	cmd.Flags().StringP("model", "m", "", fmt.Sprintf("Model type for video generation (%s)", GetSupportedModelsString()))
	cmd.Flags().StringP("audio", "a", "", "Path to the audio file for speech")
	cmd.Flags().StringP("image", "i", "", "Path to the image file for avatar face")
	cmd.Flags().StringP("positive-prompt", "", "", "Positive prompt to guide avatar motion generation (motion model only)")
	cmd.Flags().StringP("negative-prompt", "", "", "Negative prompt to guide avatar motion generation (motion model only)")
	cmd.Flags().StringP("output", "o", "", "Output file path for the generated video (e.g., ./output/video.mp4)")
	cmd.Flags().BoolP("no-save", "n", false, "Skip saving the video to disk")
	cmd.Flags().IntP("poll-interval", "p", 2, "Polling interval in seconds for checking status")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	modelStr, _ := cmd.Flags().GetString("model")
	if modelStr == "" {
		return fmt.Errorf("model type is required. Use --model flag")
	}

	model := VideoModel(modelStr)

	if !model.IsValid() {
		return fmt.Errorf("unknown model type: %s. Supported models: %s", modelStr, GetSupportedModelsString())
	}

	switch model {
	case VideoModelTalkingAvatar:
		return runGenerateTalkingAvatar(cmd, args)
	case VideoModelMotion:
		return runGenerateAvatarMotion(cmd, args)
	default:
		return fmt.Errorf("unknown model type: %s. Supported models: %s", modelStr, GetSupportedModelsString())
	}
}

func runGenerateTalkingAvatar(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	audioPath, _ := cmd.Flags().GetString("audio")
	if audioPath == "" {
		return fmt.Errorf("audio path is required. Use --audio flag")
	}

	imagePath, _ := cmd.Flags().GetString("image")
	if imagePath == "" {
		return fmt.Errorf("image path is required. Use --image flag")
	}

	outputPath, _ := cmd.Flags().GetString("output")
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
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to generate talking avatar video: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.Data.TaskId
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

			if statusResp.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine) // Clear the spinner line
				fmt.Printf("✅ Generation completed!\n")

				if statusResp.Data.FileUrl != nil {
					if noSave {
						fmt.Printf("🎥 Video generated - URL: %s\n", *statusResp.Data.FileUrl)
						return nil
					}

					// Download the video file from URL
					videoURL := *statusResp.Data.FileUrl
					fmt.Printf("🎥 Downloading video...\n")

					// Determine output path
					if outputPath == "" {
						now := time.Now()
						timestamp := fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/1000000)
						defaultFilename := fmt.Sprintf("video_%s.mp4", timestamp)
						outputPath = filepath.Join(cfg.DefaultSavePath, defaultFilename)
					}

					// Ensure .mp4 extension
					if !strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
						outputPath += ".mp4"
					}

					// Create directory if it doesn't exist
					dir := filepath.Dir(outputPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}

					// Download the video
					resp, err := http.Get(videoURL)
					if err != nil {
						return fmt.Errorf("failed to download video: %w", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("failed to download video: HTTP %d", resp.StatusCode)
					}

					// Create the output file
					outFile, err := os.Create(outputPath)
					if err != nil {
						return fmt.Errorf("failed to create output file: %w", err)
					}
					defer outFile.Close()

					// Copy the response body to the file
					bytesWritten, err := io.Copy(outFile, resp.Body)
					if err != nil {
						return fmt.Errorf("failed to save video: %w", err)
					}

					fmt.Printf("✅ Video saved successfully!\n")
					fmt.Printf("   File: %s\n", outputPath)
					fmt.Printf("   Size: %d bytes\n", bytesWritten)
					if statusResp.Data.OutputDuration != nil {
						fmt.Printf("   Duration: %.2f seconds\n", *statusResp.Data.OutputDuration)
					}
				}

				return nil
			} else if statusResp.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusFAILED || statusResp.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCANCELED || statusResp.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("talking avatar video generation failed with status: %s", statusResp.Data.Status)
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

func runGenerateAvatarMotion(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	audioPath, _ := cmd.Flags().GetString("audio")
	if audioPath == "" {
		return fmt.Errorf("audio path is required. Use --audio flag")
	}

	imagePath, _ := cmd.Flags().GetString("image")
	if imagePath == "" {
		return fmt.Errorf("image path is required. Use --image flag")
	}

	positivePrompt, _ := cmd.Flags().GetString("positive-prompt")
	if positivePrompt == "" {
		return fmt.Errorf("positive prompt is required. Use --positive-prompt flag")
	}

	if len(positivePrompt) > 512 {
		return fmt.Errorf("positive prompt must be 512 characters or less")
	}

	negativePrompt, _ := cmd.Flags().GetString("negative-prompt")

	if len(negativePrompt) > 512 {
		return fmt.Errorf("negative prompt must be 512 characters or less")
	}

	outputPath, _ := cmd.Flags().GetString("output")
	noSave, _ := cmd.Flags().GetBool("no-save")
	pollInterval, _ := cmd.Flags().GetInt("poll-interval")

	audioData, err := os.ReadFile(audioPath)
	if err != nil {
		return fmt.Errorf("failed to read audio file: %w", err)
	}
	audioBase64 := base64.StdEncoding.EncodeToString(audioData)

	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	fmt.Printf("🚀 Starting avatar motion video generation...\n")
	resp, err := client.GenerateAvatarMotion(ctx, audioBase64, imageBase64, positivePrompt, negativePrompt)
	if err != nil {
		if apiErr, ok := errors.IsAPIError(err); ok {
			return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
		}
		return fmt.Errorf("failed to generate avatar motion video: %w", err)
	}

	if resp.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.Data.TaskId
	fmt.Printf("✅ Avatar motion video generation started!\n")
	fmt.Printf("   Task ID: %s\n", taskID)

	fmt.Printf("⏳ Waiting for generation to complete...\n")

	pollTicker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	defer pollTicker.Stop()
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	currentStatus := "PROCESSING"
	clearLine := "\r\033[K"

	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine)
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-pollTicker.C:
			statusResp, err := client.GetAvatarMotionStatus(ctx, taskID)
			if err != nil {
				fmt.Print(clearLine)
				if apiErr, ok := errors.IsAPIError(err); ok {
					return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
				}
				return fmt.Errorf("failed to check status: %w", err)
			}

			if statusResp.Data == nil {
				fmt.Print(clearLine)
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(statusResp.Data.Status)

			if statusResp.Data.Status == api.GenerateAvatarMotionTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine)
				fmt.Printf("✅ Generation completed!\n")

				if statusResp.Data.FileUrl != nil {
					if noSave {
						fmt.Printf("🎥 Video generated - URL: %s\n", *statusResp.Data.FileUrl)
						return nil
					}

					videoURL := *statusResp.Data.FileUrl
					fmt.Printf("🎥 Downloading video...\n")

					if outputPath == "" {
						now := time.Now()
						timestamp := fmt.Sprintf("%s_%03d", now.Format("20060102_150405"), now.Nanosecond()/1000000)
						defaultFilename := fmt.Sprintf("video_%s.mp4", timestamp)
						outputPath = filepath.Join(cfg.DefaultSavePath, defaultFilename)
					}

					if !strings.HasSuffix(strings.ToLower(outputPath), ".mp4") {
						outputPath += ".mp4"
					}

					dir := filepath.Dir(outputPath)
					if err := os.MkdirAll(dir, 0755); err != nil {
						return fmt.Errorf("failed to create directory: %w", err)
					}

					resp, err := http.Get(videoURL)
					if err != nil {
						return fmt.Errorf("failed to download video: %w", err)
					}
					defer resp.Body.Close()

					if resp.StatusCode != http.StatusOK {
						return fmt.Errorf("failed to download video: HTTP %d", resp.StatusCode)
					}

					outFile, err := os.Create(outputPath)
					if err != nil {
						return fmt.Errorf("failed to create output file: %w", err)
					}
					defer outFile.Close()

					bytesWritten, err := io.Copy(outFile, resp.Body)
					if err != nil {
						return fmt.Errorf("failed to save video: %w", err)
					}

					fmt.Printf("✅ Video saved successfully!\n")
					fmt.Printf("   File: %s\n", outputPath)
					fmt.Printf("   Size: %d bytes\n", bytesWritten)
				}

				return nil
			} else if statusResp.Data.Status == api.GenerateAvatarMotionTaskOutputStatusFAILED || statusResp.Data.Status == api.GenerateAvatarMotionTaskOutputStatusCANCELED || statusResp.Data.Status == api.GenerateAvatarMotionTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine)
				return fmt.Errorf("avatar motion video generation failed with status: %s", statusResp.Data.Status)
			}
		case <-spinnerTicker.C:
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Status: %s", frame, currentStatus)
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

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}

	taskID := args[0]

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.GetTalkingAvatarStatus(ctx, taskID)
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

	if resp.Data.Status == api.GenerateTalkingAvatarTaskOutputStatusCOMPLETED {
		if resp.Data.FileUrl != nil {
			videoURL := *resp.Data.FileUrl
			fmt.Printf("✅ Talking avatar video generated successfully!\n")
			fmt.Printf("   Video URL: %s\n", videoURL)
			if resp.Data.OutputDuration != nil {
				fmt.Printf("   Duration: %.2f seconds\n", *resp.Data.OutputDuration)
			}

			// Ask user if they want to download the video
			reader := bufio.NewReader(os.Stdin)
			fmt.Print("\nWould you like to download the generated video? (Y/n): ")
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))

			if response == "" || response == "y" || response == "yes" {
				// Generate default filename
				defaultFilename := fmt.Sprintf("video_%s.mp4", taskID)
				defaultPath := filepath.Join(cfg.DefaultSavePath, defaultFilename)

				// Ask for save location
				fmt.Printf("Enter save path [%s]: ", defaultPath)
				savePath, _ := reader.ReadString('\n')
				savePath = strings.TrimSpace(savePath)

				if savePath == "" {
					savePath = defaultPath
				}

				// Ensure .mp4 extension
				if !strings.HasSuffix(strings.ToLower(savePath), ".mp4") {
					savePath += ".mp4"
				}

				// Create directory if it doesn't exist
				dir := filepath.Dir(savePath)
				if err := os.MkdirAll(dir, 0755); err != nil {
					return fmt.Errorf("failed to create directory: %w", err)
				}

				// Download the video
				fmt.Printf("🎥 Downloading video...\n")
				httpResp, err := http.Get(videoURL)
				if err != nil {
					return fmt.Errorf("failed to download video: %w", err)
				}
				defer httpResp.Body.Close()

				if httpResp.StatusCode != http.StatusOK {
					return fmt.Errorf("failed to download video: HTTP %d", httpResp.StatusCode)
				}

				// Create the output file
				outFile, err := os.Create(savePath)
				if err != nil {
					return fmt.Errorf("failed to create output file: %w", err)
				}
				defer outFile.Close()

				// Copy the response body to the file
				bytesWritten, err := io.Copy(outFile, httpResp.Body)
				if err != nil {
					return fmt.Errorf("failed to save video: %w", err)
				}

				fmt.Printf("✅ Video saved successfully!\n")
				fmt.Printf("   File: %s\n", savePath)
				fmt.Printf("   Size: %d bytes\n", bytesWritten)
			} else {
				fmt.Println("Video not downloaded.")
			}
		}
	}

	return nil
}
