package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/mirako-ai/mirako-cli/api"
	"github.com/mirako-ai/mirako-cli/internal/config"
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
	return resp.JSON200, nil
}

func (c *Client) GetAvatar(ctx context.Context, id string) (*api.GetAvatarApiResponseBody, error) {
	resp, err := c.apiClient.GetV1AvatarIdWithResponse(ctx, id)
	if err != nil {
		return nil, err
	}
	return resp.JSON200, nil
}

func (c *Client) GenerateAvatar(ctx context.Context, prompt string, seed *int64) (*api.PostV1AvatarAsyncGenerateResponse, error) {
	request := api.PostV1AvatarAsyncGenerateJSONRequestBody{
		Prompt: prompt,
		Seed:   seed,
	}
	return c.apiClient.PostV1AvatarAsyncGenerateWithResponse(ctx, request)
}

func (c *Client) GetAvatarStatus(ctx context.Context, taskID string) (*api.GetV1AvatarAsyncGenerateTaskIdStatusResponse, error) {
	return c.apiClient.GetV1AvatarAsyncGenerateTaskIdStatusWithResponse(ctx, taskID)
}

// Interactive methods
func (c *Client) ListSessions(ctx context.Context) (*api.GetV1InteractiveListResponse, error) {
	return c.apiClient.GetV1InteractiveListWithResponse(ctx)
}

func (c *Client) StartSession(ctx context.Context, body api.PostV1InteractiveStartSessionJSONRequestBody) (*api.PostV1InteractiveStartSessionResponse, error) {
	return c.apiClient.PostV1InteractiveStartSessionWithResponse(ctx, body)
}

func (c *Client) StopSessions(ctx context.Context, sessionIDs []string) (*api.PostV1InteractiveStopSessionsResponse, error) {
	body := api.PostV1InteractiveStopSessionsJSONRequestBody{
		SessionIds: &sessionIDs,
	}
	return c.apiClient.PostV1InteractiveStopSessionsWithResponse(ctx, body)
}

func (c *Client) GetSessionProfile(ctx context.Context, sessionID string) (*api.GetV1InteractiveSessionIdProfileResponse, error) {
	return c.apiClient.GetV1InteractiveSessionIdProfileWithResponse(ctx, sessionID)
}

// Voice methods
func (c *Client) ListPremadeProfiles(ctx context.Context) (*api.GetV1VoicePremadeProfilesResponse, error) {
	return c.apiClient.GetV1VoicePremadeProfilesWithResponse(ctx)
}

// Image methods
func (c *Client) GenerateImage(ctx context.Context, prompt string, aspectRatio api.AsyncGenerateImageApiRequestBodyAspectRatio, seed *int64) (*api.PostV1ImageAsyncGenerateResponse, error) {
	request := api.PostV1ImageAsyncGenerateJSONRequestBody{
		Prompt:      prompt,
		AspectRatio: aspectRatio,
		Seed:        seed,
	}
	return c.apiClient.PostV1ImageAsyncGenerateWithResponse(ctx, request)
}

func (c *Client) GetImageStatus(ctx context.Context, taskID string) (*api.GetV1ImageAsyncGenerateTaskIdStatusResponse, error) {
	return c.apiClient.GetV1ImageAsyncGenerateTaskIdStatusWithResponse(ctx, taskID)
}