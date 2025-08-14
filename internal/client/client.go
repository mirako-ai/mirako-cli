package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mirako-ai/mirako-cli/internal/api"
	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
)

type Client struct {
	apiClient *api.ClientWithResponses
	config    *config.Config
}

func New(cfg *config.Config) (*Client, error) {
	if !cfg.IsAuthenticated() {
		return nil, fmt.Errorf("API token is required. Run 'mirako auth login' to authenticate")
	}

	// Create HTTP client with authentication
	httpClient := &http.Client{}

	// Create API client with authentication middleware
	apiClient, err := api.NewClientWithResponses(cfg.APIURL,
		api.WithHTTPClient(httpClient),
		func(c *api.Client) error {
			// Add authentication header to all requests
			c.RequestEditors = append(c.RequestEditors, func(ctx context.Context, req *http.Request) error {
				req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
				return nil
			})
			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Client{
		apiClient: apiClient,
		config:    cfg,
	}, nil
}

// handleErrorResponse processes API response errors and returns appropriate error types
func handleErrorResponse(resp *http.Response, context string) error {
	if resp == nil {
		return errors.NewAPIError(0, "no response received", context)
	}

	statusCode := resp.StatusCode
	if statusCode >= 200 && statusCode < 300 {
		return nil
	}

	// Create API error with appropriate details
	return errors.NewAPIError(statusCode, http.StatusText(statusCode), context)
}

// Avatar methods
func (c *Client) ListAvatars(ctx context.Context) (*api.GetUserAvatarListApiResponseBody, error) {
	resp, err := c.apiClient.GetUserAvatarListWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list avatars")
}

func (c *Client) GetAvatar(ctx context.Context, id string) (*api.GetAvatarApiResponseBody, error) {
	resp, err := c.apiClient.GetAvatarByIdWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get avatar")
}

func (c *Client) GenerateAvatar(ctx context.Context, prompt string, seed *int64) (*api.AsyncGenerateAvatarApiResponseBody, error) {
	request := api.AsyncGenerateAvatarApiRequestBody{
		Prompt: prompt,
		Seed:   seed,
	}
	resp, err := c.apiClient.GenerateAvatarAsyncWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate avatar")
}

func (c *Client) GetAvatarStatus(ctx context.Context, taskID string) (*api.GenerateAvatarStatusApiResponseBody, error) {
	resp, err := c.apiClient.GetAvatarGenerationStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get avatar status")
}

func (c *Client) DeleteAvatar(ctx context.Context, avatarID string) (*api.DeleteAvatarResponse, error) {
	resp, err := c.apiClient.DeleteAvatarWithResponse(ctx, avatarID)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) BuildAvatar(ctx context.Context, name, image string) (*api.AsyncBuildApiResponseBody, error) {
	request := api.AsyncBuildApiRequestBody{
		Name:  name,
		Image: image,
	}
	resp, err := c.apiClient.BuildAvatarAsyncWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "build avatar")
}

// Interactive methods
func (c *Client) ListSessions(ctx context.Context) (*api.ListSessionsApiResponseBody, error) {
	resp, err := c.apiClient.ListInteractiveSessionsWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list sessions")
}

func (c *Client) StartSession(ctx context.Context, body api.StartSessionApiRequestBody) (*api.StartSessionApiResponseBody, error) {
	resp, err := c.apiClient.StartInteractiveSessionWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "start session")
}

func (c *Client) StopSessions(ctx context.Context, sessionIDs []string) (*api.StopSessionsApiResponseBody, error) {
	body := api.StopSessionsApiRequestBody{
		SessionIds: &sessionIDs,
	}
	resp, err := c.apiClient.StopInteractiveSessionsWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "stop sessions")
}

func (c *Client) GetSessionProfile(ctx context.Context, sessionID string) (*api.GetSessionProfileApiResponseBody, error) {
	resp, err := c.apiClient.GetSessionProfileWithResponse(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get session profile")
}

// Image methods
func (c *Client) GenerateImage(ctx context.Context, prompt string, aspectRatio api.AsyncGenerateImageApiRequestBodyAspectRatio, seed *int64) (*api.AsyncGenerateImageApiResponseBody, error) {
	request := api.AsyncGenerateImageApiRequestBody{
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Seed:        seed,
	}
	resp, err := c.apiClient.GenerateImageAsyncWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate image")
}

func (c *Client) GetImageStatus(ctx context.Context, taskID string) (*api.GenerateImageStatusApiResponseBody, error) {
	resp, err := c.apiClient.GetImageGenerationStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get image status")
}

// Speech methods
func (c *Client) SpeechToText(ctx context.Context, audio string) (*api.STTApiResponseBody, error) {
	request := api.STTApiRequestBody{
		Audio: audio,
	}
	resp, err := c.apiClient.ConvertSpeechToTextWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "speech to text")
}

