package toolutil

import (
	"encoding/json"
	"strings"
	"testing"
)

// This file covers docs/api-divergences.md §9.5: InputSchema asserted the
// EnumParams and MinimumParams interfaces on the top-level Input value alone,
// so a nested struct type's generated methods were never consulted and no
// enum was published for any field of a nested object.
//
// The shapes below are the ones the reflected schema renders differently —
// a bare struct, a pointer, a slice, a map value, an embedded struct — because
// a fix that only reaches a bare nested struct is a half-fix that reads like a
// whole one. Each is asserted through the public InputSchema, on the exact
// JSON pointer a surface would publish, not on the walker's internals.
//
// Every case here was confirmed to fail with the fix reverted to the old
// top-level-only assertion, with two deliberate exceptions: the two *embedded*
// cases pass either way, because Go promotes an embedded type's methods onto
// the embedding type, so the top-level assertion already found them. They are
// kept as guards on the opposite failure — a walk that treated an embedded
// struct as owning a sub-schema would look for properties that are not there.

type nestedEnumLeaf struct {
	MatchType *string `json:"matchType,omitempty"`
	Depth     *int    `json:"depth,omitempty"`
}

func (nestedEnumLeaf) EnumParams() map[string][]any {
	return map[string][]any{"matchType": {"file", "dir"}}
}

func (nestedEnumLeaf) MinimumParams() map[string]int {
	return map[string]int{"depth": 1}
}

type nestedBareStructInput struct {
	Settings nestedEnumLeaf `json:"settings"`
}

type nestedPointerInput struct {
	Settings *nestedEnumLeaf `json:"settings,omitempty"`
}

type nestedTwoLevelPointerInput struct {
	EdgeSettings *struct {
		RelativePathSettings *nestedEnumLeaf `json:"relativePathSettings,omitempty"`
	} `json:"edgeSettings,omitempty"`
}

type nestedSliceInput struct {
	Settings []nestedEnumLeaf `json:"settings,omitempty"`
}

type nestedSliceOfPointersInput struct {
	Settings []*nestedEnumLeaf `json:"settings,omitempty"`
}

type nestedArrayInput struct {
	Settings [2]nestedEnumLeaf `json:"settings"`
}

type nestedMapInput struct {
	Settings map[string]nestedEnumLeaf `json:"settings,omitempty"`
}

// nestedEmbeddedInput embeds the constrained type. google/jsonschema-go
// flattens an embedded struct's fields into the embedding struct's own
// properties, so "matchType" is a top-level property here and the embedded
// type's constraints must land there.
type nestedEmbeddedInput struct {
	nestedEnumLeaf
	Title string `json:"title"`
}

type middleEmbedding struct {
	nestedEnumLeaf
}

// nestedDoubleEmbeddedInput reaches the constrained type through two levels of
// embedding, which reflect.VisibleFields reports as two separate anonymous
// fields of this struct.
type nestedDoubleEmbeddedInput struct {
	middleEmbedding
	Title string `json:"title"`
}

