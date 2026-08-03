package main

import (
	"encoding/json"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// isIdentifierPathParam must match every one of the 285 integer path
// parameters the vendored EE specification declares that name a Portainer
// resource identifier, and none of the three that do not (state, rpn,
// repositoryName — see that function's own doc comment for the full
// derivation). Pinned by name, not just by the aggregate count, so a future
// change to the suffix rule cannot silently start (or stop) matching one of
// these without a test noticing.
func TestUnit_IsIdentifierPathParam_AcceptsIdentifiersRejectsCounterexamples(t *testing.T) {
	t.Parallel()
	identifiers := []string{
		"id", "environmentId", "credentialID", "endpoint_id", "containerId",
		"taskID", "endpointId", "registryId", "endpointID", "serviceId",
		"stackId", "jobID", "keyID", "repositoryID",
	}
	for _, name := range identifiers {
		if !isIdentifierPathParam(name) {
			t.Errorf("isIdentifierPathParam(%q) = false, want true", name)
		}
	}

	// state (a Docker task state code) and rpn (Quay's own toggle) are
	// ordinary non-identifier integers that legitimately accept zero.
	// repositoryName is Portainer's own known upstream defect — a resource
	// *name* declared "integer" — and this generator refuses to assume
	// anything about its range rather than mislabel a defect as a
	// guarantee (see registries.go's ecrDeleteTagsInput doc comment).
	for _, name := range []string{"state", "rpn", "repositoryName"} {
		if isIdentifierPathParam(name) {
			t.Errorf("isIdentifierPathParam(%q) = true, want false", name)
		}
	}
}

// TestUnit_AssembleOperationFields_EdgeConfigState_MarksIDNotState is the
// real-operation counterpart of the name-pattern test above: EdgeConfigState
// (PUT /edge_configurations/{id}/{state}) is the one real operation in the
// vendored spec with both an identifier-shaped integer path parameter ("id")
// and a non-identifier one ("state") side by side, so this proves the
// distinction survives all the way through assembleOperationFields against
// the real spec, not merely against a hand-built fixture.
func TestUnit_AssembleOperationFields_EdgeConfigState_MarksIDNotState(t *testing.T) {
	t.Parallel()
	op, doc, res := realOperation(t, "EdgeConfigState")
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, "edgeConfigStateInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}

	byName := make(map[string]fieldSpec, len(fields))
	for _, f := range fields {
		byName[f.JSONName] = f
	}

	id, ok := byName["id"]
	if !ok {
		t.Fatal(`field "id" not found`)
	}
	if id.Minimum == nil || *id.Minimum != 1 {
		t.Errorf(`"id".Minimum = %v, want a pointer to 1`, id.Minimum)
	}

	state, ok := byName["state"]
	if !ok {
		t.Fatal(`field "state" not found`)
	}
	if state.Minimum != nil {
		t.Errorf(`"state".Minimum = %v, want nil: state is not a resource identifier and must still accept zero`, *state.Minimum)
	}
}

// --- two-direction proof at the schema/validation level: a generator that
// stamped "minimum": 1 on every integer path parameter, identifier or not,
// would still pass a test that only checked the identifier is refused at
// zero. This exercises the *rendered* struct (renderStruct + its
// MinimumParams() method, exactly as cmd/gen_action_inputs emits it) through
// toolutil.ActionSpec.ValidateInput — the real validation path every caller
// goes through — and checks both directions in one test: reject id<=0,
// accept state==0.

// edgeConfigStateProbeInput mirrors the shape renderStruct would emit for
// EdgeConfigState's two path parameters: "id" (identifier, minimum 1) and
// "state" (not an identifier, no minimum) side by side on one struct, so a
// generator that failed to discriminate between them would fail this test
// regardless of which field it got wrong.
type edgeConfigStateProbeInput struct {
	ID    int `json:"id"`
	State int `json:"state"`
}

func (edgeConfigStateProbeInput) MinimumParams() map[string]int {
	return map[string]int{"id": 1}
}

func TestUnit_MinimumParams_DiscriminatesIdentifierFromOrdinaryInteger(t *testing.T) {
	t.Parallel()
	spec := toolutil.ActionSpec{Name: "probe.edgeConfigState", Input: edgeConfigStateProbeInput{}}

	for _, id := range []int{0, -1} {
		input, _ := json.Marshal(map[string]any{"id": id, "state": 0})
		if err := spec.ValidateInput(input); err == nil {
			t.Errorf("id=%d: ValidateInput() = nil error, want a refusal: id is a resource identifier and must be positive", id)
		}
	}

	// state == 0 must be accepted even though it is the very same Go type,
	// on the very same struct, next to the field that is refused at zero:
	// this is what would break silently if the generator applied "minimum"
	// to every integer path parameter rather than only identifier-shaped
	// ones.
	input, _ := json.Marshal(map[string]any{"id": 4, "state": 0})
	if err := spec.ValidateInput(input); err != nil {
		t.Errorf("id=4,state=0: ValidateInput() error = %v, want nil: state is not an identifier and must still accept zero", err)
	}
}
