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

	schemaCacheMu.Lock()
	schemaCache[t] = asMap
	schemaCacheMu.Unlock()

	return deepCopySchema(asMap), nil
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
