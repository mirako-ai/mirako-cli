package video

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

const maxUpscaleVideoBytes int64 = 1 << 30

func newUpscaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upscale",
		Short: "Upscale a video",
		Long:  `Submit an MP4 video for asynchronous resolution upscaling`,
		Args:  cobra.NoArgs,
		RunE:  runUpscale,
	}

	cmd.Flags().String("video", "", "Path to the MP4 video")
	cmd.Flags().StringP("resolution", "r", "", "Target resolution (1080p, 2k, or 4k)")
	cmd.Flags().StringP("output", "o", "", "Output MP4 path")
	cmd.Flags().BoolP("no-save", "n", false, "Print the result URL without saving the video")
	cmd.Flags().IntP("poll-interval", "p", 2, "Polling interval in seconds for checking status")
	cmd.Flags().Bool("no-wait", false, "Return after submission without waiting for completion")
	cmd.AddCommand(newUpscaleStatusCmd())
	return cmd
}

func runUpscale(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	videoPath, _ := cmd.Flags().GetString("video")
	if videoPath == "" {
		return fmt.Errorf("video path is required. Use --video flag")
	}
	if err := util.ValidateMediaFile(videoPath, maxUpscaleVideoBytes, ".mp4"); err != nil {
		return err
	}

	resolutionValue, _ := cmd.Flags().GetString("resolution")
	resolution, err := parseUpscaleResolution(resolutionValue)
	if err != nil {
		return err
	}
	outputPath, _ := cmd.Flags().GetString("output")
	noSave, _ := cmd.Flags().GetBool("no-save")
	noWait, _ := cmd.Flags().GetBool("no-wait")
	pollInterval, _ := cmd.Flags().GetInt("poll-interval")
	if pollInterval <= 0 {
		return fmt.Errorf("poll interval must be greater than zero")
	}
	if noSave && outputPath != "" {
		return fmt.Errorf("--output cannot be used with --no-save")
	}
	if noWait && outputPath != "" {
		return fmt.Errorf("--output cannot be used with --no-wait")
	}

	cfg, err := util.GetConfig(cmd)
	if err != nil {
		return err
	}
	apiClient, err := client.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	fmt.Printf("🚀 Starting video upscale to %s...\n", resolutionValue)
	task, err := apiClient.UpscaleVideo(ctx, videoPath, resolution)
	if err != nil {
		return upscaleVideoAPIError(err, "failed to upscale video")
	}
	fmt.Println("✅ Video upscale submitted!")
	util.PrintUpscaleTask(os.Stdout, *task)
	if noWait {
		return nil
	}

	fmt.Println("⏳ Waiting for video upscale to complete...")
	completedTask, err := util.WaitForUpscaleTask(
		ctx,
		*task,
		time.Duration(pollInterval)*time.Second,
		func(ctx context.Context, taskID string) (*api.TaskView, error) {
			return apiClient.GetVideoUpscaleTask(ctx, taskID)
		},
		os.Stdout,
	)
	if err != nil {
		return upscaleVideoAPIError(err, "video upscale failed")
	}

	fmt.Println("✅ Video upscale completed!")
	return handleUpscaledVideo(ctx, *completedTask, outputPath, cfg.DefaultSavePath, noSave, false)
}

func parseUpscaleResolution(value string) (api.UpscaleVideoParamsResolution, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1080p":
		return api.N1080p, nil
	case "2k":
		return api.N2k, nil
	case "4k":
		return api.N4k, nil
	case "":
		return "", fmt.Errorf("resolution is required. Use --resolution flag (1080p, 2k, or 4k)")
	default:
		return "", fmt.Errorf("resolution must be one of 1080p, 2k, or 4k")
	}
}

func newUpscaleStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [task-id]",
		Short: "Check a video upscale task",
		Args:  cobra.ExactArgs(1),
		RunE:  runUpscaleStatus,
	}
	cmd.Flags().StringP("output", "o", "", "Output MP4 path for a completed task")
	cmd.Flags().BoolP("no-save", "n", false, "Print the result URL without saving the video")
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

	task, err := apiClient.GetVideoUpscaleTask(ctx, args[0])
	if err != nil {
		return upscaleVideoAPIError(err, "failed to get video upscale status")
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
		return handleUpscaledVideo(ctx, *task, outputPath, cfg.DefaultSavePath, noSave, true)
	}

	defaultPath := util.ResolveTaskOutputPath("", cfg.DefaultSavePath, "video_upscaled", task.TaskId, ".mp4")
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nWould you like to save the upscaled video? (Y/n): ")
	response, _ := reader.ReadString('\n')
	response = strings.TrimSpace(strings.ToLower(response))
	if response != "" && response != "y" && response != "yes" {
		fmt.Println("Video not saved.")
		return nil
	}

	fmt.Printf("Enter save path [%s]: ", defaultPath)
	savePath, _ := reader.ReadString('\n')
	savePath = strings.TrimSpace(savePath)
	if savePath == "" {
		savePath = defaultPath
	}
	return handleUpscaledVideo(ctx, *task, savePath, cfg.DefaultSavePath, false, true)
}

func handleUpscaledVideo(ctx context.Context, task api.TaskView, outputPath, defaultSavePath string, noSave, taskFilename bool) error {
	output, err := util.ParseUpscaleOutput(task)
	if err != nil {
		return err
	}
	if noSave {
		printUpscaleVideoURL(output)
		return nil
	}

	if taskFilename {
		outputPath = util.ResolveTaskOutputPath(outputPath, defaultSavePath, "video_upscaled", task.TaskId, ".mp4")
	} else {
		outputPath = util.ResolveMediaOutputPath(outputPath, defaultSavePath, "video_upscaled", ".mp4")
	}
	fmt.Println("🎥 Downloading upscaled video...")
	bytesWritten, err := util.DownloadMedia(ctx, output.URL, outputPath)
	if err != nil {
		return err
	}
	fmt.Println("✅ Video saved successfully!")
	fmt.Printf("   File: %s\n", outputPath)
	fmt.Printf("   Size: %d bytes\n", bytesWritten)
	return nil
}

func printUpscaleVideoURL(output *util.UpscaleOutput) {
	fmt.Printf("🎥 Upscaled video URL: %s\n", output.URL)
	if output.ReadableUntil != nil {
		fmt.Printf("   Readable until: %s\n", output.ReadableUntil.Format(time.RFC3339))
	}
}

func upscaleVideoAPIError(err error, context string) error {
	if apiErr, ok := internalerrors.IsAPIError(err); ok {
		return fmt.Errorf("%s", apiErr.GetUserFriendlyMessage())
	}
	return fmt.Errorf("%s: %w", context, err)
}
