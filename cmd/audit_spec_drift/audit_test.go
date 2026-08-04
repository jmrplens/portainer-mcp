package main

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// TestUnit_VerifyCanary_RealComparator_Passes proves the canary passes
// against the actual engine every real run uses, so this command never
// refuses to report anything for a spurious reason.
func TestUnit_VerifyCanary_RealComparator_Passes(t *testing.T) {
	t.Parallel()
	if err := verifyCanary(specdiff.Compare); err != nil {
		t.Fatalf("verifyCanary(specdiff.Compare) error = %v, want nil against the real comparison engine", err)
	}
}

// TestUnit_VerifyCanary_ComparatorReturnsNothing_Fails is this audit's own
// discriminating check on its discriminating check: a comparator that
// silently stopped comparing (always returns no changes, indistinguishable
// from "nothing was actually compared") must be caught here, before this
// command trusts anything else it is about to report.
func TestUnit_VerifyCanary_ComparatorReturnsNothing_Fails(t *testing.T) {
	t.Parallel()
	broken := func(specdiff.OperationShape, specdiff.OperationShape) []specdiff.FieldChange { return nil }
	if err := verifyCanary(broken); err == nil {
		t.Fatal("verifyCanary(broken) error = nil, want an error: a comparator that never reports anything must fail the canary")
	}
}

// TestUnit_VerifyCanary_ComparatorAlwaysReportsDrift_Fails is the mirror
// image: a comparator that flags every pair as different — including two
// identical shapes — would trivially "detect" the deliberate perturbation
// while making every clean run look broken. Both failure modes are equally
// unable to discriminate, so both must be caught.
func TestUnit_VerifyCanary_ComparatorAlwaysReportsDrift_Fails(t *testing.T) {
	t.Parallel()
	alwaysDrifts := func(specdiff.OperationShape, specdiff.OperationShape) []specdiff.FieldChange {
		return []specdiff.FieldChange{{JSONName: "id", Kind: specdiff.ChangeType, Before: "integer", After: "string"}}
	}
	if err := verifyCanary(alwaysDrifts); err == nil {
		t.Fatal("verifyCanary(alwaysDrifts) error = nil, want an error: a comparator that reports drift even for identical shapes must fail the canary")
	}
}

// TestUnit_VerifyCanary_ComparatorReportsWrongKind_Fails proves the check is
// on the specific ChangeKind, not merely on "something came back" — a
// comparator that flags the perturbed pair as, say, a ChangeRequiredness
// finding instead of ChangeType is just as broken as one that finds nothing.
func TestUnit_VerifyCanary_ComparatorReportsWrongKind_Fails(t *testing.T) {
	t.Parallel()
	wrongKind := func(before, after specdiff.OperationShape) []specdiff.FieldChange {
		if len(before.Fields) > 0 && len(after.Fields) > 0 && before.Fields[0].Type == after.Fields[0].Type {
			return nil // identical shapes: report nothing, the correct half
		}
		return []specdiff.FieldChange{{JSONName: "id", Kind: specdiff.ChangeRequiredness, Before: "true", After: "false"}}
	}
	if err := verifyCanary(wrongKind); err == nil {
		t.Fatal("verifyCanary(wrongKind) error = nil, want an error: the perturbation is a type swap, not a requiredness change")
	}
}

func TestUnit_IsGating_StructuralKindsAlwaysGate(t *testing.T) {
	t.Parallel()
	for _, kind := range []specdiff.ChangeKind{
		specdiff.ChangeAdded, specdiff.ChangeRemoved, specdiff.ChangeType,
		specdiff.ChangeRequiredness, specdiff.ChangeEnum, specdiff.ChangeOrigin,
	} {
		t.Run(string(kind), func(t *testing.T) {
			t.Parallel()
			c := specdiff.FieldChange{JSONName: "x", Kind: kind, Before: "a", After: "b"}
			if !isGating(c) {
				t.Errorf("isGating(%+v) = false, want true: every structural kind gates unconditionally", c)
			}
		})
	}
}

