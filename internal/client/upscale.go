package client

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/mirako-ai/mirako-go/api"
)

// UpscaleImage streams an image file to the image upscaler.
func (c *Client) UpscaleImage(ctx context.Context, filePath string, outscale api.UpscaleImageParamsOutscale) (*api.TaskView, error) {
	body, contentType, err := newMultipartFileBody(filePath)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	params := &api.UpscaleImageParams{Outscale: &outscale}
	resp, err := c.uploadSDKClient.UpscaleImageWithBody(ctx, params, contentType, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "upscale image"); err != nil {
		return nil, err
	}
	return parseUpscaleTaskResponse(resp)
}

// GetImageUpscaleTask returns the current image upscale task state.
func (c *Client) GetImageUpscaleTask(ctx context.Context, taskID string) (*api.TaskView, error) {
	resp, err := c.sdkClient.GetImageUpscaleTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get image upscale task"); err != nil {
		return nil, err
	}
	return parseUpscaleTaskResponse(resp)
}

// UpscaleVideo streams a video file to the video upscaler.
func (c *Client) UpscaleVideo(ctx context.Context, filePath string, resolution api.UpscaleVideoParamsResolution) (*api.TaskView, error) {
	body, contentType, err := newMultipartFileBody(filePath)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	params := &api.UpscaleVideoParams{Resolution: resolution}
	resp, err := c.uploadSDKClient.UpscaleVideoWithBody(ctx, params, contentType, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "upscale video"); err != nil {
		return nil, err
	}
	return parseUpscaleTaskResponse(resp)
}

// GetVideoUpscaleTask returns the current video upscale task state.
func (c *Client) GetVideoUpscaleTask(ctx context.Context, taskID string) (*api.TaskView, error) {
	resp, err := c.sdkClient.GetVideoUpscaleTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get video upscale task"); err != nil {
		return nil, err
	}
	return parseUpscaleTaskResponse(resp)
}

type upscaleTaskEnvelope struct {
	Data      api.TaskView `json:"data"`
	RequestID string       `json:"request_id"`
}

func parseUpscaleTaskResponse(resp *http.Response) (*api.TaskView, error) {
	var envelope upscaleTaskEnvelope
	if err := parseJSONResponse(resp, &envelope); err != nil {
		return nil, err
	}
	if err := validateUpscaleTaskEnvelope(envelope); err != nil {
		return nil, err
	}
	return &envelope.Data, nil
}

func validateUpscaleTaskEnvelope(envelope upscaleTaskEnvelope) error {
	if envelope.RequestID == "" {
		return fmt.Errorf("unexpected response from server: missing request_id")
	}
	task := envelope.Data
	switch {
	case task.TaskId == "":
		return fmt.Errorf("unexpected response from server: missing task_id")
	case task.TaskType == "":
		return fmt.Errorf("unexpected response from server: missing task_type")
	case task.Status == "":
		return fmt.Errorf("unexpected response from server: missing task status")
	case task.Stage == "":
		return fmt.Errorf("unexpected response from server: missing task stage")
	case task.CreatedAt.IsZero():
		return fmt.Errorf("unexpected response from server: missing task created_at")
	default:
		return nil
	}
}

func newMultipartFileBody(filePath string) (io.ReadCloser, string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open input file: %w", err)
	}

	reader, writer := io.Pipe()
	multipartWriter := multipart.NewWriter(writer)
	contentType := multipartWriter.FormDataContentType()

	go func() {
		defer file.Close()

		part, err := multipartWriter.CreateFormFile("file", filepath.Base(filePath))
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if closeErr := multipartWriter.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.CloseWithError(err)
			return
		}
		_ = writer.Close()
	}()

	return reader, contentType, nil
}
