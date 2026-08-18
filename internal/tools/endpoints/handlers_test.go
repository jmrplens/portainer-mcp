package endpoints

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// capturedRequest is what a server saw: the request line, the query string,
// and the parsed multipart form for the two routes that send one.
type capturedRequest struct {
	method      string
	path        string
	rawQuery    string
	escapedPath string
	contentType string
	form        *multipart.Form
}

// capturingClient answers every request with body and records the request
// itself, parsed the way net/http parses a multipart upload.
//
// The parse happens in the server rather than against a saved copy of the
// bytes, following internal/tools/custom_templates's own helper and for its
// reason: http.Request.MultipartReader consumes the body, so a test that
// kept the raw bytes and parsed them itself would be asserting against its
// own reconstruction rather than against what a server can actually read off
// the wire.
func capturingClient(t *testing.T, status int, body []byte) (*portainer.Client, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured.method = r.Method
		captured.path = r.URL.Path
		captured.escapedPath = r.URL.EscapedPath()
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

// unreachableClient points at a server that fails the test if it is ever
// reached. Every "refuses before the request" assertion below uses it, which
// is what makes those assertions mean "refused locally" rather than merely
// "returned an error".
func unreachableClient(t *testing.T) *portainer.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("handler reached the network: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c
}

// createdEnvironment is a minimal 200 body for the routes that answer with
// an environment.
const createdEnvironment = `{"Id":7,"Name":"prod","EdgeKey":"","Type":2}`

// fullCreateInput exercises every optional field endpointCreate can send, so
// TestUnit_EndpointCreate_CarriesEveryFieldUnderTheSpecifiedPartName can
// check the whole mapping rather than a sample of it.
const fullCreateInput = `{
	"name": "prod",
	"endpointCreationType": 2,
	"url": "tcp://docker.example.com:2376",
	"publicUrl": "docker.example.com",
	"groupId": 3,
	"tls": true,
	"tlsSkipVerify": true,
	"tlsSkipClientVerify": true,
	"tlsCACertFile": "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----",
	"tlsCertFile": "-----BEGIN CERTIFICATE-----\ncert\n-----END CERTIFICATE-----",
	"tlsKeyFile": "-----BEGIN PRIVATE KEY-----\nkey\n-----END PRIVATE KEY-----",
	"azureApplicationId": "app",
	"azureAuthenticationKey": "secret",
	"azureTenantId": "tenant",
	"containerEngine": "docker",
	"customTemplateContent": "version: '3'",
	"customTemplateId": 4,
	"edgeAsyncMode": true,
	"edgeCheckinInterval": 5,
	"edgeCommandInterval": 6,
	"edgePingInterval": 7,
	"edgeSnapshotInterval": 8,
	"edgeTunnelServerAddress": "tunnel.example.com:8000",
	"gpus": "[{\"name\":\"gpu0\",\"value\":\"0\"}]",
	"kubeConfig": "YXBpVmVyc2lvbjogdjE=",
	"stackName": "bootstrap",
	"tagIds": "[1,2]"
}`

// TestUnit_EndpointCreate_SendsAMultipartBodyWithABoundary pins the one
// property of this request that is not a field: POST /endpoints declares
// multipart/form-data as its only content type, and a body sent as JSON —
// which is what every generated handler in this domain sends — would be
// rejected by Portainer's own form parser before any field was read.
func TestUnit_EndpointCreate_SendsAMultipartBodyWithABoundary(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(createdEnvironment))
	if _, err := endpointCreate(context.Background(), c, json.RawMessage(fullCreateInput)); err != nil {
		t.Fatalf("endpointCreate: %v", err)
	}
	if captured.method != http.MethodPost {
		t.Errorf("method = %q, want POST", captured.method)
	}
	if captured.path != "/api/endpoints" {
		t.Errorf("path = %q, want /api/endpoints", captured.path)
	}
	mediaType, params, err := mime.ParseMediaType(captured.contentType)
	if err != nil {
		t.Fatalf("parse Content-Type %q: %v", captured.contentType, err)
	}
	if mediaType != "multipart/form-data" {
		t.Errorf("media type = %q, want multipart/form-data", mediaType)
	}
	if params["boundary"] == "" {
		t.Error("Content-Type carries no boundary; the server cannot parse the body")
	}
	if captured.form == nil {
		t.Fatal("the server could not parse the body as a multipart form")
	}
}

