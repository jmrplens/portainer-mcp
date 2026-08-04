package main

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// TestUnit_AssembleOperationFields_ContainerIDAndServiceID_PublishString is
// P3.3 task 7's real-operation proof that pathParamTypeOverrides actually
// changes what assembleOperationFields renders: the four operations
// pathParamTypeOverrides names must render their identifier field as
// "string", not typeOf's ordinary "integer" -> "int" derivation, while the
// sibling environmentId path parameter on each of them is completely
// unaffected (still "int", still required, still carrying its own
// "minimum": 1 — see TestUnit_AssembleOperationFields_ContainerID_TakesNoMinimum
// and its serviceId counterpart below for that half). A generator that
// turned every path parameter to "string", or that only fixed the type
// without leaving environmentId alone, would still pass a test that checked
// one direction only.
func TestUnit_AssembleOperationFields_ContainerIDAndServiceID_PublishString(t *testing.T) {
	t.Parallel()
	cases := []struct {
		operationID string
		idField     string
	}{
		{"DockerContainerGpusInspect", "containerId"},
		{"ContainerImageStatus", "containerId"},
		{"SnapshotContainerInspect", "containerId"},
		{"ServiceImageStatus", "serviceId"},
	}
	for _, tc := range cases {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			fields := realFields(t, tc.operationID)

			id, ok := fieldByJSONName(fields, tc.idField)
			if !ok {
				t.Fatalf("field %q not found", tc.idField)
			}
			if id.GoType != "string" {
				t.Errorf("%s.GoType = %q, want \"string\": pathParamTypeOverrides should have overridden typeOf's ordinary \"integer\" -> \"int\" derivation", tc.idField, id.GoType)
			}
			if !id.Required {
				t.Errorf("%s.Required = false, want true: every path parameter is required regardless of its type", tc.idField)
			}

			env, ok := fieldByJSONName(fields, "environmentId")
			if !ok {
				t.Fatal(`field "environmentId" not found`)
			}
			if env.GoType != "int" {
				t.Errorf("environmentId.GoType = %q, want \"int\": only %s is named in pathParamTypeOverrides on this operation, not environmentId", env.GoType, tc.idField)
			}
		})
	}
}

// TestUnit_PathParamTypeOverrides_EveryEntryMatchesARealOperation keeps
// pathParamTypeOverrides honest against the vendored spec, mirroring
// TestUnit_PathParamMinimumExceptions_EveryEntryMatchesARealOperation exactly:
// an entry naming an operationId the spec does not have, or a parameter that
// operation does not declare "integer", is an override protecting nothing,
// and would silently outlive whatever respec made it stale.
func TestUnit_PathParamTypeOverrides_EveryEntryMatchesARealOperation(t *testing.T) {
	t.Parallel()
	// pathParamTypeOverrides is a map; sorted keys make subtest order (and
	// -run matching) deterministic despite Go's randomised map iteration.
	keys := make([]pathParamKey, 0, len(pathParamTypeOverrides))
	for key := range pathParamTypeOverrides {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].OperationID != keys[j].OperationID {
			return keys[i].OperationID < keys[j].OperationID
		}
		return keys[i].ParamName < keys[j].ParamName
	})

	for _, key := range keys {
		t.Run(key.OperationID+"/"+key.ParamName, func(t *testing.T) {
			t.Parallel()
			reason := pathParamTypeOverrides[key]
			if reason == "" {
				t.Errorf("%s/%s: override has no reason recorded", key.OperationID, key.ParamName)
			}
			if !isIdentifierPathParam(key.ParamName) {
				t.Errorf("%s/%s: parameter does not match isIdentifierPathParam, so it would never have rendered as an int identifier needing an override",
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
					t.Errorf("%s/%s: path parameter is not declared \"integer\" (schema %v) upstream, so there is nothing left to override",
						key.OperationID, key.ParamName, schema)
				}
			}
			if !found {
				t.Errorf("%s/%s: operation declares no path parameter by that name; remove this stale entry", key.OperationID, key.ParamName)
			}
		})
	}
}

