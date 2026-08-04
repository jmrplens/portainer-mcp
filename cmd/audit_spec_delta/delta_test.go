package main

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

func TestUnit_IsStructKind_ClassifiesEveryChangeKind(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		kind         specdiff.ChangeKind
		wantStruct   bool
		wantSchemaOK bool // sanity: every kind must land in exactly one bucket
	}{
		{specdiff.ChangeAdded, true, true},
		{specdiff.ChangeRemoved, true, true},
		{specdiff.ChangeType, true, true},
		{specdiff.ChangeRequiredness, true, true},
		{specdiff.ChangeOrigin, true, true},
		{specdiff.ChangeEnum, false, true},
		{specdiff.ChangeDescription, false, true},
		{specdiff.ChangeTitle, false, true},
		{specdiff.ChangeOperationDescription, false, true},
	} {
		t.Run(string(tc.kind), func(t *testing.T) {
			t.Parallel()
			if got := isStructKind(tc.kind); got != tc.wantStruct {
				t.Errorf("isStructKind(%v) = %v, want %v", tc.kind, got, tc.wantStruct)
			}
		})
	}
}

// deltaFixtureBefore and deltaFixtureAfter are a small, hand-built pair of
// OpenAPI documents exercising every kind of delta this tool must classify:
// one operation unchanged, one removed, one added under an existing domain,
// one added under a tag no domain claims, one whose query parameter's type
// changed (must land in ChangedStruct), and one whose query parameter's
// description changed only (must land in ChangedCosmetic).
const deltaFixtureBefore = `{
  "paths": {
    "/unchanged": {"get": {"operationId": "unchangedOp", "tags": ["widgets"],
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "ID"}]}},
    "/removed": {"get": {"operationId": "removedOp", "tags": ["widgets"], "parameters": []}},
    "/typechanged": {"get": {"operationId": "typeChangedOp", "tags": ["widgets"],
      "parameters": [{"name": "count", "in": "query", "required": false, "schema": {"type": "integer"}, "description": "Count"}]}},
    "/descchanged": {"get": {"operationId": "descChangedOp", "tags": ["widgets"],
      "parameters": [{"name": "name", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Old description"}]}}
  }
}`

const deltaFixtureAfter = `{
  "paths": {
    "/unchanged": {"get": {"operationId": "unchangedOp", "tags": ["widgets"],
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "ID"}]}},
    "/typechanged": {"get": {"operationId": "typeChangedOp", "tags": ["widgets"],
      "parameters": [{"name": "count", "in": "query", "required": false, "schema": {"type": "string"}, "description": "Count"}]}},
    "/descchanged": {"get": {"operationId": "descChangedOp", "tags": ["widgets"],
      "parameters": [{"name": "name", "in": "query", "required": false, "schema": {"type": "string"}, "description": "New description"}]}},
    "/added": {"get": {"operationId": "addedOp", "tags": ["widgets"], "parameters": []}},
    "/newdomain": {"get": {"operationId": "newDomainOp", "tags": ["gizmos"], "parameters": []}}
  }
}`

var deltaFixtureDomainTags = map[string][]string{"widgets": {"widgets"}}

func computeFixtureDelta(t *testing.T) *deltaResult {
	t.Helper()
	before, err := parseSpecOperations([]byte(deltaFixtureBefore))
	if err != nil {
		t.Fatalf("parseSpecOperations(before) error = %v", err)
	}
	after, err := parseSpecOperations([]byte(deltaFixtureAfter))
	if err != nil {
		t.Fatalf("parseSpecOperations(after) error = %v", err)
	}
	result, err := computeDelta(before, after, deltaFixtureDomainTags)
	if err != nil {
		t.Fatalf("computeDelta() error = %v", err)
	}
	return result
}

