package main

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// TestUnit_BuildReport_CleanResult_SaysNoDrift covers the positive case: no
// findings at all must read plainly as "no drift", not as an absence of
// output a reader could mistake for "nothing ran".
func TestUnit_BuildReport_CleanResult_SaysNoDrift(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport CleanResult SaysNoDrift", func(t *testing.T) {
		report := buildReport(&auditResult{ActionsAudited: 19, FieldsAudited: 51}, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		if !strings.Contains(report, "No drift") {
			t.Errorf("buildReport() = %q, want it to say plainly that no drift was found", report)
		}
		if strings.Contains(report, "GATING") {
			t.Errorf("buildReport() = %q, want no GATING tag when there are no findings", report)
		}
		if !strings.Contains(report, "Fields audited: 51") {
			t.Errorf("buildReport() = %q, want the field count printed alongside the action count: that is what distinguishes \"no drift\" from \"nothing compared\"", report)
		}
	})
}

// TestUnit_BuildReport_GatingFinding_IsNamed proves a real gating finding is
// printed with enough detail to act on: the operation, the field, the kind
// and both values.
func TestUnit_BuildReport_GatingFinding_IsNamed(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport GatingFinding IsNamed", func(t *testing.T) {
		result := &auditResult{
			ActionsAudited: 1,
			GatingCount:    1,
			Findings: []driftFinding{{
				OperationID: "Fixture", ActionName: "fixture.action", SpecEdition: "EE",
				Change: specdiff.FieldChange{JSONName: "field", Kind: specdiff.ChangeType, Before: "integer", After: "string"},
				Gating: true,
			}},
		}
		report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		for _, want := range []string{"Fixture", "field", "integer", "string", "GATING"} {
			if !strings.Contains(report, want) {
				t.Errorf("buildReport() = %q, want it to contain %q", report, want)
			}
		}
	})
}

// TestUnit_BuildReport_AllowListedFinding_ShowsReasonAndIsNotHidden proves
// the allow-list is never a hiding place: an excused finding is still
// printed, tagged distinctly from an un-excused one, with its reason.
func TestUnit_BuildReport_AllowListedFinding_ShowsReasonAndIsNotHidden(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport AllowListedFinding ShowsReasonAndIsNotHidden", func(t *testing.T) {
		result := &auditResult{
			ActionsAudited: 1,
			Findings: []driftFinding{{
				OperationID: "SystemUpgrade", ActionName: "system.upgrade", SpecEdition: "CE",
				Change:          specdiff.FieldChange{JSONName: "field", Kind: specdiff.ChangeType, Before: "integer", After: "string"},
				Gating:          true,
				AllowListed:     true,
				AllowListReason: "hand-written pilot narrows this type on purpose",
				AllowListAdded:  "2026-08-04",
			}},
		}
		report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		if !strings.Contains(report, "ALLOW-LISTED") {
			t.Errorf("buildReport() = %q, want the ALLOW-LISTED tag", report)
		}
		if !strings.Contains(report, "hand-written pilot narrows this type on purpose") {
			t.Errorf("buildReport() = %q, want the excusing reason printed", report)
		}
		if !strings.Contains(report, "SystemUpgrade") {
			t.Errorf("buildReport() = %q, want the operation named even though it is excused", report)
		}
	})
}

// TestUnit_BuildReport_StaleEntry_IsNamed proves a stale allow-list entry is
// reported by name, not merely counted — the same principle as
// cmd/audit_1to1's own report for uncovered operations.
func TestUnit_BuildReport_StaleEntry_IsNamed(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport StaleEntry IsNamed", func(t *testing.T) {
		result := &auditResult{
			ActionsAudited: 1,
			StaleEntries: []allowListEntry{
				{OperationID: "SystemUpgrade", Field: "field", Reason: "no longer diverges", Added: "2026-08-04"},
			},
		}
		report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		if !strings.Contains(report, "Stale allow-list") {
			t.Errorf("buildReport() = %q, want a stale-entries section", report)
		}
		if !strings.Contains(report, "SystemUpgrade") || !strings.Contains(report, "field") {
			t.Errorf("buildReport() = %q, want the stale entry named", report)
		}
	})
}

// TestUnit_BuildReport_CosmeticFinding_IsMarkedDistinctlyFromGating proves a
// non-gating ChangeDescription finding (spec never described the field) is
// visible but tagged so a reader does not mistake it for something that
// fails the build.
func TestUnit_BuildReport_CosmeticFinding_IsMarkedDistinctlyFromGating(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport CosmeticFinding IsMarkedDistinctlyFromGating", func(t *testing.T) {
		result := &auditResult{
			ActionsAudited: 1,
			Findings: []driftFinding{{
				OperationID: "Fixture", ActionName: "fixture.action", SpecEdition: "EE",
				Change: specdiff.FieldChange{JSONName: "field", Kind: specdiff.ChangeDescription, Before: "", After: "hand-written description"},
				Gating: false,
			}},
		}
		report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		if !strings.Contains(report, "cosmetic") {
			t.Errorf("buildReport() = %q, want the non-gating finding tagged cosmetic", report)
		}
		if strings.Contains(report, "GATING") {
			t.Errorf("buildReport() = %q, want no GATING tag for a non-gating finding", report)
		}
	})
}

// TestUnit_BuildReport_OverriddenFinding_IsMarkedDistinctlyFromCosmetic
// proves a ChangeTitle/ChangeOperationDescription finding whose catalog side
// is a deliberate toolutil.WithNarrative override (AfterOverridden) is
// tagged [OVERRIDDEN], not folded into the same "cosmetic" bucket as a
// finding nobody decided about (the spec simply never described that
// field). isGating already treats the two the same way (neither gates), but
// the report must not — a reader must be able to tell "inert by design"
// from "inert because the spec never described this field" without opening
// the source and checking AfterOverridden by hand.
func TestUnit_BuildReport_OverriddenFinding_IsMarkedDistinctlyFromCosmetic(t *testing.T) {
	t.Parallel()
	t.Run("BuildReport OverriddenFinding IsMarkedDistinctlyFromCosmetic", func(t *testing.T) {
		result := &auditResult{
			ActionsAudited: 1,
			Findings: []driftFinding{{
				OperationID: "TagDelete", ActionName: "tags.delete", SpecEdition: "EE",
				Change: specdiff.FieldChange{
					JSONName: specdiff.TitleSentinel, Kind: specdiff.ChangeTitle,
					Before: "Delete a tag", After: "Remove an environment tag",
					AfterOverridden: true,
				},
				Gating: false,
			}},
		}
		report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{}, &editionFieldCountsResult{})
		if !strings.Contains(report, "OVERRIDDEN") {
			t.Errorf("buildReport() = %q, want the overridden finding tagged OVERRIDDEN", report)
		}
		if strings.Contains(report, "[cosmetic]") {
			t.Errorf("buildReport() = %q, want no [cosmetic] tag once OVERRIDDEN applies", report)
		}
		if strings.Contains(report, "GATING") {
			t.Errorf("buildReport() = %q, want no GATING tag for an overridden, non-gating finding", report)
		}
	})
}
