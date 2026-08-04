package specdiff

import (
	"os"
	"strings"
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

// TestUnit_ShapeFromCatalog_DerivesTitleAndDescriptionFromTheActionSpec
// proves the operation-level fields come straight from spec.Title/
// spec.Description, unmodified — TagCreate's hand-authored ActionSpec
// literal (internal/tools/tags/tags.go) carries deliberately different
// prose from the vendored specification's own summary/description, and
// ShapeFromCatalog must publish exactly that hand-authored text, not
// re-derive anything from the spec itself.
func TestUnit_ShapeFromCatalog_DerivesTitleAndDescriptionFromTheActionSpec(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, tags.Specs(), "TagCreate")

	shape, err := ShapeFromCatalog(spec)
	if err != nil {
		t.Fatalf("ShapeFromCatalog() error = %v", err)
	}
	if shape.Title != spec.Title {
		t.Errorf("Title = %q, want spec.Title %q", shape.Title, spec.Title)
	}
	if shape.Description != spec.Description {
		t.Errorf("Description = %q, want spec.Description %q", shape.Description, spec.Description)
	}
	if shape.Title == "" || shape.Description == "" {
		t.Fatalf("Title/Description = %q/%q, want this fixture's action to carry both (TagCreate's own ActionSpec literal always sets them)", shape.Title, shape.Description)
	}
}

