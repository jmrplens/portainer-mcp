package main

import "testing"

func TestDanglingRefs_ResolvableRef_ReportsNothing(t *testing.T) {
	t.Parallel()
	spec := Spec{
		"paths":      map[string]any{"/a": map[string]any{"$ref": "#/components/schemas/S"}},
		"components": map[string]any{"schemas": map[string]any{"S": map[string]any{"type": "object"}}},
	}
	if broken := danglingRefs(spec); len(broken) != 0 {
		t.Errorf("danglingRefs = %v, want none", broken)
	}
}

func TestDanglingRefs_MissingTarget_IsReported(t *testing.T) {
	t.Parallel()
	spec := Spec{
		"paths":      map[string]any{"/a": map[string]any{"$ref": "#/components/schemas/Missing"}},
		"components": map[string]any{"schemas": map[string]any{}},
	}
	broken := danglingRefs(spec)
	if len(broken) != 1 {
		t.Fatalf("danglingRefs = %v, want exactly one", broken)
	}
}

// An external $ref means bundling did not finish; the generator would have to
// fetch at build time, which defeats committing the spec.
func TestDanglingRefs_ExternalRef_IsReported(t *testing.T) {
	t.Parallel()
	spec := Spec{"paths": map[string]any{"/a": map[string]any{"$ref": "other.yaml#/paths/~1a"}}}
	broken := danglingRefs(spec)
	if len(broken) != 1 {
		t.Fatalf("danglingRefs = %v, want the external ref reported", broken)
	}
}
