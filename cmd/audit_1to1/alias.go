package main

import (
	"fmt"
	"time"
)

// operationAlias records two operationIds that name one and the same route,
// one in each vendored Portainer document.
//
// # The blindness this fixes
//
// This audit keys coverage by operationId alone, which is right for 691 of
// the 692 operations the two documents declare between them and wrong for
// one. POST /stacks/webhooks/{webhookID} is a single route both editions
// serve, and the two documents name it differently: "StacksWebhookInvoke" in
// Business Edition, "WebhookInvoke" in Community Edition. The catalog is
// generated from the Business Edition document, so the action that exists
// carries the Business Edition spelling — and the Community Edition spelling
// would be reported uncovered forever, on a route the server exposes, while
// the Business Edition numbers showed nothing at all. An alias makes covering
// either name cover both.
//
// # Why an alias, and not either of the two obvious alternatives
//
// A second hand-declared ActionSpec for the Community Edition operationId is
// ruled out by actioncatalog.Build, which refuses two specs sharing a Name
// *before* it filters by edition, deliberately: a colliding pair is a defect
// in the declared specs themselves, independent of which server they are
// served against. The second spec would therefore need a distinct Name, and a
// Community Edition user would call stacks.webhook_invoke_ce for the route a
// Business Edition user calls stacks.webhook_invoke — a fact about our two
// vendored documents leaking into the name a model reads.
//
// A coverage-allow-list entry is ruled out by that file's own contract: its
// entries are for operations "that will never be exposed as an MCP action",
// and this one is exposed, under the other operationId. The entry would be a
// false statement, and this audit's allow-list is only worth anything while
// every entry in it is true.
//
// # Why this table is Go and not a third YAML file beside the other two
//
// api/coverage-allowlist.yaml and api/spec-drift-allowlist.yaml are data
// files because of what their entries are: promises no program can check.
// "This operation will never be exposed as an MCP action" is a human
// judgement, so each entry carries a reason and a date and is re-read by a
// human deciding whether it still holds. Being a separate, reviewable file is
// what makes that re-reading possible.
//
// An alias is not that kind of statement. It asserts a fact about the two
// committed documents — these two operationIds are one route — and
// checkAliases below verifies every part of it on every run, against the same
// documents the audit already loads. It excuses nothing and forgives nothing:
// it cannot hide an uncovered operation, because an entry is only accepted
// when each id resolves in exactly one document and both name the same method
// and path. A reason and a date are still carried, but as documentation of
// why the entry exists rather than as the review mechanism the allow-lists
// need theirs to be.
//
// The closer precedent in this repository is therefore not the allow-lists
// but cmd/gen_action_inputs's own operationId-keyed correction tables —
// actionNameOverrides and pathParamMinimumExceptions — which state facts
// about the vendored documents, are checked against them by tests, and live
// in Go beside the code that consults them. This table is edited by whoever
// re-vendors a specification, in the same commit and the same review as the
// checks that keep it honest; a YAML file would separate the two for a table
// that has exactly one entry in 692 operations and grows only when Portainer
// renames an operationId between editions.
type operationAlias struct {
	// Business is the operationId the Business Edition document declares for
	// the route, and Community the one the Community Edition document
	// declares. Each must resolve in its own document and in neither the
	// other — see checkAliases.
	Business  string
	Community string
	// Reason states why the two documents disagree, for a reader deciding
	// whether the entry is still the right response.
	Reason string
	// Added dates the entry, in the ISO 8601 calendar-date form
	// api/coverage-allowlist.yaml's own entries use.
	Added string
}

// operationAliases is every route the two vendored documents name differently.
//
// One entry, and it is not a sample of a larger population: a set difference
// over both documents (2026-08-18, against api/specs/ce-2.44.0.json and
// api/specs/ee-2.44.0.json) finds exactly one route declared by both with
// different operationIds. Two further Community Edition operationIds —
// GetKubernetesConfig and systemUpgrade — have no Business Edition
// counterpart at all, which is a genuine edition asymmetry rather than a
// rename and belongs in docs/api-divergences.md §5, not here.
var operationAliases = []operationAlias{
	{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason: "POST /stacks/webhooks/{webhookID} is one route both editions serve, renamed between the two documents. " +
			"The catalog is generated from the Business Edition document, so the action stacks.webhook_invoke carries the Business Edition spelling; " +
			"without this alias the Community Edition spelling reads as an uncovered operation on a route that is in fact exposed, and does so permanently.",
		Added: "2026-08-18",
	},
}