// TestUnit_ShapeFromCatalog_CarriesOverriddenFlagsFromActionSpec proves
// ShapeFromCatalog propagates toolutil.ActionSpec.TitleOverridden/
// DescriptionOverridden rather than dropping them: TagCreate is a real,
// production action built through toolutil.WithNarrative (tags/tags.go's
// narrative() hook), so both must be true here, and Compare's own test
// (TestUnit_Compare_TitleChange_CarriesAfterOverridden) is what proves the
// rest of the chain — this is the one link that would silently break the
// whole mechanism if ShapeFromCatalog ever stopped reading them.
func TestUnit_ShapeFromCatalog_CarriesOverriddenFlagsFromActionSpec(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, tags.Specs(), "TagCreate")
	if !spec.TitleOverridden || !spec.DescriptionOverridden {
		t.Fatalf("fixture premise: TagCreate's TitleOverridden/DescriptionOverridden = %v/%v, want both true (tags.go's narrative() hook sets both)",
			spec.TitleOverridden, spec.DescriptionOverridden)
	}

	shape, err := ShapeFromCatalog(spec)
	if err != nil {
		t.Fatalf("ShapeFromCatalog() error = %v", err)
	}
	if !shape.TitleOverridden {
		t.Error("TitleOverridden = false, want true: ShapeFromCatalog must carry it through from the ActionSpec")
	}
	if !shape.DescriptionOverridden {
		t.Error("DescriptionOverridden = false, want true: ShapeFromCatalog must carry it through from the ActionSpec")
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

// TestUnit_ShapeFromSpec_DerivesTitleAndStripsAccessPolicyFromDescription is
// the real-world proof that ShapeFromSpec's Title/Description agree with
// what cmd/gen_action_inputs's cleanTitleAndDescription would derive from
// the identical real operation: RegistryInspect's own vendored description
// ends with a "**Access policy**: restricted" line (verified directly
// against api/specs/ee-2.44.0.json), which must not survive into
// Description, and Title must be the vendored "summary" exactly.
func TestUnit_ShapeFromSpec_DerivesTitleAndStripsAccessPolicyFromDescription(t *testing.T) {
	t.Parallel()
	shape := realShapeFromVendoredSpec(t, "RegistryInspect")
	if shape.Title != "Inspect a registry" {
		t.Errorf("Title = %q, want the vendored summary verbatim", shape.Title)
	}
	if strings.Contains(strings.ToLower(shape.Description), "access policy") {
		t.Errorf("Description = %q, must not still mention access policy", shape.Description)
	}
	want := "Retrieve details about a registry. If endpointId is provided, applies policy overrides for that environment."
	if shape.Description != want {
		t.Errorf("Description = %q, want %q", shape.Description, want)
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

// TestUnit_ShapeFromSpec_RefusesNonObjectRequestBody is C1's regression
// guard: DeleteKubernetesNamespace's real body in api/specs/ee-2.44.0.json is
// `{"type":"array","items":{"type":"string"}}` — a required list of
// namespace names — and before this test existed ShapeFromSpec read only
// resolved.Properties, which is empty for an array, silently producing zero
// fields with no error at all. That made a drift audit compare "zero body
// fields" (this function, reporting nothing wrong) against "zero body
// fields" (the catalog, for an operation this shape has never reached) and
// print "no drift" for a reason indistinguishable from "nothing was actually
// compared" — the exact failure mode this package's own canaries exist to
// catch, just one this package's own producer could still cause. This
// mirrors cmd/gen_action_inputs/fields.go's identical refusal (same
// condition, "top-level type %q is not an object"), so an operation the
// generator would refuse to scaffold is never silently reported clean here
// either.
func TestUnit_ShapeFromSpec_RefusesNonObjectRequestBody(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	op, err := LoadSpecOperation(data, "DeleteKubernetesNamespace")
	if err != nil {
		t.Fatalf("LoadSpecOperation(DeleteKubernetesNamespace) error = %v", err)
	}

	shape, err := ShapeFromSpec(op)
	if err == nil {
		t.Fatalf("ShapeFromSpec(DeleteKubernetesNamespace) error = nil, shape = %+v, want a refusal: this operation's body is a top-level array, not an object, so it has no top-level fields to flatten", shape)
	}
	if !strings.Contains(err.Error(), "is not an object") {
		t.Errorf("ShapeFromSpec(DeleteKubernetesNamespace) error = %q, want it to say the top-level type is not an object", err)
	}
}

// TestUnit_ShapeFromSpec_RefusesNonObjectRequestBody_SyntheticShapes covers
// the two other non-object top-level types JSON Schema allows for a request
// body — "string" and "boolean" — against a minimal synthetic document
// rather than the vendored spec, so this does not depend on either vendored
// specification happening to contain an example of each. Both must refuse
// for the identical reason DeleteKubernetesNamespace's real array body does:
// resolved.Properties is empty for either, and reading only that field would
// silently report zero fields rather than refuse.
func TestUnit_ShapeFromSpec_RefusesNonObjectRequestBody_SyntheticShapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		bodyType string
	}{
		{"string body", "string"},
		{"boolean body", "boolean"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := []byte(`{
				"paths": {
					"/x": {
						"post": {
							"operationId": "SyntheticOp",
							"requestBody": {
								"content": {
									"application/json": {
										"schema": {"type": "` + tc.bodyType + `"}
									}
								}
							}
						}
					}
				},
				"components": {"schemas": {}}
			}`)
			op, err := LoadSpecOperation(doc, "SyntheticOp")
			if err != nil {
				t.Fatalf("LoadSpecOperation error = %v", err)
			}
			shape, err := ShapeFromSpec(op)
			if err == nil {
				t.Fatalf("ShapeFromSpec() error = nil, shape = %+v, want a refusal for a top-level %s body", shape, tc.bodyType)
			}
			if !strings.Contains(err.Error(), "is not an object") {
				t.Errorf("ShapeFromSpec() error = %q, want it to say the top-level type is not an object", err)
			}
		})
	}
}

// kubernetesBulkDeleteOperationIDs is all seven map-bodied Kubernetes
// bulk-delete operations in the vendored specification (fields.go's own
// MapValue doc comment names them): "a map where the key is the namespace
// and the value is an array of <kind> to delete" — the shape C2 (this
// coordinator round) closes ShapeFromSpec's blind spot for. Named directly,
// not discovered by a scan, so this test's own list cannot silently shrink
// if a future spec revision renames one of them without this test noticing.
var kubernetesBulkDeleteOperationIDs = []string{
	"DeleteCronJobs", "DeleteJobs", "DeleteKubernetesIngresses",
	"DeleteKubernetesServices", "DeleteRoleBindings", "DeleteRoles", "DeleteServiceAccounts",
}

