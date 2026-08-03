package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestUnit_ParseBaseline_ValidYAML_Parses(t *testing.T) {
	t.Parallel()
	b, err := parseBaseline([]byte("ce_covered: 15\nee_covered: 18\n"))
	if err != nil {
		t.Fatalf("parseBaseline() error = %v", err)
	}
	if b.CECovered != 15 || b.EECovered != 18 {
		t.Errorf("parseBaseline() = %+v, want CECovered=15 EECovered=18", b)
	}
}

func TestUnit_ParseBaseline_MalformedYAML_IsError(t *testing.T) {
	t.Parallel()
	if _, err := parseBaseline([]byte("not: [valid: yaml")); err == nil {
		t.Fatal("parseBaseline() = nil error, want one for malformed YAML")
	}
}

func TestUnit_ParseBaseline_NegativeCount_IsError(t *testing.T) {
	t.Parallel()
	if _, err := parseBaseline([]byte("ce_covered: -1\nee_covered: 18\n")); err == nil {
		t.Fatal("parseBaseline() = nil error, want one for a negative ce_covered")
	}
}

// TestUnit_CheckRatchet_CoverageBelowBaseline_Regresses is the ratchet's core
// property, proven by mutation: current coverage below the committed
// baseline must report Regressed for that edition, which is what
// runRatchet's caller fails the build on.
func TestUnit_CheckRatchet_CoverageBelowBaseline_Regresses(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		EE: editionReport{Name: "Business Edition (EE)", Covered: 10},
		CE: editionReport{Name: "Community Edition (CE)", Covered: 15},
	}
	base := baseline{EECovered: 18, CECovered: 15}

	outcomes := checkRatchet(result, base)
	var ee ratchetOutcome
	for _, o := range outcomes {
		if o.Name == "Business Edition (EE)" {
			ee = o
		}
	}
	if !ee.Regressed {
		t.Errorf("checkRatchet() EE outcome = %+v, want Regressed=true: current 10 < baseline 18", ee)
	}
	if ee.Stale {
		t.Errorf("checkRatchet() EE outcome = %+v, want Stale=false when coverage regressed", ee)
	}
	if !ratchetRegressed(outcomes) {
		t.Error("ratchetRegressed(outcomes) = false, want true: EE regressed")
	}
}

// TestUnit_CheckRatchet_CoverageEqualsBaseline_Passes proves the exact-match
// case neither regresses nor is reported stale.
func TestUnit_CheckRatchet_CoverageEqualsBaseline_Passes(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		EE: editionReport{Name: "Business Edition (EE)", Covered: 18},
		CE: editionReport{Name: "Community Edition (CE)", Covered: 15},
	}
	base := baseline{EECovered: 18, CECovered: 15}

	outcomes := checkRatchet(result, base)
	if ratchetRegressed(outcomes) {
		t.Fatal("ratchetRegressed(outcomes) = true, want false: coverage exactly matches the baseline")
	}
	for _, o := range outcomes {
		if o.Stale {
			t.Errorf("checkRatchet() outcome %+v, want Stale=false when current equals baseline", o)
		}
	}
}

// TestUnit_CheckRatchet_CoverageAboveBaseline_PassesAndReportsStale proves
// the third state: an improvement must not fail the gate, but must be
// reported so whoever lands it updates the committed file in the same
// commit.
func TestUnit_CheckRatchet_CoverageAboveBaseline_PassesAndReportsStale(t *testing.T) {
	t.Parallel()
	result := &auditResult{
		EE: editionReport{Name: "Business Edition (EE)", Covered: 25},
		CE: editionReport{Name: "Community Edition (CE)", Covered: 15},
	}
	base := baseline{EECovered: 18, CECovered: 15}

	outcomes := checkRatchet(result, base)
	if ratchetRegressed(outcomes) {
		t.Fatal("ratchetRegressed(outcomes) = true, want false: coverage improved, it must not fail the gate")
	}
	var ee ratchetOutcome
	for _, o := range outcomes {
		if o.Name == "Business Edition (EE)" {
			ee = o
		}
	}
	if !ee.Stale {
		t.Errorf("checkRatchet() EE outcome = %+v, want Stale=true: current 25 > baseline 18", ee)
	}
	if ee.Regressed {
		t.Errorf("checkRatchet() EE outcome = %+v, want Regressed=false when coverage improved", ee)
	}
}