// TestUnit_EndpointCreate_CarriesEveryFieldUnderTheSpecifiedPartName is the
// test the mapping in endpointCreateBody exists to be checked by. Multipart
// part names are not JSON keys: Portainer matches them literally against the
// vendored schema's own capitalised property names, so a part written under
// the lowerCamelCase name the Input publishes to a model is a field the
// server never sees, silently.
func TestUnit_EndpointCreate_CarriesEveryFieldUnderTheSpecifiedPartName(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(createdEnvironment))
	if _, err := endpointCreate(context.Background(), c, json.RawMessage(fullCreateInput)); err != nil {
		t.Fatalf("endpointCreate: %v", err)
	}
	if captured.form == nil {
		t.Fatal("no multipart form was parsed")
	}

	want := map[string]string{
		"Name":                    "prod",
		"EndpointCreationType":    "2",
		"URL":                     "tcp://docker.example.com:2376",
		"PublicURL":               "docker.example.com",
		"GroupID":                 "3",
		"TLS":                     "true",
		"TLSSkipVerify":           "true",
		"TLSSkipClientVerify":     "true",
		"AzureApplicationID":      "app",
		"AzureAuthenticationKey":  "secret",
		"AzureTenantID":           "tenant",
		"ContainerEngine":         "docker",
		"CustomTemplateContent":   "version: '3'",
		"CustomTemplateID":        "4",
		"EdgeAsyncMode":           "true",
		"EdgeCheckinInterval":     "5",
		"EdgeCommandInterval":     "6",
		"EdgePingInterval":        "7",
		"EdgeSnapshotInterval":    "8",
		"EdgeTunnelServerAddress": "tunnel.example.com:8000",
		"Gpus":                    `[{"name":"gpu0","value":"0"}]`,
		"KubeConfig":              "YXBpVmVyc2lvbjogdjE=",
		"StackName":               "bootstrap",
		"TagIds":                  "[1,2]",
	}
	for name, expected := range want {
		values := captured.form.Value[name]
		if len(values) != 1 {
			t.Errorf("part %q: got %d value(s), want exactly 1 — a part written under the wrong name is a field the server never reads", name, len(values))
			continue
		}
		if values[0] != expected {
			t.Errorf("part %q = %q, want %q", name, values[0], expected)
		}
	}
}

// TestUnit_EndpointCreate_SendsTheTLSMaterialAsFilePartsWithFilenames pins
// the three format:binary fields. Written as an ordinary value part, Go's
// multipart reader would file them under Value rather than File and
// Portainer's own handler — which reads them with a file reader — would
// report the certificate missing on a request that plainly carried it.
func TestUnit_EndpointCreate_SendsTheTLSMaterialAsFilePartsWithFilenames(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(createdEnvironment))
	if _, err := endpointCreate(context.Background(), c, json.RawMessage(fullCreateInput)); err != nil {
		t.Fatalf("endpointCreate: %v", err)
	}
	if captured.form == nil {
		t.Fatal("no multipart form was parsed")
	}
	for _, part := range []struct{ name, contains string }{
		{"TLSCACertFile", "BEGIN CERTIFICATE"},
		{"TLSCertFile", "BEGIN CERTIFICATE"},
		{"TLSKeyFile", "BEGIN PRIVATE KEY"},
	} {
		files := captured.form.File[part.name]
		if len(files) != 1 {
			t.Errorf("%s: got %d file part(s), want 1 (it must not be sent as a value part)", part.name, len(files))
			continue
		}
		if files[0].Filename == "" {
			t.Errorf("%s: file part carries no filename; Go's reader would not treat it as a file", part.name)
		}
		f, err := files[0].Open()
		if err != nil {
			t.Fatalf("%s: open part: %v", part.name, err)
		}
		defer func() { _ = f.Close() }()
		content, err := io.ReadAll(f)
		if err != nil {
			t.Fatalf("%s: read part: %v", part.name, err)
		}
		if !strings.Contains(string(content), part.contains) {
			t.Errorf("%s: part content does not contain %q; the caller's PEM was not forwarded", part.name, part.contains)
		}
	}
}

