package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// --- static schema walk: responseCredentialFields/credentialShapedFieldPaths
// resolve a real operation's success response through the vendored spec's own
// $refs (the same resolver every other schema in this package goes through),
// never a fabricated fixture — the whole point is that this generator must
// agree with the real spec, not with a shape someone hand-built to match the
// code.

func TestUnit_ResponseCredentialFields_RegistryCreate_FlagsPasswordAndAccessToken(t *testing.T) {
	t.Parallel()
	// RegistryCreate's success response is portaineree.Registry, which — per
	// P2's review, the finding this whole file exists to make structural
	// (see credential.go's package doc) — carries Password and AccessToken
	// directly, plus a nested ManagementConfiguration with its own copies.
	op, _, res := realOperation(t, "RegistryCreate")
	fields, err := responseCredentialFields(op, res)
	if err != nil {
		t.Fatalf("responseCredentialFields() error = %v", err)
	}
	if len(fields) == 0 {
		t.Fatal("responseCredentialFields(RegistryCreate) = empty, want at least Password and AccessToken flagged")
	}
	joined := strings.Join(fields, " ")
	for _, want := range []string{"Password", "AccessToken"} {
		if !strings.Contains(joined, want) {
			t.Errorf("flagged fields = %v, want one of them to mention %q", fields, want)
		}
	}
}

func TestUnit_ResponseCredentialFields_TagList_FindsNone(t *testing.T) {
	t.Parallel()
	// TagList's success response is a bare array of portainer.Tag, which
	// carries nothing credential-shaped — checked by hand against every
	// field. This is the negative case: an operation whose response really
	// has no credential-shaped field must not be flagged, or every domain
	// would need a wrapper for every operation regardless of its response.
	op, _, res := realOperation(t, "TagList")
	fields, err := responseCredentialFields(op, res)
	if err != nil {
		t.Fatalf("responseCredentialFields() error = %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("responseCredentialFields(TagList) = %v, want none: portainer.Tag carries nothing credential-shaped", fields)
	}
}

// --- checkCredentialRedaction: the refusal itself, and the escape hatch that
// lifts it once the domain supplies the wrapper.

func TestUnit_CheckCredentialRedaction_NoWrapperDeclared_RefusesNamingOperationAndField(t *testing.T) {
	t.Parallel()
	op, _, res := realOperation(t, "RegistryCreate")
	_, err := checkCredentialRedaction(op, res, map[string]bool{}, false)
	if err == nil {
		t.Fatal("checkCredentialRedaction() = nil error, want a refusal: RegistryCreate's response can carry a credential and no wrapper is declared")
	}
	// The refusal must name both the operation and at least one offending
	// field — task-4b's own acceptance property ("a generator refusal naming
	// the operation and the field", not merely a generic failure) — and the
	// expected wrapper function name, so a domain author knows exactly what
	// to declare.
	for _, want := range []string{"RegistryCreate", "Password", "redactRegistryCreate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to mention %q", err, want)
		}
	}
}

func TestUnit_CheckCredentialRedaction_WrapperDeclared_ReturnsItsName(t *testing.T) {
	t.Parallel()
	op, _, res := realOperation(t, "RegistryCreate")
	funcNames := map[string]bool{"redactRegistryCreate": true}
	wrapper, err := checkCredentialRedaction(op, res, funcNames, false)
	if err != nil {
		t.Fatalf("checkCredentialRedaction() error = %v, want nil once the wrapper is declared", err)
	}
	if wrapper != "redactRegistryCreate" {
		t.Errorf("wrapper = %q, want %q", wrapper, "redactRegistryCreate")
	}
}

func TestUnit_CheckCredentialRedaction_ResponseHasNoCredential_NeedsNoWrapper(t *testing.T) {
	t.Parallel()
	// TagList needs no wrapper at all, with or without one declared: an empty
	// funcNames map must not be treated as "missing a required wrapper" for
	// an operation that never needed one.
	op, _, res := realOperation(t, "TagList")
	wrapper, err := checkCredentialRedaction(op, res, map[string]bool{}, false)
	if err != nil {
		t.Fatalf("checkCredentialRedaction() error = %v, want nil: TagList's response carries nothing credential-shaped", err)
	}
	if wrapper != "" {
		t.Errorf("wrapper = %q, want \"\": nothing to redact", wrapper)
	}
}

// --- execution proof: a compiling generated handler that calls the wrapper
// is necessary but not sufficient — this package's standing rule is that an
// assertion must discriminate a defect, and "compiles" does not discriminate
// a wrapper that is called from one that is declared but never reached (a
// typo routing the return value around it, say). credentialHarnessProgram
// below prints what the handler actually returns, so the wrapper's
// transformation of the fixture must be visible in that output or the test
// fails — mirroring handler_test.go's own executeGeneratedHandler, but for
// the response side rather than the request side.

// credentialHarnessProgramTemplate starts a stub that answers with
// stubResponseJSON, calls the generated handler, and prints its *return
// value* (not the request the stub observed, which handler_test.go's own
// harness already covers) as JSON.
const credentialHarnessProgramTemplate = `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

func main() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(%s))
	}))
	defer server.Close()

	c, err := portainer.New(&config.Config{URL: server.URL, Token: "test-token"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build client:", err)
		os.Exit(1)
	}

	out, err := %s(context.Background(), c, json.RawMessage(%s))
	if err != nil {
		fmt.Fprintln(os.Stderr, "handler error:", err)
		os.Exit(1)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal result:", err)
		os.Exit(1)
	}
	fmt.Println(string(encoded))
}
`

