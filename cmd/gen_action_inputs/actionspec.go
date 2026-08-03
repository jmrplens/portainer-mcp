package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// actionSpecFields is every toolutil.ActionSpec field this generator
// determines mechanically, purely from the vendored specification: the
// canonical name (Task 1's ResolveActionName), the domain and operationId
// identity, a title and description derived from the operation's own
// "summary"/"description", the edition that offers it, and the three danger
// flags derived from its HTTP verb.
//
// Input and Handler — the other two fields toolutil.ActionSpec.Validate
// requires — are deliberately not here: they are Task 1's and Task 2's own
// outputs (inputStructName, handlerFuncName/buildHandlerSpec), produced from
// the same operation by code this file does not duplicate. Combining every
// field into one literal ActionSpec is Task 4's job, once a domain's
// generated output actually replaces hand-written code; this type only
// carries what actionspec.go itself computes, so it is testable in
// isolation from whichever Go type or handler symbol a given operation
// happens to resolve to.
type actionSpecFields struct {
	Name        string
	Domain      string
	OperationID string
	Title       string
	Description string
	Edition     edition.Edition
	Mutating    bool
	Destructive bool
	Idempotent  bool
}

// buildActionSpecFields assembles one operation's mechanical ActionSpec
// fields. ceOperationIDs is the set of operationIDs the vendored Community
// Edition specification declares (see ceOperationIDSet) — the only extra
// input this needs beyond what Task 1 and the operation itself already
// carry.
func buildActionSpecFields(domainName string, op operation, ceOperationIDs map[string]bool) (actionSpecFields, error) {
	name, err := ResolveActionName(domainName, op.OperationID)
	if err != nil {
		return actionSpecFields{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	title, description, err := cleanTitleAndDescription(op)
	if err != nil {
		return actionSpecFields{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	mutating, destructive, idempotent, err := dangerFlags(op.Method)
	if err != nil {
		return actionSpecFields{}, fmt.Errorf("%s: %w", op.OperationID, err)
	}
	return actionSpecFields{
		Name:        name,
		Domain:      domainName,
		OperationID: op.OperationID,
		Title:       title,
		Description: description,
		Edition:     editionOf(op.OperationID, ceOperationIDs),
		Mutating:    mutating,
		Destructive: destructive,
		Idempotent:  idempotent,
	}, nil
}

// accessPolicyPrefix is the boilerplate line every operation's own
// "description" in the vendored specification ends with — for all but two
// of the 442 operations across both specs, its very last line — of the form
// "**Access policy**: <who may call this>". It documents authorization, an
// implementation detail, not what the operation does, so cleanTitleAndDescription
// strips any line starting with it rather than surfacing it to a model as if
// it were part of the operation's own description.
const accessPolicyPrefix = "**access policy**"

// cleanTitleAndDescription derives Title and Description from op's own
// "summary" and "description" — the two fields the vendored specification
// actually carries for exactly this purpose — instead of composing new
// prose by hand: with 423 operations still to declare, hand-authoring both
// for every one repeats precisely the transcription problem
// toolutil.ActionSpec.Input already exists to avoid for parameters (see this
// package's own doc comment).
//
// Title is op.Summary, trimmed. Description is op.Description with every
// "**Access policy**: ..." line removed (see accessPolicyPrefix) and the
// remainder trimmed; when that leaves nothing — systemInfo's own
// description is exactly one such line, so stripping it leaves "" —
// Description falls back to Title rather than this generator publishing an
// empty string no model could use to decide anything.
//
// Both are refused, not defaulted, when the specification carries nothing
// to derive them from: every operation in both vendored specifications has
// a non-empty summary today, so an empty one is a specification defect this
// generator has never seen and must not paper over with a placeholder
// indistinguishable from a real title.
func cleanTitleAndDescription(op operation) (title, description string, err error) {
	title = strings.TrimSpace(op.Summary)
	if title == "" {
		return "", "", fmt.Errorf("operation has no summary to derive Title from")
	}

	lines := strings.Split(op.Description, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), accessPolicyPrefix) {
			continue
		}
		kept = append(kept, line)
	}
	description = strings.TrimSpace(strings.Join(kept, "\n"))
	if description == "" {
		description = title
	}
	return title, description, nil
}

// dangerFlags derives the three ActionSpec danger flags from an operation's
// HTTP verb — the conservative baseline this task exists to compute: GET and
// HEAD are read-only; DELETE is destructive and idempotent; PUT is mutating
// and idempotent; POST and PATCH are mutating and not idempotent. Any other
// verb is refused rather than guessed at: none of the 442 combined
// operations across both vendored specifications use one today, so a future
// spec that does must fail generation loudly rather than silently default
// to "safe".
//
// This is a floor, not a ceiling. suspectDangerMismatch below is what finds
// the operations where the verb itself is misleading — Portainer's own
// POST-verbed bulk deletes, and system.upgrade's irreversible edition
// conversion — every one of which needs a hand override in its domain file;
// this function deliberately does not attempt to guess at those itself.
func dangerFlags(method string) (mutating, destructive, idempotent bool, err error) {
	switch method {
	case http.MethodGet, http.MethodHead:
		return false, false, false, nil
	case http.MethodDelete:
		return true, true, true, nil
	case http.MethodPut:
		return true, false, true, nil
	case http.MethodPost, http.MethodPatch:
		return true, false, false, nil
	default:
		return false, false, false, fmt.Errorf(
			"verb %q has no danger-flag rule; a future specification change must decide read-only/mutating/destructive/idempotent for it explicitly rather than default to one", method)
	}
}

// editionOf reports the minimum edition offering operationID: CE when the
// vendored Community Edition specification also declares it, EE when it
// does not — matching toolutil.ActionSpec.Edition's own "CE means both"
// convention (see internal/toolutil/action.go). ceOperationIDs is built by
// ceOperationIDSet below from the same decoding this generator already
// applies to the EE specification, so an operationId's exported spelling
// compares consistently on both sides regardless of which case either
// specification happens to use for it.
//
// An operationId absent from the EE specification altogether (CE-only, such
// as systemUpgrade) never reaches this function at all: this generator's
// domain processing enumerates operations from the EE specification (see
// main.go's -spec default), and system.go's SystemUpgrade override is
// exactly the hand-written consequence of that — see this file's own
// dangerMismatchWarnings, which is what still finds it despite that gap.
func editionOf(operationID string, ceOperationIDs map[string]bool) edition.Edition {
	if ceOperationIDs[operationID] {
		return edition.CE
	}
	return edition.EE
}

// ceOperationIDSet flattens operationsByDomain's per-tag grouping into the
// flat set editionOf needs: every operationId the vendored Community
// Edition specification declares, already exported (see exportedName) the
// same way every operationId this generator otherwise handles is.
func ceOperationIDSet(byTag map[string][]operation) map[string]bool {
	out := make(map[string]bool)
	for _, ops := range byTag {
		for _, op := range ops {
			out[op.OperationID] = true
		}
	}
	return out
}

// dangerKeywords are substrings this generator treats as evidence that an
// operation's own path or operationId describes deletion or an irreversible
// change, independent of what dangerFlags' conservative, verb-only
// derivation would conclude for it. "upgrade" is here for exactly one
// documented reason: system.upgrade (POST /system/upgrade) converts a
// Community instance to Business Edition, which cannot be undone, and is
// already marked Destructive by hand in internal/tools/system/system.go —
// this keyword is what lets that override be found by this generator
// instead of merely remembered by whoever wrote it. Every other entry is
// justified by a real operation flagged in the vendored specifications
// today: the seven Kubernetes bulk deletes at
// POST /kubernetes/{id}/<kind>/delete plus five siblings of the same shape,
// POST /endpoints/delete, POST /licenses/remove and
// PUT /endpoints/{id}/association (operationId EndpointAssociationDelete).
var dangerKeywords = []string{"delete", "remove", "destroy", "purge", "wipe", "revoke", "upgrade"}

// suspectDangerMismatch reports whether op's own path or operationId
// suggests deletion or an irreversible change even though dangerFlags'
// conservative, verb-only derivation would not mark it Destructive.
//
// GET, HEAD and DELETE operations are excluded on purpose, not merely
// because dangerFlags already agrees with them for every real case: a GET
// whose path happens to mention "upgrade" (omni's two ...UpgradeStatus
// polling endpoints are the real example in the vendored specifications) is
// genuinely read-only regardless of the word, and a DELETE is already
// Destructive by dangerFlags' own rule — flagging either would only produce
// noise a reviewer has to learn to ignore rather than a real candidate to
// check.
func suspectDangerMismatch(op operation) bool {
	switch op.Method {
	case http.MethodGet, http.MethodHead, http.MethodDelete:
		return false
	}
	haystack := strings.ToLower(op.Path + " " + op.OperationID)
	for _, kw := range dangerKeywords {
		if strings.Contains(haystack, kw) {
			return true
		}
	}
	return false
}

// dangerMismatchWarnings returns one human-readable line per operation in
// ops that suspectDangerMismatch flags, sorted for deterministic output —
// the report this task's brief requires so "the override list is generated
// rather than remembered": a reviewer reads this list once per wave and
// decides which entries need a hand-written Destructive: true override in
// their own domain file, the same way system.go's SystemUpgrade already
// carries one that this function must surface rather than silently miss.
func dangerMismatchWarnings(ops []operation) []string {
	var warnings []string
	for _, op := range ops {
		if suspectDangerMismatch(op) {
			warnings = append(warnings, fmt.Sprintf(
				"%s %s (operationId %s): verb-derived flags are not Destructive, but the path or operationId suggests deletion or an irreversible change",
				op.Method, op.Path, op.OperationID))
		}
	}
	sort.Strings(warnings)
	return warnings
}

// actionNarrative carries the ActionSpec fields no specification determines:
// Usage, RelatedActions, Aliases, Tags and any ParameterGuidance beyond the
// central defaults toolutil.FillScopeParameterGuidance already applies. A
// specification says nothing about which action a model should reach for
// next, or what a caller elsewhere in Portainer's own documentation calls
// the same operation — that is domain judgement, authored once by hand and
// never regenerated.
type actionNarrative struct {
	Usage             string
	RelatedActions    []string
	Aliases           []string
	Tags              []string
	ParameterGuidance map[string]toolutil.ParameterGuidance
}

// narrativeHook is what a domain's own hand-written file supplies: one
// function, keyed by OperationID, returning whatever narrative metadata that
// operation has been authored with — the zero actionNarrative for every
// operationId nobody has written any for yet.
//
// A generated ActionSpec calls this hook for its narrative fields rather
// than a generated file ever writing
// Usage/RelatedActions/Aliases/Tags/ParameterGuidance itself — see
// applyNarrative — which is the whole mechanism that keeps regenerating
// from discarding authored text: the hook's implementation lives in a hand
// file this generator never overwrites, so re-running the generator changes
// only the mechanical fields buildActionSpecFields computes and leaves
// whatever the hook returns completely untouched.
type narrativeHook func(operationID string) actionNarrative

// noNarrative is the hook a domain with nothing hand-authored yet supplies —
// every action still gets its mechanical fields, with an empty narrative,
// rather than this generator refusing to run until every action has prose.
func noNarrative(string) actionNarrative { return actionNarrative{} }

// generatedActionSpec is the complete set of ActionSpec fields this
// generator ever determines: buildActionSpecFields' mechanical fields plus
// whatever a domain's narrativeHook supplies for the same operationId. It
// still excludes Input and Handler — see actionSpecFields' own doc comment
// for why those stay Task 1's and Task 2's outputs, combined only where
// Task 4 renders an actual ActionSpec literal.
type generatedActionSpec struct {
	actionSpecFields
	Usage             string
	RelatedActions    []string
	Aliases           []string
	Tags              []string
	ParameterGuidance map[string]toolutil.ParameterGuidance
}

// applyNarrative combines fields (this file's own mechanical derivation)
// with whatever hook returns for the same operationId. hook may be nil,
// treated the same as noNarrative — a domain that has not written one yet
// still generates.
//
// This is the function call this file's own doc comment on narrativeHook
// promises: fields is recomputed every run, from the specification; hook's
// return value is not recomputed at all, it is read from wherever the
// domain's own hand file defines it, so calling it here — rather than this
// function or its caller ever assigning a literal string to Usage, Tags,
// Aliases, RelatedActions or ParameterGuidance — is what keeps a
// regeneration from being able to discard authored text: there is no
// generated literal to discard.
func applyNarrative(fields actionSpecFields, hook narrativeHook) generatedActionSpec {
	if hook == nil {
		hook = noNarrative
	}
	narrative := hook(fields.OperationID)
	return generatedActionSpec{
		actionSpecFields:  fields,
		Usage:             narrative.Usage,
		RelatedActions:    narrative.RelatedActions,
		Aliases:           narrative.Aliases,
		Tags:              narrative.Tags,
		ParameterGuidance: narrative.ParameterGuidance,
	}
}