// TestUnit_EndpointCreateWithoutTheOptionalFields_OmitsThoseParts is the
// other half of the mapping test: an omitted optional field must produce no
// part at all, not an empty one. An empty TLSCACertFile part is a
// present-but-empty certificate, which is a different request from one that
// says nothing about certificates.
func TestUnit_EndpointCreateWithoutTheOptionalFields_OmitsThoseParts(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(createdEnvironment))
	minimal := `{"name":"local","endpointCreationType":1}`
	if _, err := endpointCreate(context.Background(), c, json.RawMessage(minimal)); err != nil {
		t.Fatalf("endpointCreate: %v", err)
	}
	if captured.form == nil {
		t.Fatal("no multipart form was parsed")
	}
	if got := len(captured.form.Value); got != 2 {
		t.Errorf("value parts = %d (%v), want exactly the two required ones", got, keysOf(captured.form.Value))
	}
	if got := len(captured.form.File); got != 0 {
		t.Errorf("file parts = %d, want 0 when no TLS material was supplied", got)
	}
	for _, required := range []string{"Name", "EndpointCreationType"} {
		if len(captured.form.Value[required]) != 1 {
			t.Errorf("required part %q is missing", required)
		}
	}
}

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// TestUnit_EndpointList_SendsTheNarrowedFloatQueryParameter proves the one
// parameter that refused this operation generation actually reaches the
// wire. A handler that decoded the input but forgot to assign the narrowed
// value would drop the filter silently and answer with every environment.
func TestUnit_EndpointList_SendsTheNarrowedFloatQueryParameter(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(`[]`))
	if _, err := endpointList(context.Background(), c, json.RawMessage(`{"edgeCheckInPassedSeconds":300}`)); err != nil {
		t.Fatalf("endpointList: %v", err)
	}
	if !strings.Contains(captured.rawQuery, "edgeCheckInPassedSeconds=300") {
		t.Errorf("query = %q, want it to carry edgeCheckInPassedSeconds=300", captured.rawQuery)
	}
}

// TestUnit_EndpointList_AFloatThatCannotBeNarrowed_IsRefusedBeforeTheRequest
// is the reason toFloat32 exists rather than letting encoding/json do the
// conversion. json.Unmarshal into the client's *float32 accepts this value
// and rounds it; the caller would get an answer computed from a number they
// did not send, with nothing said.
func TestUnit_EndpointList_AFloatThatCannotBeNarrowed_IsRefusedBeforeTheRequest(t *testing.T) {
	t.Parallel()
	c := unreachableClient(t)
	_, err := endpointList(context.Background(), c, json.RawMessage(`{"edgeCheckInPassedSeconds":0.1234567890123}`))
	if err == nil {
		t.Fatal("endpointList accepted a value it cannot send exactly; want an error")
	}
	if !strings.Contains(err.Error(), "edgeCheckInPassedSeconds") {
		t.Errorf("error = %q, want it to name the offending parameter", err)
	}
}

