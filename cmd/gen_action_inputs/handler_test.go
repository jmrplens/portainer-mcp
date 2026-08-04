package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// mustParseOnly parses (but does not type-check) generated Go source,
// failing the test if it does not even parse. Used only where mustCompile's
// importer.Default() cannot resolve this module's own internal/* import
// paths (see its call sites' own comments for why a real compile-and-run
// proof exists for the same source elsewhere in this file instead).
func mustParseOnly(t *testing.T, src []byte) {
	t.Helper()
	fset := token.NewFileSet()
	if _, err := parser.ParseFile(fset, "generated.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
}

// realOperation loads the real vendored EE specification and returns the
// operation named operationID, exactly as main.go's own run() would see it.
// Building test fixtures from the real spec, rather than hand-transcribing a
// parameter list by hand, is deliberate: a hand-built fixture can silently
// drift from what the real spec (and therefore the real generated client)
// actually declares, which is exactly the kind of defect this generator
// exists to catch, not reproduce in its own tests.
func realOperation(t *testing.T, operationID string) (operation, *document, *resolver) {
	t.Helper()
	doc, paths, err := loadDocument("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}
	for _, ops := range byTag {
		for _, op := range ops {
			if op.OperationID == operationID {
				return op, doc, &resolver{doc: doc}
			}
		}
	}
	t.Fatalf("operation %q not found in the real vendored spec", operationID)
	return operation{}, nil, nil
}

// buildRealHandlerSpec runs the real generation path (assembleOperationFields,
// then buildHandlerSpec) for operationID against the real spec, returning
// everything executeGeneratedHandler needs to compile and run it: the
// Input struct(s), if any, and the handlerSpec.
func buildRealHandlerSpec(t *testing.T, domain, operationID string) ([]structSpec, handlerSpec) {
	t.Helper()
	op, doc, res := realOperation(t, operationID)
	structName := inputStructName(op.OperationID)
	var nested []structSpec
	fields, pathOrder, err := assembleOperationFields(op, res, doc, structName, &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields(%s) error = %v", operationID, err)
	}
	var structs []structSpec
	inputStruct := ""
	if len(fields) > 0 {
		inputStruct = structName
		structs = append(structs, structSpec{Name: structName, Fields: fields})
		structs = append(structs, nested...)
	}
	spec, err := buildHandlerSpec(domain, op, fields, pathOrder, nested, inputStruct, "")
	if err != nil {
		t.Fatalf("buildHandlerSpec(%s) error = %v", operationID, err)
	}
	return structs, spec
}

// stubExecResult is what the generated harness program (see
// harnessProgramTemplate) reports about the one request it observed.
type stubExecResult struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Query  string `json:"query"`
	Body   string `json:"body"`
}

// harnessProgramTemplate is a small, throwaway `package main` this test
// writes alongside the generated Input struct(s) and handler, then runs
// with `go run`: it starts a recording HTTP stub, builds a real
// *portainer.Client pointed at it, calls the generated handler with the
// caller-supplied input, and prints the one request the stub observed as
// JSON. This is deliberately a real, separate `go run` process rather than
// go/types type-checking (mustCompile, used elsewhere in this package):
// compiling the generated call proves it is well-typed, but two int path
// arguments are interchangeable to the type checker and to a human reading
// the generated source — only a real request shows which one reached the
// URL, which is exactly the standing risk this project's own plan calls out
// for path+path, path+path+params and path+path+path.
const harnessProgramTemplate = `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

func main() {
	var method, path, query, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		method, path, query, body = r.Method, r.URL.Path, r.URL.RawQuery, string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{}"))
	}))
	defer server.Close()

	c, err := portainer.New(&config.Config{URL: server.URL, Token: "test-token"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build client:", err)
		os.Exit(1)
	}

	if _, err := %s(context.Background(), c, json.RawMessage(%s)); err != nil {
		fmt.Fprintln(os.Stderr, "handler error:", err)
		os.Exit(1)
	}

	out, _ := json.Marshal(map[string]string{"method": method, "path": path, "query": query, "body": body})
	fmt.Println(string(out))
}
`

// executeGeneratedHandler renders structs (the operation's Input struct(s),
// nil when it has none) and spec (its handler) as a real, standalone Go
// package under this directory, compiles and runs it against a recording
// HTTP stub via `go run`, and returns the single request the stub observed.
// The temporary package must live inside this module (under
// cmd/gen_action_inputs), not an arbitrary directory: Go's internal-package
// visibility rule is keyed on the importing package's own import path, and
// only a path rooted under github.com/jmrplens/portainer-mcp/... may import
// internal/portainer, internal/config or internal/toolutil at all.
func executeGeneratedHandler(t *testing.T, structs []structSpec, spec handlerSpec, inputJSON string) stubExecResult {
	t.Helper()

	dir, err := os.MkdirTemp(".", ".gentest-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if len(structs) > 0 {
		src, err := renderFile("main", "test-spec.json", structs)
		if err != nil {
			t.Fatalf("renderFile() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "input.go"), src, 0o600); err != nil {
			t.Fatalf("write input.go: %v", err)
		}
	}

	handlerSrc, err := renderActionsFile("main", "test-spec.json", []handlerSpec{spec}, nil, false)
	if err != nil {
		t.Fatalf("renderActionsFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), handlerSrc, 0o600); err != nil {
		t.Fatalf("write handler.go: %v", err)
	}

	harness := fmt.Sprintf(harnessProgramTemplate, spec.FuncName, fmt.Sprintf("%q", inputJSON))
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(harness), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run generated handler failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var result stubExecResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		t.Fatalf("harness did not print valid JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return result
}

// --- shape coverage: every scenario is built from a real operation in the
// vendored spec and actually executed against a recording stub, not merely
// type-checked, per this package's standing warning about assertions that
// would pass against a defect a generator produces.

func TestUnit_GenerateHandler_Path_BindsPathArgumentAndAcknowledgesNoBody(t *testing.T) {
	t.Parallel()
	// TagDelete: DELETE /tags/{id}, no query, no body, and — per
	// responseInfoFor — no success body at all, covering both the "path"
	// shape and the "operation with no body" acknowledgement in one case.
	structs, spec := buildRealHandlerSpec(t, "tags", "TagDelete")
	if len(spec.PathArgs) != 1 || spec.HasQuery || spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want exactly one path argument and no query/body", spec)
	}
	if spec.Response.SuccessField != "" {
		t.Fatalf("Response.SuccessField = %q, want \"\": TagDelete's response carries no success body", spec.Response.SuccessField)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"id":7}`)
	if got.Method != http.MethodDelete || got.Path != "/api/tags/7" {
		t.Errorf("request = %s %s, want DELETE /api/tags/7: the path argument was mis-bound", got.Method, got.Path)
	}
}

func TestUnit_GenerateHandler_Body_SendsOnlyBodyFields(t *testing.T) {
	t.Parallel()
	// TagCreate: POST /tags, body only ({"Name": ...}), no path, no query.
	structs, spec := buildRealHandlerSpec(t, "tags", "TagCreate")
	if len(spec.PathArgs) != 0 || spec.HasQuery || !spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want a body-only shape", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"name":"org/acme"}`)
	if got.Method != http.MethodPost || got.Path != "/api/tags" {
		t.Errorf("request = %s %s, want POST /api/tags", got.Method, got.Path)
	}
	if got.Body != `{"Name":"org/acme"}` {
		t.Errorf("request body = %s, want only the body field, rendered on the wire as apigen's own \"Name\" tag", got.Body)
	}
}

func TestUnit_GenerateHandler_PathAndBody_CompilesAndCallsWithTheRightArguments(t *testing.T) {
	t.Parallel()
	// AllowListUpdate: PUT /allowlist/{id}, one path argument plus a body —
	// the shape this project's own plan illustrates directly.
	structs, spec := buildRealHandlerSpec(t, "allowlist", "AllowListUpdate")
	if len(spec.PathArgs) != 1 || spec.HasQuery || !spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want exactly one path argument and a body, no query", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"id":7,"entries":["a.b.c"]}`)
	if got.Path != "/api/allowlist/7" {
		t.Errorf("request path = %q, want /api/allowlist/7: the path argument was mis-bound", got.Path)
	}
	if got.Method != http.MethodPut {
		t.Errorf("request method = %q, want PUT", got.Method)
	}
	if !strings.Contains(got.Body, `"Entries":["a.b.c"]`) {
		t.Errorf("request body = %s, want the body field only, not the path id", got.Body)
	}
	if strings.Contains(got.Body, "7") {
		t.Errorf("request body = %s, leaked the path argument's value into the body", got.Body)
	}
}

func TestUnit_GenerateHandler_PathAndQuery_RoutesOptionalQueryIntoParamsStruct(t *testing.T) {
	t.Parallel()
	// RegistryInspect: GET /registries/{id}?endpointId=..., the pilot
	// domain's own hand-written example of this exact shape.
	structs, spec := buildRealHandlerSpec(t, "registries", "RegistryInspect")
	if len(spec.PathArgs) != 1 || !spec.HasQuery || spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want one path argument and a query struct, no body", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"id":42,"endpointId":9}`)
	if got.Path != "/api/registries/42" {
		t.Errorf("request path = %q, want /api/registries/42: the path argument was mis-bound", got.Path)
	}
	if got.Method != http.MethodGet {
		t.Errorf("request method = %q, want GET", got.Method)
	}
	if got.Query != "endpointId=9" {
		t.Errorf("request query = %q, want endpointId=9: the query field never reached the params struct", got.Query)
	}
}

func TestUnit_GenerateHandler_ParamsAndBody_RoutesQueryAndBodySeparately(t *testing.T) {
	t.Parallel()
	// EdgeStackCreateHelmRepository: POST /edge_stacks/create/helmRepo?dryrun=...,
	// a query parameter and a body with no path at all.
	structs, spec := buildRealHandlerSpec(t, "edge_stacks", "EdgeStackCreateHelmRepository")
	if len(spec.PathArgs) != 0 || !spec.HasQuery || !spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want no path, a query struct and a body", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"name":"my-stack","dryrun":"true"}`)
	if got.Method != http.MethodPost || got.Path != "/api/edge_stacks/create/helmRepo" {
		t.Errorf("request = %s %s, want POST /api/edge_stacks/create/helmRepo", got.Method, got.Path)
	}
	if !strings.Contains(got.Body, `"Name":"my-stack"`) {
		t.Errorf("request body = %s, want the body field", got.Body)
	}
	if strings.Contains(got.Body, "dryrun") || strings.Contains(got.Body, "true") {
		t.Errorf("request body = %s, leaked the query parameter into the body", got.Body)
	}
}

func TestUnit_GenerateHandler_ParamsOnly_NoPathNoBody(t *testing.T) {
	t.Parallel()
	// AddonList: GET /addons?view=..., query only.
	structs, spec := buildRealHandlerSpec(t, "addons", "AddonList")
	if len(spec.PathArgs) != 0 || !spec.HasQuery || spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want a query-only shape", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"view":"switcher"}`)
	if got.Method != http.MethodGet || got.Path != "/api/addons" {
		t.Errorf("request = %s %s, want GET /api/addons", got.Method, got.Path)
	}
	if got.Query != "view=switcher" {
		t.Errorf("request query = %q, want view=switcher: the query field never reached the params struct", got.Query)
	}
}

func TestUnit_GenerateHandler_None_IgnoresInputAndCallsWithOnlyContext(t *testing.T) {
	t.Parallel()
	// AutoUpdateList: GET /auto_updates, no path, query or body at all.
	structs, spec := buildRealHandlerSpec(t, "auto_updates", "AutoUpdateList")
	if len(spec.PathArgs) != 0 || spec.HasQuery || spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want no path, query or body", spec)
	}
	if len(structs) != 0 {
		t.Fatalf("structs = %+v, want none: a parameterless operation needs no Input struct", structs)
	}

	got := executeGeneratedHandler(t, structs, spec, `{}`)
	if got.Method != http.MethodGet || got.Path != "/api/auto_updates" {
		t.Errorf("request = %s %s, want GET /api/auto_updates", got.Method, got.Path)
	}
}

// --- the standing-warning trio: two or three same-typed path arguments,
// where only a real request — not compilation, not a human reading the
// generated text — shows which one reached the URL.

// TestUnit_GenerateHandler_PathAndPath_DoesNotSwapTwoIntArguments is a
// one-case table: UserRemoveAPIKey (DELETE /users/{id}/tokens/{keyID}) has
// two interchangeable ints; id=101 and keyID=202 are chosen distinctly so a
// swap is unmistakable in the resulting path.
//
// This test used to exercise DockerContainerGpusInspect
// (GET /docker/{environmentId}/containers/{containerId}/gpus), the
// original "two interchangeable ints" example. P3.3 task 7 gave
// containerId its own Go type ("string": see pathParamTypeOverrides in
// fields.go, a Docker hex container ID that was never really an integer
// on the wire), which now makes DockerContainerGpusInspect refuse at
// buildHandlerSpec — see
// TestUnit_GenerateHandler_IdentifierTypeOverrides_RefuseRatherThanBindAMismatchedPathArgument
// below for that proof. UserRemoveAPIKey is a genuine, unrelated
// two-int-path-argument operation that keeps this test's own point (a
// generic handler must not silently swap two same-typed positional
// arguments) alive on a shape the fix does not touch.
func TestUnit_GenerateHandler_PathAndPath_DoesNotSwapTwoIntArguments(t *testing.T) {
	for _, tc := range []struct {
		name        string
		domain      string
		operationID string
		input       string
		wantPath    string
	}{
		{"UserRemoveAPIKey", "users", "UserRemoveAPIKey", `{"id":101,"keyID":202}`, "/api/users/101/tokens/202"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			structs, spec := buildRealHandlerSpec(t, tc.domain, tc.operationID)
			if len(spec.PathArgs) != 2 || spec.HasQuery || spec.HasBody {
				t.Fatalf("handlerSpec = %+v, want exactly two path arguments, no query/body", spec)
			}

			got := executeGeneratedHandler(t, structs, spec, tc.input)
			if got.Path != tc.wantPath {
				t.Errorf("request path = %q, want %q: the two path arguments were swapped", got.Path, tc.wantPath)
			}
		})
	}
}

