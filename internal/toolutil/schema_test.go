package toolutil

import (
	"encoding/json"
	"strings"
	"testing"
)

type sampleInput struct {
	ID   int    `json:"id" jsonschema:"Numeric identifier of the tag,required"`
	Name string `json:"name,omitempty" jsonschema:"Human-readable name"`
}

func TestInputSchema_ReflectsFieldsDescriptionsAndRequiredness(t *testing.T) {
	t.Parallel()
	spec := ActionSpec{Input: sampleInput{}}

	schema, err := spec.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	encoded, _ := json.Marshal(schema)
	text := string(encoded)

	// Each of these is produced only by reflecting the struct: none of them
	// appears anywhere else in the spec, so a stubbed-out schema cannot
	// satisfy them.
	for _, want := range []string{
		`"id"`,
		`"name"`,
		"Numeric identifier of the tag",
		"Human-readable name",
		`"required"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("schema does not carry %q:\n%s", want, text)
		}
	}
	// The permissive placeholder every surface published before this task must
	// be gone: publishing it alongside real properties would let a client
	// send anything and defeat the point.
	if strings.Contains(text, `"additionalProperties":true`) {
		t.Error("schema still carries the permissive placeholder")
	}
}

func TestInputSchema_NoInput_ReturnsAnEmptyObjectSchema(t *testing.T) {
	t.Parallel()
	// An action with no parameters must publish an object that accepts
	// nothing, not a missing schema and not a permissive one. A model reading
	// "no schema" cannot tell "takes nothing" from "unspecified".
	schema, err := ActionSpec{}.InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	if schema["type"] != "object" {
		t.Errorf("type = %v, want object", schema["type"])
	}
	if props, ok := schema["properties"].(map[string]any); ok && len(props) != 0 {
		t.Errorf("properties = %v, want none", props)
	}
}

// enumInput is a fixture Input implementing EnumParams, standing in for what
// cmd/gen_action_inputs emits for a spec-declared enum (e.g.
// registries.registryCreatePayload's "Type").
type enumInput struct {
	Type int `json:"type"`
}

func (enumInput) EnumParams() map[string][]any {
	return map[string][]any{"type": {1, 2, 3}}
}

func TestInputSchema_EnumParams_PublishesTheEnumOnItsProperty(t *testing.T) {
	t.Parallel()
	schema, err := (ActionSpec{Input: enumInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want map[string]any", schema["properties"])
	}
	typeSchema, ok := props["type"].(map[string]any)
	if !ok {
		t.Fatalf(`properties["type"] = %#v, want map[string]any`, props["type"])
	}
	enum, ok := typeSchema["enum"].([]any)
	if !ok || len(enum) != 3 {
		t.Fatalf(`properties["type"]["enum"] = %#v, want [1 2 3]`, typeSchema["enum"])
	}
}

// enumInputWithUnknownProperty implements EnumParams naming a JSON property
// the struct does not actually have — a typo, or a field renamed without its
// EnumParams entry following along.
type enumInputWithUnknownProperty struct {
	Type int `json:"type"`
}

func (enumInputWithUnknownProperty) EnumParams() map[string][]any {
	return map[string][]any{"typ": {1, 2, 3}} // "typ", not "type"
}

func TestInputSchema_EnumParams_UnknownProperty_IsRefused(t *testing.T) {
	t.Parallel()
	// Silently ignoring this would publish a schema that looks complete
	// while a caller-facing constraint quietly does nothing — exactly the
	// kind of divergence between what is published and what is true that
	// this project's generators exist to prevent.
	if _, err := (ActionSpec{Input: enumInputWithUnknownProperty{}}).InputSchema(); err == nil {
		t.Error("InputSchema() error = nil, want a refusal for an EnumParams entry naming an unknown property")
	}
}

func TestInputSchema_NonStructInput_IsRefused(t *testing.T) {
	t.Parallel()
	// A slice or a bare string would reflect into something no MCP client can
	// use as tool arguments. Refuse at declaration rather than emitting it.
	if _, err := (ActionSpec{Input: []string{}}).InputSchema(); err == nil {
		t.Error("InputSchema() error = nil, want a refusal for a non-struct input")
	}
}

func TestInputSchema_IsCachedPerType(t *testing.T) {
	t.Parallel()
	// 441 actions x three surfaces means the schema is requested thousands of
	// times at startup. Reflection is not cheap; the result is immutable.
	first, err := (ActionSpec{Input: sampleInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	second, _ := (ActionSpec{Input: sampleInput{}}).InputSchema()

	// Same content, and mutating one must not affect the other: a cache that
	// hands out the same mutable map lets one surface corrupt another's schema.
	first["injected"] = true
	if _, leaked := second["injected"]; leaked {
		t.Error("the cache hands out a shared mutable map: one caller can corrupt another's schema")
	}
}

// A top-level-only copy (e.g. maps.Clone on the outer map) satisfies
// IsCachedPerType above without protecting anything reachable through it: the
// nested "properties" map and the "required" slice would still be the very
// values held by the cache. This test mutates two levels deep and through a
// slice, which only a genuinely deep copy survives.
func TestInputSchema_IsCachedPerType_NestedMutationDoesNotLeak(t *testing.T) {
	t.Parallel()
	first, err := (ActionSpec{Input: sampleInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	second, err := (ActionSpec{Input: sampleInput{}}).InputSchema()
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}

	props, ok := first["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want map[string]any", first["properties"])
	}
	idSchema, ok := props["id"].(map[string]any)
	if !ok {
		t.Fatalf("properties[\"id\"] = %#v, want map[string]any", props["id"])
	}
	idSchema["type"] = "MUTATED"

	secondProps, _ := second["properties"].(map[string]any)
	secondID, _ := secondProps["id"].(map[string]any)
	if secondID["type"] == "MUTATED" {
		t.Error("mutating a nested property on first leaked into second: the copy is shallow")
	}

	if required, ok := first["required"].([]any); ok && len(required) > 0 {
		required[0] = "MUTATED"
		secondRequired, _ := second["required"].([]any)
		if len(secondRequired) > 0 && secondRequired[0] == "MUTATED" {
			t.Error("mutating the required slice on first leaked into second: the copy is shallow")
		}
	}
}
