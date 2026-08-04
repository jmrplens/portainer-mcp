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
	report := buildReport(&auditResult{ActionsAudited: 19}, &credentialAuditResult{}, &minimumAuditResult{})
	if !strings.Contains(report, "No drift") {
		t.Errorf("buildReport() = %q, want it to say plainly that no drift was found", report)
	}
	if strings.Contains(report, "GATING") {
		t.Errorf("buildReport() = %q, want no GATING tag when there are no findings", report)
	}
}

// TestUnit_BuildReport_GatingFinding_IsNamed proves a real gating finding is
// printed with enough detail to act on: the operation, the field, the kind
// and both values.
func TestUnit_BuildReport_GatingFinding_IsNamed(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		ActionsAudited: 1,
		GatingCount:    1,
		Findings: []driftFinding{{
			OperationID: "Fixture", ActionName: "fixture.action", SpecEdition: "EE",
			Change: specdiff.FieldChange{JSONName: "field", Kind: specdiff.ChangeType, Before: "integer", After: "string"},
			Gating: true,
		}},
	}
	report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{})
	for _, want := range []string{"Fixture", "field", "integer", "string", "GATING"} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() = %q, want it to contain %q", report, want)
		}
	}
}

// TestUnit_BuildReport_AllowListedFinding_ShowsReasonAndIsNotHidden proves
// the allow-list is never a hiding place: an excused finding is still
// printed, tagged distinctly from an un-excused one, with its reason.
func TestUnit_BuildReport_AllowListedFinding_ShowsReasonAndIsNotHidden(t *testing.T) {
	t.Parallel()
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
	report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{})
	if !strings.Contains(report, "ALLOW-LISTED") {
		t.Errorf("buildReport() = %q, want the ALLOW-LISTED tag", report)
	}
	if !strings.Contains(report, "hand-written pilot narrows this type on purpose") {
		t.Errorf("buildReport() = %q, want the excusing reason printed", report)
	}
	if !strings.Contains(report, "SystemUpgrade") {
		t.Errorf("buildReport() = %q, want the operation named even though it is excused", report)
	}
}

// TestUnit_BuildReport_StaleEntry_IsNamed proves a stale allow-list entry is
// reported by name, not merely counted — the same principle as
// cmd/audit_1to1's own report for uncovered operations.
func TestUnit_BuildReport_StaleEntry_IsNamed(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		ActionsAudited: 1,
		StaleEntries: []allowListEntry{
			{OperationID: "SystemUpgrade", Field: "field", Reason: "no longer diverges", Added: "2026-08-04"},
		},
	}
	report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{})
	if !strings.Contains(report, "Stale allow-list") {
		t.Errorf("buildReport() = %q, want a stale-entries section", report)
	}
	if !strings.Contains(report, "SystemUpgrade") || !strings.Contains(report, "field") {
		t.Errorf("buildReport() = %q, want the stale entry named", report)
	}
}

// TestUnit_BuildReport_CosmeticFinding_IsMarkedDistinctlyFromGating proves a
// non-gating ChangeDescription finding (spec never described the field) is
// visible but tagged so a reader does not mistake it for something that
// fails the build.
func TestUnit_BuildReport_CosmeticFinding_IsMarkedDistinctlyFromGating(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		ActionsAudited: 1,
		Findings: []driftFinding{{
			OperationID: "Fixture", ActionName: "fixture.action", SpecEdition: "EE",
			Change: specdiff.FieldChange{JSONName: "field", Kind: specdiff.ChangeDescription, Before: "", After: "hand-written description"},
			Gating: false,
		}},
	}
	report := buildReport(result, &credentialAuditResult{}, &minimumAuditResult{})
	if !strings.Contains(report, "cosmetic") {
		t.Errorf("buildReport() = %q, want the non-gating finding tagged cosmetic", report)
	}
	if strings.Contains(report, "GATING") {
		t.Errorf("buildReport() = %q, want no GATING tag for a non-gating finding", report)
	}
}
