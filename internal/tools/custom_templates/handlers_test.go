package custom_templates

import (
	"context"
	"encoding/json"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// capturedRequest is what a server saw: the parsed multipart form plus the
// two header values that decide whether it can be parsed at all.
type capturedRequest struct {
	method      string
	path        string
	rawQuery    string
	contentType string
	form        *multipart.Form
}

// capturingClient answers every request with body and records the request
// itself, parsed the way net/http parses a multipart upload.
//
// Deliberately not clientFor from custom_templates_test.go: that helper
// discards the request, which is exactly what this file needs to inspect.
// The parse is done here, in the server, because http.Request.MultipartReader
// consumes the body — a test that kept the raw bytes and parsed later would
// be asserting against its own copy rather than against what a server can
// actually read out of the wire format.
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

// createdTemplate is the 200 body the server answers with. Reusing
// gitBackedTemplate keeps the fixture — and its live credential — identical
// to the one the eight generated handlers are checked against; the
// identifier differs only because a create answers with a new one.
func createdTemplate(t *testing.T) []byte {
	t.Helper()
	return mustMarshal(t, gitBackedTemplate(11, "uploaded-stack"))
}

const (
	fullCreateFileInput = `{
		"title": "uploaded-stack",
		"description": "a git-backed template",
		"note": "<b>deploys nginx</b>",
		"platform": 1,
		"type": 2,
		"file": "services:\n  web:\n    image: nginx\n",
		"logo": "https://example.com/logo.png",
		"edgeSettings": "{\"prePullImage\":true}",
		"edgeTemplate": true,
		"variables": "[{\"name\":\"PORT\",\"defaultValue\":\"8080\"}]"
	}`

	minimalCreateFileInput = `{
		"title": "uploaded-stack",
		"description": "a git-backed template",
		"note": "deploys nginx",
		"platform": 1,
		"type": 2,
		"file": "services: {}"
	}`
)

// TestUnit_CreateFileRequest_IsAMultipartBodyWithABoundary pins the wire
// format itself, which no other test in this domain touches: the eight
// generated handlers all send JSON, and this route accepts nothing but
// multipart/form-data. A content type without the generated boundary is the
// failure that produces a server-side complaint about the body rather than
// about any field, so it is asserted first and separately.
func TestUnit_CreateFileRequest_IsAMultipartBodyWithABoundary(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, createdTemplate(t))

	if _, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(minimalCreateFileInput)); err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if captured.method != http.MethodPost {
		t.Errorf("method = %s, want POST", captured.method)
	}
	if want := "/api/custom_templates/create/file"; captured.path != want {
		t.Errorf("path = %s, want %s", captured.path, want)
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
}

// TestUnit_CreateFileRequest_CarriesEveryFieldUnderTheSpecifiedPartName pins
// the mapping between the Input a model fills and the parts Portainer reads.
//
// Two things here can be got wrong silently. The part names are the vendored
// schema's own capitalised ones (Title, File, EdgeSettings…) while the Input
// publishes lowerCamelCase to the model; multipart part names are matched
// literally, not case-folded like a header, so sending "title" would reach a
// server that reports Title missing. And EdgeSettings and Variables are
// JSON-encoded *strings* on this route where the two JSON siblings take a
// nested object and an array — re-marshalling them here would wrap the
// caller's document in a second layer of quoting, which the assertions below
// on the exact part bodies are what catch.
func TestUnit_CreateFileRequest_CarriesEveryFieldUnderTheSpecifiedPartName(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, createdTemplate(t))

	if _, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(fullCreateFileInput)); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if captured.form == nil {
		t.Fatal("the server could not parse the body as a multipart form")
	}

	want := map[string]string{
		"Title":        "uploaded-stack",
		"Description":  "a git-backed template",
		"Note":         "<b>deploys nginx</b>",
		"Platform":     "1",
		"Type":         "2",
		"Logo":         "https://example.com/logo.png",
		"EdgeTemplate": "true",
		// Verbatim, not re-marshalled: this route types both of these
		// "string" and Portainer unmarshals the part's own content.
		"EdgeSettings": `{"prePullImage":true}`,
		"Variables":    `[{"name":"PORT","defaultValue":"8080"}]`,
	}
	for name, value := range want {
		t.Run(name, func(t *testing.T) {
			got, ok := captured.form.Value[name]
			if !ok {
				t.Fatalf("%s is absent from the body; parts sent: %v", name, partNames(captured.form))
			}
			if len(got) != 1 || got[0] != value {
				t.Errorf("%s = %q, want [%q]", name, got, value)
			}
		})
	}
}