// TestUnit_EndpointUpdate_ASnapshotMetricThatCannotBeNarrowed_IsRefusedBeforeTheRequest
// is the same guard four levels down a nested array, which is where
// checkWireWidth actually found it. The error must name the array index, or
// a caller sending a snapshot list has no way to find which element is
// wrong.
func TestUnit_EndpointUpdate_ASnapshotMetricThatCannotBeNarrowed_IsRefusedBeforeTheRequest(t *testing.T) {
	t.Parallel()
	c := unreachableClient(t)
	input := `{"id":1,"kubernetes":{"configuration":{},"flags":{},"snapshots":[
		{"kubernetesVersion":"v1.29.0","nodeCount":1,"time":0,"totalCpu":1,"totalMemory":1},
		{"kubernetesVersion":"v1.29.0","nodeCount":1,"time":0,"totalCpu":1,"totalMemory":1,
		 "performanceMetrics":{"cpuUsage":0.1234567890123}}
	]}}`
	_, err := endpointUpdate(context.Background(), c, json.RawMessage(input))
	if err == nil {
		t.Fatal("endpointUpdate accepted a metric it cannot send exactly; want an error")
	}
	if !strings.Contains(err.Error(), "snapshots[1].performanceMetrics.cpuUsage") {
		t.Errorf("error = %q, want it to name the offending element by index", err)
	}
}

// TestUnit_EndpointUpdate_AnExactlyRepresentableMetric_IsSent guards the
// opposite mistake from the test above: a guard that refused every metric
// would also pass a test that only ever supplies an unrepresentable one.
func TestUnit_EndpointUpdate_AnExactlyRepresentableMetric_IsSent(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(createdEnvironment))
	input := `{"id":4,"kubernetes":{"configuration":{},"flags":{},"snapshots":[
		{"kubernetesVersion":"v1.29.0","nodeCount":1,"time":0,"totalCpu":1,"totalMemory":1,
		 "performanceMetrics":{"cpuUsage":0.5,"memoryUsage":0.25}}
	]}}`
	if _, err := endpointUpdate(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("endpointUpdate: %v", err)
	}
	if captured.path != "/api/endpoints/4" {
		t.Errorf("path = %q, want /api/endpoints/4", captured.path)
	}
}

// TestUnit_EndpointDockerBrowsePut_SendsTheFileAndTheVolumeQueryParameter
// covers this route's two hand-built halves at once: the multipart body and
// the query parameter, which is the field this domain first got wrong by
// deriving its wire name the way a body property's is derived rather than
// taking the specification's own spelling.
func TestUnit_EndpointDockerBrowsePut_SendsTheFileAndTheVolumeQueryParameter(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusNoContent, nil)
	input := `{"id":2,"path":"/data/tls/ca.pem","file":"pem bytes","volumeID":"vol1"}`
	if _, err := endpointDockerBrowsePut(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("endpointDockerBrowsePut: %v", err)
	}
	if captured.path != "/api/endpoints/2/docker/v2/browse/put" {
		t.Errorf("path = %q", captured.path)
	}
	if !strings.Contains(captured.rawQuery, "volumeID=vol1") {
		t.Errorf("query = %q, want volumeID=vol1 (the specification spells it volumeID, not volumeId)", captured.rawQuery)
	}
	if captured.form == nil {
		t.Fatal("no multipart form was parsed")
	}
	if got := captured.form.Value["Path"]; len(got) != 1 || got[0] != "/data/tls/ca.pem" {
		t.Errorf("Path part = %v, want the destination path", got)
	}
	if len(captured.form.File["file"]) != 1 {
		t.Error(`the "file" part was not sent as a file part`)
	}
}

// TestUnit_EndpointDockerBrowsePutWithoutAVolume_SendsNoVolumeParameter
// proves the optional query parameter is omitted rather than sent empty:
// volumeID="" would ask Portainer to write into a volume named "", not into
// the host filesystem.
func TestUnit_EndpointDockerBrowsePutWithoutAVolume_SendsNoVolumeParameter(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusNoContent, nil)
	input := `{"id":2,"path":"/data/tls/ca.pem","file":"pem bytes"}`
	if _, err := endpointDockerBrowsePut(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("endpointDockerBrowsePut: %v", err)
	}
	if strings.Contains(captured.rawQuery, "volumeID") {
		t.Errorf("query = %q, want no volumeID at all when the caller supplied none", captured.rawQuery)
	}
}

