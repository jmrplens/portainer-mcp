package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"slices"
	"strings"
	"testing"
)

// mustCompile parses and type-checks generated Go source, failing the test
// if it does not. A test that only greps for substrings in generated text
// would happily pass against output that fails to compile — exactly the
// trap this project's standing warning calls out — so every scenario below
// that expects the generator to succeed runs its output through this rather
// than relying on string assertions alone.
func mustCompile(t *testing.T, src []byte) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", src, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("generated", fset, []*ast.File{file}, nil); err != nil {
		t.Fatalf("generated source does not type-check: %v\n%s", err, src)
	}
}

// newDoc builds a document whose components.schemas is schemas, for tests
// that need $ref resolution.
func newDoc(schemas map[string]any) *document {
	return &document{schemas: schemas, requestBodies: map[string]any{}}
}

func TestUnit_RequiredPathParameter_GeneratesNonPointerFieldWithNoOmitempty(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "TagDelete",
		Method:      "DELETE",
		Path:        "/tags/{id}",
		Parameters: []map[string]any{
			{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}, "description": "Tag identifier"},
		},
	}
	res := &resolver{doc: newDoc(nil)}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, newDoc(nil), "tagDeleteInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	if len(fields) != 1 {
		t.Fatalf("fields = %+v, want exactly one", fields)
	}
	f := fields[0]
	if f.GoName != "ID" || f.GoType != "int" || f.JSONName != "id" || !f.Required {
		t.Errorf("field = %+v, want {GoName:ID GoType:int JSONName:id Required:true}", f)
	}

	src, err := renderFile("tags", "test-spec.json", []structSpec{{Name: "tagDeleteInput", Fields: fields}})
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	text := string(src)
	if !strings.Contains(text, `ID int `+"`"+`json:"id" jsonschema:"Tag identifier"`+"`") {
		t.Errorf("generated source does not declare a required, non-pointer, described ID field:\n%s", text)
	}
	if strings.Contains(text, `"id,omitempty"`) {
		t.Errorf("a required path parameter must never carry omitempty:\n%s", text)
	}
}

func TestUnit_OptionalQueryParameter_GeneratesPointerFieldWithOmitempty(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "RegistryInspect",
		Method:      "GET",
		Path:        "/registries/{id}",
		Parameters: []map[string]any{
			{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			{"name": "endpointId", "in": "query", "schema": map[string]any{"type": "integer"}, "description": "Environment identifier"},
		},
	}
	res := &resolver{doc: newDoc(nil)}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, newDoc(nil), "registryInspectInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	var endpointField *fieldSpec
	for i := range fields {
		if fields[i].JSONName == "endpointId" {
			endpointField = &fields[i]
		}
	}
	if endpointField == nil {
		t.Fatalf("no endpointId field in %+v", fields)
	}
	if endpointField.GoName != "EndpointID" || endpointField.GoType != "*int" || endpointField.Required {
		t.Errorf("endpointId field = %+v, want {GoName:EndpointID GoType:*int Required:false}", *endpointField)
	}

	src, err := renderFile("registries", "test-spec.json", []structSpec{{Name: "registryInspectInput", Fields: fields}})
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	if !strings.Contains(string(src), `EndpointID *int `+"`"+`json:"endpointId,omitempty" jsonschema:"Environment identifier"`+"`") {
		t.Errorf("generated source does not declare an optional, pointer, described EndpointID field:\n%s", src)
	}
}

func TestUnit_Enum_GeneratesEnumParamsMethodWithSpecValues(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "RegistryPing",
		Method:      "POST",
		Path:        "/registries/ping",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"type": map[string]any{"type": "integer", "enum": []any{float64(1), float64(2), float64(3)}},
						},
						"required": []any{"type"},
					},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, "registryPingInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}

	src, err := renderFile("registries", "test-spec.json", []structSpec{{Name: "registryPingInput", Fields: fields}})
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	text := string(src)
	for _, want := range []string{
		"func (registryPingInput) EnumParams() map[string][]any {",
		`"type": {1, 2, 3}`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("generated source does not carry %q:\n%s", want, text)
		}
	}
}

