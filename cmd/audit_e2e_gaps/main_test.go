package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestScan_FindsReferencesAcrossAllThreeSurfaceStyles is the scanner's core
// correctness test: each surface names an action differently, so a scanner
// that knows only one style silently reports the other two as unexercised.
func TestScan_FindsReferencesAcrossAllThreeSurfaceStyles(t *testing.T) {
	t.Parallel()
	source := `
		callAction[any](t, s, "individual", "tags.create", nil)
		session.CallTool(ctx, &mcp.CallToolParams{Name: "portainer_tags_delete"})
		session.CallTool(ctx, &mcp.CallToolParams{Name: "portainer_tags", Arguments: map[string]any{"action": "list"}})
		// tags.inspect is only mentioned in a comment and must not count
	`
	found := scanReferences([]byte(source))
	for _, want := range []string{"tags.create", "tags.delete", "tags.list"} {
		if !found[want] {
			t.Errorf("scan missed %q", want)
		}
	}
	if found["tags.inspect"] {
		t.Error("scan counted a reference that appears only in a comment")
	}
}

// TestReport_NamesTheUnreferencedCount asserts the report states the
// unreferenced count in an unambiguous form.
//
// The brief's version of this test checked strings.Contains(report, "2"),
// which a buggy report could satisfy by accident — the input catalog has 3
// actions, and a stray "2" could show up in a line number, a percentage, or
// any other decoration with no connection to the actual unreferenced count.
// Asserting the tighter "2 of 3" phrase ties the check to the specific
// referenced/unreferenced/total relationship the report is required to
// state, not to the bare digit.
func TestReport_NamesTheUnreferencedCount(t *testing.T) {
	t.Parallel()
	report := buildReport([]string{"a.one", "a.two", "a.three"}, map[string]bool{"a.one": true})
	if !strings.Contains(report, "2 of 3") {
		t.Errorf("report does not state how many actions are unreferenced:\n%s", report)
	}
}

func TestUnit_BuildReport_NoUnreferencedActions_StatesFullCoverage(t *testing.T) {
	t.Parallel()
	report := buildReport([]string{"a.one", "a.two"}, map[string]bool{"a.one": true, "a.two": true})
	if !strings.Contains(report, "0 of 2") {
		t.Errorf("report does not state zero unreferenced:\n%s", report)
	}
	if strings.Contains(report, "Actions with no e2e reference") {
		t.Errorf("report lists unreferenced actions when there are none:\n%s", report)
	}
}

func TestUnit_ScanSuiteDir_AbsentDirectory_ReturnsEmptyNotError(t *testing.T) {
	t.Parallel()
	missing := filepath.Join(t.TempDir(), "does-not-exist")

	found, err := scanSuiteDir(missing)
	if err != nil {
		t.Fatalf("scanSuiteDir(%q): unexpected error: %v", missing, err)
	}
	if len(found) != 0 {
		t.Errorf("scanSuiteDir(%q) = %v, want empty", missing, found)
	}
}

func TestUnit_ScanSuiteDir_EmptyDirectory_ReturnsEmptyNotError(t *testing.T) {
	t.Parallel()
	empty := t.TempDir()

	found, err := scanSuiteDir(empty)
	if err != nil {
		t.Fatalf("scanSuiteDir(%q): unexpected error: %v", empty, err)
	}
	if len(found) != 0 {
		t.Errorf("scanSuiteDir(%q) = %v, want empty", empty, found)
	}
}

func TestUnit_ScanSuiteDir_MatchingFile_MergesReferencesAcrossFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte(`"tags.list"`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b_test.go"), []byte(`"tags.create"`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "not-go.txt"), []byte(`"tags.delete"`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	found, err := scanSuiteDir(dir)
	if err != nil {
		t.Fatalf("scanSuiteDir(%q): unexpected error: %v", dir, err)
	}
	if !found["tags.list"] || !found["tags.create"] {
		t.Errorf("scanSuiteDir(%q) = %v, want tags.list and tags.create", dir, found)
	}
	if found["tags.delete"] {
		t.Errorf("scanSuiteDir(%q) counted a reference from a non-.go file", dir)
	}
}

// TestUnit_Run_AbsentSuiteDirectory_ReportsEveryCatalogActionUnreferenced
// covers the state this tool ships in: Tasks 6-8 of this plan have not yet
// created test/e2e/suite. The audit must report every real catalog action as
// unreferenced in that state, not crash and not silently report success.
//
// The expected total is computed through allCatalogActionNames, the same
// union-of-both-editions function run itself calls, rather than a single
// actioncatalog.Build(..., Options{Edition: edition.EE}) call: building for
// EE alone filters out system.upgrade (declared Edition: CE, resolves only in
// Community Edition's applicability index), which would silently make this
// test's own expectation reproduce the exact denominator bug this tool's
// fix exists to close, rather than catch a regression of it.
func TestUnit_Run_AbsentSuiteDirectory_ReportsEveryCatalogActionUnreferenced(t *testing.T) {
	t.Parallel()
	names, err := allCatalogActionNames()
	if err != nil {
		t.Fatalf("allCatalogActionNames: %v", err)
	}
	total := len(names)
	if total == 0 {
		t.Fatal("catalog has zero actions; test fixture assumption is stale")
	}

	missing := filepath.Join(t.TempDir(), "does-not-exist")

	var out strings.Builder
	if err := run(&out, missing); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}

	want := strconv.Itoa(total) + " of " + strconv.Itoa(total)
	if !strings.Contains(out.String(), want) {
		t.Errorf("report does not state %d unreferenced of %d total:\n%s", total, total, out.String())
	}
}

// TestUnit_AllCatalogActionNames_IncludesEditionExclusiveActionsFromBoth
// proves the denominator is the union of both editions, not one edition's
// filtered view of it. system.upgrade is declared Edition: CE and exists only
// in Community Edition's applicability index; system.update is declared
// Edition: EE and exists only in Business Edition's. A build for either
// edition alone would keep one and drop the other — this asserts both survive
// into the combined set, which is what "every action that could ever be
// registered" requires.
func TestUnit_AllCatalogActionNames_IncludesEditionExclusiveActionsFromBoth(t *testing.T) {
	t.Parallel()
	names, err := allCatalogActionNames()
	if err != nil {
		t.Fatalf("allCatalogActionNames: %v", err)
	}
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	for _, want := range []string{"system.upgrade", "system.update"} {
		if !found[want] {
			t.Errorf("allCatalogActionNames() = %v, want it to include %q", names, want)
		}
	}
}
