package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func op(id, method, path, domain string) specOperation {
	return specOperation{OperationID: id, Method: method, Path: path, Domain: domain}
}

func action(name, operationID string) toolutil.ActionSpec {
	return toolutil.ActionSpec{Name: name, OperationID: operationID}
}

// TestAudit_OperationWithNoAction_IsReportedAndFails is the core property
// this whole command exists for: an operation the spec declares with no
// catalog action must both be named in the report and make the audit fail.
//
// The exit-code half of the assertion is checked on auditCoverage's returned
// error and on HasGap directly, never on whether the rendered text happens to
// contain a particular word — see this package's main_test.go for why that
// distinction matters (a report that prints "FAIL" and returns nil satisfies
// a text-matching check and gates nothing).
func TestAudit_OperationWithNoAction_IsReportedAndFails(t *testing.T) {
	t.Parallel()
	ee := map[string]specOperation{
		"TagList":   op("TagList", "GET", "/tags", "tags"),
		"TagDelete": op("TagDelete", "DELETE", "/tags/{id}", "tags"),
	}
	ce := map[string]specOperation{
		"TagList": op("TagList", "GET", "/tags", "tags"),
	}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}

	result, err := auditCoverage(doc(ce), doc(ee), actions, nil, nil)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v, want a result to report on", err)
	}

	// The property that actually gates CI: HasGap must be true, checked as a
	// boolean, not derived from string content.
	if !result.HasGap() {
		t.Fatal("auditCoverage().HasGap() = false, want true: TagDelete has no action")
	}
	if len(result.EE.Uncovered) != 1 || result.EE.Uncovered[0].OperationID != "TagDelete" {
		t.Fatalf("auditCoverage().EE.Uncovered = %v, want exactly [TagDelete]", result.EE.Uncovered)
	}

	// The report must actually name the gap: a count with no names would tell
	// nobody what to fix.
	report := buildReport(result)
	if !strings.Contains(report, "TagDelete") {
		t.Errorf("buildReport() does not name the uncovered operation:\n%s", report)
	}
}

// TestAudit_AllowListedOperation_IsExcludedButCounted proves the allow-list
// removes an operation from the failure without removing it from the report:
// silent exclusion is exactly what would let coverage decay behind a clean
// number.
func TestAudit_AllowListedOperation_IsExcludedButCounted(t *testing.T) {
	t.Parallel()
	ee := map[string]specOperation{
		"TagList":       op("TagList", "GET", "/tags", "tags"),
		"WebsocketExec": op("WebsocketExec", "GET", "/websocket/exec", "websocket"),
	}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}
	allowList := []allowListEntry{
		{OperationID: "WebsocketExec", Reason: "MCP cannot carry a websocket upgrade.", Added: "2026-08-03"},
	}

	result, err := auditCoverage(doc(nil), doc(ee), actions, allowList, nil)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}

	if result.HasGap() {
		t.Fatalf("auditCoverage().HasGap() = true, want false: the only gap is allow-listed: %v", result.EE.Uncovered)
	}
	if result.AllowListCount != 1 {
		t.Errorf("auditCoverage().AllowListCount = %d, want 1", result.AllowListCount)
	}
	if len(result.EE.AllowListed) != 1 || result.EE.AllowListed[0].OperationID != "WebsocketExec" {
		t.Fatalf("auditCoverage().EE.AllowListed = %v, want exactly [WebsocketExec]", result.EE.AllowListed)
	}

	// Still visible in the human-facing output, not just the struct.
	report := buildReport(result)
	if !strings.Contains(report, "WebsocketExec") {
		t.Errorf("buildReport() does not mention the allow-listed operation:\n%s", report)
	}
	if !strings.Contains(report, "Allow-list entries: 1") {
		t.Errorf("buildReport() does not state the allow-list count:\n%s", report)
	}
}

// TestAudit_AllowListEntryForAnUnknownOperation_IsAnError proves a stale
// allow-list entry — one naming an operation neither vendored spec declares
// any more — is refused outright, rather than silently ignored. An ignored
// stale entry would keep forgiving something that no longer exists, and a
// real, unrelated future gap could reuse that same operation name and
// inherit the forgiveness.
func TestAudit_AllowListEntryForAnUnknownOperation_IsAnError(t *testing.T) {
	t.Parallel()
	ee := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}
	allowList := []allowListEntry{
		{OperationID: "LongRemovedOperation", Reason: "no longer exists", Added: "2026-08-03"},
	}

	_, err := auditCoverage(doc(nil), doc(ee), actions, allowList, nil)
	if err == nil {
		t.Fatal("auditCoverage() = nil error, want an error for the stale allow-list entry")
	}
	if !strings.Contains(err.Error(), "LongRemovedOperation") {
		t.Errorf("auditCoverage() error = %v, want it to name the stale entry", err)
	}
}

