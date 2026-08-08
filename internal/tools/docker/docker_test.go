package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func clientFor(t *testing.T, handler http.HandlerFunc) *portainer.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c
}

// findSpec returns the full declared ActionSpec for name, needed by tests
// that must route through tools.Execute (which takes the whole spec, to run
// the same schema validation every real caller goes through) instead of
// calling the handler directly.
func findSpec(t *testing.T, name string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range Specs() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("action %q not declared", name)
	return toolutil.ActionSpec{}
}

func find(t *testing.T, name string) func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
	t.Helper()
	return findSpec(t, name).Handler
}

// TestUnit_Specs_CoversEveryMechanicalOperation pins the set this domain
// contributes, by operationId rather than by count: a count passes just as
// happily when one operation is swapped for another.
func TestUnit_Specs_CoversEveryMechanicalOperation(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"ContainersImageStatusClear": true,
		"DockerDashboard":            true,
		"DockerImagesList":           true,
		"ServiceImageStatusClear":    true,
		"StacksImageStatusClear":     true,
	}
	got := map[string]bool{}
	for _, s := range generatedSpecs() {
		got[s.OperationID] = true
	}
	for id := range want {
		if !got[id] {
			t.Errorf("generatedSpecs() is missing %s", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("generatedSpecs() contains unexpected %s; if this is deliberate the ratchet and this table both need updating", id)
		}
	}
}

// TestUnit_Specs_AreAllValid guards the mechanical wiring itself (schema
// buildable, handler non-nil, name non-empty) rather than any one field's
// value.
func TestUnit_Specs_AreAllValid(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}

// TestUnit_Specs_EditionsMatchVendoredSpec pins each mechanical operation's
// edition individually, by OperationID, rather than a bare CE/EE count: a
// count of "2 CE, 3 EE" passes identically whether DockerDashboard or
// StacksImageStatusClear is the one wrongly marked CE, so only naming every
// operation's expected edition catches a swap between two operations of the
// same edition split.
func TestUnit_Specs_EditionsMatchVendoredSpec(t *testing.T) {
	t.Parallel()
	want := map[string]edition.Edition{
		"ContainersImageStatusClear": edition.EE,
		"DockerDashboard":            edition.CE,
		"DockerImagesList":           edition.CE,
		"ServiceImageStatusClear":    edition.EE,
		"StacksImageStatusClear":     edition.EE,
	}
	for _, s := range generatedSpecs() {
		if s.Edition != want[s.OperationID] {
			t.Errorf("%s (%s): Edition = %v, want %v", s.Name, s.OperationID, s.Edition, want[s.OperationID])
		}
	}
}

// TestUnit_Specs_MutatingAndDestructiveFlags_MatchHTTPSemantics pins each
// action's Mutating/Destructive flags by name: the three POST cache-clear
// operations must be Mutating (they change server-side cache state) but not
// Destructive (clearing a status cache loses no resource — Docker/Swarm
// simply repopulates it on the next read), and the two GET operations must
// be neither.
func TestUnit_Specs_MutatingAndDestructiveFlags_MatchHTTPSemantics(t *testing.T) {
	t.Parallel()
	mutating := map[string]bool{
		"docker.containers_image_status_clear": true,
		"docker.dashboard":                     false,
		"docker.images_list":                   false,
		"docker.service_image_status_clear":    true,
		"docker.stacks_image_status_clear":     true,
	}
	for _, s := range generatedSpecs() {
		want, ok := mutating[s.Name]
		if !ok {
			t.Fatalf("%s: no expectation recorded in this test's table", s.Name)
		}
		if s.Mutating != want {
			t.Errorf("%s: Mutating = %v, want %v", s.Name, s.Mutating, want)
		}
		if s.Destructive {
			t.Errorf("%s: Destructive = true, want false — a cache clear loses no resource", s.Name)
		}
	}
}

// TestUnit_HandWrittenSpecs_IsEmptyForThisTask guards Task 2's own scope:
// the three string-identifier operations are Task 3's, and until that task
// lands handWrittenSpecs must contribute nothing, so Specs() equals
// generatedSpecs() exactly.
func TestUnit_HandWrittenSpecs_IsEmptyForThisTask(t *testing.T) {
	t.Parallel()
	if got := len(handWrittenSpecs()); got != 0 {
		t.Errorf("handWrittenSpecs() has %d entries, want 0 until Task 3", got)
	}
	if got, want := len(Specs()), len(generatedSpecs()); got != want {
		t.Errorf("len(Specs()) = %d, want %d (generatedSpecs() alone)", got, want)
	}
}