// TestUnit_IsGating_DescriptionSpecHadText_Gates is the case the brief calls
// "drift worth catching": the vendored specification described this field,
// and the catalog's text no longer matches it.
func TestUnit_IsGating_DescriptionSpecHadText_Gates(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: "x", Kind: specdiff.ChangeDescription, Before: "Tag identifier", After: "Different wording"}
	if !isGating(c) {
		t.Errorf("isGating(%+v) = false, want true: the spec described this field and the catalog's text drifted from it", c)
	}
}

// TestUnit_IsGating_DescriptionSpecNeverDescribed_DoesNotGate is the other
// case: the vendored specification never described this field at all (true
// for roughly 286 of 771 body properties measured against the real spec).
// Before is "" whenever the spec-derived shape supplied it — see
// ShapeFromSpec — so a field that "changed" from silence to a hand-authored
// description cannot have drifted from anything.
func TestUnit_IsGating_DescriptionSpecNeverDescribed_DoesNotGate(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: "x", Kind: specdiff.ChangeDescription, Before: "", After: "A helpful, hand-written description"}
	if isGating(c) {
		t.Errorf("isGating(%+v) = true, want false: the spec never described this field, so there is nothing to have drifted from", c)
	}
}

// TestUnit_IsGating_TitleSpecHadText_Gates and
// TestUnit_IsGating_OperationDescriptionSpecHadText_Gates prove the
// operation-level kinds gate the same way ChangeDescription does when the
// spec's own side is non-empty — real drift, worth catching.
func TestUnit_IsGating_TitleSpecHadText_Gates(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: specdiff.TitleSentinel, Kind: specdiff.ChangeTitle, Before: "Create a new tag", After: "Create a tag, differently"}
	if !isGating(c) {
		t.Errorf("isGating(%+v) = false, want true: the spec's own title changed, and the catalog's no longer matches it", c)
	}
}

func TestUnit_IsGating_OperationDescriptionSpecHadText_Gates(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: specdiff.DescriptionSentinel, Kind: specdiff.ChangeOperationDescription, Before: "Create a new tag.", After: "Create a tag, worded differently."}
	if !isGating(c) {
		t.Errorf("isGating(%+v) = false, want true: the spec's own description changed, and the catalog's no longer matches it", c)
	}
}

// TestUnit_IsGating_TitleSpecEmpty_DoesNotGate and
// TestUnit_IsGating_OperationDescriptionSpecEmpty_DoesNotGate cover the
// hypothetical mirror of ChangeDescription's own silent-spec case: this
// never happens for a real vendored operation today (every one has a
// non-empty summary — see cmd/gen_action_inputs's own
// TestUnit_CleanTitleAndDescription_EveryRealOperationHasANonEmptySummary),
// but specdiff.ShapeFromSpec is deliberately lenient rather than refusing
// (see its own doc comment), so this branch is reachable in principle and
// must not gate a Title/Description the spec genuinely never stated.
func TestUnit_IsGating_TitleSpecEmpty_DoesNotGate(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: specdiff.TitleSentinel, Kind: specdiff.ChangeTitle, Before: "", After: "A hand-written title"}
	if isGating(c) {
		t.Errorf("isGating(%+v) = true, want false: the spec stated no title at all, so there is nothing to have drifted from", c)
	}
}

func TestUnit_IsGating_OperationDescriptionSpecEmpty_DoesNotGate(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{JSONName: specdiff.DescriptionSentinel, Kind: specdiff.ChangeOperationDescription, Before: "", After: "A hand-written description"}
	if isGating(c) {
		t.Errorf("isGating(%+v) = true, want false: the spec stated no description at all, so there is nothing to have drifted from", c)
	}
}

// TestUnit_IsGating_TitleOverridden_DoesNotGate is the mutation proof for
// this task's own reservation about the 35-entry allow-list: an
// AfterOverridden ChangeTitle/ChangeOperationDescription finding — the spec
// had real text, and the catalog's differs because toolutil.WithNarrative
// deliberately overrode it — must not gate, with no allow-list entry
// involved at all.
func TestUnit_IsGating_TitleOverridden_DoesNotGate(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{
		JSONName: specdiff.TitleSentinel, Kind: specdiff.ChangeTitle,
		Before: "The spec's own title", After: "A deliberately overridden title",
		AfterOverridden: true,
	}
	if isGating(c) {
		t.Errorf("isGating(%+v) = true, want false: AfterOverridden marks this as a deliberate, permanent narrative override", c)
	}
}

