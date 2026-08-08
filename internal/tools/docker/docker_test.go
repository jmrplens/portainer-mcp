package docker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

// specByOperationID returns the ActionSpec declaring OperationID id. It
// exists alongside findSpec (which looks up by the catalog's dotted Name)
// because the hand-written string-identifier tests below are keyed by
// OperationID — the identifier docs/api-divergences.md and
// cmd/gen_action_inputs's own refusal message both name — not by the action
// Name this domain happened to choose for it.
func specByOperationID(t *testing.T, id string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range Specs() {
		if s.OperationID == id {
			return s
		}
	}
	t.Fatalf("no action declares OperationID %q", id)
	return toolutil.ActionSpec{}
}

// callHandler builds a portainer.Client against serverURL — the same
// construction clientFor uses, factored out here because these tests need
// the *httptest.Server created first (to capture what path it received)
// before a client can point at it — marshals input, and calls spec.Handler
// directly.
func callHandler(t *testing.T, spec toolutil.ActionSpec, serverURL string, input any) (any, error) {
	t.Helper()
	c, err := portainer.New(&config.Config{URL: serverURL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	raw, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal input %#v: %v", input, err)
	}
	return spec.Handler(context.Background(), c, raw)
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

// TestUnit_HandWrittenSpecs_CoversTheThreeStringIdentifierOperations
// replaces Task 2's TestUnit_HandWrittenSpecs_IsEmptyForThisTask now that
// Task 3 has landed: handWrittenSpecs must declare exactly these three
// operations, pinned by OperationID rather than by count (a count passes
// just as happily when one operation is swapped for another), and Specs()
// must be generatedSpecs() plus exactly these three, nothing dropped and
// nothing extra.
func TestUnit_HandWrittenSpecs_CoversTheThreeStringIdentifierOperations(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"DockerContainerGpusInspect": true,
		"ContainerImageStatus":       true,
		"ServiceImageStatus":         true,
	}
	got := map[string]bool{}
	for _, s := range handWrittenSpecs() {
		got[s.OperationID] = true
	}
	for id := range want {
		if !got[id] {
			t.Errorf("handWrittenSpecs() is missing %s", id)
		}
	}
	for id := range got {
		if !want[id] {
			t.Errorf("handWrittenSpecs() contains unexpected %s; if this is deliberate the ratchet and this table both need updating", id)
		}
	}
	if got, want := len(Specs()), len(generatedSpecs())+len(handWrittenSpecs()); got != want {
		t.Errorf("len(Specs()) = %d, want %d (generatedSpecs() + handWrittenSpecs())", got, want)
	}
}

// TestUnit_HandWrittenSpecs_EditionsMatchVendoredSpec is the hand-written
// counterpart of TestUnit_Specs_EditionsMatchVendoredSpec above:
// DockerContainerGpusInspect is declared in both vendored specifications,
// while ContainerImageStatus and ServiceImageStatus are Business Edition
// only — confirmed directly against api/specs/{ce,ee}-2.44.0.json, and the
// exact reason Task 3's ratchet move is +1 CE but +3 EE.
func TestUnit_HandWrittenSpecs_EditionsMatchVendoredSpec(t *testing.T) {
	t.Parallel()
	want := map[string]edition.Edition{
		"DockerContainerGpusInspect": edition.CE,
		"ContainerImageStatus":       edition.EE,
		"ServiceImageStatus":         edition.EE,
	}
	for _, s := range handWrittenSpecs() {
		if s.Edition != want[s.OperationID] {
			t.Errorf("%s (%s): Edition = %v, want %v", s.Name, s.OperationID, s.Edition, want[s.OperationID])
		}
	}
}

// TestUnit_HandWrittenSpecs_PublishStringIdentifiers pins the reason these
// three exist. The vendored specification types containerId and serviceId as
// integer, and the generated client bakes that in — so an action publishing
// an integer here would be uncallable with any real Docker id, while still
// looking perfectly well-formed to every other gate.
//
// Two properties matter and they are checked separately, on purpose: that
// the published schema types the field "string" (this test), and that the
// handler actually puts that string in the request path (see
// TestUnit_Handlers_PutTheIdentifierInThePath below) — a spec that publishes
// the right type but a handler that then drops or coerces it would pass this
// test alone.
func TestUnit_HandWrittenSpecs_PublishStringIdentifiers(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ operationID, param string }{
		{"DockerContainerGpusInspect", "containerId"},
		{"ContainerImageStatus", "containerId"},
		{"ServiceImageStatus", "serviceId"},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			spec := specByOperationID(t, tc.operationID)
			schema, err := spec.InputSchema()
			if err != nil {
				t.Fatalf("InputSchema() error = %v", err)
			}
			props, _ := schema["properties"].(map[string]any)
			propRaw, ok := props[tc.param]
			if !ok {
				t.Fatalf("%s publishes no %q property", tc.operationID, tc.param)
			}
			prop, ok := propRaw.(map[string]any)
			if !ok {
				t.Fatalf("%s.%s schema is %T, want map[string]any", tc.operationID, tc.param, propRaw)
			}
			gotType, _ := prop["type"].(string)
			if gotType != "string" {
				t.Errorf("%s.%s is %q, want \"string\": the spec says integer and the spec is wrong (docs/api-divergences.md)", tc.operationID, tc.param, gotType)
			}
			if minimum, present := prop["minimum"]; present {
				t.Errorf("%s.%s carries minimum %v; a numeric bound on a string field is a constraint JSON Schema ignores", tc.operationID, tc.param, minimum)
			}
		})
	}
}

