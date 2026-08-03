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

func TestFillScopeParameterGuidance_DoesNotMutateItsInput(t *testing.T) {
	t.Parallel()
	// Specs come from package-level slices shared across editions and
	// surfaces. Filling in place would leak one catalog's guidance into
	// another's.
	type input struct {
		EndpointID int `json:"endpointId"`
	}
	original := []ActionSpec{{Name: "x.get", Input: input{}}}
	_ = FillScopeParameterGuidance(original)
	if original[0].ParameterGuidance != nil {
		t.Error("FillScopeParameterGuidance mutated the specs it was given")
	}
}
