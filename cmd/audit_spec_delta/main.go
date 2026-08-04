// Command audit_spec_delta reports what changed between two vendored (or
// candidate) OpenAPI documents, as a work list grouped by domain — the tool
// a Portainer upgrade starts with, in the owner's own words: "a diff that
// tells you what to modify or add".
//
// # Why this exists
//
// cmd/audit_spec_drift answers "does the catalog still match the
// specification it was generated from" and fails the build when it does
// not. This command answers a different question a drift audit cannot: "the
// vendor published a newer specification — what do I need to touch to catch
// up". It never gates: a candidate version has not been adopted yet, so
// there is nothing about it that can fail this project's own build. It only
// reports, to standard error, following cmd/audit_1to1 and
// cmd/audit_spec_drift's identical precedent for why never standard output.
//
// It reuses internal/specdiff's one comparison engine (ShapeFromSpec,
// Compare) exactly as cmd/audit_spec_drift does, rather than a second,
// parallel comparison — see that package's doc comment for why deriving the
// same fact twice is the defect this project keeps re-discovering.
//
// # Scope: parameter shape, not the whole operation
//
// internal/specdiff.OperationShape is deliberately flat (no response body,
// no schema nested inside a body property — see that type's own doc
// comment), because that is what a generated Input struct and a published
// JSON Schema can even see. This command inherits that scope rather than
// widening it, which means its "changed" count is narrower than a full
// operation-node diff would report: response-only changes and changes
// nested inside a body property's own nested schema are real differences
// between two spec versions, but neither one requires touching this
// project's generated input code, so neither is counted here.
//
// This was not assumed — it was checked, and the two independent numbers
// disagreed enough that the disagreement itself had to be run down (see this
// task's own report for the full trace, and OperationShape's own doc
// comment for why Title/Description were added to it after the first pass
// through this exercise). plan/research/version-delta-analysis.md measured
// the real 2.43.0 -> 2.44.0 Business Edition pair with a full operation-node
// diff: 20 added, 5 removed, 84 "changed, same operationId", split by impact
// into 12 "alters generated input struct" and 26 total "alters input JSON
// schema" (the 12 plus 14 more that are description/enum text only). Run
// against the same two documents (2.43.0 bundled fresh with
// plan/research/specs/bundle.py into a scratch path, 2.44.0 the vendored
// api/specs/ee-2.44.0.json), this command reports 20 added and 5 removed —
// exact agreement — and 19 changed operations, 11 of them touching the
// generated input struct: closer than its first pass (13 changed, before
// OperationShape carried Title/Description) but still not 26 and 12.
//
// The added/removed agreement, plus a from-scratch reimplementation of the
// same top-level-field comparison (a second script, sharing no code with
// internal/specdiff, written solely to referee this disagreement) landing on
// the identical struct/cosmetic split at every stage, rules out a bug in
// this command's own counting. What explains the remaining gap is scope,
// not error. Checked directly against the same two documents: 7 of the 421
// operations present in both carry a changed operation-level summary or
// description with no parameter or body change at all (DeleteKubernetesNamespace,
// GitOpsSourceGet, HelmShow, both TeamMembership list/create/delete
// operations, and GitOpsWorkflowsList — the last of which was already
// counted, since its query parameters also changed). OperationShape's
// Title/Description (added specifically because of this finding — see that
// type's own doc comment) now catch exactly this: 6 of those 7 are brand new
// entries, moving the changed count 13 -> 19, all landing in ChangedCosmetic
// (a Title/Description edit is a copy-paste, never a Go struct change — see
// isStructKind). The remainder of the gap against 26 is nested-body-only and
// response-only change bleeding into the research script's coarser
// per-operation "changed" classification, exactly as this repository's own
// worked example already documents (plan/research/version-delta-analysis.md's
// own breakdown table) — neither one reaches a generated Input struct or its
// published parameter schema, which is what this tool exists to size. Both
// measurements are correct for what they count.
//
// # Judgement versus mechanics
//
// A newly added operation needs a person to name it, write its narrative,
// and decide whether its response needs a redaction wrapper — judgement, so
// every addition is tagged JUDGEMENT. A removed operation just needs
// deleting — MECHANICAL. A changed operation that touches the generated
// input struct (a field appeared, disappeared, changed type, changed
// requiredness, or moved between path/query/body) needs a person to decide
// how the Go code should change — JUDGEMENT. A changed operation whose only
// differences are description or enum text is a copy-paste from the new
// specification — MECHANICAL. See isStructKind's doc comment for the exact
// rule, and buildReport for how it renders.
//
// # Machine-readable output
//
// The -json flag switches the report from the human work list to a
// structured jsonReport encoding the identical grouping and ordering, so a
// future wave can drive a scaffolding pass directly from it rather than
// transcribing the human report by hand. The human report remains the
// default: see toJSONReport's doc comment for why it is a pure relabelling
// of the same deltaResult, never a second derivation of what belongs in
// scope.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func main() {
	before := flag.String("before", "", "path to the vendored (or older) OpenAPI document to diff from (required)")
	after := flag.String("after", "", "path to the candidate (or newer) OpenAPI document to diff to (required)")
	jsonOut := flag.Bool("json", false, "emit machine-readable JSON instead of the human work list")
	flag.Parse()

	if *before == "" || *after == "" {
		fmt.Fprintln(os.Stderr, "audit_spec_delta: -before and -after are both required")
		os.Exit(2)
	}

	if err := run(os.Stderr, *before, *after, *jsonOut); err != nil {
		fmt.Fprintf(os.Stderr, "audit_spec_delta: %v\n", err)
		os.Exit(1)
	}
}

