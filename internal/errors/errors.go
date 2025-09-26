package errors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/mirako-ai/mirako-cli/internal/api"
)

// APIError represents an API error response
// It wraps the API's ErrorModel and provides additional context

type APIError struct {
	StatusCode int
	ErrorModel *api.ErrorModel
	Message    string
	Context    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.ErrorModel != nil && e.ErrorModel.Detail != nil {
		return fmt.Sprintf("API error (%d): %s", e.StatusCode, *e.ErrorModel.Detail)
	}
	return fmt.Sprintf("API error (%d)", e.StatusCode)
}

// IsInsufficientCredits returns true if the error indicates insufficient credits
func (e *APIError) IsInsufficientCredits() bool {
	return e.StatusCode == http.StatusPaymentRequired
}

// IsAuthenticationError returns true if the error is authentication-related
func (e *APIError) IsAuthenticationError() bool {
	return e.StatusCode == http.StatusUnauthorized || e.StatusCode == http.StatusForbidden
}

// IsRateLimitError returns true if the error indicates rate limiting
func (e *APIError) IsRateLimitError() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// IsNotFound returns true if the error indicates a resource was not found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// GetUserFriendlyMessage returns a user-friendly error message
func (e *APIError) GetUserFriendlyMessage() string {
	if e.IsInsufficientCredits() {
		return "❌ Insufficient credits. Please upgrade your plan or purchase more credits at https://mirako.ai/billing"
	}
	if e.IsAuthenticationError() {
		return "❌ Authentication failed. Please run 'mirako auth login' to authenticate"
	}
	if e.IsRateLimitError() {
		return "❌ Rate limit exceeded. Please wait a moment and try again"
	}
	if e.IsNotFound() {
		return "❌ Resource not found. Please check the ID and try again"
	}
	if e.ErrorModel != nil && e.ErrorModel.Detail != nil {
		return fmt.Sprintf("❌ %s", *e.ErrorModel.Detail)
	}
	return fmt.Sprintf("❌ API request failed with status %d", e.StatusCode)
}

// HandleHTTPError processes an HTTP response and returns an appropriate error
func HandleHTTPError(resp *http.Response, context string) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	// Try to read and parse the response body for detailed error information
	var errorModel *api.ErrorModel
	if resp.Body != nil {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil && len(bodyBytes) > 0 {
			// Try to parse as ErrorModel
			var em api.ErrorModel
			if json.Unmarshal(bodyBytes, &em) == nil {
				errorModel = &em
			}
		}
	}

	// Create API error with parsed error model or fallback to status text
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		ErrorModel: errorModel,
		Context:    context,
	}

	// Set message to status text if no error model detail is available
	if errorModel == nil || errorModel.Detail == nil {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}

	return apiErr
}

// NewAPIError creates a new APIError with custom message
func NewAPIError(statusCode int, message, context string) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Message:    message,
		Context:    context,
	}
}

// IsAPIError checks if an error is an APIError
func IsAPIError(err error) (*APIError, bool) {
	apiErr, ok := err.(*APIError)
	return apiErr, ok
}
