package specnaming

import (
	"sort"
	"strings"
)

// The second rule this package holds, and the second one two independent
// derivations must agree on exactly.
//
// Portainer's published documents leave some path-item operations with no
// `operationId` at all. Everything downstream of the specification is keyed
// by that name: oapi-codegen derives a generated method from it,
// internal/apiversion's operationIDs index is keyed by it, and
// internal/tools/actioncatalog resolves an action's edition through that
// index — so an operation nothing names cannot be declared as an action at
// all, however the domain package is written. cmd/gen_applicability's
// borrowIDsAcrossEditions already repairs the common case, where one edition
// names a route the other leaves nameless. This table is for what is left
// over: a route *neither* document names, for which there is nothing to
// borrow.
//
// Why the rule lives here rather than in either caller. Two commands must
// agree on the name, exactly: cmd/gen_applicability, which writes it into
// the operationIDs index an action's edition is resolved through, and
// cmd/audit_1to1, which counts the operation against that same name when
// deciding coverage. If they disagreed, the audit would report a gap on an
// operation the catalog does cover, or — far worse, and the failure that
// produced this table — count nothing at all while the catalog carries a
// working action under a different name. Both are `package main` and cannot
// import each other; both already import this package's sibling rule, so
// both can import this one, and a rule that lives here has no mirror to keep
// in step.
//
// Why the table is explicit rather than derived from method and path. A
// mechanical rule ("GET on a path ending in /{id} is <Resource>Inspect")
// would mint a name for every unnamed route in every future specification,
// including ones whose correct name is a judgement call, and it would do so
// silently — which is the same class of invisibility this table exists to
// end. Naming is a judgement, and this project already keeps
// cmd/gen_action_inputs's actionNameOverrides for exactly that reason, with
// a stated Reason on every entry. This table follows that shape: an entry is
// a decision somebody made and wrote down, and a route nobody has decided on
// stays unnamed and is reported as such by cmd/audit_1to1.

// operationKey identifies one route: an upper-case HTTP method and the path
// template exactly as the vendored document declares it, "{id}" placeholders
// included.
type operationKey struct {
	Method string
	Path   string
}

// syntheticName is one entry in the table below: the name this project gives
// a route no vendored document names, and why that name.
type syntheticName struct {
	// OperationID is in the exported-Go-identifier form oapi-codegen would
	// have derived from a real operationId, because that is the form every
	// index keyed by an operationId in this project uses.
	OperationID string
	// Reason states the judgement behind the name, so a future reader can
	// disagree with it deliberately rather than by accident — the same
	// obligation cmd/gen_action_inputs's actionNameOverride.Reason carries.
	Reason string
}

// syntheticOperationIDs names every route this project has decided on that
// neither vendored document names.
//
// One entry today. Measured over api/specs/ce-2.44.0.json and
// api/specs/ee-2.44.0.json: Community leaves 14 of its 265 operations
// nameless and Business 1 of its 442, and the intersection of those two sets
// is exactly this route. The other 13 are all named by the Business
// document, so borrowIDsAcrossEditions already resolves them and none of
// them belongs here.
var syntheticOperationIDs = map[operationKey]syntheticName{
	{Method: "GET", Path: "/endpoint_groups/{id}"}: {
		OperationID: "EndpointGroupInspect",
		Reason: "neither vendored document names this route, so there is no operationId to borrow across editions, " +
			"yet the route is served by both: it answers 200 on Community and on Business, returning the group " +
			"(Business adds a Policies field). " +
			"The name follows its five siblings in the same document — EndpointGroupList, EndpointGroupCreate, " +
			"EndpointGroupUpdate, EndpointGroupDelete and the two endpoint-membership operations — and the " +
			"catalog-wide *Inspect convention for a single-resource GET (EndpointInspect, TeamInspect, " +
			"RegistryInspect, StackInspect). " +
			"Verified collision-free when written: EndpointGroupInspect is published by neither vendored " +
			"specification, so it displaces no real operationId. The two occurrences in " +
			"internal/apiversion/applicability_gen.go are this entry's own effect, not a collision with it — " +
			"cmd/gen_applicability writes the name into operationIDs for both editions precisely because this " +
			"table supplies it, after borrowIDsAcrossEditions has already failed to find one to borrow. Every " +
			"caller refuses a synthetic name that collides with a published one rather than overwriting it: " +
			"cmd/gen_applicability's applySyntheticIDs, cmd/audit_1to1's collisionError and " +
			"cmd/audit_spec_drift's parseSpecOperations",
	},
}

// SyntheticOperationID returns the name this project gives the route
// (method, path) when no vendored document names it, and whether the table
// names it at all.
//
// It is deliberately ignorant of whether some document *does* name the
// route: it answers for a (method, path) pair alone, and every caller
// consults it only after finding no operationId of its own — cmd/audit_1to1
// per document, cmd/gen_applicability after borrowIDsAcrossEditions has
// already tried every edition's name. A route this table does not mention
// gets ("", false), which is the answer for the overwhelming majority of
// routes, including all 13 that borrowing already resolves.
//
// The method is matched case-insensitively, because the two callers reach it
// from different places: cmd/gen_applicability upper-cases the path-item key
// before building its operation, while cmd/audit_1to1 still holds the raw
// lower-case JSON key at the point it needs an answer. Requiring one of them
// to normalise first is exactly the kind of unstated precondition a mirrored
// rule drifts on.
func SyntheticOperationID(method, path string) (string, bool) {
	entry, ok := syntheticOperationIDs[operationKey{Method: strings.ToUpper(method), Path: path}]
	if !ok {
		return "", false
	}
	return entry.OperationID, true
}

// SyntheticEntry is one row of the table as SyntheticOperationIDs reports
// it: the route, the name this project gives it, and the judgement behind
// that name.
//
// Reason is carried out of the package rather than kept private because a
// promise nothing can read is a promise nothing can check. This table's own
// doc comment says an entry is "a decision somebody made and wrote down";
// cmd/audit_1to1 enforces exactly that for its allow-list, refusing an entry
// with an empty reason at parse time (see allowlist.go), and an entry here
// is the same kind of standing exception to the same kind of rule.
type SyntheticEntry struct {
	Method      string
	Path        string
	OperationID string
	Reason      string
}

// SyntheticOperationIDs returns every row of the table, sorted by path then
// method so a caller reporting on it is stable across runs.
//
// It exists so the table can be checked against the vendored documents
// without this package having to read them itself: an entry naming a route
// some document *does* name, or a route no document declares at all, is a
// judgement that has quietly gone stale, and the same obligation
// cmd/gen_action_inputs's TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation
// discharges for actionNameOverrides applies here.
func SyntheticOperationIDs() []SyntheticEntry {
	out := make([]SyntheticEntry, 0, len(syntheticOperationIDs))
	for key, entry := range syntheticOperationIDs {
		out = append(out, SyntheticEntry{
			Method:      key.Method,
			Path:        key.Path,
			OperationID: entry.OperationID,
			Reason:      entry.Reason,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path != out[j].Path {
			return out[i].Path < out[j].Path
		}
		return out[i].Method < out[j].Method
	})
	return out
}