// TestUnit_ComputeDelta_CountsMatchTheFixtureExactly is the top-line numbers
// this whole tool exists to get right: at a hundred-plus changed operations
// per real release, the counts at the top are what a reader trusts before
// looking at any detail.
func TestUnit_ComputeDelta_CountsMatchTheFixtureExactly(t *testing.T) {
	t.Parallel()
	result := computeFixtureDelta(t)

	if result.AddedCount != 2 {
		t.Errorf("AddedCount = %d, want 2 (addedOp, newDomainOp)", result.AddedCount)
	}
	if result.RemovedCount != 1 {
		t.Errorf("RemovedCount = %d, want 1 (removedOp)", result.RemovedCount)
	}
	if result.ChangedCount != 2 {
		t.Errorf("ChangedCount = %d, want 2 (typeChangedOp, descChangedOp)", result.ChangedCount)
	}
	if result.ChangedStructCount != 1 {
		t.Errorf("ChangedStructCount = %d, want 1 (typeChangedOp only)", result.ChangedStructCount)
	}
}

// TestUnit_ComputeDelta_UnchangedOperation_IsAbsentEverywhere is the
// negative half the task brief explicitly demands: a work list that merely
// asserts a changed operation "is present" would also pass against a tool
// that puts every operation, changed or not, on the list. This test proves
// the opposite half — that an operation identical on both sides appears in
// no domain's Added, Removed, ChangedStruct or ChangedCosmetic bucket at
// all — which is what makes the positive assertion below actually mean
// something.
func TestUnit_ComputeDelta_UnchangedOperation_IsAbsentEverywhere(t *testing.T) {
	t.Parallel()
	result := computeFixtureDelta(t)

	for _, g := range result.Domains {
		for _, ref := range g.Added {
			if ref.OperationID == "unchangedOp" {
				t.Fatalf("domain %q Added contains unchangedOp, want it absent (it did not change)", g.Domain)
			}
		}
		for _, ref := range g.Removed {
			if ref.OperationID == "unchangedOp" {
				t.Fatalf("domain %q Removed contains unchangedOp, want it absent", g.Domain)
			}
		}
		for _, op := range g.ChangedStruct {
			if op.OperationID == "unchangedOp" {
				t.Fatalf("domain %q ChangedStruct contains unchangedOp, want it absent", g.Domain)
			}
		}
		for _, op := range g.ChangedCosmetic {
			if op.OperationID == "unchangedOp" {
				t.Fatalf("domain %q ChangedCosmetic contains unchangedOp, want it absent", g.Domain)
			}
		}
	}
}

// TestUnit_ComputeDelta_ChangedOperations_LandInTheCorrectBucket is the
// positive half: typeChangedOp (a query parameter's type changed) must be
// classified as struct-touching, and descChangedOp (only a description
// changed) must be classified as cosmetic-only — not merely "present
// somewhere in the report".
func TestUnit_ComputeDelta_ChangedOperations_LandInTheCorrectBucket(t *testing.T) {
	t.Parallel()
	result := computeFixtureDelta(t)

	var widgets *domainGroup
	for i := range result.Domains {
		if result.Domains[i].Domain == "widgets" {
			widgets = &result.Domains[i]
		}
	}
	if widgets == nil {
		t.Fatalf("result.Domains has no \"widgets\" entry: %+v", result.Domains)
	}

	if len(widgets.ChangedStruct) != 1 || widgets.ChangedStruct[0].OperationID != "typeChangedOp" {
		t.Errorf("widgets.ChangedStruct = %+v, want exactly [typeChangedOp]", widgets.ChangedStruct)
	}
	if len(widgets.ChangedCosmetic) != 1 || widgets.ChangedCosmetic[0].OperationID != "descChangedOp" {
		t.Errorf("widgets.ChangedCosmetic = %+v, want exactly [descChangedOp]", widgets.ChangedCosmetic)
	}
	if !widgets.ChangedStruct[0].TouchesStruct {
		t.Error("typeChangedOp.TouchesStruct = false, want true")
	}
	if widgets.ChangedCosmetic[0].TouchesStruct {
		t.Error("descChangedOp.TouchesStruct = true, want false")
	}
}