// run reads before and after, computes their delta grouped by domain via
// toolutil.DomainTags, and writes either the human work list or its -json
// rendering to w.
//
// It returns an error only for a plumbing failure this command cannot run
// past — an unreadable file, a malformed document, an operation whose
// parameter shape specdiff cannot flatten, or an inconsistent DomainTags
// table. It never returns an error merely because before and after differ:
// this command reports a version that has not been adopted yet, so there is
// nothing about that difference for it to fail on. See this command's
// package doc for the full reasoning.
func run(w io.Writer, beforePath, afterPath string, jsonOut bool) error {
	beforeData, err := readSpecFile(beforePath)
	if err != nil {
		return err
	}
	afterData, err := readSpecFile(afterPath)
	if err != nil {
		return err
	}

	beforeOps, err := parseSpecOperations(beforeData)
	if err != nil {
		return fmt.Errorf("%s: %w", beforePath, err)
	}
	afterOps, err := parseSpecOperations(afterData)
	if err != nil {
		return fmt.Errorf("%s: %w", afterPath, err)
	}

	result, err := computeDelta(beforeOps, afterOps, toolutil.DomainTags)
	if err != nil {
		return err
	}

	if jsonOut {
		encoded, err := json.MarshalIndent(toJSONReport(beforePath, afterPath, result), "", "  ")
		if err != nil {
			return fmt.Errorf("encode JSON report: %w", err)
		}
		if _, err := fmt.Fprintln(w, string(encoded)); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		return nil
	}

	if _, err := fmt.Fprint(w, buildReport(beforePath, afterPath, result)); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// readSpecFile reads path, cleaning it first and refusing a relative path
// that escapes the current directory via "..". Unlike cmd/audit_1to1 and
// cmd/audit_spec_drift's identically-purposed readFileIn, -before and -after
// are not confined to one fixed directory: the whole point of this command
// (see its package doc) is comparing the vendored api/specs/ against a
// candidate bundled anywhere, typically a /tmp scratch path, so there is no
// single directory to confine reads to. This mirrors
// test/e2e/harness.LoadEstate's identical precedent for an arbitrary,
// flag-supplied file path: filepath.Clean plus a check against relative
// escape, not a confinement to a directory that would defeat this command's
// own purpose.
func readSpecFile(path string) ([]byte, error) {
	if path == "" {
		return nil, errors.New("spec path is empty")
	}
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) && strings.HasPrefix(cleaned, "..") {
		return nil, fmt.Errorf("refuse to read %q: escapes the current directory", path)
	}
	data, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cleaned, err)
	}
	return data, nil
}