// executeGeneratedHandlerResult renders spec as a real, standalone package
// (the same discipline as handler_test.go's executeGeneratedHandler: it must
// live under cmd/gen_action_inputs for the internal-package visibility rule
// to allow importing internal/portainer/internal/config), alongside
// wrapperSrc — the domain's own hand-written redaction wrapper, exactly as a
// real domain package would declare it — compiles and runs it with `go run`
// against a stub answering stubResponseJSON, and returns the handler's own
// JSON-encoded return value.
func executeGeneratedHandlerResult(t *testing.T, spec handlerSpec, wrapperSrc, stubResponseJSON, inputJSON string) string {
	t.Helper()

	dir, err := os.MkdirTemp(".", ".gentest-cred-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	handlerSrc, err := renderActionsFile("main", "test-spec.json", []handlerSpec{spec}, nil, false)
	if err != nil {
		t.Fatalf("renderActionsFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "handler.go"), handlerSrc, 0o600); err != nil {
		t.Fatalf("write handler.go: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "wrapper.go"), []byte(wrapperSrc), 0o600); err != nil {
		t.Fatalf("write wrapper.go: %v", err)
	}

	harness := fmt.Sprintf(credentialHarnessProgramTemplate, fmt.Sprintf("%q", stubResponseJSON), spec.FuncName, fmt.Sprintf("%q", inputJSON))
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
	return strings.TrimSpace(stdout.String())
}

// redactRegistryCreateWrapperSrc is a hand-written redaction wrapper for
// RegistryCreate, exactly the shape checkCredentialRedaction requires a
// domain to declare before this generator will emit registryCreate at all —
// deliberately not the real registries.go implementation (calling into
// internal/tools/registries from a throwaway `package main` would defeat the
// internal-package visibility rule this file's own doc comment relies on),
// but the same contract: same name, same parameter type, same "redact and
// return any" shape.
const redactRegistryCreateWrapperSrc = `package main

import apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"

func redactRegistryCreate(r *apigen.PortainereeRegistry) any {
	if r == nil {
		return nil
	}
	scrubbed := *r
	scrubbed.Password = nil
	scrubbed.AccessToken = nil
	return &scrubbed
}
`

func TestUnit_GeneratedHandler_CredentialResponse_CallsRedactionWrapper(t *testing.T) {
	t.Parallel()
	op, doc, res := realOperation(t, "RegistryCreate")
	var nested []structSpec
	fields, pathOrder, err := assembleOperationFields(op, res, doc, "registryCreateInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	redactWith, err := checkCredentialRedaction(op, res, map[string]bool{"redactRegistryCreate": true}, false)
	if err != nil {
		t.Fatalf("checkCredentialRedaction() error = %v", err)
	}
	if redactWith != "redactRegistryCreate" {
		t.Fatalf("redactWith = %q, want redactRegistryCreate", redactWith)
	}
	spec, err := buildHandlerSpec("registries", op, fields, pathOrder, nested, "registryCreateInput", redactWith)
	if err != nil {
		t.Fatalf("buildHandlerSpec() error = %v", err)
	}
	if spec.RedactWith != "redactRegistryCreate" {
		t.Fatalf("handlerSpec.RedactWith = %q, want redactRegistryCreate", spec.RedactWith)
	}

	// The stub answers with a real credential (Password) and a value with no
	// credential-shaped name (Name), so the printed result must contain one
	// and not the other — proving the wrapper actually ran, not merely that
	// generation and compilation succeeded (a call site that silently
	// dropped the wrapper, or one wired to the wrong response field, would
	// still compile and would still print *something*).
	got := executeGeneratedHandlerResult(t, spec, redactRegistryCreateWrapperSrc,
		`{"Id":9,"Name":"my-registry","Password":"hunter2"}`, `{"name":"my-registry","url":"registry.example.com","type":3}`)
	if strings.Contains(got, "hunter2") {
		t.Errorf("generated handler result = %s, want the password redacted by redactRegistryCreate", got)
	}
	if !strings.Contains(got, "my-registry") {
		t.Errorf("generated handler result = %s, want the non-credential field preserved", got)
	}
}

// TestUnit_ResponseCredentialFields_AuthOperationsFlagTheirJWT is C2's
// evidence, taken against the real vendored specification rather than a
// fixture.
//
// AuthenticateUser (POST /auth) and ValidateOAuth (POST /auth/oauth/validate)
// both answer {"jwt": "..."} on success — a session bearer token. Before "jwt"
// joined toolutil.credentialFieldMarkers, neither operation flagged anything
// at all: none of "password", "token", "secret", "key", "credential" or
// "cert" appears in the property name, so a generated handler for either
// would have been emitted bare and handed the token straight to a model.
//
// The auth domain has no package under internal/tools yet — P3's later waves
// own that — so this deliberately checks the resolved response schema
// directly rather than building a domain, which is the earliest point the
// defect is observable.
func TestUnit_ResponseCredentialFields_AuthOperationsFlagTheirJWT(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{"AuthenticateUser", "ValidateOAuth"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, _, res := realOperation(t, operationID)
			fields, err := responseCredentialFields(op, res)
			if err != nil {
				t.Fatalf("responseCredentialFields(%s) error = %v", operationID, err)
			}
			var found bool
			for _, f := range fields {
				if strings.HasSuffix(f, ".jwt") {
					found = true
				}
			}
			if !found {
				t.Errorf("responseCredentialFields(%s) = %v, want it to flag the \"jwt\" session token", operationID, fields)
			}

			// And the guard that follows from it: with no redaction wrapper
			// declared, generation must refuse rather than emit a bare handler.
			if _, err := checkCredentialRedaction(op, res, map[string]bool{}, false); err == nil {
				t.Errorf("checkCredentialRedaction(%s) = nil error, want a refusal naming redact%s", operationID, operationID)
			}
		})
	}
}
