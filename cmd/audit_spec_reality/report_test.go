package main

import (
	"strings"
	"testing"
)

// TestUnit_BuildReport_NoDivergence_StatesSoPlainly guards against a report
// that says nothing when it found nothing: silence would read the same as
// coverage nobody checked.
func TestUnit_BuildReport_NoDivergence_StatesSoPlainly(t *testing.T) {
	t.Parallel()
	report := buildReport([]legResult{{Leg: "CE", Total: 3}})
	if !strings.Contains(report, "No divergence") {
		t.Errorf("buildReport() = %q, want it to state plainly that nothing diverged", report)
	}
	if !strings.Contains(report, "CE leg: 3 documented operation(s) probed") {
		t.Errorf("buildReport() = %q, want the CE total stated", report)
	}
}

// TestUnit_BuildReport_ListsEveryDivergentOperationByName proves a found
// divergence is not summarised away: each operation's id, verb and path
// must appear, since a wave reading this report needs to know exactly which
// operation to investigate.
func TestUnit_BuildReport_ListsEveryDivergentOperationByName(t *testing.T) {
	t.Parallel()
	report := buildReport([]legResult{{
		Leg:   "EE",
		Total: 441,
		Divergent: []legDivergence{
			{OperationID: "AddonInstall", Method: "POST", Path: "/addons/{id}", Domain: "addons", StatusCode: 404},
		},
	}})
	for _, want := range []string{"AddonInstall", "POST", "/addons/{id}", "addons"} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() = %q, want it to contain %q", report, want)
		}
	}
	if !strings.Contains(report, "1 of 441") {
		t.Errorf("buildReport() = %q, want the divergent-of-total count stated unambiguously", report)
	}
}

// TestUnit_BuildReport_ProbeErrorsListedSeparatelyFromDivergence proves a
// transport failure is never presented as if it were a divergence: the two
// mean different things (see legResult's own doc comment) and a report that
// conflated them would overstate what was actually observed.
func TestUnit_BuildReport_ProbeErrorsListedSeparatelyFromDivergence(t *testing.T) {
	t.Parallel()
	report := buildReport([]legResult{{
		Leg:         "CE",
		Total:       2,
		ProbeErrors: []string{"SomeOp (GET /some/op): connection refused"},
	}})
	if !strings.Contains(report, "could not run at all") {
		t.Errorf("buildReport() = %q, want probe errors labelled distinctly from divergence", report)
	}
	if !strings.Contains(report, "No divergence") {
		t.Errorf("buildReport() = %q, want it to still state no divergence was found", report)
	}
}

// TestUnit_BuildReport_TotalAcrossLegs_SumsCorrectly proves the closing
// total is a real sum, not a copy of one leg's count.
func TestUnit_BuildReport_TotalAcrossLegs_SumsCorrectly(t *testing.T) {
	t.Parallel()
	report := buildReport([]legResult{
		{Leg: "CE", Total: 5, Divergent: []legDivergence{{OperationID: "A"}, {OperationID: "B"}}},
		{Leg: "EE", Total: 5, Divergent: []legDivergence{{OperationID: "C"}}},
	})
	if !strings.Contains(report, "Total divergent operations across all probed legs: 3") {
		t.Errorf("buildReport() = %q, want the combined total (2+1=3) stated", report)
	}
}
