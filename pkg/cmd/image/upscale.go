package image

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/client"
	internalerrors "github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-cli/pkg/cmd/util"
	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
)

const maxUpscaleImageBytes int64 = 32 << 20

func newUpscaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upscale",
		Short: "Upscale an image",
		Long:  `Upscale a JPEG, PNG, or static WebP image and save the PNG result`,
		Args:  cobra.NoArgs,
		RunE:  runUpscale,
	}

	cmd.Flags().StringP("image", "i", "", "Path to the JPEG, PNG, or static WebP image")
	cmd.Flags().Int("outscale", 4, "Upscale factor (2 or 4)")
	cmd.Flags().StringP("output", "o", "", "Output PNG path")
	cmd.Flags().BoolP("no-save", "n", false, "Print the result URL without saving the image")
	cmd.AddCommand(newUpscaleStatusCmd())
	return cmd
}

func runUpscale(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	imagePath, _ := cmd.Flags().GetString("image")
	if imagePath == "" {
		return fmt.Errorf("image path is required. Use --image flag")
	}
	if err := util.ValidateMediaFile(imagePath, maxUpscaleImageBytes, ".jpg", ".jpeg", ".png", ".webp"); err != nil {
		return err
	}

	outscaleValue, _ := cmd.Flags().GetInt("outscale")
	var outscale api.UpscaleImageParamsOutscale
	switch outscaleValue {
	case 2:
		outscale = api.N2
	case 4:
		outscale = api.N4
	default:
		return fmt.Errorf("outscale must be 2 or 4")
	}

	outputPath, _ := cmd.Flags().GetString("output")
	noSave, _ := cmd.Flags().GetBool("no-save")
	if noSave && outputPath != "" {
		return fmt.Errorf("--output cannot be used with --no-save")
	}

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}
	apiClient, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	fmt.Printf("🚀 Upscaling image %dx...\n", outscaleValue)
	task, err := apiClient.UpscaleImage(ctx, imagePath, outscale)
	if err != nil {
		return upscaleImageAPIError(err, "failed to upscale image")
	}
	util.PrintUpscaleTask(os.Stdout, *task)

	terminal, successful, stateErr := util.UpscaleTaskState(*task)
	if !terminal {
		return fmt.Errorf("image upscale task %s is not complete; check it with `mirako image upscale status %s`", task.TaskId, task.TaskId)
	}
	if !successful {
		return stateErr
	}

	fmt.Println("✅ Image upscale completed!")
	return handleUpscaledImage(ctx, *task, outputPath, cfg.DefaultSavePath, noSave, false)
}

func newUpscaleStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [task-id]",
		Short: "Check an image upscale task",
		Args:  cobra.ExactArgs(1),
		RunE:  runUpscaleStatus,
	}
	cmd.Flags().StringP("output", "o", "", "Output PNG path for a completed task")
	cmd.Flags().BoolP("no-save", "n", false, "Print the result URL without saving the image")
	return cmd
}

func runUpscaleStatus(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	outputPath, _ := cmd.Flags().GetString("output")
	noSave, _ := cmd.Flags().GetBool("no-save")
	if noSave && outputPath != "" {
		return fmt.Errorf("--output cannot be used with --no-save")
	}

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}
	apiClient, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	task, err := apiClient.GetImageUpscaleTask(ctx, args[0])
	if err != nil {
		return upscaleImageAPIError(err, "failed to get image upscale status")
	}
	util.PrintUpscaleTask(os.Stdout, *task)

	terminal, successful, stateErr := util.UpscaleTaskState(*task)
	if !terminal {
		return nil
	}
	if !successful {
		return stateErr
	}

	if noSave || outputPath != "" {
		return handleUpscaledImage(ctx, *task, outputPath, cfg.DefaultSavePath, noSave, true)
	}

	defaultPath := util.ResolveTaskOutputPath("", cfg.DefaultSavePath, "image_upscaled", task.TaskId, ".png")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nWould you like to save the upscaled image? (Y/n): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		fmt.Println("Image not saved.")
		return nil
	}

	fmt.Printf("Enter save path [%s]: ", defaultPath)
	savePath, _ := reader.ReadString('\n')
	savePath = strings.TrimSpace(savePath)
	if savePath == "" {
		savePath = defaultPath
	}
	return handleUpscaledImage(ctx, *task, savePath, cfg.DefaultSavePath, false, true)
}

func handleUpscaledImage(ctx context.Context, task api.TaskView, outputPath, defaultSavePath string, noSave, taskFilename bool) error {
	output, err := util.ParseUpscaleOutput(task)
	if err != nil {
		return err
	}
	if noSave {
		printUpscaleImageURL(output)
		return nil
	}

	if taskFilename {
		outputPath = util.ResolveTaskOutputPath(outputPath, defaultSavePath, "image_upscaled", task.TaskId, ".png")
	} else {
		outputPath = util.ResolveMediaOutputPath(outputPath, defaultSavePath, "image_upscaled", ".png")
	}
	fmt.Println("📸 Downloading upscaled image...")
	bytesWritten, err := util.DownloadMedia(ctx, output.URL, outputPath)
	if err != nil {
		return err
	}
	fmt.Printf("✅ Image saved to: %s\n", outputPath)
	fmt.Printf("   Size: %d bytes\n", bytesWritten)
	return nil
}

func printUpscaleImageURL(output *util.UpscaleOutput) {
	fmt.Printf("📸 Upscaled image URL: %s\n", output.URL)
	if output.ReadableUntil != nil {
		fmt.Printf("   Readable until: %s\n", output.ReadableUntil.Format(time.RFC3339))
	}
}

func upscaleImageAPIError(err error, context string) error {
	if apiErr, ok := internalerrors.IsAPIError(err); ok {
		return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
	}
	return fmt.Errorf("%s: %w", context, err)
}
