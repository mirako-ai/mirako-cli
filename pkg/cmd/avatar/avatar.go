package avatar

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/client"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/spf13/cobra"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func NewAvatarCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "avatar",
		Short: "Manage avatars",
		Long:  `Create, list, and manage AI avatars`,
	}

	cmd.AddCommand(newListCmd())
	cmd.AddCommand(newViewCmd())
	cmd.AddCommand(newGenerateCmd())
	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newStatusCmd())

	return cmd
}

func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all avatars",
		Long:  `List all avatars for the current user`,
		RunE:  runList,
	}

	cmd.Flags().BoolP("json", "j", false, "Output in JSON format")

	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	resp, err := client.ListAvatars(ctx)
	if err != nil {
		return fmt.Errorf("failed to list avatars: %w", err)
	}

	useJSON, _ := cmd.Flags().GetBool("json")
	if useJSON {
		data, _ := json.MarshalIndent(resp, "", "  ")
		fmt.Println(string(data))
		return nil
	}

	if resp.Data == nil || len(*resp.Data) == 0 {
		fmt.Println("No avatars found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tNAME\tSTATUS\tCREATED")

	for _, avatar := range *resp.Data {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			avatar.Id,
			avatar.Name,
			avatar.Status,
			avatar.CreatedAt.Format("2006-01-02 15:04"),
		)
	}

	w.Flush()
	return nil
}

func newViewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "view [avatar-id]",
		Short: "View avatar details",
		Long:  `View detailed information about a specific avatar`,
		Args:  cobra.ExactArgs(1),
		RunE:  runView,
	}
}

func runView(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	avatarID := args[0]
	resp, err := client.GetAvatar(ctx, avatarID)
	if err != nil {
		return fmt.Errorf("failed to get avatar: %w", err)
	}

	fmt.Printf("ID: %s\n", resp.Data.Id)
	fmt.Printf("Name: %s\n", resp.Data.Name)
	fmt.Printf("Status: %s\n", resp.Data.Status)
	fmt.Printf("Created: %s\n", resp.Data.CreatedAt.Format("2006-01-02 15:04"))
	fmt.Printf("User ID: %s\n", resp.Data.UserId)

	if resp.Data.Themes != nil && len(*resp.Data.Themes) > 0 {
		fmt.Println("\nThemes:")
		for _, theme := range *resp.Data.Themes {
			fmt.Printf("  - %s:\n", theme.Name)
			if theme.KeyImage != nil {
				fmt.Printf("    Key Image: %s\n", *theme.KeyImage)
			}
			if theme.LiveVideo != nil {
				fmt.Printf("    Live Video: %s\n", *theme.LiveVideo)
			}
		}
	}

	return nil
}

func newGenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate a new avatar",
		Long:  `Generate a new avatar using AI and save it to disk`,
		RunE:  runGenerate,
	}

	cmd.Flags().StringP("prompt", "p", "", "Prompt for avatar generation")
	cmd.Flags().Int64P("seed", "s", 0, "Seed for reproducible generation (optional)")
	cmd.Flags().StringP("output", "o", "", "Output file path for the generated avatar (e.g., ./output/avatar.jpg)")
	cmd.Flags().BoolP("no-save", "n", false, "Skip saving the image to disk")
	cmd.Flags().IntP("poll-interval", "i", 2, "Polling interval in seconds for checking status")

	return cmd
}

