package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// captureStderr redirects the package-global os.Stderr for the duration of
// fn and returns everything written to it. run()'s own per-operation refusal
// detail — which domain, which operation, which reason — is printed only to
// stderr (see main.go's refusals): the error run() returns carries just the
// final count-and-domain-count summary, by design, so the detailed report
// still reaches a human even though the run only fails once, at the very
// end. A test asserting on that per-operation detail has to read it here,
// not off err.Error().
//
// Reads through a pipe on a separate goroutine so a write inside fn cannot
// block on a full pipe buffer waiting for a reader that only starts once fn
// has already returned.
//
// Not safe to call from a test that also calls t.Parallel(): it mutates the
// package-global os.Stderr for as long as fn runs, and a concurrently
// running parallel test in this package writing to os.Stderr through the
// same variable would have its own output captured here instead (or vice
// versa). Every caller of this helper in this package therefore omits
// t.Parallel(), which is also what keeps it safe: Go only actually runs
// t.Parallel() tests concurrently with each other, after every non-parallel
// top-level test in the package has already finished.
func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe(): %v", err)
	}
	original := os.Stderr
	os.Stderr = w

	captured := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		captured <- buf.String()
	}()

	fn()

	os.Stderr = original
	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	return <-captured
}

// TestDomainOperations_KnownDomain_AggregatesAcrossItsTags guards the normal
// case: a domain covering more than one tag (as "cloud" does in the real
// table, covering only "cloud_credentials", but tested here with a synthetic
// two-tag domain to prove aggregation itself, independent of the real
// table's current shape) gets every operation from every tag it names,
// merged and sorted by OperationID.
func TestDomainOperations_KnownDomain_AggregatesAcrossItsTags(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"cloud": {"cloud_credentials", "kaas"}}
	byTag := map[string][]operation{
		"cloud_credentials": {{OperationID: "SharedGitCreate"}, {OperationID: "CloudCredsGetByID"}},
		"kaas":              {{OperationID: "Upgrade"}},
	}

	ops, err := domainOperations("cloud", domainTags, byTag)
	if err != nil {
		t.Fatalf("domainOperations() error = %v", err)
	}
	if len(ops) != 3 {
		t.Fatalf("domainOperations() = %v, want 3 operations aggregated across both tags", ops)
	}
	var ids []string
	for _, op := range ops {
		ids = append(ids, op.OperationID)
	}
	if got, want := strings.Join(ids, ","), "CloudCredsGetByID,SharedGitCreate,Upgrade"; got != want {
		t.Errorf("operation order = %q, want %q (sorted by OperationID)", got, want)
	}
}

// TestDomainOperations_DomainDirectoryHasNoEntry_ReturnsError is C1's core
// mutation: a domain directory this table does not name must be a hard
// error. Before the fix, main.go's loop did `ops := byDomain[domainName];
// if len(ops) == 0 { continue }` — indistinguishable, to that code, from a
// domain that legitimately has zero fields to generate for. Proven for real
// against a directory named "edgestacks": the actual tag for those
// operations is "edge_stacks" (see toolutil.DomainTags), so a directory
// spelled without the underscore has no entry and must fail loudly instead
// of silently producing nothing.
func TestDomainOperations_DomainDirectoryHasNoEntry_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"edge_stacks": {"edge_stacks"}}
	byTag := map[string][]operation{
		"edge_stacks": {{OperationID: "EdgeStackList"}},
	}

	_, err := domainOperations("edgestacks", domainTags, byTag)
	if err == nil {
		t.Fatal("domainOperations() = nil error, want one: \"edgestacks\" has no entry in domainTags")
	}
	if !strings.Contains(err.Error(), "edgestacks") {
		t.Errorf("error = %q, want it to name the unmapped directory", err)
	}
}

