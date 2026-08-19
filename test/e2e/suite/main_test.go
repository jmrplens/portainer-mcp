//go:build e2e

// Package suite drives every tool surface and edition against a live,
// provisioned Portainer estate over the MCP protocol.
//
// Every file here carries the e2e build tag and is excluded from `go test
// ./...` and `make test`: it needs Docker, a running estate (see
// `make e2e-up`), and real HTTP round trips, none of which belong in the fast,
// Docker-free default test run.
package suite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

var (
	estate   harness.Estate
	sessions *Sessions
)

// TestMain loads the estate and builds every session before any test runs,
// and fails the whole run rather than skipping when no estate is present.
//
// Running with -tags e2e is an explicit request for these tests: silently
// passing an unprovisioned run is exactly the decorative-suite failure this
// phase exists to prevent. An estate file naming no leg at all is that case,
// and harness.LoadEstate still refuses it.
//
// A missing individual leg is not. A missing Business Edition leg never was
// (see Sessions.For) — a contributor without the licence must still be able
// to run the Community Edition suites — and since CI began provisioning the
// compose legs and the Kubernetes leg in two separate jobs, a missing
// compose leg is equally ordinary. What each such run owes the reader is a
// statement of what it did not measure, which is what legSummary below and
// the per-suite skips exist to give.
func TestMain(m *testing.M) {
	path := os.Getenv(harness.EstateFileEnv)
	if path == "" {
		// harness.LoadEstate rejects a relative path that climbs above its
		// starting directory (rejectEscapingPath), by design: it exists to
		// reject a path escaping a sandbox, not to bless every ".." a caller
		// might write. Resolving the default to an absolute path here, rather
		// than passing the relative "../.estate.json" some readers might
		// expect, keeps this default on the same "absolute path, trusted as
		// given" footing that LoadEstate's own doc comment says every caller
		// in this codebase uses.
		abs, err := filepath.Abs("../.estate.json")
		if err != nil {
			fmt.Fprintf(os.Stderr, "e2e: resolve default estate path: %v\n", err)
			os.Exit(1)
		}
		path = abs
	}
	var err error
	estate, err = harness.LoadEstate(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: %v\nrun `make e2e-up` first\n", err)
		os.Exit(1)
	}

	// cleanupOrphans is the net for a previous run that died between
	// creating a resource and cleaning it up: tags and registries are
	// server-wide and shared by every session in this run, so nothing but
	// the e2e- prefix distinguishes an earlier run's leftovers from
	// anything a fresh run is about to create. It runs before any test, not
	// after, so a leftover from run N-1 never gets mistaken for something
	// run N leaked.
	if err := cleanupOrphans(context.Background(), estate); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: clean up orphaned fixtures: %v\n", err)
		os.Exit(1)
	}

	sessions, err = newSessions(estate)
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build sessions: %v\n", err)
		os.Exit(1)
	}

	// Printed before the run and again after it. A half estate is now a
	// routine shape — CI provisions the compose legs and the Kubernetes leg
	// in two separate jobs, one Business Edition licence between them — and
	// the suites for an absent leg skip. `go test` prints nothing at all for
	// a skipped test unless -v is passed, so without this line a run that
	// measured half of what it appears to would be indistinguishable, in its
	// own output, from one that measured everything. Twice, because the first
	// copy is thousands of lines above the verdict by the time anyone reads
	// the second.
	summary := legSummary(estate)
	fmt.Fprintf(os.Stderr, "e2e: %s\n", summary)
	code := m.Run()
	fmt.Fprintf(os.Stderr, "e2e: %s\n", summary)
	sessions.Close()
	os.Exit(code)
}

// legSummary describes, in one line, which legs an estate provisioned and
// which it did not — the header and footer TestMain prints around every run.
//
// It is a pure function of the estate so it can be pinned by
// TestUnit_LegSummary_NamesTheLegsThatAreAbsent without one, and it names the
// absent legs explicitly rather than only listing the present ones: "CE, EE"
// tells a reader what ran, but only "absent: Kubernetes" tells them what did
// not, which is the half that a green run otherwise hides.
func legSummary(e harness.Estate) string {
	present := make([]string, 0, 3)
	for _, leg := range e.Legs() {
		present = append(present, leg.Name)
	}
	absent := make([]string, 0, 3)
	for _, name := range []string{"CE", "EE", "Kubernetes"} {
		if !slices.Contains(present, name) {
			absent = append(absent, name)
		}
	}
	if len(absent) == 0 {
		return "estate provisions every leg (" + strings.Join(present, ", ") + "); nothing is skipped for want of one"
	}
	return "estate provisions " + strings.Join(present, ", ") +
		" — ABSENT: " + strings.Join(absent, ", ") +
		". Every suite needing an absent leg skips with a named reason; this run measures less than a full estate would."
}

// TestUnit_LegSummary_NamesTheLegsThatAreAbsent pins the one line a reader of
// a CI log has to be able to trust: on a half estate it must name what is
// missing, not merely list what is present.
//
// Both halves of the CI split are covered, plus the full local estate. The
// mutation this is written against is dropping the absent half of the
// sentence and reporting only what was provisioned — which reads perfectly
// well, is true as far as it goes, and hides exactly the thing the summary
// exists to expose.
func TestUnit_LegSummary_NamesTheLegsThatAreAbsent(t *testing.T) {
	t.Parallel()

	ce := harness.Server{Edition: "CE", BaseURL: "http://ce", Creds: harness.Credentials{APIKey: "ce-key"}}
	ee := harness.Server{Edition: "EE", BaseURL: "http://ee", Creds: harness.Credentials{APIKey: "ee-key"}}
	k8s := harness.Server{Edition: "Kubernetes", BaseURL: "https://k8s", Creds: harness.Credentials{APIKey: "k8s-key"}}

	tests := []struct {
		name        string
		estate      harness.Estate
		wantPresent []string
		wantAbsent  []string
	}{
		{
			name:        "the compose CI job: no Kubernetes leg",
			estate:      harness.Estate{CE: ce, EE: ee},
			wantPresent: []string{"CE", "EE"},
			wantAbsent:  []string{"Kubernetes"},
		},
		{
			name:        "the Kubernetes CI job: no compose leg at all",
			estate:      harness.Estate{Kubernetes: k8s},
			wantPresent: []string{"Kubernetes"},
			wantAbsent:  []string{"CE", "EE"},
		},
		{
			name:        "a contributor with no licence",
			estate:      harness.Estate{CE: ce},
			wantPresent: []string{"CE"},
			wantAbsent:  []string{"EE", "Kubernetes"},
		},
		{
			name:        "the full local estate: nothing to warn about",
			estate:      harness.Estate{CE: ce, EE: ee, Kubernetes: k8s},
			wantPresent: []string{"CE", "EE", "Kubernetes"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := legSummary(tc.estate)
			for _, leg := range tc.wantPresent {
				if !strings.Contains(got, leg) {
					t.Errorf("summary %q does not name the provisioned leg %q", got, leg)
				}
			}
			if len(tc.wantAbsent) == 0 {
				if strings.Contains(got, "ABSENT") {
					t.Errorf("summary %q warns about an absent leg on a fully provisioned estate", got)
				}
				return
			}
			_, warning, ok := strings.Cut(got, "ABSENT:")
			if !ok {
				t.Fatalf("summary %q never says which legs are absent; want it to name %v", got, tc.wantAbsent)
			}
			for _, leg := range tc.wantAbsent {
				if !strings.Contains(warning, leg) {
					t.Errorf("summary %q does not name %q among the absent legs", got, leg)
				}
			}
		})
	}
}
