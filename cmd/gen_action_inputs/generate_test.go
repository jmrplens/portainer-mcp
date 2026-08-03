package main

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
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
	fields, err := assembleOperationFields(op, res, newDoc(nil), "tagDeleteInput", &nested)
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
	if !strings.Contains(text, `ID int `+"`"+`json:"id"`+"`") {
		t.Errorf("generated source does not declare a required, non-pointer ID field:\n%s", text)
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
	fields, err := assembleOperationFields(op, res, newDoc(nil), "registryInspectInput", &nested)
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
	if !strings.Contains(string(src), `EndpointID *int `+"`"+`json:"endpointId,omitempty"`+"`") {
		t.Errorf("generated source does not declare an optional, pointer EndpointID field:\n%s", src)
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
	fields, err := assembleOperationFields(op, res, doc, "registryPingInput", &nested)
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
	fields, err := assembleOperationFields(op, res, doc, "registryCreateInput", &nested)
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
				_, err := assembleOperationFields(op, res, doc, "collideInput", &nested)
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
				_, err := assembleOperationFields(op, res, doc, "headerOpInput", &nested)
				return err
			},
			wantErr: "not supported",
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
