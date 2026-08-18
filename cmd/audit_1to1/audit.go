package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// editionReport is one edition's coverage summary.
type editionReport struct {
	// Name is the heading used in the rendered report, such as
	// "Business Edition (EE)".
	Name    string
	Total   int
	Covered int
	// AllowListed and Uncovered are sorted by OperationID so the report is
	// stable across runs.
	AllowListed []specOperation
	Uncovered   []specOperation
	// Unnamed are the routes this edition's document declares with no
	// operationId and that internal/specnaming does not name either. They
	// are not in Total, because Total counts operations this audit has a key
	// for and these have none; they are reported by name so that "not
	// counted" can never again mean "not visible". See parseSpecOperations.
	Unnamed []unnamedOperation
}

// auditResult is the outcome of comparing the declared catalog against both
// vendored specs.
type auditResult struct {
	CE, EE         editionReport
	AllowListCount int
	// Aliases are the operationId pairs that were treated as one route while
	// deciding coverage. Carried into the result, and printed by buildReport,
	// for the reason the allow-list count is: a mechanism that changes what
	// "covered" means and does not appear in the report is a mechanism nobody
	// re-reads.
	Aliases []operationAlias
}

// HasGap reports whether either edition has an operation with no catalog
// action and no allow-list entry — the property that must fail the build.
func (r *auditResult) HasGap() bool {
	return len(r.CE.Uncovered) > 0 || len(r.EE.Uncovered) > 0
}

// auditCoverage compares every declared action's OperationID against both
// vendored specs' operation sets.
//
// Both cross-checks below run before any coverage counting, and both are
// fatal rather than folded into the counts, because each is a defect in the
// inputs rather than a coverage gap:
//
//   - An allow-list entry naming an operation absent from both specs is
//     forgiving something that no longer exists; letting it through would
//     mean a real gap could later reuse that operation's name and inherit
//     the forgiveness.
//   - An action whose OperationID resolves in neither spec is a typo or a
//     stale declaration. actioncatalog.Build already refuses to build in
//     this case (against the version-applicability table), but this is the
//     only place that checks both specs loaded for this audit at once,
//     independent of that table.
//   - An alias entry that no longer names one route under two names is
//     forgiving a difference that has changed shape underneath it. See
//     checkAliases, which is what this delegates that third check to.
//
// aliases is what keeps the strict operationId key from going blind on a
// route the two documents name differently: covering either name covers
// both. It is a parameter rather than read from operationAliases directly so
// that a test can drive this with a synthetic table — a test asserting
// something about the real one only would pass just as happily against an
// implementation that never consulted a table at all.
func auditCoverage(ce, ee specDocument, actions []toolutil.ActionSpec, allowList []allowListEntry, aliases []operationAlias) (*auditResult, error) {
	if err := checkAliases(aliases, ce.Operations, ee.Operations); err != nil {
		return nil, fmt.Errorf("validating the operation aliases before auditing coverage: %w", err)
	}

	inEither := func(id string) bool {
		_, inCE := ce.Operations[id]
		_, inEE := ee.Operations[id]
		return inCE || inEE
	}

	for _, entry := range allowList {
		if !inEither(entry.OperationID) {
			return nil, fmt.Errorf(
				"allow-list entry %q: no such operation in either vendored spec", entry.OperationID)
		}
	}

	coveredIDs := make(map[string]bool, len(actions))
	for _, action := range actions {
		if !inEither(action.OperationID) {
			return nil, fmt.Errorf(
				"action %q: OperationID %q resolves in neither vendored spec", action.Name, action.OperationID)
		}
		coveredIDs[action.OperationID] = true
	}

	allowListedIDs := make(map[string]bool, len(allowList))
	for _, entry := range allowList {
		allowListedIDs[entry.OperationID] = true
	}

	// Expanded only here, at the point coverage is decided: coveredIDs and
	// allowListedIDs above are still exactly what the catalog and the
	// allow-list declared, which is what the two cross-checks needed to see.
	coveredIDs = expandAliases(coveredIDs, aliases)
	allowListedIDs = expandAliases(allowListedIDs, aliases)

	return &auditResult{
		CE:             buildEditionReport("Community Edition (CE)", ce, coveredIDs, allowListedIDs),
		EE:             buildEditionReport("Business Edition (EE)", ee, coveredIDs, allowListedIDs),
		AllowListCount: len(allowList),
		Aliases:        aliases,
	}, nil
}

// buildEditionReport classifies every operation in ops as covered,
// allow-listed or uncovered. An operation with a matching action is always
// counted as covered even if it also happens to carry an allow-list entry:
// coveredIDs is checked first, so a now-unnecessary allow-list entry does not
// hide a real action from the covered count.
func buildEditionReport(name string, doc specDocument, coveredIDs, allowListedIDs map[string]bool) editionReport {
	r := editionReport{Name: name, Total: len(doc.Operations), Unnamed: doc.Unnamed}
	for id, op := range doc.Operations {
		switch {
		case coveredIDs[id]:
			r.Covered++
		case allowListedIDs[id]:
			r.AllowListed = append(r.AllowListed, op)
		default:
			r.Uncovered = append(r.Uncovered, op)
		}
	}
	sortOperations(r.AllowListed)
	sortOperations(r.Uncovered)
	return r
}

