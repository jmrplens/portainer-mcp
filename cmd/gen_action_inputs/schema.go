package main

import (
	"fmt"
	"sort"
	"strings"
)

// maxSchemaDepth bounds $ref/allOf resolution recursion. It exists to catch a
// genuine reference cycle (jsonschema-go's own reflector refuses cycles for
// the same reason — see internal/toolutil), not to cap legitimate nesting:
// registries.registryUpdatePayload's RegistryAccesses property alone is four
// $ref/map hops deep (RegistryAccesses -> map of RegistryAccessPolicies ->
// map of AccessPolicy, with an allOf hop at one level), and each hop costs
// this resolver two depth increments (one to resolve the property, one to
// follow its $ref or map value), so the bound is set generously above any
// real nesting depth found in the vendored specs rather than tightly against
// it.
const maxSchemaDepth = 40

// schemaNode is a resolved JSON Schema node: every $ref followed, every
// allOf branch merged, with the node's own directly-declared keywords
// (type, enum, description, properties, required) layered on top so that a
// sibling keyword next to allOf/$ref wins over whatever the referenced
// schema declared — the same precedence collections.registryCreatePayload's
// "Type" property relies on (a direct enum [1..8] narrower than the
// portainer.RegistryType component's own enum [0..8]).
type schemaNode struct {
	Type        string // "string", "integer", "number", "boolean", "array", "object"
	Description string
	Enum        []any
	Properties  []schemaProperty // ordered by property name, for deterministic output
	Items       *schemaNode      // set when Type == "array"
	MapValue    *schemaNode      // set when Type == "object" with a schema-typed additionalProperties and no declared properties
}

type schemaProperty struct {
	Name     string
	Schema   *schemaNode
	Required bool
}

// resolver resolves raw OpenAPI schema nodes against one document's
// components.schemas.
type resolver struct {
	doc *document
}