// TestUnit_Handlers_PutTheIdentifierInThePath is the other half: publishing a
// string is useless if the handler then drops it or coerces it. Each handler
// is called against a stub server that records the path it received.
//
// Every want path carries the "/api" prefix portainer.Client always adds
// (baseURL := cfg.URL + "/api", internal/portainer/client.go) — the same
// prefix every other handler test in this file asserts against
// (TestUnit_DockerDashboard_Success_CallsCorrectPathWithAPIKey and its
// siblings above).
func TestUnit_Handlers_PutTheIdentifierInThePath(t *testing.T) {
	t.Parallel()
	const hexID = "631eefbb7ff1a1b2c3d4e5f60718293a4b5c6d7e8f90a1b2c3d4e5f607182930"
	for _, tc := range []struct {
		operationID string
		input       any
		wantPath    string
	}{
		{"DockerContainerGpusInspect", dockerContainerGpusInspectInput{EnvironmentID: 1, ContainerID: hexID}, "/api/docker/1/containers/" + hexID + "/gpus"},
		{"ContainerImageStatus", containerImageStatusInput{EnvironmentID: 1, ContainerID: hexID}, "/api/docker/1/containers/" + hexID + "/image_status"},
		{"ServiceImageStatus", serviceImageStatusInput{EnvironmentID: 1, ServiceID: "9mnpnzenvg8p8tdbtq4wvbkcz"}, "/api/docker/1/services/9mnpnzenvg8p8tdbtq4wvbkcz/image_status"},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			var seen string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seen = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			spec := specByOperationID(t, tc.operationID)
			if _, err := callHandler(t, spec, srv.URL, tc.input); err != nil {
				t.Fatalf("%s handler: %v", tc.operationID, err)
			}
			if seen != tc.wantPath {
				t.Errorf("%s requested %q, want %q", tc.operationID, seen, tc.wantPath)
			}
		})
	}
}

// TestUnit_Handlers_EscapePathSegments is the concern no test above covers:
// url.PathEscape is what stands between a caller-supplied identifier and a
// path parameter that reads as more than one path segment. A container or
// service ID is hex/alphanumeric today, but nothing in the wire format
// guarantees that forever, and an unescaped "/" inside the identifier would
// splice extra segments into the route regardless of whether it also spells
// "..".
//
// r.URL.Path is the wrong field to assert this against: Go's net/url always
// hands back the fully percent-decoded path there, so a "/" that arrived
// safely escaped as "%2F" looks identical, once decoded, to one that never
// was. r.RequestURI is the literal, unparsed request-target the client sent
// on the wire — the only field that can tell "one escaped segment" apart
// from "two segments" here.
func TestUnit_Handlers_EscapePathSegments(t *testing.T) {
	t.Parallel()
	const trickyID = "abc/def"
	wantEscaped := url.PathEscape(trickyID)
	for _, tc := range []struct {
		operationID string
		input       any
	}{
		{"DockerContainerGpusInspect", dockerContainerGpusInspectInput{EnvironmentID: 1, ContainerID: trickyID}},
		{"ContainerImageStatus", containerImageStatusInput{EnvironmentID: 1, ContainerID: trickyID}},
		{"ServiceImageStatus", serviceImageStatusInput{EnvironmentID: 1, ServiceID: trickyID}},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			var seenRequestURI string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenRequestURI = r.RequestURI
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			spec := specByOperationID(t, tc.operationID)
			if _, err := callHandler(t, spec, srv.URL, tc.input); err != nil {
				t.Fatalf("%s handler: %v", tc.operationID, err)
			}
			if !strings.Contains(seenRequestURI, wantEscaped) {
				t.Errorf("%s sent request-target %q, want it to contain the escaped identifier %q — an unescaped \"/\" inside a path segment is how a path parameter becomes a path traversal", tc.operationID, seenRequestURI, wantEscaped)
			}
		})
	}
}

// TestUnit_ContainerImageStatus_And_ServiceImageStatus_ForwardRefreshQuery
// covers the one field these two hand-written actions have beyond their
// path-parameter divergence: the spec's own optional "refresh" query
// parameter, added here rather than left out silently — an action that
// dropped a real, spec-documented parameter would be exactly the kind of
// gating removal make audit-spec-drift exists to catch.
func TestUnit_ContainerImageStatus_And_ServiceImageStatus_ForwardRefreshQuery(t *testing.T) {
	t.Parallel()
	refresh := true
	for _, tc := range []struct {
		operationID string
		input       any
		wantPath    string
	}{
		{"ContainerImageStatus", containerImageStatusInput{EnvironmentID: 2, ContainerID: "abc123", Refresh: &refresh}, "/api/docker/2/containers/abc123/image_status"},
		{"ServiceImageStatus", serviceImageStatusInput{EnvironmentID: 2, ServiceID: "svc123", Refresh: &refresh}, "/api/docker/2/services/svc123/image_status"},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			var seenPath, seenQuery string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenPath, seenQuery = r.URL.Path, r.URL.RawQuery
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			spec := specByOperationID(t, tc.operationID)
			if _, err := callHandler(t, spec, srv.URL, tc.input); err != nil {
				t.Fatalf("%s handler: %v", tc.operationID, err)
			}
			if seenPath != tc.wantPath {
				t.Errorf("%s path = %q, want %q", tc.operationID, seenPath, tc.wantPath)
			}
			values, err := url.ParseQuery(seenQuery)
			if err != nil {
				t.Fatalf("parse query %q: %v", seenQuery, err)
			}
			if values.Get("refresh") != "true" {
				t.Errorf("%s query refresh = %q, want true; explicit Refresh was dropped", tc.operationID, values.Get("refresh"))
			}
		})
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
