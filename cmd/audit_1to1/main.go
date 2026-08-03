// Command audit_1to1 fails when any operation documented in either vendored
// Portainer specification has no matching catalog action.
//
// This is the mechanism the whole P3 rewrite exists to install, not a report:
// the previous incarnation of this server reached 37 % API coverage with
// nothing checking it, because nothing failed when coverage decayed. Every
// operation in api/specs/ce-2.44.0.json and api/specs/ee-2.44.0.json needs
// either a catalog action naming its OperationID or a dated, reasoned entry
// in api/coverage-allowlist.yaml; anything else is reported by name and
// fails the run.
//
// With 18 of 441 Business-Edition actions declared, this fails today and
// keeps failing for most of P3 — that is correct, not a bug to work around.
// It is wired into CI as a separate, non-required job (see
// .github/workflows/ci.yml) that reports the count on every pull request; it
// becomes a required, blocking gate only once the count reaches zero. A gate
// that blocks every commit for months teaches everyone to route around it
// instead.
//
// The allow-list is the audit's honesty mechanism, not a hiding place: an
// entry excludes its operation from the failure but never from the report
// (see buildReport), and an entry naming an operation absent from both specs
// is itself a failure — a stale entry would otherwise keep forgiving
// something that no longer exists, letting a real, unrelated gap inherit its
// name.
//
// It never writes to standard output: that stream carries the MCP JSON-RPC
// transport for the real server binary, and this repository's CI guard bans
// writes to it module-wide, this command included. The report goes to
// standard error, following cmd/fetch_spec and cmd/audit_e2e_gaps's
// precedent.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Default locations of this audit's three inputs. Parameterised on run
// rather than hardcoded there, so tests can point every one of them at a
// temporary fixture without touching the working directory.
const (
	specsDir      = "api/specs"
	ceSpecFile    = "ce-2.44.0.json"
	eeSpecFile    = "ee-2.44.0.json"
	allowListDir  = "api"
	allowListFile = "coverage-allowlist.yaml"
)

func main() {
	if err := run(os.Stderr, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile); err != nil {
		fmt.Fprintf(os.Stderr, "audit_1to1: %v\n", err)
		os.Exit(1)
	}
}

// run loads both vendored specs and the allow-list, audits the real catalog
// against them, and writes the report to w. It returns an error whenever the
// build must fail: a malformed input, an allow-list or catalog entry naming
// an operation that resolves in neither spec, or any operation left
// uncovered.
func run(w io.Writer, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile string) error {
	ceData, err := readFileIn(specsDir, ceSpecFile)
	if err != nil {
		return err
	}
	eeData, err := readFileIn(specsDir, eeSpecFile)
	if err != nil {
		return err
	}
	allowData, err := readFileIn(allowListDir, allowListFile)
	if err != nil {
		return err
	}

	ceOps, err := parseSpecOperations(ceData)
	if err != nil {
		return fmt.Errorf("%s/%s: %w", specsDir, ceSpecFile, err)
	}
	eeOps, err := parseSpecOperations(eeData)
	if err != nil {
		return fmt.Errorf("%s/%s: %w", specsDir, eeSpecFile, err)
	}
	allowList, err := parseAllowList(allowData)
	if err != nil {
		return fmt.Errorf("%s/%s: %w", allowListDir, allowListFile, err)
	}

	result, err := auditCoverage(ceOps, eeOps, allCatalogSpecs(), allowList)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, buildReport(result)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if result.HasGap() {
		return fmt.Errorf("%d operation(s) in the vendored specs have no catalog action",
			len(result.CE.Uncovered)+len(result.EE.Uncovered))
	}
	return nil
}

// allCatalogSpecs collects every domain's declared actions. Each pilot
// domain package is added by hand today, matching cmd/audit_e2e_gaps's
// allSpecs; P3 grows this list alongside the domain packages themselves.
func allCatalogSpecs() []toolutil.ActionSpec {
	var specs []toolutil.ActionSpec
	specs = append(specs, system.Specs()...)
	specs = append(specs, tags.Specs()...)
	specs = append(specs, registries.Specs()...)
	return specs
}

// readFileIn reads name from dir, refusing to read anything name resolves
// outside of it. This mirrors cmd/gen_applicability's operationsIn
// confinement: dir and name are both fixed constants in production, but the
// same helper is exercised by tests with directories under t.TempDir(), so
// the guard is kept rather than assumed unnecessary because production
// inputs happen to be literals today.
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
