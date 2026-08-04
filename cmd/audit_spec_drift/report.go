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
	case c.AfterOverridden:
		// A ChangeTitle/ChangeOperationDescription finding whose catalog
		// side is a deliberate toolutil.WithNarrative override (see
		// FieldChange.AfterOverridden's own doc comment) never gates — but
		// isGating not gating is not the same fact as "this is a copy-paste
		// prose difference nobody decided about", which is what "cosmetic"
		// means for every other non-gating finding here. Tagging it
		// distinctly keeps that difference visible in the report itself,
		// not only recoverable by a reader who already knows to go check
		// AfterOverridden in the source: 35 allow-list entries became 35
		// structurally-inert comparisons, and a reader of this report alone
		// should be able to tell "inert by design" from "inert because the
		// spec never described this field" without opening any code.
		tag = "OVERRIDDEN"
	}
	line := fmt.Sprintf("    - %s.%s [%s] (spec: %s): %q -> %q [%s]\n",
		f.OperationID, c.JSONName, c.Kind, f.SpecEdition, c.Before, c.After, tag)
	if f.AllowListed {
		line += fmt.Sprintf("        excused: %s (added %s)\n", f.AllowListReason, f.AllowListAdded)
	}
	return line
}

// credentialFindingLine renders one credentialFinding as a report line. There
// is no "ALLOW-LISTED" tag here, unlike findingLine: a leaked credential is
// never excused, only fixed.
func credentialFindingLine(f credentialFinding) string {
	return fmt.Sprintf("    - %s (%s): success response can carry %v, but Handler never calls %s [GATING]\n",
		f.OperationID, f.ActionName, f.Fields, f.WrapperName)
}

// minimumFindingLine renders one minimumFinding as a report line. Never
// allow-listable, like credentialFindingLine.
func minimumFindingLine(f minimumFinding) string {
	return fmt.Sprintf("    - %s (%s): path parameter %q is identifier-shaped but the catalog publishes no \"minimum\": 1 for it [GATING]\n",
		f.OperationID, f.ActionName, f.ParamName)
}

// buildReport renders result, credResult and minResult as a human-readable
// summary, written to standard error (see this command's package doc for why
// never standard output).
//
// Every finding is printed, gating or not, allow-listed or not: an allow-list
// that can hide an entry from its own report is exactly how it turns from an
// honesty mechanism into a hiding place, the identical reasoning
// cmd/audit_1to1's buildReport states for its own allow-list.
func buildReport(result *auditResult, credResult *credentialAuditResult, minResult *minimumAuditResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP specification drift audit")
	fmt.Fprintln(&b, "=========================================")
	fmt.Fprintln(&b, "Canary self-test: passed (a deliberately perturbed shape was correctly reported as drift).")
	fmt.Fprintln(&b, "Credential canary self-test: passed (a handler that calls its wrapper and one that does not were told apart).")
	fmt.Fprintln(&b, "Identifier-minimum canary self-test: passed (an action publishing \"minimum\":1 and one not were told apart).")
	fmt.Fprintf(&b, "Actions audited: %d\n", result.ActionsAudited)
	fmt.Fprintf(&b, "Fields audited: %d\n", result.FieldsAudited)
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

	fmt.Fprintf(&b, "Credential-shaped responses checked: %d\n", credResult.ActionsChecked)
	if len(credResult.Findings) > 0 {
		fmt.Fprintln(&b, "Credential redaction findings (never allow-listed):")
		for _, f := range credResult.Findings {
			fmt.Fprint(&b, credentialFindingLine(f))
		}
		fmt.Fprintln(&b)
	}

	fmt.Fprintf(&b, "Identifier-shaped path parameters checked: %d\n", minResult.ParamsChecked)
	if len(minResult.Findings) > 0 {
		fmt.Fprintln(&b, "Identifier-minimum findings (never allow-listed):")
		for _, f := range minResult.Findings {
			fmt.Fprint(&b, minimumFindingLine(f))
		}
		fmt.Fprintln(&b)
	}

	gatingCount := result.GatingCount
	staleCount := len(result.StaleEntries)
	credCount := len(credResult.Findings)
	minCount := len(minResult.Findings)
	switch {
	case result.HasDrift() || credResult.HasLeaks() || minResult.HasGaps():
		fmt.Fprintf(&b, "%d gating finding(s), %d stale allow-list entr(y/ies), %d credential-redaction finding(s) and %d identifier-minimum finding(s): the catalog has drifted from the vendored specification.\n",
			gatingCount, staleCount, credCount, minCount)
	default:
		fmt.Fprintln(&b, "No drift: every audited action's parameter shape matches the vendored specification it was generated from, every credential-shaped response is redacted, and every identifier-shaped path parameter publishes \"minimum\": 1.")
	}
	return b.String()
}
