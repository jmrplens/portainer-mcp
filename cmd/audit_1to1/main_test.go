package main

import (
	"encoding/json"
	"fmt"
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

// syntheticEESpec builds a minimal OpenAPI document declaring one GET
// operation per id, all tagged "synthetic".
//
// Each id is written as-is, not lower-cased: exportedName only upper-cases
// the first rune, so any string that already starts with an upper-case
// letter — which is exactly the form every real toolutil.ActionSpec.
// OperationID uses — round-trips through it unchanged. That makes the ids
// passed in directly comparable to what allCatalogSpecs' real actions
// declare, with no separate raw-versus-exported translation to keep in sync
// in this file.
func syntheticEESpec(t *testing.T, ids []string) string {
	t.Helper()
	type operation struct {
		OperationID string   `json:"operationId"`
		Tags        []string `json:"tags"`
	}
	paths := make(map[string]map[string]operation, len(ids))
	for i, id := range ids {
		// Indexed by position, not by id: two different ids can otherwise
		// produce colliding path strings, and a Go map silently drops one of
		// two entries under the same key, quietly shrinking the fixture.
		paths[fmt.Sprintf("/synthetic/%d", i)] = map[string]operation{
			"get": {OperationID: id, Tags: []string{"synthetic"}},
		}
	}
	doc := struct {
		Paths map[string]map[string]operation `json:"paths"`
	}{Paths: paths}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal synthetic spec: %v", err)
	}
	return string(data)
}

// realCatalogOperationIDs returns every OperationID allCatalogSpecs declares,
// computed rather than hardcoded: run's tests below embed the real catalog
// (system, tags, registries — the same three packages audit_e2e_gaps's
// allSpecs enumerates), and hand-copying that list here would silently drift
// from it the moment a domain package changes, defeating the point of these
// being end-to-end plumbing tests rather than a restatement of audit_test.go's
// pure-logic fixtures.
func realCatalogOperationIDs(t *testing.T) []string {
	t.Helper()
	actions := allCatalogSpecs()
	ids := make([]string, 0, len(actions))
	for _, a := range actions {
		ids = append(ids, a.OperationID)
	}
	return ids
}

// TestUnit_Run_UncoveredOperation_ReturnsNonNilError is this command's whole
// point exercised end to end through run, the same function main wraps with
// os.Exit(1): the standing failure mode across this project's last two
// phases was a check that could not discriminate because its assertion
// matched something the fixture itself supplied. Here the discriminating
// check is on run's returned error, not on any string the report happens to
// contain — a report that prints "FAIL" and returns nil would satisfy a
// text-matching test and gate nothing, so passing here requires the actual
// error path, not the text.
//
// run calls the real allCatalogSpecs, so the fixture spec below declares an
// operation for every real action (satisfying the reverse "does this action
// resolve" check) plus one extra, WebsocketExec, that no action names — the
// operation run is expected to fail on.
func TestUnit_Run_UncoveredOperation_ReturnsNonNilError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ids := append(realCatalogOperationIDs(t), "WebsocketExec")
	writeFixture(t, dir, "ee.json", syntheticEESpec(t, ids))
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "allowlist.yaml", "[]\n")

	var out strings.Builder
	err := run(&out, dir, "ce.json", "ee.json", dir, "allowlist.yaml")
	if err == nil {
		t.Fatal("run() = nil error, want an error: WebsocketExec has no catalog action and is not allow-listed")
	}
	if !strings.Contains(out.String(), "WebsocketExec") {
		t.Errorf("run() report does not name the uncovered operation:\n%s", out.String())
	}
}

// TestUnit_Run_EveryOperationCoveredOrAllowListed_ReturnsNil is the positive
// control: once every operation the fixture spec declares is either matched
// by a real catalog action or allow-listed, run must return nil.
func TestUnit_Run_EveryOperationCoveredOrAllowListed_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ids := append(realCatalogOperationIDs(t), "WebsocketExec")
	writeFixture(t, dir, "ee.json", syntheticEESpec(t, ids))
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "allowlist.yaml", `
- operation_id: WebsocketExec
  reason: "MCP cannot carry a websocket upgrade."
  added: 2026-08-03
`)

	var out strings.Builder
	if err := run(&out, dir, "ce.json", "ee.json", dir, "allowlist.yaml"); err != nil {
		t.Fatalf("run() error = %v, want nil: every operation is covered or allow-listed\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "Allow-list entries: 1") {
		t.Errorf("run() report does not state the allow-list count:\n%s", out.String())
	}
}

// TestUnit_Run_MissingSpecFile_ReturnsError covers the plumbing failure
// mode: a misconfigured path must be reported, not panic or silently audit
// nothing.
func TestUnit_Run_MissingSpecFile_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out strings.Builder
	err := run(&out, dir, "does-not-exist.json", "does-not-exist.json", dir, "allowlist.yaml")
	if err == nil {
		t.Fatal("run() = nil error, want an error for a missing spec file")
	}
}

// TestUnit_ReadFileIn_ReadsFileWithinDir is the ordinary case: a plain
// filename inside dir must read normally.
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
// actually bites: a name that walks out of dir via ".." must be refused
// rather than silently followed.
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

// TestUnit_AllCatalogSpecs_IsNotEmpty guards against this list silently
// going stale if a domain package is ever removed without updating this
// file: an empty catalog would make every real run report full coverage of
// zero operations against a non-zero spec total, which fails loudly for an
// unrelated reason and would be confusing to debug from the audit's output
// alone.
func TestUnit_AllCatalogSpecs_IsNotEmpty(t *testing.T) {
	t.Parallel()
	if len(allCatalogSpecs()) == 0 {
		t.Fatal("allCatalogSpecs() = empty, want the declared system/tags/registries actions")
	}
}