func TestUnit_BodyObjectProperty_GeneratesNestedStruct(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "RegistryCreate",
		Method:      "POST",
		Path:        "/registries",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
							"ecr": map[string]any{
								"description": "ECR specific details",
								"type":        "object",
								"properties": map[string]any{
									"region": map[string]any{"type": "string"},
								},
							},
						},
						"required": []any{"name"},
					},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, "registryCreateInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	if len(nested) != 1 || nested[0].Name != "registryCreateInputEcr" {
		t.Fatalf("nested structs = %+v, want exactly one named registryCreateInputEcr", nested)
	}

	all := append([]structSpec{{Name: "registryCreateInput", Fields: fields}}, nested...)
	src, err := renderFile("registries", "test-spec.json", all)
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	text := string(src)
	if !strings.Contains(text, "type registryCreateInputEcr struct {") {
		t.Errorf("generated source does not declare the nested struct:\n%s", text)
	}
	if !strings.Contains(text, "Ecr") || !strings.Contains(text, "*registryCreateInputEcr") || !strings.Contains(text, `"ecr,omitempty"`) {
		t.Errorf("generated source does not reference the nested struct as an optional pointer field:\n%s", text)
	}
}

// TestUnit_Resolve_AllOfPropertiesSurviveASiblingPropertiesKeyword guards the
// allOf-overlay defect: a node can declare "properties" as a sibling of
// "allOf" ({"allOf": [{"$ref": "...Base"}], "properties": {...}} is the real
// shape), and resolve is supposed to overlay the node's own keywords on top
// of what allOf merged in — a correct precedence rule for scalar keywords
// like type or description. Before this fix, the properties overlay reset
// node.Properties to nil unconditionally, discarding every property the
// referenced Base schema contributed instead of adding to it, so the
// generated struct (and the published schema) silently lost every allOf-
// inherited field the moment the node also declared even one property of its
// own.
func TestUnit_Resolve_AllOfPropertiesSurviveASiblingPropertiesKeyword(t *testing.T) {
	t.Parallel()
	doc := newDoc(map[string]any{
		"Base": map[string]any{
			"type":       "object",
			"properties": map[string]any{"name": map[string]any{"type": "string"}},
			"required":   []any{"name"},
		},
	})
	res := &resolver{doc: doc}
	node, err := res.resolve(map[string]any{
		"allOf":      []any{map[string]any{"$ref": "#/components/schemas/Base"}},
		"properties": map[string]any{"extra": map[string]any{"type": "string"}},
	}, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	names := make([]string, 0, len(node.Properties))
	for _, p := range node.Properties {
		names = append(names, p.Name)
	}
	if !slices.Contains(names, "name") {
		t.Fatalf("resolve().Properties = %v, want \"name\" preserved from the allOf-merged Base schema", names)
	}
	if !slices.Contains(names, "extra") {
		t.Fatalf("resolve().Properties = %v, want \"extra\" from the node's own sibling \"properties\"", names)
	}
}

// TestUnit_Resolve_ArrayItemsFromAllOf_AreNotDiscarded guards the sibling
// defect in the same block: mergeSchema copies src.Items into the node, so
// an allOf branch can legitimately supply both "type": "array" and "items".
// Before this fix, the array-handling block read raw["items"] only,
// ignoring whatever mergeSchema had already set, and returned "array schema
// has no items" for a schema it had in fact already resolved correctly.
func TestUnit_Resolve_ArrayItemsFromAllOf_AreNotDiscarded(t *testing.T) {
	t.Parallel()
	doc := newDoc(map[string]any{
		"StringList": map[string]any{
			"type":  "array",
			"items": map[string]any{"type": "string"},
		},
	})
	res := &resolver{doc: doc}
	node, err := res.resolve(map[string]any{
		"allOf": []any{map[string]any{"$ref": "#/components/schemas/StringList"}},
	}, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v, want the allOf-merged Items to satisfy the array-items requirement", err)
	}
	if node.Items == nil || node.Items.Type != "string" {
		t.Fatalf("resolve().Items = %+v, want the string item type merged in from the allOf branch", node.Items)
	}
}

// TestUnit_Resolve_ArrayWithNoItemsAnywhere_IsStillRefused is the negative
// control: an array schema with no items at all, from either the node's own
// "items" keyword or an allOf-merged one, must still be refused.
func TestUnit_Resolve_ArrayWithNoItemsAnywhere_IsStillRefused(t *testing.T) {
	t.Parallel()
	res := &resolver{doc: newDoc(nil)}
	_, err := res.resolve(map[string]any{"type": "array"}, 0)
	if err == nil {
		t.Fatal("resolve() = nil error, want a refusal: an array schema with no items anywhere")
	}
	if !strings.Contains(err.Error(), "no items") {
		t.Errorf("resolve() error = %q, want it to say the array has no items", err)
	}
}

// TestUnit_MapTypedRequestBody_GeneratesMapField guards C2: a request body
// that resolves to an object with a typed additionalProperties and no named
// properties — the seven Kubernetes bulk-delete operations' real shape, "a
// map where the key is the namespace and the value is an array of X to
// delete" — must become a real map field, not silently contribute zero
// fields. Before the fix, assembleOperationFields called assembleFields
// directly on node.Properties (empty here) and never looked at
// node.MapValue at all, so this exact shape produced only the "id" path
// parameter and nothing for the body.
func TestUnit_MapTypedRequestBody_GeneratesMapField(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "DeleteServiceAccounts",
		Method:      "POST",
		Path:        "/kubernetes/{id}/service_accounts/delete",
		Parameters: []map[string]any{
			{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}, "description": "Environment identifier"},
		},
		RequestBody: map[string]any{
			"description": "A map where the key is the namespace and the value is an array of service accounts to delete",
			"required":    true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type":                 "object",
						"additionalProperties": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, "deleteServiceAccountsInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}

	var namespaceField *fieldSpec
	for i := range fields {
		if fields[i].JSONName == "namespace" {
			namespaceField = &fields[i]
		}
	}
	if namespaceField == nil {
		t.Fatalf("no namespace field in %+v; the map-typed body was silently discarded", fields)
	}
	if namespaceField.GoType != "map[string][]string" {
		t.Errorf("namespace field GoType = %q, want map[string][]string", namespaceField.GoType)
	}
	if !namespaceField.Required {
		t.Error("namespace field Required = false, want true: requestBody.required is true")
	}
	if namespaceField.Description == "" {
		t.Error("namespace field carries no description, want the requestBody's own description carried through")
	}

	src, err := renderFile("kubernetes", "test-spec.json", []structSpec{{Name: "deleteServiceAccountsInput", Fields: fields}})
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	if !strings.Contains(string(src), `Namespace map[string][]string `+"`"+`json:"namespace" jsonschema:"A map where the key is the namespace and the value is an array of service accounts to delete"`+"`") {
		t.Errorf("generated source does not declare the required, described Namespace map field:\n%s", src)
	}
}

// TestUnit_FreeFormRequestBody_RefusedThroughAssembleOperationFields is C2's
// other half: a request body that resolves to an object with neither
// properties nor a typed additionalProperties (PolicyCreate's and
// PolicyConflicts's real shape, `{"type": "object"}` with nothing else) must
// abort generation, not silently contribute zero fields. The existing
// free-form refusal test in TestUnit_Refusals calls typeOf directly, which
// is exactly why this defect went unnoticed: assembleOperationFields never
// called typeOf on the body node at all, so the refusal was unreachable from
// the only path a real operation takes. This test goes through
// assembleOperationFields, the real path, instead.
func TestUnit_FreeFormRequestBody_RefusedThroughAssembleOperationFields(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "PolicyCreate",
		Method:      "POST",
		Path:        "/policies",
		RequestBody: map[string]any{
			"description": "Policy details",
			"required":    true,
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{"type": "object"},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	_, _, err := assembleOperationFields(op, res, doc, "policyCreateInput", &nested)
	if err == nil {
		t.Fatal("assembleOperationFields() = nil error, want a refusal: a genuinely free-form required body must not silently contribute zero fields")
	}
	if !strings.Contains(err.Error(), "free-form") {
		t.Errorf("error = %q, want it to mention the free-form-object refusal", err)
	}
}

// TestUnit_NestedStructNameCollidesWithSiblingProperty guards I3: a body
// property "foo" (an object whose own property "bar" is itself an object)
// and a sibling property "fooBar" (also an object) both produce a nested
// struct literally named "...FooBar" — bare concatenation has no notion of
// this collision. duplicateStructName must catch it before rendering.
func TestUnit_NestedStructNameCollidesWithSiblingProperty(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "WidgetCreate",
		Method:      "POST",
		Path:        "/widgets",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"foo": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"bar": map[string]any{
										"type":       "object",
										"properties": map[string]any{"x": map[string]any{"type": "string"}},
									},
								},
							},
							"fooBar": map[string]any{
								"type":       "object",
								"properties": map[string]any{"y": map[string]any{"type": "string"}},
							},
						},
					},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	fields, _, err := assembleOperationFields(op, res, doc, "widgetCreateInput", &nested)
	if err != nil {
		t.Fatalf("assembleOperationFields() error = %v", err)
	}
	all := append([]structSpec{{Name: "widgetCreateInput", Fields: fields}}, nested...)

	name, dup := duplicateStructName(all)
	if !dup {
		t.Fatal("duplicateStructName() = false, want true: widgetCreateInputFooBar is declared by both foo.bar and the sibling fooBar")
	}
	if name != "widgetCreateInputFooBar" {
		t.Errorf("duplicateStructName() name = %q, want widgetCreateInputFooBar", name)
	}

	// Demonstrate why the check must run before rendering: go/format only
	// parses and reformats, so the two colliding (and differently shaped)
	// struct declarations format without error, and only go/types — the
	// same type-checking mustCompile does — rejects it as a redeclaration.
	src, err := renderFile("widgets", "test-spec.json", all)
	if err != nil {
		t.Fatalf("renderFile() error = %v, want go/format to succeed despite the collision", err)
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "generated.go", src, parser.AllErrors)
	if err != nil {
		t.Fatalf("generated source does not even parse: %v", err)
	}
	conf := types.Config{Importer: importer.Default()}
	if _, err := conf.Check("generated", fset, []*ast.File{file}, nil); err == nil {
		t.Fatal("type-check succeeded despite two identical struct declarations; expected a redeclaration error, proving go/format alone cannot catch this")
	}
}