// TestUnit_ComputeDelta_GroupsAddedRemovedByDomain proves the domain
// grouping itself: addedOp (tag "widgets", mapped) lands under "widgets",
// while newDomainOp (tag "gizmos", unmapped by this fixture's DomainTags)
// lands under a visibly distinct, unmapped-tag label rather than silently
// merging into "widgets" or being dropped.
func TestUnit_ComputeDelta_GroupsAddedRemovedByDomain(t *testing.T) {
	t.Parallel()
	result := computeFixtureDelta(t)

	byDomain := make(map[string]*domainGroup, len(result.Domains))
	for i := range result.Domains {
		byDomain[result.Domains[i].Domain] = &result.Domains[i]
	}

	widgets, ok := byDomain["widgets"]
	if !ok {
		t.Fatal(`result.Domains has no "widgets" entry`)
	}
	if len(widgets.Added) != 1 || widgets.Added[0].OperationID != "addedOp" {
		t.Errorf("widgets.Added = %+v, want exactly [addedOp]", widgets.Added)
	}
	if len(widgets.Removed) != 1 || widgets.Removed[0].OperationID != "removedOp" {
		t.Errorf("widgets.Removed = %+v, want exactly [removedOp]", widgets.Removed)
	}

	gizmos, ok := byDomain[unmappedTagPrefix+"gizmos"]
	if !ok {
		t.Fatalf("result.Domains has no %q entry (newDomainOp's unmapped tag), got domains %+v", unmappedTagPrefix+"gizmos", byDomain)
	}
	if len(gizmos.Added) != 1 || gizmos.Added[0].OperationID != "newDomainOp" {
		t.Errorf("gizmos.Added = %+v, want exactly [newDomainOp]", gizmos.Added)
	}
}

// summaryOnlyFixtureBefore/After declare one operation, "summaryOnlyOp",
// whose parameters are byte-identical on both sides — only the operation's
// own "summary" differs. The operation-level "description" is given its own
// explicit, identical literal text on both sides ("A description that never
// changes."), deliberately not left absent: an absent description would
// fall back to Title (specdiff.CleanTitleAndDescription's own rule), which
// itself differs before/after, and would make ChangeOperationDescription
// fire too — the exact "changed more than one thing at once" trap this
// project's standing warning names. With an explicit, unchanging
// description, only Title moves, isolating ChangeTitle in exactly the way a
// test claiming to prove that one kind is detected must. This is the exact
// real-world case Step 5 could not report before OperationShape carried
// Title/Description at all: an operation-level summary change with no
// parameter or body change whatsoever, invisible to a parameter-only
// comparison.
const summaryOnlyFixtureBefore = `{
  "paths": {
    "/summary-only": {"get": {"operationId": "summaryOnlyOp", "tags": ["widgets"], "summary": "Old summary text", "description": "A description that never changes.",
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "ID"}]}}
  }
}`

const summaryOnlyFixtureAfter = `{
  "paths": {
    "/summary-only": {"get": {"operationId": "summaryOnlyOp", "tags": ["widgets"], "summary": "New summary text", "description": "A description that never changes.",
      "parameters": [{"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "ID"}]}}
  }
}`