// TestUnit_IsGating_OperationDescriptionOverridden_DoesNotGate is the
// ChangeOperationDescription sibling of the Title case above.
func TestUnit_IsGating_OperationDescriptionOverridden_DoesNotGate(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{
		JSONName: specdiff.DescriptionSentinel, Kind: specdiff.ChangeOperationDescription,
		Before: "The spec's own description", After: "A deliberately overridden description",
		AfterOverridden: true,
	}
	if isGating(c) {
		t.Errorf("isGating(%+v) = true, want false: AfterOverridden marks this as a deliberate, permanent narrative override", c)
	}
}

// TestUnit_IsGating_TitleChangedButNotOverridden_StillGates is the
// discriminating half of the same proof: AfterOverridden false (the ordinary
// case for every action that has never called toolutil.WithNarrative) must
// still gate exactly as before this field existed — a detector that always
// treats a Title/Description change as overridden would silently stop
// catching real drift for every action in the catalog.
func TestUnit_IsGating_TitleChangedButNotOverridden_StillGates(t *testing.T) {
	t.Parallel()
	c := specdiff.FieldChange{
		JSONName: specdiff.TitleSentinel, Kind: specdiff.ChangeTitle,
		Before: "The spec's own title", After: "A title that just happens to differ",
		AfterOverridden: false,
	}
	if !isGating(c) {
		t.Errorf("isGating(%+v) = false, want true: AfterOverridden is false, so this is ordinary, un-excused drift", c)
	}
}

// TestUnit_AuditDrift_NarrativeTitleOverride_IsExcusedByAllowList is the
// end-to-end proof of this task's chosen mechanism for a deliberate
// Title/Description override: a domain's narrative() hook (or a fully
// hand-written ActionSpec literal) replaces the vendored specification's
// own title outright, toolutil.ActionSpec retains no trace that this was a
// deliberate override rather than accidental drift, and the allow-list —
// keyed by (operationId, specdiff.TitleSentinel) — is what records that
// decision and keeps the build green without hiding the finding from the
// report.
func TestUnit_AuditDrift_NarrativeTitleOverride_IsExcusedByAllowList(t *testing.T) {
	t.Parallel()
	// Deliberately built rather than reusing singleFieldSpec: this test needs
	// the spec's own Summary to differ from fixtureAction's Title ("Fixture
	// action") so there is an actual ChangeTitle finding to excuse — reusing
	// singleFieldSpec, whose Summary matches on purpose (see its own doc
	// comment), would produce no finding at all here.
	eeOps := map[string]specOperation{
		fixtureOperationID: {Op: specdiff.SpecOperation{
			OperationID: fixtureOperationID, Method: http.MethodGet, Path: "/" + fixtureOperationID,
			Summary: "A narrative override replaced this title", Description: "d",
			Parameters: []map[string]any{{"name": fixtureFieldName, "in": "query", "required": false, "schema": map[string]any{"type": "string"}}},
		}},
	}
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}
	allowList := []allowListEntry{{
		OperationID: fixtureOperationID, Field: specdiff.TitleSentinel,
		Reason: "narrative override, deliberately improved wording", Added: "2026-08-04",
	}}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, allowList)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("auditDrift() HasDrift() = true, want false: the only gating finding is excused by the allow-list; Findings = %+v", result.Findings)
	}
	found := false
	for _, f := range result.Findings {
		if f.Change.Kind == specdiff.ChangeTitle {
			found = true
			if !f.AllowListed {
				t.Errorf("Findings entry for ChangeTitle = %+v, want AllowListed true", f)
			}
		}
	}
	if !found {
		t.Fatal("auditDrift() Findings has no ChangeTitle entry at all: the allow-list must not hide a finding from the report, only from the failure")
	}
}

// --- auditDrift ---

// fixtureOperationID is the one operationId every audit_test.go fixture
// below declares, on both the spec side (singleFieldSpec) and the catalog
// side (fixtureAction). fixtureFieldName is that operation's one field,
// named identically on both sides too.
const (
	fixtureOperationID = "Fixture"
	fixtureFieldName   = "field"
)