func TestUnit_InputSchema_NestedStructConstraints_ArePublishedAtEveryShape(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		input     any
		enumAt    []string
		minimumAt []string
	}{
		{
			name:      "a bare nested struct",
			input:     nestedBareStructInput{},
			enumAt:    []string{"properties", "settings", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "properties", "depth"},
		},
		{
			name:      "a pointer to a nested struct",
			input:     nestedPointerInput{},
			enumAt:    []string{"properties", "settings", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "properties", "depth"},
		},
		{
			name:      "two levels down, through two pointers",
			input:     nestedTwoLevelPointerInput{},
			enumAt:    []string{"properties", "edgeSettings", "properties", "relativePathSettings", "properties", "matchType"},
			minimumAt: []string{"properties", "edgeSettings", "properties", "relativePathSettings", "properties", "depth"},
		},
		{
			name:      "a slice of structs, through items",
			input:     nestedSliceInput{},
			enumAt:    []string{"properties", "settings", "items", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "items", "properties", "depth"},
		},
		{
			name:      "a slice of pointers to structs",
			input:     nestedSliceOfPointersInput{},
			enumAt:    []string{"properties", "settings", "items", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "items", "properties", "depth"},
		},
		{
			name:      "an array of structs",
			input:     nestedArrayInput{},
			enumAt:    []string{"properties", "settings", "items", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "items", "properties", "depth"},
		},
		{
			name:      "a map whose values are structs, through additionalProperties",
			input:     nestedMapInput{},
			enumAt:    []string{"properties", "settings", "additionalProperties", "properties", "matchType"},
			minimumAt: []string{"properties", "settings", "additionalProperties", "properties", "depth"},
		},
		{
			name:      "an embedded struct, whose fields are flattened into the parent",
			input:     nestedEmbeddedInput{},
			enumAt:    []string{"properties", "matchType"},
			minimumAt: []string{"properties", "depth"},
		},
		{
			name:      "a struct embedded inside an embedded struct",
			input:     nestedDoubleEmbeddedInput{},
			enumAt:    []string{"properties", "matchType"},
			minimumAt: []string{"properties", "depth"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema, err := (ActionSpec{Name: "fixture.action", Input: tt.input}).InputSchema()
			if err != nil {
				t.Fatalf("InputSchema() error = %v", err)
			}
			enumProp := schemaAt(t, schema, tt.enumAt)
			values, ok := enumProp["enum"].([]any)
			if !ok {
				t.Fatalf("no \"enum\" published at %s; the property is %v", strings.Join(tt.enumAt, "."), enumProp)
			}
			if len(values) != 2 || values[0] != "file" || values[1] != "dir" {
				t.Errorf("enum at %s = %v, want [file dir]", strings.Join(tt.enumAt, "."), values)
			}
			minimumProp := schemaAt(t, schema, tt.minimumAt)
			if minimumProp["minimum"] != 1 {
				t.Errorf("minimum at %s = %v, want 1", strings.Join(tt.minimumAt, "."), minimumProp["minimum"])
			}
		})
	}
}

// nestedStaleEnumLeaf declares a constraint on a property its own struct does
// not have — a typo, or a stale entry left after a field was renamed.
type nestedStaleEnumLeaf struct {
	MatchType *string `json:"matchType,omitempty"`
}

func (nestedStaleEnumLeaf) EnumParams() map[string][]any {
	return map[string][]any{"matchTyp": {"file", "dir"}}
}

type nestedStaleEnumInput struct {
	Settings *nestedStaleEnumLeaf `json:"settings,omitempty"`
}

type nestedStaleMinimumLeaf struct {
	Depth *int `json:"depth,omitempty"`
}

func (nestedStaleMinimumLeaf) MinimumParams() map[string]int {
	return map[string]int{"deth": 1}
}

type nestedStaleMinimumInput struct {
	Settings *nestedStaleMinimumLeaf `json:"settings,omitempty"`
}