// TestUnit_ShapeFromSpec_MapBodiedKubernetesBulkDeletes_ReportTwoFields is
// the direction C1's own fix could not close: a map-shaped request body
// (JSON Schema "type":"object" with a typed "additionalProperties" and no
// named "properties") synthesizes a "namespace" field, exactly the shape
// cmd/gen_action_inputs's assembleOperationFields already does for these
// same seven operations. Before this fix, every one of them reported
// exactly one field (the "id" path parameter) — verified directly, this is
// the coordinator's own measurement — silently dropping the body entirely.
// Asserting len(Fields) == 2 is the load-bearing assertion here: a shape
// that dropped the namespace field but still had "id" would pass any
// assertion that only inspected byName["namespace"] without first checking
// how many fields exist at all, the identical discrimination gap C1's own
// non-object refusal exists to close on the array side.
func TestUnit_ShapeFromSpec_MapBodiedKubernetesBulkDeletes_ReportTwoFields(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	for _, operationID := range kubernetesBulkDeleteOperationIDs {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, err := LoadSpecOperation(data, operationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s) error = %v", operationID, err)
			}
			shape, err := ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(%s) error = %v, want the map body synthesized, not refused", operationID, err)
			}
			if len(shape.Fields) != 2 {
				t.Fatalf("ShapeFromSpec(%s).Fields = %+v, want exactly 2 (id path parameter + namespace body field)", operationID, shape.Fields)
			}

			byName := make(map[string]FieldShape, len(shape.Fields))
			for _, f := range shape.Fields {
				byName[f.JSONName] = f
			}

			id, ok := byName["id"]
			if !ok || id.Type != "integer" || !id.Required || id.Origin != "path" {
				t.Errorf(`Fields["id"] = %+v, want {Type: integer, Required: true, Origin: path}`, id)
			}

			namespace, ok := byName["namespace"]
			if !ok {
				t.Fatalf("no \"namespace\" field in %+v; the map-typed body was silently discarded", shape.Fields)
			}
			if namespace.Type != "object" {
				t.Errorf("namespace.Type = %q, want %q: a map's own JSON Schema type, regardless of its values' shape", namespace.Type, "object")
			}
			if !namespace.Required {
				t.Error("namespace.Required = false, want true: every one of these seven operations declares requestBody.required = true")
			}
			if namespace.Origin != "body" {
				t.Errorf("namespace.Origin = %q, want %q", namespace.Origin, "body")
			}
			if namespace.Description == "" {
				t.Error("namespace.Description is empty, want the requestBody's own description (or the generator's fallback) carried through")
			}
		})
	}
}

// The four structs below are hand-written to match, field for field and tag
// for tag, exactly what cmd/gen_action_inputs would generate for these four
// operations today — the same shape
// TestUnit_MapTypedRequestBody_GeneratesMapField
// (cmd/gen_action_inputs/generate_test.go) already pins for
// DeleteServiceAccounts's identical body shape, reproduced here for the
// four operations the coordinator explicitly measured. This is not a
// fixture invented for this test to pass; every field name, JSON tag and
// jsonschema description is copied verbatim from api/specs/ee-2.44.0.json
// (verified by the two test functions above and below reading the identical
// document).
type deleteCronJobsInput struct {
	ID        int                 `json:"id" jsonschema:"Environment identifier"`
	Namespace map[string][]string `json:"namespace" jsonschema:"A map where the key is the namespace and the value is an array of Cron Jobs to delete"`
}

func (deleteCronJobsInput) MinimumParams() map[string]int { return map[string]int{"id": 1} }

type deleteJobsInput struct {
	ID        int                 `json:"id" jsonschema:"Environment identifier"`
	Namespace map[string][]string `json:"namespace" jsonschema:"A map where the key is the namespace and the value is an array of Jobs to delete"`
}

func (deleteJobsInput) MinimumParams() map[string]int { return map[string]int{"id": 1} }

type deleteRolesInput struct {
	ID        int                 `json:"id" jsonschema:"Environment identifier"`
	Namespace map[string][]string `json:"namespace" jsonschema:"A map where the key is the namespace and the value is an array of roles to delete"`
}

