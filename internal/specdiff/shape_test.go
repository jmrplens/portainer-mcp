package specdiff

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/tools/tags"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// findSpec returns the ActionSpec in specs whose OperationID matches, or
// fails the test: every case below names an operationId its own domain
// package is known to declare, so a miss means the test's own assumption
// about that domain went stale, not that the code under test is broken.
func findSpec(t *testing.T, specs []toolutil.ActionSpec, operationID string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range specs {
		if s.OperationID == operationID {
			return s
		}
	}
	t.Fatalf("no ActionSpec with OperationID %q; this test's assumption about the domain is stale", operationID)
	return toolutil.ActionSpec{}
}

func TestUnit_ShapeFromCatalog_DerivesFieldTypeRequirednessAndDescription(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, tags.Specs(), "TagCreate")

	shape, err := ShapeFromCatalog(spec)
	if err != nil {
		t.Fatalf("ShapeFromCatalog() error = %v", err)
	}
	if shape.OperationID != "TagCreate" {
		t.Errorf("OperationID = %q, want %q", shape.OperationID, "TagCreate")
	}
	if len(shape.Fields) != 1 {
		t.Fatalf("Fields = %v, want exactly one field (\"name\")", shape.Fields)
	}
	f := shape.Fields[0]
	if f.JSONName != "name" || f.Type != "string" || !f.Required {
		t.Errorf("Fields[0] = %+v, want {JSONName: name, Type: string, Required: true}", f)
	}
	if f.Origin != "" {
		t.Errorf("Fields[0].Origin = %q, want empty: ActionSpec carries no path/query/body routing information", f.Origin)
	}
}

// TestUnit_ShapeFromCatalog_CollapsesNullableType is the guard for
// canonicalType: google/jsonschema-go's reflector renders every optional
// (pointer or slice) Go field's "type" as a two-element array such as
// ["null", "string"], never as a plain string the way the vendored
// specification always does. Without collapsing that, ShapeFromCatalog would
// report a type of "[null string]" for essentially every optional field in
// the catalog, and Compare would see that as differing from ShapeFromSpec's
// plain "string" for every one of them — a wall of false positives, not a
// cosmetic quirk.
func TestUnit_ShapeFromCatalog_CollapsesNullableType(t *testing.T) {
	t.Parallel()
	type optionalFields struct {
		ID   int      `json:"id"`
		Name *string  `json:"name,omitempty"`
		Tags []string `json:"tags,omitempty"`
	}
	spec := toolutil.ActionSpec{OperationID: "Synthetic", Input: optionalFields{}}

	shape, err := ShapeFromCatalog(spec)
	if err != nil {
		t.Fatalf("ShapeFromCatalog() error = %v", err)
	}

	byName := make(map[string]FieldShape, len(shape.Fields))
	for _, f := range shape.Fields {
		byName[f.JSONName] = f
	}

	for _, tc := range []struct {
		name         string
		wantType     string
		wantRequired bool
	}{
		{"id", "integer", true},
		{"name", "string", false},
		{"tags", "array", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f, ok := byName[tc.name]
			if !ok {
				t.Fatalf("no field %q in %v", tc.name, shape.Fields)
			}
			if f.Type != tc.wantType {
				t.Errorf("Type = %q, want %q (raw nullable-array type must be collapsed)", f.Type, tc.wantType)
			}
			if f.Required != tc.wantRequired {
				t.Errorf("Required = %v, want %v", f.Required, tc.wantRequired)
			}
		})
	}
}

func TestUnit_ShapeFromCatalog_AppliesEnumParams(t *testing.T) {
	t.Parallel()
	shape, err := ShapeFromCatalog(toolutil.ActionSpec{OperationID: "Synthetic", Input: withEnumParams{}})
	if err != nil {
		t.Fatalf("ShapeFromCatalog() error = %v", err)
	}
	if len(shape.Fields) != 1 {
		t.Fatalf("Fields = %v, want exactly one field", shape.Fields)
	}
	got := shape.Fields[0].Enum
	want := []string{"1", "2", "3"}
	if len(got) != len(want) {
		t.Fatalf("Enum = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Enum = %v, want %v", got, want)
		}
	}
}

