// Command audit_e2e_gaps reports which catalog actions no end-to-end test
// exercises.
//
// The catalog holds 18 actions across three pilot domains today; P3 grows it
// to 441 across 44 domains, and this is how anyone will know what that growth
// left untested. It reports and exits 0 rather than gating CI: with a small
// fraction of the catalog written, a hard gate would fail on almost the whole
// catalog and teach everyone to ignore it. It gains a threshold in P7, once
// the catalog is complete. It still exits non-zero on a genuine error, such
// as the catalog failing to build or the suite directory being unreadable —
// only "N actions are unexercised" is not treated as failure.
//
// It never writes to standard output: that stream carries the MCP JSON-RPC
// transport for the real server binary, and this repository's CI guard bans
// writes to it (and bare fmt.Print* calls) module-wide, this command
// included. The report goes to stderr, following cmd/fetch_spec's precedent.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// suiteDir is the e2e suite tree the scan reads. Tasks 6-8 of this plan
// create it; until then it does not exist, and that is an expected state
// (see scanSuiteDir), not an error.
const suiteDir = "test/e2e/suite"

func main() {
	if err := run(os.Stderr, suiteDir); err != nil {
		fmt.Fprintf(os.Stderr, "audit_e2e_gaps: %v\n", err)
		os.Exit(1)
	}
}

// run builds the catalog, scans dir for references, and writes the report to
// w. dir is a parameter (rather than always reading the suiteDir constant)
// so tests can point it at a temporary directory without changing the
// process's working directory.
func run(w io.Writer, dir string) error {
	// Business Edition, no version ceiling: the audit must account for every
	// action that could ever be registered, including EE-gated ones, not just
	// what one particular target server would see today.
	catalog, err := actioncatalog.Build(allSpecs(), actioncatalog.Options{Edition: edition.EE})
	if err != nil {
		return fmt.Errorf("build action catalog: %w", err)
	}

	names := make([]string, 0, len(catalog.Actions()))
	for _, spec := range catalog.Actions() {
		names = append(names, spec.Name)
	}

	found, err := scanSuiteDir(dir)
	if err != nil {
		return fmt.Errorf("scan %s: %w", dir, err)
	}

	_, err = fmt.Fprint(w, buildReport(names, found))
	if err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}

// allSpecs collects every domain's declared actions. Each pilot domain
// package is added by hand today; P3 grows this list alongside the domain
// packages themselves.
func allSpecs() []toolutil.ActionSpec {
	var specs []toolutil.ActionSpec
	specs = append(specs, system.Specs()...)
	specs = append(specs, tags.Specs()...)
	specs = append(specs, registries.Specs()...)
	return specs
}

// scanSuiteDir reads every .go file under dir and merges what scanReferences
// finds in each.
//
// An absent directory is reported as "found nothing", not an error: Tasks 6-8
// of this plan have not created test/e2e/suite yet, and that is the state
// this tool ships in. Any other failure to stat or walk the tree is a genuine
// error and is returned as one.
func scanSuiteDir(dir string) (map[string]bool, error) {
	found := map[string]bool{}

	// os.Root confines every subsequent read to this directory tree, so a
	// symlink inside it cannot be walked or read outside of dir (gosec G122).
	root, err := os.OpenRoot(dir)
	if errors.Is(err, os.ErrNotExist) {
		return found, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dir, err)
	}
	defer func() { _ = root.Close() }()

	fsys := root.FS()
	walkErr := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk %s: %w", path, err)
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		for name := range scanReferences(data) {
			found[name] = true
		}
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("scan %s: %w", dir, walkErr)
	}
	return found, nil
}

// The three shapes a test file can name an action, one regexp each:
//
//   - dynamicLiteralRE: the canonical "domain.action" literal the dynamic
//     surface's find/execute pair takes as an argument.
//   - individualLiteralRE: the rendered tool name the individual surface
//     registers, "portainer_domain_action". Rendering is not invertible in
//     general — RenderToolName maps "." to "_", so a name itself containing
//     "_" collides with the domain/action boundary (this is exactly the
//     collision actioncatalog.Build checks for). scanReferences does not
//     guess which split is right; it records every split as a candidate.
//     That only ever adds extra, unused map keys: main looks up real catalog
//     action names, so a candidate that is not one is simply never read.
//   - metaLiteralRE / actionFieldRE: the meta surface's tool name,
//     "portainer_domain", paired with an "action": "name" argument a few
//     lines away. The action name here is explicit, so no split-guessing is
//     needed.
var (
	dynamicLiteralRE    = regexp.MustCompile(`"([A-Za-z][A-Za-z0-9_]*\.[A-Za-z][A-Za-z0-9_.]*)"`)
	individualLiteralRE = regexp.MustCompile(`"portainer_([A-Za-z0-9_]+)"`)
	actionFieldRE       = regexp.MustCompile(`"action"\s*:\s*"([A-Za-z0-9_.]+)"`)
)

