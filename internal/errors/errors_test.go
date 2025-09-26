package errors

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mirako-ai/mirako-cli/internal/api"
)

func TestAPIError_GetUserFriendlyMessage(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		expected string
	}{
		{
			name: "insufficient credits",
			apiErr: &APIError{
				StatusCode: http.StatusPaymentRequired,
			},
			expected: "❌ Insufficient credits. Please upgrade your plan or purchase more credits at https://mirako.ai/billing",
		},
		{
			name: "authentication error",
			apiErr: &APIError{
				StatusCode: http.StatusUnauthorized,
			},
			expected: "❌ Authentication failed. Please run 'mirako auth login' to authenticate",
		},
		{
			name: "rate limit error",
			apiErr: &APIError{
				StatusCode: http.StatusTooManyRequests,
			},
			expected: "❌ Rate limit exceeded. Please wait a moment and try again",
		},
		{
			name: "not found error",
			apiErr: &APIError{
				StatusCode: http.StatusNotFound,
			},
			expected: "❌ Resource not found. Please check the ID and try again",
		},
		{
			name: "error with detail",
			apiErr: &APIError{
				StatusCode: http.StatusBadRequest,
				ErrorModel: &api.ErrorModel{
					Detail: stringPtr("Invalid prompt provided"),
				},
			},
			expected: "❌ Invalid prompt provided",
		},
		{
			name: "generic error",
			apiErr: &APIError{
				StatusCode: http.StatusInternalServerError,
				Message:    "Internal Server Error",
			},
			expected: "❌ API request failed with status 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.apiErr.GetUserFriendlyMessage()
			if result != tt.expected {
				t.Errorf("GetUserFriendlyMessage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHandleHTTPError(t *testing.T) {
	// Test with a response that has an error body
	errorResponse := `{"detail": "The prompt is too long. Maximum length is 1000 characters."}`
	resp := httptest.NewRecorder()
	resp.WriteHeader(http.StatusBadRequest)
	resp.WriteString(errorResponse)

	err := HandleHTTPError(resp.Result(), "test context")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
	}

	if apiErr.ErrorModel == nil {
		t.Fatal("Expected ErrorModel to be populated")
	}

	if apiErr.ErrorModel.Detail == nil {
		t.Fatal("Expected Detail to be populated")
	}

	expectedDetail := "The prompt is too long. Maximum length is 1000 characters."
	if *apiErr.ErrorModel.Detail != expectedDetail {
		t.Errorf("Expected detail %q, got %q", expectedDetail, *apiErr.ErrorModel.Detail)
	}

	userMessage := apiErr.GetUserFriendlyMessage()
	expectedUserMessage := "❌ The prompt is too long. Maximum length is 1000 characters."
	if userMessage != expectedUserMessage {
		t.Errorf("Expected user message %q, got %q", expectedUserMessage, userMessage)
	}
}

func TestHandleHTTPError_NoBody(t *testing.T) {
	// Test with a response that has no body
	resp := httptest.NewRecorder()
	resp.WriteHeader(http.StatusInternalServerError)

	err := HandleHTTPError(resp.Result(), "test context")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status code %d, got %d", http.StatusInternalServerError, apiErr.StatusCode)
	}

	if apiErr.ErrorModel != nil {
		t.Error("Expected ErrorModel to be nil for response without body")
	}

	userMessage := apiErr.GetUserFriendlyMessage()
	expectedUserMessage := "❌ API request failed with status 500"
	if userMessage != expectedUserMessage {
		t.Errorf("Expected user message %q, got %q", expectedUserMessage, userMessage)
	}
}

func TestHandleHTTPError_InvalidJSON(t *testing.T) {
	// Test with invalid JSON in response body
	resp := httptest.NewRecorder()
	resp.WriteHeader(http.StatusBadRequest)
	resp.WriteString("not json")

	err := HandleHTTPError(resp.Result(), "test context")

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("Expected APIError, got %T", err)
	}

	if apiErr.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected status code %d, got %d", http.StatusBadRequest, apiErr.StatusCode)
	}

	if apiErr.ErrorModel != nil {
		t.Error("Expected ErrorModel to be nil for invalid JSON")
	}

	userMessage := apiErr.GetUserFriendlyMessage()
	expectedUserMessage := "❌ API request failed with status 400"
	if userMessage != expectedUserMessage {
		t.Errorf("Expected user message %q, got %q", expectedUserMessage, userMessage)
	}
}

func stringPtr(s string) *string {
	return &s
}
