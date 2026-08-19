package harness

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// This file walks test/e2e/suite's own source and fails when a test reaches
// the estate's leg axis directly.
//
// The defect it exists to prevent has already happened once, and the way it
// happened is the argument for the file. `composeLegs(estate)` answers "which
// compose legs did this estate actually provision". Ranging over that in a
// test means an absent leg contributes no iteration, so no subtest is ever
// named, and the test finishes in the PASS column having executed nothing.
// On the Kubernetes-only estate CI's own second job provisions, that slice is
// empty: seventeen tests reported PASS in 0.00s with zero descendants,
// measured under `go test -json`, two of them with "Kubernetes" in their
// names. composeLegsUnderTest is the fix; this is what stops a twenty-third
// call site from being written in the old shape.
//
// Nothing else would catch it. There is no lint rule for it, `composeLegs`
// has to stay reachable because cleanupOrphans and newSessions legitimately
// call it, and the descendant-count technique that found the seventeen is a
// thing a person runs by hand. The protection was a doc comment, which is to
// say it relied on a future author reading it — the same class of gap as the
// original defect.
//
// It lives in package harness, not in the suite, deliberately: the suite is
// behind the `e2e` build tag, so a guard there would only ever run when an
// estate is already up, which is exactly when nobody needs telling. Here it
// runs under plain `make test` and gates CI, the same reason
// e2e_workflow_test.go sits beside it. go/parser reads a file's syntax
// without caring about its build constraints, so the tag is no obstacle.
//
// The shape is borrowed from internal/wiring/registration_parity_test.go,
// including the part worth borrowing: it refuses to pass on a shape it cannot
// read. A walk that finds nothing is a broken walk, not a clean repository,
// so every anchor it depends on — the directory, the declaration, the known
// good call sites — is asserted present before any conclusion is drawn.

// legAxisAllowed maps a function that may call composeLegs to the reason it
// may. Every entry has to be a function that does NOT turn the result into
// subtests, or the function's own unit test.
//
// A new entry is meant to be a deliberate act. Renaming one of these makes
// this test fail until the new name is added, which is the intended speed
// bump: the question "does this caller create subtests from what it gets
// back?" has to be answered by a person, once, out loud.
var legAxisAllowed = map[string]string{
	"cleanupOrphans": "sweeps each provisioned leg before any test runs; creates no subtests",
	"newSessions":    "builds one session per provisioned leg; creates no subtests",
	"composeLegsUnderTest": "the wrapper this guard exists to steer callers towards: it adds the " +
		"catalog-declared axis on top and emits a named, skipping subtest for every declared " +
		"edition the estate lacks",
	"TestComposeLegs_ExcludesKubernetes": "composeLegs' own unit test, which passes a hand-built " +
		"estate rather than the live one and asserts on the returned slice directly",
}

// legAxisCall is one call, in test/e2e/suite, to something that answers
// "which legs did this estate actually provision".
type legAxisCall struct {
	axis string // "composeLegs" or "estate.Legs"
	fn   string // the enclosing top-level function or method
	file string
	line int
}

// String renders a finding as file:line: function, so a failure can be
// pasted into an editor and land on the offending call.
func (c legAxisCall) String() string {
	return c.file + ":" + strconv.Itoa(c.line) + ": " + c.fn + " calls " + c.axis
}

// suiteLegAxisWalk is the parse. It returns every call to the two leg-axis
// expressions, keyed by the top-level function containing it, plus the counts
// this test uses to prove it actually read the code it claims to have read.
//
// Calls inside a function literal — a t.Run body, a cleanup closure — are
// attributed to the enclosing top-level function, which is the attribution
// that matters here: a composeLegs(estate) buried in a subtest closure is
// still a test reaching for the axis.
func suiteLegAxisWalk(t *testing.T) (calls []legAxisCall, wrapperCalls, filesParsed int, sawDeclaration bool) {
	t.Helper()

	dir := filepath.Join(repoRoot(t), "test", "e2e", "suite")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v — the suite this guard walks is not where it is expected to be", dir, err)
	}

	fset := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			// A parse failure is a refusal, never a skip: a file this walk
			// cannot read is a file that could carry the very call it is
			// looking for.
			t.Fatalf("parse %s: %v", path, err)
		}
		filesParsed++

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name == "composeLegs" {
				sawDeclaration = true
			}
			if fn.Body == nil {
				continue
			}
			name := funcLabel(fn)
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					switch fun.Name {
					case "composeLegs":
						calls = append(calls, legAxisCall{
							axis: "composeLegs", fn: name,
							file: entry.Name(), line: fset.Position(call.Pos()).Line,
						})
					case "composeLegsUnderTest":
						wrapperCalls++
					}
				case *ast.SelectorExpr:
					// estate.Legs(): the package-global estate specifically,
					// not any Estate value a helper was handed.
					ident, ok := fun.X.(*ast.Ident)
					if ok && ident.Name == "estate" && fun.Sel.Name == "Legs" {
						calls = append(calls, legAxisCall{
							axis: "estate.Legs", fn: name,
							file: entry.Name(), line: fset.Position(call.Pos()).Line,
						})
					}
				}
				return true
			})
		}
	}
	return calls, wrapperCalls, filesParsed, sawDeclaration
}

