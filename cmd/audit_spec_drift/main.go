// Command audit_spec_drift fails when a declared catalog action's parameter
// shape no longer matches the vendored OpenAPI operation it was generated
// from.
//
// # Why this exists
//
// The owner decided to generate the action catalog once and then hand-
// maintain it, rather than regenerating forever. That is only safe if
// something notices when hand-edited code stops matching the specification
// it came from. Measured across twelve published Portainer specifications:
// between minor releases, 77 to 105 operations change their parameter shape
// while keeping their operationId — and every one of those passes every
// other audit in this repository silently. cmd/audit_1to1 compares by
// operationId alone, so a renamed field, a widened type, or a dropped
// "required" is invisible to it. cmd/audit_spec_reality probes whether a
// route exists at all, never what it accepts. This audit is what the freeze
// this project's next phase installs is actually defensible against: it is
// the net that would have caught internal/specdiff's own worked example
// (EndpointList's "order" and "edgeStackStatus" query parameters silently
// swapping type between 2.42.0 and 2.43.0) had it happened to a field this
// project already published.
//
// It uses internal/specdiff's one comparison engine (ShapeFromSpec,
// ShapeFromCatalog, Compare) rather than a second, parallel comparison —
// see that package's doc comment for why deriving the same fact twice is
// exactly the defect this project keeps re-discovering.
//
// # It gates
//
// Unlike cmd/audit_spec_reality and cmd/audit_discovery, drift against the
// vendored specification is a defect in this project's own code, not a fact
// about Portainer: this fails the build. It starts clean — every action
// today was generated from the vendored specification cmd/gen_action_inputs
// reads — so unlike cmd/audit_1to1 there is no case for a ratchet here: a
// ratchet exists to let a real, en-route gate turn on before the target is
// met, and the target is already met on day one.
//
// # Descriptions gate conditionally, not absolutely
//
// ChangeDescription is specdiff's own kind for "only the field's prose
// changed", documented there as unable, by itself, to make a caller send the
// wrong thing. Measured directly against the vendored Business Edition
// specification: 341/341 path parameters and 237/237 query parameters carry
// a description, but only 485 of 771 body properties do — roughly 286 body
// fields the specification itself never describes. A field the
// specification never described cannot have drifted away from a description
// that never existed, so gating on every ChangeDescription finding
// unconditionally would fail the build for any one of those 286 fields the
// moment a hand-written action added a helpful description the spec never
// had — an improvement, not a defect. This audit gates a ChangeDescription
// finding only when the specification's own side is non-empty: see
// isGating's doc comment for the full reasoning, and the pilots' own
// verbatim-republication of the spec's text (cmd/gen_action_inputs's
// fieldTag) for why that case does not fire against the catalog as
// generated today.
//
// # The canary is mandatory
//
// Every run first proves specdiff.Compare can still tell two different
// shapes apart, using a shape this command invents and deliberately breaks
// itself — never one read from a vendored spec or the real catalog, so it
// can fail independently of whatever this run's real inputs happen to
// contain. See verifyCanary's doc comment; this mirrors
// cmd/audit_spec_reality's selftestPath precedent, because this repository
// has already shipped seventeen checks that could not discriminate, every
// one because its assertion matched something its own fixture supplied.
//
// # Deliberate divergence
//
// Some actions legitimately differ from the specification: the hand-written
// pilot actions that resisted generation take a deliberately narrower type
// for a field or two, on purpose. api/spec-drift-allowlist.yaml excuses one
// (operationId, field) pair at a time, each entry dated and reasoned — never
// a whole operation, so an unrelated field on the same operation that
// starts drifting later is not silently forgiven by an entry that was never
// about it. An entry that excuses nothing a real run currently finds gating
// is itself a build error: see auditResult.StaleEntries's doc comment for
// why a stale exemption is exactly as dangerous as a missing one.
//
// # Credential redaction is checked here too, unconditionally
//
// cmd/gen_action_inputs refuses to emit a bare generated handler for an
// operation whose success response can carry a credential-shaped field (85
// operations across 17 domains, measured against the vendored Business
// Edition specification), requiring a domain-supplied redact<OperationID>
// wrapper instead. After the freeze the generator no longer runs against an
// owned domain, so that refusal alone can no longer stand between a hand
// edit and a leaked credential — P2 shipped exactly that bug once, in three
// hand-written registries handlers, and no fixture carried a password so no
// test caught it. auditCredentialRedaction (credential_audit.go) is that
// refusal, moved here: for every action whose vendored response is
// credential-shaped, it statically confirms the declared Handler's own body
// calls the required wrapper (see wrapper.go's handlerRedactsCredential, and
// credential.go for how "credential-shaped" is resolved from the spec
// without depending on cmd/gen_action_inputs's internals). Unlike a
// parameter-shape finding, a credential-redaction finding is never
// allow-listable: there is no legitimate reason for a real credential to
// reach a model. It has its own canary (verifyCredentialCanary), run
// alongside verifyCanary, for the identical reason: this mechanism is new
// with this task and has shipped zero times, which is exactly when a canary
// is needed, not after.
//
// # Never writes to standard output
//
// That stream carries the MCP JSON-RPC transport for the real server
// binary, and this repository's CI guard bans writes to it module-wide,
// this command included. The report goes to standard error, following
// cmd/audit_1to1 and cmd/audit_spec_reality's identical precedent.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

