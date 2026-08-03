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
}

// auditResult is the outcome of comparing the declared catalog against both
// vendored specs.
type auditResult struct {
	CE, EE         editionReport
	AllowListCount int
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
func auditCoverage(ce, ee map[string]specOperation, actions []toolutil.ActionSpec, allowList []allowListEntry) (*auditResult, error) {
	inEither := func(id string) bool {
		_, inCE := ce[id]
		_, inEE := ee[id]
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

	return &auditResult{
		CE:             buildEditionReport("Community Edition (CE)", ce, coveredIDs, allowListedIDs),
		EE:             buildEditionReport("Business Edition (EE)", ee, coveredIDs, allowListedIDs),
		AllowListCount: len(allowList),
	}, nil
}

// buildEditionReport classifies every operation in ops as covered,
// allow-listed or uncovered. An operation with a matching action is always
// counted as covered even if it also happens to carry an allow-list entry:
// coveredIDs is checked first, so a now-unnecessary allow-list entry does not
// hide a real action from the covered count.
func buildEditionReport(name string, ops map[string]specOperation, coveredIDs, allowListedIDs map[string]bool) editionReport {
	r := editionReport{Name: name, Total: len(ops)}
	for id, op := range ops {
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

// buildReport renders result as a human-readable summary.
//
// The allow-list count is printed unconditionally, alongside the coverage
// numbers, on every run: an allow-list nobody's report shows is exactly how
// it turns from an honesty mechanism into a hiding place. Allow-listed
// operations are named, not just counted, for the same reason exclusion from
// the failure must never mean exclusion from the report.
func buildReport(result *auditResult) string {
	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP 1:1 API coverage audit")
	fmt.Fprintln(&b, "=====================================")
	fmt.Fprintf(&b, "Allow-list entries: %d\n\n", result.AllowListCount)

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
		fmt.Fprintln(&b)
	}

	if result.HasGap() {
		fmt.Fprintf(&b, "%d operation(s) across both editions have no catalog action.\n",
			len(result.CE.Uncovered)+len(result.EE.Uncovered))
		return b.String()
	}
	fmt.Fprintln(&b, "Every operation in both vendored specs has a catalog action or an allow-list entry.")
	return b.String()
}
