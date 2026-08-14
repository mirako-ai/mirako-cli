package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/mirako-ai/mirako-go/api"
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

const testRevokedAgentRouteResponseJSON = `{
  "data": {
    "id": "route-capability-1",
    "agent_id": "agent-1",
    "label": "Production website",
    "expires_at": null,
    "revoked_at": "2026-05-25T01:00:00Z",
    "route_version": 2,
    "status": "revoked",
    "created_at": "2026-05-25T00:00:00Z",
    "updated_at": "2026-05-25T01:00:00Z",
    "path": "/a/route-capability-1",
    "url": "https://view.example.test/a/route-capability-1"
  }
}`

const testAgentRoutesListResponseJSON = `{
  "data": [
    {
      "id": "route-capability-2",
      "agent_id": "agent-1",
      "label": null,
      "expires_at": null,
      "revoked_at": null,
      "route_version": 1,
      "status": "active",
      "created_at": "2026-05-25T02:00:00Z",
      "updated_at": "2026-05-25T02:00:00Z",
      "path": "/a/route-capability-2"
    },
    {
      "id": "route-capability-1",
      "agent_id": "agent-1",
      "label": "Production website",
      "expires_at": null,
      "revoked_at": "2026-05-25T01:00:00Z",
      "route_version": 2,
      "status": "revoked",
      "created_at": "2026-05-25T00:00:00Z",
      "updated_at": "2026-05-25T01:00:00Z",
      "path": "/a/route-capability-1",
      "url": "https://view.example.test/a/route-capability-1"
    }
  ]
}`

func TestAgentRoutesCommandShape(t *testing.T) {
	agentCmd := NewAgentCmd()
	routesCmd, _, err := agentCmd.Find([]string{"routes"})
	if err != nil {
		t.Fatalf("find routes command: %v", err)
	}
	if routesCmd.Name() != "routes" {
		t.Fatalf("command = %q, want routes", routesCmd.Name())
	}
	if got := len(routesCmd.Commands()); got != 4 {
		t.Fatalf("routes subcommand count = %d, want 4", got)
	}

	commands := []struct {
		name  string
		use   string
		flags []string
	}{
		{name: "list", use: "list [agent-id]", flags: []string{"json"}},
		{name: "view", use: "view [route-id]", flags: []string{"json"}},
		{name: "create", use: "create [agent-id]", flags: []string{"label", "valid-for", "json"}},
		{name: "revoke", use: "revoke [route-id]", flags: []string{"force", "json"}},
	}
	for _, tt := range commands {
		t.Run(tt.name, func(t *testing.T) {
			command, _, err := agentCmd.Find([]string{"routes", tt.name})
			if err != nil {
				t.Fatalf("find routes %s command: %v", tt.name, err)
			}
			if command.Use != tt.use {
				t.Fatalf("Use = %q, want %q", command.Use, tt.use)
			}
			for _, flag := range tt.flags {
				if command.Flags().Lookup(flag) == nil {
					t.Errorf("missing --%s flag", flag)
				}
			}
			if err := command.Args(command, nil); err == nil {
				t.Error("missing positional ID should fail")
			}
			if err := command.Args(command, []string{"id-1"}); err != nil {
				t.Errorf("one positional ID should pass: %v", err)
			}
			if err := command.Args(command, []string{"id-1", "extra"}); err == nil {
				t.Error("extra positional argument should fail")
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

func TestRunRoutesListRequestAndOutput(t *testing.T) {
	t.Run("table", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents/agent-1/routes") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, testAgentRoutesListResponseJSON)
		})
		configureAgentTest(t, server.URL)

		cmd := newRoutesListCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error { return runRoutesList(cmd, []string{"agent-1"}) })
		if err != nil {
			t.Fatalf("runRoutesList() error = %v", err)
		}
		for _, want := range []string{"LABEL", "ROUTE ID", "STATUS", "EXPIRES", "UPDATED", "route-capability-2", "ACTIVE", "Production website", "route-capability-1", "REVOKED", "Never"} {
			if !strings.Contains(output, want) {
				t.Errorf("output missing %q: %q", want, output)
			}
		}
		if strings.Contains(output, "test-token") {
			t.Fatalf("output leaked owner token: %q", output)
		}
	})

	t.Run("empty", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents/agent-1/routes") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, `{"data":[]}`)
		})
		configureAgentTest(t, server.URL)

		cmd := newRoutesListCmd()
		cmd.SetContext(context.Background())
		output, err := captureStdout(t, func() error { return runRoutesList(cmd, []string{"agent-1"}) })
		if err != nil {
			t.Fatalf("runRoutesList() error = %v", err)
		}
		if strings.TrimSpace(output) != "No agent routes found" {
			t.Fatalf("output = %q, want empty-list message", output)
		}
	})

	t.Run("JSON", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agents/agent-1/routes") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, testAgentRoutesListResponseJSON)
		})
		configureAgentTest(t, server.URL)

		cmd := newRoutesListCmd()
		cmd.SetContext(context.Background())
		setFlags(t, cmd, map[string]string{"json": "true"})
		output, err := captureStdout(t, func() error { return runRoutesList(cmd, []string{"agent-1"}) })
		if err != nil {
			t.Fatalf("runRoutesList() error = %v", err)
		}
		var result map[string]any
		if err := json.Unmarshal([]byte(output), &result); err != nil {
			t.Fatalf("output is not JSON: %v\n%s", err, output)
		}
		data, ok := result["data"].([]any)
		if !ok || len(data) != 2 {
			t.Fatalf("JSON output data = %#v, want two routes", result["data"])
		}
	})
}

