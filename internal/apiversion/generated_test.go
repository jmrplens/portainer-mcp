package apiversion

import (
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
// emits an empty table.
func TestGeneratedTable_IsPopulated(t *testing.T) {
	for _, e := range []edition.Edition{edition.CE, edition.EE} {
		if got := len(spans[e]); got < 200 {
			t.Errorf("%s table has %d operations, want at least 200", e, got)
		}
	}
}