// aliasDateFormat is the ISO 8601 calendar-date layout every entry's Added
// field must use. The same layout allowListDateFormat requires, deliberately:
// two date conventions in one command's inputs is one more thing to get
// wrong.
const aliasDateFormat = allowListDateFormat

// checkAliases verifies every alias against the two vendored documents and
// returns an error naming the first entry that no longer holds.
//
// It is fatal rather than a report line for the same reason a stale allow-list
// entry is: an entry nobody re-checks is an entry that keeps applying after
// the fact it asserted stopped being true. The two failure modes this catches
// are exactly the two an alias can decay into:
//
//   - An operationId that no longer exists where the entry says it does. A
//     rename upstream, or a re-vendoring, and the alias silently applies to
//     nothing — or worse, a later, unrelated operation reuses the name and
//     inherits the aliasing.
//   - A pair that is no longer the same route. If the two ids drift onto
//     different methods or paths, marking one covered because the other is
//     hides a real, uncovered operation, which is precisely what this audit
//     exists to make impossible.
//
// The requirement that each id resolve in its own document and in neither the
// other is what makes the whole mechanism safe to have at all: an alias can
// only ever join two names that no single document uses together, so it can
// never mark a second, genuinely distinct operation covered.
func checkAliases(aliases []operationAlias, ce, ee map[string]specOperation) error {
	seen := make(map[string]int, 2*len(aliases))
	for i, a := range aliases {
		if a.Business == "" || a.Community == "" {
			return fmt.Errorf("alias entry %d: both business and community operation ids are required", i)
		}
		if a.Business == a.Community {
			return fmt.Errorf("alias entry %d: %q is aliased to itself", i, a.Business)
		}
		if a.Reason == "" {
			return fmt.Errorf("alias entry %q/%q: reason is required", a.Business, a.Community)
		}
		if a.Added == "" {
			return fmt.Errorf("alias entry %q/%q: added date is required", a.Business, a.Community)
		}
		if _, err := time.Parse(aliasDateFormat, a.Added); err != nil {
			return fmt.Errorf("alias entry %q/%q: added %q is not an ISO date (YYYY-MM-DD): %w",
				a.Business, a.Community, a.Added, err)
		}
		for _, id := range []string{a.Business, a.Community} {
			if prior, dup := seen[id]; dup {
				return fmt.Errorf("alias entry %d: %q is already aliased by entry %d; an operationId may name at most one route",
					i, id, prior)
			}
			seen[id] = i
		}

		business, inEE := ee[a.Business]
		if !inEE {
			return fmt.Errorf("alias entry %q/%q: %q is not declared in the Business Edition document; the alias is stale",
				a.Business, a.Community, a.Business)
		}
		if _, alsoInCE := ce[a.Business]; alsoInCE {
			return fmt.Errorf("alias entry %q/%q: %q is declared in the Community Edition document too, so the two ids are not one route under two names",
				a.Business, a.Community, a.Business)
		}
		community, inCE := ce[a.Community]
		if !inCE {
			return fmt.Errorf("alias entry %q/%q: %q is not declared in the Community Edition document; the alias is stale",
				a.Business, a.Community, a.Community)
		}
		if _, alsoInEE := ee[a.Community]; alsoInEE {
			return fmt.Errorf("alias entry %q/%q: %q is declared in the Business Edition document too, so the two ids are not one route under two names",
				a.Business, a.Community, a.Community)
		}
		if business.Method != community.Method || business.Path != community.Path {
			return fmt.Errorf("alias entry %q/%q: %s is %s %s and %s is %s %s; the two are no longer the same route",
				a.Business, a.Community,
				a.Business, business.Method, business.Path,
				a.Community, community.Method, community.Path)
		}
	}
	return nil
}

// expandAliases returns ids with, for every alias one of whose names it
// already contains, the other name added.
//
// Applied to the covered set — covering a route under either of its names
// covers it under both — and to the allow-listed set, for the same reason:
// an allow-list entry excuses a route, and a route does not stop being
// excused because the other document spells it differently.
//
// It returns a new map rather than mutating ids: the sets it is applied to
// are built from the catalog and the allow-list, and a caller comparing
// against either afterwards must still see what was actually declared.
func expandAliases(ids map[string]bool, aliases []operationAlias) map[string]bool {
	expanded := make(map[string]bool, len(ids)+len(aliases))
	for id := range ids {
		expanded[id] = true
	}
	for _, a := range aliases {
		if ids[a.Business] || ids[a.Community] {
			expanded[a.Business] = true
			expanded[a.Community] = true
		}
	}
	return expanded
}
