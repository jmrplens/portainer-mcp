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
func captureStderr(t *testing.T, fn func()) (out string) {
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

	// Deferred rather than run only after fn's normal return: if fn panics,
	// or a future caller's fn calls t.Fatalf (which unwinds the goroutine via
	// runtime.Goexit, not a panic this function's own stack could recover),
	// code after a plain fn() call below would never run at all. os.Stderr
	// would then stay pointed at the pipe writer for the rest of the package
	// run, the drain goroutine above would never see w closed and so never
	// return, and whatever fn had already written — including any panic
	// output — would sit unread in the pipe forever.
	defer func() {
		os.Stderr = original
		if cerr := w.Close(); cerr != nil {
			t.Errorf("close pipe writer: %v", cerr)
		}
		out = <-captured
	}()

	fn()
	return
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
// is Task 1's own acceptance test, extended by P3.3 task 3 to also cover the
// per-*operation* (not per-domain) granularity that task introduced:
// "stacks" has two operations that refuse for two genuinely different
// reasons in the real, unmodified vendored spec:
//
//   - StackMigrate refuses inside buildHandlerSpec: its query parameter
//     "endpointId" and its body's "EndpointID" property both render the JSON
//     name "endpointId", so internal/specnaming publishes the parameter as
//     "endpointIdQuery" — a name the generated client's own
//     apigen.StackMigrateParams is not tagged with, which makes the
//     mechanical handler's JSON round trip unable to distribute it (see
//     buildHandlerSpec's refusal for what it would otherwise send).
//   - StackList refuses inside checkCredentialRedaction: its success response
//     nests a GitConfig.Authentication.Password, and this domain has declared
//     no redactStackList wrapper.
//
// The fixture declares exactly one hand-written function, redactStackMigrate,
// for the sole purpose of getting StackMigrate *past* the credential check so
// that the two refusals stay two different kinds. Before internal/specnaming,
// StackMigrate refused earlier, in assembleOperationFields, and never reached
// the credential check at all; now it does, and without that stub both of
// this test'"'"'s refusals would be credential-shaped and the "two different
// reasons" premise would be quietly false while every assertion still
// passed.
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
// ordering-hazard half: before Task 1, any refusal in "stacks" aborted run()
// immediately, so "tags" (and every other domain sorting after "stacks") was
// never even attempted. Asserting it is still fully written is what makes
// that regression concrete rather than assumed.
//
// Task 3's own half: since "stacks" has 25 operations and only these two are
// refused (plus every other credential-needing one, for the identical
// missing-wrapper reason StackList is refused for — this fixture declares no
// wrapper stubs at all), stacks/inputs.go and stacks/actions.go must still
// exist, containing every one of the domain's clean operations
// (stackDelete, a real generated handler with no credential and no wire
// issue, is asserted by name) — and must not contain anything StackMigrate
// or StackList would have contributed (their mechanical Input struct names,
// stackMigrateInput/stackListInput, or their mechanical handler names,
// stackMigrate/stackList). A leftover whole-domain-poisoning implementation
// cannot pass this: it leaves the domain entirely unwritten instead.
func TestUnit_Run_TwoDifferentRefusalsInOneDomain_BothReported_AndLaterDomainStillWritten(t *testing.T) {
	toolsDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(toolsDir, "stacks"), 0o750); err != nil {
		t.Fatalf("mkdir stacks: %v", err)
	}
	// See this test'"'"'s doc comment: the one hand-written declaration this
	// domain gets, so StackMigrate reaches buildHandlerSpec instead of
	// stopping at the credential check the way its 14 siblings do.
	handWritten := "package stacks\n\nfunc redactStackMigrate(v any) any { return v }\n"
	if err := os.WriteFile(filepath.Join(toolsDir, "stacks", "stacks.go"), []byte(handWritten), 0o600); err != nil {
		t.Fatalf("write stacks/stacks.go: %v", err)
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

	wantStackMigrateRefusal := "domain stacks: POST /stacks/{id}/migrate (operationId StackMigrate): StackMigrate: query parameter \"endpointId\" is published as \"endpointIdQuery\""
	if !strings.Contains(stderr, wantStackMigrateRefusal) {
		t.Errorf("stderr report does not contain StackMigrate's refusal line %q:\n%s", wantStackMigrateRefusal, stderr)
	}
	// Anchored on the whole refusal line, not two independent substrings: a
	// bare `strings.Contains(stderr, "StackList")` also matches
	// "EdgeStackList" in the unrelated verb-derived-flags diagnostic above,
	// and `strings.Contains(stderr, "credential-shaped")` also matches any of
	// the 13 *other* credential-shaped refusals this same run reports for
	// stacks' other Git-config-carrying operations (StackInspect,
	// StackCreateDockerStandalone*, StackUpdate*, StackStart, StackStop,
	// ...). Both conjuncts of the old check were individually satisfiable by
	// those unrelated lines, so a mutation that silently dropped StackList's
	// own refusal (confirmed: swallowing it in run() left this test passing)
	// went uncaught. Asserting the exact, single line StackList's refusal
	// renders as is the only check that fails when this operation stops
	// being refused.
	wantStackListRefusal := "domain stacks: GET /stacks (operationId StackList): StackList: success response can carry credential-shaped field(s) [StackList[].GitConfig.Authentication.Password]; declare func redactStackList(<the response's success-field type>) any in this domain's own hand-written file before generating (see toolutil.IsCredentialShapedName and internal/tools/registries's redact/redactList for the pattern)"
	if !strings.Contains(stderr, wantStackListRefusal) {
		t.Errorf("stderr report does not contain StackList's exact refusal line %q:\n%s", wantStackListRefusal, stderr)
	}

	for _, name := range []string{"actions.go", "inputs.go"} {
		if _, statErr := os.Stat(filepath.Join(toolsDir, "stacks", name)); statErr != nil {
			t.Errorf("stacks/%s was not written (%v); since P3.3 task 3 a refusal costs only the refused operation, not the whole domain, and stacks has clean operations besides StackMigrate/StackList", name, statErr)
		}
	}
	actionsSrc, err := os.ReadFile(filepath.Join(toolsDir, "stacks", "actions.go"))
	if err != nil {
		t.Fatalf("read stacks/actions.go: %v", err)
	}
	inputsSrc, err := os.ReadFile(filepath.Join(toolsDir, "stacks", "inputs.go"))
	if err != nil {
		t.Fatalf("read stacks/inputs.go: %v", err)
	}
	if !strings.Contains(string(actionsSrc), "func stackDelete(") {
		t.Errorf("stacks/actions.go does not declare stackDelete, a clean operation unrelated to either refusal:\n%s", actionsSrc)
	}
	for _, name := range []string{"stackMigrate", "stackList", "stackMigrateInput", "stackListInput", "StackMigrate", "StackList"} {
		if strings.Contains(string(actionsSrc), name) {
			t.Errorf("stacks/actions.go contains %q, want nothing contributed by a refused operation", name)
		}
		if strings.Contains(string(inputsSrc), name) {
			t.Errorf("stacks/inputs.go contains %q, want nothing contributed by a refused operation", name)
		}
	}

	for _, name := range []string{"actions.go", "inputs.go"} {
		if _, statErr := os.Stat(filepath.Join(toolsDir, "tags", name)); statErr != nil {
			t.Errorf("tags/%s was not written (%v); stacks' refusals must not block an unrelated, later-sorting domain's regeneration", name, statErr)
		}
	}
}

// TestUnit_Run_DeprecatedOperation_SkippedByDefaultUnlessOverridden is table-driven over I5's second half's
// two cases, which share one fixture shape and differ only by whether a
// hand-written override exists and by the expected report line:
//
//   - "default": a vendored operation the specification itself marks
//     "deprecated": true (EndpointDeleteBatchDeprecated, DELETE /endpoints —
//     the operationId itself says so, and its generated client method
//     carries a `// Deprecated:` doc comment staticcheck's SA1019 flags) must
//     contribute nothing at all by default — no struct, no handler, no
//     ActionSpec entry — rather than generate normally and leave a
//     deprecated-API call for a human to notice later. Before this fix it
//     generated exactly like any other operation: endpoints/actions.go:77
//     and :81 in a real scaffold called
//     apigen.EndpointDeleteBatchDeprecatedJSONRequestBody and
//     c.API.EndpointDeleteBatchDeprecatedWithResponse, both deprecated.
//     endpoints has no other hand-written files in this fresh scaffold, so
//     every operation in it (deprecated or not) is generated from scratch; a
//     real non-deprecated operation (EndpointDelete, which needs no
//     redaction wrapper) is asserted present precisely so this case cannot
//     pass merely because the whole domain failed to write anything.
//   - "hand override": the "unless explicitly asked" half — a domain author
//     who has already declared a handler under the mechanical function name
//     for a deprecated operationId gets it committed like any other
//     overridden operation (the escape hatch scanHandOverrides/overrideReason
//     already provides), rather than have the generator's own default skip
//     silently override the human's explicit choice.
//
// The negative half of "hand override" is anchored on the exact line run()
// prints for a skipped operation (see main.go's deprecatedOps append, "%s %s
// (operationId %s)", listed under its own "  - " bullet) rather than two
// independent substrings checked anywhere in the whole stderr blob:
// "skipped as deprecated upstream" is a domain-wide header printed on its
// own line, so requiring it and the operationId to both merely be *present
// somewhere* in stderr would still pass if either appeared for an unrelated
// reason — the operationId is also named on the "already covered by
// hand-written code" line this case asserts next, and the header phrase
// would reappear on its own if any other deprecated, unoverridden operation
// ever joined this domain. The full bullet line this generator emits only
// for a genuinely skipped EndpointDeleteBatchDeprecated is what actually
// discriminates the two.
func TestUnit_Run_DeprecatedOperation_SkippedByDefaultUnlessOverridden(t *testing.T) {
	for _, tc := range []struct {
		name          string
		writeOverride bool
	}{
		{"default", false},
		{"hand override", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			toolsDir := t.TempDir()
			domainDir := filepath.Join(toolsDir, "endpoints")
			if err := os.MkdirAll(domainDir, 0o750); err != nil {
				t.Fatalf("mkdir endpoints: %v", err)
			}
			if tc.writeOverride {
				override := "package endpoints\n\nfunc endpointDeleteBatchDeprecated() {}\n"
				if err := os.WriteFile(filepath.Join(domainDir, "override.go"), []byte(override), 0o600); err != nil {
					t.Fatalf("write override.go: %v", err)
				}
			}

			// run's error is asserted, not discarded: a deferred refusal
			// elsewhere in the spec leaves the expected files in place, so a
			// test that only inspects artefacts passes without ever proving
			// the deprecated-operation path completed.
			var runErr error
			stderr := captureStderr(t, func() {
				runErr = run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", toolsDir})
			})
			// run's error is asserted rather than discarded. It is expected
			// to be non-nil here — the endpoints domain legitimately refuses
			// several operations and the generator exits non-zero whenever it
			// does — but asserting its shape means any *other* failure, which
			// would leave the same artefacts in place, is distinguishable.
			if runErr == nil {
				t.Fatal("run() error = nil, want the refusal summary this domain always produces")
			}
			if !strings.Contains(runErr.Error(), "refused generation across") {
				t.Fatalf("run() error = %v, want the refusal summary rather than an unrelated failure", runErr)
			}

			skipLine := "  - DELETE /endpoints (operationId EndpointDeleteBatchDeprecated)"
			inputsSrc, err := os.ReadFile(filepath.Join(domainDir, "inputs.go"))
			if err != nil {
				t.Fatalf("read endpoints/inputs.go: %v", err)
			}

			if tc.writeOverride {
				if strings.Contains(stderr, skipLine) {
					t.Errorf("stderr contains %q, want EndpointDeleteBatchDeprecated not reported as skipped: a hand-written override for it exists:\n%s", skipLine, stderr)
				}
				if !strings.Contains(stderr, "already covered by hand-written code") {
					t.Errorf("stderr does not report EndpointDeleteBatchDeprecated as covered by the hand-written override:\n%s", stderr)
				}
				if !strings.Contains(string(inputsSrc), "endpointDeleteBatchDeprecatedInput") {
					t.Errorf("endpoints/inputs.go does not declare endpointDeleteBatchDeprecatedInput, want the overridden operation's Input struct still committed:\n%s", inputsSrc)
				}
				return
			}

			if !strings.Contains(stderr, skipLine) {
				t.Errorf("stderr does not contain %q, want the deprecated-operation skip reported:\n%s", skipLine, stderr)
			}

			actionsSrc, err := os.ReadFile(filepath.Join(domainDir, "actions.go"))
			if err != nil {
				t.Fatalf("read endpoints/actions.go: %v", err)
			}
			for _, name := range []string{"EndpointDeleteBatchDeprecated", "endpointDeleteBatchDeprecated"} {
				if strings.Contains(string(inputsSrc), name) {
					t.Errorf("endpoints/inputs.go contains %q, want a deprecated operation to contribute nothing", name)
				}
				if strings.Contains(string(actionsSrc), name) {
					t.Errorf("endpoints/actions.go contains %q, want a deprecated operation to contribute nothing", name)
				}
			}
			if !strings.Contains(string(actionsSrc), "func endpointDelete(") {
				t.Errorf("endpoints/actions.go does not declare endpointDelete, a clean, non-deprecated operation unrelated to the skip:\n%s", actionsSrc)
			}
		})
	}
}