// singleFieldSpec builds a one-operation, one-field vendored spec fixture:
// fixtureOperationID takes one query parameter named fixtureFieldName, of
// the given type, with the given description ("" omits the key entirely,
// matching how a real vendored operation with no description for a
// parameter looks).
// singleFieldSpec's Summary/Description are set to exactly fixtureAction's
// Title ("Fixture action") and Description ("d") — not left blank — so that
// specdiff.ShapeFromSpec's cleaned operation-level Title/Description agree
// with the catalog side by construction. Every test built on this fixture
// exercises one field's drift in isolation; if this operation-level text
// disagreed instead, every one of those tests would also see an unrelated
// ChangeTitle/ChangeOperationDescription finding alongside whatever it means
// to test, silently inflating GatingCount and len(Findings) for a reason
// that has nothing to do with what the test's name says it proves — exactly
// the "changed more than one thing at once" trap a fixture must not set.
func singleFieldSpec(typ, description string) map[string]specOperation {
	param := map[string]any{"name": fixtureFieldName, "in": "query", "required": false, "schema": map[string]any{"type": typ}}
	if description != "" {
		param["description"] = description
	}
	return map[string]specOperation{
		fixtureOperationID: {
			Op: specdiff.SpecOperation{
				OperationID: fixtureOperationID,
				Method:      http.MethodGet,
				Path:        "/" + fixtureOperationID,
				Summary:     "Fixture action",
				Description: "d",
				Parameters:  []map[string]any{param},
			},
			Domain: "fixture",
		},
	}
}

// fixtureInput is the catalog-side Input type auditDrift's tests reflect a
// schema from. jsonschema-go's reflector marks a field required unless it
// carries "omitempty" — every fixture field below is optional, matching the
// singleFieldSpec fixtures' own "required": false, so a test that wants to
// exercise ChangeType/ChangeEnum/ChangeDescription is not incidentally also
// tripping ChangeRequiredness.
type fixtureInput struct {
	Field string `json:"field,omitempty" jsonschema:"description text"`
}

type fixtureInputNoDescription struct {
	Field string `json:"field,omitempty"`
}

type fixtureInputInt struct {
	Field int `json:"field,omitempty"`
}

// fixtureAction builds a minimal, real toolutil.ActionSpec naming
// operationID, with the given Input — enough for specdiff.ShapeFromCatalog
// to reflect a real schema from, mirroring
// internal/wiring's schema_parity_test.go's identical fixture-over-real-domain
// approach so this test never needs to change when a real domain's schema
// does.
func fixtureAction(operationID string, input any) toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: "fixture.action", Domain: "fixture", OperationID: operationID,
		Title: "Fixture action", Description: "d", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
		Input:   input,
	}
}

// TestUnit_AuditDrift_TypeSwap_IsGatingAndUnexcused is this command's whole
// point exercised end to end through auditDrift: the real EndpointList-style
// defect this phase exists to catch, expressed as a fixture (a query
// parameter typed "integer" in the spec, "string" in the catalog).
func TestUnit_AuditDrift_TypeSwap_IsGatingAndUnexcused(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("integer", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.GatingCount != 1 {
		t.Fatalf("auditDrift() GatingCount = %d, want 1", result.GatingCount)
	}
	if !result.HasDrift() {
		t.Error("auditDrift() HasDrift() = false, want true: an un-excused ChangeType finding must gate")
	}
	if len(result.Findings) != 1 || result.Findings[0].Change.Kind != specdiff.ChangeType {
		t.Fatalf("auditDrift() Findings = %+v, want exactly one ChangeType finding", result.Findings)
	}
	if result.Findings[0].AllowListed {
		t.Error("auditDrift() Findings[0].AllowListed = true, want false: no allow-list entry was given")
	}
}

// TestUnit_AuditDrift_IdenticalShapes_ReportsNoDrift is the positive
// control: an action whose catalog shape exactly matches its vendored
// operation must produce no findings at all.
func TestUnit_AuditDrift_IdenticalShapes_ReportsNoDrift(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("auditDrift() HasDrift() = true, want false: the catalog matches the spec exactly; Findings = %+v", result.Findings)
	}
	if result.ActionsAudited != 1 {
		t.Errorf("auditDrift() ActionsAudited = %d, want 1", result.ActionsAudited)
	}
}