// TestUnit_ServiceImageStatus_ServiceIDTakesNoMinimum is the serviceId
// counterpart of TestUnit_AssembleOperationFields_ContainerID_TakesNoMinimum
// (minimum_test.go): the fifth pathParamMinimumExceptions entry this task
// adds must drop serviceId's vestigial "minimum": 1 now that it publishes
// "string", while its own environmentId sibling keeps the bound.
func TestUnit_ServiceImageStatus_ServiceIDTakesNoMinimum(t *testing.T) {
	for _, tc := range []struct {
		name        string
		operationID string
	}{
		{"ServiceImageStatus", "ServiceImageStatus"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			minimums := minimumByJSONName(t, tc.operationID)

			serviceID, ok := minimums["serviceId"]
			if !ok {
				t.Fatal(`field "serviceId" not found`)
			}
			if serviceID != nil {
				t.Errorf(`"serviceId".Minimum = %d, want nil: serviceId now publishes "string", and a numeric minimum on a string field is a constraint JSON Schema ignores`, *serviceID)
			}

			environmentID, ok := minimums["environmentId"]
			if !ok {
				t.Fatal(`field "environmentId" not found`)
			}
			if environmentID == nil || *environmentID != 1 {
				t.Errorf(`"environmentId".Minimum = %v, want a pointer to 1: only serviceId is excepted on this operation`, environmentID)
			}
		})
	}
}

// realFields runs assembleOperationFields against the real vendored spec for
// operationID, exactly as buildRealHandlerSpec does, but returns only the
// flat field list — what the tests in this file need to inspect.
func realFields(t *testing.T, operationID string) []fieldSpec {
	t.Helper()
	op, doc, res := realOperation(t, operationID)
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, operationID+"Input", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields(%s) error = %v", operationID, err)
	}
	return fields
}

// fieldByJSONName finds fields' entry named name, the same lookup
// buildHandlerSpec itself does by JSONName.
func fieldByJSONName(fields []fieldSpec, name string) (fieldSpec, bool) {
	for _, f := range fields {
		if f.JSONName == name {
			return f, true
		}
	}
	return fieldSpec{}, false
}

// reflectTypeForGoType maps one of typeOf's own rendered Go type strings back
// to a real reflect.Type, so buildProbeInputType can hand reflect.StructOf a
// live field list built from the generator's own fieldSpec output rather
// than a second, hand-transcribed struct definition that could silently
// drift from what this generator actually renders.
func reflectTypeForGoType(t *testing.T, goType string) reflect.Type {
	t.Helper()
	base, pointer := strings.CutPrefix(goType, "*")
	var rt reflect.Type
	switch base {
	case "string":
		rt = reflect.TypeOf("")
	case "int":
		rt = reflect.TypeOf(0)
	case "bool":
		rt = reflect.TypeOf(false)
	case "float64":
		rt = reflect.TypeOf(float64(0))
	default:
		t.Fatalf("reflectTypeForGoType: %q is not one of the scalar Go types typeOf can produce for a top-level operation field", goType)
	}
	if pointer {
		rt = reflect.PointerTo(rt)
	}
	return rt
}

// buildProbeInputType builds a real, live reflect.Type for fields — one
// exported field per fieldSpec, in the exact Go type and struct tag
// (fieldTag) this generator would render — via reflect.StructOf. This is
// deliberately not a hand-written probe struct: fields comes from a real
// assembleOperationFields run against the real vendored spec, so the type
// this produces is what cmd/gen_action_inputs actually derives for the real
// operation today, not a second, hand-authored guess at its shape that could
// silently stop matching the generator's own output.
//
// Every operation this file tests has only top-level scalar path/query
// fields (no nested object, no enum) — see docker's own spec entries for
// DockerContainerGpusInspect/ContainerImageStatus/SnapshotContainerInspect/
// ServiceImageStatus — so reflectTypeForGoType's four cases are the only ones
// this needs to handle.
func buildProbeInputType(t *testing.T, fields []fieldSpec) reflect.Type {
	t.Helper()
	sfs := make([]reflect.StructField, len(fields))
	for i, f := range fields {
		sfs[i] = reflect.StructField{
			Name: f.GoName,
			Type: reflectTypeForGoType(t, f.GoType),
			Tag:  reflect.StructTag(rawStructTag(t, f.JSONName, f.Required, f.Description, f.RequiresEdition)),
		}
	}
	return reflect.StructOf(sfs)
}

// rawStructTag calls the generator's own fieldTag and unwraps its result back
// to the bare `key:"value" ...` content reflect.StructField.Tag expects.
// fieldTag renders a string meant to be spliced directly into generated Go
// *source text*, delimited either by backticks or, when the content itself
// contains one, by an ordinary quoted string literal (see fieldTag's own doc
// comment on render.go) — reflect.StructField.Tag wants neither delimiter,
// only what is between them, so this is the one place a test needs to peel
// that back off rather than duplicate fieldTag's own key:"value" assembly a
// second time.
func rawStructTag(t *testing.T, name string, required bool, description string, requiresEdition edition.Edition) string {
	t.Helper()
	rendered := fieldTag(name, required, description, requiresEdition)
	if strings.HasPrefix(rendered, "`") && strings.HasSuffix(rendered, "`") && len(rendered) >= 2 {
		return rendered[1 : len(rendered)-1]
	}
	unquoted, err := strconv.Unquote(rendered)
	if err != nil {
		t.Fatalf("fieldTag(%q) = %q, could not unwrap to a raw struct tag: %v", name, rendered, err)
	}
	return unquoted
}

