package toolutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
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
	// A spec carrying a baked-in schema (see WithInputSchema) always returns
	// it, in preference to reflecting Input and consulting schemaCache below:
	// actioncatalog.Build bakes in the edition-pruned shape for exactly the
	// specs whose Input type declares an `edition:"EE"` field, and every
	// later reader of that spec — every surface, and ValidateInput through
	// resolvedInputSchema below — must see the pruned shape with nothing
	// further to call. deepCopySchema, not the map itself, for the identical
	// reason the ordinary reflected path never hands out schemaCache's own
	// entry: a caller mutating its result must never reach what another
	// caller (or a later call to this same method) sees next.
	if s.prunedInputSchema != nil {
		return deepCopySchema(s.prunedInputSchema), nil
	}
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

	if err := applyTypeConstraints(t, asMap, map[reflect.Type]bool{}); err != nil {
		return nil, fmt.Errorf("input schema for %s: %w", describeAction(s), err)
	}

	schemaCacheMu.Lock()
	schemaCache[t] = asMap
	schemaCacheMu.Unlock()

	return deepCopySchema(asMap), nil
}

// WithInputSchema returns a copy of s whose InputSchema — and, through it,
// ValidateInput — always returns schema instead of reflecting s.Input and
// consulting the process-wide schemaCache/resolvedSchemaCache below.
//
// actioncatalog.Build is the only intended caller: for a spec whose Input
// type declares an `edition:"EE"` field, Build computes the edition-pruned
// schema once, when the catalog is constructed, and bakes it into the spec
// through this method before the spec is ever stored in the catalog or
// handed to a surface. Every later reader of that spec — Catalog.Actions,
// ByDomain and Lookup, and therefore every one of the three model-facing
// surfaces, plus Execute's own ValidateInput call — then sees the pruned
// shape with nothing further to call: there is no unpruned path left to
// take, unlike an opt-in accessor a fourth reader could simply not call.
//
// schema is stored as-is, not copied again on the way in: the caller must
// already hand over an independent map it owns — Build always does, since it
// is itself built from spec.InputSchema()'s own already-independent return
// value (see that method's doc comment above) — because WithInputSchema does
// not defensively clone it a second time.
func (s ActionSpec) WithInputSchema(schema map[string]any) ActionSpec {
	s.prunedInputSchema = schema
	return s
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

// MinimumParams optionally lets an Input type declare a lower-bound
// constraint per JSON property name, the same way EnumParams declares an
// enum constraint above: google/jsonschema-go v0.4.3's reflector has no
// struct-tag syntax for "minimum" either, so an Input whose spec-derived
// shape needs one implements this interface instead of encoding a constraint
// the library would silently ignore if it were written as a tag.
//
// Every constraint an Input declares this way is cmd/gen_action_inputs's own
// addition, never something the vendored specification itself states — see
// that generator's isIdentifierPathParam (fields.go) for exactly which
// fields get one and why: none of the 285 integer path parameters in the
// vendored Business Edition specification declare a "minimum" of their own,
// yet every one of them that names a Portainer resource identifier is never
// valid at zero or below. Declaring that constraint once, here, means
// tools.Execute's central validation refuses a non-positive identifier
// before any handler runs and before any network call — the same refusal
// every hand-written "id must be a positive integer" guard clause across
// this project's domains already enacted individually, now expressed once
// where every surface's schema validation already looks.
type MinimumParams interface {
	// MinimumParams returns, for each constrained field's JSON property name,
	// the smallest value that field may hold. A field name absent from the
	// returned map carries no lower-bound constraint.
	MinimumParams() map[string]int
}

// applyMinimumParams writes each entry of minimums into schema's matching
// property as JSON Schema's "minimum" keyword, mutating schema in place.
//
// It refuses — returns an error rather than silently no-op-ing — an entry
// naming a property the reflected schema does not have, for the identical
// reason applyEnumParams does: a typo or a stale entry left after a field
// was renamed would otherwise publish a schema that looks complete while
// quietly failing to constrain anything.
func applyMinimumParams(schema map[string]any, minimums map[string]int) error {
	if len(minimums) == 0 {
		return nil
	}
	props, _ := schema["properties"].(map[string]any)
	for name, minimum := range minimums {
		propSchema, ok := props[name].(map[string]any)
		if !ok {
			return fmt.Errorf("MinimumParams declares %q, which is not a property of this schema", name)
		}
		propSchema["minimum"] = minimum
	}
	return nil
}

// enumParamsInterface and minimumParamsInterface are the two constraint
// interfaces applyTypeConstraints looks for on every struct type it reaches,
// resolved once rather than per node.
var (
	enumParamsInterface    = reflect.TypeFor[EnumParams]()
	minimumParamsInterface = reflect.TypeFor[MinimumParams]()
)

// applyTypeConstraints walks the Go type tree of an Input alongside the JSON
// Schema jsonschema.ForType reflected from it, applying the EnumParams and
// MinimumParams constraints declared by *every* struct type it reaches — not
// only the top-level Input type.
//
// InputSchema used to assert both interfaces on the top-level Input value
// alone:
//
//	if enumer, ok := s.Input.(EnumParams); ok { ... }
//
// which meant a nested struct type's generated EnumParams()/MinimumParams()
// was never consulted, however faithfully cmd/gen_action_inputs had emitted
// it. The measured consequence (docs/api-divergences.md §9.5) was that
// custom_templates.create_repository published no "enum" at all for
// edgeSettings.relativePathSettings.perDeviceConfigsMatchType, and
// ValidateInput — which resolves the same map — enforced none either, so a
// model got no help filling a field whose legal values the specification
// states, and a wrong value came back as a Portainer error rather than a
// catalog refusal.
//
// # What the walk has to match
//
// The shape of the reflected schema, exactly, or a constraint lands on the
// wrong node or on nothing. google/jsonschema-go v0.4.3 inlines everything —
// there are no $defs or $ref to follow — and renders:
//
//   - a struct as an object with "properties";
//   - a pointer as its element's schema with "null" added to "type", so a
//     *T property and a T property carry the same "properties" map;
//   - a slice or array as an array with the element's schema under "items";
//   - a map with string keys as an object with the value's schema under
//     "additionalProperties";
//   - an *embedded* struct's fields flattened into the embedding struct's own
//     "properties" (its forType loop skips every anonymous field and emits the
//     promoted leaves reflect.VisibleFields hands it).
//
// The last of those is why an embedded type's own constraints are applied to
// the *embedding* node rather than to a sub-schema: there is no sub-schema for
// an embedded struct to own. reflect.VisibleFields already reports every level
// of embedding as an anonymous field of the outermost struct, so a struct
// embedded inside an embedded struct is reached without recursing through it.
//
// seen guards against a type cycle. jsonschema.ForType refuses a cyclic type
// outright, so a cycle cannot reach this walk today through InputSchema; the
// guard is there so that this function cannot be the thing that hangs if that
// ever changes.
func applyTypeConstraints(t reflect.Type, node map[string]any, seen map[reflect.Type]bool) error {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		items, ok := node["items"].(map[string]any)
		if !ok {
			return nil
		}
		return applyTypeConstraints(t.Elem(), items, seen)
	case reflect.Map:
		// A struct node's "additionalProperties" is the JSON literal false, not
		// an object, so this type assertion is also what keeps a struct's own
		// "no unknown properties" marker from being walked as a map value.
		values, ok := node["additionalProperties"].(map[string]any)
		if !ok {
			return nil
		}
		return applyTypeConstraints(t.Elem(), values, seen)
	case reflect.Struct:
		return applyStructConstraints(t, node, seen)
	default:
		return nil
	}
}