// TestUnit_AuditDrift_DescriptionNeitherSideHas_ReportsNoDrift covers the
// 286-body-fields-with-no-spec-description case directly: neither side
// states a description, so Compare should not even produce a
// ChangeDescription finding to begin with (both sides are "").
func TestUnit_AuditDrift_DescriptionNeitherSideHas_ReportsNoDrift(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if len(result.Findings) != 0 {
		t.Errorf("auditDrift() Findings = %+v, want none: neither side describes the field", result.Findings)
	}
}

// TestUnit_AuditDrift_DescriptionSpecHadTextCatalogDropped_Gates is the real
// drift case: the spec describes the field, the catalog's Input carries no
// jsonschema tag for it at all (a plausible hand-edit mistake — deleting a
// struct tag while renaming a field, say). This must gate.
func TestUnit_AuditDrift_DescriptionSpecHadTextCatalogDropped_Gates(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "The field's spec description")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.GatingCount != 1 {
		t.Fatalf("auditDrift() GatingCount = %d, want 1: the spec's description was dropped from the catalog", result.GatingCount)
	}
	if result.Findings[0].Change.Kind != specdiff.ChangeDescription {
		t.Errorf("auditDrift() Findings[0].Kind = %v, want ChangeDescription", result.Findings[0].Change.Kind)
	}
}

// TestUnit_AuditDrift_DescriptionSpecSilentCatalogAdded_DoesNotGate is the
// improvement case the brief explicitly distinguishes: the spec never
// described this field, but the catalog (a hand-written pilot) adds a
// helpful description anyway. That must not fail the build.
func TestUnit_AuditDrift_DescriptionSpecSilentCatalogAdded_DoesNotGate(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInput{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("auditDrift() HasDrift() = true, want false: the spec never described this field, so a hand-added description is an improvement, not drift; Findings = %+v", result.Findings)
	}
	if len(result.Findings) != 1 || result.Findings[0].Change.Kind != specdiff.ChangeDescription {
		t.Fatalf("auditDrift() Findings = %+v, want exactly one (non-gating) ChangeDescription finding, for visibility", result.Findings)
	}
	if result.Findings[0].Gating {
		t.Error("auditDrift() Findings[0].Gating = true, want false")
	}
}

// TestUnit_AuditDrift_AllowListedField_ExcusesGatingFindingButStillReports
// proves the allow-list's honesty mechanism: a gating finding it names is
// excluded from GatingCount but never from Findings.
func TestUnit_AuditDrift_AllowListedField_ExcusesGatingFindingButStillReports(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("integer", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}
	allowList := []allowListEntry{{OperationID: "Fixture", Field: "field", Reason: "deliberate narrower type", Added: "2026-08-04"}}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, allowList)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("auditDrift() HasDrift() = true, want false: the only gating finding is allow-listed; Findings = %+v, Stale = %+v", result.Findings, result.StaleEntries)
	}
	if len(result.Findings) != 1 {
		t.Fatalf("auditDrift() Findings = %+v, want the excused finding still reported", result.Findings)
	}
	if !result.Findings[0].AllowListed {
		t.Error("auditDrift() Findings[0].AllowListed = false, want true")
	}
	if len(result.StaleEntries) != 0 {
		t.Errorf("auditDrift() StaleEntries = %+v, want none: the entry matched a real gating finding", result.StaleEntries)
	}
}

// TestUnit_AuditDrift_StaleAllowListEntry_IsItselfAnError is the standing
// warning this audit must not repeat: an allow-list entry that excuses
// nothing a real run currently finds gating must fail the build on its own,
// exactly like cmd/audit_1to1's identical rule for its own allow-list.
func TestUnit_AuditDrift_StaleAllowListEntry_IsItselfAnError(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "") // no drift at all
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}
	allowList := []allowListEntry{{OperationID: "Fixture", Field: "field", Reason: "used to diverge, no longer does", Added: "2026-08-04"}}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, allowList)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if len(result.StaleEntries) != 1 {
		t.Fatalf("auditDrift() StaleEntries = %+v, want exactly one: the entry excuses nothing", result.StaleEntries)
	}
	if !result.HasDrift() {
		t.Error("auditDrift() HasDrift() = false, want true: a stale allow-list entry must fail the build on its own")
	}
}

