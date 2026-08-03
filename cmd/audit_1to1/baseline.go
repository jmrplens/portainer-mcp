package main

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// baseline is the coverage ratchet's committed floor, loaded from
// api/coverage-baseline.yaml: the number of operations, per edition, the
// catalog already covers as of the last commit that touched this file.
// runRatchet's CI gate fails only when current coverage drops below these
// numbers; see runRatchet's own doc comment for why that is the right gate
// for CI while `make audit-1to1` (run, HasGap) stays the 100%-or-bust gate a
// human asking "are we done" wants.
type baseline struct {
	CECovered int `yaml:"ce_covered"`
	EECovered int `yaml:"ee_covered"`
}

// parseBaseline decodes the committed ratchet baseline file.
func parseBaseline(data []byte) (baseline, error) {
	var b baseline
	if err := yaml.Unmarshal(data, &b); err != nil {
		return baseline{}, fmt.Errorf("decode coverage baseline: %w", err)
	}
	if b.CECovered < 0 || b.EECovered < 0 {
		return baseline{}, fmt.Errorf("coverage baseline: ce_covered (%d) and ee_covered (%d) must not be negative", b.CECovered, b.EECovered)
	}
	return b, nil
}

// ratchetOutcome is the ratchet's per-edition verdict: how current coverage
// (Current) compares to the committed floor (Baseline) for one edition.
type ratchetOutcome struct {
	Name      string
	Current   int
	Baseline  int
	Regressed bool
	Stale     bool
}

// checkRatchet compares result's per-edition covered counts against base,
// one outcome per edition. Regressed (current < baseline) is what
// runRatchet's caller must fail the build on: coverage has gotten worse than
// the last commit that touched the baseline recorded. Stale (current >
// baseline) is not a failure — coverage has improved, which is exactly what
// P3 is supposed to do — but is worth saying plainly, because the committed
// number no longer reflects reality and should be updated in the same commit
// that improved it: that is what makes the improvement itself visible in the
// diff, rather than silently absorbed by a ratchet that just keeps
// tightening on its own.
func checkRatchet(result *auditResult, base baseline) []ratchetOutcome {
	return []ratchetOutcome{
		{
			Name: result.EE.Name, Current: result.EE.Covered, Baseline: base.EECovered,
			Regressed: result.EE.Covered < base.EECovered,
			Stale:     result.EE.Covered > base.EECovered,
		},
		{
			Name: result.CE.Name, Current: result.CE.Covered, Baseline: base.CECovered,
			Regressed: result.CE.Covered < base.CECovered,
			Stale:     result.CE.Covered > base.CECovered,
		},
	}
}

// ratchetRegressed reports whether any outcome regressed — the boolean
// runRatchet's caller gates the build on.
func ratchetRegressed(outcomes []ratchetOutcome) bool {
	for _, o := range outcomes {
		if o.Regressed {
			return true
		}
	}
	return false
}

// renderRatchet renders outcomes as a human-readable summary, stating both
// numbers — current and baseline — for every edition plainly, so nobody has
// to open api/coverage-baseline.yaml to know where the gate stands.
func renderRatchet(outcomes []ratchetOutcome) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Coverage ratchet")
	fmt.Fprintln(&b, "================")
	for _, o := range outcomes {
		status := "ok"
		switch {
		case o.Regressed:
			status = "REGRESSED: coverage dropped below the committed baseline"
		case o.Stale:
			status = "baseline is stale: coverage improved beyond it — update api/coverage-baseline.yaml in this commit"
		}
		fmt.Fprintf(&b, "  %s: current %d, baseline %d (%s)\n", o.Name, o.Current, o.Baseline, status)
	}
	if ratchetRegressed(outcomes) {
		fmt.Fprintln(&b, "\nCoverage regressed below the baseline committed in api/coverage-baseline.yaml.")
	} else {
		fmt.Fprintln(&b, "\nCoverage ratchet holds: no edition dropped below its committed baseline.")
	}
	return b.String()
}
