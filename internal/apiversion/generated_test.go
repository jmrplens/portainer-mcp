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
// operationId in any published CE or EE spec. As measured against the
// vendored history: CE is missing 16 of 273, EE 4 of 471. The tolerance below
// is set comfortably above both so a genuine regression (an empty or
// near-empty index) still trips it.
func TestOperationIDIndex_CoversNearlyEveryOperation(t *testing.T) {
	const tolerance = 25
	for _, e := range []edition.Edition{edition.CE, edition.EE} {
		ops, ids := len(spans[e]), len(operationIDs[e])
		if ids < ops-tolerance {
			t.Errorf("%s: %d operationIds for %d operations, want nearly all", e, ids, ops)
		}
	}
}