// TestAudit_ActionNamingAnOperationNotInEitherSpec_IsAnError is the reverse
// direction: a catalog action whose OperationID resolves in neither vendored
// spec is a typo or a stale declaration. actioncatalog.Build already refuses
// to build such a catalog against the version-applicability table, but this
// audit is the one place that checks both vendored specs at once, and it
// must not let such an action through just because it happens to be
// well-formed otherwise.
func TestAudit_ActionNamingAnOperationNotInEitherSpec_IsAnError(t *testing.T) {
	t.Parallel()
	ee := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	actions := []toolutil.ActionSpec{
		action("tags.list", "TagList"),
		action("tags.typo", "TagLisst"),
	}

	_, err := auditCoverage(doc(nil), doc(ee), actions, nil, nil)
	if err == nil {
		t.Fatal("auditCoverage() = nil error, want an error for the action naming an unresolvable operation")
	}
	if !strings.Contains(err.Error(), "tags.typo") || !strings.Contains(err.Error(), "TagLisst") {
		t.Errorf("auditCoverage() error = %v, want it to name the action and its OperationID", err)
	}
}

// TestAudit_EveryOperationCovered_NoGap is the positive control for
// TestAudit_OperationWithNoAction_IsReportedAndFails: with a matching action
// for every declared operation, the audit must not report a gap and must not
// return an error.
func TestAudit_EveryOperationCovered_NoGap(t *testing.T) {
	t.Parallel()
	ce := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	ee := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}

	result, err := auditCoverage(doc(ce), doc(ee), actions, nil, nil)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}
	if result.HasGap() {
		t.Fatalf("auditCoverage().HasGap() = true, want false: %v / %v", result.CE.Uncovered, result.EE.Uncovered)
	}
	if result.CE.Covered != 1 || result.EE.Covered != 1 {
		t.Errorf("auditCoverage() covered = CE:%d EE:%d, want 1 and 1", result.CE.Covered, result.EE.Covered)
	}
}

// TestAudit_ActionCoveringBothEditions_CountsInBoth proves an action shared
// by both specs (the common case: most operations exist in both editions)
// is counted as covered in each edition's own report, not just one.
func TestAudit_ActionCoveringBothEditions_CountsInBoth(t *testing.T) {
	t.Parallel()
	shared := op("TagList", "GET", "/tags", "tags")
	ce := map[string]specOperation{"TagList": shared}
	ee := map[string]specOperation{"TagList": shared}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}

	result, err := auditCoverage(doc(ce), doc(ee), actions, nil, nil)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}
	if result.CE.Covered != 1 {
		t.Errorf("auditCoverage().CE.Covered = %d, want 1", result.CE.Covered)
	}
	if result.EE.Covered != 1 {
		t.Errorf("auditCoverage().EE.Covered = %d, want 1", result.EE.Covered)
	}
}

// TestUnit_BuildReport_PrintsDeprecatedInline guards against a reviewer
// having to open the vendored spec directly to tell a deprecated route from
// a live gap: Deprecated is documented as a legitimate reason to allow-list
// an operation, so buildReport must say so next to the operation itself, in
// both the allow-listed and the uncovered sections.
func TestUnit_BuildReport_PrintsDeprecatedInline(t *testing.T) {
	t.Parallel()
	deprecatedAllowListed := op("OldRoute", "GET", "/old", "legacy")
	deprecatedAllowListed.Deprecated = true
	deprecatedUncovered := op("AnotherOldRoute", "GET", "/another-old", "legacy")
	deprecatedUncovered.Deprecated = true
	liveGap := op("LiveGap", "GET", "/live", "live")

	result := &auditResult{
		EE: editionReport{
			Name:        "Business Edition (EE)",
			Total:       3,
			AllowListed: []specOperation{deprecatedAllowListed},
			Uncovered:   []specOperation{deprecatedUncovered, liveGap},
		},
		CE: editionReport{Name: "Community Edition (CE)"},
	}
	report := buildReport(result)

	if !strings.Contains(report, "OldRoute (GET /old) [legacy] (deprecated)") {
		t.Errorf("buildReport() does not mark the deprecated allow-listed operation:\n%s", report)
	}
	if !strings.Contains(report, "AnotherOldRoute (GET /another-old) [legacy] (deprecated)") {
		t.Errorf("buildReport() does not mark the deprecated uncovered operation:\n%s", report)
	}
	if strings.Contains(report, "LiveGap (GET /live) [live] (deprecated)") {
		t.Errorf("buildReport() marks a non-deprecated operation as deprecated:\n%s", report)
	}
}

func TestUnit_BuildReport_FullCoverage_StatesSo(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		CE: editionReport{Name: "Community Edition (CE)", Total: 1, Covered: 1},
		EE: editionReport{Name: "Business Edition (EE)", Total: 1, Covered: 1},
	}
	report := buildReport(result)
	if !strings.Contains(report, "Every operation in both vendored specs has a catalog action") {
		t.Errorf("buildReport() does not state full coverage:\n%s", report)
	}
}

