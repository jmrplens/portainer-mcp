package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"runtime"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// handlerRedactsCredential reports whether handler's own source calls a
// function named wrapperName anywhere in its body.
//
// # Why source, not the response value
//
// cmd/gen_action_inputs's own redaction-guard test (renderRedactionGuardFile,
// still generated for every domain today) already proves a wrapper function,
// called directly with a populated fixture, strips every credential-shaped
// field. That is not what P2's original defect was, though: registries'
// create/inspect/update handlers returned the API client's response *body*
// directly, never calling redact at all — the wrapper existed and worked
// fine, nothing ever routed through it. A wrapper existing and working is not
// the same fact as a handler calling it, and only the second is what a hand
// edit to an owned actions.go can silently undo (delete the call, leave the
// now-dead wrapper function in place — it still compiles, the redaction-guard
// test for the untouched wrapper still passes, and the handler leaks the
// credential regardless). This function is what closes that specific gap:
// it inspects the handler's own compiled source for a call to the wrapper,
// which is exactly the edit P2 made and exactly the edit a future one could
// repeat.
//
// This audit cannot invoke handler itself and inspect what it returns —
// every real handler calls the live Portainer API over HTTP
// (internal/portainer.Client), and audit_spec_drift is not e2e; it runs with
// no estate at all, by design (see this package's doc comment). Static
// inspection of the handler's own call graph is what is available without
// one.
//
// # How it finds the source
//
// Every declared toolutil.ActionSpec.Handler in this catalog is a
// package-level named function (never a closure or a method value — verified
// directly: internal/tools/system, tags and registries assign Handler a bare
// identifier in every one of their Specs(), and Validate's own compile-time
// proof pins the signature every one of them must share). For exactly that
// shape, reflect.ValueOf(handler).Pointer() plus runtime.FuncForPC resolves
// the function's fully qualified name and defining source file with no
// ambiguity, and go/parser can then find its *ast.FuncDecl by name in that
// file. A closure or a method value would report a synthetic name
// (".funcN" or a receiver-qualified one) that does not match any top-level
// *ast.FuncDecl; this function refuses rather than silently reporting "not
// called" for a handler shape it cannot actually inspect, matching this
// project's standing refuse-rather-than-guess convention.
func handlerRedactsCredential(handler toolutil.Handler, wrapperName string) (bool, error) {
	pc := reflect.ValueOf(handler).Pointer()
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return false, fmt.Errorf("cannot resolve handler function via runtime.FuncForPC: no function found at its entry address")
	}
	file, _ := fn.FileLine(pc)
	if file == "" {
		return false, fmt.Errorf("runtime.Func %q reports no source file; cannot inspect its body", fn.Name())
	}

	// fn.Name() is package-qualified, e.g.
	// "github.com/jmrplens/portainer-mcp/internal/tools/registries.registryInspect".
	// Only the segment after the final "." names the function itself, and a
	// closure or method value's name carries a further "." (".func1",
	// ".funcName-fm") that this lookup below will simply fail to find as a
	// top-level declaration — the refusal this function's own doc comment
	// describes.
	name := fn.Name()
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		name = name[idx+1:]
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, file, nil, 0)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", file, err)
	}

	var decl *ast.FuncDecl
	for _, d := range astFile.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == name {
			decl = fd
			break
		}
	}
	if decl == nil || decl.Body == nil {
		return false, fmt.Errorf(
			"could not find a top-level function declaration named %q in %s (handler %q resolved to this name; "+
				"a closure or method value cannot be inspected this way)",
			name, file, fn.Name())
	}

	called := false
	ast.Inspect(decl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch f := call.Fun.(type) {
		case *ast.Ident:
			if f.Name == wrapperName {
				called = true
			}
		case *ast.SelectorExpr:
			if f.Sel.Name == wrapperName {
				called = true
			}
		}
		return true
	})
	return called, nil
}
