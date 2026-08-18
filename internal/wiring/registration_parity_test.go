package wiring

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"testing"
)

// domainsAppendedIn returns the domain package names appended to the named
// function's specs slice, by walking the AST rather than matching text: a
// regexp over the whole file would happily read a neighbouring function's
// body, which is exactly how a mirror test stops discriminating.
//
// It only recognises the `append(<something>, <pkg>.Specs()...)` shape. That
// is deliberately narrow, so it refuses to guess: every spread-append in the
// function body (`append(x, Y...)`, whatever Y is) is counted, and if any of
// them does not match the `<pkg>.Specs()` shape, this fails loudly naming
// the file, the function, and the fact that it registers something in a
// shape this test cannot read. Without that check, a future domain exposed
// under a different accessor name (say, `Actions()` instead of `Specs()`)
// would be invisible to every one of the four call sites alike -- want and
// every got would agree, and the parity test would report green while a
// domain was genuinely missing from an audit. A parity gate that can pass
// on something it never saw is worse than no gate.
func domainsAppendedIn(t *testing.T, path, funcName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var domains []string
	spreadAppends := 0
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Name.Name != funcName {
			return true
		}
		ast.Inspect(fn.Body, func(inner ast.Node) bool {
			call, ok := inner.(*ast.CallExpr)
			if !ok {
				return true
			}
			ident, ok := call.Fun.(*ast.Ident)
			if !ok || ident.Name != "append" || len(call.Args) != 2 || !call.Ellipsis.IsValid() {
				return true
			}
			// append(specs, X...): every spread-append counts, whatever X
			// is, so a shape this walk cannot read still shows up as a
			// discrepancy rather than vanishing silently.
			spreadAppends++
			// append(specs, <pkg>.Specs()...)
			specsCall, ok := call.Args[1].(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := specsCall.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Specs" {
				return true
			}
			pkg, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			domains = append(domains, pkg.Name)
			return true
		})
		return false
	})
	if len(domains) == 0 {
		t.Fatalf("%s: found no domain appends in %s -- the test itself is broken, not the code", path, funcName)
	}
	if len(domains) != spreadAppends {
		t.Fatalf("%s: %s has %d append(...) calls but only %d match <pkg>.Specs(...) -- this test cannot see the other, so it cannot prove the lists agree", path, funcName, spreadAppends, len(domains))
	}
	sort.Strings(domains)
	return domains
}

func TestUnit_AuditDomainLists_MatchTheServer(t *testing.T) {
	root := filepath.Join("..", "..")
	want := domainsAppendedIn(t, filepath.Join(root, "internal", "wiring", "wiring.go"), "AllSpecs")

	mirrors := []struct {
		path string
		fn   string
	}{
		{filepath.Join(root, "cmd", "audit_1to1", "main.go"), "allCatalogSpecs"},
		{filepath.Join(root, "cmd", "audit_e2e_gaps", "main.go"), "allSpecs"},
		{filepath.Join(root, "cmd", "audit_discovery", "main.go"), "allSpecs"},
	}

	for _, m := range mirrors {
		t.Run(m.path, func(t *testing.T) {
			got := domainsAppendedIn(t, m.path, m.fn)
			if len(got) != len(want) {
				t.Fatalf("%s sees %v, wiring.AllSpecs sees %v", m.fn, got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("%s sees %v, wiring.AllSpecs sees %v", m.fn, got, want)
				}
			}
		})
	}
}
