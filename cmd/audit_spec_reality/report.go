package main

import (
	"fmt"
	"strings"
)

// buildReport renders a human-readable summary of every leg's audit,
// unambiguous about counts (never a bare number that could be mistaken for
// full coverage) and about what each section actually means: "divergent"
// names an operation this server does not serve at all, distinct from a
// probe that could not run.
func buildReport(results []legResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP spec-vs-reality audit")
	fmt.Fprintln(&b, "====================================")
	fmt.Fprintln(&b, "Reports only; does not gate. A divergence here is a fact about the")
	fmt.Fprintln(&b, "Portainer server, not a defect in this project's code.")

	totalDivergent := 0
	for _, r := range results {
		totalDivergent += len(r.Divergent)

		fmt.Fprintf(&b, "\n%s leg: %d documented operation(s) probed\n", r.Leg, r.Total)

		if len(r.ProbeErrors) > 0 {
			fmt.Fprintf(&b, "  %d probe(s) could not run at all (transport error, not a divergence):\n", len(r.ProbeErrors))
			for _, e := range r.ProbeErrors {
				fmt.Fprintf(&b, "    - %s\n", e)
			}
		}

		if len(r.Divergent) == 0 {
			fmt.Fprintln(&b, "  No divergence: every documented operation is served by a real route.")
			continue
		}

		fmt.Fprintf(&b, "  %d of %d documented operation(s) are NOT served: this server answers with\n", len(r.Divergent), r.Total)
		fmt.Fprintln(&b, "  Go's literal default \"404 page not found\" for a route the specification documents.")
		for _, d := range r.Divergent {
			fmt.Fprintf(&b, "    - %s: %s %s (tag %q)\n", d.OperationID, d.Method, d.Path, d.Domain)
		}
	}

	fmt.Fprintf(&b, "\nTotal divergent operations across all probed legs: %d\n", totalDivergent)
	return b.String()
}