func runGenerate(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	prompt, _ := cmd.Flags().GetString("prompt")
	if prompt == "" {
		return fmt.Errorf("prompt is required. Use --prompt flag")
	}

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
	fmt.Printf("🚀 Starting avatar generation...\n")
	resp, err := client.GenerateAvatar(ctx, prompt, seedPtr)
	if err != nil {
		return fmt.Errorf("failed to generate avatar: %w", err)
	}

	if resp.JSON200 == nil {
		return fmt.Errorf("unexpected response from server")
	}

	taskID := resp.JSON200.Data.TaskId
	fmt.Printf("✅ Avatar generation started!\n")
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
			statusResp, err := client.GetAvatarStatus(ctx, taskID)
			if err != nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("failed to check status: %w", err)
			}

			if statusResp.JSON200 == nil || statusResp.JSON200.Data == nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(statusResp.JSON200.Data.Status)

			if statusResp.JSON200.Data.Status == api.GenerateAvatarTaskOutputStatusCOMPLETED {
				fmt.Print(clearLine) // Clear the spinner line
				fmt.Printf("✅ Generation completed!\n")

				if statusResp.JSON200.Data.Image != nil {
					if noSave {
						fmt.Printf("📸 Image generated (%d bytes) - skipping save due to --no-save flag\n", len(*statusResp.JSON200.Data.Image))
						return nil
					}

					// Save the image
					imageData := *statusResp.JSON200.Data.Image

					// Determine output path
					if outputPath == "" {
						defaultFilename := fmt.Sprintf("avatar_%s.jpg", time.Now().Format("20060102_150405"))
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
			} else if statusResp.JSON200.Data.Status == api.GenerateAvatarTaskOutputStatusFAILED || statusResp.JSON200.Data.Status == api.GenerateAvatarTaskOutputStatusCANCELED || statusResp.JSON200.Data.Status == api.GenerateAvatarTaskOutputStatusTIMEDOUT {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("avatar generation failed with status: %s", statusResp.JSON200.Data.Status)
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
		Short: "Check avatar generation status",
		Long:  `Check the status of an avatar generation task`,
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

	resp, err := client.GetAvatarStatus(ctx, taskID)
	if err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	fmt.Printf("Task ID: %s\n", resp.JSON200.Data.TaskId)

	if resp.JSON200.Data.Image != nil {
		fmt.Printf("✅ Avatar generated successfully!\n")
		fmt.Printf("   Image: %d bytes\n", len(*resp.JSON200.Data.Image))

		// Ask user if they want to save the image
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("\nWould you like to save the generated image? (Y/n): ")
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response == "" || response == "y" || response == "yes" {
			// Generate default filename
			defaultFilename := fmt.Sprintf("avatar_%s.jpg", taskID)
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
			imageData := *resp.JSON200.Data.Image
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

func newBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Build a new avatar from image",
		Long:  `Build a new avatar from a base image, which then you can generate images and create interactive with it.`,
		RunE:  runBuild,
	}

	cmd.Flags().StringP("name", "n", "", "Name for the new avatar")
	cmd.Flags().StringP("image", "i", "", "Path to the base image file")
	cmd.Flags().IntP("poll-interval", "p", 10, "Polling interval in seconds for checking status")

	return cmd
}

func runBuild(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	name, _ := cmd.Flags().GetString("name")
	if name == "" {
		return fmt.Errorf("name is required. Use --name flag")
	}

	imagePath, _ := cmd.Flags().GetString("image")
	if imagePath == "" {
		return fmt.Errorf("image path is required. Use --image flag")
	}

	pollInterval, _ := cmd.Flags().GetInt("poll-interval")

	// Read and encode the image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	// Encode as base64
	encodedImage := base64.StdEncoding.EncodeToString(imageData)

	client, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Start build
	fmt.Printf("🚀 Starting avatar build...\n")
	resp, err := client.BuildAvatar(ctx, name, encodedImage)
	if err != nil {
		return fmt.Errorf("failed to build avatar: %w", err)
	}

	if resp.JSON200 == nil || resp.JSON200.Data == nil {
		return fmt.Errorf("unexpected response from server")
	}

	avatarID := resp.JSON200.Data.AvatarId
	fmt.Printf("✅ Avatar build started!\n")
	fmt.Printf("   Avatar ID: %s\n", avatarID)

	// Give user the option to background the build
	fmt.Printf("\n⏳ Avatar build in progress...\n")
	fmt.Printf("Avatar ID: %s\n", avatarID)

	// Check if user wants to background the build
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nWould you like to background this build and check status later? (y/N): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))

	if response == "y" || response == "yes" {
		fmt.Printf("\n✅ Build backgrounded!\n")
		fmt.Printf("   Avatar ID: %s\n", avatarID)
		fmt.Printf("\n💡 Tip: You can check build status with:\n")
		fmt.Printf("   mirako avatar view %s\n", avatarID)
		fmt.Printf("\n💡 Or view all your avatars with:\n")
		fmt.Printf("   mirako avatar list\n")
		return nil
	}

	// Continue with polling if user chooses to wait
	fmt.Printf("\n⏳ Waiting for build to complete...\n")

	// Use separate tickers for polling and spinner animation
	pollTicker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	spinnerTicker := time.NewTicker(100 * time.Millisecond) // Smooth spinner animation
	defer pollTicker.Stop()
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	currentStatus := "PENDING" // Initial status for avatar build
	clearLine := "\r\033[K"    // ANSI escape codes to clear the line

	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearLine) // Clear the spinner line
			return fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-pollTicker.C:
			avatarResp, err := client.GetAvatar(ctx, avatarID)
			if err != nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("failed to check avatar status: %w", err)
			}

			if avatarResp == nil {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("unexpected response from server")
			}

			currentStatus = string(avatarResp.Data.Status)

			if avatarResp.Data.Status == api.READY {
				fmt.Print(clearLine) // Clear the spinner line
				fmt.Printf("✅ Avatar build completed!\n")
				fmt.Printf("   Avatar ID: %s\n", avatarID)
				fmt.Printf("\n💡 Tip: You can view all your avatars with:\n")
				fmt.Printf("   mirako avatar list\n")
				return nil
			} else if avatarResp.Data.Status == api.ERROR {
				fmt.Print(clearLine) // Clear the spinner line
				return fmt.Errorf("avatar build failed with status: %s", avatarResp.Data.Status)
			}
			// Continue polling for other statuses (PENDING, BUILDING)
		case <-spinnerTicker.C:
			// Update spinner animation smoothly
			frame := spinnerFrames[spinnerIndex%len(spinnerFrames)]
			fmt.Printf("\r\033[K%s Status: %s", frame, currentStatus)
			spinnerIndex++
		}
	}
}
