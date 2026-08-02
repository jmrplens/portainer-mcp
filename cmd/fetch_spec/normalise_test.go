package main

import (
	"strings"
	"testing"
)

func TestNormalise_DuplicateEnumValues_AreRemoved(t *testing.T) {
	t.Parallel()
	spec := Spec{"components": map[string]any{"schemas": map[string]any{
		"Status": map[string]any{"type": "string", "enum": []any{"a", "b", "a", "c", "b"}},
	}}}

	changes := normalise(spec)

	schemas := spec["components"].(map[string]any)["schemas"].(map[string]any)
	values := schemas["Status"].(map[string]any)["enum"].([]any)
	if len(values) != 3 {
		t.Fatalf("enum = %v, want 3 unique values in original order", values)
	}
	for i, want := range []any{"a", "b", "c"} {
		if values[i] != want {
			t.Errorf("enum[%d] = %v, want %v (order must be preserved)", i, values[i], want)
		}
	}
	if len(changes) != 1 || !strings.Contains(changes[0], "enum") {
		t.Errorf("changes = %v, want one line describing the enum fix", changes)
	}
}

func TestNormalise_WildcardContentType_BecomesJSON(t *testing.T) {
	t.Parallel()
	schema := map[string]any{"type": "object"}
	spec := Spec{"paths": map[string]any{"/stacks": map[string]any{
		"get": map[string]any{"responses": map[string]any{
			"200": map[string]any{"content": map[string]any{
				"*/*": map[string]any{"schema": schema},
			}},
		}},
	}}}

	changes := normalise(spec)

	content := spec["paths"].(map[string]any)["/stacks"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
	if _, still := content["*/*"]; still {
		t.Error(`content still carries "*/*"`)
	}
	got, ok := content["application/json"].(map[string]any)
	if !ok {
		t.Fatalf("content = %v, want application/json", content)
	}
	if got["schema"] == nil {
		t.Error("the schema was lost while renaming the content type")
	}
	if len(changes) != 1 {
		t.Errorf("changes = %v, want one line", changes)
	}
}

// A spec that already declares application/json alongside */* must keep the
// explicit entry — overwriting it would silently change the contract.
func TestNormalise_WildcardAlongsideJSON_LeavesJSONIntact(t *testing.T) {
	t.Parallel()
	spec := Spec{"paths": map[string]any{"/x": map[string]any{
		"get": map[string]any{"responses": map[string]any{
			"200": map[string]any{"content": map[string]any{
				"*/*":              map[string]any{"schema": map[string]any{"type": "string"}},
				"application/json": map[string]any{"schema": map[string]any{"type": "object"}},
			}},
		}},
	}}}

	normalise(spec)

	content := spec["paths"].(map[string]any)["/x"].(map[string]any)["get"].(map[string]any)["responses"].(map[string]any)["200"].(map[string]any)["content"].(map[string]any)
	kept := content["application/json"].(map[string]any)["schema"].(map[string]any)
	if kept["type"] != "object" {
		t.Errorf("application/json schema = %v, want the original object schema preserved", kept)
	}
}

func TestNormalise_CleanSpec_ReportsNoChanges(t *testing.T) {
	t.Parallel()
	spec := Spec{"components": map[string]any{"schemas": map[string]any{
		"Status": map[string]any{"enum": []any{"a", "b"}},
	}}}
	if changes := normalise(spec); len(changes) != 0 {
		t.Errorf("changes = %v, want none for an already-clean spec", changes)
	}
}
