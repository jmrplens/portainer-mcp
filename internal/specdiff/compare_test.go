package specdiff

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// withField returns a copy of base with f substituted for the existing field
// sharing f.JSONName, or appended when no field of that name exists yet. It
// exists purely to build the "after" side of each
// TestUnit_Compare_DetectsEveryChangeKind case without repeating base's other
// fields.
func withField(base OperationShape, f FieldShape) OperationShape {
	out := base
	out.Fields = make([]FieldShape, len(base.Fields))
	copy(out.Fields, base.Fields)
	for i, existing := range out.Fields {
		if existing.JSONName == f.JSONName {
			out.Fields[i] = f
			return out
		}
	}
	out.Fields = append(out.Fields, f)
	return out
}

// shapeFromSpecFile loads the named operation from a vendored (or, for a
// test fixture, a trimmed) OpenAPI document on disk and flattens it through
// ShapeFromSpec, failing the test on any error along the way.
func shapeFromSpecFile(t *testing.T, path, operationID string) OperationShape {
	t.Helper()
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	op, err := LoadSpecOperation(data, operationID)
	if err != nil {
		t.Fatalf("LoadSpecOperation(%s, %q) error = %v", path, operationID, err)
	}
	shape, err := ShapeFromSpec(op)
	if err != nil {
		t.Fatalf("ShapeFromSpec(%q) error = %v", operationID, err)
	}
	return shape
}

// realShapeFromVendoredSpec loads operationID from the vendored Business
// Edition specification this repository already ships in api/specs/ — real,
// many-fielded operation shapes, not a hand-built fixture that might be
// trivially empty (an empty OperationShape would make
// TestUnit_Compare_IdenticalShapes_ReportNothing pass regardless of whether
// Compare actually compares anything).
func realShapeFromVendoredSpec(t *testing.T, operationID string) OperationShape {
	t.Helper()
	return shapeFromSpecFile(t, "../../api/specs/ee-2.44.0.json", operationID)
}

func TestUnit_Compare_DetectsEveryChangeKind(t *testing.T) {
	t.Parallel()
	// One case per kind, each differing from the baseline in exactly one way,
	// so a Compare that reports a blanket "changed" cannot pass. Title and
	// Description are set on base (and left untouched by every case below
	// except the two that test them) so a spurious operation-level finding
	// cannot sneak into an otherwise single-field case and inflate len(got).
	base := OperationShape{OperationID: "X", Method: "GET", Path: "/x", Title: "Do the X thing", Description: "Does the X thing to the named resource.", Fields: []FieldShape{
		{JSONName: "id", Type: "integer", Required: true, Origin: "path", Description: "The id"},
	}}
	for _, tc := range []struct {
		name  string
		after OperationShape
		want  ChangeKind
	}{
		{"type swapped", withField(base, FieldShape{JSONName: "id", Type: "string", Required: true, Origin: "path", Description: "The id"}), ChangeType},
		{"became optional", withField(base, FieldShape{JSONName: "id", Type: "integer", Required: false, Origin: "path", Description: "The id"}), ChangeRequiredness},
		{"moved to query", withField(base, FieldShape{JSONName: "id", Type: "integer", Required: true, Origin: "query", Description: "The id"}), ChangeOrigin},
		{"reworded", withField(base, FieldShape{JSONName: "id", Type: "integer", Required: true, Origin: "path", Description: "Identifier"}), ChangeDescription},
		{"removed", withTitleAndDescription(OperationShape{OperationID: "X", Method: "GET", Path: "/x"}, base.Title, base.Description), ChangeRemoved},
		{"title reworded", withTitle(base, "Do the Y thing"), ChangeTitle},
		{"operation description reworded", withDescription(base, "Does the X thing, but described differently."), ChangeOperationDescription},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Compare(base, tc.after)
			if len(got) != 1 {
				t.Fatalf("Compare() = %v, want exactly one change", got)
			}
			if got[0].Kind != tc.want {
				t.Errorf("Kind = %v, want %v", got[0].Kind, tc.want)
			}
		})
	}
}

// withTitle and withDescription return a copy of base with only Title or
// only Description replaced — the operation-level equivalent of withField,
// so a table case that means to exercise ChangeTitle in isolation cannot
// incidentally also change Description (or vice versa) and mask which kind
// actually fired. withTitleAndDescription sets both at once, used only to
// build the "removed" case's baseline Title/Description so that case's
// single field removal is not itself accompanied by a spurious Title/
// Description diff against base.
func withTitle(base OperationShape, title string) OperationShape {
	out := base
	out.Title = title
	return out
}

func withDescription(base OperationShape, description string) OperationShape {
	out := base
	out.Description = description
	return out
}

