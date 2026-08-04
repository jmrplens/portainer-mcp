package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// realSpecsDir and realAllowListDir point at this repository's own
// production inputs — the exact files `make audit-spec-drift` reads —
// resolved relative to this test file the same way
// internal/specdiff/compare_test.go's realShapeFromVendoredSpec does.
const (
	realSpecsDir     = "../../api/specs"
	realAllowListDir = "../../api"
)

// TestUnit_Run_RealCatalogAgainstRealSpecs_ReturnsNil is Step 6 of this
// task: run the audit against the real tree and record the result. This is
// the exact function main wraps with os.Exit(1), called with this
// repository's real vendored specs, real allow-list and (via
// internal/wiring.AllSpecs, called inside run/auditDrift) the real declared
// catalog. A nil error here is what "make audit-spec-drift holds clean"
// means; a non-nil error would be a real finding about the generator or the
// hand-written pilots, not something to silence by loosening this test.
func TestUnit_Run_RealCatalogAgainstRealSpecs_ReturnsNil(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	err := run(&out, realSpecsDir, "ce-2.44.0.json", "ee-2.44.0.json", realAllowListDir, "spec-drift-allowlist.yaml")
	if err != nil {
		t.Fatalf("run() error = %v, want nil: the real catalog is expected to match the vendored spec it was generated from\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "No drift") {
		t.Errorf("run() report = %q, want it to say plainly that no drift was found", out.String())
	}
}

// mutateEESpecTagDeleteIDToString loads the real vendored Business Edition
// specification and returns a byte-identical copy except for exactly one
// change: TagDelete's "id" path parameter's type, flipped from "integer" to
// "string". This is a real, if synthetic, instance of the exact defect this
// whole task exists to catch (internal/specdiff's own worked example,
// EndpointList's order/edgeStackStatus type swap between published Portainer
// versions) — introduced deliberately here, against the one operation this
// repository's real catalog actually declares (tags.delete), rather than a
// hand-built fixture spec that might happen to already agree with the
// catalog by construction.
func mutateEESpecTagDeleteIDToString(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(realSpecsDir, "ee-2.44.0.json"))
	if err != nil {
		t.Fatalf("read real ee spec: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatalf("decode real ee spec: %v", err)
	}
	paths, _ := doc["paths"].(map[string]any)
	tagByID, _ := paths["/tags/{id}"].(map[string]any)
	del, _ := tagByID["delete"].(map[string]any)
	params, _ := del["parameters"].([]any)
	if len(params) == 0 {
		t.Fatal("real ee spec: /tags/{id} delete has no parameters; this test's premise (TagDelete's \"id\" path parameter) no longer holds")
	}
	idParam, _ := params[0].(map[string]any)
	if idParam["name"] != "id" {
		t.Fatalf("real ee spec: /tags/{id} delete's first parameter is %v, want \"id\"", idParam["name"])
	}
	schema, _ := idParam["schema"].(map[string]any)
	if schema["type"] != "integer" {
		t.Fatalf("real ee spec: TagDelete's \"id\" parameter type is %v, want \"integer\" (this test's baseline assumption)", schema["type"])
	}
	schema["type"] = "string"

	mutated, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("re-encode mutated ee spec: %v", err)
	}
	return mutated
}

