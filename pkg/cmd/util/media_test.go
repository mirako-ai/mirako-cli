package util

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateMediaFile(t *testing.T) {
	dir := t.TempDir()
	validPath := filepath.Join(dir, "image.PNG")
	if err := os.WriteFile(validPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ValidateMediaFile(validPath, 4, ".jpg", ".png"); err != nil {
		t.Fatalf("valid file rejected: %v", err)
	}

	tests := []struct {
		name string
		path string
		max  int64
		want string
	}{
		{name: "missing", path: filepath.Join(dir, "missing.png"), max: 10, want: "failed to access"},
		{name: "directory", path: dir, max: 10, want: "regular file"},
		{name: "too large", path: validPath, max: 3, want: "too large"},
		{name: "extension", path: validPath, max: 10, want: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			allowed := []string{".jpg"}
			if tt.name != "extension" {
				allowed = []string{".png"}
			}
			err := ValidateMediaFile(tt.path, tt.max, allowed...)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestResolveOutputPaths(t *testing.T) {
	if got := ResolveMediaOutputPath("result", ".", "image_upscaled", ".png"); got != "result.png" {
		t.Fatalf("ResolveMediaOutputPath() = %q", got)
	}
	if got := ResolveMediaOutputPath("RESULT.PNG", ".", "image_upscaled", ".png"); got != "RESULT.PNG" {
		t.Fatalf("ResolveMediaOutputPath() = %q", got)
	}
	got := ResolveTaskOutputPath("", "/tmp/output", "video_upscaled", "task/1", ".mp4")
	want := filepath.Join("/tmp/output", "video_upscaled_task_1.mp4")
	if got != want {
		t.Fatalf("ResolveTaskOutputPath() = %q, want %q", got, want)
	}
}

func TestDownloadMedia(t *testing.T) {
	payload := []byte("downloaded media")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != nil {
			t.Fatalf("request context unexpectedly cancelled: %v", r.Context().Err())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "nested", "result.mp4")
	bytesWritten, err := DownloadMedia(context.Background(), server.URL, outputPath)
	if err != nil {
		t.Fatalf("DownloadMedia() error = %v", err)
	}
	if bytesWritten != int64(len(payload)) {
		t.Fatalf("bytesWritten = %d", bytesWritten)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("output = %q", got)
	}
	if matches, _ := filepath.Glob(filepath.Join(filepath.Dir(outputPath), ".mirako-download-*.part")); len(matches) != 0 {
		t.Fatalf("temporary files left behind: %v", matches)
	}
}

func TestReplaceMediaFileOverwritesExistingRegularFile(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "temporary.part")
	outputPath := filepath.Join(dir, "result.mp4")
	if err := os.WriteFile(tempPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := replaceMediaFile(tempPath, outputPath); err != nil {
		t.Fatalf("replaceMediaFile() error = %v", err)
	}
	got, err := os.ReadFile(outputPath)
	if err != nil || string(got) != "new" {
		t.Fatalf("output = %q, %v", got, err)
	}
}

func TestReplaceMediaFilePreservesNonRegularDestinationOnFailure(t *testing.T) {
	dir := t.TempDir()
	tempPath := filepath.Join(dir, "temporary.part")
	outputPath := filepath.Join(dir, "existing-directory")
	if err := os.WriteFile(tempPath, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outputPath, 0755); err != nil {
		t.Fatal(err)
	}
	if err := replaceMediaFile(tempPath, outputPath); err == nil {
		t.Fatal("expected replacement error")
	}
	info, err := os.Stat(outputPath)
	if err != nil || !info.IsDir() {
		t.Fatalf("existing destination was not preserved: %v, %v", info, err)
	}
}

func TestDownloadMediaDoesNotReplaceOutputOnHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "failure", http.StatusBadGateway)
	}))
	defer server.Close()

	outputPath := filepath.Join(t.TempDir(), "result.png")
	if err := os.WriteFile(outputPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := DownloadMedia(context.Background(), server.URL, outputPath)
	if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("HTTP %d", http.StatusBadGateway)) {
		t.Fatalf("error = %v", err)
	}
	got, readErr := os.ReadFile(outputPath)
	if readErr != nil || string(got) != "existing" {
		t.Fatalf("existing output changed: %q, %v", got, readErr)
	}
}
