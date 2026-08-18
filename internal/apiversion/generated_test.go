package apiversion

import (
	"net/http"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// This file reads the real generated table (spans, produced by
// cmd/gen_applicability). It must never run in parallel with the tests in
// applicability_test.go, which replace spans wholesale — neither this file
// nor that one declares t.Parallel(), so Go runs every test in this package
// to completion, in order, before starting the next; there is no
// interleaving between the two files.

// TestGeneratedTable_GitcredentialsGap_IsEncoded pins the generated data, not
// the mechanism. /cloud/gitcredentials was withdrawn in 2.43.0 and
// reintroduced in 2.44.0 — confirmed against a live Portainer 2.43.0 EE
// instance, where the route answers "404 page not found". A table that
// flattens the gap into one MinVersion/MaxVersion range claims the operation
// exists on the one version where it provably does not.
func TestGeneratedTable_GitcredentialsGap_IsEncoded(t *testing.T) {
	op := Operation{Method: "GET", Path: "/cloud/gitcredentials"}

	found, ok := Applicability(edition.EE, op)
	if !ok {
		t.Fatalf("%s %s missing from the generated EE table", op.Method, op.Path)
	}
	if len(found) != 2 {
		t.Fatalf("spans = %+v, want two: the gap in 2.43.0 must be encoded", found)
	}
	if Available(edition.EE, op, "2.43.0") {
		t.Error("the generated table reports the operation available on 2.43.0, where the route does not exist")
	}
	if !Available(edition.EE, op, "2.42.0") || !Available(edition.EE, op, "2.44.0") {
		t.Error("the versions on either side of the gap must remain available")
	}
}

// TestGeneratedTable_IsPopulated guards against a generator that silently
// emits an empty table. The floors are deliberately just below the current
// counts (CE 273, EE 471 as of this writing) so a genuine loss — a generator
// regression dropping a quarter of the table — trips this test, while a
// handful of upstream removals between spec releases does not.
func TestGeneratedTable_IsPopulated(t *testing.T) {
	floors := map[edition.Edition]int{edition.CE: 260, edition.EE: 450}
	for _, e := range []edition.Edition{edition.CE, edition.EE} {
		if got := len(spans[e]); got < floors[e] {
			t.Errorf("%s table has %d operations, want at least %d", e, got, floors[e])
		}
	}
}

// TestByOperationID_KnownID_ResolvesToItsOperation uses a mapping that is not
// guessable from the path, which is the entire reason this index exists.
func TestByOperationID_KnownID_ResolvesToItsOperation(t *testing.T) {
	op, ok := ByOperationID(edition.EE, "SystemStatus")
	if !ok {
		t.Fatal("SystemStatus missing from the EE operationId index")
	}
	if op.Method != http.MethodGet || op.Path != "/system/status" {
		t.Errorf("resolved to %+v, want GET /system/status", op)
	}
}

func TestByOperationID_UnknownID_IsNotFound(t *testing.T) {
	if _, ok := ByOperationID(edition.EE, "NoSuchOperation"); ok {
		t.Error("an invented operationId resolved to an operation")
	}
}

// TestOperationIDIndex_CoversNearlyEveryOperation guards against a generator
// that silently emits an empty or partial index.
//
// The newest EE spec (2.44.0) alone has exactly one operation with no
// operationId (GET /endpoint_groups/{id}). But operationIDs is built across
// every historical spec — a withdrawn operation resolves through the newest
// version in which it appeared, which may predate operationId annotations
// existing at all — and a handful of route families (the /webhooks CRUD
// endpoints, the /websocket/* upgrade endpoints) have never carried an
// operationId in any published CE or EE spec. Those are repaired by
// cmd/gen_applicability's two naming passes: borrowIDsAcrossEditions, which
// takes a name the other edition publishes, and applySyntheticIDs, which
// takes internal/specnaming's explicit name for a route neither edition
// names. What is left over is what nothing can name: measured against the
// vendored history, 2 of CE's 273 operations and 3 of EE's 471 resolve to no
// operationId at all — GET /endpoints/{id}/kubernetes/helm/{name}, POST
// /docker/{environmentId}/dashboard and (EE only) GET
// /websocket/microk8s-shell, every one of them withdrawn upstream by 2.41.0
// and never named while it existed. The counts this test compares are id
// counts rather than operation counts, and they differ slightly from those
// five because a renamed operationId leaves both names in the index. The
// tolerance below is set far above either reading, so a genuine regression
// (an empty or near-empty index) still trips it while a handful of upstream
// removals between spec releases does not.
func TestOperationIDIndex_CoversNearlyEveryOperation(t *testing.T) {
	const tolerance = 25
	for _, e := range []edition.Edition{edition.CE, edition.EE} {
		ops, ids := len(spans[e]), len(operationIDs[e])
		if ids < ops-tolerance {
			t.Errorf("%s: %d operationIds for %d operations, want nearly all", e, ids, ops)
		}
	}
}

// TestGeneratedTable_RouteNoDocumentNames_ResolvesInBothEditions pins the
// generated data for the one route neither vendored document gives an
// operationId.
//
// GET /endpoint_groups/{id} answers 200 on Community and on Business —
// measured against both, Business differing only by an extra Policies field
// — but Community leaves it nameless and so does Business, so there was
// nothing for cmd/gen_applicability's borrowIDsAcrossEditions to borrow.
// operationIDs is what internal/tools/actioncatalog resolves an action's
// edition through, and an operationId absent from both editions of this index
// makes actioncatalog.Build refuse the action outright as "resolves in
// neither edition" — so a route Portainer serves could not be declared at
// all. The name comes from internal/specnaming's explicit table, applied by
// applySyntheticIDs; this test fails if that table entry is removed, if the
// pass stops running, or if it lands in only one edition.
func TestGeneratedTable_RouteNoDocumentNames_ResolvesInBothEditions(t *testing.T) {
	for _, e := range []edition.Edition{edition.CE, edition.EE} {
		op, ok := ByOperationID(e, "EndpointGroupInspect")
		if !ok {
			t.Errorf("%s: EndpointGroupInspect missing from the operationId index; an action carrying it cannot be declared at all", e)
			continue
		}
		if op.Method != http.MethodGet || op.Path != "/endpoint_groups/{id}" {
			t.Errorf("%s: EndpointGroupInspect resolved to %+v, want GET /endpoint_groups/{id}", e, op)
		}
		if !Available(e, op, "2.44.0") {
			t.Errorf("%s: the generated table reports GET /endpoint_groups/{id} unavailable on 2.44.0, where both editions serve it", e)
		}
	}
}