// TestUnit_InputSchema_NestedConstraintNamingAnUnknownProperty_IsRefused
// carries the top-level refusal down with the constraint. Silently skipping a
// nested entry that matches nothing would publish a schema that looks complete
// while constraining nothing — the same defect §9.5 records, one rename later.
func TestUnit_InputSchema_NestedConstraintNamingAnUnknownProperty_IsRefused(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{name: "a nested EnumParams entry", input: nestedStaleEnumInput{}, want: `EnumParams declares "matchTyp"`},
		{name: "a nested MinimumParams entry", input: nestedStaleMinimumInput{}, want: `MinimumParams declares "deth"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := (ActionSpec{Name: "fixture.action", Input: tt.input}).InputSchema()
			if err == nil {
				t.Fatal("InputSchema() error = nil, want a refusal for a nested entry naming an unknown property")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("InputSchema() error = %q, want it to contain %q", err, tt.want)
			}
			if !strings.Contains(err.Error(), `property "settings"`) {
				t.Errorf("InputSchema() error = %q, want it to name the property path that led to the bad entry", err)
			}
		})
	}
}

// pointerReceiverLeaf declares its constraint on a pointer receiver. The
// generated methods all use value receivers, but a hand-written one need not,
// and being ignored for that reason is the same silence §9.5 is about.
type pointerReceiverLeaf struct {
	MatchType *string `json:"matchType,omitempty"`
}

func (*pointerReceiverLeaf) EnumParams() map[string][]any {
	return map[string][]any{"matchType": {"file", "dir"}}
}

type pointerReceiverInput struct {
	Settings *pointerReceiverLeaf `json:"settings,omitempty"`
}

func TestUnit_InputSchema_NestedConstraintOnAPointerReceiver_IsStillApplied(t *testing.T) {
	t.Parallel()
	schema, err := (ActionSpec{Name: "fixture.action", Input: pointerReceiverInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	prop := schemaAt(t, schema, []string{"properties", "settings", "properties", "matchType"})
	if _, ok := prop["enum"]; !ok {
		t.Errorf("no \"enum\" published for a pointer-receiver EnumParams; the property is %v", prop)
	}
}

// unconstrainedLeaf implements neither interface: a nested struct with no
// declared constraints must come back with no enum and no minimum invented for
// it, or the walk would be writing keywords nobody asked for.
type unconstrainedLeaf struct {
	MatchType *string `json:"matchType,omitempty"`
}

type unconstrainedInput struct {
	Settings *unconstrainedLeaf `json:"settings,omitempty"`
}

func TestUnit_InputSchema_NestedStructWithNoConstraints_PublishesNone(t *testing.T) {
	t.Parallel()
	schema, err := (ActionSpec{Name: "fixture.action", Input: unconstrainedInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	prop := schemaAt(t, schema, []string{"properties", "settings", "properties", "matchType"})
	for _, keyword := range []string{"enum", "minimum"} {
		if _, ok := prop[keyword]; ok {
			t.Errorf("property carries %q it never declared: %v", keyword, prop)
		}
	}
}

// TestUnit_ValidateInput_NestedEnum_RefusesAnOutOfEnumValue closes the loop the
// published schema opens: §9.5's second consequence was that no ValidateInput
// enforced a nested constraint either, so a wrong value was refused by
// Portainer rather than by this catalog. ValidateInput resolves the very map
// InputSchema publishes, so this is the same fix observed from the enforcement
// side rather than the publication side.
func TestUnit_ValidateInput_NestedEnum_RefusesAnOutOfEnumValue(t *testing.T) {
	t.Parallel()
	spec := ActionSpec{Name: "fixture.action", Input: nestedPointerInput{}}
	tests := []struct {
		name      string
		arguments string
		wantError bool
	}{
		{name: "a value the nested enum allows", arguments: `{"settings":{"matchType":"dir"}}`, wantError: false},
		{name: "a value it does not", arguments: `{"settings":{"matchType":"folder"}}`, wantError: true},
		{name: "the vendored document's malformed spelling", arguments: `{"settings":{"matchType":" dir"}}`, wantError: true},
		{name: "below the nested minimum", arguments: `{"settings":{"depth":0}}`, wantError: true},
		{name: "at the nested minimum", arguments: `{"settings":{"depth":1}}`, wantError: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := spec.ValidateInput(json.RawMessage(tt.arguments))
			if tt.wantError && err == nil {
				t.Errorf("ValidateInput(%s) error = nil, want a refusal", tt.arguments)
			}
			if !tt.wantError && err != nil {
				t.Errorf("ValidateInput(%s) error = %v, want nil", tt.arguments, err)
			}
		})
	}
}

// schemaAt walks a decoded JSON Schema down a path of map keys, failing the
// test with the path it got stuck on rather than panicking on a nil map.
func schemaAt(t *testing.T, schema map[string]any, path []string) map[string]any {
	t.Helper()
	node := schema
	for i, key := range path {
		next, ok := node[key].(map[string]any)
		if !ok {
			t.Fatalf("schema has no object at %s (stopped at %q): %v", strings.Join(path[:i+1], "."), key, node)
		}
		node = next
	}
	return node
}
