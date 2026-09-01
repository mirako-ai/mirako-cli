package util

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/mirako-ai/mirako-go/api"
)

var upscaleSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// UpscaleOutput is the downloadable projection of a completed upscale task.
type UpscaleOutput struct {
	URL           string
	ReadableUntil *time.Time
}

// ParseUpscaleOutput converts the SDK's untyped result projection into the
// fields needed by the CLI.
func ParseUpscaleOutput(task api.TaskView) (*UpscaleOutput, error) {
	if task.Result == nil {
		return nil, fmt.Errorf("upscale task %s completed without a result", task.TaskId)
	}

	result := *task.Result
	projection := result
	if nested, ok := result["output"].(map[string]interface{}); ok {
		projection = nested
	}

	url, _ := projection["file_url"].(string)
	if url == "" {
		url, _ = projection["url"].(string)
	}
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, fmt.Errorf("upscale task %s result does not contain a download URL", task.TaskId)
	}

	output := &UpscaleOutput{URL: url}
	if value, ok := projection["readable_until"].(string); ok && value != "" {
		readableUntil, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return nil, fmt.Errorf("upscale task %s returned an invalid readable_until value: %w", task.TaskId, err)
		}
		output.ReadableUntil = &readableUntil
	}
	return output, nil
}

// UpscaleTaskState reports whether an upscale task has reached a terminal
// state and whether that terminal state is successful.
func UpscaleTaskState(task api.TaskView) (terminal bool, successful bool, err error) {
	if task.Error != nil {
		return true, false, fmt.Errorf("upscale failed (%s): %s", task.Error.Code, task.Error.Message)
	}

	status := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(task.Status), "-", "_"))
	switch status {
	case "FAILED", "ERROR", "CANCELED", "CANCELLED", "TIMED_OUT":
		return true, false, fmt.Errorf("upscale failed with status: %s", task.Status)
	}

	if task.CompletedAt != nil || status == "COMPLETED" || status == "SUCCEEDED" || status == "SUCCESS" {
		if task.Result == nil {
			return true, false, fmt.Errorf("upscale task %s completed without a result", task.TaskId)
		}
		return true, true, nil
	}
	return false, false, nil
}

// WaitForUpscaleTask polls until the task completes, fails, or the context is
// cancelled. The initial submission response is inspected before polling.
func WaitForUpscaleTask(
	ctx context.Context,
	initial api.TaskView,
	pollInterval time.Duration,
	fetch func(context.Context, string) (*api.TaskView, error),
	output io.Writer,
) (*api.TaskView, error) {
	if pollInterval <= 0 {
		return nil, fmt.Errorf("poll interval must be greater than zero")
	}
	if output == nil {
		output = io.Discard
	}

	current := initial
	if terminal, successful, err := UpscaleTaskState(current); terminal {
		if !successful {
			return nil, err
		}
		return &current, nil
	}

	pollTicker := time.NewTicker(pollInterval)
	spinnerTicker := time.NewTicker(100 * time.Millisecond)
	defer pollTicker.Stop()
	defer spinnerTicker.Stop()

	spinnerIndex := 0
	clearLine := "\r\033[K"
	for {
		select {
		case <-ctx.Done():
			fmt.Fprint(output, clearLine)
			return nil, fmt.Errorf("operation cancelled: %w", ctx.Err())
		case <-pollTicker.C:
			task, err := fetch(ctx, current.TaskId)
			if err != nil {
				fmt.Fprint(output, clearLine)
				return nil, fmt.Errorf("failed to check upscale status: %w", err)
			}
			if task == nil {
				fmt.Fprint(output, clearLine)
				return nil, fmt.Errorf("unexpected response from server")
			}
			current = *task
			if terminal, successful, err := UpscaleTaskState(current); terminal {
				fmt.Fprint(output, clearLine)
				if !successful {
					return nil, err
				}
				return &current, nil
			}
		case <-spinnerTicker.C:
			frame := upscaleSpinnerFrames[spinnerIndex%len(upscaleSpinnerFrames)]
			fmt.Fprintf(output, "\r\033[K%s Status: %s | Stage: %s", frame, current.Status, current.Stage)
			spinnerIndex++
		}
	}
}

// PrintUpscaleTask writes a concise task summary.
func PrintUpscaleTask(output io.Writer, task api.TaskView) {
	fmt.Fprintf(output, "Task ID: %s\n", task.TaskId)
	fmt.Fprintf(output, "Status: %s\n", task.Status)
	fmt.Fprintf(output, "Stage: %s\n", task.Stage)
	if task.StatusUrl != nil {
		fmt.Fprintf(output, "Status URL: %s\n", *task.StatusUrl)
	}
	if task.Error != nil {
		fmt.Fprintf(output, "Error: %s: %s\n", task.Error.Code, task.Error.Message)
	}
}