func sortOperations(ops []specOperation) {
	sort.Slice(ops, func(i, j int) bool { return ops[i].OperationID < ops[j].OperationID })
}

// operationLine renders one report line for op, in both the allow-listed and
// uncovered sections. "Deprecated" is documented as a legitimate reason to
// allow-list an operation (see specOperation's own doc comment), but until
// this it was never actually printed: a reviewer scanning "operations with no
// catalog action" had to open the vendored spec directly to tell a deprecated
// route from a live gap.
func operationLine(op specOperation) string {
	suffix := ""
	if op.Deprecated {
		suffix = " (deprecated)"
	}
	return fmt.Sprintf("    - %s (%s %s) [%s]%s\n", op.OperationID, op.Method, op.Path, op.Domain, suffix)
}

// unnamedLine renders one report line for a route no document and no table
// names. It has no operationId to lead with — that is the whole point of the
// line — so it leads with the method and path instead.
func unnamedLine(op unnamedOperation) string {
	return fmt.Sprintf("    - %s %s [%s]\n", op.Method, op.Path, op.Domain)
}

// buildReport renders result as a human-readable summary.
//
// The allow-list count is printed unconditionally, alongside the coverage
// numbers, on every run: an allow-list nobody's report shows is exactly how
// it turns from an honesty mechanism into a hiding place. Allow-listed
// operations are named, not just counted, for the same reason exclusion from
// the failure must never mean exclusion from the report.
//
// The unnamed operations are printed for a third variant of that same
// reason, and it is the sharpest of the three. An allow-listed operation is
// excluded from the failure but stays in the report and in the denominator;
// an aliased one is counted as covered through another name. An operation
// with no operationId at all was in neither — not the numerator, not the
// denominator, not any list — so the report could truthfully say "every
// operation is accounted for" while a route Portainer serves was accounted
// for nowhere. Naming them here is what makes the totals below readable as
// what they are: coverage of the operations this audit has a key for, with
// the keyless ones listed separately rather than dropped.
//
// The aliases are printed by name for that same reason, and it is arguably
// more important for them: an allow-list entry at least leaves its operation
// visible in the allow-listed section, while an alias makes one operationId
// count as covered because another is, and nothing in the coverage numbers
// alone would ever say so.
func buildReport(result *auditResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP 1:1 API coverage audit")
	fmt.Fprintln(&b, "=====================================")
	fmt.Fprintf(&b, "Allow-list entries: %d\n", result.AllowListCount)
	fmt.Fprintf(&b, "Operation aliases:  %d\n", len(result.Aliases))
	for _, a := range result.Aliases {
		fmt.Fprintf(&b, "    - %s (EE) = %s (CE)\n", a.Business, a.Community)
	}
	fmt.Fprintln(&b)

	for _, r := range []editionReport{result.EE, result.CE} {
		fmt.Fprintf(&b, "%s\n", r.Name)
		fmt.Fprintf(&b, "  covered:      %d of %d\n", r.Covered, r.Total)
		fmt.Fprintf(&b, "  allow-listed: %d of %d\n", len(r.AllowListed), r.Total)
		fmt.Fprintf(&b, "  uncovered:    %d of %d\n", len(r.Uncovered), r.Total)

		if len(r.AllowListed) > 0 {
			fmt.Fprintln(&b, "  allow-listed operations:")
			for _, op := range r.AllowListed {
				fmt.Fprint(&b, operationLine(op))
			}
		}
		if len(r.Uncovered) > 0 {
			fmt.Fprintln(&b, "  operations with no catalog action:")
			for _, op := range r.Uncovered {
				fmt.Fprint(&b, operationLine(op))
			}
		}
		if len(r.Unnamed) > 0 {
			fmt.Fprintf(&b, "  routes with no operationId, not counted above (%d):\n", len(r.Unnamed))
			fmt.Fprintln(&b, "    this document names none of these, and internal/specnaming's table does not either,")
			fmt.Fprintln(&b, "    so there is no key to audit coverage against. Add a table entry to make one countable.")
			for _, op := range r.Unnamed {
				fmt.Fprint(&b, unnamedLine(op))
			}
		}
		fmt.Fprintln(&b)
	}

	// Stated on both paths, and before the verdict on the success one: the
	// bottom line of this report is what a reader takes away, and "every
	// operation is accounted for" must not be able to mean "every operation
	// this audit can see".
	if unnamed := len(result.CE.Unnamed) + len(result.EE.Unnamed); unnamed > 0 {
		fmt.Fprintf(&b, "%d route(s) across both editions carry no operationId and are outside the totals above; they are listed by edition.\n", unnamed)
	}

	if result.HasGap() {
		fmt.Fprintf(&b, "%d operation(s) across both editions have no catalog action.\n",
			len(result.CE.Uncovered)+len(result.EE.Uncovered))
		return b.String()
	}
	fmt.Fprintln(&b, "Every operation in both vendored specs has a catalog action or an allow-list entry.")
	return b.String()
}