func (c *Client) TextToSpeech(ctx context.Context, text, voiceProfileID, returnType string, chineseLanguage *api.TTSApiRequestBodyChineseLanguage, opts *api.TTSParams) (*api.TTSApiResponseBody, error) {
	request := api.TTSApiRequestBody{
		Text:            text,
		VoiceProfileId:  voiceProfileID,
		ReturnType:      returnType,
		ChineseLanguage: chineseLanguage,
		Opts:            opts,
	}
	resp, err := c.apiClient.ConvertTextToSpeechWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "text to speech")
}

// Video methods
func (c *Client) GenerateTalkingAvatar(ctx context.Context, audio, image string) (*api.AsyncGenerateTalkingAvatarApiResponseBody, error) {
	request := api.AsyncGenerateTalkingAvatarApiRequestBody{
		Audio: audio,
		Image: image,
	}
	resp, err := c.apiClient.GenerateTalkingAvatarAsyncWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate talking avatar video")
}

func (c *Client) GetTalkingAvatarStatus(ctx context.Context, taskID string) (*api.GenerateTalkingAvatarStatusApiResponseBody, error) {
	resp, err := c.apiClient.GetTalkingAvatarGenerationStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get talking avatar video status")
}

// Voice methods

func (c *Client) ListPremadeProfiles(ctx context.Context) (*api.GetPremadeProfilesApiResponseBody, error) {
	resp, err := c.apiClient.GetPremadeVoiceProfilesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list voice profiles")
}

func (c *Client) ListVoiceProfiles(ctx context.Context) (*api.GetVoiceProfilesApiResponseBody, error) {
	resp, err := c.apiClient.GetUserVoiceProfilesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list custom voice profiles")
}

func (c *Client) GetVoiceProfile(ctx context.Context, profileID string) (*api.GetVoiceProfileApiResponseBody, error) {
	resp, err := c.apiClient.GetVoiceProfileWithResponse(ctx, profileID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get voice profile")
}

func (c *Client) DeleteVoiceProfile(ctx context.Context, profileID string) (*api.DeleteVoiceProfileResponse, error) {
	resp, err := c.apiClient.DeleteVoiceProfileWithResponse(ctx, profileID)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) CloneVoice(ctx context.Context, name string, audioDir string, annotationFile string, cleanData bool) (*api.AsyncFinetuningApiResponseBody, error) {
	// Create multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add name field
	if err := writer.WriteField("name", name); err != nil {
		return nil, fmt.Errorf("failed to write name field: %w", err)
	}

	// Add clean_data field
	cleanDataStr := "false"
	if cleanData {
		cleanDataStr = "true"
	}
	if err := writer.WriteField("clean_data", cleanDataStr); err != nil {
		return nil, fmt.Errorf("failed to write clean_data field: %w", err)
	}

	// Add annotation file
	annotationFileHandle, err := os.Open(annotationFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open annotation file: %w", err)
	}
	defer annotationFileHandle.Close()

	annotationWriter, err := writer.CreatePart(map[string][]string{
		"Content-Disposition": {fmt.Sprintf(`form-data; name="annotation_list"; filename="%s"`, filepath.Base(annotationFile))},
		"Content-Type":        {"text/plain"},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create annotation form file: %w", err)
	}

	// Stream annotation file content instead of loading into memory
	if _, err := io.Copy(annotationWriter, annotationFileHandle); err != nil {
		return nil, fmt.Errorf("failed to write annotation data: %w", err)
	}

	// Add audio sample files
	audioFiles, err := ScanAudioFiles(audioDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan audio files: %w", err)
	}

	if len(audioFiles) == 0 {
		return nil, fmt.Errorf("no audio files (.wav or .mp3) found in directory: %s", audioDir)
	}

	// Log the number of files being uploaded for debugging
	fmt.Printf("Uploading %d audio files for voice cloning...\n", len(audioFiles))

	for _, audioFile := range audioFiles {
		// Open file for streaming instead of loading into memory
		file, err := os.Open(audioFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open audio file %s: %w", audioFile, err)
		}

		// Create form file with proper content type for audio/wav
		audioWriter, err := writer.CreatePart(map[string][]string{
			"Content-Disposition": {fmt.Sprintf(`form-data; name="audio_samples"; filename="%s"`, filepath.Base(audioFile))},
			"Content-Type":        {"audio/wav"},
		})
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create audio form file: %w", err)
		}

		// Stream file content instead of loading into memory
		if _, err := io.Copy(audioWriter, file); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write audio data: %w", err)
		}

		file.Close()
	}

	writer.Close()

	// Create HTTP request with multipart form data
	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/voice/clone", c.config.APIURL), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)

	// Make HTTP request directly since oapi-codegen doesn't support multipart form data well
	httpClient := &http.Client{
		Timeout: 1 * time.Hour, // Extended timeout for large file uploads
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body once for consistent handling
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if response is successful
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var apiResp api.AsyncFinetuningApiResponseBody
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (response: %s)", err, string(bodyBytes))
	}

	return &apiResp, nil
}

func (c *Client) GetVoiceCloneStatus(ctx context.Context, taskID string) (*api.FinetuningStatusApiResponseBody, error) {
	resp, err := c.apiClient.GetVoiceCloningStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.HTTPResponse.StatusCode >= 200 && resp.HTTPResponse.StatusCode < 300 {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get voice clone status")
}