// TestUnit_GenerateHandler_PathPathAndParams_BindsBothPathArgumentsAndTheQuery
// is a one-case table: EndpointEdgeStackInspect
// (GET /endpoints/{id}/edge/stacks/{stackId}?version=...).
//
// This test used to exercise ContainerImageStatus
// (GET /docker/{environmentId}/containers/{containerId}/image_status?refresh=...),
// the shape this project's own plan named as the priority case. P3.3
// task 7 gave containerId its own Go type ("string": see
// pathParamTypeOverrides in fields.go), which now makes ContainerImageStatus
// refuse at buildHandlerSpec — see
// TestUnit_GenerateHandler_IdentifierTypeOverrides_RefuseRatherThanBindAMismatchedPathArgument
// below for that proof. EndpointEdgeStackInspect is a genuine, unrelated
// two-int-path-argument-plus-query operation that keeps this test's own
// point (path arguments and the query struct must each reach their own
// place, not each other's) alive on a shape the fix does not touch.
func TestUnit_GenerateHandler_PathPathAndParams_BindsBothPathArgumentsAndTheQuery(t *testing.T) {
	for _, tc := range []struct {
		name        string
		domain      string
		operationID string
		input       string
		wantPath    string
		wantQuery   string
	}{
		{"EndpointEdgeStackInspect", "endpoints", "EndpointEdgeStackInspect", `{"id":11,"stackId":22,"version":3}`, "/api/endpoints/11/edge/stacks/22", "version=3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			structs, spec := buildRealHandlerSpec(t, tc.domain, tc.operationID)
			if len(spec.PathArgs) != 2 || !spec.HasQuery || spec.HasBody {
				t.Fatalf("handlerSpec = %+v, want two path arguments and a query struct, no body", spec)
			}

			got := executeGeneratedHandler(t, structs, spec, tc.input)
			if got.Path != tc.wantPath {
				t.Errorf("request path = %q, want %q: a path argument was mis-bound", got.Path, tc.wantPath)
			}
			if got.Query != tc.wantQuery {
				t.Errorf("request query = %q, want %s: the query field never reached the params struct", got.Query, tc.wantQuery)
			}
		})
	}
}

