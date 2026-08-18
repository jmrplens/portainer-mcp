package stacks

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// capturedRequest is what a server saw: the parsed multipart form (or, for a
// JSON route, the raw body bytes) plus the header and query values that
// decide whether it can be read at all.
type capturedRequest struct {
	method      string
	path        string
	rawQuery    string
	contentType string
	form        *multipart.Form
	// body is the request body as read off the wire, populated only for a
	// request that is not multipart. The two are exclusive on purpose: see
	// capturingClient for why a multipart body is never buffered here.
	body []byte
}

// capturingClient answers every request with body and records the request
// itself, parsed the way net/http parses a multipart upload.
//
// Deliberately not clientFor from stacks_test.go: that helper discards the
// request, which is exactly what this file needs to inspect. The multipart
// parse is done inside the server because http.Request.MultipartReader
// consumes the body — a test that kept the raw bytes and parsed them
// afterwards would be asserting against its own copy rather than against what
// a server can actually read off the wire. A body that is not multipart is
// simply read whole into captured.body: StackMigrate sends JSON, and the
// assertions on it are about which key carries which value, not about whether
// a streaming parser can find them.
func capturingClient(t *testing.T, status int, body []byte) (*portainer.Client, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.rawQuery = r.URL.RawQuery
		captured.contentType = r.Header.Get("Content-Type")

		if _, params, err := mime.ParseMediaType(captured.contentType); err == nil && params["boundary"] != "" {
			if form, err := multipart.NewReader(r.Body, params["boundary"]).ReadForm(1 << 20); err == nil {
				captured.form = form
				t.Cleanup(func() { _ = form.RemoveAll() })
			}
		} else {
			captured.body, _ = io.ReadAll(r.Body)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)

	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c, captured
}

// partNames renders a form's text part names for a failure message, sorted
// so the message does not depend on Go's map iteration order.
func partNames(form *multipart.Form) []string {
	names := make([]string, 0, len(form.Value))
	for name := range form.Value {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// filePartContent returns the bytes of the one file part named name.
func filePartContent(t *testing.T, form *multipart.Form, name string) string {
	t.Helper()
	headers, ok := form.File[name]
	if !ok || len(headers) != 1 {
		t.Fatalf("%s is not a single file part; file parts sent: %v", name, form.File)
	}
	opened, err := headers[0].Open()
	if err != nil {
		t.Fatalf("open the %s part: %v", name, err)
	}
	defer func() { _ = opened.Close() }()
	content := make([]byte, headers[0].Size)
	if _, err := opened.Read(content); err != nil {
		t.Fatalf("read the %s part: %v", name, err)
	}
	return string(content)
}

// createdStack is the 200 body both routes answer with. It reuses
// gitBackedStack so the fixture — and its live credential — is identical to
// the one the twenty-two generated handlers are checked against.
func createdStack(t *testing.T) []byte {
	t.Helper()
	return mustMarshal(t, gitBackedStack())
}

const (
	fullStandaloneFileInput = `{
		"endpointId": 7,
		"name": "nginx-stack",
		"env": "[{\"name\":\"PORT\",\"value\":\"8080\"}]",
		"file": "services:\n  web:\n    image: nginx\n"
	}`

	// minimalStandaloneFileInput carries only what the vendored multipart
	// schema lists required (Name) plus the required query parameter. It is
	// deliberately NOT "what the server needs": file is published optional
	// because the document says so, and this input is what exercises that.
	minimalStandaloneFileInput = `{"endpointId":7,"name":"nginx-stack"}`

	fullSwarmFileInput = `{
		"endpointId": 7,
		"name": "nginx-stack",
		"swarmId": "jpofkc0i9uo9wtx1zesuk649w",
		"env": "[{\"name\":\"PORT\",\"value\":\"8080\"}]",
		"file": "services:\n  web:\n    image: nginx\n"
	}`

	// minimalSwarmFileInput carries the query parameter and nothing else:
	// POST /stacks/create/swarm/file declares no required body fields at
	// all, so this is a request ValidateInput accepts.
	minimalSwarmFileInput = `{"endpointId":7}`
)

// TestUnit_FileCreateRequest_IsAMultipartBodyOnTheRouteWithTheQueryParameter
// pins the wire format, which nothing else in this domain touches: the
// twenty-two generated handlers all send JSON, and these two routes accept
// nothing but multipart/form-data.
//
// The query is asserted alongside the body because endpointId is the one
// field of these Inputs that must NOT become a part. It is a query parameter
// on both routes, the handlers pass it through apigen's Params struct, and a
// version that wrote it into the form instead would still produce a
// parseable body and a 200 from any test server.
func TestUnit_FileCreateRequest_IsAMultipartBodyOnTheRouteWithTheQueryParameter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		input  string
		path   string
	}{
		{
			name:   "standalone",
			action: "stacks.create_docker_standalone_file",
			input:  fullStandaloneFileInput,
			path:   "/api/stacks/create/standalone/file",
		},
		{
			name:   "swarm",
			action: "stacks.create_docker_swarm_file",
			input:  fullSwarmFileInput,
			path:   "/api/stacks/create/swarm/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, createdStack(t))

			if _, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("handler error = %v", err)
			}

			if captured.method != http.MethodPost {
				t.Errorf("method = %s, want POST", captured.method)
			}
			if captured.path != tt.path {
				t.Errorf("path = %s, want %s", captured.path, tt.path)
			}
			if captured.rawQuery != "endpointId=7" {
				t.Errorf("query = %q, want %q: endpointId is a query parameter on this route, not a form part", captured.rawQuery, "endpointId=7")
			}
			mediaType, params, err := mime.ParseMediaType(captured.contentType)
			if err != nil {
				t.Fatalf("parse the request's content type %q: %v", captured.contentType, err)
			}
			if mediaType != "multipart/form-data" {
				t.Errorf("media type = %q, want multipart/form-data", mediaType)
			}
			if params["boundary"] == "" {
				t.Errorf("content type = %q, want a boundary parameter; without one the server cannot split the body", captured.contentType)
			}
			if captured.form == nil {
				t.Fatal("the server could not parse the body as a multipart form")
			}
			if got, sent := captured.form.Value["endpointId"]; sent {
				t.Errorf("endpointId was written into the body as %q; it belongs in the query alone", got)
			}
		})
	}
}