// TestUnit_ComputeDelta_SummaryOnlyChange_LandsInChangedCosmetic is the
// direct regression test for the gap this task's coordinator identified:
// before OperationShape carried Title/Description, an operation whose only
// difference between two spec versions was its own summary/description
// text was completely invisible to this tool — not merely miscategorised,
// absent. It must now appear, classified as cosmetic (a Title/Description
// change is a copy-paste, never a Go struct edit — see isStructKind's own
// doc comment), never as struct-touching and never silently dropped.
func TestUnit_ComputeDelta_SummaryOnlyChange_LandsInChangedCosmetic(t *testing.T) {
	t.Parallel()
	before, err := parseSpecOperations([]byte(summaryOnlyFixtureBefore))
	if err != nil {
		t.Fatalf("parseSpecOperations(before) error = %v", err)
	}
	after, err := parseSpecOperations([]byte(summaryOnlyFixtureAfter))
	if err != nil {
		t.Fatalf("parseSpecOperations(after) error = %v", err)
	}

	result, err := computeDelta(before, after, deltaFixtureDomainTags)
	if err != nil {
		t.Fatalf("computeDelta() error = %v", err)
	}
	if result.ChangedCount != 1 {
		t.Fatalf("ChangedCount = %d, want 1", result.ChangedCount)
	}
	if result.ChangedStructCount != 0 {
		t.Errorf("ChangedStructCount = %d, want 0: a Title change alone must not count as struct-touching", result.ChangedStructCount)
	}

	var widgets *domainGroup
	for i := range result.Domains {
		if result.Domains[i].Domain == "widgets" {
			widgets = &result.Domains[i]
		}
	}
	if widgets == nil {
		t.Fatalf("result.Domains has no \"widgets\" entry: %+v", result.Domains)
	}
	if len(widgets.ChangedStruct) != 0 {
		t.Errorf("widgets.ChangedStruct = %+v, want none", widgets.ChangedStruct)
	}
	if len(widgets.ChangedCosmetic) != 1 || widgets.ChangedCosmetic[0].OperationID != "summaryOnlyOp" {
		t.Fatalf("widgets.ChangedCosmetic = %+v, want exactly [summaryOnlyOp]", widgets.ChangedCosmetic)
	}
	changes := widgets.ChangedCosmetic[0].Changes
	if len(changes) != 1 || changes[0].Kind != specdiff.ChangeTitle {
		t.Fatalf("summaryOnlyOp.Changes = %+v, want exactly one ChangeTitle (description falls back to title unchanged on both sides, so it must not also fire)", changes)
	}
}

// unresolvableFixtureSpec declares one operation with a header parameter —
// ShapeFromSpec only ever flattens path and query parameters (see its own
// doc comment), so a header parameter is refused, the real, pre-existing
// case this project's own vendored specification has (X-Setup-Token on
// RestoreFromS3 and others).
const unresolvableFixtureSpec = `{
  "paths": {
    "/restore": {"post": {"operationId": "restoreOp", "tags": ["widgets"],
      "parameters": [{"name": "X-Setup-Token", "in": "header", "required": true, "schema": {"type": "string"}}]}}
  }
}`