// TestUnit_GenerateHandler_IdentifierTypeOverrides_RefuseRatherThanBindAMismatchedPathArgument
// is P3.3 task 7's own proof that fixing containerId/serviceId's published
// type does not silently produce a broken handler.
//
// The vendored specification's own generated client (internal/portainer/gen,
// oapi-codegen run over the identical "integer" declaration
// pathParamTypeOverrides corrects) still takes containerId/serviceId as a Go
// `int` — oapi-codegen has no way to know the declaration is wrong either.
// So the moment this generator publishes "string" for one of these four
// fields (fixing the schema a caller sees) it also creates a mismatch this
// generator did not have before (a `string` Input field bound to an `int`
// client parameter), and buildHandlerSpec's own goTypeMatchesReflectType
// check (handler.go) is what catches it: every one of these four operations
// now refuses generation rather than emit a handler that would try to
// json.Marshal a hex container ID or a Swarm service ID into an int
// argument, silently truncating or failing at the first real call.
//
// This is not a defect the type fix introduced — it is the type fix
// surfacing a defect that was always there, one level down, in Portainer's
// own generated client: no automatically generated handler can call these
// four operations correctly, whichever type this generator publishes for
// them, because the underlying client method's signature cannot carry the
// real identifier either. A domain author scaffolding docker/endpoints in a
// later wave must hand-write these four handlers — building the HTTP
// request directly with the real string identifier, the same way the four
// existing hand-written pilot actions (EcrDeleteTags, RegistryConfigure,
// RepositoryTagsDelete, SystemUpgrade) already bypass generation for their
// own reasons. See docs/api-divergences.md for the account of why this must
// never be "solved" by passing an integer that merely happens to validate
// (docker.service_image_status's own probe-container cheat) instead.
func TestUnit_GenerateHandler_IdentifierTypeOverrides_RefuseRatherThanBindAMismatchedPathArgument(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{
		"DockerContainerGpusInspect",
		"ContainerImageStatus",
		"SnapshotContainerInspect",
		"ServiceImageStatus",
	} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, doc, res := realOperation(t, operationID)
			structName := inputStructName(op.OperationID)
			var nested []structSpec
			fields, pathOrder, err := assembleOperationFields(op, res, doc, structName, &nested)
			if err != nil {
				t.Fatalf("assembleOperationFields(%s) error = %v", operationID, err)
			}

			_, err = buildHandlerSpec("docker", op, fields, pathOrder, nested, structName, "")
			if err == nil {
				t.Fatalf("buildHandlerSpec(%s) = nil error, want a refusal: its identifier path parameter now publishes \"string\", "+
					"but the generated client (internal/portainer/gen) still declares it \"int\"", operationID)
			}
			if !strings.Contains(err.Error(), "is Go type string in the generated Input, but the generated client's positional argument is int") {
				t.Errorf("buildHandlerSpec(%s) error = %q, want it to name the string-vs-int path-argument mismatch", operationID, err)
			}
		})
	}
}

