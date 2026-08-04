package main

import (
	"fmt"
	"strings"
)

// opRefLine renders one added/removed operation reference as a single work
// list line, tagged with what kind of work it is: judgement for an addition
// (a name, a narrative and possibly a redaction wrapper are not mechanical
// decisions), mechanical for a removal (delete the action).
func opRefLine(ref opRef, tag string) string {
	return fmt.Sprintf("    - %s [%s %s] [%s]\n", ref.OperationID, ref.Method, ref.Path, tag)
}

// changedOpLines renders one changed operation and every FieldChange found
// for it, tagged the same way cmd/audit_spec_drift's findingLine tags a
// drift finding, so a reader who already knows that report's convention
// reads this one for free.
func changedOpLines(op changedOp, tag string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "    - %s [%s %s] [%s]\n", op.OperationID, op.Method, op.Path, tag)
	for _, c := range op.Changes {
		fmt.Fprintf(&b, "        %s [%s]: %q -> %q\n", c.JSONName, c.Kind, c.Before, c.After)
	}
	return b.String()
}

// unresolvedOpLines renders one operation this tool could not flatten on
// either side, with the refusal's own reason — see unresolvedOp's doc
// comment for why this happens and why it is surfaced rather than silently
// dropped or allowed to abort the rest of the report.
func unresolvedOpLines(op unresolvedOp) string {
	return fmt.Sprintf("    - %s [%s %s] [JUDGEMENT]\n        cannot compare automatically: %s\n",
		op.OperationID, op.Method, op.Path, op.Reason)
}

// buildReport renders result as a human-readable work list, written to
// standard error (see this command's package doc for why never standard
// output).
//
// The counts come first, unconditionally, so the size of the job is visible
// before a reader scrolls past a single line of detail — the brief this
// command implements states that explicitly, and at a hundred-plus changed
// operations between two published Portainer minor versions, a reader who
// has to count entries to learn the scope will not.
//
// Domains are listed alphabetically, and within a domain the four buckets
// always appear in the same order — added, removed, struct-changed,
// cosmetic-changed — regardless of which ones are actually present, so two
// runs over the same inputs produce byte-identical output and a reader
// building muscle memory for "where do I look first" is never surprised.
func buildReport(before, after string, result *deltaResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP specification delta")
	fmt.Fprintln(&b, "==================================")
	fmt.Fprintf(&b, "Before: %s (%d operations)\n", before, result.BeforeCount)
	fmt.Fprintf(&b, "After:  %s (%d operations)\n\n", after, result.AfterCount)

	fmt.Fprintf(&b, "Added:   %d\n", result.AddedCount)
	fmt.Fprintf(&b, "Removed: %d\n", result.RemovedCount)
	fmt.Fprintf(&b, "Changed: %d (%d touching the generated input struct, %d cosmetic only)\n",
		result.ChangedCount, result.ChangedStructCount, result.ChangedCount-result.ChangedStructCount)
	if result.UnresolvableCount > 0 {
		fmt.Fprintf(&b, "Unresolvable: %d (present on both sides, but this tool could not flatten their parameter shape — review by hand)\n", result.UnresolvableCount)
	}
	fmt.Fprintln(&b)

	if result.AddedCount == 0 && result.RemovedCount == 0 && result.ChangedCount == 0 && result.UnresolvableCount == 0 {
		fmt.Fprintln(&b, "No difference: every operation in before is unchanged in after, and neither side declares an operation the other lacks.")
		return b.String()
	}

	for _, g := range result.Domains {
		fmt.Fprintf(&b, "%s (%d added, %d removed, %d unresolvable, %d struct-changed, %d cosmetic-changed)\n",
			g.Domain, len(g.Added), len(g.Removed), len(g.Unresolvable), len(g.ChangedStruct), len(g.ChangedCosmetic))

		if len(g.Added) > 0 {
			fmt.Fprintln(&b, "  Added (JUDGEMENT: needs a name, a narrative, possibly a redaction wrapper):")
			for _, ref := range g.Added {
				fmt.Fprint(&b, opRefLine(ref, "JUDGEMENT"))
			}
		}
		if len(g.Removed) > 0 {
			fmt.Fprintln(&b, "  Removed (MECHANICAL: delete the action):")
			for _, ref := range g.Removed {
				fmt.Fprint(&b, opRefLine(ref, "MECHANICAL"))
			}
		}
		if len(g.Unresolvable) > 0 {
			fmt.Fprintln(&b, "  Unresolvable (JUDGEMENT: this tool cannot flatten the parameter shape — review by hand):")
			for _, op := range g.Unresolvable {
				fmt.Fprint(&b, unresolvedOpLines(op))
			}
		}
		if len(g.ChangedStruct) > 0 {
			fmt.Fprintln(&b, "  Changed, touches the input struct (JUDGEMENT: a field's name, type, requiredness or origin moved):")
			for _, op := range g.ChangedStruct {
				fmt.Fprint(&b, changedOpLines(op, "JUDGEMENT"))
			}
		}
		if len(g.ChangedCosmetic) > 0 {
			fmt.Fprintln(&b, "  Changed, cosmetic only (MECHANICAL: description or enum text, copy-paste):")
			for _, op := range g.ChangedCosmetic {
				fmt.Fprint(&b, changedOpLines(op, "MECHANICAL"))
			}
		}
		fmt.Fprintln(&b)
	}

	return b.String()
}