// TestUnit_BuildReport_UnnamedRoutes_AreNamedNotJustCounted is the report
// half of this audit's honesty contract, and the one that was missing.
//
// A route with no operationId cannot be counted — there is no key to count it
// against — so it appears in neither the numerator nor the denominator.
// Before this, that was the end of it: the route appeared nowhere at all, and
// the report's bottom line could say every operation was accounted for while
// a route Portainer serves was accounted for nowhere. The three assertions
// below are what a fix that merely counted them differently would not
// satisfy: the section exists, every route in it is named individually, and
// the bottom line says the totals above are not the whole surface.
func TestUnit_BuildReport_UnnamedRoutes_AreNamedNotJustCounted(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		EE: editionReport{Name: "Business Edition (EE)", Total: 1, Covered: 1},
		CE: editionReport{
			Name: "Community Edition (CE)", Total: 1, Covered: 1,
			Unnamed: []unnamedOperation{
				{Method: "GET", Path: "/websocket/exec", Domain: "websocket"},
				{Method: "POST", Path: "/webhooks", Domain: "webhooks"},
			},
		},
	}
	report := buildReport(result)

	for _, want := range []string{
		"routes with no operationId, not counted above (2):",
		"    - GET /websocket/exec [websocket]\n",
		"    - POST /webhooks [webhooks]\n",
		"2 route(s) across both editions carry no operationId and are outside the totals above",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() does not contain %q:\n%s", want, report)
		}
	}
}

// TestUnit_BuildReport_NoUnnamedRoutes_SaysNothingAboutThem keeps the section
// from becoming noise on the day the documents name everything: the whole
// point of naming these routes is that a reader notices them, which stops
// being true if an empty section is printed on every run.
func TestUnit_BuildReport_NoUnnamedRoutes_SaysNothingAboutThem(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		CE: editionReport{Name: "Community Edition (CE)", Total: 1, Covered: 1},
		EE: editionReport{Name: "Business Edition (EE)", Total: 1, Covered: 1},
	}
	if report := buildReport(result); strings.Contains(report, "no operationId") {
		t.Errorf("buildReport() mentions unnamed routes when there are none:\n%s", report)
	}
}

// TestUnit_AuditCoverage_UnnamedRoutes_ReachTheReport closes the seam
// between the two halves this task added, and review found it open.
//
// parseSpecOperations carries a route nothing names out of the document, and
// buildReport prints one when the result holds it. Each half had a test.
// What nothing exercised was the wire between them — buildEditionReport
// copying doc.Unnamed onto the edition report — so deleting that one field
// assignment left every test green while the real report silently lost all
// 13 routes. That is this task's own defect class one layer up: a change
// that re-hides the routes passing every gate.
//
// The doc(...) helper the migrated call sites use is what made it invisible:
// it supplies Unnamed: nil, so no auditCoverage-level test ever carried an
// unnamed route end to end. This one does, through the real function, and
// asserts both that the result holds them and that the rendered report names
// them.
func TestUnit_AuditCoverage_UnnamedRoutes_ReachTheReport(t *testing.T) {
	t.Parallel()
	ceUnnamed := []unnamedOperation{
		{Method: "GET", Path: "/websocket/exec", Domain: "websocket"},
		{Method: "POST", Path: "/webhooks", Domain: "webhooks"},
	}
	eeUnnamed := []unnamedOperation{
		{Method: "GET", Path: "/websocket/pod", Domain: "websocket"},
	}
	ce := specDocument{
		Operations: map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")},
		Unnamed:    ceUnnamed,
	}
	ee := specDocument{
		Operations: map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")},
		Unnamed:    eeUnnamed,
	}
	actions := []toolutil.ActionSpec{action("tags.list", "TagList")}

	result, err := auditCoverage(ce, ee, actions, nil, nil)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}

	if !reflect.DeepEqual(result.CE.Unnamed, ceUnnamed) {
		t.Errorf("auditCoverage().CE.Unnamed = %+v, want %+v", result.CE.Unnamed, ceUnnamed)
	}
	if !reflect.DeepEqual(result.EE.Unnamed, eeUnnamed) {
		t.Errorf("auditCoverage().EE.Unnamed = %+v, want %+v", result.EE.Unnamed, eeUnnamed)
	}
	// Unnamed routes have no key, so they must not have moved the totals or
	// turned into gaps: coverage is still complete over what is countable.
	if result.CE.Total != 1 || result.EE.Total != 1 {
		t.Errorf("auditCoverage() totals = CE:%d EE:%d, want 1 and 1: an unnamed route has no key and must not enter the denominator", result.CE.Total, result.EE.Total)
	}
	if result.HasGap() {
		t.Errorf("auditCoverage().HasGap() = true, want false: %v / %v", result.CE.Uncovered, result.EE.Uncovered)
	}

	report := buildReport(result)
	for _, want := range []string{
		"    - GET /websocket/exec [websocket]\n",
		"    - POST /webhooks [webhooks]\n",
		"    - GET /websocket/pod [websocket]\n",
		"3 route(s) across both editions carry no operationId and are outside the totals above",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport(auditCoverage(...)) does not contain %q:\n%s", want, report)
		}
	}
}
