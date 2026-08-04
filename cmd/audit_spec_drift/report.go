package main

import (
	"fmt"
	"strings"
)

// findingLine renders one driftFinding as a single report line, marking
// whether it gates the build and, if excused, why — the same "excluded from
// the failure, never from the report" principle cmd/audit_1to1's
// operationLine follows for its own allow-list.
func findingLine(f driftFinding) string {
	c := f.Change
	tag := "cosmetic"
	switch {
	case f.Gating && f.AllowListed:
		tag = "ALLOW-LISTED"
	case f.Gating:
		tag = "GATING"
	}
	line := fmt.Sprintf("    - %s.%s [%s] (spec: %s): %q -> %q [%s]\n",
		f.OperationID, c.JSONName, c.Kind, f.SpecEdition, c.Before, c.After, tag)
	if f.AllowListed {
		line += fmt.Sprintf("        excused: %s (added %s)\n", f.AllowListReason, f.AllowListAdded)
	}
	return line
}

// buildReport renders result as a human-readable summary, written to
// standard error (see this command's package doc for why never standard
// output).
//
// Every finding is printed, gating or not, allow-listed or not: an allow-list
// that can hide an entry from its own report is exactly how it turns from an
// honesty mechanism into a hiding place, the identical reasoning
// cmd/audit_1to1's buildReport states for its own allow-list.
func buildReport(result *auditResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP specification drift audit")
	fmt.Fprintln(&b, "=========================================")
	fmt.Fprintln(&b, "Canary self-test: passed (a deliberately perturbed shape was correctly reported as drift).")
	fmt.Fprintf(&b, "Actions audited: %d\n", result.ActionsAudited)
	fmt.Fprintf(&b, "Allow-list entries: %d\n\n", result.AllowListCount)

	var currentOp string
	for _, f := range result.Findings {
		if f.OperationID != currentOp {
			currentOp = f.OperationID
			fmt.Fprintf(&b, "%s (%s)\n", f.OperationID, f.ActionName)
		}
		fmt.Fprint(&b, findingLine(f))
	}
	if len(result.Findings) > 0 {
		fmt.Fprintln(&b)
	}

	if len(result.StaleEntries) > 0 {
		fmt.Fprintln(&b, "Stale allow-list entries (no matching finding — remove them):")
		for _, e := range result.StaleEntries {
			fmt.Fprintf(&b, "    - %s.%s: %q (added %s)\n", e.OperationID, e.Field, e.Reason, e.Added)
		}
		fmt.Fprintln(&b)
	}

	switch {
	case result.HasDrift():
		fmt.Fprintf(&b, "%d gating finding(s) and %d stale allow-list entr(y/ies): the catalog has drifted from the vendored specification.\n",
			result.GatingCount, len(result.StaleEntries))
	default:
		fmt.Fprintln(&b, "No drift: every audited action's parameter shape matches the vendored specification it was generated from.")
	}
	return b.String()
}
