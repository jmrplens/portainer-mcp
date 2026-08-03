package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file covers I6: the hand-written-override escape hatch used to bypass
// the credential-redaction guard entirely.
//
// run()'s domain loop checked overrideReason first and `continue`d on a match,
// so checkCredentialRedaction never ran for any operation a domain already
// covered by hand. That is precisely the shape of P2's original defect —
// registries' hand-written RegistryCreate/Inspect/Update returning Password
// and AccessToken unredacted — reproduced in the one code path built to make
// it structurally impossible. The guard now runs for every operation, and a
// hand-written handler states that it redacts by declaring the same
// redact<OperationID> function a generated one would call.

// overriddenDomainDir builds a temporary registries package whose only
// hand-written file declares an ActionSpec for every registries operation, so
// run() treats all of them as overridden and generates no handlers at all. The
// named functions are declared as empty funcs purely so scanHandOverrides sees
// them; nothing here is compiled, only parsed.
//
// Built as a composite literal of OperationID keys rather than by copying the
// real registries.go because the point is to control *exactly* which
// acknowledgements exist — copying the real file would bring its four genuine
// redact wrappers with it and the test could never observe their absence.
func overriddenDomainDir(t *testing.T, redactors []string) string {
	t.Helper()
	toolsDir := t.TempDir()
	dir := filepath.Join(toolsDir, "registries")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	var b strings.Builder
	b.WriteString("package registries\n\nvar _ = []struct{ OperationID string }{\n")
	for _, id := range []string{
		"RegistryList", "RegistryCreate", "RegistryInspect", "RegistryUpdate",
		"RegistryDelete", "RegistryPing", "RegistryConfigure",
		"EcrDeleteRepository", "EcrDeleteTags", "RepositoryTagsDelete",
	} {
		b.WriteString("\t{OperationID: \"" + id + "\"},\n")
	}
	b.WriteString("}\n\n")
	for _, name := range redactors {
		b.WriteString("func " + name + "() {}\n")
	}

	if err := os.WriteFile(filepath.Join(dir, "hand.go"), []byte(b.String()), 0o600); err != nil {
		t.Fatalf("write hand.go: %v", err)
	}
	return toolsDir
}

// TestUnit_Run_OverriddenCredentialOperationWithoutAcknowledgementIsRefused is
// the acceptance criterion: an overridden operation whose response is
// credential-shaped, with no statement that it redacts, must stop generation
// exactly as an ungenerated one already does.
func TestUnit_Run_OverriddenCredentialOperationWithoutAcknowledgementIsRefused(t *testing.T) {
	t.Parallel()
	toolsDir := overriddenDomainDir(t, nil)

	err := run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", toolsDir})
	if err == nil {
		t.Fatal("run() = nil error, want a refusal: registries' inspect/create/update/list are overridden, their responses carry a Password, and no redaction wrapper is declared")
	}
	// Operations are processed in sorted order, so the first refusal is
	// whichever credential-shaped operation sorts first — the assertion is on
	// the wrapper naming convention, not on which of the four is reported.
	if !strings.Contains(err.Error(), "redactRegistry") {
		t.Errorf("error = %q, want it to name the redact<OperationID> wrapper the domain must declare", err)
	}
	if !strings.Contains(err.Error(), "Password") {
		t.Errorf("error = %q, want it to name the credential-shaped field that triggered the refusal", err)
	}
	// The message must say why the hand-written case is different, or the
	// author's only clue is a function name with no explanation of what
	// declaring it is meant to assert.
	if !strings.Contains(err.Error(), "hand-written handler") {
		t.Errorf("error = %q, want it to explain that this operation is covered by hand-written code the generator cannot inspect", err)
	}
}

// TestUnit_Run_OverriddenCredentialOperationWithAcknowledgementIsAccepted is
// the discriminating other half. Without it, a guard that refused every
// overridden operation unconditionally — or refused for some unrelated reason
// — would pass the test above and look correct.
func TestUnit_Run_OverriddenCredentialOperationWithAcknowledgementIsAccepted(t *testing.T) {
	t.Parallel()
	toolsDir := overriddenDomainDir(t, []string{
		"redactRegistryList", "redactRegistryCreate",
		"redactRegistryInspect", "redactRegistryUpdate",
	})

	if err := run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", toolsDir}); err != nil {
		t.Fatalf("run() error = %v, want nil once every credential-shaped overridden operation declares its redaction wrapper", err)
	}
}

// TestUnit_Run_OverriddenOperationWithNoCredentialInItsResponseNeedsNothing
// keeps the requirement proportionate. registries' three genuinely
// hand-written operations (RegistryConfigure, EcrDeleteTags,
// RepositoryTagsDelete) all answer 204 with no body, so none of them is
// credential-shaped and none may be asked for a wrapper. A guard that demanded
// an acknowledgement from every overridden operation would make the escape
// hatch unusable.
func TestUnit_Run_OverriddenOperationWithNoCredentialInItsResponseNeedsNothing(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{"RegistryConfigure", "EcrDeleteTags", "RepositoryTagsDelete"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, _, res := realOperation(t, operationID)
			wrapper, err := checkCredentialRedaction(op, res, map[string]bool{}, true)
			if err != nil {
				t.Errorf("checkCredentialRedaction(%s, overridden) error = %v, want nil: a 204 response carries nothing to redact", operationID, err)
			}
			if wrapper != "" {
				t.Errorf("wrapper = %q, want \"\": nothing to redact means no wrapper is required", wrapper)
			}
		})
	}
}
