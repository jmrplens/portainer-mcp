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

// minimumByJSONName runs assembleOperationFields against a real operation from
// the vendored spec and returns each field's Minimum by wire name, so the
// per-operation carve-out tests below read as one assertion per parameter
// rather than repeating the same six lines of setup.
func minimumByJSONName(t *testing.T, operationID string) map[string]*int {
	t.Helper()
	op, doc, res := realOperation(t, operationID)
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, operationID+"Input", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields(%s) error = %v", operationID, err)
	}
	out := make(map[string]*int, len(fields))
	for _, f := range fields {
		out[f.JSONName] = f.Minimum
	}
	return out
}

// TestUnit_AssembleOperationFields_DockerhubStatus_RegistryIDKeepsNoMinimum is
// I3's real-operation proof. registryId matches isIdentifierPathParam's suffix
// rule, and on every other operation that is correct — but on
// endpointDockerhubStatus, zero is Portainer's documented sentinel for the
// anonymous DockerHub registry and is the one value guaranteed to resolve.
// The endpoint "id" alongside it must still carry the bound, so this fails
// whichever way the carve-out is got wrong: too broad (id loses its minimum)
// or too narrow (registryId keeps one).
func TestUnit_AssembleOperationFields_DockerhubStatus_RegistryIDKeepsNoMinimum(t *testing.T) {
	t.Parallel()
	minimums := minimumByJSONName(t, "EndpointDockerhubStatus")

	registryID, ok := minimums["registryId"]
	if !ok {
		t.Fatal(`field "registryId" not found`)
	}
	if registryID != nil {
		t.Errorf(`"registryId".Minimum = %d, want nil: 0 is the anonymous-DockerHub sentinel and must stay callable`, *registryID)
	}

	id, ok := minimums["id"]
	if !ok {
		t.Fatal(`field "id" not found`)
	}
	if id == nil || *id != 1 {
		t.Errorf(`"id".Minimum = %v, want a pointer to 1: the environment identifier is unaffected by registryId's carve-out`, id)
	}
}

// TestUnit_AssembleOperationFields_RegistryIDKeepsItsMinimumElsewhere is the
// control for the test above: the carve-out is keyed by (operationId,
// parameter), not by parameter name, so the very same "registryId" on a
// different operation must still be refused at zero. Without this, deleting
// the operationId from the exception key and excluding registryId everywhere
// would pass the test above unnoticed.
func TestUnit_AssembleOperationFields_RegistryIDKeepsItsMinimumElsewhere(t *testing.T) {
	t.Parallel()
	minimums := minimumByJSONName(t, "EndpointRegistryAccess")

	registryID, ok := minimums["registryId"]
	if !ok {
		t.Fatal(`field "registryId" not found`)
	}
	if registryID == nil || *registryID != 1 {
		t.Errorf(`"registryId".Minimum = %v, want a pointer to 1: only endpointDockerhubStatus's registryId is excepted`, registryID)
	}
}

// TestUnit_AssembleOperationFields_ContainerID_TakesNoMinimum is I4's
// real-operation proof, across all three operations that declare a Docker hex
// container ID as "integer". environmentId sits next to containerId on every
// one of them and must keep its bound, so each case checks both directions on
// the same operation.
func TestUnit_AssembleOperationFields_ContainerID_TakesNoMinimum(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{
		"DockerContainerGpusInspect",
		"ContainerImageStatus",
		"SnapshotContainerInspect",
	} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			minimums := minimumByJSONName(t, operationID)

			containerID, ok := minimums["containerId"]
			if !ok {
				t.Fatal(`field "containerId" not found`)
			}
			if containerID != nil {
				t.Errorf(`"containerId".Minimum = %d, want nil: Portainer passes this through to Docker as a hex string, so no numeric bound is meaningful`, *containerID)
			}

			environmentID, ok := minimums["environmentId"]
			if !ok {
				t.Fatal(`field "environmentId" not found`)
			}
			if environmentID == nil || *environmentID != 1 {
				t.Errorf(`"environmentId".Minimum = %v, want a pointer to 1: only containerId is excepted on this operation`, environmentID)
			}
		})
	}
}

// TestUnit_PathParamMinimumExceptions_EveryEntryMatchesARealOperation keeps the
// carve-out table honest against the vendored spec, the same way
// TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation keeps
// actionNameOverrides honest. An entry naming an operationId the spec does not
// have, or a parameter that operation does not declare as an integer path
// parameter, is a carve-out protecting nothing — and it would silently outlive
// whatever respec made it stale.
func TestUnit_PathParamMinimumExceptions_EveryEntryMatchesARealOperation(t *testing.T) {
	t.Parallel()
	for key, reason := range pathParamMinimumExceptions {
		if reason == "" {
			t.Errorf("%s/%s: exception has no reason recorded", key.OperationID, key.ParamName)
		}
		if !isIdentifierPathParam(key.ParamName) {
			t.Errorf("%s/%s: parameter does not match isIdentifierPathParam, so it would never have taken a minimum and needs no exception",
				key.OperationID, key.ParamName)
		}

		op, _, _ := realOperation(t, key.OperationID)
		var found bool
		for _, param := range op.Parameters {
			name, _ := param["name"].(string)
			in, _ := param["in"].(string)
			if name != key.ParamName || in != "path" {
				continue
			}
			found = true
			schema, _ := param["schema"].(map[string]any)
			if schema == nil || schema["type"] != "integer" {
				t.Errorf("%s/%s: path parameter is not declared \"integer\" (schema %v), so no minimum would be stamped and the exception is dead",
					key.OperationID, key.ParamName, schema)
			}
		}
		if !found {
			t.Errorf("%s/%s: operation declares no path parameter by that name; remove this stale entry", key.OperationID, key.ParamName)
		}
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