// TestCheckDomainTagsCoverSpec_ValidTable_Succeeds is the baseline: every tag
// domainTags names has operations, and every tag with operations is named.
func TestCheckDomainTagsCoverSpec_ValidTable_Succeeds(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{
		"tags":   {"tags"},
		"backup": {"backup"},
	}
	byTag := map[string][]operation{
		"tags":   {{OperationID: "TagList"}},
		"backup": {{OperationID: "Backup"}, {OperationID: "Restore"}},
	}
	if err := checkDomainTagsCoverSpec(domainTags, byTag); err != nil {
		t.Fatalf("checkDomainTagsCoverSpec() error = %v, want nil for a fully-covered table", err)
	}
}

// TestCheckDomainTagsCoverSpec_TagWithOperationsHasNoEntry_ReturnsError is
// C1's reverse direction: a tag the vendored spec has real operations under,
// but that no domain in the table claims, must fail loudly. This is exactly
// how 127 operations across 12 tags went unreachable before this table
// existed — nothing ever noticed a tag the table had simply never been told
// about.
func TestCheckDomainTagsCoverSpec_TagWithOperationsHasNoEntry_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"tags": {"tags"}}
	byTag := map[string][]operation{
		"tags":   {{OperationID: "TagList"}},
		"backup": {{OperationID: "Backup"}}, // no domain claims "backup"
	}
	err := checkDomainTagsCoverSpec(domainTags, byTag)
	if err == nil {
		t.Fatal("checkDomainTagsCoverSpec() = nil error, want one: \"backup\" has operations but no domain entry")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Errorf("error = %q, want it to name the uncovered tag", err)
	}
}

// TestCheckDomainTagsCoverSpec_TableNamesATagAbsentFromSpec_ReturnsError
// guards a typo in the table itself: a domain naming a tag that has zero
// operations in the vendored spec would otherwise resolve to zero
// operations for that domain and be indistinguishable from a domain that
// legitimately generates nothing — reintroducing this table's own defect
// one level up, inside the table.
func TestCheckDomainTagsCoverSpec_TableNamesATagAbsentFromSpec_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"backup": {"backupp"}} // typo
	byTag := map[string][]operation{
		"backup": {{OperationID: "Backup"}},
	}
	err := checkDomainTagsCoverSpec(domainTags, byTag)
	if err == nil {
		t.Fatal("checkDomainTagsCoverSpec() = nil error, want one: \"backupp\" does not exist in the vendored spec")
	}
	if !strings.Contains(err.Error(), "backupp") {
		t.Errorf("error = %q, want it to name the typo'd tag", err)
	}
}