// TestUnit_CreateFileRequest_SendsTheStackFileAsAFilePartWithAFilename is
// the assertion that separates a working upload from one whose bytes are all
// present and still unreadable. Go's multipart reader — Portainer's own
// parser — files a part under Form.File only when its Content-Disposition
// names a file, so a "File" part written as ordinary text arrives at the
// server's lookup as nothing at all.
func TestUnit_CreateFileRequest_SendsTheStackFileAsAFilePartWithAFilename(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, createdTemplate(t))

	if _, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(fullCreateFileInput)); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if captured.form == nil {
		t.Fatal("the server could not parse the body as a multipart form")
	}

	if _, isText := captured.form.Value["File"]; isText {
		t.Error("File was sent as a text part; a server's FormFile lookup finds nothing without a filename")
	}
	headers, ok := captured.form.File["File"]
	if !ok || len(headers) != 1 {
		t.Fatalf("File is not a file part; file parts sent: %v", captured.form.File)
	}
	if headers[0].Filename == "" {
		t.Error("the File part carries no filename")
	}
	if headers[0].Filename != uploadFilename {
		t.Errorf("File filename = %q, want %q", headers[0].Filename, uploadFilename)
	}

	opened, err := headers[0].Open()
	if err != nil {
		t.Fatalf("open the File part: %v", err)
	}
	defer func() { _ = opened.Close() }()
	content := make([]byte, headers[0].Size)
	if _, err := opened.Read(content); err != nil {
		t.Fatalf("read the File part: %v", err)
	}
	if want := "services:\n  web:\n    image: nginx\n"; string(content) != want {
		t.Errorf("File content = %q, want %q", content, want)
	}
}

// TestUnit_CreateFileRequestWithoutTheOptionalFields_OmitsThoseParts pins
// that an unset optional field emits no part rather than an empty one. The
// difference is not cosmetic on this route: EdgeSettings and Variables carry
// JSON documents, and an empty part is a present field whose document fails
// to parse — a 500 for a field the caller never mentioned.
func TestUnit_CreateFileRequestWithoutTheOptionalFields_OmitsThoseParts(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, createdTemplate(t))

	if _, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(minimalCreateFileInput)); err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if captured.form == nil {
		t.Fatal("the server could not parse the body as a multipart form")
	}

	for _, name := range []string{"Logo", "EdgeSettings", "EdgeTemplate", "Variables"} {
		t.Run(name, func(t *testing.T) {
			if got, ok := captured.form.Value[name]; ok {
				t.Errorf("%s was not supplied but the body carries it as %q", name, got)
			}
		})
	}
	// The six required parts are still there: an implementation that emitted
	// nothing at all would pass every assertion above.
	for _, name := range []string{"Title", "Description", "Note", "Platform", "Type"} {
		if _, ok := captured.form.Value[name]; !ok {
			t.Errorf("required part %s is missing; parts sent: %v", name, partNames(captured.form))
		}
	}
	if _, ok := captured.form.File["File"]; !ok {
		t.Errorf("required file part File is missing; file parts sent: %v", captured.form.File)
	}
}

