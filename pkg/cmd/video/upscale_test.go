package video

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirako-ai/mirako-go/api"
)

func TestVideoUpscaleCommandShape(t *testing.T) {
	videoCmd := NewVideoCmd()
	upscaleCmd, _, err := videoCmd.Find([]string{"upscale"})
	if err != nil {
		t.Fatalf("find upscale command: %v", err)
	}
	if upscaleCmd.Name() != "upscale" || upscaleCmd.RunE == nil {
		t.Fatalf("upscale command = %+v", upscaleCmd)
	}
	for _, flag := range []string{"video", "resolution", "output", "no-save", "poll-interval", "no-wait"} {
		if upscaleCmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s flag", flag)
		}
	}
	for _, flag := range []string{"webhook-url", "webhook-token"} {
		if upscaleCmd.Flags().Lookup(flag) != nil {
			t.Errorf("unexpected --%s flag", flag)
		}
	}

	statusCmd, _, err := videoCmd.Find([]string{"upscale", "status"})
	if err != nil {
		t.Fatalf("find upscale status command: %v", err)
	}
	if statusCmd.Use != "status [task-id]" {
		t.Fatalf("Use = %q", statusCmd.Use)
	}
	if err := statusCmd.Args(statusCmd, []string{"task-1"}); err != nil {
		t.Fatalf("valid status args rejected: %v", err)
	}
	if err := statusCmd.Args(statusCmd, nil); err == nil {
		t.Fatal("missing task ID should fail")
	}
}

func TestParseUpscaleResolution(t *testing.T) {
	tests := []struct {
		value string
		want  api.UpscaleVideoParamsResolution
		err   bool
	}{
		{value: "1080p", want: api.N1080p},
		{value: "2K", want: api.N2k},
		{value: " 4k ", want: api.N4k},
		{value: "", err: true},
		{value: "8k", err: true},
	}
	for _, tt := range tests {
		got, err := parseUpscaleResolution(tt.value)
		if (err != nil) != tt.err || got != tt.want {
			t.Fatalf("parseUpscaleResolution(%q) = %q, %v", tt.value, got, err)
		}
	}
}

func TestRunVideoUpscaleValidation(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "video.mp4")
	if err := os.WriteFile(inputPath, []byte("video"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		flags map[string]string
		want  string
	}{
		{name: "missing video", flags: map[string]string{}, want: "video path is required"},
		{name: "missing resolution", flags: map[string]string{"video": inputPath}, want: "resolution is required"},
		{name: "invalid resolution", flags: map[string]string{"video": inputPath, "resolution": "8k"}, want: "resolution must be one of"},
		{name: "invalid poll interval", flags: map[string]string{"video": inputPath, "resolution": "4k", "poll-interval": "0"}, want: "poll interval must be greater than zero"},
		{name: "no save output conflict", flags: map[string]string{"video": inputPath, "resolution": "4k", "no-save": "true", "output": "result.mp4"}, want: "--output cannot be used with --no-save"},
		{name: "no wait output conflict", flags: map[string]string{"video": inputPath, "resolution": "4k", "no-wait": "true", "output": "result.mp4"}, want: "--output cannot be used with --no-wait"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newUpscaleCmd()
			for name, value := range tt.flags {
				if err := cmd.Flags().Set(name, value); err != nil {
					t.Fatal(err)
				}
			}
			err := runUpscale(cmd, nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestHandleUpscaledVideoDownloadsMP4(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("mp4 result"))
	}))
	defer server.Close()

	result := map[string]any{"url": server.URL}
	completedAt := time.Now()
	task := api.TaskView{
		TaskId:      "task-1",
		TaskType:    "video_upscale",
		Status:      "completed",
		Stage:       "complete",
		CreatedAt:   time.Now(),
		CompletedAt: &completedAt,
		Result:      &result,
	}
	requested := filepath.Join(t.TempDir(), "upscaled")
	if err := handleUpscaledVideo(context.Background(), task, requested, ".", false, false); err != nil {
		t.Fatalf("handleUpscaledVideo() error = %v", err)
	}
	data, err := os.ReadFile(requested + ".mp4")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "mp4 result" {
		t.Fatalf("output = %q", data)
	}
}
