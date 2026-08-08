package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

func TestUnit_IsIdentifierPathParam_SuffixRule(t *testing.T) {
	t.Parallel()
	t.Run("IsIdentifierPathParam SuffixRule", func(t *testing.T) {
		for _, tc := range []struct {
			name string
			want bool
		}{
			{"id", true},
			{"Id", true},
			{"environmentId", true},
			{"registryID", true},
			{"repositoryName", false},
			{"state", false},
			{"rpn", false},
			{"i", false},
		} {
			if got := isIdentifierPathParam(tc.name); got != tc.want {
				t.Errorf("isIdentifierPathParam(%q) = %v, want %v", tc.name, got, tc.want)
			}
		}
	})
}

func TestUnit_PathParamRequiresMinimum_NonIntegerType_False(t *testing.T) {
	t.Parallel()
	t.Run("PathParamRequiresMinimum NonIntegerType False", func(t *testing.T) {
		if pathParamRequiresMinimum("Op", "id", "string") {
			t.Error("pathParamRequiresMinimum() = true, want false for a string-typed parameter")
		}
	})
}

func TestUnit_PathParamRequiresMinimum_ExceptedPair_False(t *testing.T) {
	t.Parallel()
	t.Run("PathParamRequiresMinimum ExceptedPair False", func(t *testing.T) {
		if pathParamRequiresMinimum("EndpointDockerhubStatus", "registryId", "integer") {
			t.Error("pathParamRequiresMinimum() = true, want false: this exact pair is a recorded exception")
		}
	})
}

func TestUnit_PathParamRequiresMinimum_OrdinaryIdentifier_True(t *testing.T) {
	t.Parallel()
	t.Run("PathParamRequiresMinimum OrdinaryIdentifier True", func(t *testing.T) {
		if !pathParamRequiresMinimum("TagDelete", "id", "integer") {
			t.Error("pathParamRequiresMinimum() = false, want true for an ordinary identifier-shaped integer path parameter")
		}
	})
}

// TestUnit_AuditIdentifierMinimum_CatalogHasMinimum_NoFinding is the clean
// case: a fixture Input struct implementing MinimumParams for its identifier
// field.
func TestUnit_AuditIdentifierMinimum_CatalogHasMinimum_NoFinding(t *testing.T) {
	t.Parallel()
	t.Run("AuditIdentifierMinimum CatalogHasMinimum NoFinding", func(t *testing.T) {
		ops := canaryMinimumSpecOps()
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "CanaryMinimumSelfTest",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: canaryNoopHandler, Input: canaryMinIDInput{},
		}}
		result, err := auditIdentifierMinimum(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditIdentifierMinimum() error = %v", err)
		}
		if result.HasGaps() {
			t.Errorf("auditIdentifierMinimum() findings = %v, want none", result.Findings)
		}
		if result.ParamsChecked != 1 {
			t.Errorf("auditIdentifierMinimum() ParamsChecked = %d, want 1", result.ParamsChecked)
		}
	})
}

// TestUnit_AuditIdentifierMinimum_CatalogMissingMinimum_Finding is this
// task's mutation proof at the audit level: the identical operation, but the
// Input struct declares no MinimumParams() at all — exactly what a hand edit
// that deletes the method (or the "id": 1 entry inside it) leaves behind.
func TestUnit_AuditIdentifierMinimum_CatalogMissingMinimum_Finding(t *testing.T) {
	t.Parallel()
	t.Run("AuditIdentifierMinimum CatalogMissingMinimum Finding", func(t *testing.T) {
		ops := canaryMinimumSpecOps()
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "CanaryMinimumSelfTest",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: canaryNoopHandler, Input: canaryNoMinIDInput{},
		}}
		result, err := auditIdentifierMinimum(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditIdentifierMinimum() error = %v", err)
		}
		if !result.HasGaps() {
			t.Fatal("auditIdentifierMinimum() reported no gap, want one: the Input struct declares no MinimumParams()")
		}
		if len(result.Findings) != 1 || result.Findings[0].ParamName != "id" {
			t.Errorf("auditIdentifierMinimum() findings = %+v, want one finding naming \"id\"", result.Findings)
		}
	})
}