// TestUnit_FileCreateRequest_CarriesEveryFieldUnderTheSpecifiedPartName pins
// the mapping between the Input a model fills and the parts Portainer reads.
//
// Two things here can be got wrong silently. The part names are the vendored
// schema's own, which on these two routes are inconsistent with each other —
// Name, Env and SwarmID capitalised, file not — and multipart part names are
// matched literally rather than case-folded like a header, so "name" or
// "File" would reach a server that then reports the field missing. And Env is
// a JSON-encoded string on these routes where the four JSON siblings take a
// real array of name/value objects: re-marshalling it here would wrap the
// caller's document in a second layer of quoting, which the assertion on the
// exact part body is what catches.
func TestUnit_FileCreateRequest_CarriesEveryFieldUnderTheSpecifiedPartName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		input  string
		want   map[string]string
	}{
		{
			name:   "standalone",
			action: "stacks.create_docker_standalone_file",
			input:  fullStandaloneFileInput,
			want: map[string]string{
				"Name": "nginx-stack",
				// Verbatim, not re-marshalled: this route types Env "string"
				// and Portainer unmarshals the part's own content.
				"Env": `[{"name":"PORT","value":"8080"}]`,
			},
		},
		{
			name:   "swarm",
			action: "stacks.create_docker_swarm_file",
			input:  fullSwarmFileInput,
			want: map[string]string{
				"Name":    "nginx-stack",
				"SwarmID": "jpofkc0i9uo9wtx1zesuk649w",
				"Env":     `[{"name":"PORT","value":"8080"}]`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, createdStack(t))

			if _, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if captured.form == nil {
				t.Fatal("the server could not parse the body as a multipart form")
			}

			for name, value := range tt.want {
				got, ok := captured.form.Value[name]
				if !ok {
					t.Errorf("%s is absent from the body; parts sent: %v", name, partNames(captured.form))
					continue
				}
				if len(got) != 1 || got[0] != value {
					t.Errorf("%s = %q, want [%q]", name, got, value)
				}
			}
		})
	}
}

