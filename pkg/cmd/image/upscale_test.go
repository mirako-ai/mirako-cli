package image

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

func TestImageUpscaleCommandShape(t *testing.T) {
	imageCmd := NewImageCmd()
	upscaleCmd, _, err := imageCmd.Find([]string{"upscale"})
	if err != nil {
		t.Fatalf("find upscale command: %v", err)
	}
	if upscaleCmd.Name() != "upscale" || upscaleCmd.RunE == nil {
		t.Fatalf("upscale command = %+v", upscaleCmd)
	}
	for _, flag := range []string{"image", "outscale", "output", "no-save"} {
		if upscaleCmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s flag", flag)
		}
	}
	for _, flag := range []string{"webhook-url", "webhook-token"} {
		if upscaleCmd.Flags().Lookup(flag) != nil {
			t.Errorf("unexpected --%s flag", flag)
		}
	}

	statusCmd, _, err := imageCmd.Find([]string{"upscale", "status"})
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

func TestRunImageUpscaleValidation(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "image.png")
	if err := os.WriteFile(inputPath, []byte("image"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		flags map[string]string
		want  string
	}{
		{name: "missing image", flags: map[string]string{}, want: "image path is required"},
		{name: "invalid outscale", flags: map[string]string{"image": inputPath, "outscale": "3"}, want: "outscale must be 2 or 4"},
		{name: "conflicting output", flags: map[string]string{"image": inputPath, "output": "result.png", "no-save": "true"}, want: "--output cannot be used with --no-save"},
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

func TestHandleUpscaledImageDownloadsPNG(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("png result"))
	}))
	defer server.Close()

	result := map[string]any{"url": server.URL}
	completedAt := time.Now()
	task := api.TaskView{
		TaskId:      "task-1",
		TaskType:    "image_upscale",
		Status:      "completed",
		Stage:       "complete",
		CreatedAt:   time.Now(),
		CompletedAt: &completedAt,
		Result:      &result,
	}
	requested := filepath.Join(t.TempDir(), "upscaled")
	if err := handleUpscaledImage(context.Background(), task, requested, ".", false, false); err != nil {
		t.Fatalf("handleUpscaledImage() error = %v", err)
	}
	data, err := os.ReadFile(requested + ".png")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "png result" {
		t.Fatalf("output = %q", data)
	}
}
