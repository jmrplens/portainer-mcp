package main

import (
	"fmt"
	"strings"
)

// buildReport renders a human-readable summary of every leg's audit,
// unambiguous about counts (never a bare number that could be mistaken for
// full coverage) and about what each section actually means. Four outcomes,
// deliberately never merged into one number: "divergent" names an operation
// this server does not serve at all; "wrong verb" names one it serves under
// a different method than the specification documents; "unmeasured" names a
// public route that was not probed; and a probe error names one that could
// not run.
func buildReport(results []legResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP spec-vs-reality audit")
	fmt.Fprintln(&b, "====================================")
	fmt.Fprintln(&b, "Reports only; does not gate. A divergence here is a fact about the")
	fmt.Fprintln(&b, "Portainer server, not a defect in this project's code.")

	totalDivergent := 0
	totalWrongVerb := 0
	for _, r := range results {
		totalDivergent += len(r.Divergent)
		totalWrongVerb += len(r.WrongVerb)

		// r.Total counts every documented operation, including the ones
		// SkippedPublic records as unmeasured — reporting r.Total alone as
		// "probed" would overstate what this run actually observed.
		probed := r.Total - len(r.SkippedPublic)
		fmt.Fprintf(&b, "\n%s leg: %d documented operation(s), %d probed\n", r.Leg, r.Total, probed)

		if len(r.SkippedPublic) > 0 {
			fmt.Fprintf(&b, "  %d PublicAccess operation(s) were NOT probed: this estate could not be confirmed\n", len(r.SkippedPublic))
			fmt.Fprintln(&b, "  initialized, and a public route has no credential check to make probing it safe.")
			fmt.Fprintln(&b, "  These are neither served nor divergent — they are unmeasured:")
			for _, s := range r.SkippedPublic {
				fmt.Fprintf(&b, "    - %s\n", s)
			}
		}

		if len(r.ProbeErrors) > 0 {
			fmt.Fprintf(&b, "  %d probe(s) could not run at all (transport error, not a divergence):\n", len(r.ProbeErrors))
			for _, e := range r.ProbeErrors {
				fmt.Fprintf(&b, "    - %s\n", e)
			}
		}

		if len(r.WrongVerb) > 0 {
			fmt.Fprintf(&b, "  %d documented operation(s) are served under a DIFFERENT VERB than documented:\n", len(r.WrongVerb))
			fmt.Fprintln(&b, "  the path is registered and answers 405 to the method the specification names.")
			fmt.Fprintln(&b, "  An action generated from the document is uncallable until its handler is corrected.")
			for _, w := range r.WrongVerb {
				served := "no other verb answered either"
				if len(w.ServedBy) > 0 {
					served = "served by " + strings.Join(w.ServedBy, ", ")
				}
				fmt.Fprintf(&b, "    - %s: documents %s %s (tag %q) — %s\n",
					w.OperationID, w.Method, w.Path, w.Domain, served)
			}
		}

		if len(r.Divergent) == 0 {
			switch {
			case len(r.SkippedPublic) > 0:
				fmt.Fprintln(&b, "  No absent route among the operations that were probed; the skipped ones above remain unmeasured.")
			case len(r.WrongVerb) > 0:
				fmt.Fprintln(&b, "  No absent route: every documented operation's path is registered (see the verb findings above).")
			default:
				fmt.Fprintln(&b, "  No divergence: every documented operation is served by a real route, under the verb documented.")
			}
			continue
		}

		fmt.Fprintf(&b, "  %d of %d documented operation(s) are NOT served: this server answers with\n", len(r.Divergent), probed)
		fmt.Fprintln(&b, "  Go's literal default \"404 page not found\" for a route the specification documents.")
		for _, d := range r.Divergent {
			fmt.Fprintf(&b, "    - %s: %s %s (tag %q)\n", d.OperationID, d.Method, d.Path, d.Domain)
		}
	}

	fmt.Fprintf(&b, "\nTotal operations not served at all, across all probed legs: %d\n", totalDivergent)
	fmt.Fprintf(&b, "Total operations documented under the wrong verb:            %d\n", totalWrongVerb)
	return b.String()
}
