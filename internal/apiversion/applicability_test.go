package apiversion

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// These tests replace the package-level spans variable, so none of them may
// declare t.Parallel(): they must run sequentially, both with respect to each
// other and to generated_test.go, which reads the real generated table. None
// of the tests in this package (including this file's TestCompareVersions_Ordering,
// which touches no shared state) uses t.Parallel(), so there is no ambiguity
// about ordering.
//
// Sequential ordering alone is not enough, though: an assignment to a
// package-level variable outlives the test that made it. Every test here
// restores the real generated table via t.Cleanup, so the tests in
// generated_test.go — which run after these alphabetically, in the same
// process — see the actual output of cmd/gen_applicability rather than
// whatever fixture the last mutating test happened to leave behind.

func withRestoredSpans(t *testing.T) {
	t.Helper()
	original := spans
	t.Cleanup(func() { spans = original })
}

func TestAvailable_WithinRange_IsTrue(t *testing.T) {
	withRestoredSpans(t)
	spans = map[edition.Edition]map[Operation][]Span{
		edition.EE: {{Method: "GET", Path: "/stacks"}: {{MinVersion: "2.30.0"}}},
	}
	if !Available(edition.EE, Operation{Method: "GET", Path: "/stacks"}, "2.44.0") {
		t.Error("operation reported unavailable on a newer server")
	}
}

func TestAvailable_BeforeMinVersion_IsFalse(t *testing.T) {
	withRestoredSpans(t)
	spans = map[edition.Edition]map[Operation][]Span{
		edition.EE: {{Method: "POST", Path: "/addons"}: {{MinVersion: "2.42.0"}}},
	}
	if Available(edition.EE, Operation{Method: "POST", Path: "/addons"}, "2.39.5") {
		t.Error("operation reported available on a server older than its MinVersion")
	}
}

// /cloud/gitcredentials was withdrawn in 2.43.0 and reintroduced in 2.44.0 —
// confirmed against a live 2.43.0 EE instance, where the route does not exist.
// This models the operation as the two spans it actually has, rather than a
// single range that would flatten the gap away.
func TestAvailable_InsideAReintroductionGap_IsFalse(t *testing.T) {
	withRestoredSpans(t)
	spans = map[edition.Edition]map[Operation][]Span{
		edition.EE: {{Method: "GET", Path: "/cloud/gitcredentials"}: {
			{MinVersion: "2.34.0", MaxVersion: "2.42.0"},
			{MinVersion: "2.44.0", MaxVersion: ""},
		}},
	}
	op := Operation{Method: "GET", Path: "/cloud/gitcredentials"}

	if !Available(edition.EE, op, "2.40.0") {
		t.Error("2.40.0 is inside the first span but reported unavailable")
	}
	if Available(edition.EE, op, "2.43.0") {
		t.Error("2.43.0 is inside the withdrawal gap but reported available")
	}
	if !Available(edition.EE, op, "2.44.0") {
		t.Error("2.44.0 is inside the second span but reported unavailable")
	}
}

func TestAvailable_UnknownOperation_IsFalse(t *testing.T) {
	withRestoredSpans(t)
	spans = map[edition.Edition]map[Operation][]Span{edition.EE: {}}
	if Available(edition.EE, Operation{Method: "GET", Path: "/invented"}, "2.44.0") {
		t.Error("an operation absent from the table was reported available")
	}
}

// An unparseable server version must not silently hide every operation.
func TestAvailable_UnparseableServerVersion_FallsBackToAvailable(t *testing.T) {
	withRestoredSpans(t)
	spans = map[edition.Edition]map[Operation][]Span{
		edition.EE: {{Method: "GET", Path: "/stacks"}: {{MinVersion: "2.30.0"}}},
	}
	if !Available(edition.EE, Operation{Method: "GET", Path: "/stacks"}, "not-a-version") {
		t.Error("an unparseable server version hid a known operation; it should fall back to available")
	}
}

func TestCompareVersions_Ordering(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"2.44.0", "2.43.0", 1},
		{"2.39.5", "2.40.0", -1},
		{"2.39.10", "2.39.9", 1}, // numeric, not lexicographic
		{"2.44.0", "2.44.0", 0},
	}
	for _, tc := range cases {
		got, err := compareVersions(tc.a, tc.b)
		if err != nil {
			t.Errorf("compareVersions(%q,%q) error = %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("compareVersions(%q,%q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
