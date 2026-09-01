package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-go/api"
)

func TestUpscaleImageStreamsMultipartRequest(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "source image.png")
	inputData := []byte("png contents")
	if err := os.WriteFile(inputPath, inputData, 0644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/image/upscale" {
			t.Fatalf("request = %s %s, want POST /v1/image/upscale", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("outscale"); got != "2" {
			t.Fatalf("outscale = %q, want 2", got)
		}
		assertUpscaleAuthorization(t, r)
		assertUpscaleMultipartFile(t, r, filepath.Base(inputPath), inputData)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		fmt.Fprint(w, taskEnvelopeJSON("image-task", "completed", "complete", true))
	}))
	defer server.Close()

	apiClient := newUpscaleTestClient(t, server.URL)
	task, err := apiClient.UpscaleImage(context.Background(), inputPath, api.N2)
	if err != nil {
		t.Fatalf("UpscaleImage() error = %v", err)
	}
	if task.TaskId != "image-task" || task.Status != "completed" {
		t.Fatalf("task = %+v", task)
	}
}

func TestUpscaleVideoStreamsMultipartRequest(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "source.mp4")
	inputData := []byte("mp4 contents")
	if err := os.WriteFile(inputPath, inputData, 0644); err != nil {
		t.Fatal(err)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/video/upscale" {
			t.Fatalf("request = %s %s, want POST /v1/video/upscale", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("resolution"); got != "4k" {
			t.Fatalf("resolution = %q, want 4k", got)
		}
		assertUpscaleAuthorization(t, r)
		assertUpscaleMultipartFile(t, r, filepath.Base(inputPath), inputData)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		fmt.Fprint(w, taskEnvelopeJSON("video-task", "queued", "queued", false))
	}))
	defer server.Close()

	apiClient := newUpscaleTestClient(t, server.URL)
	task, err := apiClient.UpscaleVideo(context.Background(), inputPath, api.N4k)
	if err != nil {
		t.Fatalf("UpscaleVideo() error = %v", err)
	}
	if task.TaskId != "video-task" || task.Status != "queued" {
		t.Fatalf("task = %+v", task)
	}
}

func TestGetUpscaleTasks(t *testing.T) {
	tests := []struct {
		name string
		path string
		get  func(*Client) (*api.TaskView, error)
	}{
		{
			name: "image",
			path: "/v1/image/upscale/task-1",
			get: func(c *Client) (*api.TaskView, error) {
				return c.GetImageUpscaleTask(context.Background(), "task-1")
			},
		},
		{
			name: "video",
			path: "/v1/video/upscale/task-1",
			get: func(c *Client) (*api.TaskView, error) {
				return c.GetVideoUpscaleTask(context.Background(), "task-1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.URL.Path != tt.path {
					t.Fatalf("request = %s %s, want GET %s", r.Method, r.URL.Path, tt.path)
				}
				assertUpscaleAuthorization(t, r)
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, taskEnvelopeJSON("task-1", "completed", "complete", true))
			}))
			defer server.Close()

			task, err := tt.get(newUpscaleTestClient(t, server.URL))
			if err != nil {
				t.Fatalf("get task error = %v", err)
			}
			if task.TaskId != "task-1" || task.Result == nil {
				t.Fatalf("task = %+v", task)
			}
		})
	}
}

func TestGetUpscaleTaskRejectsMalformedSuccessResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{}`)
	}))
	defer server.Close()

	_, err := newUpscaleTestClient(t, server.URL).GetImageUpscaleTask(context.Background(), "task-1")
	if err == nil || !strings.Contains(err.Error(), "missing request_id") {
		t.Fatalf("error = %v, want malformed response error", err)
	}
}

func TestValidateUpscaleTaskEnvelope(t *testing.T) {
	valid := upscaleTaskEnvelope{
		RequestID: "request-1",
		Data: api.TaskView{
			TaskId:    "task-1",
			TaskType:  "image_upscale",
			Status:    "processing",
			Stage:     "processing",
			CreatedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	if err := validateUpscaleTaskEnvelope(valid); err != nil {
		t.Fatalf("valid envelope rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*upscaleTaskEnvelope)
	}{
		{name: "request ID", mutate: func(v *upscaleTaskEnvelope) { v.RequestID = "" }},
		{name: "task ID", mutate: func(v *upscaleTaskEnvelope) { v.Data.TaskId = "" }},
		{name: "task type", mutate: func(v *upscaleTaskEnvelope) { v.Data.TaskType = "" }},
		{name: "status", mutate: func(v *upscaleTaskEnvelope) { v.Data.Status = "" }},
		{name: "stage", mutate: func(v *upscaleTaskEnvelope) { v.Data.Stage = "" }},
		{name: "created at", mutate: func(v *upscaleTaskEnvelope) { v.Data.CreatedAt = time.Time{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envelope := valid
			tt.mutate(&envelope)
			if err := validateUpscaleTaskEnvelope(envelope); err == nil {
				t.Fatal("expected malformed response error")
			}
		})
	}
}

func TestUpscaleReturnsAPIError(t *testing.T) {
	inputPath := filepath.Join(t.TempDir(), "source.png")
	if err := os.WriteFile(inputPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprint(w, `{"status":422,"detail":"invalid image"}`)
	}))
	defer server.Close()

	_, err := newUpscaleTestClient(t, server.URL).UpscaleImage(context.Background(), inputPath, api.N4)
	if err == nil {
		t.Fatal("expected API error")
	}
}

func newUpscaleTestClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	apiClient, err := New(&config.Config{
		APIToken:        "test-token",
		APIURL:          serverURL,
		DefaultSavePath: t.TempDir(),
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return apiClient
}

func assertUpscaleAuthorization(t *testing.T, r *http.Request) {
	t.Helper()
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Fatalf("Authorization = %q", got)
	}
}

func assertUpscaleMultipartFile(t *testing.T, r *http.Request, wantFilename string, wantData []byte) {
	t.Helper()
	if r.ContentLength != -1 {
		t.Fatalf("ContentLength = %d, want streamed request", r.ContentLength)
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		t.Fatalf("ParseMultipartForm() error = %v", err)
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		t.Fatalf("FormFile(file) error = %v", err)
	}
	defer file.Close()
	if header.Filename != wantFilename {
		t.Fatalf("filename = %q, want %q", header.Filename, wantFilename)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(wantData) {
		t.Fatalf("file data = %q, want %q", data, wantData)
	}
	if _, ok := r.MultipartForm.Value["webhook_url"]; ok {
		t.Fatal("webhook_url must not be sent")
	}
	if _, ok := r.MultipartForm.Value["webhook_token"]; ok {
		t.Fatal("webhook_token must not be sent")
	}
}

func taskEnvelopeJSON(taskID, status, stage string, completed bool) string {
	data := map[string]any{
		"task_id":    taskID,
		"task_type":  "upscale",
		"status":     status,
		"stage":      stage,
		"created_at": "2026-09-01T00:00:00Z",
	}
	if completed {
		data["completed_at"] = "2026-09-01T00:01:00Z"
		data["result"] = map[string]any{
			"url":            "https://media.example.test/result",
			"readable_until": "2026-09-02T00:01:00Z",
		}
	}
	envelope, _ := json.Marshal(map[string]any{
		"data":       data,
		"request_id": "request-1",
	})
	return string(envelope)
}
