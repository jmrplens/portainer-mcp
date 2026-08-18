package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSpec(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func TestOperationsIn_ValidSpec_ExtractsMethodsAndPaths(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "spec.json", `{
		"paths": {
			"/stacks": {"get": {}, "post": {}},
			"/stacks/{id}": {"delete": {}}
		}
	}`)

	ops, _, err := operationsIn(dir, "spec.json")
	if err != nil {
		t.Fatalf("operationsIn() error = %v", err)
	}
	want := map[operation]bool{
		{Method: "GET", Path: "/stacks"}:         true,
		{Method: "POST", Path: "/stacks"}:        true,
		{Method: "DELETE", Path: "/stacks/{id}"}: true,
	}
	if len(ops) != len(want) {
		t.Fatalf("operationsIn() = %v, want %v", ops, want)
	}
	for op := range want {
		if !ops[op] {
			t.Errorf("operationsIn() is missing %+v", op)
		}
	}
}

func TestOperationsIn_MissingFile_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, _, err := operationsIn(t.TempDir(), "no-such-file.json"); err == nil {
		t.Error("operationsIn() error = nil, want an error for a missing file")
	}
}

func TestOperationsIn_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "broken.json", `not json`)
	if _, _, err := operationsIn(dir, "broken.json"); err == nil {
		t.Error("operationsIn() error = nil, want an error for unparseable JSON")
	}
}

// TestOperationsIn_ValidSpec_ExtractsOperationIDs pins the addition this
// function exists for: without a captured operationId, cmd/gen_applicability
// cannot link a generated Go client method back to its version applicability.
func TestOperationsIn_ValidSpec_ExtractsOperationIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "spec.json", `{
		"paths": {
			"/system/status": {"get": {"operationId": "SystemStatus"}},
			"/stacks": {"post": {}}
		}
	}`)

	_, ids, err := operationsIn(dir, "spec.json")
	if err != nil {
		t.Fatalf("operationsIn() error = %v", err)
	}
	got, ok := ids[operation{Method: "GET", Path: "/system/status"}]
	if !ok || got != "SystemStatus" {
		t.Errorf("ids[GET /system/status] = %q, %v, want %q, true", got, ok, "SystemStatus")
	}
	if _, ok := ids[operation{Method: "POST", Path: "/stacks"}]; ok {
		t.Error("an operation with no operationId must be absent from the ids map")
	}
}

// TestOperationsIn_NameEscapesDir_ReturnsError guards the re-confinement
// check: every name reaching operationsIn comes from a directory listing, but
// nothing stops a caller from passing a name containing "..", so the
// function must reject it rather than reading outside dir.
func TestOperationsIn_NameEscapesDir_ReturnsError(t *testing.T) {
	t.Parallel()
	parent := t.TempDir()
	dir := filepath.Join(parent, "history")
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeSpec(t, parent, "outside.json", `{"paths": {"/secret": {"get": {}}}}`)

	if _, _, err := operationsIn(dir, filepath.Join("..", "outside.json")); err == nil {
		t.Error("operationsIn() error = nil, want an error for a name that escapes dir")
	}
}

func TestOperationsIn_NonMethodKeys_AreIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// "parameters" and "summary" are valid OpenAPI path-item keys that are not
	// HTTP methods; they must not be mistaken for operations.
	writeSpec(t, dir, "spec.json", `{
		"paths": {
			"/stacks": {"get": {}, "parameters": [], "summary": "stacks"}
		}
	}`)

	ops, _, err := operationsIn(dir, "spec.json")
	if err != nil {
		t.Fatalf("operationsIn() error = %v", err)
	}
	if len(ops) != 1 || !ops[operation{Method: "GET", Path: "/stacks"}] {
		t.Errorf("operationsIn() = %v, want only GET /stacks", ops)
	}
}