// TestUnit_AssembleFields_FieldNameCollision_IsRefused is the field-level
// analogue of TestUnit_NestedStructNameCollidesWithSiblingProperty above:
// Task 4 added collision detection for struct names, but the same defect one
// level down, on fields, went unguarded. "TLSSkipVerify" and "TlsSkipVerify"
// are the real example naming.go's own doc comment now documents: both
// collapse to the JSON tag "tlsSkipVerify" via bodyJSONTag (and to the same
// Go field name "TLSSkipVerify" via goFieldName). Before this fix,
// assembleFields built both fields anyway — go/format reformats the result,
// go/types accepts two struct fields with distinct declarations sharing one
// json tag, and encoding/json then silently drops both fields that share a
// tag at (un)marshal time, so the published schema disagreed with what the
// generated handler actually parsed.
func TestUnit_AssembleFields_FieldNameCollision_IsRefused(t *testing.T) {
	t.Parallel()
	props := []schemaProperty{
		{Name: "TLSSkipVerify", Schema: &schemaNode{Type: "boolean"}},
		{Name: "TlsSkipVerify", Schema: &schemaNode{Type: "boolean"}},
	}
	var structs []structSpec
	_, err := assembleFields(props, "widgetInput", &structs)
	if err == nil {
		t.Fatal("assembleFields() = nil error, want a refusal: both properties render the same JSON tag (\"tlsSkipVerify\")")
	}
	for _, want := range []string{"TLSSkipVerify", "TlsSkipVerify"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("assembleFields() error = %q, want it to name %q", err, want)
		}
	}
}

