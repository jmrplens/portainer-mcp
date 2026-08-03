package main

import (
	"fmt"
	"sort"
	"strings"
)

// maxSchemaDepth bounds $ref/allOf resolution recursion. Since resolve()
// detects genuine $ref cycles directly (see the resolving stack below), this
// is no longer the mechanism that terminates a cycle — it is a backstop
// against unbounded nesting from some shape cycle detection does not model,
// and it is not expected to fire. It does not cap legitimate nesting:
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
	// TruncatedRef is non-empty when this node stands in for a $ref that was
	// already being resolved further up the current resolution stack — a
	// genuine cycle in the vendored document, not deep nesting. The node
	// carries no properties of its own: it is a marker, and every consumer
	// must decide explicitly what to do with one rather than mistake it for a
	// legitimately empty object. See resolve's doc comment for why truncating
	// is sound for a response walk and refused for a request body.
	TruncatedRef string
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
	// resolving counts how many times each $ref is currently open on the
	// resolution stack. Resolution is depth-first and single-threaded, so
	// incrementing before recursing and decrementing after is exact: a ref
	// found with a non-zero count is, by construction, one the current branch
	// is already inside. Reference-counted rather than a plain set because the
	// same component can legitimately appear twice in one branch through
	// different parents without that being a cycle in the branch itself.
	resolving map[string]int
}

// resolve turns a raw schema node (as decoded from the vendored spec) into a
// schemaNode, following $ref and merging allOf. It refuses — returns an
// error, never a best-effort guess — a node using oneOf, anyOf or not: none
// of those compose into a single Go type without arbitrarily picking one
// branch, and picking one silently is exactly the kind of guess this
// generator exists to avoid.
//
// # Cycles
//
// The vendored Business Edition document contains at least one genuine
// self-reference: portaineree.EdgeConfig declares a "prev" property that is a
// $ref back to portaineree.EdgeConfig itself, so the graph is infinite, not
// merely deep. Before cycle detection existed, the only thing that terminated
// that recursion was maxSchemaDepth, which meant EdgeConfigInspect and
// EdgeConfigList could not be resolved at all: every attempt returned "schema
// nesting exceeds depth 40". Raising the bound cannot fix an infinite graph —
// it only buys more recursion before the same error — so resolve detects the
// cycle directly instead. A $ref already open on the current resolution stack
// yields a marker node carrying TruncatedRef and no properties.
//
// Truncating is sound for the consumer that walks a *response* schema looking
// for credential-shaped property names (credentialShapedFieldPaths): a cycle
// repeats the identical schema, so every property name reachable through the
// second lap is, by construction, already reachable on the first. The set of
// distinct names — which is all that walk collects — is unchanged.
//
// Truncating is *not* sound for building a Go type, because a struct field
// standing for the cut edge has no type to be given. typeOf therefore refuses
// a truncated node by name rather than silently emitting an empty struct; see
// its own doc comment. In practice no request body in either vendored spec is
// cyclic, so that refusal is a guard, not a limitation anything hits today.
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
		if r.resolving[ref] > 0 {
			return &schemaNode{Type: "object", TruncatedRef: ref}, nil
		}
		target, err := r.lookupSchema(ref)
		if err != nil {
			return nil, err
		}
		if r.resolving == nil {
			r.resolving = map[string]int{}
		}
		r.resolving[ref]++
		node, err := r.resolve(target, depth+1)
		r.resolving[ref]--
		if r.resolving[ref] == 0 {
			delete(r.resolving, ref)
		}
		return node, err
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
