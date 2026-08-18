package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestUnit_Run_UnknownDomainDirectory_WritesNothingAndNamesEveryOne pins the
// pre-flight resolution pass in run().
//
// Resolving a domain's operations used to happen inside the write loop, so a
// directory with no toolutil.DomainTags entry aborted the run part-way:
// domains sorting earlier already had their generated files on disk, and the
// end-of-run report — including the refusal list, which is the reason to run
// this at all — never printed. Wave 1 stage B hit exactly that by adding
// internal/tools/redact, a shared helper rather than a domain.
//
// Two properties, and the second is the one that actually costs time when it
// is missing: nothing is written when any directory fails to resolve, and
// every failing directory is named in one run rather than one per rerun.
func TestUnit_Run_UnknownDomainDirectory_WritesNothingAndNamesEveryOne(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// "tags" is a real domain with a DomainTags entry and no refusals, so it
	// would generate cleanly on its own. It sorts before both unknowns, which
	// is what makes it evidence: under the old behaviour its files were
	// already written by the time the first unknown was reached.
	for _, name := range []string{"tags", "zzz_helper", "zzz_other"} {
		if err := os.MkdirAll(filepath.Join(dir, name), 0o750); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
	}

	err := run([]string{"-spec", "../../api/specs/ee-2.44.0.json", "-tools-dir", dir})
	if err == nil {
		t.Fatal("run() = nil error, want a refusal: zzz_helper and zzz_other have no toolutil.DomainTags entry")
	}

	for _, want := range []string{"zzz_helper", "zzz_other"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error does not name %q, so an operator fixing one directory at a time reruns to be told about the next: %v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "nothing was written") {
		t.Errorf("error = %q, want it to state that nothing was written — an operator who cannot tell whether the tree was touched has to check by hand", err)
	}

	entries, readErr := os.ReadDir(filepath.Join(dir, "tags"))
	if readErr != nil {
		t.Fatalf("read tags dir: %v", readErr)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("tags/ holds %v after a failed run; the pre-flight pass must reject before anything is written, or a failure leaves a half-generated tree", names)
	}
}