// TestUnit_AuditIdentifierMinimum_NoPathParams_NoCheckPerformed proves an
// operation with no path parameters at all costs this audit nothing.
func TestUnit_AuditIdentifierMinimum_NoPathParams_NoCheckPerformed(t *testing.T) {
	t.Parallel()
	t.Run("AuditIdentifierMinimum NoPathParams NoCheckPerformed", func(t *testing.T) {
		ops := map[string]specOperation{
			"FixtureOp": {Op: specdiff.SpecOperation{OperationID: "FixtureOp"}},
		}
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "FixtureOp",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: canaryNoopHandler,
		}}
		result, err := auditIdentifierMinimum(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditIdentifierMinimum() error = %v", err)
		}
		if result.HasGaps() || result.ParamsChecked != 0 {
			t.Errorf("auditIdentifierMinimum() = %+v, want no checks performed", result)
		}
	})
}

// TestUnit_VerifyMinimumCanary_RealMechanism_Passes proves the canary run()
// calls before trusting anything auditIdentifierMinimum reports.
func TestUnit_VerifyMinimumCanary_RealMechanism_Passes(t *testing.T) {
	t.Parallel()
	t.Run("VerifyMinimumCanary RealMechanism Passes", func(t *testing.T) {
		if err := verifyMinimumCanary(); err != nil {
			t.Fatalf("verifyMinimumCanary() error = %v, want nil", err)
		}
	})
}

// TestUnit_RealCatalog_EveryIdentifierPathParam_HasMinimum is the integration
// proof against production code: every identifier-shaped integer path
// parameter wiring.AllSpecs() declares today must publish "minimum": 1.
func TestUnit_RealCatalog_EveryIdentifierPathParam_HasMinimum(t *testing.T) {
	t.Parallel()
	t.Run("RealCatalog EveryIdentifierPathParam HasMinimum", func(t *testing.T) {
		ceData, err := readFileIn(realSpecsDir, "ce-2.44.0.json")
		if err != nil {
			t.Fatalf("read real ce spec: %v", err)
		}
		eeData, err := readFileIn(realSpecsDir, "ee-2.44.0.json")
		if err != nil {
			t.Fatalf("read real ee spec: %v", err)
		}
		ceOps, err := parseSpecOperations(ceData)
		if err != nil {
			t.Fatalf("parseSpecOperations(ce) error = %v", err)
		}
		eeOps, err := parseSpecOperations(eeData)
		if err != nil {
			t.Fatalf("parseSpecOperations(ee) error = %v", err)
		}

		result, err := auditIdentifierMinimum(eeOps, ceOps, wiring.AllSpecs())
		if err != nil {
			t.Fatalf("auditIdentifierMinimum() error = %v", err)
		}
		if result.ParamsChecked == 0 {
			t.Fatal("auditIdentifierMinimum() ParamsChecked = 0, want at least tags.delete/registries.inspect's own \"id\" to be checked")
		}
		if result.HasGaps() {
			t.Errorf("auditIdentifierMinimum() found %d real, unresolved finding(s): %+v", len(result.Findings), result.Findings)
		}
	})
}

// TestUnit_PathParamMinimumExceptions_MirrorsTheGeneratorsOwnTable pins the
// relationship the two tables have, rather than their contents. They are
// deliberately separate — the generator's carries a reason string it stamps
// into the emitted code, the audit's only needs to know a key is excused —
// but a key present in one and absent from the other is always a defect, in
// whichever direction it points.
//
// It points one way today: the generator excuses
// {ServiceImageStatus, serviceId} and the audit does not. A minimum finding
// cannot be allow-listed, so that gap turns into a CI failure with no legal
// remedy the moment docker.service_image_status is declared — which is the
// very next commit. The audit's own doc comment predicted this ("inert ...
// until a future wave adds the endpoints/docker domains") and then did not
// carry the entry that would have made it true.
func TestUnit_PathParamMinimumExceptions_MirrorsTheGeneratorsOwnTable(t *testing.T) {
	t.Parallel()
	t.Run("PathParamMinimumExceptions MirrorsTheGeneratorsOwnTable", func(t *testing.T) {
		// The generator's table is in package main of another command, so it
		// cannot be imported. Parse the source and extract the keys, which
		// also means this test sees a new entry the moment someone adds one
		// there.
		src, err := os.ReadFile("../gen_action_inputs/fields.go")
		if err != nil {
			t.Fatalf("reading the generator's own table: %v", err)
		}
		generator := parsePathParamMinimumExceptionsKeys(t, string(src))
		if len(generator) == 0 {
			t.Fatal("extracted 0 keys from pathParamMinimumExceptions in the generator's source; the declaration was not found, or no longer has this shape")
		}

		for key := range generator {
			if !pathParamMinimumExceptions[key] {
				t.Errorf("the generator excuses %+v but this audit does not; a minimum finding cannot be allow-listed, so the first action using it fails CI with no legal remedy", key)
			}
		}
		for key := range pathParamMinimumExceptions {
			if !generator[key] {
				t.Errorf("this audit excuses %+v but the generator does not; the audit would stay silent about a missing minimum the generator intends to stamp", key)
			}
		}
	})
}