// TestUnit_SnapshotContainerInspect_SendsTheContainerIDAsAString is the
// whole reason this handler is hand-written: the generated client's
// containerId argument is an int, and a Docker container ID never is.
func TestUnit_SnapshotContainerInspect_SendsTheContainerIDAsAString(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(`{"Id":"3f2a"}`))
	hex := "3f2a1b9c8d7e6f5a4b3c2d1e0f9a8b7c6d5e4f3a2b1c0d9e8f7a6b5c4d3e2f1a"
	input := `{"environmentId":1,"containerId":"` + hex + `"}`
	if _, err := snapshotContainerInspect(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("snapshotContainerInspect: %v", err)
	}
	want := "/api/docker/1/snapshot/containers/" + hex
	if captured.path != want {
		t.Errorf("path = %q, want %q", captured.path, want)
	}
}

// TestUnit_SnapshotContainerInspect_EscapesTheContainerIDIntoOneSegment
// guards the same thing internal/tools/docker's handlers guard: a
// caller-supplied identifier containing a slash must not splice extra
// segments into the route. portainer.Client.Do refuses a decoded ".."
// segment on its own, which is a different guard and does not cover this.
func TestUnit_SnapshotContainerInspect_EscapesTheContainerIDIntoOneSegment(t *testing.T) {
	t.Parallel()
	c, captured := capturingClient(t, http.StatusOK, []byte(`{}`))
	input := `{"environmentId":1,"containerId":"a/b"}`
	if _, err := snapshotContainerInspect(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("snapshotContainerInspect: %v", err)
	}
	// The assertion is on the escaped path, not r.URL.Path: net/url decodes
	// %2F back to "/" in Path, so a test reading Path would report the
	// escaping as absent whether or not it happened, and would keep passing
	// if url.PathEscape were dropped from the handler.
	if !strings.HasSuffix(captured.escapedPath, "/containers/a%2Fb") {
		t.Errorf("escaped path = %q: the container ID was not escaped into a single path segment", captured.escapedPath)
	}
}

// TestUnit_HandWrittenSpecs_MatchTheirGeneratedNeighbours checks the five
// declarations this domain writes by hand against the twenty-two the
// generator wrote, on the properties a hand-written entry is most likely to
// get wrong: a missing Input, a Handler that is not the operation's own, a
// mutating route not marked Mutating, and a Title or Description assigned
// directly instead of routed through toolutil.WithNarrative — which
// audit_spec_drift would then gate on as accidental drift.
func TestUnit_HandWrittenSpecs_MatchTheirGeneratedNeighbours(t *testing.T) {
	t.Parallel()
	byName := map[string]toolutil.ActionSpec{}
	for _, s := range Specs() {
		byName[s.Name] = s
	}
	for _, tc := range []struct {
		name     string
		mutating bool
		hasInput bool
	}{
		{"endpoints.list", false, true},
		{"endpoints.create", true, true},
		{"endpoints.update", true, true},
		{"endpoints.docker_browse_put", true, true},
		{"endpoints.snapshot_container_inspect", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			spec, ok := byName[tc.name]
			if !ok {
				t.Fatalf("%s is not declared by Specs()", tc.name)
			}
			if spec.Handler == nil {
				t.Error("Handler is nil")
			}
			if (spec.Input != nil) != tc.hasInput {
				t.Errorf("Input presence = %v, want %v", spec.Input != nil, tc.hasInput)
			}
			if spec.Mutating != tc.mutating {
				t.Errorf("Mutating = %v, want %v", spec.Mutating, tc.mutating)
			}
			if !spec.TitleOverridden || !spec.DescriptionOverridden {
				t.Error("narrative did not route through toolutil.WithNarrative; audit_spec_drift will read the prose as accidental drift")
			}
		})
	}
}

