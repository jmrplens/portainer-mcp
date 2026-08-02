package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
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

// TestReport_NamesTheUnexercisedCount asserts the report states the
// unexercised count in an unambiguous form.
//
// The brief's version of this test checked strings.Contains(report, "2"),
// which a buggy report could satisfy by accident — the input catalog has 3
// actions, and a stray "2" could show up in a line number, a percentage, or
// any other decoration with no connection to the actual unexercised count.
// Asserting the tighter "2 of 3" phrase ties the check to the specific
// exercised/unexercised/total relationship the report is required to state,
// not to the bare digit.
func TestReport_NamesTheUnexercisedCount(t *testing.T) {
	t.Parallel()
	report := buildReport([]string{"a.one", "a.two", "a.three"}, map[string]bool{"a.one": true})
	if !strings.Contains(report, "2 of 3") {
		t.Errorf("report does not state how many actions are unexercised:\n%s", report)
	}
}

func TestUnit_BuildReport_NoUnexercisedActions_StatesFullCoverage(t *testing.T) {
	t.Parallel()
	report := buildReport([]string{"a.one", "a.two"}, map[string]bool{"a.one": true, "a.two": true})
	if !strings.Contains(report, "0 of 2") {
		t.Errorf("report does not state zero unexercised:\n%s", report)
	}
	if strings.Contains(report, "Actions with no e2e reference") {
		t.Errorf("report lists unexercised actions when there are none:\n%s", report)
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

// TestUnit_Run_AbsentSuiteDirectory_ReportsEveryCatalogActionUnexercised
// covers the state this tool ships in: Tasks 6-8 of this plan have not yet
// created test/e2e/suite. The audit must report every real catalog action as
// unexercised in that state, not crash and not silently report success.
func TestUnit_Run_AbsentSuiteDirectory_ReportsEveryCatalogActionUnexercised(t *testing.T) {
	t.Parallel()
	catalog, err := actioncatalog.Build(allSpecs(), actioncatalog.Options{Edition: edition.EE})
	if err != nil {
		t.Fatalf("actioncatalog.Build: %v", err)
	}
	total := len(catalog.Actions())
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
		t.Errorf("report does not state %d unexercised of %d total:\n%s", total, total, out.String())
	}
}
