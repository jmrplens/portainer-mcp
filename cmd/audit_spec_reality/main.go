// Command audit_spec_reality probes a live Portainer server for every
// operation the vendored specification documents, and reports which of
// those operations the server does not actually serve.
//
// Portainer's specification has already been wrong about Portainer four
// times in this project's own history — a required header the spec never
// mentions, a feature the spec claims a deployment target gets for free and
// does not, a documented mutation that silently does not persist, and a
// field typed wrong for what it plainly holds — and all four were found by
// accident. This command exists so the next one is found on purpose, and so
// every one of P3's 46 domain waves checks for it the same way instead of
// however that wave happens to remember to.
//
// # The mechanism
//
// Portainer answers an absent *route* with Go's own default mux fallback: a
// plain-text "404 page not found", regardless of credentials. It answers an
// absent *resource* — or any other outcome a matched route's handler chain
// produces, including an authentication failure — with its own JSON
// {"message","details"} body (or a success). Those two are always
// distinguishable, which is what makes "does the specification's route
// exist on this server" machine-checkable at all: see isRouteAbsent.
//
// # Why every probe here is safe
//
// This audit sends every request — GET, POST, PUT, PATCH, DELETE alike,
// 441 documented operations including deletes, restores, backups and the
// Community-to-Business upgrade — with a credential that is not, and will
// never be, valid for any Portainer (see probeSentinelValue). Measured directly
// against this project's own live estate before this command was written:
// an invalid credential is rejected by Portainer's own auth check before any
// handler's business logic runs, on every verb, including POST
// /system/upgrade and POST /restore (see task-5-report.md for the full
// transcript). A route that does not exist never reaches that check at all
// — the request never resolves to a handler in the first place — so the
// credential being wrong changes nothing about whether route-absence is
// detected; it only guarantees that a route which *does* exist can never
// act on the request. No operation is skipped as "cannot be made safe":
// this technique covers all of them uniformly. auditLeg additionally proves
// the mechanism against a route it manufactures and knows is absent
// (selftestPath) before trusting anything it reports about a real one.
//
// # What this does not check
//
// A route existing is not the same as a route behaving as documented — the
// registries.configure non-persistence and the missing Kubernetes
// auto-registration mentioned above are both real, both already known, and
// neither is a route-existence divergence this command's mechanism can see.
// It also does not probe the Kubernetes leg: see task-5-report.md for why
// that leg's route table is not expected to differ from the Community and
// Business Edition legs already probed.
//
// # Report, never gate
//
// This never fails the build over what it finds: a divergence is a fact
// about the Portainer server under test, not a defect in this project's
// code, and audit_1to1 is the audit that gates. This exits non-zero only
// when it cannot run at all — no estate, an unreadable spec, or a failed
// self-test — never because it found a divergence.
//
// It never writes to standard output: that stream carries the MCP JSON-RPC
// transport for the real server binary, and this repository's CI guard bans
// writes to it module-wide. The report goes to standard error, following
// cmd/audit_1to1 and cmd/audit_e2e_gaps's precedent.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// Default locations of this audit's inputs.
const (
	specsDir       = "api/specs"
	defaultSpecVer = "2.44.0"
	defaultEstate  = "test/e2e/.estate.json"
	defaultTimeout = 15 * time.Second
)

func main() {
	specVersion := flag.String("spec-version", defaultSpecVer,
		"vendored Portainer spec version to audit (must match a ce-<version>.json/ee-<version>.json pair under api/specs)")
	estatePath := flag.String("estate", estateDefault(),
		"path to the e2e estate file written by test/e2e/scripts/up.sh (default: $"+harness.EstateFileEnv+", or "+defaultEstate)
	timeout := flag.Duration("timeout", defaultTimeout, "per-probe HTTP timeout")
	flag.Parse()

	if err := run(os.Stderr, *estatePath, specsDir, *specVersion, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "audit_spec_reality: %v\n", err)
		os.Exit(1)
	}
}

// estateDefault resolves the estate flag's default the same way
// test/e2e/suite's TestMain does: prefer the environment variable the
// harness scripts set, falling back to the path up.sh writes by default when
// nothing overrides it.
func estateDefault() string {
	if v := os.Getenv(harness.EstateFileEnv); v != "" {
		return v
	}
	return defaultEstate
}

// run loads the estate and the vendored specs, audits every leg the estate
// actually provisioned, and writes the report to w.
//
// It returns an error only for what "cannot run at all" means: an unreadable
// estate or spec file, or a leg whose self-test failed (see auditLeg). It
// never returns an error for a divergence found — see this package's doc
// comment for why reporting one is not a gate.
func run(w io.Writer, estatePath, specsDir, specVersion string, timeout time.Duration) error {
	estate, err := harness.LoadEstate(estatePath)
	if err != nil {
		return fmt.Errorf("load estate: %w", err)
	}

	ctx := context.Background()
	var results []legResult

	ceOps, err := loadSpecOperations(specsDir, fmt.Sprintf("ce-%s.json", specVersion))
	if err != nil {
		return err
	}
	ceResult, err := auditLeg(ctx, "CE", apiBaseURL(estate.CE.BaseURL), ceOps, timeout)
	if err != nil {
		return fmt.Errorf("audit CE leg: %w", err)
	}
	results = append(results, ceResult)

	if estate.HasBusinessEdition() {
		eeOps, err := loadSpecOperations(specsDir, fmt.Sprintf("ee-%s.json", specVersion))
		if err != nil {
			return err
		}
		eeResult, err := auditLeg(ctx, "EE", apiBaseURL(estate.EE.BaseURL), eeOps, timeout)
		if err != nil {
			return fmt.Errorf("audit EE leg: %w", err)
		}
		results = append(results, eeResult)
	} else {
		if _, err := fmt.Fprintln(w, "no Business Edition leg in this estate (no licence provisioned): EE operations not probed this run."); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
	}

	if _, err := fmt.Fprint(w, buildReport(results)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// apiBaseURL trims a trailing slash and appends the API root, mirroring
// internal/portainer.New's identical normalisation of the same estate field.
func apiBaseURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/api"
}

// loadSpecOperations reads and parses one vendored spec file from specsDir.
func loadSpecOperations(specsDir, specFile string) (map[string]specOperation, error) {
	data, err := readFileIn(specsDir, specFile)
	if err != nil {
		return nil, err
	}
	ops, err := parseSpecOperations(data)
	if err != nil {
		return nil, fmt.Errorf("%s/%s: %w", specsDir, specFile, err)
	}
	return ops, nil
}

// readFileIn reads name from dir, refusing to read anything name resolves
// outside of it. Mirrors cmd/audit_1to1's helper of the same name and
// purpose: dir and name are fixed constants/flags in production, but tests
// exercise this with a t.TempDir(), so the confinement guard is kept rather
// than assumed unnecessary.
func readFileIn(dir, name string) ([]byte, error) {
	path := filepath.Clean(filepath.Join(dir, name))
	if rel, err := filepath.Rel(dir, path); err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("refuse to read %s: escapes %s", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}