// funcLabel names a declaration the way a reader would look for it, methods
// included.
func funcLabel(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}
	var recv string
	switch typ := fn.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if ident, ok := typ.X.(*ast.Ident); ok {
			recv = "*" + ident.Name
		}
	case *ast.Ident:
		recv = typ.Name
	}
	return "(" + recv + ")." + fn.Name.Name
}

// TestUnit_SuiteLegAxis_NoTestReachesForWhatTheEstateHappenedToProvision is
// the guard. Two axes, one rule: a test must range over the axis the catalog
// declares — Sessions.Editions(), reached through composeLegsUnderTest — and
// never over the legs this particular estate happens to carry, because an
// empty or short slice there produces silence rather than a skip.
//
// The estate.Legs() row is the same rule applied one level down, so the class
// is closed rather than the single instance: every caller of it today is a
// helper that returns or skips (serverFor, rawServerFor, rawClientFor,
// estateCarriesLeg), and a Test function calling it directly would range over
// exactly the same thing composeLegs does.
func TestUnit_SuiteLegAxis_NoTestReachesForWhatTheEstateHappenedToProvision(t *testing.T) {
	t.Parallel()

	calls, wrapperCalls, filesParsed, sawDeclaration := suiteLegAxisWalk(t)

	// Non-vacuity first, and as failures rather than skips. Each of these is
	// an anchor the conclusion below rests on; without them "no offending
	// call was found" is indistinguishable from "nothing was looked at".
	if filesParsed < 10 {
		t.Fatalf("parsed only %d files in test/e2e/suite: the suite is ~20 files, so this walk is "+
			"looking in the wrong place and would report clean whatever the code said", filesParsed)
	}
	if !sawDeclaration {
		t.Fatal("no func composeLegs declaration found in test/e2e/suite: it was renamed or moved, " +
			"so this guard is now watching a name nothing uses and every assertion below is vacuous")
	}
	if wrapperCalls == 0 {
		t.Fatal("no call to composeLegsUnderTest found in test/e2e/suite: the wrapper this guard " +
			"steers callers towards is unused, which means either it was renamed or every test " +
			"went back to the axis this guard forbids")
	}

	byAxis := map[string][]legAxisCall{}
	for _, c := range calls {
		byAxis[c.axis] = append(byAxis[c.axis], c)
	}

	tests := []struct {
		axis string
		// offending reports whether a call is a violation. Kept as a
		// predicate per axis rather than one shared rule: the two axes are
		// forbidden to different populations — composeLegs to everything
		// except its named callers, estate.Legs only to tests.
		offending func(legAxisCall) bool
		why       string
	}{
		{
			axis: "composeLegs",
			offending: func(c legAxisCall) bool {
				_, allowed := legAxisAllowed[c.fn]
				return !allowed
			},
			why: "call composeLegsUnderTest(t) instead: composeLegs returns only the legs this " +
				"estate provisioned, so on a half estate the loop body never runs, no subtest is " +
				"ever named, and the test passes having measured nothing",
		},
		{
			axis: "estate.Legs",
			offending: func(c legAxisCall) bool {
				return strings.HasPrefix(c.fn, "Test")
			},
			why: "a test must not range over estate.Legs() either: it is the same axis one level " +
				"down, with the same silence when a leg is missing — go through serverFor, " +
				"fixtureClient or composeLegsUnderTest, each of which skips by name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.axis, func(t *testing.T) {
			t.Parallel()
			found := byAxis[tc.axis]
			if len(found) == 0 {
				t.Fatalf("found no %s call anywhere in test/e2e/suite: this walk cannot see the "+
					"shape it is checking, so its silence means nothing", tc.axis)
			}

			var offenders []string
			for _, c := range found {
				if tc.offending(c) {
					offenders = append(offenders, c.String())
				}
			}
			if len(offenders) == 0 {
				return
			}
			sort.Strings(offenders)
			t.Errorf("%d call(s) reach the estate's own leg axis from a test:\n    %s\n\n%s\n\n"+
				"(functions allowed to call composeLegs, and why: %s)",
				len(offenders), strings.Join(offenders, "\n    "), tc.why, allowedSummary())
		})
	}
}

// allowedSummary renders legAxisAllowed deterministically for a failure
// message, so the reader is told what the exceptions are and why at the
// moment they need to know.
func allowedSummary() string {
	names := make([]string, 0, len(legAxisAllowed))
	for name := range legAxisAllowed {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, name+" — "+legAxisAllowed[name])
	}
	return strings.Join(parts, "; ")
}
