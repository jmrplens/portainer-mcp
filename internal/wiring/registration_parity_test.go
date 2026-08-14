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
func domainsAppendedIn(t *testing.T, path, funcName string) []string {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	var domains []string
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
			if !ok || ident.Name != "append" || len(call.Args) != 2 {
				return true
			}
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
	sort.Strings(domains)
	return domains
}

func TestUnit_EveryAuditSeesTheSameDomainsAsTheServer(t *testing.T) {
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