// TestUnit_IdentifierTypeOverrides_ValidateInput_AcceptsRealisticStringRejectsInteger
// is the measurement the twelve-agent reconnaissance itself used, run again
// here against the fixed generator: toolutil.ActionSpec.ValidateInput is the
// real validation path every tool surface's Execute goes through before any
// handler runs (see internal/toolutil/schema.go), so this is the same gate a
// real call would hit.
//
// Both directions matter, and the second is what actually proves the type
// changed. Accepting the realistic string alone would also pass against the
// unfixed, "integer"-typed schema if JSON Schema validation happened to be
// loose — it is not, but a test that only checks acceptance could not tell
// "the type is now string" apart from "the type was never enforced at all".
// Rejecting a plain integer is the half that can only pass once the
// published schema actually says "string": that is exactly what P3.3 task
// 7's own brief calls "the cheat" for docker.service_image_status specifically
// — a probe container labelled com.docker.swarm.service.id=1 would make the
// integer 1 a real, resolvable service ID, and a test that only tried
// "realistic string in, success" would never notice the schema itself still
// published "integer" underneath. containerId has no equivalent shortcut
// (Docker, not a test author, assigns its 64-hex ID), which is exactly why
// the fix is not optional there and this test still checks both directions
// uniformly across all four fields rather than treating containerId as
// somehow already safe.
func TestUnit_IdentifierTypeOverrides_ValidateInput_AcceptsRealisticStringRejectsInteger(t *testing.T) {
	t.Parallel()
	cases := []struct {
		operationID string
		idField     string
		realisticID string
	}{
		{"DockerContainerGpusInspect", "containerId", "3f4e2f8a9b1c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e"},
		{"ContainerImageStatus", "containerId", "3f4e2f8a9b1c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e"},
		{"SnapshotContainerInspect", "containerId", "3f4e2f8a9b1c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e"},
		{"ServiceImageStatus", "serviceId", "9mnpnzenvg8p8tdbtq4wvbkcz"},
	}
	for _, tc := range cases {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			fields := realFields(t, tc.operationID)
			inputType := buildProbeInputType(t, fields)
			spec := toolutil.ActionSpec{
				Name:  "probe." + tc.operationID,
				Input: reflect.New(inputType).Elem().Interface(),
			}

			// Every other required field (environmentId on all four; there is
			// no third required field on any of them) is supplied identically
			// in both directions, so the accept/reject verdict is attributable
			// only to the identifier field varied below.
			accept, err := json.Marshal(map[string]any{"environmentId": 7, tc.idField: tc.realisticID})
			if err != nil {
				t.Fatalf("marshal accept payload: %v", err)
			}
			if err := spec.ValidateInput(accept); err != nil {
				t.Errorf("%s: realistic string %s %q was rejected: %v", tc.operationID, tc.idField, tc.realisticID, err)
			}

			reject, err := json.Marshal(map[string]any{"environmentId": 7, tc.idField: 12345})
			if err != nil {
				t.Fatalf("marshal reject payload: %v", err)
			}
			if err := spec.ValidateInput(reject); err == nil {
				t.Errorf("%s: integer %s = 12345 was accepted, want a refusal: the published schema must no longer declare \"integer\" for this field", tc.operationID, tc.idField)
			}
		})
	}
}

