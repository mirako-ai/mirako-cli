package agent

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

	"github.com/mirako-ai/mirako-go/api"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const testAgentJSON = `{
  "id": "agent-1",
  "user_id": "user-1",
  "name": "Agent One",
  "description": "Helpful agent",
  "avatar_id": "avatar-1",
  "voice_profile_id": "voice-1",
  "model": "metis-2.5",
  "llm_model": "gemini-2.0-flash",
  "instruction": "Be helpful",
  "tools": [{"type":"function","name":"search"}],
  "runtime_kind": "managed_agent",
  "has_custom_agent_bearer_token": false,
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:01:00Z"
}`

func TestRunCreateValidation(t *testing.T) {
	tests := []struct {
		name          string
		flags         map[string]string
		errorContains string
	}{
		{
			name:          "missing name",
			flags:         map[string]string{},
			errorContains: "name is required",
		},
		{
			name: "missing avatar",
			flags: map[string]string{
				"name": "agent",
			},
			errorContains: "avatar ID is required",
		},
		{
			name: "missing voice",
			flags: map[string]string{
				"name":   "agent",
				"avatar": "avatar-1",
			},
			errorContains: "voice profile ID is required",
		},
		{
			name: "missing llm model",
			flags: map[string]string{
				"name":        "agent",
				"avatar":      "avatar-1",
				"voice":       "voice-1",
				"instruction": "Be helpful",
			},
			errorContains: "LLM model is required",
		},
		{
			name: "missing instruction",
			flags: map[string]string{
				"name":      "agent",
				"avatar":    "avatar-1",
				"voice":     "voice-1",
				"llm-model": "gemini-2.0-flash",
			},
			errorContains: "instruction is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCreateCmd()
			for name, value := range tt.flags {
				if err := cmd.Flags().Set(name, value); err != nil {
					t.Fatalf("failed to set flag %s: %v", name, err)
				}
			}

			err := runCreate(cmd, nil)
			if err == nil {
				t.Fatal("expected error but got nil")
			}
			if !strings.Contains(err.Error(), tt.errorContains) {
				t.Fatalf("expected error to contain %q, got %q", tt.errorContains, err.Error())
			}
		})
	}
}