// Default locations of this audit's inputs, mirroring cmd/audit_1to1's
// identical constants.
const (
	specsDir       = "api/specs"
	defaultSpecVer = "2.44.0"
	allowListDir   = "api"
	allowListFile  = "spec-drift-allowlist.yaml"
)

func main() {
	specVersion := flag.String("spec-version", defaultSpecVer,
		"vendored Portainer spec version to audit (must match a ce-<version>.json/ee-<version>.json pair under api/specs)")
	flag.Parse()

	ceSpecFile := fmt.Sprintf("ce-%s.json", *specVersion)
	eeSpecFile := fmt.Sprintf("ee-%s.json", *specVersion)

	if err := run(os.Stderr, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile); err != nil {
		fmt.Fprintf(os.Stderr, "audit_spec_drift: %v\n", err)
		os.Exit(1)
	}
}

// run runs the mandatory canary, loads both vendored specs, the allow-list
// and the real catalog, audits every declared action's parameter shape
// against the vendored operation it was generated from, and writes the
// report to w.
//
// It returns an error whenever the build must fail: the canary could not
// confirm the comparison engine still discriminates (in which case nothing
// else is even attempted), a malformed input, an action whose OperationID
// resolves in neither vendored spec, an un-excused gating finding, or a
// stale allow-list entry.
func run(w io.Writer, specsDir, ceSpecFile, eeSpecFile, allowListDir, allowListFile string) error {
	if err := verifyCanary(specdiff.Compare); err != nil {
		return fmt.Errorf("refusing to report: %w", err)
	}
	if err := verifyCredentialCanary(); err != nil {
		return fmt.Errorf("refusing to report: %w", err)
	}
	if err := verifyMinimumCanary(); err != nil {
		return fmt.Errorf("refusing to report: %w", err)
	}

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

	actions := wiring.AllSpecs()

	result, err := auditDrift(eeOps, ceOps, actions, allowList)
	if err != nil {
		return err
	}
	credResult, err := auditCredentialRedaction(eeOps, ceOps, actions)
	if err != nil {
		return err
	}
	minResult, err := auditIdentifierMinimum(eeOps, ceOps, actions)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprint(w, buildReport(result, credResult, minResult)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if result.HasDrift() || credResult.HasLeaks() || minResult.HasGaps() {
		return fmt.Errorf(
			"%d gating finding(s), %d stale allow-list entr(y/ies), %d credential-redaction finding(s), %d identifier-minimum finding(s): the catalog has drifted from the vendored specification",
			result.GatingCount, len(result.StaleEntries), len(credResult.Findings), len(minResult.Findings))
	}
	return nil
}

// readFileIn reads name from dir, refusing to read anything name resolves
// outside of it, mirroring cmd/audit_1to1's identical helper.
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
