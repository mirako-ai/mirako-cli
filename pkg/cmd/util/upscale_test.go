package util

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/mirako-ai/mirako-go/api"
)

func TestParseUpscaleOutput(t *testing.T) {
	result := map[string]any{
		"output": map[string]any{
			"file_url":       "https://media.example.test/result.png",
			"readable_until": "2026-09-02T00:01:00Z",
		},
	}
	task := upscaleTestTask("completed", &result)

	output, err := ParseUpscaleOutput(task)
	if err != nil {
		t.Fatalf("ParseUpscaleOutput() error = %v", err)
	}
	if output.URL != "https://media.example.test/result.png" {
		t.Fatalf("URL = %q", output.URL)
	}
	if output.ReadableUntil == nil || output.ReadableUntil.Format(time.RFC3339) != "2026-09-02T00:01:00Z" {
		t.Fatalf("ReadableUntil = %v", output.ReadableUntil)
	}
}

func TestParseUpscaleOutputSupportsMirakoFileURL(t *testing.T) {
	result := map[string]any{"file_url": "https://media.example.test/result.mp4"}
	output, err := ParseUpscaleOutput(upscaleTestTask("completed", &result))
	if err != nil {
		t.Fatalf("ParseUpscaleOutput() error = %v", err)
	}
	if output.URL != "https://media.example.test/result.mp4" {
		t.Fatalf("URL = %q", output.URL)
	}
}

func TestParseUpscaleOutputRejectsMalformedResult(t *testing.T) {
	tests := []api.TaskView{
		upscaleTestTask("completed", nil),
		upscaleTestTask("completed", &map[string]any{"readable_until": "2026-09-02T00:01:00Z"}),
		upscaleTestTask("completed", &map[string]any{"url": "https://media.example.test/result", "readable_until": "bad"}),
	}
	for _, task := range tests {
		if _, err := ParseUpscaleOutput(task); err == nil {
			t.Fatalf("ParseUpscaleOutput(%+v) expected error", task.Result)
		}
	}
}

func TestUpscaleTaskState(t *testing.T) {
	completedAt := time.Now()
	result := map[string]any{"url": "https://example.test/result"}
	tests := []struct {
		name       string
		task       api.TaskView
		terminal   bool
		successful bool
		wantErr    bool
	}{
		{name: "pending", task: upscaleTestTask("processing", nil)},
		{name: "completed by timestamp", task: func() api.TaskView {
			task := upscaleTestTask("processing", &result)
			task.CompletedAt = &completedAt
			return task
		}(), terminal: true, successful: true},
		{name: "completed by status", task: upscaleTestTask("COMPLETED", &result), terminal: true, successful: true},
		{name: "failed status", task: upscaleTestTask("failed", nil), terminal: true, wantErr: true},
		{name: "safe error", task: func() api.TaskView {
			task := upscaleTestTask("processing", nil)
			task.Error = &api.SafeError{Code: "upscale_failed", Message: "backend failure"}
			return task
		}(), terminal: true, wantErr: true},
		{name: "completed missing result", task: upscaleTestTask("completed", nil), terminal: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			terminal, successful, err := UpscaleTaskState(tt.task)
			if terminal != tt.terminal || successful != tt.successful || (err != nil) != tt.wantErr {
				t.Fatalf("state = terminal %t successful %t err %v", terminal, successful, err)
			}
		})
	}
}

func TestWaitForUpscaleTaskPollsUntilComplete(t *testing.T) {
	initial := upscaleTestTask("queued", nil)
	result := map[string]any{"url": "https://example.test/result"}
	completed := upscaleTestTask("completed", &result)
	calls := 0
	var output bytes.Buffer

	task, err := WaitForUpscaleTask(
		context.Background(),
		initial,
		time.Millisecond,
		func(context.Context, string) (*api.TaskView, error) {
			calls++
			if calls == 1 {
				processing := upscaleTestTask("processing", nil)
				return &processing, nil
			}
			return &completed, nil
		},
		&output,
	)
	if err != nil {
		t.Fatalf("WaitForUpscaleTask() error = %v", err)
	}
	if task.Status != "completed" || calls != 2 {
		t.Fatalf("task = %+v, calls = %d", task, calls)
	}
	if !strings.Contains(output.String(), "\033[K") {
		t.Fatalf("output should clear spinner line: %q", output.String())
	}
}

func TestWaitForUpscaleTaskPropagatesFetchError(t *testing.T) {
	initial := upscaleTestTask("queued", nil)
	wantErr := errors.New("network failure")
	_, err := WaitForUpscaleTask(
		context.Background(),
		initial,
		time.Millisecond,
		func(context.Context, string) (*api.TaskView, error) { return nil, wantErr },
		io.Discard,
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped network failure", err)
	}
}

func TestWaitForUpscaleTaskHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := WaitForUpscaleTask(
		ctx,
		upscaleTestTask("queued", nil),
		time.Second,
		func(context.Context, string) (*api.TaskView, error) {
			t.Fatal("fetch must not be called")
			return nil, nil
		},
		nil,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

func upscaleTestTask(status string, result *map[string]any) api.TaskView {
	return api.TaskView{
		TaskId:    "task-1",
		TaskType:  "upscale",
		Status:    status,
		Stage:     "processing",
		CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		Result:    result,
	}
}