// resolve turns a raw schema node (as decoded from the vendored spec) into a
// schemaNode, following $ref and merging allOf. It refuses — returns an
// error, never a best-effort guess — a node using oneOf, anyOf or not: none
// of those compose into a single Go type without arbitrarily picking one
// branch, and picking one silently is exactly the kind of guess this
// generator exists to avoid.
func (r *resolver) resolve(raw map[string]any, depth int) (*schemaNode, error) {
	if depth > maxSchemaDepth {
		return nil, fmt.Errorf("schema nesting exceeds depth %d (possible cycle)", maxSchemaDepth)
	}
	for _, unsupported := range []string{"oneOf", "anyOf", "not"} {
		if _, ok := raw[unsupported]; ok {
			return nil, fmt.Errorf("%q is not expressible as a single Go type", unsupported)
		}
	}

	if ref, ok := raw["$ref"].(string); ok {
		target, err := r.lookupSchema(ref)
		if err != nil {
			return nil, err
		}
		return r.resolve(target, depth+1)
	}

	node := &schemaNode{}
	if allOf, ok := raw["allOf"].([]any); ok {
		for _, sub := range allOf {
			subMap, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			subNode, err := r.resolve(subMap, depth+1)
			if err != nil {
				return nil, err
			}
			mergeSchema(node, subNode)
		}
	}

	// Overlay this node's own directly-declared keywords. These are siblings
	// of $ref/allOf in the raw document and take precedence over whatever
	// was merged in above.
	if t, ok := raw["type"].(string); ok && t != "" {
		node.Type = t
	}
	if d, ok := raw["description"].(string); ok && d != "" {
		node.Description = d
	}
	if e, ok := raw["enum"].([]any); ok && len(e) > 0 {
		node.Enum = e
	}

	if propsRaw, ok := raw["properties"].(map[string]any); ok {
		required := map[string]bool{}
		if reqRaw, ok := raw["required"].([]any); ok {
			for _, r := range reqRaw {
				if s, ok := r.(string); ok {
					required[s] = true
				}
			}
		}
		names := make([]string, 0, len(propsRaw))
		for name := range propsRaw {
			names = append(names, name)
		}
		sort.Strings(names)
		if node.Type == "" {
			node.Type = "object"
		}
		// Union with whatever allOf already merged in above, rather than
		// discarding it: a node can legitimately declare its own "properties"
		// as a sibling of "allOf" ({"allOf": [{"$ref": "...Base"}],
		// "properties": {...}} is the real shape), and resetting node.Properties
		// to nil here dropped every property Base contributed, silently
		// shrinking the generated struct and the published schema to just the
		// node's own fields.
		byName := map[string]schemaProperty{}
		for _, p := range node.Properties {
			byName[p.Name] = p
		}
		for _, name := range names {
			propMap, ok := propsRaw[name].(map[string]any)
			if !ok {
				continue
			}
			propNode, err := r.resolve(propMap, depth+1)
			if err != nil {
				return nil, fmt.Errorf("property %q: %w", name, err)
			}
			byName[name] = schemaProperty{Name: name, Schema: propNode, Required: required[name]}
		}
		allNames := make([]string, 0, len(byName))
		for n := range byName {
			allNames = append(allNames, n)
		}
		sort.Strings(allNames)
		node.Properties = nil
		for _, n := range allNames {
			node.Properties = append(node.Properties, byName[n])
		}
	}

	if node.Type == "array" {
		if itemsRaw, ok := raw["items"].(map[string]any); ok {
			items, err := r.resolve(itemsRaw, depth+1)
			if err != nil {
				return nil, fmt.Errorf("array items: %w", err)
			}
			node.Items = items
		} else if node.Items == nil {
			// An allOf branch can supply both "type": "array" and "items"
			// (mergeSchema already copied node.Items in that case); only refuse
			// when neither this node's own "items" nor a merged one is present.
			return nil, fmt.Errorf("array schema has no items")
		}
	}

	if node.Type == "object" && len(node.Properties) == 0 {
		if ap, ok := raw["additionalProperties"].(map[string]any); ok {
			mapValue, err := r.resolve(ap, depth+1)
			if err != nil {
				return nil, fmt.Errorf("additionalProperties: %w", err)
			}
			node.MapValue = mapValue
		}
	}

	return node, nil
}

// lookupSchema resolves a "#/components/schemas/Name" reference.
func (r *resolver) lookupSchema(ref string) (map[string]any, error) {
	name, ok := strings.CutPrefix(ref, "#/components/schemas/")
	if !ok {
		return nil, fmt.Errorf("unsupported $ref %q: only #/components/schemas/* is resolved", ref)
	}
	target, ok := r.doc.schemas[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unresolved $ref %q", ref)
	}
	return target, nil
}

// mergeSchema folds src (one allOf branch, already resolved) into dst.
// Scalar keywords (Type, Description, Enum) are overwritten by src whenever
// src sets them non-empty — safe because the caller overlays the node's own
// directly-declared keywords afterwards, which is what actually decides
// precedence between allOf branches and a sibling keyword. Properties are
// unioned by name, later branches overriding earlier ones for the same name.
func mergeSchema(dst, src *schemaNode) {
	if src.Type != "" {
		dst.Type = src.Type
	}
	if src.Description != "" {
		dst.Description = src.Description
	}
	if len(src.Enum) > 0 {
		dst.Enum = src.Enum
	}
	if len(src.Properties) > 0 {
		byName := map[string]schemaProperty{}
		for _, p := range dst.Properties {
			byName[p.Name] = p
		}
		for _, p := range src.Properties {
			byName[p.Name] = p
		}
		names := make([]string, 0, len(byName))
		for n := range byName {
			names = append(names, n)
		}
		sort.Strings(names)
		dst.Properties = nil
		for _, n := range names {
			dst.Properties = append(dst.Properties, byName[n])
		}
	}
	if src.Items != nil {
		dst.Items = src.Items
	}
	if src.MapValue != nil {
		dst.MapValue = src.MapValue
	}
}