func TestResolveInstruction(t *testing.T) {
	tmpDir := t.TempDir()
	instructionFile := filepath.Join(tmpDir, "instruction.md")
	if err := os.WriteFile(instructionFile, []byte("File instruction"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}

	tests := []struct {
		name          string
		instruction   string
		file          string
		want          string
		expectError   bool
		errorContains string
	}{
		{
			name:        "inline instruction",
			instruction: "Inline instruction",
			want:        "Inline instruction",
		},
		{
			name: "instruction file",
			file: instructionFile,
			want: "File instruction",
		},
		{
			name:          "inline and file conflict",
			instruction:   "Inline instruction",
			file:          instructionFile,
			expectError:   true,
			errorContains: "use either --instruction or --instruction-file",
		},
		{
			name:          "missing instruction file",
			file:          filepath.Join(tmpDir, "missing.md"),
			expectError:   true,
			errorContains: "failed to read instruction file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCreateCmd()
			if tt.instruction != "" {
				if err := cmd.Flags().Set("instruction", tt.instruction); err != nil {
					t.Fatalf("failed to set instruction flag: %v", err)
				}
			}
			if tt.file != "" {
				if err := cmd.Flags().Set("instruction-file", tt.file); err != nil {
					t.Fatalf("failed to set instruction-file flag: %v", err)
				}
			}

			got, err := resolveInstruction(cmd)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("resolveInstruction() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveTools(t *testing.T) {
	tmpDir := t.TempDir()
	toolsFile := filepath.Join(tmpDir, "tools.json")
	if err := os.WriteFile(toolsFile, []byte(`[{"name":"from-file"}]`), 0644); err != nil {
		t.Fatalf("failed to write tools file: %v", err)
	}

	tests := []struct {
		name          string
		tools         string
		file          string
		wantLen       int
		wantFirstName string
		expectError   bool
		errorContains string
	}{
		{
			name:    "empty tools",
			wantLen: 0,
		},
		{
			name:          "inline tools",
			tools:         `[{"name":"inline"}]`,
			wantLen:       1,
			wantFirstName: "inline",
		},
		{
			name:          "tools file",
			file:          toolsFile,
			wantLen:       1,
			wantFirstName: "from-file",
		},
		{
			name:          "inline and file conflict",
			tools:         `[]`,
			file:          toolsFile,
			expectError:   true,
			errorContains: "use either --tools or --tools-file",
		},
		{
			name:          "invalid JSON",
			tools:         `{invalid`,
			expectError:   true,
			errorContains: "tools must be a valid JSON array",
		},
		{
			name:          "JSON object is not an array",
			tools:         `{"name":"not-array"}`,
			expectError:   true,
			errorContains: "tools must be a valid JSON array",
		},
		{
			name:          "missing tools file",
			file:          filepath.Join(tmpDir, "missing.json"),
			expectError:   true,
			errorContains: "failed to read tools file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newCreateCmd()
			if tt.tools != "" {
				if err := cmd.Flags().Set("tools", tt.tools); err != nil {
					t.Fatalf("failed to set tools flag: %v", err)
				}
			}
			if tt.file != "" {
				if err := cmd.Flags().Set("tools-file", tt.file); err != nil {
					t.Fatalf("failed to set tools-file flag: %v", err)
				}
			}

			got, err := resolveTools(cmd)
			if tt.expectError {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				if !strings.Contains(err.Error(), tt.errorContains) {
					t.Fatalf("expected error to contain %q, got %q", tt.errorContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != tt.wantLen {
				t.Fatalf("resolveTools() length = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantFirstName != "" {
				tool, ok := got[0].(map[string]any)
				if !ok {
					t.Fatalf("first tool = %T, want map[string]any", got[0])
				}
				if tool["name"] != tt.wantFirstName {
					t.Fatalf("first tool name = %v, want %q", tool["name"], tt.wantFirstName)
				}
			}
		})
	}
}

func TestAgentCommandsUseSDKClient(t *testing.T) {
	t.Run("list agents", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[%s]}`, testAgentJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newListCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error { return runList(cmd, nil) })
		if err != nil {
			t.Fatalf("runList() returned error: %v", err)
		}
		if !strings.Contains(output, "Agent One") || !strings.Contains(output, "agent-1") {
			t.Fatalf("expected list output to contain agent details, got %q", output)
		}
	})

	t.Run("view agent", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents/agent-1") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":%s}`, testAgentJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newViewCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error { return runView(cmd, []string{"agent-1"}) })
		if err != nil {
			t.Fatalf("runView() returned error: %v", err)
		}
		if !strings.Contains(output, "Description: Helpful agent") || !strings.Contains(output, "Be helpful") {
			t.Fatalf("expected view output to contain agent details, got %q", output)
		}
	})

	t.Run("create agent", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}

			var body api.CreateAgentJSONRequestBody
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("failed to decode request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if body.Name != "Agent One" || body.AvatarId != "avatar-1" || body.VoiceProfileId != "voice-1" || body.LlmModel == nil || *body.LlmModel != "gemini-2.0-flash" {
				t.Errorf("unexpected create request body: %+v", body)
				http.Error(w, "unexpected request body", http.StatusBadRequest)
				return
			}
			if body.Instruction == nil || *body.Instruction != "Be helpful" {
				t.Errorf("instruction = %v, want Be helpful", body.Instruction)
				http.Error(w, "unexpected instruction", http.StatusBadRequest)
				return
			}
			if body.Description == nil || *body.Description != "Helpful agent" {
				t.Errorf("description = %v, want Helpful agent", body.Description)
				http.Error(w, "unexpected description", http.StatusBadRequest)
				return
			}
			if body.Model == nil || *body.Model != "metis-2.5" {
				t.Errorf("model = %v, want metis-2.5", body.Model)
				http.Error(w, "unexpected model", http.StatusBadRequest)
				return
			}
			if body.Tools == nil || len(*body.Tools) != 1 {
				t.Errorf("tools = %v, want one tool", body.Tools)
				http.Error(w, "unexpected tools", http.StatusBadRequest)
				return
			}

			writeJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":%s}`, testAgentJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newCreateCmd()
		cmd.SetContext(context.Background())
		setFlags(t, cmd, map[string]string{
			"name":        "Agent One",
			"description": "Helpful agent",
			"avatar":      "avatar-1",
			"voice":       "voice-1",
			"model":       "metis-2.5",
			"llm-model":   "gemini-2.0-flash",
			"instruction": "Be helpful",
			"tools":       `[{"type":"function","name":"search"}]`,
		})

		output, err := captureStdout(t, func() error { return runCreate(cmd, nil) })
		if err != nil {
			t.Fatalf("runCreate() returned error: %v", err)
		}
		if !strings.Contains(output, "Agent created successfully") || !strings.Contains(output, "agent-1") {
			t.Fatalf("expected create output to contain success details, got %q", output)
		}
	})

	t.Run("delete agent", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodDelete, "/v1/agents/agent-1") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, `{"data":{}}`)
		})
		configureAgentTest(t, server.URL)

		cmd := newDeleteCmd()
		cmd.SetContext(context.Background())
		if err := cmd.Flags().Set("force", "true"); err != nil {
			t.Fatalf("failed to set force flag: %v", err)
		}

		output, err := captureStdout(t, func() error { return runDelete(cmd, []string{"agent-1"}) })
		if err != nil {
			t.Fatalf("runDelete() returned error: %v", err)
		}
		if !strings.Contains(output, "Successfully deleted agent: agent-1") {
			t.Fatalf("expected delete output to contain success details, got %q", output)
		}
	})
}

func newAgentTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func configureAgentTest(t *testing.T, apiURL string) {
	t.Helper()

	tmpDir := t.TempDir()
	configContent := fmt.Sprintf("api_token: test-token\napi_url: %q\n", apiURL)
	if err := os.WriteFile(filepath.Join(tmpDir, "config.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	viper.Reset()
	t.Setenv("MIRAKO_CONFIG_PATH", tmpDir)
	t.Setenv("MIRAKO_API_TOKEN", "test-token")
	t.Setenv("MIRAKO_API_URL", apiURL)
	t.Cleanup(viper.Reset)
}

func assertRequest(t *testing.T, r *http.Request, method, path string) bool {
	t.Helper()
	if r.Method != method {
		t.Errorf("method = %s, want %s", r.Method, method)
		return false
	}
	if r.URL.Path != path {
		t.Errorf("path = %s, want %s", r.URL.Path, path)
		return false
	}
	if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
		t.Errorf("Authorization header = %q, want %q", got, "Bearer test-token")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, statusCode int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_, _ = w.Write([]byte(body))
}

func setFlags(t *testing.T, cmd *cobra.Command, flags map[string]string) {
	t.Helper()
	for name, value := range flags {
		if err := cmd.Flags().Set(name, value); err != nil {
			t.Fatalf("failed to set flag %s: %v", name, err)
		}
	}
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create stdout pipe: %v", err)
	}
	defer r.Close()

	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	fnErr := fn()

	if err := w.Close(); err != nil {
		t.Fatalf("failed to close stdout pipe: %v", err)
	}

	output, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read stdout pipe: %v", err)
	}
	return string(output), fnErr
}