// TestUnit_FileCreateRequest_SendsTheStackFileAsAFilePartNamedFile is the
// assertion that separates a working upload from one whose bytes are all
// present and still unreadable.
//
// Go's multipart reader — Portainer's own parser — files a part under
// Form.File only when its Content-Disposition names a file, so a "file" part
// written as ordinary text arrives at the server's lookup as nothing at all.
// The part name is asserted lowercase because that is what both vendored
// schemas declare, against three neighbouring parts that are capitalised.
func TestUnit_FileCreateRequest_SendsTheStackFileAsAFilePartNamedFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		input  string
	}{
		{name: "standalone", action: "stacks.create_docker_standalone_file", input: fullStandaloneFileInput},
		{name: "swarm", action: "stacks.create_docker_swarm_file", input: fullSwarmFileInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, createdStack(t))

			if _, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if captured.form == nil {
				t.Fatal("the server could not parse the body as a multipart form")
			}

			if _, isText := captured.form.Value["file"]; isText {
				t.Error("file was sent as a text part; a server's FormFile lookup finds nothing without a filename")
			}
			if _, capitalised := captured.form.File["File"]; capitalised {
				t.Error("the upload was sent as \"File\"; both vendored schemas name this part \"file\", and multipart names are matched literally")
			}
			headers, ok := captured.form.File["file"]
			if !ok || len(headers) != 1 {
				t.Fatalf("file is not a file part; file parts sent: %v", captured.form.File)
			}
			if headers[0].Filename != uploadFilename {
				t.Errorf("file filename = %q, want %q; an empty one makes the server report the field missing", headers[0].Filename, uploadFilename)
			}
			if want := "services:\n  web:\n    image: nginx\n"; filePartContent(t, captured.form, "file") != want {
				t.Errorf("file content = %q, want %q", filePartContent(t, captured.form, "file"), want)
			}
		})
	}
}

// TestUnit_FileCreateRequestWithoutTheOptionalFields_OmitsThoseParts pins
// that an unset optional field emits no part rather than an empty one.
//
// The difference is not cosmetic on these routes: Env carries a JSON
// document, and an empty part is a present field whose document fails to
// parse — a server-side error for a field the caller never mentioned — while
// an empty file part is an empty stack file rather than no stack file. On the
// Swarm route Name goes through the same optional path, because that route's
// schema declares no required fields at all.
func TestUnit_FileCreateRequestWithoutTheOptionalFields_OmitsThoseParts(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   string
		input    string
		omitted  []string
		required []string
	}{
		{
			name:     "standalone keeps its one required part",
			action:   "stacks.create_docker_standalone_file",
			input:    minimalStandaloneFileInput,
			omitted:  []string{"Env"},
			required: []string{"Name"},
		},
		{
			name:    "swarm has no required part at all",
			action:  "stacks.create_docker_swarm_file",
			input:   minimalSwarmFileInput,
			omitted: []string{"Name", "SwarmID", "Env"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, createdStack(t))

			if _, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("handler error = %v", err)
			}
			if captured.form == nil {
				t.Fatal("the server could not parse the body as a multipart form")
			}

			for _, name := range tt.omitted {
				if got, ok := captured.form.Value[name]; ok {
					t.Errorf("%s was not supplied but the body carries it as %q", name, got)
				}
			}
			// The part the route does require is still there: an
			// implementation that emitted nothing at all would pass every
			// assertion above.
			for _, name := range tt.required {
				if _, ok := captured.form.Value[name]; !ok {
					t.Errorf("required part %s is missing; parts sent: %v", name, partNames(captured.form))
				}
			}
			if _, ok := captured.form.File["file"]; ok {
				t.Error("no file was supplied but the body carries a file part; an empty upload is not the same as none")
			}
		})
	}
}