// applyStructConstraints applies t's own declared constraints to node, then
// walks t's fields into node's matching sub-schemas.
func applyStructConstraints(t reflect.Type, node map[string]any, seen map[reflect.Type]bool) error {
	if t.Name() != "" {
		if seen[t] {
			return nil
		}
		seen[t] = true
		defer delete(seen, t)
	}
	if err := applyDeclaredConstraints(t, node); err != nil {
		return err
	}

	props, _ := node["properties"].(map[string]any)
	for _, field := range reflect.VisibleFields(t) {
		if field.Anonymous {
			embedded := field.Type
			for embedded.Kind() == reflect.Pointer {
				embedded = embedded.Elem()
			}
			if embedded.Kind() != reflect.Struct {
				continue
			}
			// An embedded struct's fields are this node's own properties, so
			// its constraints are applied here. Its fields are not walked
			// again: VisibleFields lists them, and any struct they embed in
			// turn, as further entries of this same loop.
			if err := applyDeclaredConstraints(embedded, node); err != nil {
				return err
			}
			continue
		}
		name, ok := jsonPropertyName(field)
		if !ok {
			continue
		}
		child, ok := props[name].(map[string]any)
		if !ok {
			continue
		}
		if err := applyTypeConstraints(field.Type, child, seen); err != nil {
			return fmt.Errorf("property %q: %w", name, err)
		}
	}
	return nil
}

