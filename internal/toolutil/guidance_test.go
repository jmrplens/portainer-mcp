package toolutil

import (
	"strings"
	"testing"
)

func TestFillScopeParameterGuidance_FillsKnownNamesLeavesExplicitOnesAlone(t *testing.T) {
	t.Parallel()
	type input struct {
		EndpointID int `json:"endpointId" jsonschema:"Environment identifier,required"`
		Custom     int `json:"custom" jsonschema:"Something domain-specific"`
	}
	specs := []ActionSpec{{
		Name: "x.get", Domain: "x", Input: input{},
		ParameterGuidance: map[string]ParameterGuidance{
			"custom": {SemanticRole: "hand written"},
		},
	}}

	filled := FillScopeParameterGuidance(specs)
	g := filled[0].ParameterGuidance

	if g["endpointId"].SemanticRole == "" {
		t.Error("endpointId got no default guidance: the central table is what makes 441 actions affordable")
	}
	if !strings.Contains(strings.ToLower(g["endpointId"].ValueSource), "environment") {
		t.Errorf("endpointId guidance = %+v, want it to name the environment it identifies", g["endpointId"])
	}
	if g["custom"].SemanticRole != "hand written" {
		t.Error("the default table overwrote hand-written guidance")
	}
}

func TestFillScopeParameterGuidance_WarnsAboutTheEnvironmentOverload(t *testing.T) {
	t.Parallel()
	// Portainer calls three different numbers "environment" in different
	// routes. A model that confuses them acts on the wrong host. This is the
	// single most valuable sentence in the whole default table, so it is
	// pinned rather than left to whoever edits the table next.
	type input struct {
		EndpointID int `json:"endpointId" jsonschema:"Environment identifier,required"`
	}
	filled := FillScopeParameterGuidance([]ActionSpec{{Name: "x.get", Input: input{}}})
	confusions := strings.ToLower(strings.Join(filled[0].ParameterGuidance["endpointId"].CommonConfusions, " "))

	for _, want := range []string{"edge", "group"} {
		if !strings.Contains(confusions, want) {
			t.Errorf("endpointId confusions = %q, want them to mention %q", confusions, want)
		}
	}
}

// The brief's own fixture hand-writes guidance only for "custom", a name
// absent from the central table, so it cannot prove the table never
// overwrites an explicit entry for a name the table also knows about. This
// test closes that gap: endpointId is both explicitly set and a default
// table key, so overwriting it would go unnoticed by the other test alone.
func TestFillScopeParameterGuidance_LeavesExplicitDefaultTableKeyAlone(t *testing.T) {
	t.Parallel()
	type input struct {
		EndpointID int `json:"endpointId"`
	}
	specs := []ActionSpec{{
		Name: "x.get", Domain: "x", Input: input{},
		ParameterGuidance: map[string]ParameterGuidance{
			"endpointId": {SemanticRole: "hand written for endpointId"},
		},
	}}

	filled := FillScopeParameterGuidance(specs)
	if got := filled[0].ParameterGuidance["endpointId"].SemanticRole; got != "hand written for endpointId" {
		t.Errorf("endpointId SemanticRole = %q, want the explicit value preserved, not the default table's", got)
	}
}

// TestFillScopeParameterGuidance_DoesNotMutateItsInput guards against writing
// straight into the caller's ParameterGuidance map instead of copying it
// first. The fixture must start with a non-nil map that already carries one
// default-table key ("endpointId") and an Input whose JSON fields name a
// second, not-yet-present default-table key ("stackId"): only then does a
// buggy implementation that adds "stackId" directly to the map object the
// caller passed in have anything to leak into. A fixture with
// ParameterGuidance left nil (as this test used to build it) forces even a
// buggy implementation to allocate a fresh map — there being nothing to
// mutate in place — so it could never fail here regardless of whether the
// real map object is copied before merging.
func TestFillScopeParameterGuidance_DoesNotMutateItsInput(t *testing.T) {
	t.Parallel()
	// Specs come from package-level slices shared across editions and
	// surfaces. Filling in place would leak one catalog's guidance into
	// another's.
	type input struct {
		EndpointID int `json:"endpointId"`
		StackID    int `json:"stackId"`
	}
	callerMap := map[string]ParameterGuidance{
		"endpointId": {SemanticRole: "hand written"},
	}
	original := []ActionSpec{{Name: "x.get", Input: input{}, ParameterGuidance: callerMap}}

	_ = FillScopeParameterGuidance(original)

	if len(callerMap) != 1 {
		t.Errorf("caller's map grew from 1 entry to %d: FillScopeParameterGuidance must copy the map before merging defaults into it, not write through the reference it was given", len(callerMap))
	}
	if _, leaked := callerMap["stackId"]; leaked {
		t.Error("FillScopeParameterGuidance wrote a new key (\"stackId\") straight into the caller's ParameterGuidance map")
	}
	if original[0].ParameterGuidance["endpointId"].SemanticRole != "hand written" {
		t.Error("the caller's own map entry was altered")
	}
}
