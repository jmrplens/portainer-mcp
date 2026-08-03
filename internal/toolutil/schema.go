package toolutil

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/google/jsonschema-go/jsonschema"
)

// schemaCache holds the JSON Schema already reflected for a given input type,
// keyed by the struct's reflect.Type. 441 actions across three surfaces mean
// this is requested thousands of times at startup; reflection is not free, so
// each type is reflected once.
//
// InputSchema never hands out a cached value directly: callers receive a deep
// copy, so one surface mutating its result cannot corrupt what another
// surface reads next.
var (
	schemaCacheMu sync.Mutex
	schemaCache   = map[reflect.Type]map[string]any{}
)

// emptyObjectSchema is published for an action whose Input is nil. It
// describes an object that accepts nothing, which is different from
// publishing no schema at all: a model reading "no schema" cannot tell
// "takes nothing" from "unspecified".
func emptyObjectSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"properties":           map[string]any{},
		"additionalProperties": false,
	}
}

// InputSchema returns the JSON Schema describing this action's parameters,
// reflected from Input. Every tool surface publishes this value, and every
// handler unmarshals into the same Input type, so the published shape and the
// parsed shape cannot drift.
//
// A nil Input yields an empty object schema. A non-struct, non-nil Input is
// refused: Validate should already have caught this at catalog build time,
// but InputSchema re-checks so it never silently reflects something no MCP
// client could use as tool arguments.
func (s ActionSpec) InputSchema() (map[string]any, error) {
	if s.Input == nil {
		return emptyObjectSchema(), nil
	}

	t := reflect.TypeOf(s.Input)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return nil, fmt.Errorf("input schema for %s: Input must be a struct, got %s", describeAction(s), t.Kind())
	}

	schemaCacheMu.Lock()
	cached, ok := schemaCache[t]
	schemaCacheMu.Unlock()
	if ok {
		return deepCopySchema(cached), nil
	}

	schema, err := jsonschema.ForType(t, nil)
	if err != nil {
		return nil, fmt.Errorf("input schema for %s: reflect %s: %w", describeAction(s), t, err)
	}
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("input schema for %s: encode %s: %w", describeAction(s), t, err)
	}
	var asMap map[string]any
	if err := json.Unmarshal(encoded, &asMap); err != nil {
		return nil, fmt.Errorf("input schema for %s: decode %s: %w", describeAction(s), t, err)
	}

	if enumer, ok := s.Input.(EnumParams); ok {
		if err := applyEnumParams(asMap, enumer.EnumParams()); err != nil {
			return nil, fmt.Errorf("input schema for %s: %w", describeAction(s), err)
		}
	}

	schemaCacheMu.Lock()
	schemaCache[t] = asMap
	schemaCacheMu.Unlock()

	return deepCopySchema(asMap), nil
}

// EnumParams optionally lets an Input type declare enum constraints per JSON
// property name.
//
// google/jsonschema-go v0.4.3's reflector (jsonschema.ForType, called by
// InputSchema below) recognises exactly one struct tag, "jsonschema", and
// only as free-form description text — see its infer.go, which sets
// fs.Description from the tag and nothing else. There is no tag syntax for
// restricting a field to a fixed set of values. An Input whose spec-derived
// shape includes an enum (cmd/gen_action_inputs measured 13 across 7
// operations in the vendored Business Edition specification) implements this
// interface instead of encoding a constraint the library would silently
// ignore if it were written as a tag.
type EnumParams interface {
	// EnumParams returns, for each constrained field's JSON property name,
	// the exact values that field may hold. A field name absent from the
	// returned map carries no enum constraint.
	EnumParams() map[string][]any
}

// applyEnumParams writes each entry of enums into schema's matching property
// as JSON Schema's "enum" keyword, mutating schema in place.
//
// It refuses — returns an error rather than silently no-op-ing — an entry
// naming a property the reflected schema does not have: a typo or a stale
// entry left after a field was renamed would otherwise publish a schema that
// looks complete while quietly failing to constrain anything, exactly the
// kind of silent divergence this project's generators exist to avoid.
func applyEnumParams(schema map[string]any, enums map[string][]any) error {
	if len(enums) == 0 {
		return nil
	}
	props, _ := schema["properties"].(map[string]any)
	for name, values := range enums {
		propSchema, ok := props[name].(map[string]any)
		if !ok {
			return fmt.Errorf("EnumParams declares %q, which is not a property of this schema", name)
		}
		propSchema["enum"] = values
	}
	return nil
}

// RequiredParams returns the "required" property names from a decoded JSON
// Schema object such as ActionSpec.InputSchema's return value, in schema
// order. Both the meta and dynamic surfaces call this rather than each
// decoding the "required" array themselves, so a change to that decoding
// cannot drift between the two.
func RequiredParams(schema map[string]any) []string {
	raw, _ := schema["required"].([]any)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// describeAction names the action for error messages, falling back to
// something legible when the spec is incomplete.
func describeAction(s ActionSpec) string {
	if s.Name != "" {
		return s.Name
	}
	return "(unnamed action)"
}

// deepCopySchema copies a decoded JSON Schema deeply enough that mutating the
// result can never reach the cached value: nested maps and slices - the only
// composite shapes encoding/json produces when decoding into map[string]any -
// are copied recursively, not just the top-level map.
func deepCopySchema(m map[string]any) map[string]any {
	copied, ok := deepCopyValue(m).(map[string]any)
	if !ok {
		return map[string]any{}
	}
	return copied
}

func deepCopyValue(v any) any {
	switch vv := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(vv))
		for k, val := range vv {
			out[k] = deepCopyValue(val)
		}
		return out
	case []any:
		out := make([]any, len(vv))
		for i, val := range vv {
			out[i] = deepCopyValue(val)
		}
		return out
	default:
		return v
	}
}
