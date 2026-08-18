package specdiff

import (
	"os"
	"sort"
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

// multipartOperationFieldCounts names the five wave-1 multipart/form-data
// operations the coordinator measured directly against the vendored
// specification, and the field count cmd/gen_action_inputs's own
// assembleOperationFields already produces for each — the number
// ShapeFromSpec must now match, having previously reported 0, 0, 1, 1 and 2
// respectively (requestBodySchemaNode read only "application/json", so a
// multipart-only body flattened as no body at all; only each operation's own
// path/query parameters, if any, ever contributed a field). Verified
// independently against api/specs/ee-2.44.0.json for this test: EndpointCreate
// and CustomTemplateCreateFile declare no path or query parameters at all, so
// their entire count is body properties (27 and 10 respectively);
// StackCreateDockerSwarmFile/StackCreateDockerStandaloneFile each add one
// query parameter ("endpointId") to 4/3 body properties; EndpointDockerBrowsePut
// adds one path ("id") and one query ("volumeID") parameter to 2 body
// properties ("Path", "file").
var multipartOperationFieldCounts = map[string]int{
	"EndpointCreate":                  27,
	"CustomTemplateCreateFile":        10,
	"StackCreateDockerSwarmFile":      5,
	"StackCreateDockerStandaloneFile": 4,
	"EndpointDockerBrowsePut":         4,
}

// TestUnit_ShapeFromSpec_MultipartFormDataBody_MatchesGeneratorFieldCount is
// the regression guard for the multipart defect this round's fix closes:
// before it, every operation named in multipartOperationFieldCounts reported
// 0 (or, when the operation also has path/query parameters, only those)
// fields, because requestBodySchemaNode looked only at "application/json"
// content and treated a multipart-only body as no body at all. Asserting the
// exact count the generator itself would emit (not merely "> 0", which a
// single stray field would also satisfy) is the load-bearing check: a count
// off by even one still means a real field silently missing (or a spurious
// one added) from a drift comparison that is supposed to catch exactly that.
func TestUnit_ShapeFromSpec_MultipartFormDataBody_MatchesGeneratorFieldCount(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("read vendored spec: %v", err)
	}
	// Sorted operation IDs for deterministic subtest order.
	operationIDs := make([]string, 0, len(multipartOperationFieldCounts))
	for id := range multipartOperationFieldCounts {
		operationIDs = append(operationIDs, id)
	}
	sort.Strings(operationIDs)
	for _, operationID := range operationIDs {
		wantFields := multipartOperationFieldCounts[operationID]
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, err := LoadSpecOperation(data, operationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s) error = %v", operationID, err)
			}
			shape, err := ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(%s) error = %v, want the multipart/form-data body resolved like any other object body", operationID, err)
			}
			if len(shape.Fields) != wantFields {
				t.Errorf("ShapeFromSpec(%s).Fields = %+v (%d), want exactly %d — the generator's own count for this operation",
					operationID, shape.Fields, len(shape.Fields), wantFields)
			}
		})
	}
}

// TestUnit_ShapeFromSpec_MultipartFormDataBody_ResolvesFileUploadField is the
// discriminating half of the guard above: a field-count match alone could be
// satisfied by an implementation that resolved the multipart schema into the
// right *number* of fields but the wrong ones (e.g. reading query parameters
// twice, or synthesizing placeholder fields rather than the body's actual
// properties). EndpointDockerBrowsePut's own body declares exactly two
// properties, "Path" (a string) and "file" (a file upload, declared
// `"type":"string","format":"binary"` — format is not a JSON Schema "type"
// keyword, so this must still resolve as an ordinary string field, not be
// dropped or mistyped), both required. Asserting their JSONName, Type,
// Required and Origin directly is what a bare field count could not catch.
func TestUnit_ShapeFromSpec_MultipartFormDataBody_ResolvesFileUploadField(t *testing.T) {
	t.Parallel()
	shape := realShapeFromVendoredSpec(t, "EndpointDockerBrowsePut")

	byName := make(map[string]FieldShape, len(shape.Fields))
	for _, f := range shape.Fields {
		byName[f.JSONName] = f
	}

	for _, tc := range []struct {
		name         string
		wantType     string // "" when Type is not asserted for this field
		wantRequired bool
		wantOrigin   string
	}{
		{"file", "string", true, "body"},
		{"path", "string", true, "body"},
		{"id", "integer", true, "path"},
		{"volumeID", "", false, "query"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			field, ok := byName[tc.name]
			if !ok {
				t.Fatalf("Fields[%q] not found", tc.name)
			}
			if tc.wantType != "" && field.Type != tc.wantType {
				t.Errorf("Fields[%q].Type = %q, want %q", tc.name, field.Type, tc.wantType)
			}
			if field.Required != tc.wantRequired {
				t.Errorf("Fields[%q].Required = %v, want %v", tc.name, field.Required, tc.wantRequired)
			}
			if field.Origin != tc.wantOrigin {
				t.Errorf("Fields[%q].Origin = %q, want %q", tc.name, field.Origin, tc.wantOrigin)
			}
		})
	}
}