// TestUnit_AssembleFields_NestedObjectFieldCollision_IsRefused proves the
// check actually fires one level down, inside a nested object's own
// properties — the case assembleOperationFields's top-level merge check (its
// "add" closure, which only ever sees path/query parameters and a request
// body's flattened top-level fields) never reaches, because a nested
// object's properties are assembled by assembleFields directly and never
// pass through that merge at all.
func TestUnit_AssembleFields_NestedObjectFieldCollision_IsRefused(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "WidgetCreate",
		Method:      "POST",
		Path:        "/widgets",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"config": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"TLSSkipVerify": map[string]any{"type": "boolean"},
									"TlsSkipVerify": map[string]any{"type": "boolean"},
								},
							},
						},
					},
				},
			},
		},
	}
	doc := newDoc(nil)
	res := &resolver{doc: doc}
	var nested []structSpec
	_, _, err := assembleOperationFields(op, res, doc, "widgetCreateInput", &nested)
	if err == nil {
		t.Fatal("assembleOperationFields() = nil error, want a refusal: the nested \"config\" object's two properties render the same JSON tag")
	}
}

// TestUnit_WholeDomainFile_MultipleOperationsCompileTogether builds and
// compiles a whole rendered domain file assembled from two independent
// operations, the way main.go's per-domain loop actually does (concatenating
// each operation's top-level struct and nested structs into one file) —
// rather than the 1-4 hand-built structs every other test in this file
// renders. Cross-operation struct redeclaration can only be in reach at this
// altitude.
func TestUnit_WholeDomainFile_MultipleOperationsCompileTogether(t *testing.T) {
	t.Parallel()
	doc := newDoc(nil)
	res := &resolver{doc: doc}

	ops := []operation{
		{
			OperationID: "TagCreate",
			Method:      "POST",
			Path:        "/tags",
			RequestBody: map[string]any{
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": map[string]any{
							"type":       "object",
							"properties": map[string]any{"name": map[string]any{"type": "string"}},
							"required":   []any{"name"},
						},
					},
				},
			},
		},
		{
			OperationID: "TagDelete",
			Method:      "DELETE",
			Path:        "/tags/{id}",
			Parameters: []map[string]any{
				{"name": "id", "in": "path", "required": true, "schema": map[string]any{"type": "integer"}},
			},
		},
	}

	var allStructs []structSpec
	for _, op := range ops {
		structName := inputStructName(op.OperationID)
		var nested []structSpec
		fields, _, err := assembleOperationFields(op, res, doc, structName, &nested)
		if err != nil {
			t.Fatalf("assembleOperationFields(%s) error = %v", op.OperationID, err)
		}
		allStructs = append(allStructs, structSpec{Name: structName, Fields: fields})
		allStructs = append(allStructs, nested...)
	}

	if name, dup := duplicateStructName(allStructs); dup {
		t.Fatalf("unexpected duplicate struct name %q across independent operations", name)
	}

	src, err := renderFile("tags", "test-spec.json", allStructs)
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	text := string(src)
	if !strings.Contains(text, "type tagCreateInput struct {") || !strings.Contains(text, "type tagDeleteInput struct {") {
		t.Errorf("whole-file render is missing one of the two operations' structs:\n%s", text)
	}
}

