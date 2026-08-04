package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpecFixture(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
	return path
}

// TestUnit_ReadSpecFile_ReadsAnAbsolutePath is the ordinary case this
// command actually needs, unlike cmd/audit_1to1 and cmd/audit_spec_drift's
// directory-confined readFileIn: -before and -after name arbitrary files,
// typically outside this repository entirely (a /tmp scratch bundle), so an
// absolute path must work with no surrounding directory involved at all.
func TestUnit_ReadSpecFile_ReadsAnAbsolutePath(t *testing.T) {
	t.Parallel()
	path := writeSpecFixture(t, t.TempDir(), "spec.json", `{"paths": {}}`)
	data, err := readSpecFile(path)
	if err != nil {
		t.Fatalf("readSpecFile() error = %v", err)
	}
	if string(data) != `{"paths": {}}` {
		t.Errorf("readSpecFile() = %q, want the fixture's content", data)
	}
}

// TestUnit_ReadSpecFile_RefusesARelativePathThatEscapes proves the one guard
// this function does keep — filepath.Clean plus a check against a relative
// path climbing above the current directory via ".." — actually bites,
// mirroring test/e2e/harness.LoadEstate's identical precedent for an
// arbitrary, flag-supplied path.
func TestUnit_ReadSpecFile_RefusesARelativePathThatEscapes(t *testing.T) {
	t.Parallel()
	if _, err := readSpecFile("../../../../etc/passwd"); err == nil {
		t.Fatal("readSpecFile() error = nil, want an error for a relative path that escapes the current directory")
	}
}

// TestUnit_ReadSpecFile_EmptyPath_ReturnsError is the plumbing edge case: an
// empty -before or -after value must not silently resolve to the current
// directory.
func TestUnit_ReadSpecFile_EmptyPath_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := readSpecFile(""); err == nil {
		t.Fatal("readSpecFile() error = nil, want an error for an empty path")
	}
}

// TestUnit_Run_HumanReport_ContainsCountsAndFindings is the ordinary case,
// end to end through run() rather than buildReport directly: two small
// fixture documents differing in every way this tool classifies produce a
// report naming the added, removed and changed operations.
func TestUnit_Run_HumanReport_ContainsCountsAndFindings(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", deltaFixtureBefore)
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureAfter)

	var out strings.Builder
	if err := run(&out, before, after, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	report := out.String()
	for _, want := range []string{"Added:   2", "Removed: 1", "Changed: 2", "addedOp", "removedOp", "typeChangedOp", "descChangedOp"} {
		if !strings.Contains(report, want) {
			t.Errorf("run() report does not contain %q:\n%s", want, report)
		}
	}
	if strings.Contains(report, "unchangedOp") {
		t.Errorf("run() report contains \"unchangedOp\", want it absent (it did not change)")
	}
}

// TestUnit_Run_JSONFlag_EmitsParseableJSON proves the -json path (jsonOut
// parameter here) actually switches output format, and that what comes out
// parses back into the same counts the human report states.
func TestUnit_Run_JSONFlag_EmitsParseableJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", deltaFixtureBefore)
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureAfter)

	var out strings.Builder
	if err := run(&out, before, after, true); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	var decoded jsonReport
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("run() -json output does not parse as JSON: %v\noutput: %s", err, out.String())
	}
	if decoded.Counts.Added != 2 || decoded.Counts.Removed != 1 || decoded.Counts.Changed != 2 {
		t.Errorf("decoded.Counts = %+v, want {Added:2 Removed:1 Changed:2}", decoded.Counts)
	}
}

// TestUnit_Run_NeverReturnsErrorForARealDifference is the core behavioural
// contract this task states explicitly: this tool reports, it never gates.
// A candidate spec differing from the vendored one — the entire point of
// running it — must never be treated as a run failure the way
// cmd/audit_spec_drift's drift findings are.
func TestUnit_Run_NeverReturnsErrorForARealDifference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", deltaFixtureBefore)
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureAfter)

	var out strings.Builder
	if err := run(&out, before, after, false); err != nil {
		t.Fatalf("run() error = %v, want nil: a real difference between before and after is what this tool reports, never a failure", err)
	}
}

// TestUnit_Run_MissingBeforeFile_ReturnsError and its -after counterpart are
// the plumbing failure modes this tool does still fail on.
func TestUnit_Run_MissingBeforeFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureAfter)
	var out strings.Builder
	if err := run(&out, filepath.Join(dir, "does-not-exist.json"), after, false); err == nil {
		t.Fatal("run() error = nil, want an error for a missing -before file")
	}
}

func TestUnit_Run_MissingAfterFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", deltaFixtureBefore)
	var out strings.Builder
	if err := run(&out, before, filepath.Join(dir, "does-not-exist.json"), false); err == nil {
		t.Fatal("run() error = nil, want an error for a missing -after file")
	}
}

// TestUnit_Run_MalformedSpec_ReturnsError proves a decode failure on either
// side is still surfaced as an error, not swallowed into an empty report.
func TestUnit_Run_MalformedSpec_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", "not json")
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureAfter)
	var out strings.Builder
	if err := run(&out, before, after, false); err == nil {
		t.Fatal("run() error = nil, want an error for a malformed -before document")
	}
}

// TestUnit_Run_IdenticalDocuments_ReportsNoDifference is the clean-run case
// end to end: comparing a document against itself produces zero counts and
// a plain "no difference" report, mirroring a patch release where the
// research in plan/research/version-delta-analysis.md found 0-4 changed
// operations.
func TestUnit_Run_IdenticalDocuments_ReportsNoDifference(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	before := writeSpecFixture(t, dir, "before.json", deltaFixtureBefore)
	after := writeSpecFixture(t, dir, "after.json", deltaFixtureBefore)

	var out strings.Builder
	if err := run(&out, before, after, false); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	if !strings.Contains(out.String(), "No difference") {
		t.Errorf("run() report = %q, want it to say plainly there is no difference", out.String())
	}
}