type withEnumParams struct {
	Kind int `json:"kind"`
}

func (withEnumParams) EnumParams() map[string][]any {
	return map[string][]any{"kind": {1, 2, 3}}
}

func TestUnit_ShapeFromSpec_FlattensPathAndQueryParameters(t *testing.T) {
	t.Parallel()
	// RegistryInspect: a required path parameter and an optional query
	// parameter, GET /registries/{id} — the ordinary case
	// assembleOperationFields's own doc comment uses as its example.
	shape := realShapeFromVendoredSpec(t, "RegistryInspect")

	byName := make(map[string]FieldShape, len(shape.Fields))
	for _, f := range shape.Fields {
		byName[f.JSONName] = f
	}

	id, ok := byName["id"]
	if !ok || id.Type != "integer" || !id.Required || id.Origin != "path" {
		t.Errorf(`Fields["id"] = %+v, want {Type: integer, Required: true, Origin: path}`, id)
	}
	endpointID, ok := byName["endpointId"]
	if !ok || endpointID.Type != "integer" || endpointID.Required || endpointID.Origin != "query" {
		t.Errorf(`Fields["endpointId"] = %+v, want {Type: integer, Required: false, Origin: query}`, endpointID)
	}
}

func TestUnit_ShapeFromSpec_RendersBodyPropertyAsWireJSONTag(t *testing.T) {
	t.Parallel()
	// TagCreate's request body declares its property as "Name" (the
	// Portainer API's own PascalCase convention for a payload schema), but
	// the field a model is actually asked for is "name": naming.go's
	// bodyJSONTag is what makes ShapeFromSpec agree with ShapeFromCatalog on
	// this JSONName rather than reporting one field removed ("Name") and one
	// added ("name") for every operation with a body.
	shape := realShapeFromVendoredSpec(t, "TagCreate")
	if len(shape.Fields) != 1 {
		t.Fatalf("Fields = %v, want exactly one field", shape.Fields)
	}
	f := shape.Fields[0]
	if f.JSONName != "name" || f.Type != "string" || !f.Required || f.Origin != "body" {
		t.Errorf("Fields[0] = %+v, want {JSONName: name, Type: string, Required: true, Origin: body}", f)
	}
}

// TestUnit_ShapeFromCatalogAndShapeFromSpec_AgreeStructurallyForPilotDomain is
// the end-to-end proof that the two producers this engine compares actually
// agree on an operation nothing has changed for: the catalog's tags and
// registries actions here were generated from this exact vendored
// specification and have not been hand-edited since. Only structural kinds
// are asserted — added, removed, type, requiredness, enum and origin — not
// ChangeDescription: this run itself found that path/query parameter
// descriptions never reach the published catalog schema at all (neither
// cmd/gen_action_inputs's generated struct nor google/jsonschema-go's
// reflector carries one — see ShapeFromCatalog's field-level doc comment),
// which is a real, structural gap in what the generator emits today,
// independent of any spec drift, and outside what Task 1 exists to fix. It
// is a genuine finding worth carrying into Task 2's design, not a defect in
// this comparison engine — the engine reports it precisely because it is
// real.
func TestUnit_ShapeFromCatalogAndShapeFromSpec_AgreeStructurallyForPilotDomain(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		domain      string
		specs       []toolutil.ActionSpec
		operationID string
	}{
		{"tags", tags.Specs(), "TagCreate"},
		{"tags", tags.Specs(), "TagDelete"},
		{"registries", registries.Specs(), "RegistryInspect"},
	} {
		t.Run(tc.domain+"/"+tc.operationID, func(t *testing.T) {
			t.Parallel()
			catalogShape, err := ShapeFromCatalog(findSpec(t, tc.specs, tc.operationID))
			if err != nil {
				t.Fatalf("ShapeFromCatalog() error = %v", err)
			}
			specShape := realShapeFromVendoredSpec(t, tc.operationID)

			var structural []FieldChange
			for _, c := range Compare(catalogShape, specShape) {
				if c.Kind != ChangeDescription {
					structural = append(structural, c)
				}
			}
			if len(structural) != 0 {
				t.Errorf("Compare(catalog, spec) structural changes = %v, want none for an operation nothing has changed for", structural)
			}
		})
	}
}