// --- refusal cases ---

func TestUnit_Refusals(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		build   func() error
		wantErr string
	}{
		{
			name: "oneOf is not expressible as a single Go type",
			build: func() error {
				res := &resolver{doc: newDoc(nil)}
				_, err := res.resolve(map[string]any{
					"oneOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}},
				}, 0)
				return err
			},
			wantErr: "oneOf",
		},
		{
			name: "anyOf is not expressible as a single Go type",
			build: func() error {
				res := &resolver{doc: newDoc(nil)}
				_, err := res.resolve(map[string]any{
					"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "integer"}},
				}, 0)
				return err
			},
			wantErr: "anyOf",
		},
		{
			name: "unresolved $ref",
			build: func() error {
				res := &resolver{doc: newDoc(map[string]any{})}
				_, err := res.resolve(map[string]any{"$ref": "#/components/schemas/does.NotExist"}, 0)
				return err
			},
			wantErr: "unresolved",
		},
		{
			name: "free-form object with no declared shape",
			build: func() error {
				res := &resolver{doc: newDoc(nil)}
				node, err := res.resolve(map[string]any{"type": "object"}, 0)
				if err != nil {
					return err
				}
				var structs []structSpec
				_, err = typeOf(node, true, "x", &structs)
				return err
			},
			wantErr: "free-form",
		},
		{
			name: "two request body content types",
			build: func() error {
				doc := newDoc(nil)
				_, err := doc.requestBodySchema(map[string]any{
					"content": map[string]any{
						"application/json":    map[string]any{"schema": map[string]any{"type": "object"}},
						"multipart/form-data": map[string]any{"schema": map[string]any{"type": "object"}},
					},
				})
				return err
			},
			wantErr: "content types",
		},
		{
			name: "path parameter collides with a body property",
			build: func() error {
				op := operation{
					OperationID: "Collide",
					Method:      "POST",
					Path:        "/collide/{name}",
					Parameters: []map[string]any{
						{"name": "name", "in": "path", "required": true, "schema": map[string]any{"type": "string"}},
					},
					RequestBody: map[string]any{
						"content": map[string]any{
							"application/json": map[string]any{
								"schema": map[string]any{
									"type":       "object",
									"properties": map[string]any{"name": map[string]any{"type": "string"}},
									"required":   []any{"name"},
								},
							},
						},
					},
				}
				doc := newDoc(nil)
				res := &resolver{doc: doc}
				var nested []structSpec
				_, _, err := assembleOperationFields(op, res, doc, "collideInput", &nested)
				return err
			},
			wantErr: "contributed by both",
		},
		{
			name: "header parameter is not supported",
			build: func() error {
				op := operation{
					OperationID: "HeaderOp",
					Method:      "GET",
					Path:        "/header-op",
					Parameters: []map[string]any{
						{"name": "X-Setup-Token", "in": "header", "schema": map[string]any{"type": "string"}},
					},
				}
				doc := newDoc(nil)
				res := &resolver{doc: doc}
				var nested []structSpec
				_, _, err := assembleOperationFields(op, res, doc, "headerOpInput", &nested)
				return err
			},
			wantErr: "not supported",
		},
		{
			// A components.parameters $ref parameter object carries no "name"
			// of its own. Before this fix, the loop's `if name == "" {
			// continue }` silently dropped it — including an unmet required
			// path/query parameter — with no trace in the generated Input
			// struct.
			name: "parameter with no name is refused rather than silently skipped",
			build: func() error {
				op := operation{
					OperationID: "RefParamOp",
					Method:      "GET",
					Path:        "/ref-param-op/{id}",
					Parameters: []map[string]any{
						{"$ref": "#/components/parameters/IDParam"},
					},
				}
				doc := newDoc(nil)
				res := &resolver{doc: doc}
				var nested []structSpec
				_, _, err := assembleOperationFields(op, res, doc, "refParamOpInput", &nested)
				return err
			},
			wantErr: "no name",
		},
		{
			name: "requestBody content declares no schema",
			build: func() error {
				doc := newDoc(nil)
				_, err := doc.requestBodySchema(map[string]any{
					"content": map[string]any{
						"application/json": map[string]any{},
					},
				})
				return err
			},
			wantErr: "no schema",
		},
		{
			name: "domain tag claimed by two domains",
			build: func() error {
				domainTags := map[string][]string{
					"tags":      {"tags"},
					"tags_dupe": {"tags"},
				}
				byTag := map[string][]operation{
					"tags": {{OperationID: "TagList"}},
				}
				return checkDomainTagsCoverSpec(domainTags, byTag)
			},
			wantErr: "claimed by both",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.build()
			if err == nil {
				t.Fatal("error = nil, want a refusal")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to mention %q", err.Error(), tt.wantErr)
			}
		})
	}
}