// TestUnit_FileCreateWithGitCredentialInResponse_ReturnsNoCredential is the
// discriminating redaction test for these two handlers, and it has to exist
// separately from every other redaction check in this domain.
//
// redaction_test.go's table drives each generated wrapper and each generated
// handler; neither of these operations has a generated handler, so no entry
// there covers this code path. cmd/gen_action_inputs's
// checkCredentialRedaction is a static AST scan over generated code, not over
// handlers.go. So nothing but this test stands between a hand-written handler
// that returns resp.JSON200 directly and a git password reaching a model —
// and the mutation was performed to confirm exactly that: replacing each
// redaction call with the raw response makes this test fail naming hunter2
// (verbatim output in this stage's task-4 report).
//
// It must be a unit test over a response constructed here. A live Portainer
// may blank the git password itself, in which case an end-to-end check cannot
// tell a redactor that works from one that does nothing: the field arrives
// empty either way.
//
// Username and GitCredentialID are asserted absent alongside Password because
// redactStack drops the whole Authentication sub-object; asserting only
// Password would pass a field-by-field redactor that kept the username, which
// no narrative in this domain promises a caller anyway.
func TestUnit_FileCreateWithGitCredentialInResponse_ReturnsNoCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		input  string
	}{
		{name: "standalone", action: "stacks.create_docker_standalone_file", input: fullStandaloneFileInput},
		{name: "swarm", action: "stacks.create_docker_swarm_file", input: fullSwarmFileInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := capturingClient(t, http.StatusOK, createdStack(t))

			out, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input))
			if err != nil {
				t.Fatalf("handler error = %v", err)
			}
			// The created stack and its repository URL are what the action
			// exists to return: a handler that returned nil, or dropped
			// GitConfig whole, would satisfy every credential assertion.
			assertNoCredential(t, tt.action, out, "nginx-stack", "https://git.example.com/team/app.git")
		})
	}
}

// TestUnit_FileCreateWhenPortainerRefuses_ReturnsTheServerError pins that
// these handlers run their response through toolutil.Check like their
// twenty-two generated neighbours. Without that call a refusal returns a nil
// JSON200 and no error, which reaches a model as a successful deployment that
// never happened.
func TestUnit_FileCreateWhenPortainerRefuses_ReturnsTheServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		action      string
		input       string
		operationID string
		status      int
		body        string
		want        string
	}{
		{
			name:        "standalone",
			action:      "stacks.create_docker_standalone_file",
			input:       minimalStandaloneFileInput,
			operationID: "StackCreateDockerStandaloneFile",
			status:      http.StatusBadRequest,
			body:        `{"message":"Invalid request payload"}`,
			want:        "Invalid request payload",
		},
		{
			name:        "swarm",
			action:      "stacks.create_docker_swarm_file",
			input:       minimalSwarmFileInput,
			operationID: "StackCreateDockerSwarmFile",
			status:      http.StatusConflict,
			body:        `{"message":"Stack name or webhook id is not unique"}`,
			want:        "Stack name or webhook id is not unique",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := capturingClient(t, tt.status, []byte(tt.body))

			out, err := find(t, tt.action)(context.Background(), c, json.RawMessage(tt.input))
			if err == nil {
				t.Fatalf("handler error = nil and returned %v, want the server's refusal", out)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to carry the server's own message", err)
			}
			if !strings.Contains(err.Error(), tt.operationID) {
				t.Errorf("error = %v, want it wrapped with the operation's own name", err)
			}
		})
	}
}

// TestUnit_FileCreateWithUnparseableInput_ReturnsAWrappedError pins the
// decode step's error context, the one thing a hand-written handler can get
// wrong that the generated ones cannot.
func TestUnit_FileCreateWithUnparseableInput_ReturnsAWrappedError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		action string
		want   string
	}{
		{name: "standalone", action: "stacks.create_docker_standalone_file", want: "StackCreateDockerStandaloneFile: parse input"},
		{name: "swarm", action: "stacks.create_docker_swarm_file", want: "StackCreateDockerSwarmFile: parse input"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := capturingClient(t, http.StatusOK, createdStack(t))

			_, err := find(t, tt.action)(context.Background(), c, json.RawMessage(`{"endpointId":"seven"}`))
			if err == nil {
				t.Fatal("handler error = nil, want a decode failure")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it wrapped with %q", err, tt.want)
			}
		})
	}
}

