package wiring

import (
	"strings"
	"testing"
)

// This file asserts the two defects docs/api-divergences.md §6.6 and §9.5
// record, at the one place a model actually reads: the schema a registered
// action publishes.
//
// §9.5 measured custom_templates.create_repository publishing
// {"description": "Per device configs match type", "type": ["null","string"]}
// for edgeSettings.relativePathSettings.perDeviceConfigsMatchType — no "enum"
// keyword at all, because InputSchema asserted EnumParams on the top-level
// Input value only. §6.6 measured the value that generated nested EnumParams
// returned as {"file", " dir"}, transcribed from a malformed inline enum in
// the vendored document that overrides a clean allOf $ref.
//
// The two are coupled: fixing §9.5 alone would have started publishing " dir"
// to models as the only spelling this catalog admits, which is why the enum's
// contents are asserted here and not just its presence.
//
// AllSpecs, rather than a built catalog, because these fields are EE-only: a
// Community catalog prunes the whole edgeSettings property (its parent field
// is tagged edition:"EE"), so the unpruned registered spec is where the
// constraint is observable at all.
func TestUnit_RegisteredSpec_NestedEnumConstraints_ArePublishedAndTrimmed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		action string
		path   []string
		want   []any
	}{
		{
			name:   "create_repository's relative path match type",
			action: "custom_templates.create_repository",
			path:   []string{"edgeSettings", "relativePathSettings", "perDeviceConfigsMatchType"},
			want:   []any{"file", "dir"},
		},
		{
			name:   "create_repository's relative path group match type",
			action: "custom_templates.create_repository",
			path:   []string{"edgeSettings", "relativePathSettings", "perDeviceConfigsGroupMatchType"},
			want:   []any{"file", "dir"},
		},
		{
			name:   "create_repository's stagger option",
			action: "custom_templates.create_repository",
			path:   []string{"edgeSettings", "staggerConfig", "staggerOption"},
			want:   []any{0, 1, 2},
		},
		{
			name:   "create_string's relative path match type",
			action: "custom_templates.create_string",
			path:   []string{"edgeSettings", "relativePathSettings", "perDeviceConfigsMatchType"},
			want:   []any{"file", "dir"},
		},
		{
			name:   "update's relative path match type",
			action: "custom_templates.update",
			path:   []string{"edgeSettings", "relativePathSettings", "perDeviceConfigsMatchType"},
			want:   []any{"file", "dir"},
		},
		{
			name:   "update's update failure action",
			action: "custom_templates.update",
			path:   []string{"edgeSettings", "staggerConfig", "updateFailureAction"},
			want:   []any{0, 1, 2, 3},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			schema, err := registeredInputSchema(t, tt.action)
			if err != nil {
				t.Fatalf("InputSchema() error = %v", err)
			}
			prop := propertyAt(t, schema, tt.path)
			values, ok := prop["enum"].([]any)
			if !ok {
				t.Fatalf("%s publishes no \"enum\" for %s: %v", tt.action, strings.Join(tt.path, "."), prop)
			}
			if len(values) != len(tt.want) {
				t.Fatalf("enum for %s = %v, want %v", strings.Join(tt.path, "."), values, tt.want)
			}
			for i := range tt.want {
				if values[i] != tt.want[i] {
					t.Errorf("enum for %s [%d] = %#v, want %#v (full: %v)", strings.Join(tt.path, "."), i, values[i], tt.want[i], values)
				}
			}
		})
	}
}

// TestUnit_RegisteredSpecs_NoPublishedEnumValueCarriesWhitespace sweeps every
// registered action's published schema, at every depth, rather than only the
// six values §6.6 counted: the generator now trims as it reads, so a value
// with surrounding space reappearing anywhere means either a hand edit
// reintroduced one or the trim stopped covering a path.
func TestUnit_RegisteredSpecs_NoPublishedEnumValueCarriesWhitespace(t *testing.T) {
	t.Parallel()
	for _, spec := range AllSpecs() {
		if spec.Input == nil {
			continue
		}
		schema, err := spec.InputSchema()
		if err != nil {
			t.Errorf("InputSchema() for %s error = %v", spec.Name, err)
			continue
		}
		for _, found := range enumValuesWithWhitespace(schema, "") {
			t.Errorf("%s publishes enum value %q at %s, which carries surrounding whitespace", spec.Name, found.value, found.path)
		}
	}
}

type whitespaceEnumValue struct {
	path  string
	value string
}

func enumValuesWithWhitespace(node any, path string) []whitespaceEnumValue {
	var found []whitespaceEnumValue
	switch typed := node.(type) {
	case map[string]any:
		if values, ok := typed["enum"].([]any); ok {
			for _, value := range values {
				s, ok := value.(string)
				if !ok || s == "" || s == strings.TrimSpace(s) {
					continue
				}
				found = append(found, whitespaceEnumValue{path: path + "/enum", value: s})
			}
		}
		for key, value := range typed {
			found = append(found, enumValuesWithWhitespace(value, path+"/"+key)...)
		}
	case []any:
		for _, value := range typed {
			found = append(found, enumValuesWithWhitespace(value, path+"[]")...)
		}
	}
	return found
}

func registeredInputSchema(t *testing.T, action string) (map[string]any, error) {
	t.Helper()
	for _, spec := range AllSpecs() {
		if spec.Name == action {
			return spec.InputSchema()
		}
	}
	t.Fatalf("action %q is not registered", action)
	return nil, nil
}

// propertyAt walks a published schema down a chain of property names, so a
// test reads as the JSON path a model would follow rather than as an
// alternating sequence of "properties" keys.
func propertyAt(t *testing.T, schema map[string]any, names []string) map[string]any {
	t.Helper()
	node := schema
	for i, name := range names {
		props, ok := node["properties"].(map[string]any)
		if !ok {
			t.Fatalf("no \"properties\" at %s", strings.Join(names[:i], "."))
		}
		next, ok := props[name].(map[string]any)
		if !ok {
			t.Fatalf("no property %q at %s", name, strings.Join(names[:i], "."))
		}
		node = next
	}
	return node
}