// nearbyLines bounds how far apart a meta tool name and its "action" argument
// may be and still be read as the same call. A handful of lines covers a
// struct literal spanning a CallTool invocation without also pairing a tool
// name with an unrelated action mentioned much later in the same file.
const nearbyLines = 5

// scanReferences finds every action name a piece of Go source references,
// across all three surface styles, and returns them as a set. Comments are
// stripped before matching, so a name mentioned only in prose does not count
// as a reference.
func scanReferences(source []byte) map[string]bool {
	stripped := stripComments(source)
	found := map[string]bool{}

	for _, m := range dynamicLiteralRE.FindAllSubmatch(stripped, -1) {
		found[string(m[1])] = true
	}

	individualMatches := individualLiteralRE.FindAllSubmatchIndex(stripped, -1)
	for _, idx := range individualMatches {
		rest := string(stripped[idx[2]:idx[3]])
		segments := strings.Split(rest, "_")
		for split := 1; split < len(segments); split++ {
			domain := strings.Join(segments[:split], "_")
			action := strings.Join(segments[split:], "_")
			found[domain+"."+action] = true
		}
	}

	actionMatches := actionFieldRE.FindAllSubmatchIndex(stripped, -1)
	lineOf := func(pos int) int { return bytes.Count(stripped[:pos], []byte("\n")) }
	for _, pIdx := range individualMatches {
		domain := string(stripped[pIdx[2]:pIdx[3]])
		pLine := lineOf(pIdx[0])
		for _, aIdx := range actionMatches {
			if absDiff(pLine, lineOf(aIdx[0])) <= nearbyLines {
				action := string(stripped[aIdx[2]:aIdx[3]])
				found[domain+"."+action] = true
			}
		}
	}

	return found
}

func absDiff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// stripComments removes // line comments and /* */ block comments from Go
// source, leaving string, rune and raw-string literals untouched so that a
// "//" or "/*" inside a literal (a URL, for instance) is never mistaken for a
// comment start.
func stripComments(src []byte) []byte {
	out := make([]byte, 0, len(src))
	n := len(src)
	for i := 0; i < n; {
		switch {
		case src[i] == '/' && i+1 < n && src[i+1] == '/':
			for i < n && src[i] != '\n' {
				i++
			}
		case src[i] == '/' && i+1 < n && src[i+1] == '*':
			i += 2
			for i+1 < n && (src[i] != '*' || src[i+1] != '/') {
				i++
			}
			i += 2
			if i > n {
				i = n
			}
		case src[i] == '"' || src[i] == '\'':
			quote := src[i]
			out = append(out, src[i])
			i++
			for i < n && src[i] != quote {
				if src[i] == '\\' && i+1 < n {
					out = append(out, src[i], src[i+1])
					i += 2
					continue
				}
				out = append(out, src[i])
				i++
			}
			if i < n {
				out = append(out, src[i])
				i++
			}
		case src[i] == '`':
			out = append(out, src[i])
			i++
			for i < n && src[i] != '`' {
				out = append(out, src[i])
				i++
			}
			if i < n {
				out = append(out, src[i])
				i++
			}
		default:
			out = append(out, src[i])
			i++
		}
	}
	return out
}

// buildReport renders a human-readable summary of catalog coverage. It
// always states the exercised and unexercised counts in an unambiguous
// "N of M" form, never just a bare number lost among unrelated digits,
// because silent truncation reads as full coverage.
func buildReport(names []string, found map[string]bool) string {
	sorted := append([]string(nil), names...)
	sort.Strings(sorted)

	var unexercised []string
	for _, name := range sorted {
		if !found[name] {
			unexercised = append(unexercised, name)
		}
	}

	var b strings.Builder
	fmt.Fprintln(&b, "Portainer MCP e2e coverage audit")
	fmt.Fprintln(&b, "================================")
	fmt.Fprintf(&b, "Catalog actions:   %d\n", len(sorted))
	fmt.Fprintf(&b, "Exercised:         %d of %d\n", len(sorted)-len(unexercised), len(sorted))
	fmt.Fprintf(&b, "Unexercised:       %d of %d\n", len(unexercised), len(sorted))

	if len(unexercised) == 0 {
		fmt.Fprintln(&b, "\nEvery catalog action is referenced by the e2e suite.")
		return b.String()
	}

	fmt.Fprintln(&b, "\nActions with no e2e reference:")
	for _, name := range unexercised {
		fmt.Fprintf(&b, "  - %s\n", name)
	}
	return b.String()
}