// TestUnit_HandWrittenSpecs_MatchTheDomainsGeneratedNeighbours pins the
// declarations themselves. handWrittenSpecs is written by hand, so nothing
// regenerates it and nothing else checks that it names the operation, the
// Input type and the redaction-bearing handler the rest of this domain
// assumes.
func TestUnit_HandWrittenSpecs_MatchTheDomainsGeneratedNeighbours(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		action      string
		operationID string
		input       any
	}{
		{
			name:        "standalone",
			action:      "stacks.create_docker_standalone_file",
			operationID: "StackCreateDockerStandaloneFile",
			input:       stackCreateDockerStandaloneFileInput{},
		},
		{
			name:        "swarm",
			action:      "stacks.create_docker_swarm_file",
			operationID: "StackCreateDockerSwarmFile",
			input:       stackCreateDockerSwarmFileInput{},
		},
		{
			name:        "migrate",
			action:      "stacks.migrate",
			operationID: "StackMigrate",
			input:       stackMigrateInput{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := findSpec(t, tt.action)

			if spec.OperationID != tt.operationID {
				t.Errorf("OperationID = %q, want %q", spec.OperationID, tt.operationID)
			}
			if spec.Domain != "stacks" {
				t.Errorf("Domain = %q, want stacks", spec.Domain)
			}
			if spec.Handler == nil {
				t.Fatal("Handler is nil")
			}
			if got, want := fmt.Sprintf("%T", spec.Input), fmt.Sprintf("%T", tt.input); got != want {
				t.Errorf("Input = %s, want %s", got, want)
			}
			// Declared through toolutil.WithNarrative rather than as a
			// literal, the same as the twenty-two in actions.go:
			// audit-spec-drift reads these two flags to tell a deliberate
			// improvement from prose that drifted.
			if !spec.TitleOverridden {
				t.Error("TitleOverridden = false; the spec was not built through toolutil.WithNarrative")
			}
			if !spec.DescriptionOverridden {
				t.Error("DescriptionOverridden = false; the spec was not built through toolutil.WithNarrative")
			}
		})
	}
}

// vendoredSpec is the Business Edition document cmd/gen_action_inputs
// generates from and cmd/audit_spec_drift compares against first
// (resolveSpecOperation's EE-then-CE precedence). Both routes are declared in
// it, so the Community Edition document — whose multipart schemas for these
// two routes are byte-identical — is never reached.
const vendoredSpec = "../../../api/specs/ee-2.44.0.json"

// TestUnit_HandWrittenInputs_PublishTheShapeTheSpecificationDeclares runs the
// real drift comparison over all three hand-written actions, here, because
// nothing else will until this domain is registered.
//
// cmd/audit_spec_drift audits the catalog, and stacks is not in it yet: a
// published shape that disagrees with the specification would gate the build
// in a later task's commit rather than in the one that introduced it. So the
// same engine both audits use — specdiff.ShapeFromCatalog against
// specdiff.ShapeFromSpec, compared by specdiff.Compare — is driven directly
// against these two actions and the vendored document they were transcribed
// from.
//
// Only Title and Description may differ, and only when the catalog side
// declares the difference deliberate: toolutil.WithNarrative sets
// AfterOverridden, which is exactly what cmd/audit_spec_drift's isGating
// consults before excusing a ChangeTitle or ChangeOperationDescription.
// Every other kind — a field added or removed, a type, a required-ness, an
// enum, an origin, or a field-level description — gates there and fails here,
// with the same reasoning and one commit earlier.
//
// What this pins, concretely, is the pair of required-ness rulings inputs.go
// argues for: file optional on both routes, and Name and SwarmID optional on
// the Swarm route, because neither vendored multipart schema says otherwise.
// Tightening any of them without an allow-list entry fails right here.
//
// For stacks.migrate it pins something else again, and something no other
// test in this package can see: that the published field is called
// "endpointIdQuery" and not "endpointId". internal/specdiff derives the
// vendored side's names through the same internal/specnaming rule
// cmd/gen_action_inputs uses, so publishing the query parameter under its raw
// name would land here as one field added and another removed — and it is the
// only mechanism that would notice, since the request the handler builds is
// identical either way (the value would simply come from the wrong field).
// It also pins the five body properties' empty descriptions, which are the
// Business Edition document's own: filling them in from the Community
// Edition document, which describes three of them, is a gating
// ChangeFieldDescription on each.
func TestUnit_HandWrittenInputs_PublishTheShapeTheSpecificationDeclares(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(vendoredSpec)
	if err != nil {
		t.Fatalf("read %s: %v", vendoredSpec, err)
	}

	for _, action := range []string{"stacks.create_docker_standalone_file", "stacks.create_docker_swarm_file", "stacks.migrate"} {
		t.Run(action, func(t *testing.T) {
			t.Parallel()
			spec := findSpec(t, action)

			catalog, err := specdiff.ShapeFromCatalog(spec)
			if err != nil {
				t.Fatalf("ShapeFromCatalog: %v", err)
			}
			op, err := specdiff.LoadSpecOperation(data, spec.OperationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s): %v", spec.OperationID, err)
			}
			vendored, err := specdiff.ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(%s): %v", spec.OperationID, err)
			}

			for _, change := range specdiff.Compare(vendored, catalog) {
				switch change.Kind {
				case specdiff.ChangeTitle, specdiff.ChangeOperationDescription:
					if !change.AfterOverridden {
						t.Errorf("%s: %s on %s is not declared an override; build the spec through toolutil.WithNarrative",
							spec.OperationID, change.Kind, change.JSONName)
					}
				default:
					t.Errorf("%s: %s on field %q — before %q, after %q. This gates cmd/audit_spec_drift; the published shape must match the vendored one or carry a dated api/spec-drift-allowlist.yaml entry",
						spec.OperationID, change.Kind, change.JSONName, change.Before, change.After)
				}
			}
		})
	}
}