// TestUnit_EveryDeclaredAction_HasANarrative is the domain-wide version of
// the check above. narrative() has no default that silently returns the
// vendored wording, so an action added later without an entry would publish
// the specification's own text — which for this tag is empty or a restated
// summary on fourteen of twenty-seven operations.
func TestUnit_EveryDeclaredAction_HasANarrative(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if !s.TitleOverridden || !s.DescriptionOverridden {
			t.Errorf("%s (%s) has no narrative() entry", s.Name, s.OperationID)
		}
	}
}

// capturingJSONBodyClient answers with one environment and hands back the
// JSON body the handler sent, decoded.
//
// Distinct from capturingClient above, which parses a multipart form: the two
// settings tests care about the JSON body's own shape and nothing else, and a
// helper that decoded both would have to guess which.
func capturingJSONBodyClient(t *testing.T) (*portainer.Client, *map[string]any) {
	t.Helper()
	seen := map[string]any{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&seen)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(createdEnvironment))
	}))
	t.Cleanup(server.Close)
	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c, &seen
}

// TestUnit_EndpointSettingsUpdate_SendsBothEditionShapes is the guard on the
// one defect in this domain no refusal from the generator pointed at.
//
// Community Edition reads the ten security settings as top-level body
// fields and ignores a nested "securitySettings" object; Business Edition
// does the exact opposite. Both answer 200 either way, so the ignored case
// is invisible from the response — a caller asking to forbid privileged
// containers is told it worked. Sending both shapes is what makes one
// published field correct on either server; this is what proves the handler
// still does.
func TestUnit_EndpointSettingsUpdate_SendsBothEditionShapes(t *testing.T) {
	t.Parallel()
	c, seen := capturingJSONBodyClient(t)

	input := `{"id":1,"securitySettings":{"allowBindMountsForRegularUsers":false,"allowPrivilegedModeForRegularUsers":true}}`
	if _, err := endpointSettingsUpdate(context.Background(), c, json.RawMessage(input)); err != nil {
		t.Fatalf("endpointSettingsUpdate: %v", err)
	}

	nested, ok := (*seen)["securitySettings"].(map[string]any)
	if !ok {
		t.Fatalf("body carried no nested securitySettings object; Business Edition would ignore this call: %v", *seen)
	}
	if nested["allowBindMountsForRegularUsers"] != false || nested["allowPrivilegedModeForRegularUsers"] != true {
		t.Errorf("nested securitySettings = %v, want the caller's own values", nested)
	}
	if (*seen)["allowBindMountsForRegularUsers"] != false || (*seen)["allowPrivilegedModeForRegularUsers"] != true {
		t.Errorf("body carried no top-level copy of the security settings (%v); Community Edition would answer 200 and change nothing", *seen)
	}
}

// TestUnit_EndpointSettingsUpdate_WithoutSecuritySettings_SendsTheGeneratedBody
// is the other half: a caller who set no security settings must get exactly
// the body the generated handler would have sent, with no invented
// top-level keys, and with the path parameter kept out of it.
func TestUnit_EndpointSettingsUpdate_WithoutSecuritySettings_SendsTheGeneratedBody(t *testing.T) {
	t.Parallel()
	c, seen := capturingJSONBodyClient(t)

	if _, err := endpointSettingsUpdate(context.Background(), c,
		json.RawMessage(`{"id":3,"enableGPUManagement":true}`)); err != nil {
		t.Fatalf("endpointSettingsUpdate: %v", err)
	}
	if _, present := (*seen)["id"]; present {
		t.Error(`the body carried "id"; it is a path parameter and the specification declares no such body property`)
	}
	if (*seen)["enableGPUManagement"] != true {
		t.Errorf("enableGPUManagement = %v, want true", (*seen)["enableGPUManagement"])
	}
	if len(*seen) != 1 {
		t.Errorf("body = %v, want exactly the one field the caller sent", *seen)
	}
}