// TestUnit_ComputeDelta_UnflattenableOperation_IsUnresolvableNotFatal proves
// the exact real-world case Step 5 surfaced: an operation present on both
// sides that specdiff.ShapeFromSpec refuses to flatten (a header parameter,
// or — measured directly against the real 2.44.0 spec — a body field
// colliding with a path/query parameter of the same wire name) must not
// abort the whole comparison. It is reported as Unresolvable, visibly, with
// the refusal's reason, and every other operation is still classified.
func TestUnit_ComputeDelta_UnflattenableOperation_IsUnresolvableNotFatal(t *testing.T) {
	t.Parallel()
	ops, err := parseSpecOperations([]byte(unresolvableFixtureSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}

	result, err := computeDelta(ops, ops, deltaFixtureDomainTags)
	if err != nil {
		t.Fatalf("computeDelta() error = %v, want nil: an unflattenable operation must not abort the run", err)
	}
	if result.UnresolvableCount != 1 {
		t.Fatalf("UnresolvableCount = %d, want 1", result.UnresolvableCount)
	}
	if result.ChangedCount != 0 {
		t.Errorf("ChangedCount = %d, want 0: an unresolvable operation is not the same thing as a changed one", result.ChangedCount)
	}

	var widgets *domainGroup
	for i := range result.Domains {
		if result.Domains[i].Domain == "widgets" {
			widgets = &result.Domains[i]
		}
	}
	if widgets == nil {
		t.Fatalf("result.Domains has no \"widgets\" entry: %+v", result.Domains)
	}
	if len(widgets.Unresolvable) != 1 || widgets.Unresolvable[0].OperationID != "restoreOp" {
		t.Fatalf("widgets.Unresolvable = %+v, want exactly [restoreOp]", widgets.Unresolvable)
	}
	if widgets.Unresolvable[0].Reason == "" {
		t.Error("Unresolvable[0].Reason is empty, want the ShapeFromSpec refusal's own message")
	}
}

// TestUnit_ComputeDelta_UnresolvableOperationAlongsideOthers_DoesNotBlockThem
// proves the "not fatal" property holds even when other, perfectly
// classifiable operations share the run: the real 2.43.0 -> 2.44.0
// comparison has 9 unresolvable operations sitting among 427-442 others, and
// none of the 9 may prevent the rest from being reported.
func TestUnit_ComputeDelta_UnresolvableOperationAlongsideOthers_DoesNotBlockThem(t *testing.T) {
	t.Parallel()
	before, err := parseSpecOperations([]byte(deltaFixtureBefore))
	if err != nil {
		t.Fatalf("parseSpecOperations(before) error = %v", err)
	}
	after, err := parseSpecOperations([]byte(deltaFixtureAfter))
	if err != nil {
		t.Fatalf("parseSpecOperations(after) error = %v", err)
	}
	unresolvable, err := parseSpecOperations([]byte(unresolvableFixtureSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations(unresolvable) error = %v", err)
	}
	for id, op := range unresolvable {
		before[id] = op
		after[id] = op
	}

	result, err := computeDelta(before, after, deltaFixtureDomainTags)
	if err != nil {
		t.Fatalf("computeDelta() error = %v", err)
	}
	if result.UnresolvableCount != 1 {
		t.Errorf("UnresolvableCount = %d, want 1", result.UnresolvableCount)
	}
	// The ordinary fixture's own counts must be completely unaffected by the
	// unresolvable operation sharing the run.
	if result.AddedCount != 2 || result.RemovedCount != 1 || result.ChangedCount != 2 || result.ChangedStructCount != 1 {
		t.Errorf("result = %+v, want the ordinary fixture's counts (Added:2 Removed:1 Changed:2 ChangedStruct:1) unaffected by the unresolvable operation",
			result)
	}
}

// TestUnit_ComputeDelta_InvalidDomainTags_ReturnsError proves computeDelta
// refuses to guess when the DomainTags table itself is ambiguous, rather
// than silently picking a domain by map iteration order.
func TestUnit_ComputeDelta_InvalidDomainTags_ReturnsError(t *testing.T) {
	t.Parallel()
	before, _ := parseSpecOperations([]byte(`{"paths": {}}`))
	after, _ := parseSpecOperations([]byte(`{"paths": {}}`))
	broken := map[string][]string{"a": {"shared"}, "b": {"shared"}}
	if _, err := computeDelta(before, after, broken); err == nil {
		t.Fatal("computeDelta() error = nil, want an error for an ambiguous DomainTags table")
	}
}

// TestUnit_ComputeDelta_EmptyBothSides_ReturnsZeroCounts is the boundary
// case: two documents with nothing declared produce a result with no
// domains and every count zero, never an error.
func TestUnit_ComputeDelta_EmptyBothSides_ReturnsZeroCounts(t *testing.T) {
	t.Parallel()
	empty, _ := parseSpecOperations([]byte(`{"paths": {}}`))
	result, err := computeDelta(empty, empty, deltaFixtureDomainTags)
	if err != nil {
		t.Fatalf("computeDelta() error = %v", err)
	}
	if result.AddedCount != 0 || result.RemovedCount != 0 || result.ChangedCount != 0 {
		t.Errorf("computeDelta() = %+v, want all counts zero", result)
	}
	if len(result.Domains) != 0 {
		t.Errorf("result.Domains = %+v, want none", result.Domains)
	}
}