// migratedStack is the 200 body POST /stacks/{id}/migrate answers with. Same
// fixture, and same live credential, as the two create routes above: the
// route returns a PortainereeStack too.
func migratedStack(t *testing.T) []byte {
	t.Helper()
	return mustMarshal(t, gitBackedStack())
}

const (
	// fullMigrateInput sets endpointId and endpointIdQuery to DIFFERENT
	// values, and that is the whole point of it.
	//
	// Both fields are called "endpointId" on the wire — one in the body, one
	// in the query string — and internal/specnaming publishes the query one
	// as "endpointIdQuery" precisely because they collide. A handler that
	// wired them backwards, or that fed both from one field (which is exactly
	// what the generated handler cmd/gen_action_inputs refused to emit would
	// have done), still produces a well-formed request and a 200. Only
	// distinct values can tell the two apart: 9 is the migration target and
	// must reach the body, 3 is the pre-1.18 fixup and must reach the query.
	fullMigrateInput = `{
		"id": 4,
		"endpointId": 9,
		"endpointIdQuery": 3,
		"name": "moved-stack",
		"swarmId": "jpofkc0i9uo9wtx1zesuk649w",
		"namespace": "default",
		"isHelm": true
	}`

	// minimalMigrateInput carries only what the vendored document lists
	// required — the path identifier and the body's EndpointID — and no
	// endpointIdQuery at all. It is the second half of the query/body check:
	// the query string must come out EMPTY. A handler that unmarshalled the
	// caller's raw input into apigen.StackMigrateParams would send
	// endpointId=9 here, silently applying the migration target as a
	// pre-1.18 environment fixup for a caller who asked for no such thing.
	minimalMigrateInput = `{"id":4,"endpointId":9}`
)

// decodeRequestBody decodes a captured JSON request body into a map, so an
// assertion can name the exact wire keys the server will read.
//
// A map rather than apigen.StacksStackMigratePayload deliberately: decoding
// into that struct would match keys case-insensitively, the way Go's
// encoding/json always does, so a body that spelled the property
// "endpointId" instead of the document's "EndpointID" would decode
// identically and the test would not notice. The multipart tests above assert
// literal part names for the same reason.
func decodeRequestBody(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("the server received no request body")
	}
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("decode the captured request body %q: %v", raw, err)
	}
	return body
}