func TestRunRoutesViewRequestAndOutput(t *testing.T) {
	server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if !assertRequest(t, r, http.MethodGet, "/v1/agent-routes/route-capability-1") {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, testAgentRouteResponseJSON)
	})
	configureAgentTest(t, server.URL)

	cmd := newRoutesViewCmd()
	cmd.SetContext(context.Background())
	output, err := captureStdout(t, func() error { return runRoutesView(cmd, []string{"route-capability-1"}) })
	if err != nil {
		t.Fatalf("runRoutesView() error = %v", err)
	}
	for _, want := range []string{"Route ID", "route-capability-1", "Agent ID", "agent-1", "URL", "Path", "Label", "Status", "Expires", "Revoked", "Route Version", "Created", "Updated"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "test-token") {
		t.Fatalf("output leaked owner token: %q", output)
	}
}

func TestRunRoutesRevokeRequestAndOutput(t *testing.T) {
	requestCount := 0
	server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		if !assertRequest(t, r, http.MethodPost, "/v1/agent-routes/route-capability-1/revoke") {
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.ContentLength > 0 {
			t.Errorf("revoke request must not include a body")
		}
		writeJSON(w, http.StatusOK, testRevokedAgentRouteResponseJSON)
	})
	configureAgentTest(t, server.URL)

	cmd := newRoutesRevokeCmd()
	cmd.SetContext(context.Background())
	setFlags(t, cmd, map[string]string{"force": "true"})
	output, err := captureStdout(t, func() error {
		if err := runRoutesRevoke(cmd, []string{"route-capability-1"}); err != nil {
			return err
		}
		return runRoutesRevoke(cmd, []string{"route-capability-1"})
	})
	if err != nil {
		t.Fatalf("runRoutesRevoke() error = %v", err)
	}
	for _, want := range []string{"Agent route revoked successfully.", "route-capability-1", "revoked", "Route Version", "2", "Revoked"} {
		if !strings.Contains(output, want) {
			t.Errorf("output missing %q: %q", want, output)
		}
	}
	if strings.Contains(output, "test-token") {
		t.Fatalf("output leaked owner token: %q", output)
	}
	if requestCount != 2 {
		t.Fatalf("revoke request count = %d, want 2 idempotent calls", requestCount)
	}
}