func TestUnit_GenerateHandler_PathPathAndBody_DoesNotSwapTwoIntArguments(t *testing.T) {
	t.Parallel()
	// NamespacesAccessUpdate: PUT /endpoints/{id}/pools/{rpn}/access, two
	// interchangeable ints plus a body, and no success response body.
	structs, spec := buildRealHandlerSpec(t, "endpoints", "NamespacesAccessUpdate")
	if len(spec.PathArgs) != 2 || spec.HasQuery || !spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want exactly two path arguments and a body, no query", spec)
	}
	if spec.Response.SuccessField != "" {
		t.Fatalf("Response.SuccessField = %q, want \"\"", spec.Response.SuccessField)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"id":3,"rpn":9,"teamsToAdd":[5]}`)
	want := "/api/endpoints/3/pools/9/access"
	if got.Path != want {
		t.Errorf("request path = %q, want %q: the two path arguments were swapped", got.Path, want)
	}
	if !strings.Contains(got.Body, `"TeamsToAdd":[5]`) {
		t.Errorf("request body = %s, want the body field, not the path arguments", got.Body)
	}
}

func TestUnit_GenerateHandler_PathPathAndPath_BindsThreeMixedTypeArgumentsInOrder(t *testing.T) {
	t.Parallel()
	// GetKubernetesConfigMap: GET /kubernetes/{id}/namespaces/{namespace}/configmaps/{configmap} —
	// three path arguments, two of them strings with no type-level
	// distinction the compiler could catch if swapped.
	structs, spec := buildRealHandlerSpec(t, "kubernetes", "GetKubernetesConfigMap")
	if len(spec.PathArgs) != 3 || spec.HasQuery || spec.HasBody {
		t.Fatalf("handlerSpec = %+v, want exactly three path arguments, no query/body", spec)
	}

	got := executeGeneratedHandler(t, structs, spec, `{"id":5,"namespace":"prod","configmap":"app-config"}`)
	want := "/api/kubernetes/5/namespaces/prod/configmaps/app-config"
	if got.Path != want {
		t.Errorf("request path = %q, want %q: one of the three path arguments was mis-bound", got.Path, want)
	}
}

// --- refusals ---

func TestUnit_ResponseInfoFor_TwoSuccessBodies_IsRefused(t *testing.T) {
	t.Parallel()
	// AddonInstall is the one operation across the entire vendored client
	// whose response declares two success bodies (JSON200 and JSON201,
	// both *AddonsAddonLifecycleResponse) — checked directly against the
	// real generated client via reflection, not a fabricated response type,
	// since the whole point is that this is the one real ambiguous case.
	_, err := responseInfoFor("AddonInstall")
	if err == nil {
		t.Fatal("responseInfoFor(\"AddonInstall\") = nil error, want a refusal: two success bodies, and this generator must not guess which one a caller wants")
	}
	if !strings.Contains(err.Error(), "two success bodies") && !strings.Contains(err.Error(), "declares 2 success bodies") {
		t.Errorf("error = %q, want it to say the response declares more than one success body", err)
	}
}

func TestUnit_ResponseInfoFor_UnknownOperation_IsRefusedNotPanicked(t *testing.T) {
	t.Parallel()
	_, err := responseInfoFor("ThisOperationDoesNotExist")
	if err == nil {
		t.Fatal("responseInfoFor() = nil error, want a refusal for an operationId with no generated client method")
	}
}

func TestUnit_BuildHandlerSpec_FieldWithNoOrigin_IsRefused(t *testing.T) {
	t.Parallel()
	// A field with no Origin set is an internal-error case (every field
	// assembleOperationFields returns is always path, query or body) —
	// guarded here because a silent default would misroute the field to
	// neither the path arguments nor the query struct nor the body,
	// dropping it without a trace.
	op := operation{OperationID: "TagDelete", Method: "DELETE", Path: "/tags/{id}"}
	fields := []fieldSpec{{GoName: "ID", GoType: "int", JSONName: "id", Required: true}}
	_, err := buildHandlerSpec("tags", op, fields, nil, nil, "tagDeleteInput", "")
	if err == nil {
		t.Fatal("buildHandlerSpec() = nil error, want a refusal: the field carries no Origin")
	}
	if !strings.Contains(err.Error(), "Origin") {
		t.Errorf("error = %q, want it to mention the missing Origin", err)
	}
}

func TestUnit_BuildHandlerSpec_PathOrderNamesAMissingField_IsRefused(t *testing.T) {
	t.Parallel()
	// pathOrder naming a field absent from fields would be an internal
	// contradiction between assembleOperationFields' two return values —
	// refused rather than indexing out of range or silently skipping the
	// argument.
	op := operation{OperationID: "TagDelete", Method: "DELETE", Path: "/tags/{id}"}
	_, err := buildHandlerSpec("tags", op, nil, []string{"id"}, nil, "tagDeleteInput", "")
	if err == nil {
		t.Fatal("buildHandlerSpec() = nil error, want a refusal: pathOrder names a field with no match in fields")
	}
}

func TestUnit_BuildHandlerSpec_PathArgumentTypeMismatch_IsRefused(t *testing.T) {
	t.Parallel()
	// TagDelete's real generated client method is
	// TagDeleteWithResponse(ctx, id int, ...): its one positional path
	// argument is Go type int. A fieldSpec claiming GoType "string" for
	// that same JSON name is exactly the class of defect a generator (not
	// a human transcribing by hand) produces — fields.go's own path-typing
	// disagreeing with the generated client — and pathArgTypesFor's
	// reflection cross-check must refuse it rather than emit a call
	// go/types would still happily accept (int and string are each
	// internally consistent, just wrong against each other).
	op := operation{OperationID: "TagDelete", Method: "DELETE", Path: "/tags/{id}"}
	fields := []fieldSpec{{GoName: "ID", GoType: "string", JSONName: "id", Required: true, Origin: originPath}}
	_, err := buildHandlerSpec("tags", op, fields, []string{"id"}, nil, "tagDeleteInput", "")
	if err == nil {
		t.Fatal("buildHandlerSpec() = nil error, want a refusal: the claimed path argument type (string) disagrees with the generated client's (int)")
	}
	if !strings.Contains(err.Error(), "mismatched type") {
		t.Errorf("error = %q, want it to say the path argument type is mismatched", err)
	}
}

// --- wire-width safety: the generic JSON round trip this file's package doc
// describes is only correct where recasing a wire tag is the only thing
// that differs between this generator's own Go type and the generated
// client's. Where the generated client's own type is narrower — a value
// this generator's own type can hold that the client's cannot — the round
// trip can silently truncate or fail outright, which is exactly what
// checkWireWidth exists to refuse before a handler is ever generated for
// it, rather than leave 44 future domain authors to notice on their own.

func TestUnit_CheckWireWidth_RegistriesConfigure_RefusesNarrowedTLSCertificateBytes(t *testing.T) {
	t.Parallel()
	// registries.registryConfigurePayload's TLSCACertFile, TLSCertFile and
	// TLSKeyFile are generated client-side as *[]int32 against a JSON
	// Schema "integer" property (this generator's own []int) — the real,
	// previously-investigated example (registries.go's own toTLSFileBytes
	// exists by hand specifically because of this; see
	// plan/carry-forward.md) of what this check exists to catch. Run
	// through the real generation path against the real spec, not a
	// fabricated fixture, so this is a verified refusal of the actual known
	// case, not merely a plausible one.
	op, doc, res := realOperation(t, "RegistryConfigure")
	var nested []structSpec
	fields, pathOrder, err := assembleOperationFields(op, res, doc, "registryConfigureInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	_, err = buildHandlerSpec("registries", op, fields, pathOrder, nested, "registryConfigureInput", "")
	if err == nil {
		t.Fatal("buildHandlerSpec(RegistryConfigure) = nil error, want a refusal: TLSCACertFile/TLSCertFile/TLSKeyFile narrow from []int to the generated client's []int32")
	}
	if !strings.Contains(err.Error(), "tlsCACertFile") && !strings.Contains(err.Error(), "tlsCertFile") && !strings.Contains(err.Error(), "tlsKeyFile") {
		t.Errorf("error = %q, want it to name one of the narrowed TLS certificate fields", err)
	}
	if !strings.Contains(err.Error(), "int vs int32") {
		t.Errorf("error = %q, want it to say which two types disagree (int vs int32)", err)
	}
}

// TestUnit_TypeWidthCompatible_DrawsTheBoundaryAtNarrowerConcreteWidth is the
// standing-warning pair: two cases differing *only* in the generated
// client's own integer width, so a check that always refuses (or always
// allows) integers would fail exactly one of the two, not both — the
// mutation this project's own standing warning calls out passing every
// prior verification step precisely because the fixture never varied the
// one thing under test.
func TestUnit_TypeWidthCompatible_DrawsTheBoundaryAtNarrowerConcreteWidth(t *testing.T) {
	t.Parallel()
	type narrow struct {
		V int32
	}
	type wide struct {
		V int64
	}
	narrowField, _ := reflect.TypeOf(narrow{}).FieldByName("V")
	wideField, _ := reflect.TypeOf(wide{}).FieldByName("V")

	if ok, detail := typeWidthCompatible("int", narrowField.Type, nil); ok {
		t.Errorf("typeWidthCompatible(\"int\", int32, nil) = true, want false: int32 is narrower than this generator's int and can silently truncate a value it holds")
	} else if !strings.Contains(detail, "int32") {
		t.Errorf("detail = %q, want it to mention int32", detail)
	}

	if ok, _ := typeWidthCompatible("int", wideField.Type, nil); !ok {
		t.Error("typeWidthCompatible(\"int\", int64, nil) = false, want true: int64 cannot silently narrow anything this generator's int can hold")
	}
}

func TestUnit_TypeWidthCompatible_PointerOptionalityIsNotWidth(t *testing.T) {
	t.Parallel()
	// *int against "integer" (this generator's own optional-field pointer
	// convention) must not be confused with the int32-narrowing case above:
	// pointer-ness is "not provided" vs "provided", not a range restriction,
	// on both this generator's side (a leading "*" in ourGoType) and the
	// generated client's (apigenType itself being a Pointer).
	type withPointerInt struct {
		V *int
	}
	f, _ := reflect.TypeOf(withPointerInt{}).FieldByName("V")
	if ok, detail := typeWidthCompatible("*int", f.Type, nil); !ok {
		t.Errorf("typeWidthCompatible(\"*int\", *int, nil) = false (%s), want true: pointer-ness on both sides is optionality, not width", detail)
	}
}

// --- the escape hatch ---

func TestUnit_ScanHandOverrides_DetectsOperationIDAndFuncNameIndependently(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Mirrors system.go's own real deviation: an ActionSpec's OperationID
	// can be declared by hand under a shorter, hand-chosen function name
	// than this generator's mechanical rule would mint (SystemNodesCount ->
	// systemNodes, not systemNodesCount) — scanHandOverrides must catch the
	// operation by its OperationID even though no function named
	// systemNodesCount exists, and must independently catch a plain
	// function-name collision (helperFunc below) that names no OperationID
	// at all.
	src := `package system