// parsePathParamMinimumExceptionsKeys extracts every pathParamKey out of the
// pathParamMinimumExceptions declaration in a Go source file, found by
// identifier name via go/ast rather than by pattern-matching the
// {OperationID: "...", ParamName: "..."} literal shape across the whole
// file. That shape is not unique to this table: fields.go also declares
// pathParamTypeOverrides using the identical shape, with a
// ServiceImageStatus/serviceId entry of its own, so a scan blind to which
// map it is inside would silently read a key out of the wrong table. The
// shape is also not the only valid spelling: a positional literal
// ({"OperationID value", "ParamName value"}, with no field names at all) is
// ordinary, go-vet-clean Go for a value inside its own declaring package,
// and a regexp anchored to the keyed spelling would simply not see it —
// which would make this test blind to an entry rewritten that way in either
// table. Walking the declaration's own *ast.CompositeLit and accepting both
// a *ast.KeyValueExpr and a bare positional element avoids both failure
// modes: it can only ever look inside pathParamMinimumExceptions, and it
// does not care which of the two equivalent forms an entry is written in.
func parsePathParamMinimumExceptionsKeys(t *testing.T, src string) map[pathParamKey]bool {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "fields.go", src, 0)
	if err != nil {
		t.Fatalf("parsing generator source as Go: %v", err)
	}

	out := map[pathParamKey]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		valueSpec, ok := n.(*ast.ValueSpec)
		if !ok {
			return true
		}
		for i, name := range valueSpec.Names {
			if name.Name != "pathParamMinimumExceptions" || i >= len(valueSpec.Values) {
				continue
			}
			mapLit, ok := valueSpec.Values[i].(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, elt := range mapLit.Elts {
				entry, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				keyLit, ok := entry.Key.(*ast.CompositeLit)
				if !ok {
					continue
				}
				if key, ok := pathParamKeyFromCompositeLit(keyLit); ok {
					out[key] = true
				}
			}
		}
		return true
	})
	return out
}

// pathParamKeyFromCompositeLit reads a single map-key composite literal —
// either {OperationID: "...", ParamName: "..."} or the equivalent
// positional {"...", "..."} — into a pathParamKey.
func pathParamKeyFromCompositeLit(lit *ast.CompositeLit) (pathParamKey, bool) {
	var operationID, paramName string
	positionalIndex := 0
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			ident, ok := kv.Key.(*ast.Ident)
			if !ok {
				continue
			}
			value, ok := stringLiteralValue(kv.Value)
			if !ok {
				continue
			}
			switch ident.Name {
			case "OperationID":
				operationID = value
			case "ParamName":
				paramName = value
			}
			continue
		}

		value, ok := stringLiteralValue(elt)
		if !ok {
			continue
		}
		switch positionalIndex {
		case 0:
			operationID = value
		case 1:
			paramName = value
		}
		positionalIndex++
	}
	if operationID == "" || paramName == "" {
		return pathParamKey{}, false
	}
	return pathParamKey{OperationID: operationID, ParamName: paramName}, true
}

// stringLiteralValue unquotes e if it is a string literal, and reports
// whether it was one.
func stringLiteralValue(e ast.Expr) (string, bool) {
	basicLit, ok := e.(*ast.BasicLit)
	if !ok || basicLit.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(basicLit.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