// applyDeclaredConstraints applies whichever of the two constraint interfaces
// t implements to node, which must be the schema node standing for t's own
// properties.
//
// Both a value receiver and a pointer receiver are honoured: the generated
// methods use value receivers, but a hand-written Input declaring one on *T
// would otherwise be silently ignored — the same class of silence this whole
// function exists to end.
func applyDeclaredConstraints(t reflect.Type, node map[string]any) error {
	if v, ok := valueImplementing(t, enumParamsInterface); ok {
		if err := applyEnumParams(node, v.(EnumParams).EnumParams()); err != nil {
			return err
		}
	}
	if v, ok := valueImplementing(t, minimumParamsInterface); ok {
		if err := applyMinimumParams(node, v.(MinimumParams).MinimumParams()); err != nil {
			return err
		}
	}
	return nil
}

// valueImplementing returns a value satisfying iface, built from t or from *t,
// or reports that neither does.
func valueImplementing(t, iface reflect.Type) (any, bool) {
	if t.Implements(iface) {
		return reflect.Zero(t).Interface(), true
	}
	if pointer := reflect.PointerTo(t); pointer.Implements(iface) {
		return reflect.New(t).Interface(), true
	}
	return nil, false
}

// jsonPropertyName returns the JSON property name field is reflected under,
// mirroring google/jsonschema-go's own fieldJSONInfo: an unexported field and
// a `json:"-"` field have no property, a tag name overrides the Go field name,
// and `json:",omitempty"` (an empty name with options) keeps the Go field
// name. A name computed any other way would look up a property that the
// reflected schema does not have, and silently skip the constraint.
func jsonPropertyName(field reflect.StructField) (string, bool) {
	if !field.IsExported() {
		return "", false
	}
	tag, ok := field.Tag.Lookup("json")
	if !ok {
		return field.Name, true
	}
	name, _, hasOptions := strings.Cut(tag, ",")
	if name == "-" && !hasOptions {
		return "", false
	}
	if name == "" {
		return field.Name, true
	}
	return name, true
}

// resolvedSchemaCache holds the *jsonschema.Resolved used to validate raw call
// arguments, keyed the same way as schemaCache. It is a separate cache because
// the two serve different consumers: schemaCache hands callers a decoded map
// they own and may mutate (surfaces publish it as-is), while validation needs
// the library's own resolved *jsonschema.Schema, which a map cannot represent.
//
// The zero reflect.Type (Input == nil) is a valid map key here: every
// nil-Input action shares the identical empty-object schema, so they share one
// cache entry rather than each resolving it again.
var (
	resolvedSchemaCacheMu sync.Mutex
	resolvedSchemaCache   = map[reflect.Type]*jsonschema.Resolved{}
)

