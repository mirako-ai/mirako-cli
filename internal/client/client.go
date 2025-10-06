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

	"github.com/mirako-ai/mirako-cli/internal/config"
	"github.com/mirako-ai/mirako-cli/internal/errors"
	"github.com/mirako-ai/mirako-go/api"
	sdkclient "github.com/mirako-ai/mirako-go/client"
)

type Client struct {
	sdkClient *sdkclient.Client
	config    *config.Config
}

func New(cfg *config.Config) (*Client, error) {
	if !cfg.IsAuthenticated() {
		return nil, fmt.Errorf("API token is required. Run 'mirako auth login' to authenticate")
	}

	sdkClient, err := sdkclient.NewClient(
		sdkclient.WithAPIKey(cfg.APIToken),
		sdkclient.WithBaseURL(cfg.APIURL),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create API client: %w", err)
	}

	return &Client{
		sdkClient: sdkClient,
		config:    cfg,
	}, nil
}

func handleHTTPResponse(resp *http.Response, context string) error {
	if resp == nil {
		return errors.NewAPIError(0, "no response received", context)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	return errors.HandleHTTPError(resp, context)
}

func parseJSONResponse(resp *http.Response, target interface{}) error {
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	if err := json.Unmarshal(bodyBytes, target); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

func (c *Client) ListAvatars(ctx context.Context) (*api.GetUserAvatarListApiResponseBody, error) {
	resp, err := c.sdkClient.GetUserAvatarList(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "list avatars"); err != nil {
		return nil, err
	}

	var result api.GetUserAvatarListApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAvatar(ctx context.Context, id string) (*api.GetAvatarApiResponseBody, error) {
	resp, err := c.sdkClient.GetAvatarById(ctx, id)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get avatar"); err != nil {
		return nil, err
	}

	var result api.GetAvatarApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateAvatar(ctx context.Context, prompt string, seed *int64) (*api.AsyncGenerateAvatarApiResponseBody, error) {
	body := api.GenerateAvatarAsyncJSONRequestBody{
		Prompt: prompt,
		Seed:   seed,
	}
	resp, err := c.sdkClient.GenerateAvatarAsync(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "generate avatar"); err != nil {
		return nil, err
	}

	var result api.AsyncGenerateAvatarApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAvatarStatus(ctx context.Context, taskID string) (*api.GenerateAvatarStatusApiResponseBody, error) {
	resp, err := c.sdkClient.GetAvatarGenerationStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get avatar status"); err != nil {
		return nil, err
	}

	var result api.GenerateAvatarStatusApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteAvatar(ctx context.Context, avatarID string) error {
	resp, err := c.sdkClient.DeleteAvatar(ctx, avatarID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleHTTPResponse(resp, "delete avatar")
}

func (c *Client) BuildAvatar(ctx context.Context, name, image string) (*api.AsyncBuildApiResponseBody, error) {
	body := api.BuildAvatarAsyncJSONRequestBody{
		Name:  name,
		Image: image,
	}
	resp, err := c.sdkClient.BuildAvatarAsync(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "build avatar"); err != nil {
		return nil, err
	}

	var result api.AsyncBuildApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListSessions(ctx context.Context) (*api.ListSessionsApiResponseBody, error) {
	resp, err := c.sdkClient.ListInteractiveSessions(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "list sessions"); err != nil {
		return nil, err
	}

	var result api.ListSessionsApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) StartSession(ctx context.Context, body api.StartInteractiveSessionJSONRequestBody) (*api.StartSessionApiResponseBody, error) {
	resp, err := c.sdkClient.StartInteractiveSession(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "start session"); err != nil {
		return nil, err
	}

	var result api.StartSessionApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) StopSessions(ctx context.Context, sessionIDs []string) (*api.StopSessionsApiResponseBody, error) {
	body := api.StopInteractiveSessionsJSONRequestBody{
		SessionIds: &sessionIDs,
	}
	resp, err := c.sdkClient.StopInteractiveSessions(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "stop sessions"); err != nil {
		return nil, err
	}

	var result api.StopSessionsApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetSessionProfile(ctx context.Context, sessionID string) (*api.GetSessionProfileApiResponseBody, error) {
	resp, err := c.sdkClient.GetSessionProfile(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get session profile"); err != nil {
		return nil, err
	}

	var result api.GetSessionProfileApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateImage(ctx context.Context, prompt string, aspectRatio api.AsyncGenerateImageApiRequestBodyAspectRatio, seed *int64) (*api.AsyncGenerateImageApiResponseBody, error) {
	body := api.GenerateImageAsyncJSONRequestBody{
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Seed:        seed,
	}
	resp, err := c.sdkClient.GenerateImageAsync(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "generate image"); err != nil {
		return nil, err
	}

	var result api.AsyncGenerateImageApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetImageStatus(ctx context.Context, taskID string) (*api.GenerateImageStatusApiResponseBody, error) {
	resp, err := c.sdkClient.GetImageGenerationStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get image status"); err != nil {
		return nil, err
	}

	var result api.GenerateImageStatusApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) SpeechToText(ctx context.Context, audio string) (*api.STTApiResponseBody, error) {
	body := api.ConvertSpeechToTextJSONRequestBody{
		Audio: audio,
	}
	resp, err := c.sdkClient.ConvertSpeechToText(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "speech to text"); err != nil {
		return nil, err
	}

	var result api.STTApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) TextToSpeech(ctx context.Context, text, voiceProfileID, returnType string, chineseLanguage *api.TTSApiRequestBodyChineseLanguage, opts *api.TTSParams) (*api.TTSApiResponseBody, error) {
	body := api.ConvertTextToSpeechJSONRequestBody{
		Text:            text,
		VoiceProfileId:  voiceProfileID,
		ReturnType:      returnType,
		ChineseLanguage: chineseLanguage,
		Opts:            opts,
	}
	resp, err := c.sdkClient.ConvertTextToSpeech(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "text to speech"); err != nil {
		return nil, err
	}

	var result api.TTSApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateTalkingAvatar(ctx context.Context, audio, image string) (*api.AsyncGenerateTalkingAvatarApiResponseBody, error) {
	body := api.GenerateTalkingAvatarAsyncJSONRequestBody{
		Audio: audio,
		Image: image,
	}
	resp, err := c.sdkClient.GenerateTalkingAvatarAsync(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "generate talking avatar video"); err != nil {
		return nil, err
	}

	var result api.AsyncGenerateTalkingAvatarApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GenerateAvatarMotion(ctx context.Context, audio, image, positivePrompt, negativePrompt string) (*api.AsyncGenerateAvatarMotionApiResponseBody, error) {
	body := api.GenerateAvatarMotionAsyncJSONRequestBody{
		Audio:          audio,
		Image:          image,
		PositivePrompt: positivePrompt,
		NegativePrompt: negativePrompt,
	}
	resp, err := c.sdkClient.GenerateAvatarMotionAsync(ctx, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "generate avatar motion video"); err != nil {
		return nil, err
	}

	var result api.AsyncGenerateAvatarMotionApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetAvatarMotionStatus(ctx context.Context, taskID string) (*api.GenerateAvatarMotionStatusApiResponseBody, error) {
	resp, err := c.sdkClient.GetAvatarMotionGenerationStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get avatar motion video status"); err != nil {
		return nil, err
	}

	var result api.GenerateAvatarMotionStatusApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetTalkingAvatarStatus(ctx context.Context, taskID string) (*api.GenerateTalkingAvatarStatusApiResponseBody, error) {
	resp, err := c.sdkClient.GetTalkingAvatarGenerationStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get talking avatar video status"); err != nil {
		return nil, err
	}

	var result api.GenerateTalkingAvatarStatusApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListPremadeProfiles(ctx context.Context) (*api.GetPremadeProfilesApiResponseBody, error) {
	resp, err := c.sdkClient.GetPremadeVoiceProfiles(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "list voice profiles"); err != nil {
		return nil, err
	}

	var result api.GetPremadeProfilesApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) ListVoiceProfiles(ctx context.Context) (*api.GetVoiceProfilesApiResponseBody, error) {
	resp, err := c.sdkClient.GetUserVoiceProfiles(ctx)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "list custom voice profiles"); err != nil {
		return nil, err
	}

	var result api.GetVoiceProfilesApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) GetVoiceProfile(ctx context.Context, profileID string) (*api.GetVoiceProfileApiResponseBody, error) {
	resp, err := c.sdkClient.GetVoiceProfile(ctx, profileID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get voice profile"); err != nil {
		return nil, err
	}

	var result api.GetVoiceProfileApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) DeleteVoiceProfile(ctx context.Context, profileID string) error {
	resp, err := c.sdkClient.DeleteVoiceProfile(ctx, profileID)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return handleHTTPResponse(resp, "delete voice profile")
}

func (c *Client) CloneVoice(ctx context.Context, name string, audioDir string, annotationFile string, cleanData bool, description string) (*api.AsyncFinetuningApiResponseBody, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("name", name); err != nil {
		return nil, fmt.Errorf("failed to write name field: %w", err)
	}

	cleanDataStr := "false"
	if cleanData {
		cleanDataStr = "true"
	}
	if err := writer.WriteField("clean_data", cleanDataStr); err != nil {
		return nil, fmt.Errorf("failed to write clean_data field: %w", err)
	}

	if description != "" {
		if err := writer.WriteField("description", description); err != nil {
			return nil, fmt.Errorf("failed to write description field: %w", err)
		}
	}

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

	if _, err := io.Copy(annotationWriter, annotationFileHandle); err != nil {
		return nil, fmt.Errorf("failed to write annotation data: %w", err)
	}

	audioFiles, err := ScanAudioFiles(audioDir)
	if err != nil {
		return nil, fmt.Errorf("failed to scan audio files: %w", err)
	}

	if len(audioFiles) == 0 {
		return nil, fmt.Errorf("no audio files (.wav or .mp3) found in directory: %s", audioDir)
	}

	fmt.Printf("Uploading %d audio files for voice cloning...\n", len(audioFiles))

	for _, audioFile := range audioFiles {
		file, err := os.Open(audioFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open audio file %s: %w", audioFile, err)
		}

		audioWriter, err := writer.CreatePart(map[string][]string{
			"Content-Disposition": {fmt.Sprintf(`form-data; name="audio_samples"; filename="%s"`, filepath.Base(audioFile))},
			"Content-Type":        {"audio/wav"},
		})
		if err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to create audio form file: %w", err)
		}

		if _, err := io.Copy(audioWriter, file); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write audio data: %w", err)
		}

		file.Close()
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/v1/voice/clone", c.config.APIURL), body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.config.APIToken)

	httpClient := &http.Client{
		Timeout: 1 * time.Hour,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var apiResp api.AsyncFinetuningApiResponseBody
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w (response: %s)", err, string(bodyBytes))
	}

	return &apiResp, nil
}

func (c *Client) GetVoiceCloneStatus(ctx context.Context, taskID string) (*api.FinetuningStatusApiResponseBody, error) {
	resp, err := c.sdkClient.GetVoiceCloningStatus(ctx, taskID)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := handleHTTPResponse(resp, "get voice clone status"); err != nil {
		return nil, err
	}

	var result api.FinetuningStatusApiResponseBody
	if err := parseJSONResponse(resp, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