func TestRouteRevokeJSONRequiresForce(t *testing.T) {
	cmd := newRoutesRevokeCmd()
	setFlags(t, cmd, map[string]string{"json": "true"})
	err := runRoutesRevoke(cmd, []string{"route-capability-1"})
	if err == nil || err.Error() != "--json requires --force for route revocation" {
		t.Fatalf("error = %v, want --json/--force validation", err)
	}
}

func TestAgentRouteCapabilityIsRedactedFromTransportErrors(t *testing.T) {
	server := newAgentTestServer(t, func(http.ResponseWriter, *http.Request) {})
	apiURL := server.URL
	server.Close()
	configureAgentTest(t, apiURL)

	cmd := newRoutesViewCmd()
	cmd.SetContext(context.Background())
	err := runRoutesView(cmd, []string{"route-capability-1"})
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), "route-capability-1") {
		t.Fatalf("transport error leaked route capability: %q", err)
	}
	if !strings.Contains(err.Error(), "REDACTED") {
		t.Fatalf("transport error did not mark redaction: %q", err)
	}
}

func TestSanitizeAgentRouteOutput(t *testing.T) {
	got := sanitizeAgentRouteOutput("Production\twebsite\n\x1b[31m")
	if got != "Production website [31m" {
		t.Fatalf("sanitized output = %q", got)
	}
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

func TestValidateAgentRouteRejectsInvalidTimestampOrder(t *testing.T) {
	createdAt := time.Date(2026, 5, 25, 1, 0, 0, 0, time.UTC)
	route := api.AgentRouteResponse{
		Id:           "route-capability-1",
		AgentId:      "agent-1",
		Path:         "/a/route-capability-1",
		Status:       api.Active,
		RouteVersion: 1,
		CreatedAt:    createdAt,
		UpdatedAt:    createdAt.Add(-time.Minute),
	}
	if err := validateAgentRoute(route); err == nil || !strings.Contains(err.Error(), "inconsistent lifecycle timestamps") {
		t.Fatalf("error = %v, want timestamp-order validation", err)
	}
}

func TestRouteManagementRejectsMalformedResponses(t *testing.T) {
	t.Run("view missing data", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodGet, "/v1/agent-routes/route-capability-1") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, `{}`)
		})
		configureAgentTest(t, server.URL)
		cmd := newRoutesViewCmd()
		cmd.SetContext(context.Background())
		if err := runRoutesView(cmd, []string{"route-capability-1"}); err == nil || !strings.Contains(err.Error(), "missing its ID") {
			t.Fatalf("error = %v, want malformed route error", err)
		}
	})

	t.Run("revoke returns active route", func(t *testing.T) {
		server := newAgentTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			if !assertRequest(t, r, http.MethodPost, "/v1/agent-routes/route-capability-1/revoke") {
				http.Error(w, "unexpected request", http.StatusBadRequest)
				return
			}
			writeJSON(w, http.StatusOK, testAgentRouteResponseJSON)
		})
		configureAgentTest(t, server.URL)
		cmd := newRoutesRevokeCmd()
		cmd.SetContext(context.Background())
		setFlags(t, cmd, map[string]string{"force": "true"})
		if err := runRoutesRevoke(cmd, []string{"route-capability-1"}); err == nil || !strings.Contains(err.Error(), "inconsistent lifecycle state") {
			t.Fatalf("error = %v, want revoke lifecycle error", err)
		}
	})
}

func TestRouteManagementRejectsBlankIDs(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
		want string
	}{
		{name: "list", run: func() error { return runRoutesList(newRoutesListCmd(), []string{"   "}) }, want: "agent ID is required"},
		{name: "view", run: func() error { return runRoutesView(newRoutesViewCmd(), []string{"   "}) }, want: "route ID is required"},
		{name: "revoke", run: func() error { return runRoutesRevoke(newRoutesRevokeCmd(), []string{"   "}) }, want: "route ID is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v, want %q", err, tt.want)
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
