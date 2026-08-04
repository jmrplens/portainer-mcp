package main

import (
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// verifyCanary proves specdiff.Compare can still tell two different
// operation shapes apart, before this command trusts anything else it is
// about to report.
//
// This mirrors cmd/audit_spec_reality's selftestPath precedent exactly, for
// the identical reason stated there: this repository has already shipped
// seventeen checks that could not discriminate, every one because its
// assertion matched something the fixture itself supplied. A comparison
// engine that silently stopped comparing — a refactor that made Compare
// always return nil, say — would make every real run of this audit print
// "no drift" for a reason indistinguishable from "nothing was actually
// compared". The canary below is not read from any vendored spec or catalog
// action; it is a shape this function invents and then deliberately breaks,
// so it can fail on its own, independent of whatever this run's real inputs
// happen to contain.
//
// It checks both directions on purpose. The perturbed pair must produce
// exactly one ChangeType finding and nothing else (a detector that flags
// every field as changed would still "detect" the perturbation, and that
// failure mode is exactly as unable to discriminate as one that finds
// nothing); the pair compared against itself must produce no findings at
// all (a detector that always reports drift would trivially pass the first
// check while making every clean run look broken). Either result failing
// means specdiff.Compare or isGating cannot be trusted this run, and run
// refuses to report anything about the real catalog rather than risk a
// false "no drift".
//
// compare is specdiff.Compare in every real run (see run, which calls
// verifyCanary(specdiff.Compare)); it is a parameter, not a direct call,
// solely so this function's own tests can hand it a deliberately broken
// stand-in (one that always returns nil, or one that flags everything) and
// prove verifyCanary actually refuses in that case — asserting that the
// canary mechanism itself discriminates, not merely that today's real
// specdiff.Compare happens to pass it.
func verifyCanary(compare func(before, after specdiff.OperationShape) []specdiff.FieldChange) error {
	before := specdiff.OperationShape{OperationID: "CanarySelfTest", Fields: []specdiff.FieldShape{
		{JSONName: "id", Type: "integer", Required: true, Origin: "path", Description: "canary field, deliberately perturbed below"},
	}}
	perturbed := specdiff.OperationShape{OperationID: "CanarySelfTest", Fields: []specdiff.FieldShape{
		{JSONName: "id", Type: "string", Required: true, Origin: "path", Description: "canary field, deliberately perturbed below"},
	}}

	changes := compare(before, perturbed)
	if len(changes) != 1 || changes[0].Kind != specdiff.ChangeType {
		return fmt.Errorf(
			"canary self-test failed: comparing a deliberately perturbed shape (field %q: integer -> string) against its original produced %v, want exactly one ChangeType finding; the comparison engine cannot be trusted this run",
			"id", changes)
	}
	if !isGating(changes[0]) {
		return fmt.Errorf("canary self-test failed: a ChangeType finding must gate the build, but isGating(%+v) reported false", changes[0])
	}

	if identical := compare(before, before); len(identical) != 0 {
		return fmt.Errorf(
			"canary self-test failed: comparing the unperturbed shape against itself produced %v, want no findings; a detector that always reports drift is as untrustworthy as one that never does",
			identical)
	}
	return nil
}

// isGating reports whether c must fail the build on its own, unless an
// allow-list entry excuses it.
//
// Every structural kind gates unconditionally: ChangeAdded, ChangeRemoved,
// ChangeType, ChangeRequiredness and ChangeEnum can each make a caller send
// the wrong thing, an incomplete call, or a value the real operation
// refuses, and ChangeOrigin would too if ShapeFromCatalog ever started
// stating an origin (it does not today — see FieldShape.Origin's doc
// comment — so this kind is inert in practice, not exempted in principle).
//
// ChangeDescription is the one kind specdiff.ChangeKind's own doc comment
// already calls out as unable to make a caller send the wrong thing, and
// this audit's decision is to gate it only conditionally, not exempt it
// outright: c.Before is always the vendored specification's own text (see
// auditDrift, which always calls specdiff.Compare(specShape, catalogShape)),
// so c.Before == "" means the specification never described this field at
// all. Measured directly (see this command's package doc): 341/341 path
// parameters and 237/237 query parameters carry a description in the
// vendored Business Edition specification, but only 485 of 771 body
// properties do — roughly 286 body fields the specification itself leaves
// silent. A field the specification never described cannot have drifted
// away from a description that never existed; gating on that case would
// fail the build for every one of those 286 fields the moment a hand-written
// action adds a helpful description the spec never had, which is an
// improvement, not a defect. A field the specification DID describe, whose
// catalog description no longer matches, is different: cmd/gen_action_inputs
// republishes the specification's text verbatim (see its fieldTag, and the
// exact-match assertion in internal/tools/tags/tags_test.go's
// TestUnit_TagDeleteInputSchema_PublishesTheSpecDescription), so a mismatch
// there means the catalog fell out of sync with prose the specification
// still states — worth surfacing as a real, if cosmetic, finding rather than
// silently dropped.
//
// ChangeTitle and ChangeOperationDescription apply the identical
// non-empty-spec-text rule as ChangeDescription, for the same reason and one
// more: c.Before is the vendored specification's own cleaned Title/
// Description (specdiff.ShapeFromSpec always calls
// specdiff.CleanTitleAndDescription — see that function's doc comment), and
// in every real operation across both vendored specifications that text is
// non-empty (cmd/gen_action_inputs's own
// TestUnit_CleanTitleAndDescription_EveryRealOperationHasANonEmptySummary
// proves this for the 442-operation Business Edition document and its
// Community Edition counterpart), so this branch gates unconditionally in
// practice today. It is written as the same conditional as ChangeDescription
// rather than an unconditional "return true", because specdiff.ShapeFromSpec
// does not itself refuse an empty summary (see that function's own doc
// comment for why it is lenient where the generator is not) — a real
// distinction between the two, not merely a stylistic one, and treating
// both alike where they are alike is what this audit already committed to.
//
// A hand-authored Title or Description that deliberately improves on the
// specification's own wording — the tags and registries pilots' own
// narrative() hooks, and registries/system's four directly hand-written
// literals (EcrDeleteTags, RegistryConfigure, RepositoryTagsDelete,
// SystemUpgrade) — is real, permanent divergence, but not one this audit
// needs an allow-list entry to excuse: toolutil.ActionSpec.TitleOverridden /
// DescriptionOverridden (set by toolutil.WithNarrative — see its own doc
// comment) is exactly the plumbing an earlier version of this comment said
// did not exist, added once the cost of adding it was weighed against 35
// allow-list entries that would otherwise never shrink. c.AfterOverridden
// (specdiff.FieldChange's own doc comment) carries that fact through
// Compare, so a ChangeTitle/ChangeOperationDescription finding whose catalog
// side was deliberately overridden never gates here — while
// cmd/audit_spec_delta, which calls the identical Compare and reads the
// identical AfterOverridden, still surfaces the specification's own wording
// change at upgrade time regardless, exactly as before: Compare tags the
// finding, it never suppresses it.
func isGating(c specdiff.FieldChange) bool {
	switch c.Kind {
	case specdiff.ChangeAdded, specdiff.ChangeRemoved, specdiff.ChangeType,
		specdiff.ChangeRequiredness, specdiff.ChangeEnum, specdiff.ChangeOrigin:
		return true
	case specdiff.ChangeTitle, specdiff.ChangeOperationDescription:
		return c.Before != "" && !c.AfterOverridden
	case specdiff.ChangeDescription:
		return c.Before != ""
	default:
		return false
	}
}

// driftFinding is one FieldChange found for one catalog action, decorated
// with what this audit decided about it: whether it gates the build on its
// own, and whether an allow-list entry excuses it from doing so.
type driftFinding struct {
	OperationID string
	ActionName  string
	// SpecEdition names which vendored document grounded the comparison
	// ("EE" or "CE") — see resolveSpecOperation's doc comment for why that is
	// not an arbitrary pick.
	SpecEdition string
	Change      specdiff.FieldChange
	Gating      bool
	AllowListed bool
	// AllowListReason and AllowListAdded carry the excusing entry's own
	// fields when AllowListed is true, so buildReport can print why a gating
	// finding did not fail the build without re-indexing the allow-list
	// itself.
	AllowListReason string
	AllowListAdded  string
}

// auditResult is the outcome of comparing every declared catalog action's
// parameter shape against the vendored specification operation it was
// generated from.
type auditResult struct {
	// Findings lists every FieldChange found, for every audited action,
	// gating or not, allow-listed or not: exclusion from the failure must
	// never mean exclusion from the report, the identical principle
	// cmd/audit_1to1's own allow-list follows.
	Findings []driftFinding
	// GatingCount is the number of Findings that gate the build and are not
	// excused by an allow-list entry — the only number HasDrift needs, kept
	// alongside Findings so buildReport does not have to recompute it.
	GatingCount int
	// StaleEntries lists every allow-list entry that named no real, currently
	// gating finding: an operation no longer declared, a field that no longer
	// diverges, or a typo that never matched anything. Each one is itself a
	// build error — see this command's package doc for why a stale exemption
	// silently forgives the next, unrelated, real divergence that happens to
	// reuse its name.
	StaleEntries   []allowListEntry
	AllowListCount int
	ActionsAudited int
	// FieldsAudited is the total number of specification-side fields
	// compared across every audited action — the sum of len(specShape.Fields)
	// per action, not merely a count of actions. ActionsAudited alone cannot
	// tell a reader "no drift because every field genuinely matched" apart
	// from "no drift because there was nothing to compare": 8 of the 19
	// actions this audit ran against at the time this field was added
	// compare zero fields (a parameterless action such as registries.list),
	// and "Actions audited: 19" reads identically whether the other 11
	// actions compare 51 real fields between them or the comparison engine
	// silently stopped comparing anything at all. Printed alongside
	// ActionsAudited so a reader does not have to already know that
	// distinction exists to ask for it.
	FieldsAudited int
}

// HasDrift reports whether the build must fail: an un-excused gating finding,
// or a stale allow-list entry (see StaleEntries's doc comment for why a
// stale entry is a failure in its own right, not merely a warning).
func (r *auditResult) HasDrift() bool {
	return r.GatingCount > 0 || len(r.StaleEntries) > 0
}

// auditDrift compares every action's catalog-published parameter shape
// against the vendored specification operation it names, in deterministic
// (OperationID) order so two runs over the same inputs produce a
// byte-identical report.
//
// An action whose OperationID resolves in neither vendored spec is a fatal
// input error, not a finding this audit can classify — cmd/audit_1to1's
// coverage cross-check already refuses this at catalog-build time in
// practice, but this function does not assume that check ran first.
func auditDrift(eeOps, ceOps map[string]specOperation, actions []toolutil.ActionSpec, allowList []allowListEntry) (*auditResult, error) {
	sorted := make([]toolutil.ActionSpec, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OperationID < sorted[j].OperationID })

	// allowed indexes every entry by (operationId, field) so a matching
	// finding can look itself up in O(1); used tracks which entries actually
	// excused something, so any entry left unused after every action is
	// audited is reported as stale.
	type allowKey struct{ operationID, field string }
	allowed := make(map[allowKey]*allowListEntry, len(allowList))
	used := make(map[*allowListEntry]bool, len(allowList))
	for i := range allowList {
		allowed[allowKey{allowList[i].OperationID, allowList[i].Field}] = &allowList[i]
	}

	result := &auditResult{AllowListCount: len(allowList)}

	for _, action := range sorted {
		specOp, specEdition, ok := resolveSpecOperation(action.OperationID, eeOps, ceOps)
		if !ok {
			return nil, fmt.Errorf("action %q: OperationID %q resolves in neither vendored spec", action.Name, action.OperationID)
		}

		specShape, err := specdiff.ShapeFromSpec(specOp.Op)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", action.Name, err)
		}
		catalogShape, err := specdiff.ShapeFromCatalog(action)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", action.Name, err)
		}

		result.ActionsAudited++
		result.FieldsAudited += len(specShape.Fields)
		for _, change := range specdiff.Compare(specShape, catalogShape) {
			gating := isGating(change)
			allowListed := false
			var reason, added string
			if entry, ok := allowed[allowKey{action.OperationID, change.JSONName}]; ok && gating {
				allowListed = true
				used[entry] = true
				reason, added = entry.Reason, entry.Added
			}
			result.Findings = append(result.Findings, driftFinding{
				OperationID:     action.OperationID,
				ActionName:      action.Name,
				SpecEdition:     specEdition,
				Change:          change,
				Gating:          gating,
				AllowListed:     allowListed,
				AllowListReason: reason,
				AllowListAdded:  added,
			})
			if gating && !allowListed {
				result.GatingCount++
			}
		}
	}

	for i := range allowList {
		if !used[&allowList[i]] {
			result.StaleEntries = append(result.StaleEntries, allowList[i])
		}
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		a, b := result.Findings[i], result.Findings[j]
		if a.OperationID != b.OperationID {
			return a.OperationID < b.OperationID
		}
		if a.Change.JSONName != b.Change.JSONName {
			return a.Change.JSONName < b.Change.JSONName
		}
		return a.Change.Kind < b.Change.Kind
	})
	sort.Slice(result.StaleEntries, func(i, j int) bool {
		a, b := result.StaleEntries[i], result.StaleEntries[j]
		if a.OperationID != b.OperationID {
			return a.OperationID < b.OperationID
		}
		return a.Field < b.Field
	})

	return result, nil
}