// TestUnit_ShapeFromSpec_RefusesMultipleContentTypes mirrors
// cmd/gen_action_inputs's requestBodySchema's identical refusal
// ("no single Go type represents both") for the one operation in either
// vendored specification whose requestBody declares more than one content
// type: CustomTemplateCreate (application/json and multipart/form-data, EE
// spec only). Before this round's fix, requestBodySchemaNode silently picked
// the "application/json" entry alone and happened to land on a free-form
// object ShapeFromSpec already refused for an unrelated reason (see
// cmd/gen_action_inputs/fieldcount_crosscheck_test.go's own doc comment on
// this exact operation); now that requestBodySchemaNode reads whichever
// single content type is present, it must refuse outright, the same way the
// generator does, rather than pick either entry arbitrarily — picking one
// silently would mean a real future disagreement between the two entries'
// shapes goes uncompared.
func TestUnit_ShapeFromSpec_RefusesMultipleContentTypes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		operationID string
	}{
		{"CustomTemplateCreate", "CustomTemplateCreate"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile("../../api/specs/ee-2.44.0.json")
			if err != nil {
				t.Fatalf("read vendored spec: %v", err)
			}
			op, err := LoadSpecOperation(data, tc.operationID)
			if err != nil {
				t.Fatalf("LoadSpecOperation(%s) error = %v", tc.operationID, err)
			}
			shape, err := ShapeFromSpec(op)
			if err == nil {
				t.Fatalf("ShapeFromSpec(%s) error = nil, shape = %+v, want a refusal: this operation's requestBody declares two content types, and no single schema can represent both", tc.operationID, shape)
			}
			if !strings.Contains(err.Error(), "content type") {
				t.Errorf("ShapeFromSpec(%s) error = %q, want it to mention the multiple content types", tc.operationID, err)
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

// TestUnit_ShapeFromSpec_StackMigrate_DisambiguatesTheQueryParameterFromTheBodyProperty
// is the operation this project's disambiguation rule was written for, shaped
// against both real vendored documents rather than a fixture: POST
// /stacks/{id}/migrate declares an optional query parameter "endpointId" and
// a required body property "EndpointID", which bodyJSONTag renders to the
// same "endpointId". ShapeFromSpec used to refuse the whole operation, and
// because cmd/audit_spec_drift turns that refusal into a returned error
// before any FieldChange exists, no api/spec-drift-allowlist.yaml entry could
// excuse it and the audit failed for every domain in the catalog, not only
// for stacks.
//
// The specific names are asserted, not merely the absence of an error: the
// whole point of the rule is that both this package and cmd/gen_action_inputs
// produce the *same* names, and a test that only checked "no error" would
// pass just as happily against a rule that qualified the body instead of the
// parameter — which would put the catalog and the specification permanently
// one added and one removed field apart.
func TestUnit_ShapeFromSpec_StackMigrate_DisambiguatesTheQueryParameterFromTheBodyProperty(t *testing.T) {
	t.Parallel()
	for _, specPath := range []string{"../../api/specs/ee-2.44.0.json", "../../api/specs/ce-2.44.0.json"} {
		t.Run(specPath, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(specPath)
			if err != nil {
				t.Fatalf("read vendored spec: %v", err)
			}
			op, err := LoadSpecOperation(data, "StackMigrate")
			if err != nil {
				t.Fatalf("LoadSpecOperation(StackMigrate) error = %v", err)
			}
			shape, err := ShapeFromSpec(op)
			if err != nil {
				t.Fatalf("ShapeFromSpec(StackMigrate) error = %v, want the collision disambiguated, not refused", err)
			}

			byName := make(map[string]FieldShape, len(shape.Fields))
			for _, f := range shape.Fields {
				byName[f.JSONName] = f
			}

			body, ok := byName["endpointId"]
			if !ok {
				t.Fatalf("no \"endpointId\" field in %+v; the body property must keep the plain name", shape.Fields)
			}
			if body.Origin != "body" {
				t.Errorf(`Fields["endpointId"].Origin = %q, want "body": the required body property that names the migration target keeps the plain name, not the optional pre-1.18 query parameter`, body.Origin)
			}
			if !body.Required || body.Type != "integer" {
				t.Errorf(`Fields["endpointId"] = %+v, want {Type: integer, Required: true}`, body)
			}

			query, ok := byName["endpointIdQuery"]
			if !ok {
				t.Fatalf("no \"endpointIdQuery\" field in %+v; the query parameter must survive under its origin-qualified name, not be dropped", shape.Fields)
			}
			if query.Origin != "query" {
				t.Errorf(`Fields["endpointIdQuery"].Origin = %q, want "query"`, query.Origin)
			}
			if query.Required {
				t.Error(`Fields["endpointIdQuery"].Required = true, want false: the specification declares this parameter optional`)
			}
			if !strings.Contains(query.Description, "1.18") {
				t.Errorf(`Fields["endpointIdQuery"].Description = %q, want the parameter's own description (the pre-1.18 fixup), not the body property's`, query.Description)
			}

			// The path parameter, and every body property that never
			// collided, are untouched: a rule that renamed more than it had
			// to would drift every one of them against the catalog.
			for _, want := range []string{"id", "name", "swarmId"} {
				if _, ok := byName[want]; !ok {
					t.Errorf("no %q field in %+v; disambiguation renamed a field that never collided", want, shape.Fields)
				}
			}
		})
	}
}

// TestUnit_ShapeFromSpec_TwoBodyPropertiesRenderingToOneTag_StillRefuses is
// the half of the collision refusal that must survive the disambiguation
// rule. bodyJSONTag is not injective, so two distinct specification property
// names can render to one JSON tag; unlike a parameter colliding with a body
// property, there is no principled winner between them and no origin to
// qualify either by, so this stays a refusal. Synthetic rather than
// spec-derived because neither vendored document contains this shape today —
// which is exactly why nothing would notice if the refusal quietly stopped
// firing.
func TestUnit_ShapeFromSpec_TwoBodyPropertiesRenderingToOneTag_StillRefuses(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"paths": {
			"/x": {
				"post": {
					"operationId": "SyntheticCollide",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"EndpointID": {"type": "integer"},
										"endpointId": {"type": "integer"}
									}
								}
							}
						}
					}
				}
			}
		},
		"components": {"schemas": {}}
	}`)
	op, err := LoadSpecOperation(doc, "SyntheticCollide")
	if err != nil {
		t.Fatalf("LoadSpecOperation error = %v", err)
	}
	shape, err := ShapeFromSpec(op)
	if err == nil {
		t.Fatalf("ShapeFromSpec() error = nil, shape = %+v, want a refusal: two body properties rendering to one JSON tag have no principled winner", shape)
	}
	if !strings.Contains(err.Error(), "both render as JSON field") {
		t.Errorf("ShapeFromSpec() error = %q, want the body-property collision refusal, not some other failure", err)
	}
}

// TestUnit_ShapeFromSpec_QualifiedNameAlreadyTaken_Refuses covers the one
// cross-origin collision the disambiguation rule declines to resolve: the
// name it would rename the parameter to is already contributed by a third
// field, so renaming into it would shadow that field instead — the same
// defect one step removed.
func TestUnit_ShapeFromSpec_QualifiedNameAlreadyTaken_Refuses(t *testing.T) {
	t.Parallel()
	doc := []byte(`{
		"paths": {
			"/x/{namespace}": {
				"post": {
					"operationId": "SyntheticTaken",
					"parameters": [
						{"name": "namespace", "in": "path", "required": true, "schema": {"type": "string"}}
					],
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"Namespace": {"type": "string"},
										"NamespacePath": {"type": "string"}
									}
								}
							}
						}
					}
				}
			}
		},
		"components": {"schemas": {}}
	}`)
	op, err := LoadSpecOperation(doc, "SyntheticTaken")
	if err != nil {
		t.Fatalf("LoadSpecOperation error = %v", err)
	}
	shape, err := ShapeFromSpec(op)
	if err == nil {
		t.Fatalf("ShapeFromSpec() error = nil, shape = %+v, want a refusal: \"namespacePath\" is already a body property", shape)
	}
	if !strings.Contains(err.Error(), "already contributed by another field") {
		t.Errorf("ShapeFromSpec() error = %q, want the refusal internal/specnaming raises when a qualified name is taken", err)
	}
}
