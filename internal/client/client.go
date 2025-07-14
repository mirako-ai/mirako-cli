package client

import (
	"context"
	"fmt"
	"net/http"

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

// Avatar methods
func (c *Client) ListAvatars(ctx context.Context) (*api.GetUserAvatarListApiResponseBody, error) {
	resp, err := c.apiClient.GetV1AvatarListWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list avatars")
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

func (c *Client) GetAvatar(ctx context.Context, id string) (*api.GetAvatarApiResponseBody, error) {
	resp, err := c.apiClient.GetV1AvatarIdWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get avatar")
}

func (c *Client) GenerateAvatar(ctx context.Context, prompt string, seed *int64) (*api.PostV1AvatarAsyncGenerateResponse, error) {
	request := api.PostV1AvatarAsyncGenerateJSONRequestBody{
		Prompt: prompt,
		Seed:   seed,
	}
	resp, err := c.apiClient.PostV1AvatarAsyncGenerateWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate avatar")
}

func (c *Client) GetAvatarStatus(ctx context.Context, taskID string) (*api.GetV1AvatarAsyncGenerateTaskIdStatusResponse, error) {
	resp, err := c.apiClient.GetV1AvatarAsyncGenerateTaskIdStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get avatar status")
}

// Interactive methods
func (c *Client) ListSessions(ctx context.Context) (*api.GetV1InteractiveListResponse, error) {
	resp, err := c.apiClient.GetV1InteractiveListWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list sessions")
}

func (c *Client) StartSession(ctx context.Context, body api.PostV1InteractiveStartSessionJSONRequestBody) (*api.PostV1InteractiveStartSessionResponse, error) {
	resp, err := c.apiClient.PostV1InteractiveStartSessionWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "start session")
}

func (c *Client) StopSessions(ctx context.Context, sessionIDs []string) (*api.PostV1InteractiveStopSessionsResponse, error) {
	body := api.PostV1InteractiveStopSessionsJSONRequestBody{
		SessionIds: &sessionIDs,
	}
	resp, err := c.apiClient.PostV1InteractiveStopSessionsWithResponse(ctx, body)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "stop sessions")
}

func (c *Client) GetSessionProfile(ctx context.Context, sessionID string) (*api.GetV1InteractiveSessionIdProfileResponse, error) {
	resp, err := c.apiClient.GetV1InteractiveSessionIdProfileWithResponse(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get session profile")
}

// Voice methods
func (c *Client) ListPremadeProfiles(ctx context.Context) (*api.GetV1VoicePremadeProfilesResponse, error) {
	resp, err := c.apiClient.GetV1VoicePremadeProfilesWithResponse(ctx)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "list voice profiles")
}

// Image methods
func (c *Client) GenerateImage(ctx context.Context, prompt string, aspectRatio api.AsyncGenerateImageApiRequestBodyAspectRatio, seed *int64) (*api.PostV1ImageAsyncGenerateResponse, error) {
	request := api.PostV1ImageAsyncGenerateJSONRequestBody{
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Seed:        seed,
	}
	resp, err := c.apiClient.PostV1ImageAsyncGenerateWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate image")
}

func (c *Client) GetImageStatus(ctx context.Context, taskID string) (*api.GetV1ImageAsyncGenerateTaskIdStatusResponse, error) {
	resp, err := c.apiClient.GetV1ImageAsyncGenerateTaskIdStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get image status")
}

// Video methods
func (c *Client) GenerateTalkingAvatar(ctx context.Context, audio, image string) (*api.PostV1VideoAsyncGenerateTalkingAvatarResponse, error) {
	request := api.PostV1VideoAsyncGenerateTalkingAvatarJSONRequestBody{
		Audio: audio,
		Image: image,
	}
	resp, err := c.apiClient.PostV1VideoAsyncGenerateTalkingAvatarWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "generate talking avatar video")
}

func (c *Client) GetTalkingAvatarStatus(ctx context.Context, taskID string) (*api.GetV1VideoAsyncGenerateTalkingAvatarTaskIdStatusResponse, error) {
	resp, err := c.apiClient.GetV1VideoAsyncGenerateTalkingAvatarTaskIdStatusWithResponse(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "get talking avatar video status")
}

// Avatar build methods
func (c *Client) BuildAvatar(ctx context.Context, name, image string) (*api.PostV1AvatarAsyncBuildResponse, error) {
	request := api.PostV1AvatarAsyncBuildJSONRequestBody{
		Name:  name,
		Image: image,
	}
	resp, err := c.apiClient.PostV1AvatarAsyncBuildWithResponse(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp.JSON200 != nil {
		return resp, nil
	}
	return nil, handleErrorResponse(resp.HTTPResponse, "build avatar")
}