// TestUnit_Run_RealCatalogTypeDriftAgainstMutatedSpec_ReturnsNonNilError is
// Step 5 of this task: prove this audit gates by asserting on run's returned
// error — the value main wraps with os.Exit(1) — never on a word the report
// happens to contain. A command that printed "DRIFT" and returned nil would
// satisfy a text-matching test and gate nothing; this test would not notice
// that mistake if it only checked out.String(), so it checks err instead.
//
// The vendored spec fed to run here is not a hand-built fixture: it is this
// repository's real ee-2.44.0.json with exactly one byte-level change
// (mutateEESpecTagDeleteIDToString), so the drift this test exercises is the
// real catalog's real tags.delete action compared against a real
// specification that has genuinely moved out from under it.
func TestUnit_Run_RealCatalogTypeDriftAgainstMutatedSpec_ReturnsNonNilError(t *testing.T) {
	t.Parallel()
	mutatedEE := mutateEESpecTagDeleteIDToString(t)

	realCE, err := os.ReadFile(filepath.Join(realSpecsDir, "ce-2.44.0.json"))
	if err != nil {
		t.Fatalf("read real ce spec: %v", err)
	}
	realAllowList, err := os.ReadFile(filepath.Join(realAllowListDir, "spec-drift-allowlist.yaml"))
	if err != nil {
		t.Fatalf("read real allow-list: %v", err)
	}

	dir := t.TempDir()
	writeFixture(t, dir, "ee-2.44.0.json", string(mutatedEE))
	writeFixture(t, dir, "ce-2.44.0.json", string(realCE))
	writeFixture(t, dir, "spec-drift-allowlist.yaml", string(realAllowList))

	var out strings.Builder
	err = run(&out, dir, "ce-2.44.0.json", "ee-2.44.0.json", dir, "spec-drift-allowlist.yaml")
	if err == nil {
		t.Fatalf("run() error = nil, want an error: TagDelete's \"id\" was mutated integer -> string in the spec fed to run\n%s", out.String())
	}
	// The report is still checked, but only as a secondary, human-facing
	// property — the discriminating assertion above is on err.
	if !strings.Contains(out.String(), "TagDelete") {
		t.Errorf("run() report does not name the drifted operation:\n%s", out.String())
	}
}

// TestUnit_Run_MissingSpecFile_ReturnsError is the plumbing failure mode.
func TestUnit_Run_MissingSpecFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFixture(t, dir, "allowlist.yaml", "[]\n")
	var out strings.Builder
	err := run(&out, dir, "does-not-exist.json", "does-not-exist.json", dir, "allowlist.yaml")
	if err == nil {
		t.Fatal("run() error = nil, want an error for a missing spec file")
	}
}

// TestUnit_Run_MissingAllowListFile_ReturnsError covers the other input.
func TestUnit_Run_MissingAllowListFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "ee.json", `{"paths": {}}`)
	var out strings.Builder
	err := run(&out, dir, "ce.json", "ee.json", dir, "does-not-exist.yaml")
	if err == nil {
		t.Fatal("run() error = nil, want an error for a missing allow-list file")
	}
}

// TestUnit_Run_CanaryCannotBeBypassed_StillRunsAgainstCleanTree is a
// sanity check that the mandatory canary (verifyCanary(specdiff.Compare),
// called first inside run) does not itself block an otherwise-clean run —
// see TestUnit_VerifyCanary_RealComparator_Passes in audit_test.go for the
// unit-level proof; this just confirms run() actually reaches the report
// rather than stopping at the canary for the real tree.
func TestUnit_Run_CanaryCannotBeBypassed_StillRunsAgainstCleanTree(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	if err := run(&out, realSpecsDir, "ce-2.44.0.json", "ee-2.44.0.json", realAllowListDir, "spec-drift-allowlist.yaml"); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Canary self-test: passed") {
		t.Errorf("run() report = %q, want it to record that the canary passed", out.String())
	}
}

// TestUnit_ReadFileIn_ReadsFileWithinDir is the ordinary case, mirroring
// cmd/audit_1to1's identical test for the identical helper.
func TestUnit_ReadFileIn_ReadsFileWithinDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFixture(t, dir, "spec.json", `{"paths": {}}`)

	data, err := readFileIn(dir, "spec.json")
	if err != nil {
		t.Fatalf("readFileIn() error = %v", err)
	}
	if string(data) != `{"paths": {}}` {
		t.Errorf("readFileIn() = %q, want the fixture's content", data)
	}
}

// TestUnit_ReadFileIn_RefusesToEscapeDir proves the confinement check
// actually bites.
func TestUnit_ReadFileIn_RefusesToEscapeDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	rel, err := filepath.Rel(dir, outside)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if _, err := readFileIn(dir, rel); err == nil {
		t.Fatal("readFileIn() = nil error, want it to refuse a name that escapes dir")
	}
}
