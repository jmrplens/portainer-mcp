package main

import (
	"errors"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// opRef names one operation added or removed between before and after: just
// enough to place it on the work list (which domain, which route) without
// pretending to know anything about its parameter shape, which an added or
// removed operation has only on one side of the comparison anyway.
type opRef struct {
	OperationID string
	Method      string
	Path        string
}

// changedOp is one operation present on both sides whose parameter shape —
// path parameters, query parameters, and top-level request-body fields, the
// same flattened scope internal/specdiff's own doc comment states for
// OperationShape — differs. TouchesStruct is precomputed once here, not
// re-derived by every report writer, so buildReport's structural/cosmetic
// split and the JSON encoder's own "touches_struct" field can never disagree
// about the same operation.
type changedOp struct {
	OperationID   string
	Method        string
	Path          string
	Changes       []specdiff.FieldChange
	TouchesStruct bool
}

// unresolvedOp is one operation present on both sides that specdiff.ShapeFromSpec
// refused to flatten on at least one side — not a change this tool failed to
// compute, but an operation neither side of the comparison can even describe
// as a flat parameter shape. See computeDelta's doc comment for why this
// happens on 9 of 441 real Business Edition operations today (a header
// parameter, or a body field colliding with a path/query parameter of the
// same wire name) and why that must not abort the whole run.
type unresolvedOp struct {
	OperationID string
	Method      string
	Path        string
	// Reason is the error ShapeFromSpec returned, verbatim, so a person
	// reviewing this operation by hand knows exactly what to look for
	// without re-running the tool with more logging.
	Reason string
}

// domainGroup is one domain's slice of the work list, in the five buckets a
// reader is meant to work through in order: operations added (judgement —
// a new operation needs a name, a narrative, possibly a redaction wrapper),
// operations removed (mechanical — delete the action), operations this
// tool could not flatten on either side and so cannot classify at all
// (judgement — review by hand against both spec versions), operations whose
// change touches the generated input struct (judgement — a field's name,
// type or requiredness moved, which changes generated Go code), and
// operations whose change is cosmetic only (mechanical — a description or
// enum edit is a copy-paste). See buildReport for why this exact order, not
// alphabetical or by operationId, is what makes the list usable at a hundred
// entries.
type domainGroup struct {
	Domain          string
	Added           []opRef
	Removed         []opRef
	Unresolvable    []unresolvedOp
	ChangedStruct   []changedOp
	ChangedCosmetic []changedOp
}

// deltaResult is the outcome of comparing every operation declared by before
// against every operation declared by after, grouped by domain.
//
// An operation present on both sides whose specdiff.Compare returns no
// changes at all is not recorded anywhere in Domains — see computeDelta's
// doc comment for why that omission, not merely "reported as unchanged", is
// the point: at a hundred-plus changed operations per minor release, a work
// list that also lists everything that did NOT change would bury the part
// that does.
type deltaResult struct {
	Domains []domainGroup
	// BeforeCount and AfterCount are the total operation count each document
	// declares (with an operationId — see parseSpecOperations), so a reader
	// can see the whole shape of the comparison, not only what moved.
	BeforeCount, AfterCount int
	AddedCount              int
	RemovedCount            int
	// ChangedCount is every operation present on both sides whose parameter
	// shape differs in at least one FieldChange — see isStructKind's doc
	// comment for why this is not the same number as a full-document diff
	// would report, and why that difference is by design, not a bug to
	// reconcile away.
	ChangedCount int
	// ChangedStructCount is the subset of ChangedCount where at least one of
	// those FieldChanges alters the generated input struct (a field's name,
	// type, requiredness, or which of path/query/body it comes from) rather
	// than only its published description or enum text.
	ChangedStructCount int
	// UnresolvableCount is the number of operations present on both sides
	// that specdiff.ShapeFromSpec could not flatten on at least one side —
	// see unresolvedOp's doc comment. These are not counted in ChangedCount:
	// this tool genuinely does not know whether they changed, only that it
	// cannot tell, which is a different fact a reader must not confuse with
	// "unchanged".
	UnresolvableCount int
}

// isStructKind reports whether kind can change the Go struct
// cmd/gen_action_inputs would generate for this operation — a field
// appearing, disappearing, changing type, moving between required and
// optional, or moving between path/query/body — as opposed to a kind that
// only changes what a model reads in the published JSON Schema (its prose or
// its enum constraint) while the struct itself stays the same shape.
//
// This is the same distinction plan/research/version-delta-analysis.md
// measures as "alters generated input struct" versus "alters input JSON
// schema only": ChangeEnum and ChangeDescription are schema-only there
// because neither one changes a Go field's name or type, only the
// constraint or prose a model is shown alongside it.
//
// ChangeTitle and ChangeOperationDescription are schema-only for the
// identical reason, one level up: OperationShape.Title/Description are
// toolutil.ActionSpec.Title/Description, model-facing text a domain author
// copy-pastes from the new specification's summary/description — never a
// Go struct field, so never structural. This is exactly the distinction
// that lets the seven operations named in this command's package doc (only
// an operation-level summary/description changed, no parameter moved at
// all) land in ChangedCosmetic rather than being invisible, the way they
// were before OperationShape carried these two fields at all.
func isStructKind(kind specdiff.ChangeKind) bool {
	switch kind {
	case specdiff.ChangeAdded, specdiff.ChangeRemoved, specdiff.ChangeType,
		specdiff.ChangeRequiredness, specdiff.ChangeOrigin:
		return true
	case specdiff.ChangeEnum, specdiff.ChangeDescription, specdiff.ChangeTitle, specdiff.ChangeOperationDescription:
		return false
	default:
		return false
	}
}

// computeDelta compares every operation before declares against every
// operation after declares, keyed by operationId (see parseSpecOperations
// for why: an operation with no operationId cannot be tracked across a
// version boundary by identity, so it never enters either map).
//
// An operation present on only one side is Added or Removed. An operation
// present on both sides is compared with specdiff.ShapeFromSpec and
// specdiff.Compare — the identical engine cmd/audit_spec_drift uses for the
// catalog-vs-spec comparison, per internal/specdiff's own doc comment: one
// engine, two consumers, never a second derivation of "did this operation's
// shape change".
//
// Deliberately not counted here: response-shape changes (OperationShape
// never reads an operation's "responses" at all — see ShapeFromSpec) and
// changes nested inside a body property's own nested schema (OperationShape
// is flat by design — see its doc comment). Both are real changes a full
// operation-node diff would find, and plan/research/version-delta-analysis.md
// measures them separately as "response-only" and "nested-body-only". This
// tool's ChangedCount is deliberately narrower: what a model's published
// input schema and a generated Go struct can see, because that is the part
// that actually needs a person to edit code — see this command's package doc
// for how that narrower scope was verified against the same measurement's
// own "alters input JSON schema" column rather than assumed.
//
// A ShapeFromSpec failure on either side does not abort the run: measured
// against the real 2.44.0 Business Edition document, 9 of 441 operations
// refuse to flatten at all (a header parameter — ShapeFromSpec only
// flattens path and query — or a body field whose wire name collides with a
// path/query parameter of the same name, e.g. CreateKubernetesIngress's body
// "Namespace" against its own path parameter "namespace"). Every one of
// those 9 predates this tool: cmd/audit_spec_drift never exercised them
// because it only calls ShapeFromSpec for the 19 operations the catalog
// declares today, none of which are among the 9. This tool is the first
// thing in this repository to call ShapeFromSpec across literally every
// operation a real specification declares, and aborting the entire report
// over 9 pre-existing, already-documented refusals (see specdiff.ShapeFromSpec's
// own doc comment for why it refuses rather than guesses) would make this
// tool unusable against the real API it exists to serve. Such an operation
// is recorded as Unresolvable instead — visibly, with the refusal's reason —
// never silently dropped and never allowed to abort the other 400-plus
// operations this tool can classify.
func computeDelta(before, after map[string]specOperation, domainTags map[string][]string) (*deltaResult, error) {
	reverse, err := reverseDomainTags(domainTags)
	if err != nil {
		return nil, err
	}

	ids := make(map[string]bool, len(before)+len(after))
	for id := range before {
		ids[id] = true
	}
	for id := range after {
		ids[id] = true
	}
	sortedIDs := make([]string, 0, len(ids))
	for id := range ids {
		sortedIDs = append(sortedIDs, id)
	}
	sort.Strings(sortedIDs)

	groups := make(map[string]*domainGroup)
	group := func(domain string) *domainGroup {
		g, ok := groups[domain]
		if !ok {
			g = &domainGroup{Domain: domain}
			groups[domain] = g
		}
		return g
	}

	result := &deltaResult{BeforeCount: len(before), AfterCount: len(after)}

	for _, id := range sortedIDs {
		b, inBefore := before[id]
		a, inAfter := after[id]

		switch {
		case inAfter && !inBefore:
			g := group(domainForTag(reverse, a.Tag))
			g.Added = append(g.Added, opRef{OperationID: id, Method: a.Op.Method, Path: a.Op.Path})
			result.AddedCount++

		case inBefore && !inAfter:
			g := group(domainForTag(reverse, b.Tag))
			g.Removed = append(g.Removed, opRef{OperationID: id, Method: b.Op.Method, Path: b.Op.Path})
			result.RemovedCount++

		default:
			beforeShape, beforeErr := specdiff.ShapeFromSpec(b.Op)
			afterShape, afterErr := specdiff.ShapeFromSpec(a.Op)
			if beforeErr != nil || afterErr != nil {
				reason := errors.Join(beforeErr, afterErr).Error()
				tag := a.Tag
				if tag == "" {
					tag = b.Tag
				}
				g := group(domainForTag(reverse, tag))
				g.Unresolvable = append(g.Unresolvable, unresolvedOp{
					OperationID: id, Method: a.Op.Method, Path: a.Op.Path, Reason: reason,
				})
				result.UnresolvableCount++
				continue
			}
			changes := specdiff.Compare(beforeShape, afterShape)
			if len(changes) == 0 {
				continue // unchanged: not recorded anywhere, on purpose (see deltaResult's doc comment)
			}

			touchesStruct := false
			for _, c := range changes {
				if isStructKind(c.Kind) {
					touchesStruct = true
					break
				}
			}

			tag := a.Tag
			if tag == "" {
				tag = b.Tag // a tag disappearing between versions is rare; the before side is still a real domain to place this under
			}
			op := changedOp{OperationID: id, Method: a.Op.Method, Path: a.Op.Path, Changes: changes, TouchesStruct: touchesStruct}

			g := group(domainForTag(reverse, tag))
			if touchesStruct {
				g.ChangedStruct = append(g.ChangedStruct, op)
			} else {
				g.ChangedCosmetic = append(g.ChangedCosmetic, op)
			}
			result.ChangedCount++
			if touchesStruct {
				result.ChangedStructCount++
			}
		}
	}

	domains := make([]string, 0, len(groups))
	for domain := range groups {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	result.Domains = make([]domainGroup, 0, len(domains))
	for _, domain := range domains {
		g := groups[domain]
		sort.Slice(g.Added, func(i, j int) bool { return g.Added[i].OperationID < g.Added[j].OperationID })
		sort.Slice(g.Removed, func(i, j int) bool { return g.Removed[i].OperationID < g.Removed[j].OperationID })
		sort.Slice(g.Unresolvable, func(i, j int) bool { return g.Unresolvable[i].OperationID < g.Unresolvable[j].OperationID })
		sort.Slice(g.ChangedStruct, func(i, j int) bool { return g.ChangedStruct[i].OperationID < g.ChangedStruct[j].OperationID })
		sort.Slice(g.ChangedCosmetic, func(i, j int) bool { return g.ChangedCosmetic[i].OperationID < g.ChangedCosmetic[j].OperationID })
		result.Domains = append(result.Domains, *g)
	}

	return result, nil
}
