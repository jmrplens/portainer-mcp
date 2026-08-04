package main

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// ceEEFieldDiffFixture is one operation resolved on both sides for the
// tests below: EE declares a shared top-level field ("name"), an EE-only
// top-level field ("edgeKey"), a shared nested object ("tlsConfig") whose
// own "edgeCert" property is EE-only (nested-inclusive divergence one level
// deep — the real shape endpointUpdateInput.kubernetesConfiguration has:
// the object itself is shared, only some of its own properties are not),
// and a nested object ("kubeConfig") that Community does not declare at
// all, itself containing a further nested object ("advanced") — the real
// shape customTemplateCreateRepositoryInput.edgeSettings has, an
// EE-only object several of whose own descendants are also EE-only. CE
// declares "name" and "tlsConfig" (with only "caCert"), nothing else.
func ceEEFieldDiffFixture(t *testing.T) (structName string, eeFields []fieldSpec, eeNested []structSpec, ceOp operation, ceRes *resolver, ceDoc *document) {
	t.Helper()
	structName = "endpointCreateLikeInput"

	eeOp := operation{
		OperationID: "EndpointCreateLike",
		Method:      "POST",
		Path:        "/endpoints-like",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name":    map[string]any{"type": "string"},
							"edgeKey": map[string]any{"type": "string"},
							"tlsConfig": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"caCert":   map[string]any{"type": "string"},
									"edgeCert": map[string]any{"type": "string"},
								},
							},
							"kubeConfig": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"namespace": map[string]any{"type": "string"},
									"advanced": map[string]any{
										"type": "object",
										"properties": map[string]any{
											"timeout": map[string]any{"type": "integer"},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	eeDoc := newDoc(nil)
	eeRes := &resolver{doc: eeDoc}
	fields, _, err := assembleOperationFields(eeOp, eeRes, eeDoc, structName, &eeNested)
	if err != nil {
		t.Fatalf("assembleOperationFields(ee) error = %v", err)
	}

	ceOp = operation{
		OperationID: "EndpointCreateLike",
		Method:      "POST",
		Path:        "/endpoints-like",
		RequestBody: map[string]any{
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"name": map[string]any{"type": "string"},
							"tlsConfig": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"caCert": map[string]any{"type": "string"},
								},
							},
						},
					},
				},
			},
		},
	}
	ceDoc = newDoc(nil)
	ceRes = &resolver{doc: ceDoc}

	return structName, fields, eeNested, ceOp, ceRes, ceDoc
}

func TestUnit_CeEEFieldDiff_SharedTopLevelField_IsNotMarked(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}
	if eeOnly[structName+".name"] {
		t.Error("\"name\" is shared by both editions and must not be reported EE-only")
	}
}

func TestUnit_CeEEFieldDiff_TopLevelFieldAbsentFromCE_IsMarkedEEOnly(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}
	if !eeOnly[structName+".edgeKey"] {
		t.Errorf("eeOnly = %v, want %q.edgeKey reported EE-only", eeOnly, structName)
	}
}

// TestUnit_CeEEFieldDiff_NestedFieldAbsentFromCE_IsMarkedEEOnly_NestedInclusive
// is the case a purely top-level comparison would miss entirely:
// "tlsConfig" itself exists on both sides, but only one of its own
// properties, "edgeCert", is Community-absent. This is the divergence a
// nested-inclusive comparison exists to catch (see fields.go's own doc
// comment on the 65-vs-159 measurement this exact distinction produced
// against the real wave-1 specifications).
func TestUnit_CeEEFieldDiff_NestedFieldAbsentFromCE_IsMarkedEEOnly_NestedInclusive(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}

	tlsConfigStruct := structName + "TLSConfig"
	if !eeOnly[tlsConfigStruct+".edgeCert"] {
		t.Errorf("eeOnly = %v, want %s.edgeCert reported EE-only", eeOnly, tlsConfigStruct)
	}
	if eeOnly[tlsConfigStruct+".caCert"] {
		t.Error("caCert is shared by both editions (inside a shared nested struct) and must not be reported EE-only")
	}
	// The struct itself is shared (present on both sides), so the top-level
	// field pointing to it must not be marked either — only its own
	// "edgeCert" property diverges.
	if eeOnly[structName+".tlsConfig"] {
		t.Error("tlsConfig itself is shared by both editions and must not be reported EE-only")
	}
}