func (deleteRolesInput) MinimumParams() map[string]int { return map[string]int{"id": 1} }

type deleteKubernetesServicesInput struct {
	ID        int                 `json:"id" jsonschema:"Environment identifier"`
	Namespace map[string][]string `json:"namespace" jsonschema:"A map where the key is the namespace and the value is an array of services to delete"`
}

func (deleteKubernetesServicesInput) MinimumParams() map[string]int { return map[string]int{"id": 1} }

// TestUnit_ShapeFromCatalogAndShapeFromSpec_AgreeOnMapBodiedKubernetesBulkDeletes
// is the half that actually matters (the coordinator's own framing): a
// shape that reports the namespace field with the wrong type or origin
// still gates the build, just with a different message than "field
// missing". Each Input struct above is what the generator would emit for
// its operation today (mirroring TestUnit_MapTypedRequestBody_GeneratesMapField's
// identical DeleteServiceAccounts fixture), wrapped in a plain ActionSpec —
// no toolutil.WithNarrative override, no hand-improved Title/Description —
// exactly as a freshly scaffolded, never-hand-edited action would be
// declared. specdiff.Compare between that catalog shape and the real
// vendored operation must report zero changes: not merely "namespace is
// present on both sides", but present with the identical type, requiredness,
// origin and description-matching-or-excused-cosmetic on both.
func TestUnit_ShapeFromCatalogAndShapeFromSpec_AgreeOnMapBodiedKubernetesBulkDeletes(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	for _, tc := range []struct {
		operationID string
		title       string
		description string
		input       any
	}{
		{"DeleteCronJobs", "Delete Cron Jobs", "Delete the provided list of Cron Jobs.", deleteCronJobsInput{}},
		{"DeleteJobs", "Delete Jobs", "Delete the provided list of Jobs.", deleteJobsInput{}},
		{"DeleteRoles", "Delete roles", "Delete the provided list of roles.", deleteRolesInput{}},
		{"DeleteKubernetesServices", "Delete services", "Delete the provided list of services.", deleteKubernetesServicesInput{}},
	} {
		t.Run(tc.operationID, func(t *testing.T) {
			t.Parallel()
			op, err := LoadSpecOperation(data, tc.operationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s) error = %v", tc.operationID, err)
			}
			specShape, err := ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(%s) error = %v", tc.operationID, err)
			}

			catalogSpec := toolutil.ActionSpec{
				Name: "kubernetes." + tc.operationID, Domain: "kubernetes",
				OperationID: tc.operationID, Title: tc.title, Description: tc.description,
				Input: tc.input,
			}
			catalogShape, err := ShapeFromCatalog(catalogSpec)
			if err != nil {
				t.Fatalf("ShapeFromCatalog(%s) error = %v", tc.operationID, err)
			}

			changes := Compare(specShape, catalogShape)
			if len(changes) != 0 {
				t.Errorf("Compare() = %+v, want no drift: this catalog shape is exactly what the generator would emit for %s today", changes, tc.operationID)
			}
		})
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
//
// ChangeTitle and ChangeOperationDescription are excluded for the identical
// reason, discovered the same way: all three of these operations are
// hand-authored ActionSpec literals (internal/tools/tags/tags.go,
// internal/tools/registries/registries.go), not generated output, and their
// Title/Description are deliberately improved prose ("Create a tag" /
// "Creates a new environment tag with the given name." here, versus the
// spec's own "Create a new tag" / "Create a new tag.") — a real, permanent
// divergence from the specification's own wording, exactly the class of
// thing cmd/audit_spec_drift's allow-list exists to excuse for a parameter,
// now excused here the same way at the operation level too.
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
				if c.Kind == ChangeDescription || c.Kind == ChangeTitle || c.Kind == ChangeOperationDescription {
					continue
				}
				structural = append(structural, c)
			}
			if len(structural) != 0 {
				t.Errorf("Compare(catalog, spec) structural changes = %v, want none for an operation nothing has changed for", structural)
			}
		})
	}
}
