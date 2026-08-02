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

	ops, err := operationsIn(dir, "spec.json")
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
	if _, err := operationsIn(t.TempDir(), "no-such-file.json"); err == nil {
		t.Error("operationsIn() error = nil, want an error for a missing file")
	}
}

func TestOperationsIn_InvalidJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeSpec(t, dir, "broken.json", `not json`)
	if _, err := operationsIn(dir, "broken.json"); err == nil {
		t.Error("operationsIn() error = nil, want an error for unparseable JSON")
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

	if _, err := operationsIn(dir, filepath.Join("..", "outside.json")); err == nil {
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

	ops, err := operationsIn(dir, "spec.json")
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

// TestRun_ContiguousPresence_WritesRangesWithoutGaps exercises the common
// case: an operation present in every version of a two-version history gets a
// range spanning both, with an empty MaxVersion because it is still current.
func TestRun_ContiguousPresence_WritesRangesWithoutGaps(t *testing.T) {
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
	if !strings.Contains(content, `{Method: "GET", Path: "/stacks"}: {MinVersion: "2.39.0", MaxVersion: ""}`) {
		t.Errorf("generated file = %s, want a MinVersion of 2.39.0 and an empty MaxVersion", content)
	}
}

// TestRun_GapInPresence_IsRecordedInMinAndMaxVersion exercises the exact shape
// of the removed-then-reintroduced case: an operation present in the first
// and last of three versions but absent from the middle one must still
// produce a single MinVersion/MaxVersion pair, spanning the gap rather than
// reporting it — the gap is reported separately on stderr, not encoded in the
// table itself.
func TestRun_GapInPresence_IsRecordedInMinAndMaxVersion(t *testing.T) {
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
	if !strings.Contains(content, `{Method: "GET", Path: "/cloud/gitcredentials"}: {MinVersion: "2.30.0", MaxVersion: ""}`) {
		t.Errorf("generated file = %s, want MinVersion 2.30.0 and empty MaxVersion (last=newest despite the gap)", content)
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