import "github.com/jmrplens/portainer-mcp/internal/toolutil"

func Specs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		{Name: "system.nodes_count", Domain: "system", OperationID: "SystemNodesCount", Handler: systemNodes},
	}
}

func systemNodes() {}

func helperFunc() {}
`
	if err := os.WriteFile(filepath.Join(dir, "system.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	overrides, err := scanHandOverrides(dir)
	if err != nil {
		t.Fatalf("scanHandOverrides() error = %v", err)
	}
	if !overrides.operationIDs["SystemNodesCount"] {
		t.Error("scanHandOverrides() did not find SystemNodesCount's OperationID, even though system.go declares it by hand")
	}
	if !overrides.funcNames["helperFunc"] {
		t.Error("scanHandOverrides() did not find the plain function helperFunc")
	}

	overriddenOp := operation{OperationID: "SystemNodesCount"}
	if reason, ok := overrides.overrideReason(overriddenOp); !ok {
		t.Error("overrideReason() = false for SystemNodesCount, want true: its OperationID is already declared by hand under a different function name")
	} else if !strings.Contains(reason, "ActionSpec") {
		t.Errorf("overrideReason() = %q, want it to say an ActionSpec already covers this operationId", reason)
	}

	collidingOp := operation{OperationID: "HelperFunc"} // handlerFuncName("HelperFunc") == "helperFunc"
	if reason, ok := overrides.overrideReason(collidingOp); !ok {
		t.Error("overrideReason() = false for an operation whose mechanical function name is already taken, want true")
	} else if !strings.Contains(reason, "helperFunc") {
		t.Errorf("overrideReason() = %q, want it to name the colliding function", reason)
	}

	freshOp := operation{OperationID: "SystemInfo"}
	if _, ok := overrides.overrideReason(freshOp); ok {
		t.Error("overrideReason() = true for SystemInfo, want false: nothing in the fixture covers it")
	}
}

func TestUnit_ScanHandOverrides_IgnoresGeneratedFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A previous run's own actions.gen.go must never be read back as a
	// hand-written override — that would make every domain's second run
	// treat its own first run's output as "already covered", generating
	// nothing on every run after the first.
	genSrc := `// Code generated by cmd/gen_action_inputs from test-spec.json. DO NOT EDIT.