// TestUnit_AuditDrift_StaleTitleAllowListEntry_IsItselfAnError proves the
// stale-entry check above is not merely a property of the generic
// (operationId, field) mechanism exercised only for a real parameter name —
// it is exercised here for the sentinel field specdiff.TitleSentinel too, a
// distinct wire shape (starts with "$", never a real JSON property name)
// that a less careful implementation could have special-cased differently.
// singleFieldSpec's Summary matches fixtureAction's Title exactly (see its
// own doc comment), so there is genuinely no ChangeTitle finding for this
// allow-list entry to excuse.
func TestUnit_AuditDrift_StaleTitleAllowListEntry_IsItselfAnError(t *testing.T) {
	t.Parallel()
	eeOps := singleFieldSpec("string", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}
	allowList := []allowListEntry{{
		OperationID: "Fixture", Field: specdiff.TitleSentinel,
		Reason: "used to diverge, no longer does", Added: "2026-08-04",
	}}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, allowList)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if len(result.StaleEntries) != 1 {
		t.Fatalf("auditDrift() StaleEntries = %+v, want exactly one: the $title entry excuses nothing, Title matches on both sides", result.StaleEntries)
	}
	if !result.HasDrift() {
		t.Error("auditDrift() HasDrift() = false, want true: a stale $title allow-list entry must fail the build on its own")
	}
}

// TestUnit_AuditDrift_UnresolvableOperationID_ReturnsError covers the fatal
// input error: an action whose OperationID resolves in neither vendored
// spec cannot be classified as a finding at all.
func TestUnit_AuditDrift_UnresolvableOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{fixtureAction("NoSuchOperation", fixtureInputNoDescription{})}
	if _, err := auditDrift(map[string]specOperation{}, map[string]specOperation{}, actions, nil); err == nil {
		t.Fatal("auditDrift() error = nil, want an error: NoSuchOperation resolves in neither spec")
	}
}

// TestUnit_AuditDrift_EnumChange_Gates covers ChangeEnum specifically, since
// it is one of the structural kinds this audit must not accidentally treat
// as cosmetic.
func TestUnit_AuditDrift_EnumChange_Gates(t *testing.T) {
	t.Parallel()
	param := map[string]any{
		"name": "field", "in": "query", "required": false,
		"schema": map[string]any{"type": "integer", "enum": []any{float64(1), float64(2), float64(3)}},
	}
	eeOps := map[string]specOperation{
		// Summary/Description match fixtureAction's Title/Description exactly
		// — see singleFieldSpec's doc comment for why: otherwise this test
		// would also pick up an unrelated ChangeTitle/ChangeOperationDescription
		// finding and its GatingCount==1 assertion below would be testing two
		// things at once.
		"Fixture": {Op: specdiff.SpecOperation{
			OperationID: "Fixture", Method: http.MethodGet, Path: "/fixture",
			Summary: "Fixture action", Description: "d",
			Parameters: []map[string]any{param},
		}},
	}
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputInt{})}

	result, err := auditDrift(eeOps, map[string]specOperation{}, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.GatingCount != 1 || result.Findings[0].Change.Kind != specdiff.ChangeEnum {
		t.Fatalf("auditDrift() = GatingCount %d, Findings %+v, want exactly one gating ChangeEnum finding", result.GatingCount, result.Findings)
	}
}

// TestUnit_AuditDrift_FallsBackToCESpec proves auditDrift itself, not just
// resolveSpecOperation in isolation, actually uses the CE fallback when an
// action's OperationID exists only there — SystemUpgrade's real situation.
func TestUnit_AuditDrift_FallsBackToCESpec(t *testing.T) {
	t.Parallel()
	ceOps := singleFieldSpec("string", "")
	actions := []toolutil.ActionSpec{fixtureAction("Fixture", fixtureInputNoDescription{})}

	result, err := auditDrift(map[string]specOperation{}, ceOps, actions, nil)
	if err != nil {
		t.Fatalf("auditDrift() error = %v", err)
	}
	if result.HasDrift() {
		t.Errorf("auditDrift() HasDrift() = true, want false: the CE-only operation matches its CE spec; Findings = %+v", result.Findings)
	}
	if result.ActionsAudited != 1 {
		t.Errorf("auditDrift() ActionsAudited = %d, want 1", result.ActionsAudited)
	}
}
