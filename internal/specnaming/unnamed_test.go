package specnaming_test

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

// TestUnit_SyntheticOperationID_TableDrivenLookup_NamesOnlyTheNameless pins
// both halves of the rule at once, because either half alone is satisfied by
// a wrong implementation.
//
// The positive case is the whole reason the table exists: GET
// /endpoint_groups/{id} is served by both editions and named by neither
// document, so without a name here no catalog action can carry it —
// actioncatalog.Build resolves an action's edition through
// apiversion.ByOperationID and refuses an operationId that resolves in
// neither edition.
//
// The negative cases are what keep the table from becoming the mechanical
// rule it deliberately is not. A rule that derived "<Resource>Inspect" from
// "GET on a path ending in /{id}" would answer for GET /endpoints/{id} and
// GET /stacks/{id} too — routes both documents already name — and would
// quietly mint names for routes nobody has decided on. Asserting only the
// positive case would pass against exactly that implementation.
func TestUnit_SyntheticOperationID_TableDrivenLookup_NamesOnlyTheNameless(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name      string
		method    string
		path      string
		wantID    string
		wantOK    bool
		rationale string
	}{
		{
			name:      "the one route neither vendored document names",
			method:    "GET",
			path:      "/endpoint_groups/{id}",
			wantID:    "EndpointGroupInspect",
			wantOK:    true,
			rationale: "served 200 by both editions, named by neither document, and nothing to borrow across editions",
		},
		{
			name:      "the same route, method as the raw lower-case JSON key",
			method:    "get",
			path:      "/endpoint_groups/{id}",
			wantID:    "EndpointGroupInspect",
			wantOK:    true,
			rationale: "cmd/audit_1to1 consults this while still holding the path-item key verbatim; cmd/gen_applicability has already upper-cased it",
		},
		{
			name:      "a route some document does name",
			method:    "GET",
			path:      "/endpoints/{id}",
			wantOK:    false,
			rationale: "EndpointInspect comes from the document itself; this table is only for the nameless",
		},
		{
			name:      "a nameless route the other edition names",
			method:    "POST",
			path:      "/endpoint_groups",
			wantOK:    false,
			rationale: "Community leaves it nameless but Business calls it EndpointGroupCreate, so borrowIDsAcrossEditions resolves it and it must not be named twice",
		},
		{
			name:      "a route the table does not mention",
			method:    "GET",
			path:      "/some/future/route/{id}",
			wantOK:    false,
			rationale: "naming is a judgement; a route nobody has decided on stays unnamed and is reported as uncovered",
		},
		{
			name:      "the right path with the wrong method",
			method:    "DELETE",
			path:      "/endpoint_groups/{id}",
			wantOK:    false,
			rationale: "the table is keyed by method and path together; EndpointGroupDelete is the document's own name for this one",
		},
		{
			name:      "an empty lookup",
			wantOK:    false,
			rationale: "no route at all must not fall through to some default entry",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotID, gotOK := specnaming.SyntheticOperationID(tc.method, tc.path)
			if gotID != tc.wantID || gotOK != tc.wantOK {
				t.Errorf("SyntheticOperationID(%q, %q) = (%q, %v), want (%q, %v): %s",
					tc.method, tc.path, gotID, gotOK, tc.wantID, tc.wantOK, tc.rationale)
			}
		})
	}
}

// TestUnit_SyntheticOperationIDs_EveryEntry_IsResolvableSortedAndReasoned
// checks the listing accessor against the lookup it lists, and checks that
// every row states why it exists.
//
// The two accessors are separate code paths over the same map, and a listing
// that reported a row SyntheticOperationID does not answer for — a stale
// method casing, say — would make the honesty checks built on top of it
// (cmd/audit_1to1's check that no entry names a route the vendored documents
// already name) assert something about a row nothing else consults.
//
// The Reason assertion is what makes this table's doc comment true rather
// than aspirational. It promises that an entry is a decision somebody wrote
// down, and cmd/audit_1to1 refuses an allow-list entry with an empty reason
// at parse time for the same reason: an unexplained standing exception is
// one nobody can ever judge stale. A Go map literal has no parse step to
// hook, so this test is where that guard lives.
func TestUnit_SyntheticOperationIDs_EveryEntry_IsResolvableSortedAndReasoned(t *testing.T) {
	t.Parallel()
	entries := specnaming.SyntheticOperationIDs()
	if len(entries) == 0 {
		t.Fatal("SyntheticOperationIDs() is empty, want at least the one route neither document names")
	}
	for i, e := range entries {
		if i > 0 {
			prev := entries[i-1]
			if prev.Path > e.Path || (prev.Path == e.Path && prev.Method >= e.Method) {
				t.Errorf("SyntheticOperationIDs() is not sorted by path then method: %+v precedes %+v", prev, e)
			}
		}
		gotID, ok := specnaming.SyntheticOperationID(e.Method, e.Path)
		if !ok || gotID != e.OperationID {
			t.Errorf("SyntheticOperationID(%q, %q) = (%q, %v), want (%q, true) as listed", e.Method, e.Path, gotID, ok, e.OperationID)
		}
		if e.OperationID == "" {
			t.Errorf("SyntheticOperationIDs() row %+v names nothing", e)
		}
		if strings.TrimSpace(e.Reason) == "" {
			t.Errorf("SyntheticOperationIDs() row %s %s -> %s states no Reason; this table only holds names somebody decided on and wrote down",
				e.Method, e.Path, e.OperationID)
		}
	}
}