// TestUnit_Run_CESpecPathEqualsEESpecPath_RefusesToClassifyEverythingAsCE
// proves the guard on the CE spec derivation: -spec's filename carrying no
// "ee-" substring for strings.Replace to swap must refuse rather than
// silently resolving the CE spec path to *specPath itself. Before this
// guard, that would load the EE document a second time under the "CE"
// label, populate ceOperationIDs with every EE operationId, and make
// editionOf classify every EE-only action as CE — the exact failure
// editionConstName already refuses at render time (see emit.go), just one
// step earlier and unguarded.
func TestUnit_Run_CESpecPathEqualsEESpecPath_RefusesToClassifyEverythingAsCE(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	specPath := filepath.Join(dir, "spec-with-no-edition-prefix.json")
	if err := os.WriteFile(specPath, []byte(`{"paths": {}}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	err := run([]string{"-spec", specPath})
	if err == nil {
		t.Fatal("run() error = nil, want a refusal: -spec's filename carries no \"ee-\" for the CE derivation to swap")
	}
	if !strings.Contains(err.Error(), "ee-") {
		t.Errorf("error = %q, want it to explain the missing \"ee-\" substring", err)
	}
}

// TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec cross-checks
// toolutil.DomainTags — the real, curated table — against the real vendored
// EE specification, the superset gen-action-inputs actually runs against
// (see Makefile's gen-action-inputs target). This is what proves the table
// was genuinely populated for all 46 tags, not merely asserted to have 46
// entries in internal/toolutil's own unit test.
func TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec(t *testing.T) {
	t.Parallel()
	if err := toolutil.ValidateDomainTags(toolutil.DomainTags); err != nil {
		t.Fatalf("toolutil.ValidateDomainTags(toolutil.DomainTags) error = %v", err)
	}

	_, paths, err := loadDocument("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}

	if err := checkDomainTagsCoverSpec(toolutil.DomainTags, byTag); err != nil {
		t.Fatalf("checkDomainTagsCoverSpec(toolutil.DomainTags, <real spec>) error = %v, want the curated table to fully cover the vendored spec's tags", err)
	}
}

// TestUnit_Run_TwoDifferentRefusalsInOneDomain_BothReported_AndLaterDomainStillWritten
// is Task 1's own acceptance test: "stacks" (freshly created, zero
// hand-written files — exactly what a domain looks like the moment before
// its first scaffold) has two operations that refuse for two genuinely
// different reasons in the real, unmodified vendored spec, with no fixture
// mutation needed to produce either:
//
//   - StackMigrate refuses inside assembleOperationFields: its query
//     parameter "endpointId" and its body's "EndpointID" property both render
//     the JSON name "endpointId" (a field collision) — the exact real case
//     main.go's own doc comment names, and the reason a hand-written override
//     cannot suppress it (assembleOperationFields runs before overrideReason).
//   - StackList refuses inside checkCredentialRedaction: its success response
//     nests a GitConfig.Authentication.Password, and this freshly created
//     domain has declared no redactStackList wrapper yet.
//
// The trap this guards against (this project's own standing warning, caught
// by seven implementers in a row before it reached this one): a test with
// only one refusable operation cannot tell "collects it" apart from "aborts
// on it" — aborting on the only refusal in a domain looks, from a report
// with one entry, identical to correctly collecting it. Asserting both
// StackMigrate and StackList are named is what a leftover abort-on-first
// implementation cannot pass, since it would report exactly one of the two
// (whichever assembleOperationFields or checkCredentialRedaction reaches
// first) and return before the other is ever checked.
//
// "tags" — real, already hand-written, sorting after "stacks" — is the
// ordering-hazard half: before this fix, any refusal in "stacks" aborted
// run() immediately, so "tags" (and every other domain sorting after
// "stacks") was never even attempted. Asserting it is still fully written is
// what makes that regression concrete rather than assumed.
func TestUnit_Run_TwoDifferentRefusalsInOneDomain_BothReported_AndLaterDomainStillWritten(t *testing.T) {
	toolsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(toolsDir, "stacks"), 0o750); err != nil {
		t.Fatalf("mkdir stacks: %v", err)
	}
	freshDomainDir(t, toolsDir, "tags")

	var runErr error
	stderr := captureStderr(t, func() {
		runErr = run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", toolsDir})
	})

	if runErr == nil {
		t.Fatal("run() = nil error, want a refusal: stacks has zero hand-written files, and StackMigrate's field collision refuses regardless of any override")
	}
	if !strings.Contains(runErr.Error(), "refused generation") {
		t.Errorf("error = %q, want the deferred-refusal summary", runErr)
	}

	if !strings.Contains(stderr, "StackMigrate") || !strings.Contains(stderr, "contributed by both") {
		t.Errorf("stderr report does not name StackMigrate's field collision (\"endpointId\" contributed by both query parameter and body):\n%s", stderr)
	}
	if !strings.Contains(stderr, "StackList") || !strings.Contains(stderr, "credential-shaped") {
		t.Errorf("stderr report does not also name StackList's unrelated refusal (a credential-shaped response field with no redaction wrapper declared):\n%s", stderr)
	}

	for _, name := range []string{"actions.go", "inputs.go"} {
		if _, statErr := os.Stat(filepath.Join(toolsDir, "stacks", name)); !os.IsNotExist(statErr) {
			t.Errorf("stacks/%s exists (stat error %v), want it unwritten: a domain with any refused operation must not be trusted to write", name, statErr)
		}
	}
	for _, name := range []string{"actions.go", "inputs.go"} {
		if _, statErr := os.Stat(filepath.Join(toolsDir, "tags", name)); statErr != nil {
			t.Errorf("tags/%s was not written (%v); stacks' refusals must not block an unrelated, later-sorting domain's regeneration", name, statErr)
		}
	}
}