package tags

func tagList() {}
`
	if err := os.WriteFile(filepath.Join(dir, "actions.gen.go"), []byte(genSrc), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	overrides, err := scanHandOverrides(dir)
	if err != nil {
		t.Fatalf("scanHandOverrides() error = %v", err)
	}
	if overrides.funcNames["tagList"] {
		t.Error("scanHandOverrides() read tagList from actions.gen.go; generated files must be excluded")
	}
}

func TestUnit_HandlerFuncName_LowersOnlyTheFirstRune(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ operationID, want string }{
		{"TagCreate", "tagCreate"},
		{"EcrDeleteRepository", "ecrDeleteRepository"},
		{"SystemNodesCount", "systemNodesCount"},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			if got := handlerFuncName(tc.operationID); got != tc.want {
				t.Errorf("handlerFuncName(%q) = %q, want %q", tc.operationID, got, tc.want)
			}
		})
	}
}

func TestUnit_RenderActionsFile_OmitsAPIGenImportWhenNoHandlerNeedsIt(t *testing.T) {
	t.Parallel()
	// A domain whose only generated handler is a "none"/"path"-only shape
	// (system.info, tags.delete) never references apigen.* in its body:
	// including the import anyway would fail to compile ("imported and not
	// used"), a defect go/format's own parse step cannot catch since an
	// unused import is a go/types-level error, not a syntax one.
	spec := handlerSpec{Domain: "tags", OperationID: "TagDelete", FuncName: "tagDelete", InputStruct: "tagDeleteInput", PathArgs: []pathArg{{GoName: "ID", GoType: "int"}}}
	src, err := renderActionsFile("tags", "test-spec.json", []handlerSpec{spec}, nil, false)
	if err != nil {
		t.Fatalf("renderActionsFile() error = %v", err)
	}
	// mustParseOnly, not mustCompile: mustCompile's importer.Default() cannot
	// resolve this module's own internal/* import paths at all (it resolves
	// against GOPATH/installed export data, not this module's source tree),
	// so it would fail on any import list this generator actually emits,
	// regardless of correctness. The corresponding real compile-and-run
	// proof for this exact operation already exists —
	// TestUnit_GenerateHandler_Path_BindsPathArgumentAndAcknowledgesNoBody
	// builds and executes TagDelete's generated handler in a real `go run`
	// process — so this test only needs to check the import list itself.
	mustParseOnly(t, src)
	if strings.Contains(string(src), `apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"`) {
		t.Errorf("generated source imports apigen even though no handler needs it:\n%s", src)
	}
}

