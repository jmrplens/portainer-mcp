package main

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// TestUnit_BuildReport_CountsAppearBeforeAnyDetail proves the brief's
// explicit requirement: the counts must be visible at the top, unconditional
// on there being any detail below them, so a reader can see the size of the
// job before scrolling into a hundred-plus-line report.
func TestUnit_BuildReport_CountsAppearBeforeAnyDetail(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		BeforeCount: 427, AfterCount: 442,
		AddedCount: 20, RemovedCount: 5, ChangedCount: 26, ChangedStructCount: 12,
	}
	report := buildReport("ee-2.43.0.json", "ee-2.44.0.json", result)

	countsIdx := strings.Index(report, "Changed: 26")
	if countsIdx < 0 {
		t.Fatalf("buildReport() = %q, want it to contain the changed count line", report)
	}
	domainIdx := strings.Index(report, "widgets")
	if domainIdx >= 0 && domainIdx < countsIdx {
		t.Errorf("buildReport() prints domain detail (at %d) before the counts (at %d)", domainIdx, countsIdx)
	}
	for _, want := range []string{"Added:   20", "Removed: 5", "12 touching the generated input struct", "14 cosmetic only"} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() = %q, want it to contain %q", report, want)
		}
	}
}

// TestUnit_BuildReport_NoDifference_SaysSoPlainly is the clean-run case: a
// patch release with nothing to report must read as "nothing to do", not as
// blank output a reader could mistake for a tool that did not run.
func TestUnit_BuildReport_NoDifference_SaysSoPlainly(t *testing.T) {
	t.Parallel()
	report := buildReport("a.json", "b.json", &deltaResult{BeforeCount: 379, AfterCount: 379})
	if !strings.Contains(report, "No difference") {
		t.Errorf("buildReport() = %q, want it to say plainly that there is no difference", report)
	}
}

// TestUnit_BuildReport_AddedOperation_IsTaggedJudgement and its removed
// counterpart prove the judgement/mechanics split the brief calls "most of
// this tool's value" actually renders, not merely that the operation's name
// appears somewhere in the text.
func TestUnit_BuildReport_AddedOperation_IsTaggedJudgement(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		AddedCount: 1,
		Domains: []domainGroup{{
			Domain: "widgets",
			Added:  []opRef{{OperationID: "WidgetCreate", Method: "POST", Path: "/widgets"}},
		}},
	}
	report := buildReport("a", "b", result)
	if !strings.Contains(report, "WidgetCreate") || !strings.Contains(report, "JUDGEMENT") {
		t.Errorf("buildReport() = %q, want WidgetCreate tagged JUDGEMENT", report)
	}
}

func TestUnit_BuildReport_RemovedOperation_IsTaggedMechanical(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		RemovedCount: 1,
		Domains: []domainGroup{{
			Domain:  "widgets",
			Removed: []opRef{{OperationID: "WidgetDelete", Method: "DELETE", Path: "/widgets/{id}"}},
		}},
	}
	report := buildReport("a", "b", result)
	if !strings.Contains(report, "WidgetDelete") || !strings.Contains(report, "MECHANICAL") {
		t.Errorf("buildReport() = %q, want WidgetDelete tagged MECHANICAL", report)
	}
}

// TestUnit_BuildReport_ChangedStructOperation_ShowsTheFieldChange proves a
// changed operation's own FieldChange detail (field name, kind, before,
// after) is rendered, not summarised away — the level of detail a person
// actually edits code from.
func TestUnit_BuildReport_ChangedStructOperation_ShowsTheFieldChange(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		ChangedCount: 1, ChangedStructCount: 1,
		Domains: []domainGroup{{
			Domain: "endpoints",
			ChangedStruct: []changedOp{{
				OperationID: "EndpointList", Method: "GET", Path: "/endpoints",
				Changes:       []specdiff.FieldChange{{JSONName: "order", Kind: specdiff.ChangeType, Before: "integer", After: "string"}},
				TouchesStruct: true,
			}},
		}},
	}
	report := buildReport("a", "b", result)
	for _, want := range []string{"EndpointList", "order", "integer", "string", "JUDGEMENT"} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() = %q, want it to contain %q", report, want)
		}
	}
}