// TestUnit_CreateFileWithGitCredentialInResponse_ReturnsNoCredential is the
// discriminating redaction test for this handler, and it has to exist
// separately from every other redaction check in this domain.
//
// redaction_test.go's generated table drives each wrapper and each
// *generated* handler; CustomTemplateCreateFile has no generated handler, so
// no entry there covers this code path. cmd/gen_action_inputs's
// checkCredentialRedaction is a static AST scan for a call site, and it runs
// against generated code, not against this file. So nothing but this test
// stands between a hand-written handler that returns resp.JSON200 directly
// and a git password reaching a model — and the mutation was performed to
// confirm that: replacing the redaction call with the raw response makes
// this test fail naming the password (recorded in this stage's task-5
// report).
//
// It must be a unit test over a response constructed here. Portainer 2.44.0
// was measured blanking the git password itself on the sibling create route,
// so an end-to-end test against a live server cannot tell a redactor that
// works from one that does nothing: the field arrives empty either way.
//
// Username is asserted absent alongside Password because redactCustomTemplate
// drops the whole Authentication sub-object; asserting only Password would
// pass a field-by-field redactor that kept the username, which no narrative
// in this domain promises a caller anyway.
func TestUnit_CreateFileWithGitCredentialInResponse_ReturnsNoCredential(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusOK, createdTemplate(t))

	out, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(fullCreateFileInput))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded := string(mustMarshal(t, out))

	if strings.Contains(encoded, "hunter2") {
		t.Errorf("custom_templates.create_file returned the git password to the caller: %s", encoded)
	}
	if strings.Contains(encoded, "deploy-bot") {
		t.Errorf("custom_templates.create_file returned the git username to the caller; Authentication must be dropped whole: %s", encoded)
	}
	if strings.Contains(encoded, `"GitCredentialID"`) {
		t.Errorf("custom_templates.create_file returned the git credential identifier to the caller: %s", encoded)
	}
	// Redaction must remove the credential and nothing else: the created
	// template is what the action exists to return.
	if !strings.Contains(encoded, "uploaded-stack") {
		t.Errorf("redaction removed more than the credential, the template title is gone: %s", encoded)
	}
	if !strings.Contains(encoded, "https://git.example.com/team/app.git") {
		t.Errorf("redaction removed more than the credential, the repository URL is gone: %s", encoded)
	}
}

// TestUnit_CreateFileWhenPortainerRefuses_ReturnsTheServerError pins that
// this handler runs its response through toolutil.Check like its eight
// generated neighbours. Without that call a refusal returns a nil JSON200
// and no error, which reaches a model as a successful creation that did not
// happen.
func TestUnit_CreateFileWhenPortainerRefuses_ReturnsTheServerError(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusInternalServerError, []byte(`{"message":"Invalid custom template platform"}`))

	out, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(minimalCreateFileInput))
	if err == nil {
		t.Fatalf("handler error = nil and returned %v, want the server's refusal", out)
	}
	if !strings.Contains(err.Error(), "Invalid custom template platform") {
		t.Errorf("error = %v, want it to carry the server's own message", err)
	}
	if !strings.Contains(err.Error(), "CustomTemplateCreateFile") {
		t.Errorf("error = %v, want it wrapped with the operation's own name", err)
	}
}

// TestUnit_CreateFileWithUnparseableInput_ReturnsAWrappedError pins the
// decode step's error context, the one thing a hand-written handler can get
// wrong that the generated ones cannot.
func TestUnit_CreateFileWithUnparseableInput_ReturnsAWrappedError(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusOK, createdTemplate(t))

	_, err := find(t, "custom_templates.create_file")(context.Background(), c, json.RawMessage(`{"platform":"linux"}`))
	if err == nil {
		t.Fatal("handler error = nil, want a decode failure")
	}
	if !strings.Contains(err.Error(), "CustomTemplateCreateFile: parse input") {
		t.Errorf("error = %v, want it wrapped with the operation and the step", err)
	}
}

// TestUnit_CreateFileSpec_MatchesTheDomainsGeneratedNeighbours pins the
// declaration itself. handWrittenSpecs is written by hand, so nothing
// regenerates it and nothing else checks that it names the operation and the
// route the rest of this domain assumes.
func TestUnit_CreateFileSpec_MatchesTheDomainsGeneratedNeighbours(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, "custom_templates.create_file")

	if spec.OperationID != "CustomTemplateCreateFile" {
		t.Errorf("OperationID = %q, want CustomTemplateCreateFile", spec.OperationID)
	}
	if spec.Domain != "custom_templates" {
		t.Errorf("Domain = %q, want custom_templates", spec.Domain)
	}
	if !spec.Mutating {
		t.Error("Mutating = false; this route creates a template")
	}
	if spec.Destructive {
		t.Error("Destructive = true; this route creates at a new identifier and removes nothing, like its two sibling creates")
	}
	if spec.Handler == nil {
		t.Fatal("Handler is nil")
	}
	if _, ok := spec.Input.(customTemplateCreateFileInput); !ok {
		t.Errorf("Input = %T, want customTemplateCreateFileInput", spec.Input)
	}
	// Declared through toolutil.WithNarrative rather than as a literal, the
	// same as the eight in actions.go: audit-spec-drift reads these two
	// flags to tell a deliberate improvement from prose that drifted.
	if !spec.TitleOverridden {
		t.Error("TitleOverridden = false; the spec was not built through toolutil.WithNarrative")
	}
	if !spec.DescriptionOverridden {
		t.Error("DescriptionOverridden = false; the spec was not built through toolutil.WithNarrative")
	}
}