// TestUnit_CeEEFieldDiff_WholeNestedObjectAbsentFromCE_CascadesToEveryDescendant
// is the other real shape this generator's wave-1 measurement found
// (customTemplateCreateRepositoryInput.edgeSettings and its own
// staggerConfig/relativePathSettings descendants): a nested object Community
// does not declare at all must mark not only the top-level property that
// names it, but every one of its own fields, and every field of anything
// nested inside *that*, transitively — because none of that shape exists on
// a Community server at any depth.
func TestUnit_CeEEFieldDiff_WholeNestedObjectAbsentFromCE_CascadesToEveryDescendant(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}

	kubeConfigStruct := structName + "KubeConfig"
	advancedStruct := kubeConfigStruct + "Advanced"
	for _, key := range []string{
		structName + ".kubeConfig",
		kubeConfigStruct + ".namespace",
		kubeConfigStruct + ".advanced",
		advancedStruct + ".timeout",
	} {
		if !eeOnly[key] {
			t.Errorf("eeOnly = %v, want %q reported EE-only (a whole object Community does not declare cascades to every descendant)", eeOnly, key)
		}
	}
}

// TestUnit_ApplyFieldEditionGate_StampsOnlyMatchedFields proves the mutation
// half: applyFieldEditionGate must set RequiresEdition on exactly the
// fields eeOnly names — top-level and nested alike — and leave every other
// field's RequiresEdition at its zero value.
func TestUnit_ApplyFieldEditionGate_StampsOnlyMatchedFields(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}
	applyFieldEditionGate(structName, fields, nested, eeOnly)

	gated := map[string]edition.Edition{}
	for _, f := range fields {
		gated[structName+"."+f.JSONName] = f.RequiresEdition
	}
	for _, s := range nested {
		for _, f := range s.Fields {
			gated[s.Name+"."+f.JSONName] = f.RequiresEdition
		}
	}

	for key, want := range map[string]edition.Edition{
		structName + ".name":                      "",
		structName + ".edgeKey":                   edition.EE,
		structName + "TLSConfig.caCert":           "",
		structName + "TLSConfig.edgeCert":         edition.EE,
		structName + "KubeConfig.namespace":       edition.EE,
		structName + "KubeConfigAdvanced.timeout": edition.EE,
	} {
		if got := gated[key]; got != want {
			t.Errorf("RequiresEdition[%q] = %q, want %q", key, got, want)
		}
	}
}

// TestUnit_CeEEFieldDiff_UnresolvableCESide_ReturnsWrappedError is the
// degrade-not-refuse path main.go's per-operation loop relies on: ceOp's own
// shape can be unresolvable for the identical reasons assembleOperationFields
// already refuses generally (here, a header-located parameter, which
// assembleOperationFields does not support merging into a flat Input at
// all), independent of whether the Business Edition side resolved cleanly.
// ceEEFieldDiff must surface that as an error — never silently report "no
// divergence", which would generate the Business Edition Input struct
// completely ungated and indistinguishable from a genuinely CE-shared,
// non-divergent operation.
func TestUnit_CeEEFieldDiff_UnresolvableCESide_ReturnsWrappedError(t *testing.T) {
	t.Parallel()
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	ceOp.Parameters = []map[string]any{
		{"name": "x-trace-id", "in": "header", "schema": map[string]any{"type": "string"}},
	}

	_, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err == nil {
		t.Fatal("ceEEFieldDiff() error = nil, want a refusal: the Community side has an unresolvable header parameter")
	}
	if !strings.Contains(err.Error(), "Community Edition") {
		t.Errorf("error = %q, want it to say this is the Community Edition side that failed to resolve", err)
	}
}

// TestUnit_CeOperationsByID_FlattensAcrossTags mirrors ceOperationIDSet's own
// test coverage for the presence-only set, proving the by-operation variant
// aggregates across every tag the same way and carries the operation itself,
// not merely its presence.
func TestUnit_CeOperationsByID_FlattensAcrossTags(t *testing.T) {
	t.Parallel()
	byTag := map[string][]operation{
		"endpoints": {{OperationID: "EndpointList", Method: "GET", Path: "/endpoints"}},
		"stacks":    {{OperationID: "StackList", Method: "GET", Path: "/stacks"}},
	}
	got := ceOperationsByID(byTag)
	if len(got) != 2 {
		t.Fatalf("ceOperationsByID() = %v, want exactly 2 entries", got)
	}
	if got["EndpointList"].Path != "/endpoints" {
		t.Errorf("ceOperationsByID()[\"EndpointList\"] = %+v, want Path /endpoints", got["EndpointList"])
	}
	if got["StackList"].Path != "/stacks" {
		t.Errorf("ceOperationsByID()[\"StackList\"] = %+v, want Path /stacks", got["StackList"])
	}
}