// TestUnit_MigrateRequest_SendsTheQueryFieldAsTheQueryAndTheBodyFieldAsTheBody
// is the test this hand-written handler exists for.
//
// POST /stacks/{id}/migrate is the one operation in this domain whose query
// parameter and request body contribute the same wire name: the query's
// optional "endpointId" (a fixup for a stack created before Portainer 1.18.0
// that recorded no environment) and the body's required "EndpointID" (the
// environment the stack is migrating to). internal/specnaming gives the body
// the plain name and publishes the query parameter as "endpointIdQuery", and
// cmd/gen_action_inputs refuses the operation because a generated handler
// would unmarshal the caller's raw input into apigen.StackMigrateParams —
// whose field is tagged `json:"endpointId,omitempty"`, an exact match for the
// BODY's key — and so would migrate to an environment the caller never named.
//
// Nothing about that failure is visible from a status code: the request stays
// well-formed and Portainer answers 200 after migrating the stack somewhere
// else. So the two fields carry different values here and each is asserted at
// its own destination, and the second row asserts the query string is empty
// when the caller supplies no endpointIdQuery at all — the case a
// raw-input unmarshal gets wrong most loudly, by inventing a parameter.
func TestUnit_MigrateRequest_SendsTheQueryFieldAsTheQueryAndTheBodyFieldAsTheBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantQuery string
		wantBody  map[string]any
	}{
		{
			name:  "both endpoint fields set, to different values",
			input: fullMigrateInput,
			// 3, the endpointIdQuery field — never 9, the migration target.
			wantQuery: "endpointId=3",
			wantBody: map[string]any{
				"EndpointID": float64(9),
				"Name":       "moved-stack",
				"SwarmID":    "jpofkc0i9uo9wtx1zesuk649w",
				"Namespace":  "default",
				"IsHelm":     true,
			},
		},
		{
			name:  "no endpointIdQuery leaves the query string empty",
			input: minimalMigrateInput,
			// Not "endpointId=9", and not "endpointId=0" either: the
			// parameter is optional and the caller said nothing about it.
			wantQuery: "",
			wantBody: map[string]any{
				"EndpointID": float64(9),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, migratedStack(t))

			if _, err := find(t, "stacks.migrate")(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("handler error = %v", err)
			}

			if captured.method != http.MethodPost {
				t.Errorf("method = %s, want POST", captured.method)
			}
			// The path identifier is the third field that could be crossed
			// with the other two; 4 is neither 9 nor 3.
			if want := "/api/stacks/4/migrate"; captured.path != want {
				t.Errorf("path = %s, want %s", captured.path, want)
			}
			if captured.rawQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q — endpointIdQuery is the pre-1.18 fixup parameter and the body's endpointId is the migration target; sending either in the other's place migrates the stack to the wrong environment",
					captured.rawQuery, tt.wantQuery)
			}

			body := decodeRequestBody(t, captured.body)
			if !reflect.DeepEqual(body, tt.wantBody) {
				t.Errorf("request body = %v, want %v", body, tt.wantBody)
			}
			// Stated separately from the DeepEqual above so a failure names
			// the defect rather than a whole-map difference: the qualified
			// name is this project's own invention and means nothing to
			// Portainer, and the body must never carry the query's value.
			if _, leaked := body["endpointIdQuery"]; leaked {
				t.Error("the request body carries endpointIdQuery; that name exists only to disambiguate the published schema and is not a property of stacks.stackMigratePayload")
			}
		})
	}
}

// TestUnit_MigrateWithGitCredentialInResponse_ReturnsNoCredential is the
// discriminating redaction test for this handler, and like the two above it
// has to exist separately.
//
// redaction_test.go's table drives generated wrappers and generated handlers;
// StackMigrate has no generated handler, so no entry there reaches this code
// path, and cmd/gen_action_inputs's checkCredentialRedaction is a static AST
// scan over generated code that only proves redactStackMigrate is declared.
// Nothing but this stands between a hand-written handler that returns
// resp.JSON200 directly and a git password reaching a model — confirmed by
// mutation: replacing the redaction call with the raw response makes this
// test fail naming hunter2.
func TestUnit_MigrateWithGitCredentialInResponse_ReturnsNoCredential(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "every field supplied", input: fullMigrateInput},
		{name: "only the required fields", input: minimalMigrateInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := capturingClient(t, http.StatusOK, migratedStack(t))

			out, err := find(t, "stacks.migrate")(context.Background(), c, json.RawMessage(tt.input))
			if err != nil {
				t.Fatalf("handler error = %v", err)
			}
			// The migrated stack and its repository URL are what the action
			// exists to return: a handler returning nil, or dropping
			// GitConfig whole, would satisfy every credential assertion.
			assertNoCredential(t, "stacks.migrate", out, "nginx-stack", "https://git.example.com/team/app.git")
		})
	}
}