func TestLess_NumericVersions_OrdersNumerically(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want bool
	}{
		{"2.39.9", "2.39.10", true},
		{"2.39.10", "2.39.9", false},
		{"2.44.0", "2.44.0", false},
		{"2.39.0", "2.40.0", true},
	}
	for _, tc := range cases {
		if got := less(tc.a, tc.b); got != tc.want {
			t.Errorf("less(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestSortedKeys_ReturnsKeysInAscendingOrder(t *testing.T) {
	t.Parallel()
	got := sortedKeys(map[string]int{"c": 3, "a": 1, "b": 2})
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("sortedKeys() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("sortedKeys() = %v, want %v", got, want)
		}
	}
}

func TestRun_MissingHistoryDir_ReturnsError(t *testing.T) {
	t.Parallel()
	err := run([]string{"-history", filepath.Join(t.TempDir(), "does-not-exist"), "-out", filepath.Join(t.TempDir(), "out.go")})
	if err == nil {
		t.Error("run() error = nil, want an error for a missing history directory")
	}
}

func TestRun_EmptyHistoryDir_ReturnsError(t *testing.T) {
	t.Parallel()
	err := run([]string{"-history", t.TempDir(), "-out", filepath.Join(t.TempDir(), "out.go")})
	if err == nil {
		t.Error("run() error = nil, want an error when no specs are found")
	}
}

// TestRun_ContiguousPresence_WritesOneSpan exercises the common case: an
// operation present in every version of a two-version history gets a single
// span covering both, with an empty MaxVersion because it is still current.
func TestRun_ContiguousPresence_WritesOneSpan(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "ee-2.39.0.json", `{"paths": {"/stacks": {"get": {}}}}`)
	writeSpec(t, dir, "ee-2.44.0.json", `{"paths": {"/stacks": {"get": {}}}}`)
	outPath := filepath.Join(t.TempDir(), "out.go")

	if err := run([]string{"-history", dir, "-out", outPath}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	content := string(got)
	if !strings.Contains(content, `{Method: "GET", Path: "/stacks"}: {{MinVersion: "2.39.0", MaxVersion: ""}}`) {
		t.Errorf("generated file = %s, want a single span with MinVersion 2.39.0 and an empty MaxVersion", content)
	}
}

// TestRun_GapInPresence_ProducesTwoSpans is the regression test for the
// flattening defect found in review: an operation present in the first and
// last of three versions but absent from the middle one must produce TWO
// spans — one closed at the last version where it was present before the
// gap, one starting at the version where it reappears — never a single
// MinVersion/MaxVersion pair that silently claims availability across the
// gap. The gap itself is still reported separately on stderr.
func TestRun_GapInPresence_ProducesTwoSpans(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "ee-2.30.0.json", `{"paths": {"/cloud/gitcredentials": {"get": {}}}}`)
	writeSpec(t, dir, "ee-2.43.0.json", `{"paths": {}}`)
	writeSpec(t, dir, "ee-2.44.0.json", `{"paths": {"/cloud/gitcredentials": {"get": {}}}}`)
	outPath := filepath.Join(t.TempDir(), "out.go")

	if err := run([]string{"-history", dir, "-out", outPath}); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	content := string(got)
	want := `{Method: "GET", Path: "/cloud/gitcredentials"}: {{MinVersion: "2.30.0", MaxVersion: "2.30.0"}, {MinVersion: "2.44.0", MaxVersion: ""}}`
	if !strings.Contains(content, want) {
		t.Errorf("generated file = %s, want two spans: %s", content, want)
	}
	// The flattened, defect-reproducing shape must never appear.
	flattened := `{Method: "GET", Path: "/cloud/gitcredentials"}: {{MinVersion: "2.30.0", MaxVersion: ""}}`
	if strings.Contains(content, flattened) {
		t.Errorf("generated file = %s, contains the flattened single-span shape that hides the 2.43.0 gap", content)
	}
}

func TestRun_UnwritableOutputPath_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "ee-2.44.0.json", `{"paths": {"/stacks": {"get": {}}}}`)

	err := run([]string{"-history", dir, "-out", filepath.Join(dir, "no-such-subdir", "out.go")})
	if err == nil {
		t.Error("run() error = nil, want an error when the output path cannot be written")
	}
}