func TestUnit_DockerDashboard_Success_CallsCorrectPathWithAPIKey(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod, gotAPIKey string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotAPIKey = r.Header.Get("X-Api-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"images":{"total":3,"size":1024},"networks":2,"services":1,"stacks":1,"volumes":4}`))
	})

	out, err := find(t, "docker.dashboard")(context.Background(), c, json.RawMessage(`{"environmentId":5}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/docker/5/dashboard" || gotMethod != http.MethodGet {
		t.Errorf("called %s %s, want GET /api/docker/5/dashboard", gotMethod, gotPath)
	}
	if gotAPIKey != "t" {
		t.Errorf("X-Api-Key = %q, want the configured token — this domain must never add a JWT code path", gotAPIKey)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

// TestUnit_DockerDashboard_InvalidEnvironmentID_ReturnsErrorWithoutCallingAPI
// pins the non-positive-id guard the generated "minimum": 1 constraint
// enforces via tools.Execute, before any handler runs.
func TestUnit_DockerDashboard_InvalidEnvironmentID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, "docker.dashboard")
	for _, id := range []int{0, -1} {
		var called atomic.Bool
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

		input, _ := json.Marshal(map[string]any{"environmentId": id})
		result, err := tools.Execute(context.Background(), spec, tools.Deps{Client: c}, input)
		if err != nil {
			t.Fatalf("environmentId=%d: Execute error = %v", id, err)
		}
		if !result.IsError {
			t.Errorf("environmentId=%d: result.IsError = false, want true for a non-positive environmentId", id)
		}
		if called.Load() {
			t.Errorf("environmentId=%d: the API was called despite an invalid environmentId", id)
		}
	}
}

func TestUnit_DockerImagesList_Success_ForwardsWithUsageQueryParam(t *testing.T) {
	t.Parallel()
	var gotPath, gotQuery string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":"sha256:abc"}]`))
	})

	out, err := find(t, "docker.images_list")(context.Background(), c, json.RawMessage(`{"environmentId":7,"withUsage":true}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/docker/7/images" {
		t.Errorf("path = %q, want /api/docker/7/images", gotPath)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", gotQuery, err)
	}
	if values.Get("withUsage") != "true" {
		t.Errorf("query withUsage = %q, want true; explicit withUsage was dropped", values.Get("withUsage"))
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestUnit_ContainersImageStatusClear_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "docker.containers_image_status_clear")(context.Background(), c, json.RawMessage(`{"environmentId":9}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/docker/9/containers/image_status/clear" || gotMethod != http.MethodPost {
		t.Errorf("called %s %s, want POST /api/docker/9/containers/image_status/clear", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestUnit_ServiceImageStatusClear_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "docker.service_image_status_clear")(context.Background(), c, json.RawMessage(`{"environmentId":9}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/docker/9/services/image_status/clear" || gotMethod != http.MethodPost {
		t.Errorf("called %s %s, want POST /api/docker/9/services/image_status/clear", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

// TestUnit_StacksImageStatusClear_Success_CallsRootStacksPath is the
// regression guard for this domain's own documented surprise: the operation
// is tagged "docker" but its route is /stacks/image_status/clear, not
// /docker/.... Asserting the literal path is what would catch a future hand
// edit that "fixed" the route to look like its sibling actions.
func TestUnit_StacksImageStatusClear_Success_CallsRootStacksPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod, gotQuery string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "docker.stacks_image_status_clear")(context.Background(), c, json.RawMessage(`{"environmentId":9,"swarmId":"abc123"}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/stacks/image_status/clear" || gotMethod != http.MethodPost {
		t.Errorf("called %s %s, want POST /api/stacks/image_status/clear", gotMethod, gotPath)
	}
	values, err := url.ParseQuery(gotQuery)
	if err != nil {
		t.Fatalf("parse query %q: %v", gotQuery, err)
	}
	if values.Get("environmentId") != "9" || values.Get("swarmId") != "abc123" {
		t.Errorf("query = %q, want environmentId=9 and swarmId=abc123 forwarded", gotQuery)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestUnit_DockerDashboard_Unauthorized_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})

	_, err := find(t, "docker.dashboard")(context.Background(), c, json.RawMessage(`{"environmentId":1}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 401 classified")
	}
}
