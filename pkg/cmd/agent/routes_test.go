package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

const testPermanentAgentRouteResponseJSON = `{
  "data": {
    "id": "route-capability-1",
    "agent_id": "agent-1",
    "label": null,
    "expires_at": null,
    "revoked_at": null,
    "route_version": 1,
    "status": "active",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-25T00:01:00Z",
    "path": "/a/route-capability-1",
    "url": "https://view.example.test/a/route-capability-1"
  }
}`

const testAgentRouteResponseJSON = `{
  "data": {
    "id": "route-capability-1",
    "agent_id": "agent-1",
    "label": "Production website",
    "expires_at": "2026-05-26T00:00:00Z",
    "revoked_at": null,
    "route_version": 1,
    "status": "active",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-25T00:01:00Z",
    "path": "/a/route-capability-1",
    "url": "https://view.example.test/a/route-capability-1"
  }
}`

func TestAgentRoutesCreateCommandShape(t *testing.T) {
	agentCmd := NewAgentCmd()
	routesCmd, _, err := agentCmd.Find([]string{"routes"})
	if err != nil {
		t.Fatalf("find routes command: %v", err)
	}
	if routesCmd.Name() != "routes" {
		t.Fatalf("command = %q, want routes", routesCmd.Name())
	}
	if got := len(routesCmd.Commands()); got != 1 {
		t.Fatalf("routes subcommand count = %d, want 1", got)
	}

	createCmd, _, err := agentCmd.Find([]string{"routes", "create"})
	if err != nil {
		t.Fatalf("find routes create command: %v", err)
	}
	if createCmd.Use != "create [agent-id]" {
		t.Fatalf("Use = %q, want %q", createCmd.Use, "create [agent-id]")
	}
	for _, flag := range []string{"label", "valid-for", "json"} {
		if createCmd.Flags().Lookup(flag) == nil {
			t.Errorf("missing --%s flag", flag)
		}
	}

	for _, tt := range []struct {
		name    string
		args    []string
		wantErr bool
	}{
		{name: "missing agent ID", args: nil, wantErr: true},
		{name: "one agent ID", args: []string{"agent-1"}},
		{name: "extra argument", args: []string{"agent-1", "extra"}, wantErr: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := createCmd.Args(createCmd, tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Args() error = %v, wantErr %t", err, tt.wantErr)
			}
		})
	}
}

func TestParseValidFor(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		wantSeconds *int64
		wantErr     bool
	}{
		{name: "omitted", value: ""},
		{name: "whitespace omitted", value: "  "},
		{name: "24 hours", value: "24h", wantSeconds: int64Pointer(86400)},
		{name: "combined duration", value: "1h30m", wantSeconds: int64Pointer(5400)},
		{name: "trimmed duration", value: " 2m ", wantSeconds: int64Pointer(120)},
		{name: "zero", value: "0s", wantErr: true},
		{name: "negative", value: "-1h", wantErr: true},
		{name: "malformed", value: "tomorrow", wantErr: true},
		{name: "sub-second", value: "1500ms", wantErr: true},
		{name: "overflow", value: "2562048h", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValidFor(tt.value)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseValidFor(%q) error = %v, wantErr %t", tt.value, err, tt.wantErr)
			}
			if !reflect.DeepEqual(got, tt.wantSeconds) {
				t.Fatalf("parseValidFor(%q) = %v, want %v", tt.value, got, tt.wantSeconds)
			}
		})
	}
}