func withTitleAndDescription(base OperationShape, title, description string) OperationShape {
	out := base
	out.Title = title
	out.Description = description
	return out
}

func TestUnit_Compare_AddedField_ReportsChangeAdded(t *testing.T) {
	t.Parallel()
	// The mirror image of "removed" in TestUnit_Compare_DetectsEveryChangeKind:
	// a field present only in after, not before.
	before := OperationShape{OperationID: "X", Method: "GET", Path: "/x", Fields: []FieldShape{
		{JSONName: "id", Type: "integer", Required: true, Origin: "path"},
	}}
	after := withField(before, FieldShape{JSONName: "limit", Type: "integer", Required: false, Origin: "query"})

	got := Compare(before, after)
	if len(got) != 1 {
		t.Fatalf("Compare() = %v, want exactly one change", got)
	}
	if got[0].Kind != ChangeAdded || got[0].JSONName != "limit" {
		t.Errorf("Compare() = %+v, want one ChangeAdded for \"limit\"", got[0])
	}
}

func TestUnit_Compare_EnumSetChanged_ReportsChangeEnum(t *testing.T) {
	t.Parallel()
	before := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "status", Type: "integer", Enum: []string{"0", "1", "2"}},
	}}
	after := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "status", Type: "integer", Enum: []string{"0", "1", "2", "3"}},
	}}
	got := Compare(before, after)
	if len(got) != 1 || got[0].Kind != ChangeEnum {
		t.Fatalf("Compare() = %v, want exactly one ChangeEnum", got)
	}
}

func TestUnit_Compare_EnumSameSetDifferentOrder_ReportsNothing(t *testing.T) {
	t.Parallel()
	// The vendored specification's own enum arrays have been observed to
	// reorder harmlessly across versions (see
	// plan/research/version-delta-analysis.md); what is model-facing is the
	// set of allowed values, not their declaration order.
	before := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "status", Type: "integer", Enum: []string{"0", "1", "2"}},
	}}
	after := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "status", Type: "integer", Enum: []string{"2", "0", "1"}},
	}}
	if got := Compare(before, after); len(got) != 0 {
		t.Errorf("Compare() = %v, want no changes for a reordered but identical enum set", got)
	}
}

func TestUnit_Compare_OriginUnknownOnOneSide_DoesNotReportChangeOrigin(t *testing.T) {
	t.Parallel()
	// ShapeFromCatalog cannot always state a field's origin (see
	// FieldShape.Origin's doc comment): comparing a catalog-derived shape
	// against a spec-derived one must not manufacture a ChangeOrigin for
	// every single field just because one side left Origin "".
	before := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "id", Type: "integer", Required: true, Origin: ""},
	}}
	after := OperationShape{OperationID: "X", Fields: []FieldShape{
		{JSONName: "id", Type: "integer", Required: true, Origin: "path"},
	}}
	if got := Compare(before, after); len(got) != 0 {
		t.Errorf("Compare() = %v, want no changes when only one side states an origin", got)
	}
}

func TestUnit_Compare_IdenticalShapes_ReportNothing(t *testing.T) {
	t.Parallel()
	// The half that makes the other half meaningful. An engine that reports
	// every field as changed would pass every test above.
	base := realShapeFromVendoredSpec(t, "TagCreate")
	if len(base.Fields) == 0 {
		t.Fatal("realShapeFromVendoredSpec(TagCreate) produced no fields; this test needs a real, non-trivial shape to be meaningful")
	}
	if got := Compare(base, base); len(got) != 0 {
		t.Errorf("Compare(x, x) = %v, want no changes", got)
	}
}

func TestUnit_Compare_TheRealEndpointListTypeSwap_IsDetected(t *testing.T) {
	t.Parallel()
	// A change that actually happened between two published versions:
	// EndpointList's "order" and "edgeStackStatus" query parameters swapped
	// types. It broke nothing and no audit saw it. This is the case the whole
	// phase exists for, so it is pinned against the real specifications rather
	// than a fixture.
	before := shapeFromSpecFile(t, "testdata/ee-2.42.0-endpointlist.json", "EndpointList")
	after := shapeFromSpecFile(t, "testdata/ee-2.43.0-endpointlist.json", "EndpointList")

	changes := Compare(before, after)
	var typeChanges []string
	for _, c := range changes {
		if c.Kind == ChangeType {
			typeChanges = append(typeChanges, c.JSONName)
		}
	}
	if !slices.Contains(typeChanges, "order") {
		t.Errorf("type changes = %v, want the real swap on \"order\" to be detected", typeChanges)
	}
	if !slices.Contains(typeChanges, "edgeStackStatus") {
		t.Errorf("type changes = %v, want the real swap on \"edgeStackStatus\" to be detected", typeChanges)
	}
}