func TestUnit_RenderActionsFile_IncludesAPIGenImportWhenABodyHandlerNeedsIt(t *testing.T) {
	t.Parallel()
	spec := handlerSpec{Domain: "tags", OperationID: "TagCreate", FuncName: "tagCreate", HasBody: true, Response: responseInfo{SuccessField: "JSON200"}}
	src, err := renderActionsFile("tags", "test-spec.json", []handlerSpec{spec}, nil, false)
	if err != nil {
		t.Fatalf("renderActionsFile() error = %v", err)
	}
	// See the sibling test above for why this is mustParseOnly, not
	// mustCompile: TestUnit_GenerateHandler_Body_SendsOnlyBodyFields already
	// builds and executes this exact operation's generated handler for real.
	mustParseOnly(t, src)
	if !strings.Contains(string(src), `apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"`) {
		t.Errorf("generated source does not import apigen even though TagCreate's handler needs apigen.TagCreateJSONRequestBody:\n%s", src)
	}
}

func TestUnit_PathOrder_MatchesDeclarationOrderNotAlphabeticalOrder(t *testing.T) {
	t.Parallel()
	// GetKubernetesConfigMap's own three path parameters, id/namespace/configmap,
	// are not in alphabetical order (configmap/id/namespace would be) — this
	// is precisely why pathOrder is returned separately from the
	// alphabetically-sorted fields, rather than derived by filtering them.
	op, doc, res := realOperation(t, "GetKubernetesConfigMap")
	var nested []structSpec
	_, pathOrder, err := assembleOperationFields(op, res, doc, "getKubernetesConfigMapInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	got := strings.Join(pathOrder, ",")
	want := "id,namespace,configmap"
	if got != want {
		t.Errorf("pathOrder = %q, want %q (URL declaration order, not alphabetical)", got, want)
	}
}

// pathTemplateParams returns the path parameter names in the order the path
// template itself declares them, e.g. "/endpoints/{id}/dockerhub/{registryId}"
// -> ["id", "registryId"]. Derived from the path string, entirely
// independently of assembleOperationFields, which reads op.Parameters instead.
func pathTemplateParams(path string) []string {
	var out []string
	for {
		open := strings.Index(path, "{")
		if open < 0 {
			return out
		}
		closeIdx := strings.Index(path[open:], "}")
		if closeIdx < 0 {
			return out
		}
		out = append(out, path[open+1:open+closeIdx])
		path = path[open+closeIdx:]
	}
}

// TestUnit_AssembleOperationFields_PathOrderMatchesTheRouteAcrossTheWholeSpec
// pins the invariant every generated handler silently depends on: pathOrder
// drives buildHandlerSpec's positional arguments to the generated client, so
// if it ever disagreed with the order the route actually declares its
// segments in, two same-typed identifiers would be passed to the wrong
// positions — a request to the wrong resource, with no type error and no
// runtime error, only the wrong answer.
//
// assembleOperationFields builds pathOrder by walking op.Parameters in
// declaration order, which is a different source from the path template this
// test parses. They agree for all 441 operations today; nothing enforced that
// until now.
func TestUnit_AssembleOperationFields_PathOrderMatchesTheRouteAcrossTheWholeSpec(t *testing.T) {
	t.Parallel()
	doc, paths, err := loadDocument("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}
	res := &resolver{doc: doc}

	checked := 0
	for _, ops := range byTag {
		for _, op := range ops {
			var nested []structSpec
			_, pathOrder, err := assembleOperationFields(op, res, doc, "probe", &nested)
			if err != nil {
				// A refusal is this generator's documented behaviour for the
				// shapes it will not guess at; it says nothing about ordering.
				continue
			}
			want := pathTemplateParams(op.Path)
			if strings.Join(pathOrder, ",") != strings.Join(want, ",") {
				t.Errorf("%s %s (operationId %s): pathOrder = %v, want %v (the order the route declares)",
					op.Method, op.Path, op.OperationID, pathOrder, want)
			}
			checked++
		}
	}
	if checked < 400 {
		t.Errorf("only %d operations were checked; the assertion is not covering the specification", checked)
	}
}