func TestBuildCreateAgentRouteBody(t *testing.T) {
	t.Run("permanent route omits optional fields", func(t *testing.T) {
		body, err := buildCreateAgentRouteBody(newRoutesCreateCmd())
		if err != nil {
			t.Fatalf("buildCreateAgentRouteBody() error = %v", err)
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		if string(encoded) != "{}" {
			t.Fatalf("body = %s, want {}", encoded)
		}
	})

	t.Run("forwards label and validity", func(t *testing.T) {
		cmd := newRoutesCreateCmd()
		setFlags(t, cmd, map[string]string{
			"label":     " Production website ",
			"valid-for": "24h",
		})

		body, err := buildCreateAgentRouteBody(cmd)
		if err != nil {
			t.Fatalf("buildCreateAgentRouteBody() error = %v", err)
		}
		if body.Label == nil || *body.Label != "Production website" {
			t.Fatalf("Label = %v, want Production website", body.Label)
		}
		if body.ValiditySeconds == nil || *body.ValiditySeconds != 86400 {
			t.Fatalf("ValiditySeconds = %v, want 86400", body.ValiditySeconds)
		}
	})

	t.Run("rejects long label", func(t *testing.T) {
		cmd := newRoutesCreateCmd()
		setFlags(t, cmd, map[string]string{"label": strings.Repeat("界", 101)})
		if _, err := buildCreateAgentRouteBody(cmd); err == nil || !strings.Contains(err.Error(), "at most 100 characters") {
			t.Fatalf("error = %v, want maximum label length error", err)
		}
	})
}

func TestRunRoutesCreateRequestAndOutput(t *testing.T) {
	t.Run("permanent route", func(t *testing.T) {
		var requestBody map[string]any
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agents/agent-1/routes") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			writeJSON(w, http.StatusCreated, testPermanentAgentRouteResponseJSON)
		})
		configureAgentTest(t, server.URL)

		cmd := newRoutesCreateCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error {
			return runRoutesCreate(cmd, []string{"agent-1"})
		})
		if err != nil {
			t.Fatalf("runRoutesCreate() error = %v", err)
		}
		if len(requestBody) != 0 {
			t.Fatalf("request body = %#v, want empty object", requestBody)
		}

		for _, want := range []string{
			"Agent route created successfully.",
			"Route ID", "route-capability-1",
			"URL", "https://view.example.test/a/route-capability-1",
			"Path", "/a/route-capability-1",
			"Status", "active",
			"Expires", "Never", "Revoked", "Not revoked",
			"Route Version", "Created", "Updated",
		} {
			if !strings.Contains(output, want) {
				t.Errorf("output missing %q: %q", want, output)
			}
		}
		if strings.Contains(output, "test-token") {
			t.Fatalf("output leaked owner token: %q", output)
		}
	})

	t.Run("label duration and JSON output", func(t *testing.T) {
		var requestBody map[string]any
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agents/agent-1/routes") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			if err := json.NewDecoder(r.Body).Decode(&requestBody); err != nil {
				t.Errorf("decode request body: %v", err)
			}
			writeJSON(w, http.StatusCreated, testAgentRouteResponseJSON)
		})
		configureAgentTest(t, server.URL)

		cmd := newRoutesCreateCmd()
		cmd.SetContext(context.Background())
		setFlags(t, cmd, map[string]string{
			"label":     "Production website",
			"valid-for": "24h",
			"json":      "true",
		})

		output, err := captureStdout(t, func() error {
			return runRoutesCreate(cmd, []string{"agent-1"})
		})
		if err != nil {
			t.Fatalf("runRoutesCreate() error = %v", err)
		}
		wantBody := map[string]any{
			"label":            "Production website",
			"validity_seconds": float64(86400),
		}
		if !reflect.DeepEqual(requestBody, wantBody) {
			t.Fatalf("request body = %#v, want %#v", requestBody, wantBody)
		}

		var jsonOutput map[string]any
		if err := json.Unmarshal([]byte(output), &jsonOutput); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, output)
		}
		data, ok := jsonOutput["data"].(map[string]any)
		if !ok {
			t.Fatalf("JSON output data = %T, want object", jsonOutput["data"])
		}
		if data["id"] != "route-capability-1" || data["status"] != "active" || data["route_version"] != float64(1) {
			t.Fatalf("unexpected JSON lifecycle output: %s", output)
		}
		if strings.Contains(output, "test-token") {
			t.Fatalf("output leaked owner token: %q", output)
		}
	})
}

func TestRunRoutesCreateRejectsMalformedSuccessResponse(t *testing.T) {
	for _, tt := range []struct {
		name string
		body string
	}{
		{name: "missing data", body: `{}`},
		{name: "null data", body: `{"data":null}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if !assertRequest(t, r, http.MethodPost, "/v1/agents/agent-1/routes") {
					http.Error(w, "unexpected request", http.StatusBadRequest)
					return
				}
				writeJSON(w, http.StatusCreated, tt.body)
			})
			configureAgentTest(t, server.URL)

			cmd := newRoutesCreateCmd()
			cmd.SetContext(context.Background())
			output, err := captureStdout(t, func() error {
				return runRoutesCreate(cmd, []string{"agent-1"})
			})
			if err == nil || !strings.Contains(err.Error(), "created route is missing its ID") {
				t.Fatalf("error = %v, want malformed success response error", err)
			}
			if output != "" {
				t.Fatalf("output = %q, want no capability output", output)
			}
		})
	}
}

func TestRunRoutesCreateRejectsBlankAgentID(t *testing.T) {
	cmd := newRoutesCreateCmd()
	err := runRoutesCreate(cmd, []string{"   "})
	if err == nil || err.Error() != "agent ID is required" {
		t.Fatalf("error = %v, want agent ID is required", err)
	}
}

func int64Pointer(value int64) *int64 {
	return &value
}

func Example_parseValidFor() {
	seconds, _ := parseValidFor("24h")
	fmt.Println(*seconds)
	// Output: 86400
}