// TestUnit_FieldTag_EEOnlyField_RendersEditionTag proves the rendering half
// in isolation: fieldTag appends `edition:"EE"` after "jsonschema", never
// before, and omits the keyword entirely for the zero-value edition.Edition.
func TestUnit_FieldTag_EEOnlyField_RendersEditionTag(t *testing.T) {
	t.Parallel()
	got := fieldTag("edgeKey", false, "Edge key", edition.EE)
	want := "`json:\"edgeKey,omitempty\" jsonschema:\"Edge key\" edition:\"EE\"`"
	if got != want {
		t.Errorf("fieldTag() = %s, want %s", got, want)
	}

	ungated := fieldTag("name", true, "", "")
	if strings.Contains(ungated, "edition:") {
		t.Errorf("fieldTag() = %s, want no edition keyword for the zero-value edition.Edition", ungated)
	}
}

// TestUnit_GenerateEditionGatedOperation_RendersEditionTagOnTheRightFieldOnly
// is the end-to-end proof Step 1 of Task 2's brief asks for, run through
// this generator's own real assembly-and-render pipeline rather than
// asserted against fieldTag or ceEEFieldDiff in isolation: given an EE
// operation whose Community counterpart is missing one field, the rendered
// Go source carries `edition:"EE"` on exactly that field's struct tag —
// nested ones included — and the generated file still compiles.
func TestUnit_GenerateEditionGatedOperation_RendersEditionTagOnTheRightFieldOnly(t *testing.T) {
	structName, fields, nested, ceOp, ceRes, ceDoc := ceEEFieldDiffFixture(t)
	eeOnly, err := ceEEFieldDiff(structName, fields, nested, ceOp, ceRes, ceDoc)
	if err != nil {
		t.Fatalf("ceEEFieldDiff() error = %v", err)
	}
	applyFieldEditionGate(structName, fields, nested, eeOnly)

	all := append([]structSpec{{Name: structName, Fields: fields}}, nested...)
	src, err := renderFile("endpoints", "test-spec.json", all)
	if err != nil {
		t.Fatalf("renderFile() error = %v", err)
	}
	mustCompile(t, src)
	text := string(src)

	// gofmt column-aligns struct fields (extra inter-field spacing varies
	// with the longest sibling name/type in the same struct), so these
	// assertions match the tag itself — the part fieldTag actually
	// controls — rather than the whole line including that alignment
	// whitespace.
	if !strings.Contains(text, `json:"edgeKey,omitempty" edition:"EE"`) {
		t.Errorf("generated source does not tag the top-level EE-only field edgeKey:\n%s", text)
	}
	if !strings.Contains(text, `json:"edgeCert,omitempty" edition:"EE"`) {
		t.Errorf("generated source does not tag the nested EE-only field tlsConfig.edgeCert:\n%s", text)
	}
	if !strings.Contains(text, `json:"namespace,omitempty" edition:"EE"`) {
		t.Errorf("generated source does not tag the wholly-EE-only nested field kubeConfig.namespace:\n%s", text)
	}
	if !strings.Contains(text, `json:"timeout,omitempty" edition:"EE"`) {
		t.Errorf("generated source does not tag the doubly-nested EE-only field kubeConfig.advanced.timeout:\n%s", text)
	}
	if !strings.Contains(text, `json:"name,omitempty"`+"`") {
		t.Errorf("generated source does not declare the shared \"name\" field with its expected, ungated tag:\n%s", text)
	}
	if strings.Contains(text, `json:"name,omitempty" edition`) {
		t.Errorf("generated source tags the shared \"name\" field with an edition keyword it must not carry:\n%s", text)
	}
	if strings.Contains(text, `json:"caCert,omitempty" edition`) {
		t.Errorf("generated source tags the shared nested field tlsConfig.caCert with an edition keyword it must not carry:\n%s", text)
	}
}
