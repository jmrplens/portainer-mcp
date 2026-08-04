package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file is P3.3 task 3's own acceptance test: a refused operation must
// contribute nothing at all to its domain — no input struct, no nested
// type, no MinimumParams/EnumParams method, no redaction wrapper stub, no
// guard row — while an unrelated clean operation in the very same domain
// still contributes every one of those things. The two real stacks
// operations below are not invented for this test: they are the exact case
// this task's own brief measured on the real vendored specification.
//
// StackCreateDockerStandaloneFile's success response is credential-shaped
// (it needs a redactStackCreateDockerStandaloneFile wrapper, declared by
// hand below, exactly as checkCredentialRedaction requires before this
// generator will even consider the operation), but its request body is
// multipart/form-data: oapi-codegen therefore generates only a
// WithBodyWithResponse variant, never the plain
// StackCreateDockerStandaloneFileWithResponse this generator's handlers
// call (see handler.go's own package doc), so buildHandlerSpec refuses it.
// Before this task, assembleOperationFields had already appended its Input
// struct to allStructs (fields.go still computes it — it is only main.go's
// own commit that must now wait), and main.go's own guard-row append ran
// before buildHandlerSpec too, so a hand-declared wrapper for a permanently
// unsupported operation used to leave both an unused struct/methods and a
// redaction_test.go guard row naming a handler (stackCreateDockerStandaloneFile)
// this refusal prevented from ever being defined.
//
// StackGitRedeploy is the discriminating half a fixture with only the
// refused operation could not prove: its response is credential-shaped too
// (needing the identical wrapper convention), but it generates cleanly, and
// its Input struct carries both a Minimum-bound identifier ("id") and two
// enum-constrained fields ("repositoryAuthorizationType",
// "repositoryProvider") — so this one operation alone exercises every kind
// of contribution this task protects: struct, MinimumParams, EnumParams,
// handler and guard row. An implementation that emits nothing at all for
// every operation (the trap this brief names explicitly) would pass a test
// that only checked the refused operation's absence; asserting
// StackGitRedeploy's presence is what such an implementation cannot pass.
func TestUnit_Run_RefusedOperationContributesNothing_CleanOperationStillContributesEverything(t *testing.T) {
	t.Parallel()

	gitRedeployType := wrapperParamType(t, "StackGitRedeploy")

	dir := t.TempDir()
	stacksDir := filepath.Join(dir, "stacks")
	if err := os.MkdirAll(stacksDir, 0o750); err != nil {
		t.Fatalf("mkdir stacks: %v", err)
	}

	// Both wrapper stubs are declared by hand before generation, exactly as
	// checkCredentialRedaction requires: a domain author cannot even
	// scaffold a credential-bearing domain without declaring one first (see
	// this generator's own package doc), which is precisely why a refused
	// operation's wrapper being unused-but-declared is a real trap, not a
	// hypothetical one. redactStackCreateDockerStandaloneFile's parameter
	// type is never actually checked against anything (buildHandlerSpec
	// refuses before any call site referencing it is ever generated), so
	// `any` is enough; redactStackGitRedeploy's must match the real
	// generated client's success-field type exactly, or the handler this
	// generator does emit for it would fail to compile.
	hand := "package stacks\n\n" +
		"import apigen \"github.com/jmrplens/portainer-mcp/internal/portainer/gen\"\n\n" +
		"func redactStackGitRedeploy(v " + gitRedeployType + ") any { return v }\n\n" +
		"func redactStackCreateDockerStandaloneFile(v any) any { return v }\n"
	if err := os.WriteFile(filepath.Join(stacksDir, "hand.go"), []byte(hand), 0o600); err != nil {
		t.Fatalf("write hand.go: %v", err)
	}

	err := run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", dir})
	if err == nil {
		t.Fatal("run() = nil error, want a refusal: StackMigrate (field collision) and StackCreateDockerStandaloneFile/StackCreateDockerSwarmFile (no generated client method) all refuse regardless of any wrapper declared by hand")
	}
	if !strings.Contains(err.Error(), "refused generation") {
		t.Errorf("error = %q, want the deferred-refusal summary", err)
	}

	inputsSrc, ierr := os.ReadFile(filepath.Join(stacksDir, "inputs.go"))
	if ierr != nil {
		t.Fatalf("stacks/inputs.go was not written (%v): a domain with a permanently-unsupported operation must still generate its other, clean operations", ierr)
	}
	actionsSrc, aerr := os.ReadFile(filepath.Join(stacksDir, "actions.go"))
	if aerr != nil {
		t.Fatalf("stacks/actions.go was not written: %v", aerr)
	}
	guardSrc, gerr := os.ReadFile(filepath.Join(stacksDir, "redaction_test.go"))
	if gerr != nil {
		t.Fatalf("stacks/redaction_test.go was not written: %v", gerr)
	}

	// The clean operation contributes everything.
	for _, want := range []string{
		"type stackGitRedeployInput struct",
		"func (stackGitRedeployInput) MinimumParams() map[string]int",
		"func (stackGitRedeployInput) EnumParams() map[string][]any",
	} {
		if !strings.Contains(string(inputsSrc), want) {
			t.Errorf("stacks/inputs.go does not contain %q; the clean operation StackGitRedeploy must still contribute its struct and both methods:\n%s", want, inputsSrc)
		}
	}
	if !strings.Contains(string(actionsSrc), "func stackGitRedeploy(") {
		t.Errorf("stacks/actions.go does not declare stackGitRedeploy, the clean operation's generated handler:\n%s", actionsSrc)
	}
	if !strings.Contains(string(guardSrc), `operationID: "StackGitRedeploy"`) || !strings.Contains(string(guardSrc), "handler: stackGitRedeploy") {
		t.Errorf("stacks/redaction_test.go does not guard StackGitRedeploy's handler:\n%s", guardSrc)
	}

	// The refused operation contributes nothing at all: no input struct, no
	// MinimumParams/EnumParams method (it would have none of its own fields
	// to hang one off anyway, but the struct itself is the point), no
	// handler and no guard row — even though its wrapper was declared by
	// hand and is therefore, correctly, still just as unused as any other
	// stub written ahead of the handler that will eventually call it.
	for _, unwanted := range []string{"StackCreateDockerStandaloneFile", "stackCreateDockerStandaloneFile"} {
		if strings.Contains(string(inputsSrc), unwanted) {
			t.Errorf("stacks/inputs.go contains %q, want no input struct for a refused operation", unwanted)
		}
		if strings.Contains(string(actionsSrc), unwanted) {
			t.Errorf("stacks/actions.go contains %q, want no handler for a refused operation", unwanted)
		}
		if strings.Contains(string(guardSrc), unwanted) {
			t.Errorf("stacks/redaction_test.go contains %q, want no guard row for a refused operation even though its wrapper was declared by hand", unwanted)
		}
	}
}

// wrapperParamType derives the Go source type expression a hand-written
// redact<OperationID> wrapper must accept for the generated handler's own
// call site (renderHandlerFunc's `return %s(resp.%s), nil`) to compile: the
// real generated client method's own success-field type, reflected off
// apigen.ClientWithResponses exactly the way responseInfoFor already does,
// qualified with this package's own import alias for
// internal/portainer/gen ("apigen") rather than that package's own
// self-declared name ("portainerapi" — visible only through reflection,
// since Go's package clause and its importers' chosen alias are
// independent; see client.gen.go's own "package portainerapi").
func wrapperParamType(t *testing.T, operationID string) string {
	t.Helper()
	m, ok := clientMethodFor(operationID)
	if !ok {
		t.Fatalf("no generated client method %sWithResponse; this fixture needs an operation that has one", operationID)
	}
	respType := m.Type.Out(0).Elem()
	for i := 0; i < respType.NumField(); i++ {
		f := respType.Field(i)
		if code, ok := parseJSONFieldCode(f.Name); ok && code >= 200 && code < 300 {
			return strings.Replace(f.Type.String(), "portainerapi.", "apigen.", 1)
		}
	}
	t.Fatalf("%s's response declares no 2xx success field", operationID)
	return ""
}