// TestUnit_IdentifierTypeOverrides_WouldGateSpecDriftAuditOnceWired is P3.3
// task 7's proof for Step 5 of its own brief — "confirm ... that removing
// one [allow-list entry] makes the audit gate" — measured honestly against
// what is actually true of this tree right now, not against a run that would
// pass or fail for an unrelated reason.
//
// docker and endpoints are not scaffolded domains yet (see this plan's own
// "Nothing here is a domain" instruction): wiring.AllSpecs() names 18 EE/15
// CE operations today, none of them one of these four, and
// cmd/audit_spec_drift's own allow-list only ever excuses a finding a real
// run currently produces for a *declared* catalog action (see its own
// package doc and api/spec-drift-allowlist.yaml's header: "an entry that
// excuses nothing a real run currently finds gating is itself a build
// error"). Adding four live entries to that file today, before any action
// with these OperationIDs is declared, would not excuse a real finding — it
// would make audit_spec_drift's own stale-entry check report all four as
// stale and fail the build (confirmed directly: adding one such entry and
// running `go run ./cmd/audit_spec_drift` reports exactly
// "Stale allow-list entries (no matching finding — remove them)" and a
// non-zero exit), which would violate this plan's own global constraint
// ("make audit-spec-drift clean") the moment this commit lands. That is also
// why Step 5 as literally read — run the real audit, remove an entry, watch
// it gate — cannot discriminate today: with all four entries present the
// real audit already fails for staleness, so removing one changes nothing
// about the outcome, exactly the non-discriminating shape this plan's own
// standing warning describes.
//
// So the allow-list entries are not added to api/spec-drift-allowlist.yaml
// in this commit (see the task 7 report for the full reasoning). What this
// test proves instead, honestly, is the fact those four entries would need
// to record: using internal/specdiff's own comparison engine — the identical
// ShapeFromSpec/ShapeFromCatalog/Compare cmd/audit_spec_drift itself calls,
// imported here rather than reimplemented — each of the four operations'
// identifier field, once a catalog action publishes pathParamTypeOverrides'
// "string" for it, produces exactly one specdiff.ChangeType finding against
// the vendored specification's own "integer". specdiff's exported ChangeKind
// doc comment (model.go) and cmd/audit_spec_drift's own isGating both treat
// ChangeAdded/ChangeRemoved/ChangeType/ChangeRequiredness/ChangeEnum as
// unconditionally gating — never allow-listable away except by a named,
// dated entry — so this is a real, would-gate divergence, not a hypothetical
// one, and four entries (one per operation, keyed on this exact field) are
// genuinely what a future wave must add in the same commit that wires
// docker/endpoints into wiring.AllSpecs().
func TestUnit_IdentifierTypeOverrides_WouldGateSpecDriftAuditOnceWired(t *testing.T) {
	t.Parallel()
	eeSpec, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("read vendored EE spec: %v", err)
	}

	cases := []struct {
		operationID string
		// specOperationID is the vendored spec's own literal "operationId"
		// string, which specdiff.LoadSpecOperation matches verbatim against
		// the raw document rather than through exportedName's PascalCase
		// conversion (spec.go, used everywhere else in this package): three
		// of the four are declared lower-camel-case upstream
		// ("dockerContainerGpusInspect", "containerImageStatus",
		// "snapshotContainerInspect"), and the fourth is, unusually,
		// declared PascalCase already ("ServiceImageStatus") — confirmed
		// directly against api/specs/ee-2.44.0.json, not assumed.
		specOperationID string
		idField         string
	}{
		{"DockerContainerGpusInspect", "dockerContainerGpusInspect", "containerId"},
		{"ContainerImageStatus", "containerImageStatus", "containerId"},
		{"SnapshotContainerInspect", "snapshotContainerInspect", "containerId"},
		{"ServiceImageStatus", "ServiceImageStatus", "serviceId"},
	}
	for _, tc := range cases {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			specOp, err := specdiff.LoadSpecOperation(eeSpec, tc.specOperationID)
			if err != nil {
				t.Fatalf("specdiff.LoadSpecOperation(%s): %v", tc.specOperationID, err)
			}
			specShape, err := specdiff.ShapeFromSpec(specOp)
			if err != nil {
				t.Fatalf("specdiff.ShapeFromSpec(%s): %v", tc.operationID, err)
			}

			fields := realFields(t, tc.operationID)
			inputType := buildProbeInputType(t, fields)
			action := toolutil.ActionSpec{
				Name:        "probe." + tc.operationID,
				OperationID: tc.operationID,
				Input:       reflect.New(inputType).Elem().Interface(),
			}
			catalogShape, err := specdiff.ShapeFromCatalog(action)
			if err != nil {
				t.Fatalf("specdiff.ShapeFromCatalog(%s): %v", tc.operationID, err)
			}

			var found *specdiff.FieldChange
			for _, c := range specdiff.Compare(specShape, catalogShape) {
				if c.JSONName == tc.idField {
					c := c
					found = &c
				}
			}
			if found == nil {
				t.Fatalf("Compare(spec, catalog) found no change at all for %q; want a ChangeType finding now that the catalog side publishes \"string\" against the spec's \"integer\" — this would mean the four allow-list entries are no longer needed, which is itself worth knowing", tc.idField)
			}
			if found.Kind != specdiff.ChangeType {
				t.Errorf("%s.%s: Compare reported kind %q, want %q", tc.operationID, tc.idField, found.Kind, specdiff.ChangeType)
			}
			if found.Before != "integer" || found.After != "string" {
				t.Errorf("%s.%s: Compare reported %q -> %q, want \"integer\" -> \"string\"", tc.operationID, tc.idField, found.Before, found.After)
			}
		})
	}
}
