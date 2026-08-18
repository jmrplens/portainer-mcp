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
// With 18 of 441 Business-Edition actions declared, plain run (what `make
// audit-1to1` calls, and what a human asking "are we done" wants) fails
// today and keeps failing for most of P3 — that is correct, not a bug to
// work around.
//
// The CI job (see .github/workflows/ci.yml) does not call plain run, though:
// it runs with -ratchet, which compares coverage against the number
// committed in api/coverage-baseline.yaml instead of requiring 100%. That
// ratchet is what lets this be a real, required CI gate from day one rather
// than a job stuck failing for months (which teaches everyone to route
// around it) or one that never fails at all (which would let coverage decay
// silently behind a clean check) — see runRatchet's own doc comment for the
// mechanics. `make audit-1to1` itself is untouched: it still fails on any
// uncovered operation, ratchet or not.
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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/tools/custom_templates"
	"github.com/jmrplens/portainer-mcp/internal/tools/docker"
	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Default locations of this audit's inputs. Parameterised on run rather than
// hardcoded there, so tests can point every one of them at a temporary
// fixture without touching the working directory.
const (
	specsDir       = "api/specs"
	defaultSpecVer = "2.44.0"
	allowListDir   = "api"
	allowListFile  = "coverage-allowlist.yaml"
	baselineDir    = "api"
	baselineFile   = "coverage-baseline.yaml"
)

func main() {
	specVersion := flag.String("spec-version", defaultSpecVer,
		"vendored Portainer spec version to audit (must match a ce-<version>.json/ee-<version>.json pair under api/specs)")
	baseline := flag.Bool("ratchet", false,
		"pass/fail by comparing coverage against the committed ratchet baseline (api/coverage-baseline.yaml) instead of requiring full coverage")
	flag.Parse()

	ceSpecFile := fmt.Sprintf("ce-%s.json", *specVersion)
	eeSpecFile := fmt.Sprintf("ee-%s.json", *specVersion)

	var err error
	if *baseline {
		err = runRatchet(os.Stderr, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile, baselineDir, baselineFile)
	} else {
		err = run(os.Stderr, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile)
	}
	if err != nil {
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

// runRatchet is run's sibling for the CI gate: it loads and audits exactly
// the same inputs, and prints exactly the same coverage report, but decides
// pass/fail differently.
//
// A gate that requires 100% coverage from day one — what run's HasGap decides
// — is correct for a human asking "are we done", but wrong for CI at the
// start of P3: with 659 of 692 combined operations uncovered, that gate would
// fail every pull request for months. A red check nobody expects to turn
// green for months is a check everyone learns to ignore, and worse, a red
// check that never changes state hides a *new* regression in the same job:
// nobody re-reads a report whose bottom line has said "FAIL" every day since
// P3 started. But a gate that never fails at all is just as useless — it
// would let coverage decay silently behind a permanently clean job.
//
// The ratchet is the third option. api/coverage-baseline.yaml commits the
// coverage this project has already reached; this gate fails only when
// current coverage drops below that committed floor (a real regression),
// passes when it matches, and passes — while saying so plainly — when
// coverage has improved beyond what the file records (the file is stale and
// should be updated in the same commit that improved it, which is also what
// makes the improvement visible in the diff). It cannot go backwards, it is
// green today, and it tightens automatically as P3 lands each domain.
func runRatchet(w io.Writer, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile, baselineDir, baselineFile string) error {
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
	baselineData, err := readFileIn(baselineDir, baselineFile)
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
	base, err := parseBaseline(baselineData)
	if err != nil {
		return fmt.Errorf("%s/%s: %w", baselineDir, baselineFile, err)
	}

	result, err := auditCoverage(ceOps, eeOps, allCatalogSpecs(), allowList)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, buildReport(result)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	outcomes := checkRatchet(result, base)
	if _, err := fmt.Fprint(w, renderRatchet(outcomes)); err != nil {
		return fmt.Errorf("write ratchet report: %w", err)
	}
	if ratchetRegressed(outcomes) {
		return fmt.Errorf("coverage ratchet: regressed below the baseline committed in %s/%s", baselineDir, baselineFile)
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
	specs = append(specs, docker.Specs()...)
	specs = append(specs, custom_templates.Specs()...)
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