// TestUnit_ApplySyntheticIDs_RouteNoEditionNames_TakesTheTableName is driven
// by a synthetic lookup rather than internal/specnaming's real one, for the
// same reason resolveActionName in cmd/gen_action_inputs takes its override
// table as an argument: a test that only asserted what the real single-entry
// table produces would pass just as happily against an implementation that
// hardcoded that entry, or one that never consulted a table at all.
//
// The three sub-cases are the three decisions this pass makes. It names a
// route this edition serves and nothing names; it leaves alone a route that
// already resolves, whether from its own document or borrowed; and it writes
// nothing at all for a route this edition does not serve, however willing the
// table is to name it — a name in operationIDs is what makes an action
// declarable on an edition, so inventing one for a route the edition does not
// answer would send calls to a 404.
func TestUnit_ApplySyntheticIDs_RouteNoEditionNames_TakesTheTableName(t *testing.T) {
	t.Parallel()
	nameFor := func(method, path string) (string, bool) {
		if method == "GET" && path == "/groups/{id}" {
			return "GroupInspect", true
		}
		return "", false
	}

	for _, tc := range []struct {
		name      string
		all       map[operation]bool
		ids       map[string]operation
		wantIDs   map[string]operation
		wantCount int
	}{
		{
			name:      "a route the edition serves and nothing names",
			all:       map[operation]bool{{Method: "GET", Path: "/groups/{id}"}: true},
			ids:       map[string]operation{},
			wantIDs:   map[string]operation{"GroupInspect": {Method: "GET", Path: "/groups/{id}"}},
			wantCount: 1,
		},
		{
			name: "a route that already resolves is never overridden",
			all:  map[operation]bool{{Method: "GET", Path: "/groups/{id}"}: true},
			ids:  map[string]operation{"GroupsRealName": {Method: "GET", Path: "/groups/{id}"}},
			// Untouched: no second name is minted for an operation that has one.
			wantIDs:   map[string]operation{"GroupsRealName": {Method: "GET", Path: "/groups/{id}"}},
			wantCount: 0,
		},
		{
			name:      "a route this edition does not serve is not named",
			all:       map[operation]bool{{Method: "GET", Path: "/other"}: true},
			ids:       map[string]operation{},
			wantIDs:   map[string]operation{},
			wantCount: 0,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			opIDs := map[string]map[string]operation{"ce": tc.ids}
			got, err := applySyntheticIDs(map[string]map[operation]bool{"ce": tc.all}, opIDs, nameFor)
			if err != nil {
				t.Fatalf("applySyntheticIDs() error = %v, want nil", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("applySyntheticIDs() reported %v, want %d entr(ies)", got, tc.wantCount)
			}
			if len(opIDs["ce"]) != len(tc.wantIDs) {
				t.Fatalf("operationIDs[ce] = %v, want %v", opIDs["ce"], tc.wantIDs)
			}
			for id, want := range tc.wantIDs {
				if opIDs["ce"][id] != want {
					t.Errorf("operationIDs[ce][%q] = %+v, want %+v", id, opIDs["ce"][id], want)
				}
			}
		})
	}
}

// TestUnit_ApplySyntheticIDs_NameAlreadyTakenByAnotherRoute_IsError is the
// guard that keeps internal/specnaming's "verified collision-free" claim true
// against a future specification rather than only on the day each entry was
// written. Overwriting would move an action's edition on the strength of a
// name this project invented; refusing names both routes and stops the build.
func TestUnit_ApplySyntheticIDs_NameAlreadyTakenByAnotherRoute_IsError(t *testing.T) {
	t.Parallel()
	nameFor := func(method, path string) (string, bool) {
		if method == "GET" && path == "/groups/{id}" {
			return "GroupInspect", true
		}
		return "", false
	}
	opIDs := map[string]map[string]operation{"ce": {
		"GroupInspect": {Method: "GET", Path: "/somewhere/else"},
	}}
	all := map[string]map[operation]bool{"ce": {{Method: "GET", Path: "/groups/{id}"}: true}}

	_, err := applySyntheticIDs(all, opIDs, nameFor)
	if err == nil {
		t.Fatal("applySyntheticIDs() = nil error, want a refusal when the synthetic name is already taken")
	}
	for _, want := range []string{"GroupInspect", "/somewhere/else", "/groups/{id}"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("applySyntheticIDs() error = %v, want it to name %q", err, want)
		}
	}
}

// TestUnit_Run_RouteNoDocumentNames_ReachesTheGeneratedIndex is the
// end-to-end half: the pass must actually be wired into run, after borrowing,
// and its result must reach the emitted operationIDs map. A unit test on
// applySyntheticIDs alone would pass against a run that never called it — the
// exact defect shape this task exists to close.
func TestUnit_Run_RouteNoDocumentNames_ReachesTheGeneratedIndex(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// GET /endpoint_groups/{id} carries no operationId in either document,
	// exactly as the real vendored pair declares it.
	writeSpec(t, dir, "ce-2.44.0.json", `{"paths": {"/endpoint_groups/{id}": {"get": {}}}}`)
	writeSpec(t, dir, "ee-2.44.0.json", `{"paths": {"/endpoint_groups/{id}": {"get": {}}}}`)
	outPath := filepath.Join(t.TempDir(), "out.go")

	if err := run([]string{"-history", dir, "-out", outPath}); err != nil {
		t.Fatalf("run() error = %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	want := `"EndpointGroupInspect": {Method: "GET", Path: "/endpoint_groups/{id}"},`
	if strings.Count(string(got), want) != 2 {
		t.Errorf("generated file = %s\nwant %s once per edition", got, want)
	}
}
