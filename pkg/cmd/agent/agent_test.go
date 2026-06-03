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
	"reflect"
	"strings"
	"testing"

	promptui "github.com/mirako-ai/mirako-cli/pkg/ui/prompt"
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
  "instruction": "Be helpful",
  "tools": [{"type":"function","name":"search"}],
  "runtime_kind": "managed_agent",
  "custom_agent_protocol": "vercel_ai_sdk",
  "has_custom_agent_bearer_token": false,
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:01:00Z"
}`

const testCustomAgentJSON = `{
  "id": "custom-agent-1",
  "user_id": "user-1",
  "name": "Custom Agent",
  "description": "Custom endpoint agent",
  "avatar_id": "avatar-1",
  "voice_profile_id": "voice-1",
  "model": "metis-2.5",
  "runtime_kind": "custom_agent",
  "custom_agent_url": "https://agent.example.test/api/chat",
  "custom_agent_protocol": "vercel_ai_sdk",
  "has_custom_agent_bearer_token": true,
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:01:00Z"
}`

const testCustomAgentWithSecretJSON = `{
  "id": "custom-agent-1",
  "user_id": "user-1",
  "name": "Custom Agent",
  "description": "Custom endpoint agent",
  "avatar_id": "avatar-1",
  "voice_profile_id": "voice-1",
  "model": "metis-2.5",
  "runtime_kind": "custom_agent",
  "custom_agent_url": "https://agent.example.test/api/chat",
  "custom_agent_protocol": "vercel_ai_sdk",
  "custom_agent_bearer_token": "super-secret-token",
  "has_custom_agent_bearer_token": true,
  "created_at": "2026-05-25T00:00:00Z",
  "updated_at": "2026-05-25T00:01:00Z"
}`

func TestAgentSDKTypesOmitLlmModel(t *testing.T) {
	types := map[string]reflect.Type{
		"Agent":                      reflect.TypeOf(api.Agent{}),
		"AgentResponse":              reflect.TypeOf(api.AgentResponse{}),
		"CreateAgentJSONRequestBody": reflect.TypeOf(api.CreateAgentJSONRequestBody{}),
		"UpdateAgentJSONRequestBody": reflect.TypeOf(api.UpdateAgentJSONRequestBody{}),
	}

	for name, typ := range types {
		t.Run(name, func(t *testing.T) {
			if _, ok := typ.FieldByName("LlmModel"); ok {
				t.Fatalf("%s must not expose LlmModel", name)
			}
			for i := 0; i < typ.NumField(); i++ {
				if strings.Contains(typ.Field(i).Tag.Get("json"), "llm_model") {
					t.Fatalf("%s must not expose json field llm_model", name)
				}
			}
		})
	}
}

func TestRunCreateValidation(t *testing.T) {
	forceNonInteractive(t)

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
			name: "missing instruction",
			flags: map[string]string{
				"name":   "agent",
				"avatar": "avatar-1",
				"voice":  "voice-1",
			},
			errorContains: "instruction is required",
		},
		{
			name: "invalid runtime kind",
			flags: map[string]string{
				"name":         "agent",
				"avatar":       "avatar-1",
				"voice":        "voice-1",
				"runtime-kind": "other",
			},
			errorContains: "runtime kind must be one of managed_agent, custom_agent",
		},
		{
			name: "custom missing URL",
			flags: map[string]string{
				"name":         "agent",
				"avatar":       "avatar-1",
				"voice":        "voice-1",
				"runtime-kind": "custom_agent",
			},
			errorContains: "custom agent URL is required",
		},
		{
			name: "custom invalid protocol",
			flags: map[string]string{
				"name":                  "agent",
				"avatar":                "avatar-1",
				"voice":                 "voice-1",
				"runtime-kind":          "custom_agent",
				"custom-agent-url":      "https://agent.example.test/api/chat",
				"custom-agent-protocol": "unknown",
			},
			errorContains: "custom agent protocol must be one of vercel_ai_sdk",
		},
		{
			name: "custom bearer token conflict",
			flags: map[string]string{
				"name":                           "agent",
				"avatar":                         "avatar-1",
				"voice":                          "voice-1",
				"runtime-kind":                   "custom_agent",
				"custom-agent-url":               "https://agent.example.test/api/chat",
				"custom-agent-bearer-token":      "token",
				"custom-agent-bearer-token-file": "token.txt",
			},
			errorContains: "use either --custom-agent-bearer-token or --custom-agent-bearer-token-file",
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

func TestBuildCreateAgentBodyInteractiveManagedPrompts(t *testing.T) {
	instructionFile := filepath.Join(t.TempDir(), "instruction.md")
	instruction := "Be helpful\nUse tools wisely."
	if err := os.WriteFile(instructionFile, []byte(instruction), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}

	cmd := newCreateCmd()
	prompter := &fakeAgentPrompter{
		selects: map[string]string{
			"Agent type": managedAgentRuntimeKind,
		},
		inputs: map[string]string{
			"Agent name":                  "Managed Agent",
			"Avatar ID":                   "avatar-1",
			"Voice profile ID":            "voice-1",
			"Description (optional)":      "Helpful managed agent",
			"Interactive model":           "metis-2.5",
			instructionFilePromptLabel:    instructionFile,
			"Tools JSON array (optional)": `[{"type":"function","name":"search"}]`,
		},
	}

	body, err := buildCreateAgentBody(cmd, prompter, true)
	if err != nil {
		t.Fatalf("buildCreateAgentBody() returned error: %v", err)
	}
	if len(prompter.calls) == 0 || prompter.calls[0] != "select:Agent type" {
		t.Fatalf("expected first prompt to ask for agent type, got calls %v", prompter.calls)
	}
	if body.RuntimeKind == nil || *body.RuntimeKind != api.CreateAgentInputRuntimeKindManagedAgent {
		t.Fatalf("runtime kind = %v, want managed_agent", body.RuntimeKind)
	}
	if body.Name != "Managed Agent" || body.AvatarId != "avatar-1" || body.VoiceProfileId != "voice-1" {
		t.Fatalf("unexpected common fields: %+v", body)
	}
	assertJSONFieldAbsent(t, body, "llm_model")
	if body.Instruction == nil || *body.Instruction != instruction {
		t.Fatalf("instruction = %v, want %q", body.Instruction, instruction)
	}
	if !containsCall(prompter.calls, "input:"+instructionFilePromptLabel) {
		t.Fatalf("expected instruction file path input prompt, got calls %v", prompter.calls)
	}
	if containsCallPrefix(prompter.calls, "multiline:") {
		t.Fatalf("instruction must not use a multiline paste prompt, got calls %v", prompter.calls)
	}
	if body.Tools == nil || len(*body.Tools) != 1 {
		t.Fatalf("tools = %v, want one tool", body.Tools)
	}
}

func TestBuildCreateAgentBodyInteractiveCustomPrompts(t *testing.T) {
	cmd := newCreateCmd()
	prompter := &fakeAgentPrompter{
		selects: map[string]string{
			"Agent type": customAgentRuntimeKind,
		},
		inputs: map[string]string{
			"Agent name":        "Custom Agent",
			"Avatar ID":         "avatar-1",
			"Voice profile ID":  "voice-1",
			"Interactive model": "metis-2.5",
			"Custom agent URL":  "https://agent.example.test/api/chat",
		},
		passwords: map[string]string{
			"Custom agent bearer token (optional)": "super-secret-token",
		},
	}

	body, err := buildCreateAgentBody(cmd, prompter, true)
	if err != nil {
		t.Fatalf("buildCreateAgentBody() returned error: %v", err)
	}
	if len(prompter.calls) == 0 || prompter.calls[0] != "select:Agent type" {
		t.Fatalf("expected first prompt to ask for agent type, got calls %v", prompter.calls)
	}
	if body.RuntimeKind == nil || *body.RuntimeKind != api.CreateAgentInputRuntimeKindCustomAgent {
		t.Fatalf("runtime kind = %v, want custom_agent", body.RuntimeKind)
	}
	if body.CustomAgentUrl == nil || *body.CustomAgentUrl != "https://agent.example.test/api/chat" {
		t.Fatalf("custom agent URL = %v, want endpoint", body.CustomAgentUrl)
	}
	if body.CustomAgentBearerToken == nil || *body.CustomAgentBearerToken != "super-secret-token" {
		t.Fatalf("custom agent bearer token was not set from hidden prompt")
	}
	if body.CustomAgentProtocol == nil || *body.CustomAgentProtocol != api.CreateAgentInputCustomAgentProtocolVercelAiSdk {
		t.Fatalf("custom agent protocol = %v, want vercel_ai_sdk", body.CustomAgentProtocol)
	}
	assertJSONFieldAbsent(t, body, "llm_model")
	if body.Instruction != nil || body.Tools != nil {
		t.Fatalf("custom agents must not include managed fields: %+v", body)
	}
}

func TestAgentTypePromptOptionsIncludeDescriptionsAndRuntimeValues(t *testing.T) {
	options := agentTypePromptOptions()
	if len(options) != 2 {
		t.Fatalf("agentTypePromptOptions() length = %d, want 2", len(options))
	}

	tests := []struct {
		index       int
		wantLabel   string
		wantDesc    string
		wantValue   string
		wantRuntime api.CreateAgentInputRuntimeKind
	}{
		{
			index:       0,
			wantLabel:   "managed agent",
			wantDesc:    "provide prompt/tools and host runtime on Mirako",
			wantValue:   managedAgentRuntimeKind,
			wantRuntime: api.CreateAgentInputRuntimeKindManagedAgent,
		},
		{
			index:       1,
			wantLabel:   "custom agent",
			wantDesc:    "integrate your existing agent endpoint",
			wantValue:   customAgentRuntimeKind,
			wantRuntime: api.CreateAgentInputRuntimeKindCustomAgent,
		},
	}

	for _, tt := range tests {
		option := options[tt.index]
		if option.Label != tt.wantLabel || option.Description != tt.wantDesc || option.Value != tt.wantValue {
			t.Fatalf("option %d = %+v, want label=%q description=%q value=%q", tt.index, option, tt.wantLabel, tt.wantDesc, tt.wantValue)
		}
		got, err := runtimeKindFromChoice(option.Value)
		if err != nil {
			t.Fatalf("runtimeKindFromChoice(%q) returned error: %v", option.Value, err)
		}
		if got != tt.wantRuntime {
			t.Fatalf("runtimeKindFromChoice(%q) = %q, want %q", option.Value, got, tt.wantRuntime)
		}
	}
}

func TestBuildCreateAgentBodyCustomBearerTokenFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "token.txt")
	if err := os.WriteFile(tokenFile, []byte("file-secret\n"), 0600); err != nil {
		t.Fatalf("failed to write token file: %v", err)
	}

	cmd := newCreateCmd()
	setFlags(t, cmd, map[string]string{
		"name":                           "Custom Agent",
		"avatar":                         "avatar-1",
		"voice":                          "voice-1",
		"runtime-kind":                   "custom_agent",
		"custom-agent-url":               "https://agent.example.test/api/chat",
		"custom-agent-bearer-token-file": tokenFile,
	})

	body, err := buildCreateAgentBody(cmd, &fakeAgentPrompter{}, false)
	if err != nil {
		t.Fatalf("buildCreateAgentBody() returned error: %v", err)
	}
	if body.CustomAgentBearerToken == nil || *body.CustomAgentBearerToken != "file-secret" {
		t.Fatalf("custom agent bearer token = %v, want token from file", body.CustomAgentBearerToken)
	}
}

func TestResolveInstruction(t *testing.T) {
	tmpDir := t.TempDir()
	instructionFile := filepath.Join(tmpDir, "instruction.md")
	if err := os.WriteFile(instructionFile, []byte("File instruction"), 0644); err != nil {
		t.Fatalf("failed to write instruction file: %v", err)
	}
	emptyInstructionFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyInstructionFile, []byte("\n\t  \n"), 0644); err != nil {
		t.Fatalf("failed to write empty instruction file: %v", err)
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
		{
			name:          "empty instruction file",
			file:          emptyInstructionFile,
			expectError:   true,
			errorContains: "instruction file is empty",
		},
		{
			name:          "instruction file path is a directory",
			file:          tmpDir,
			expectError:   true,
			errorContains: "must point to a file",
		},
		{
			name:          "blank instruction file flag",
			file:          " \t ",
			expectError:   true,
			errorContains: "instruction file path is required",
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
	forceNonInteractive(t)

	t.Run("list agents", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[%s,%s]}`, testAgentJSON, testCustomAgentJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newListCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error { return runList(cmd, nil) })
		if err != nil {
			t.Fatalf("runList() returned error: %v", err)
		}
		if !strings.Contains(output, "Agent One") || !strings.Contains(output, "agent-1") || !strings.Contains(output, "custom_agent") || !strings.Contains(output, "true") {
			t.Fatalf("expected list output to contain agent details and token status, got %q", output)
		}
	})

	t.Run("list agents JSON redacts bearer token", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":[%s]}`, testCustomAgentWithSecretJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newListCmd()
		cmd.SetContext(context.Background())
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatalf("failed to set json flag: %v", err)
		}
		output, err := captureStdout(t, func() error { return runList(cmd, nil) })
		if err != nil {
			t.Fatalf("runList() returned error: %v", err)
		}
		assertNoSecret(t, output)
		if !strings.Contains(output, "has_custom_agent_bearer_token") {
			t.Fatalf("expected JSON output to include token status, got %q", output)
		}
	})

	t.Run("view managed agent", func(t *testing.T) {
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
		if !strings.Contains(output, "Description: Helpful agent") || !strings.Contains(output, "Be helpful") || !strings.Contains(output, "Custom Agent Bearer Token Configured: false") {
			t.Fatalf("expected view output to contain managed agent details, got %q", output)
		}
		if strings.Contains(output, "LLM Model:") {
			t.Fatalf("managed agent view should not show llm model, got %q", output)
		}
		if strings.Contains(output, "Custom Agent URL:") {
			t.Fatalf("managed agent view should not show custom-agent endpoint fields, got %q", output)
		}
	})

	t.Run("view custom agent JSON redacts bearer token", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents/custom-agent-1") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, fmt.Sprintf(`{"data":%s}`, testCustomAgentWithSecretJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newViewCmd()
		cmd.SetContext(context.Background())
		if err := cmd.Flags().Set("json", "true"); err != nil {
			t.Fatalf("failed to set json flag: %v", err)
		}
		output, err := captureStdout(t, func() error { return runView(cmd, []string{"custom-agent-1"}) })
		if err != nil {
			t.Fatalf("runView() returned error: %v", err)
		}
		assertNoSecret(t, output)
		if !strings.Contains(output, "has_custom_agent_bearer_token") {
			t.Fatalf("expected JSON output to include token status, got %q", output)
		}
	})

	t.Run("create managed agent", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			var body api.CreateAgentJSONRequestBody
			if err := json.Unmarshal(bodyBytes, &body); err != nil {
				t.Errorf("failed to decode request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			var raw map[string]any
			if err := json.Unmarshal(bodyBytes, &raw); err != nil {
				t.Errorf("failed to decode raw request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if body.Name != "Agent One" || body.AvatarId != "avatar-1" || body.VoiceProfileId != "voice-1" {
				t.Errorf("unexpected create request body: %+v", body)
				http.Error(w, "unexpected request body", http.StatusBadRequest)
				return
			}
			if _, ok := raw["llm_model"]; ok {
				t.Errorf("managed create request included llm_model: %s", string(bodyBytes))
				http.Error(w, "llm_model present", http.StatusBadRequest)
				return
			}
			if body.RuntimeKind == nil || *body.RuntimeKind != api.CreateAgentInputRuntimeKindManagedAgent {
				t.Errorf("runtime kind = %v, want managed_agent", body.RuntimeKind)
				http.Error(w, "unexpected runtime kind", http.StatusBadRequest)
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
			"instruction": "Be helpful",
			"tools":       `[{"type":"function","name":"search"}]`,
		})

		output, err := captureStdout(t, func() error { return runCreate(cmd, nil) })
		if err != nil {
			t.Fatalf("runCreate() returned error: %v", err)
		}
		if !strings.Contains(output, "Agent created successfully") || !strings.Contains(output, "agent-1") || !strings.Contains(output, "Runtime Kind: managed_agent") {
			t.Fatalf("expected create output to contain managed success details, got %q", output)
		}
		if strings.Contains(output, "LLM Model:") {
			t.Fatalf("managed agent create output should not show llm model, got %q", output)
		}
		if strings.Contains(output, "Custom Agent URL:") {
			t.Fatalf("managed agent create output should not show custom-agent endpoint fields, got %q", output)
		}
	})

	t.Run("create custom agent", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agents") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("failed to read request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			var raw map[string]any
			if err := json.Unmarshal(bodyBytes, &raw); err != nil {
				t.Errorf("failed to decode request body: %v", err)
				http.Error(w, "invalid request body", http.StatusBadRequest)
				return
			}
			if raw["name"] != "Custom Agent" || raw["avatar_id"] != "avatar-1" || raw["voice_profile_id"] != "voice-1" {
				t.Errorf("unexpected custom create request body: %s", string(bodyBytes))
				http.Error(w, "unexpected request body", http.StatusBadRequest)
				return
			}
			if raw["runtime_kind"] != customAgentRuntimeKind || raw["custom_agent_url"] != "https://agent.example.test/api/chat" || raw["custom_agent_protocol"] != customAgentProtocolVercelAISDK {
				t.Errorf("unexpected custom runtime fields: %s", string(bodyBytes))
				http.Error(w, "unexpected runtime fields", http.StatusBadRequest)
				return
			}
			if raw["custom_agent_bearer_token"] != "super-secret-token" {
				t.Errorf("custom_agent_bearer_token = %v, want request token", raw["custom_agent_bearer_token"])
				http.Error(w, "unexpected token", http.StatusBadRequest)
				return
			}
			for _, managedField := range []string{"llm_model", "instruction", "tools"} {
				if _, ok := raw[managedField]; ok {
					t.Errorf("custom create request included managed field %s: %s", managedField, string(bodyBytes))
					http.Error(w, "managed field present", http.StatusBadRequest)
					return
				}
			}

			writeJSON(w, http.StatusCreated, fmt.Sprintf(`{"data":%s}`, testCustomAgentWithSecretJSON))
		})
		configureAgentTest(t, server.URL)

		cmd := newCreateCmd()
		cmd.SetContext(context.Background())
		setFlags(t, cmd, map[string]string{
			"name":                      "Custom Agent",
			"description":               "Custom endpoint agent",
			"avatar":                    "avatar-1",
			"voice":                     "voice-1",
			"runtime-kind":              "custom_agent",
			"custom-agent-url":          "https://agent.example.test/api/chat",
			"custom-agent-bearer-token": "super-secret-token",
			"instruction":               "Do not send",
			"tools":                     `[{"name":"do-not-send"}]`,
		})

		output, err := captureStdout(t, func() error { return runCreate(cmd, nil) })
		if err != nil {
			t.Fatalf("runCreate() returned error: %v", err)
		}
		assertNoSecret(t, output)
		if !strings.Contains(output, "Agent created successfully") || !strings.Contains(output, "custom-agent-1") || !strings.Contains(output, "Custom Agent Bearer Token Configured: true") {
			t.Fatalf("expected custom create output to contain success details and token status, got %q", output)
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

func forceNonInteractive(t *testing.T) {
	t.Helper()
	old := stdinIsTTY
	stdinIsTTY = func() bool { return false }
	t.Cleanup(func() { stdinIsTTY = old })
}

func assertNoSecret(t *testing.T, output string) {
	t.Helper()
	if strings.Contains(output, "super-secret-token") {
		t.Fatalf("output leaked bearer token: %q", output)
	}
}

func assertJSONFieldAbsent(t *testing.T, value any, field string) {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("failed to marshal value: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("failed to decode marshaled value: %v", err)
	}
	if _, ok := raw[field]; ok {
		t.Fatalf("field %q must be absent from JSON: %s", field, string(encoded))
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

func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func containsCallPrefix(calls []string, prefix string) bool {
	for _, call := range calls {
		if strings.HasPrefix(call, prefix) {
			return true
		}
	}
	return false
}

type fakeAgentPrompter struct {
	selects       map[string]string
	selectOptions map[string][]promptui.SelectOption
	inputs        map[string]string
	passwords     map[string]string
	calls         []string
}

func (p *fakeAgentPrompter) Select(message string, options []promptui.SelectOption, defaultValue string) (string, error) {
	p.calls = append(p.calls, "select:"+message)
	if p.selectOptions == nil {
		p.selectOptions = map[string][]promptui.SelectOption{}
	}
	p.selectOptions[message] = append([]promptui.SelectOption(nil), options...)
	if p.selects != nil {
		if answer, ok := p.selects[message]; ok {
			return answer, nil
		}
	}
	return defaultValue, nil
}

func (p *fakeAgentPrompter) Input(message string, defaultValue string, required bool) (string, error) {
	p.calls = append(p.calls, "input:"+message)
	if p.inputs != nil {
		if answer, ok := p.inputs[message]; ok {
			return answer, nil
		}
	}
	return defaultValue, nil
}

func (p *fakeAgentPrompter) Password(message string) (string, error) {
	p.calls = append(p.calls, "password:"+message)
	if p.passwords != nil {
		if answer, ok := p.passwords[message]; ok {
			return answer, nil
		}
	}
	return "", nil
}