// TestUnit_MigrateWhenPortainerRefuses_ReturnsTheServerError pins that this
// handler runs its response through toolutil.Check, and that a decode failure
// is wrapped with the operation's own name.
//
// The 409 is the route's own: "A stack with the same name is already running
// on the target environment(endpoint)" is what a second migrate is answered
// with, and it is the measured fact behind this action not being marked
// Idempotent. Without the Check call it would reach a model as a nil body and
// no error — a migration that never happened, reported as one that did.
func TestUnit_MigrateWhenPortainerRefuses_ReturnsTheServerError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		status int
		body   string
		want   string
	}{
		{
			name:   "the target environment already runs a stack of that name",
			input:  minimalMigrateInput,
			status: http.StatusConflict,
			body:   `{"message":"A stack with the same name is already running on the target environment"}`,
			want:   "A stack with the same name is already running on the target environment",
		},
		{
			name:   "no such stack",
			input:  minimalMigrateInput,
			status: http.StatusNotFound,
			body:   `{"message":"Stack not found"}`,
			want:   "Stack not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, _ := capturingClient(t, tt.status, []byte(tt.body))

			out, err := find(t, "stacks.migrate")(context.Background(), c, json.RawMessage(tt.input))
			if err == nil {
				t.Fatalf("handler error = nil and returned %v, want the server's refusal", out)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to carry the server's own message", err)
			}
			if !strings.Contains(err.Error(), "StackMigrate") {
				t.Errorf("error = %v, want it wrapped with the operation's own name", err)
			}
		})
	}
}

// TestUnit_MigrateWithUnparseableInput_ReturnsAWrappedError pins the decode
// step's error context, the one thing a hand-written handler can get wrong
// that the generated ones cannot.
func TestUnit_MigrateWithUnparseableInput_ReturnsAWrappedError(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusOK, migratedStack(t))

	_, err := find(t, "stacks.migrate")(context.Background(), c, json.RawMessage(`{"id":"four"}`))
	if err == nil {
		t.Fatal("handler error = nil, want a decode failure")
	}
	if want := "StackMigrate: parse input"; !strings.Contains(err.Error(), want) {
		t.Errorf("error = %v, want it wrapped with %q", err, want)
	}
}

// TestUnit_DeleteKubernetesByName_SendsTheUndocumentedNamespaceParameter pins
// the one thing that makes this action callable at all.
//
// Neither vendored document declares `namespace`, and the server requires it:
// without it the route answers 400 "Invalid query parameter: namespace" on
// both editions, measured, and the check runs before the environment is
// resolved (docs/api-divergences.md §3.9). The generated handler could not
// send it, so every call it could make failed.
//
// The assertion is on the query string the server would receive, not on a
// status code: a handler that accepted the field and dropped it would return
// whatever the stub answers and look perfectly healthy.
func TestUnit_DeleteKubernetesByName_SendsTheUndocumentedNamespaceParameter(t *testing.T) {
	t.Parallel()

	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	c, err := portainer.New(&config.Config{URL: srv.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	input := json.RawMessage(`{"name":"probe-k8s","endpointId":1,"namespace":"default"}`)
	if _, err := find(t, "stacks.delete_kubernetes_by_name")(context.Background(), c, input); err != nil {
		t.Fatalf("handler error = %v", err)
	}

	q, parseErr := url.ParseQuery(gotQuery)
	if parseErr != nil {
		t.Fatalf("parsing the query the server received (%q): %v", gotQuery, parseErr)
	}
	if got := q.Get("namespace"); got != "default" {
		t.Errorf("namespace = %q, want \"default\"; without it every call this action makes answers 400 \"Invalid query parameter: namespace\" (docs/api-divergences.md §3.9). Query was %q", got, gotQuery)
	}
	if got := q.Get("endpointId"); got != "1" {
		t.Errorf("endpointId = %q, want \"1\"; the namespace must be added alongside the documented parameters, not instead of them. Query was %q", got, gotQuery)
	}
}
