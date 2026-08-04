package main

import (
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// This file is where task-4b's generation-time refusal moves to survive the
// freeze. cmd/gen_action_inputs's own credential.go refuses to emit a bare
// handler for an operation whose success response can carry a
// credential-shaped field (85 operations across 17 domains, measured against
// the vendored Business Edition specification). After the freeze the
// generator does not run against an owned domain any more (see main.go's
// domain-level overwrite refusal), so that refusal can no longer be the only
// thing standing between a hand-edited handler and a leaked credential. This
// audit is what stands there instead.
//
// # Why this does not import cmd/gen_action_inputs's resolver
//
// cmd/gen_action_inputs is `package main`; nothing outside it can import its
// types at all (see internal/specdiff's SpecOperation doc comment for the
// identical reimplement-vs-move decision, made for the identical reason).
// Depending on it here would also be backwards: this audit exists to keep
// gating on the vendored specification after the generator stops being load-
// bearing, and a scaffolding tool that could theoretically be deleted is not
// something a permanent build gate should depend on. The resolver below is
// deliberately narrower than the generator's: it resolves just enough of a
// response schema ($ref, allOf, properties, items, additionalProperties) to
// recover property *names* and their coarse shape, because
// toolutil.IsCredentialShapedField needs nothing more — it never needs to
// synthesize a Go type the way cmd/gen_action_inputs's schemaNode does for
// fields.go.
//
// "Credential-shaped" is defined exactly once, in
// toolutil.IsCredentialShapedName / toolutil.IsCredentialShapedField, and
// this resolver is the second of what is now three readers of that single
// definition (the generator's own credential.go, this audit, and the runtime
// walk toolutil.WalkForCredentialShapedFields performs against a real,
// populated response value in every domain's redaction-guard test). All three
// agree on what counts because none of them re-derives the predicate itself —
// only the walk that reaches a property name differs, by necessity, between a
// resolved OpenAPI schema and a reflected Go value.

// credMaxSchemaDepth bounds $ref/allOf resolution recursion, mirroring
// cmd/gen_action_inputs's maxSchemaDepth. Cycle detection (via resolving,
// below) is what actually terminates a genuine self-reference; this is
// defence in depth against a nesting shape cycle detection does not model,
// not a bound expected to fire against either vendored specification.
const credMaxSchemaDepth = 40

// credSchemaNode is a resolved JSON Schema node, keeping only what
// credential-shape detection needs: a type (to rule out a shape that can
// never carry a secret), named properties (object), an item shape (array) and
// a value shape (map). Compare cmd/gen_action_inputs's schemaNode, which
// additionally carries Description, Enum and Required — none of which this
// audit's narrower question needs.
type credSchemaNode struct {
	Type         string
	Properties   []credSchemaProperty
	Items        *credSchemaNode
	MapValue     *credSchemaNode
	TruncatedRef string
}

type credSchemaProperty struct {
	Name   string
	Schema *credSchemaNode
}

// credResolver resolves raw OpenAPI schema nodes against one document's
// components.schemas — the identical $ref/allOf/cycle handling
// cmd/gen_action_inputs's resolver performs, reimplemented narrowly (see this
// file's own package doc comment for why independently, not imported).
type credResolver struct {
	schemas   map[string]any
	resolving map[string]int
}

// resolve mirrors cmd/gen_action_inputs's resolver.resolve exactly in
// behaviour — same refusal for oneOf/anyOf/not, same cycle truncation via a
// TruncatedRef marker, sound for the identical reason: a cycle repeats the
// same schema, so every distinct property name reachable through a second lap
// is already reachable on the first, and the set of distinct names is all
// this walk ever collects.
func (r *credResolver) resolve(raw map[string]any, depth int) (*credSchemaNode, error) {
	if depth > credMaxSchemaDepth {
		return nil, fmt.Errorf("schema nesting exceeds depth %d (possible cycle)", credMaxSchemaDepth)
	}
	for _, unsupported := range []string{"oneOf", "anyOf", "not"} {
		if _, ok := raw[unsupported]; ok {
			return nil, fmt.Errorf("%q is not expressible as a single credential-shape check", unsupported)
		}
	}

	if ref, ok := raw["$ref"].(string); ok {
		if r.resolving[ref] > 0 {
			return &credSchemaNode{Type: "object", TruncatedRef: ref}, nil
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

	node := &credSchemaNode{}
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
			mergeCredSchema(node, subNode)
		}
	}

	if t, ok := raw["type"].(string); ok && t != "" {
		node.Type = t
	}

	if propsRaw, ok := raw["properties"].(map[string]any); ok {
		names := make([]string, 0, len(propsRaw))
		for name := range propsRaw {
			names = append(names, name)
		}
		sort.Strings(names)
		if node.Type == "" {
			node.Type = "object"
		}
		byName := map[string]credSchemaProperty{}
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
			byName[name] = credSchemaProperty{Name: name, Schema: propNode}
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

func (r *credResolver) lookupSchema(ref string) (map[string]any, error) {
	const prefix = "#/components/schemas/"
	name, ok := stripPrefix(ref, prefix)
	if !ok {
		return nil, fmt.Errorf("unsupported $ref %q: only %s* is resolved", ref, prefix)
	}
	target, ok := r.schemas[name].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unresolved $ref %q", ref)
	}
	return target, nil
}

func stripPrefix(s, prefix string) (string, bool) {
	if len(s) < len(prefix) || s[:len(prefix)] != prefix {
		return "", false
	}
	return s[len(prefix):], true
}

// mergeCredSchema folds src (one resolved allOf branch) into dst, mirroring
// cmd/gen_action_inputs's mergeSchema for the fields this narrower node
// carries.
func mergeCredSchema(dst, src *credSchemaNode) {
	if src.Type != "" {
		dst.Type = src.Type
	}
	if len(src.Properties) > 0 {
		byName := map[string]credSchemaProperty{}
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
	if src.TruncatedRef != "" {
		dst.TruncatedRef = src.TruncatedRef
	}
}

// successResponseSchemas returns the raw (unresolved) schema of every 2xx
// response in responses that declares an application/json body, mirroring
// cmd/gen_action_inputs's identical function and skipping a 2xx with no body
// (a 204, or a 200 with no "content") for the identical reason.
func successResponseSchemas(responses map[string]map[string]any) []map[string]any {
	var schemas []map[string]any
	codes := make([]string, 0, len(responses))
	for code := range responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if len(code) == 0 || code[0] != '2' {
			continue
		}
		resp := responses[code]
		content, _ := resp["content"].(map[string]any)
		if content == nil {
			continue
		}
		aj, _ := content["application/json"].(map[string]any)
		if aj == nil {
			continue
		}
		schema, _ := aj["schema"].(map[string]any)
		if schema == nil {
			continue
		}
		schemas = append(schemas, schema)
	}
	return schemas
}

// credentialShapedFieldNames walks node (already resolved) and returns the
// name of every property whose name and coarse shape matches
// toolutil.IsCredentialShapedField, at any depth reachable through an
// object's properties, an array's items or a map's value.
func credentialShapedFieldNames(node *credSchemaNode, depth int) []string {
	if node == nil || depth > credMaxSchemaDepth {
		return nil
	}
	var out []string
	for _, p := range node.Properties {
		shape := toolutil.ShapeUnknown
		if p.Schema != nil {
			shape = toolutil.FieldShapeOfJSONType(p.Schema.Type)
		}
		if toolutil.IsCredentialShapedField(p.Name, shape) {
			out = append(out, p.Name)
		}
		out = append(out, credentialShapedFieldNames(p.Schema, depth+1)...)
	}
	if node.Items != nil {
		out = append(out, credentialShapedFieldNames(node.Items, depth+1)...)
	}
	if node.MapValue != nil {
		out = append(out, credentialShapedFieldNames(node.MapValue, depth+1)...)
	}
	return out
}

// responseCredentialFields resolves every success-response schema op
// declares and returns the sorted, de-duplicated union of every
// credential-shaped field name reachable in any of them, or nil when none is.
func responseCredentialFields(responses map[string]map[string]any, schemas map[string]any) ([]string, error) {
	res := &credResolver{schemas: schemas}
	seen := map[string]bool{}
	var out []string
	for _, raw := range successResponseSchemas(responses) {
		node, err := res.resolve(raw, 0)
		if err != nil {
			return nil, fmt.Errorf("resolve response schema: %w", err)
		}
		for _, name := range credentialShapedFieldNames(node, 0) {
			if !seen[name] {
				seen[name] = true
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// redactionWrapperName is the naming convention a domain's own file must
// follow to supply a redaction wrapper for operationID, mirroring
// cmd/gen_action_inputs's identical function exactly: "redact" + OperationID,
// e.g. "redactRegistryInspect". Both must agree, since a wrapper the
// generator accepted at scaffold time must be the identical name this audit
// looks for afterwards, or every scaffolded domain would immediately fail the
// audit that is supposed to hold it steady.
func redactionWrapperName(operationID string) string {
	return "redact" + operationID
}
