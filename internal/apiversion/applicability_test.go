package apiversion

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// These tests replace the package-level ranges variable, so none of them may
// run in parallel with each other or with any test that reads the real
// generated table — doing so would race on that shared variable.

func TestAvailable_WithinRange_IsTrue(t *testing.T) {
	ranges = map[edition.Edition]map[Operation]Range{
		edition.EE: {{Method: "GET", Path: "/stacks"}: {MinVersion: "2.30.0"}},
	}
	if !Available(edition.EE, Operation{Method: "GET", Path: "/stacks"}, "2.44.0") {
		t.Error("operation reported unavailable on a newer server")
	}
}

func TestAvailable_BeforeMinVersion_IsFalse(t *testing.T) {
	ranges = map[edition.Edition]map[Operation]Range{
		edition.EE: {{Method: "POST", Path: "/addons"}: {MinVersion: "2.42.0"}},
	}
	if Available(edition.EE, Operation{Method: "POST", Path: "/addons"}, "2.39.5") {
		t.Error("operation reported available on a server older than its MinVersion")
	}
}

// /cloud/gitcredentials was removed in 2.43.0 and reintroduced in 2.44.0 —
// confirmed against a live 2.43.0 EE instance, where the route does not exist.
func TestAvailable_WithinRemovedWindow_IsFalse(t *testing.T) {
	ranges = map[edition.Edition]map[Operation]Range{
		edition.EE: {{Method: "GET", Path: "/cloud/gitcredentials"}: {MinVersion: "2.34.0", MaxVersion: "2.42.0"}},
	}
	if Available(edition.EE, Operation{Method: "GET", Path: "/cloud/gitcredentials"}, "2.43.0") {
		t.Error("operation reported available inside the window where it was removed")
	}
}

func TestAvailable_UnknownOperation_IsFalse(t *testing.T) {
	ranges = map[edition.Edition]map[Operation]Range{edition.EE: {}}
	if Available(edition.EE, Operation{Method: "GET", Path: "/invented"}, "2.44.0") {
		t.Error("an operation absent from the table was reported available")
	}
}

// An unparseable server version must not silently hide every operation.
func TestAvailable_UnparseableServerVersion_FallsBackToAvailable(t *testing.T) {
	ranges = map[edition.Edition]map[Operation]Range{
		edition.EE: {{Method: "GET", Path: "/stacks"}: {MinVersion: "2.30.0"}},
	}
	if !Available(edition.EE, Operation{Method: "GET", Path: "/stacks"}, "not-a-version") {
		t.Error("an unparseable server version hid a known operation; it should fall back to available")
	}
}

func TestCompareVersions_Ordering(t *testing.T) {
	t.Parallel()
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