// resolvedInputSchema returns the resolved schema ValidateInput checks raw
// arguments against, built from the same map InputSchema publishes so the two
// can never describe different shapes.
//
// A spec carrying a baked-in, edition-pruned schema (s.prunedInputSchema, set
// by WithInputSchema) never reads or writes resolvedSchemaCache below: two
// ActionSpec values built from the identical Go Input type — one from a
// Community catalog, one from a Business catalog — legitimately resolve to
// two different schemas once per-field edition pruning applies, and that
// cache is keyed only by reflect.Type, with no notion of which catalog's
// edition produced this particular spec. Caching the Community build's
// resolved schema under the shared type key would then leak into the very
// next Business build that happens to share the Go Input type, silently
// re-admitting (or wrongly forbidding) fields regardless of which edition
// actually asked — the identical cache-poisoning trap
// TestUnit_CatalogInputSchema_BuildingCEThenEE_DoesNotLoseFieldForEE already
// guards for InputSchema's own map cache. Resolving fresh here instead is the
// only way to keep the two builds from clobbering each other's entry;
// actioncatalog.Build runs once at server startup and ValidateInput runs once
// per tool invocation, so the cost is one JSON encode/decode/resolve per call
// for exactly the actions this mechanism gates, not a hot loop.
func (s ActionSpec) resolvedInputSchema() (*jsonschema.Resolved, error) {
	if s.prunedInputSchema != nil {
		schemaMap, err := s.InputSchema()
		if err != nil {
			return nil, err
		}
		return resolveSchemaMap(schemaMap)
	}

	var key reflect.Type
	if s.Input != nil {
		key = reflect.TypeOf(s.Input)
		for key.Kind() == reflect.Pointer {
			key = key.Elem()
		}
	}

	resolvedSchemaCacheMu.Lock()
	cached, ok := resolvedSchemaCache[key]
	resolvedSchemaCacheMu.Unlock()
	if ok {
		return cached, nil
	}

	schemaMap, err := s.InputSchema()
	if err != nil {
		return nil, err
	}
	resolved, err := resolveSchemaMap(schemaMap)
	if err != nil {
		return nil, err
	}

	resolvedSchemaCacheMu.Lock()
	resolvedSchemaCache[key] = resolved
	resolvedSchemaCacheMu.Unlock()

	return resolved, nil
}

// resolveSchemaMap encodes schemaMap (a decoded JSON Schema object, such as
// InputSchema's own return value) and resolves it into the form
// jsonschema.Resolved.Validate checks raw arguments against. Factored out of
// resolvedInputSchema so both its cached path (an ordinary, unpruned Input
// type) and its uncached path (a spec carrying a baked-in, edition-pruned
// schema — see that method's own doc comment on why those must never share
// resolvedSchemaCache) resolve identically.
func resolveSchemaMap(schemaMap map[string]any) (*jsonschema.Resolved, error) {
	encoded, err := json.Marshal(schemaMap)
	if err != nil {
		return nil, fmt.Errorf("encode schema for validation: %w", err)
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(encoded, &schema); err != nil {
		return nil, fmt.Errorf("decode schema for validation: %w", err)
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return nil, fmt.Errorf("resolve schema for validation: %w", err)
	}
	return resolved, nil
}

// ValidateInput checks raw JSON call arguments against this action's
// published schema — the same check the MCP SDK performs automatically for a
// typed tool's arguments (see google/jsonschema-go's Resolved.Validate, which
// the SDK's applySchema calls before a typed handler ever runs).
//
// Every tool surface routes through tools.Execute, and Execute calls this
// once, so a missing required field or an out-of-enum value is refused
// identically regardless of which surface accepted the call — rather than
// being caught only where the SDK happens to validate a surface's own typed
// wrapper automatically, and silently passed through everywhere else.
//
// raw should already be a normalized JSON object (tools.Execute normalizes a
// caller-omitted or null input to "{}" before calling this); a raw value that
// still is not valid JSON is refused as such rather than silently treated as
// {}, since a wire-level parse failure is not the same defect as a schema
// mismatch.
func (s ActionSpec) ValidateInput(raw json.RawMessage) error {
	resolved, err := s.resolvedInputSchema()
	if err != nil {
		return fmt.Errorf("input schema for %s: %w", describeAction(s), err)
	}

	decoded := map[string]any{}
	if len(bytes.TrimSpace(raw)) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return fmt.Errorf("%s: arguments are not a valid JSON object: %w", describeAction(s), err)
		}
	}

	var instance any = decoded
	if err := resolved.Validate(&instance); err != nil {
		return fmt.Errorf("%s: %w", describeAction(s), err)
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