// TestUnit_RenderRatchet_StatesCurrentAndBaselinePlainly proves the report
// text actually carries both numbers for every edition, so nobody has to
// open api/coverage-baseline.yaml to know where the gate stands.
func TestUnit_RenderRatchet_StatesCurrentAndBaselinePlainly(t *testing.T) {
	t.Parallel()
	outcomes := []ratchetOutcome{
		{Name: "Business Edition (EE)", Current: 25, Baseline: 18, Stale: true},
		{Name: "Community Edition (CE)", Current: 15, Baseline: 15},
	}
	report := renderRatchet(outcomes)
	if !strings.Contains(report, "Business Edition (EE): current 25, baseline 18") {
		t.Errorf("renderRatchet() does not state EE's current and baseline numbers:\n%s", report)
	}
	if !strings.Contains(report, "Community Edition (CE): current 15, baseline 15") {
		t.Errorf("renderRatchet() does not state CE's current and baseline numbers:\n%s", report)
	}
	if !strings.Contains(report, "stale") {
		t.Errorf("renderRatchet() does not say the baseline is stale for the improved edition:\n%s", report)
	}
}

// TestUnit_RenderRatchet_Regressed_SaysSo is the discriminating counterpart:
// a regressed outcome's report text must say so, distinctly from "ok" or
// "stale".
func TestUnit_RenderRatchet_Regressed_SaysSo(t *testing.T) {
	t.Parallel()
	outcomes := []ratchetOutcome{
		{Name: "Business Edition (EE)", Current: 10, Baseline: 18, Regressed: true},
	}
	report := renderRatchet(outcomes)
	if !strings.Contains(report, "REGRESSED") {
		t.Errorf("renderRatchet() does not flag the regression:\n%s", report)
	}
}

// TestUnit_RunRatchet_CoverageBelowBaseline_ReturnsError is the end-to-end
// proof through runRatchet itself, the function main wraps with os.Exit(1):
// a baseline overstating today's real coverage must fail the CI gate.
func TestUnit_RunRatchet_CoverageBelowBaseline_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ids := realCatalogOperationIDs(t)
	writeFixture(t, dir, "ee.json", syntheticEESpec(t, ids))
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "allowlist.yaml", "[]\n")
	// The real catalog covers len(ids) EE operations; claim one more than
	// that was already covered, which the fixture cannot possibly show.
	writeFixture(t, dir, "baseline.yaml", fmt.Sprintf("ce_covered: 0\nee_covered: %d\n", len(ids)+1))

	var out strings.Builder
	err := runRatchet(&out, dir, "ce.json", "ee.json", dir, "allowlist.yaml", dir, "baseline.yaml")
	if err == nil {
		t.Fatal("runRatchet() = nil error, want one: current EE coverage is below the inflated baseline")
	}
	if !strings.Contains(out.String(), "REGRESSED") {
		t.Errorf("runRatchet() report does not flag the regression:\n%s", out.String())
	}
}

// TestUnit_RunRatchet_CoverageMeetsBaseline_ReturnsNil is the positive
// control: a baseline matching real coverage exactly must pass.
func TestUnit_RunRatchet_CoverageMeetsBaseline_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ids := realCatalogOperationIDs(t)
	writeFixture(t, dir, "ee.json", syntheticEESpec(t, ids))
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "allowlist.yaml", "[]\n")
	writeFixture(t, dir, "baseline.yaml", fmt.Sprintf("ce_covered: 0\nee_covered: %d\n", len(ids)))

	var out strings.Builder
	if err := runRatchet(&out, dir, "ce.json", "ee.json", dir, "allowlist.yaml", dir, "baseline.yaml"); err != nil {
		t.Fatalf("runRatchet() error = %v, want nil: coverage exactly meets the baseline\n%s", err, out.String())
	}
}

// TestUnit_RunRatchet_CoverageAboveBaseline_ReturnsNilButReportsStale proves
// runRatchet itself, not just checkRatchet in isolation, passes on an
// improvement while still saying the baseline is stale.
func TestUnit_RunRatchet_CoverageAboveBaseline_ReturnsNilButReportsStale(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ids := realCatalogOperationIDs(t)
	writeFixture(t, dir, "ee.json", syntheticEESpec(t, ids))
	writeFixture(t, dir, "ce.json", `{"paths": {}}`)
	writeFixture(t, dir, "allowlist.yaml", "[]\n")
	writeFixture(t, dir, "baseline.yaml", "ce_covered: 0\nee_covered: 0\n")

	var out strings.Builder
	if err := runRatchet(&out, dir, "ce.json", "ee.json", dir, "allowlist.yaml", dir, "baseline.yaml"); err != nil {
		t.Fatalf("runRatchet() error = %v, want nil: coverage exceeds the baseline\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "stale") {
		t.Errorf("runRatchet() report does not say the baseline is stale:\n%s", out.String())
	}
}
