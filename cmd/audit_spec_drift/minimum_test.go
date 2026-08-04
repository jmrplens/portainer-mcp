package main

import (
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