// partNames renders the text part names of a form for a failure message.
func partNames(form *multipart.Form) []string {
	names := make([]string, 0, len(form.Value))
	for name := range form.Value {
		names = append(names, name)
	}
	return names
}

// TestUnit_CustomTemplateList_SendsTheTypeParameterRepeated pins the one
// thing the hand-written list handler exists for.
//
// The generated client rendered `type=1,2,3` because the vendored
// specification declares the parameter explode: false, and a live 2.44.0
// answers that with 400 "Failed parsing template type: strconv.Atoi:
// parsing \"1,2,3\"" on both editions, while `type=1&type=2&type=3`
// answers 200 (docs/api-divergences.md §6.7). Only the wire encoding
// distinguishes the two, so only an assertion on the wire encoding can
// catch a regression back to the comma form — url.Values.Encode is what
// produces the repeated key today, and a future edit reaching for
// strings.Join would compile, pass every other test in this package, and
// break the action against every real server.
//
// The single-type call is asserted too, and deliberately: it is the one
// call the broken encoding also got right, so a test that only ever passed
// one type could not tell the two encodings apart at all.
func TestUnit_CustomTemplateList_SendsTheTypeParameterRepeated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "three types",
			input: `{"type":[1,2,3]}`,
			want:  "type=1&type=2&type=3",
		},
		{
			name:  "one type",
			input: `{"type":[2]}`,
			want:  "type=2",
		},
		{
			name:  "types and the edge filter",
			input: `{"type":[1,2],"edge":true}`,
			want:  "edge=true&type=1&type=2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, captured := capturingClient(t, http.StatusOK, []byte(`[]`))

			if _, err := customTemplateList(context.Background(), c, json.RawMessage(tt.input)); err != nil {
				t.Fatalf("customTemplateList(%s): %v", tt.input, err)
			}
			if captured.path != "/api/custom_templates" {
				t.Errorf("path = %q, want %q", captured.path, "/api/custom_templates")
			}
			if captured.rawQuery != tt.want {
				t.Errorf("query = %q, want %q (the repeated form the server parses; a comma-joined value is a 400)", captured.rawQuery, tt.want)
			}
			if strings.Contains(captured.rawQuery, "%2C") || strings.Contains(captured.rawQuery, ",") {
				t.Errorf("query = %q carries a comma: that is the encoding the server refuses", captured.rawQuery)
			}
		})
	}
}

// TestUnit_CustomTemplateList_EmptyBodyAnswersAnEmptyList records the one
// behavioural difference between this handler and the generated one it
// replaced: a 200 with no body at all now answers [] rather than the null
// the generated decoder's nil pointer produced.
func TestUnit_CustomTemplateList_EmptyBodyAnswersAnEmptyList(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusOK, nil)

	out, err := customTemplateList(context.Background(), c, json.RawMessage(`{"type":[2]}`))
	if err != nil {
		t.Fatalf("customTemplateList: %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if string(encoded) != "[]" {
		t.Errorf("customTemplateList over an empty body = %s, want []", encoded)
	}
}

// TestUnit_CustomTemplateList_ServerErrorIsReported proves the hand-written
// handler classifies a failing status instead of decoding the error body as
// a template list. The message is the server's own, which is what makes a
// 400 like §6.7's diagnosable from a tool result.
func TestUnit_CustomTemplateList_ServerErrorIsReported(t *testing.T) {
	t.Parallel()
	c, _ := capturingClient(t, http.StatusBadRequest,
		[]byte(`{"message":"Invalid Custom template type","details":"Failed parsing template type"}`))

	_, err := customTemplateList(context.Background(), c, json.RawMessage(`{"type":[2]}`))
	if err == nil {
		t.Fatal("customTemplateList against a 400 returned no error")
	}
	if !strings.Contains(err.Error(), "Invalid Custom template type") {
		t.Errorf("error = %v, want it to carry the server's own message", err)
	}
}
