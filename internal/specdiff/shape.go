package specdiff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// ShapeFromCatalog derives an OperationShape from a declared action's
// reflected InputSchema — the same map[string]any every tool surface
// publishes to a model — rather than from spec.Input's Go type directly or
// from any parallel bookkeeping this package might otherwise keep about the
// action. That is the whole point of comparing this shape against
// ShapeFromSpec's: a drift audit built on it checks what a model is actually
// told, not what some other derivation believes the schema says.
func ShapeFromCatalog(spec toolutil.ActionSpec) (OperationShape, error) {
	schema, err := spec.InputSchema()
	if err != nil {
		return OperationShape{}, fmt.Errorf("shape from catalog for %s: %w", spec.OperationID, err)
	}

	required := make(map[string]bool, len(schema))
	for _, name := range toolutil.RequiredParams(schema) {
		required[name] = true
	}

	props, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)

	fields := make([]FieldShape, 0, len(names))
	for _, name := range names {
		propSchema, ok := props[name].(map[string]any)
		if !ok {
			return OperationShape{}, fmt.Errorf("shape from catalog for %s: property %q is not an object schema", spec.OperationID, name)
		}
		description, _ := propSchema["description"].(string)
		fields = append(fields, FieldShape{
			JSONName:    name,
			Type:        canonicalType(propSchema["type"]),
			Required:    required[name],
			Enum:        stringifyEnum(propSchema["enum"]),
			Description: description,
			// Origin is deliberately left "": see FieldShape.Origin's doc
			// comment for why the published schema has nothing to read it
			// back from.
		})
	}

	// Method and Path are left "" for the same reason: toolutil.ActionSpec
	// carries no route information, only OperationID — the one link to the
	// vendored specification a catalog-derived shape has (see ActionSpec's
	// own doc comment on OperationID). A caller that needs Method/Path for a
	// matched pair already has them from the spec-derived side of the
	// comparison.
	return OperationShape{OperationID: spec.OperationID, Fields: fields}, nil
}

// canonicalType collapses a decoded JSON Schema "type" keyword down to the
// one substantive type it names. google/jsonschema-go's reflector (which
// ActionSpec.InputSchema calls) renders every Go pointer or slice field's
// type as a two-element array such as ["null", "string"], regardless of
// whether the field is required — its own way of saying "this may be
// absent", distinct from the vendored specification's convention of the same
// fact (omission from the "required" list, with "type" always a single
// string; verified against every parameter and body property in both
// vendored specs). Comparing those two spellings of "optional" as if they
// were different types would report ChangeType for a large fraction of every
// optional field in the catalog, for no real difference at all.
func canonicalType(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case []any:
		for _, e := range t {
			if s, ok := e.(string); ok && s != "null" {
				return s
			}
		}
		return ""
	default:
		return ""
	}
}

// stringifyEnum renders a decoded JSON Schema "enum" array as FieldShape.Enum
// expects: each value through fmt.Sprint, so an integer enum decoded as
// float64 (json.Unmarshal's own convention for every JSON number, on both the
// catalog and the spec side) and a string enum compare identically regardless
// of which producer built the FieldShape. Returns nil for anything that is
// not a non-empty array — an absent "enum" key included — so a field with no
// constraint carries a nil slice rather than an empty one indistinguishable
// from "constrained to nothing".
func stringifyEnum(v any) []string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, e := range arr {
		out = append(out, fmt.Sprint(e))
	}
	return out
}

// maxSchemaDepth bounds resolveSchema's $ref/allOf recursion, mirroring
// cmd/gen_action_inputs's identical constant and its own doc comment: genuine
// cycles are caught directly (the resolving map below), so this is a backstop
// against unbounded nesting, not the mechanism that terminates a cycle.
const maxSchemaDepth = 40

// resolvedNode is a schema node resolved just far enough to answer
// ShapeFromSpec's questions about it: its own JSON Schema type, description
// and enum — following $ref and merging allOf, with a node's own directly
// declared keywords winning over whatever a referenced or merged schema
// contributed, the identical precedence cmd/gen_action_inputs's schemaNode
// documents — plus, for an object node, its properties and required set. Each
// property is left as its own raw, unresolved node: ShapeFromSpec resolves a
// property only far enough to read that property's own type/description/
// enum, never its nested properties, array items or map values, because
// OperationShape's Fields are flat (see OperationShape's doc comment).
//
// This is deliberately smaller than cmd/gen_action_inputs's own resolver,
// which builds a full nested Go-type tree because it emits source code; see
// LoadSpecOperation's doc comment for why that resolver is reimplemented
// rather than reused.
type resolvedNode struct {
	Type        string
	Description string
	Enum        []any
	Properties  map[string]map[string]any
	Required    map[string]bool
}

// resolveSchema resolves one raw schema node against schemas
// (components.schemas), following $ref and merging allOf. resolving tracks
// which $ref names are currently open on the resolution stack, exactly as
// cmd/gen_action_inputs's resolver does, to detect a genuine cycle
// (portaineree.EdgeConfig's self-referencing "prev" property is the real one
// in the vendored spec) rather than recursing until maxSchemaDepth trips.
//
// Unlike that resolver, a detected cycle here returns an empty object rather
// than a refusal: ShapeFromSpec never reads anything past the cut edge (no
// nested properties, no array items), so there is no missing detail a
// refusal would be protecting a caller from.
func resolveSchema(schemas map[string]any, raw map[string]any, resolving map[string]int, depth int) (resolvedNode, error) {
	if depth > maxSchemaDepth {
		return resolvedNode{}, fmt.Errorf("schema nesting exceeds depth %d (possible cycle)", maxSchemaDepth)
	}
	for _, unsupported := range []string{"oneOf", "anyOf", "not"} {
		if _, ok := raw[unsupported]; ok {
			return resolvedNode{}, fmt.Errorf("%q is not expressible as a single field shape", unsupported)
		}
	}

	if ref, ok := raw["$ref"].(string); ok {
		if resolving[ref] > 0 {
			return resolvedNode{Type: "object"}, nil
		}
		name, ok := strings.CutPrefix(ref, "#/components/schemas/")
		if !ok {
			return resolvedNode{}, fmt.Errorf("unsupported $ref %q: only #/components/schemas/* is resolved", ref)
		}
		target, ok := schemas[name].(map[string]any)
		if !ok {
			return resolvedNode{}, fmt.Errorf("unresolved $ref %q", ref)
		}
		resolving[ref]++
		node, err := resolveSchema(schemas, target, resolving, depth+1)
		resolving[ref]--
		if resolving[ref] == 0 {
			delete(resolving, ref)
		}
		return node, err
	}

	var node resolvedNode
	if allOf, ok := raw["allOf"].([]any); ok {
		for _, sub := range allOf {
			subMap, ok := sub.(map[string]any)
			if !ok {
				continue
			}
			subNode, err := resolveSchema(schemas, subMap, resolving, depth+1)
			if err != nil {
				return resolvedNode{}, err
			}
			mergeResolved(&node, subNode)
		}
	}

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
		if node.Properties == nil {
			node.Properties = map[string]map[string]any{}
		}
		if node.Type == "" {
			node.Type = "object"
		}
		for name, p := range propsRaw {
			if pm, ok := p.(map[string]any); ok {
				node.Properties[name] = pm
			}
		}
		if reqRaw, ok := raw["required"].([]any); ok {
			if node.Required == nil {
				node.Required = map[string]bool{}
			}
			for _, r := range reqRaw {
				if s, ok := r.(string); ok {
					node.Required[s] = true
				}
			}
		}
	}

	return node, nil
}

// mergeResolved folds src (one resolved allOf branch) into dst. Scalar
// keywords are overwritten by src whenever src sets them non-empty — safe
// because resolveSchema overlays the node's own directly-declared keywords
// afterwards, which is what actually decides precedence between an allOf
// branch and a sibling keyword. Properties and required are unioned, later
// branches overriding earlier ones for the same property name — mirroring
// cmd/gen_action_inputs's mergeSchema exactly, for the identical reason.
func mergeResolved(dst *resolvedNode, src resolvedNode) {
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
		if dst.Properties == nil {
			dst.Properties = map[string]map[string]any{}
		}
		for name, p := range src.Properties {
			dst.Properties[name] = p
		}
	}
	if len(src.Required) > 0 {
		if dst.Required == nil {
			dst.Required = map[string]bool{}
		}
		for name := range src.Required {
			dst.Required[name] = true
		}
	}
}

// requestBodySchemaNode returns op's request body's single application/json
// schema node, resolving one components.requestBodies $ref first if the body
// itself is declared that way — the same indirection
// cmd/gen_action_inputs's requestBodySchema resolves. Returns nil, nil for an
// operation with no body, or one whose body declares no application/json
// content at all (a multipart-only body is the real example,
// CustomTemplateCreate in the vendored spec): ShapeFromSpec has no non-JSON
// content to flatten into fields either way, so there is nothing to
// distinguish that from "no body" for this package's purposes.
func requestBodySchemaNode(op SpecOperation) (map[string]any, error) {
	rb := op.RequestBody
	if rb == nil {
		return nil, nil
	}
	if ref, ok := rb["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/requestBodies/")
		resolved, ok := op.RequestBodies[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unresolved requestBody $ref %q", ref)
		}
		rb = resolved
	}
	content, _ := rb["content"].(map[string]any)
	entry, ok := content["application/json"].(map[string]any)
	if !ok {
		return nil, nil
	}
	schema, _ := entry["schema"].(map[string]any)
	return schema, nil
}

// ShapeFromSpec flattens op's path parameters, query parameters and
// top-level request-body properties into the single OperationShape both
// audit consumers compare — mirroring cmd/gen_action_inputs's
// assembleOperationFields's flattening exactly (same three sources, same
// "path parameters are always required" rule, same refusal when one wire
// name is contributed by more than one source), because a divergence between
// how this package flattens an operation and how the generator does would
// reintroduce the exact defect this package exists to catch: one fact,
// derived twice.
func ShapeFromSpec(op SpecOperation) (OperationShape, error) {
	var fields []FieldShape
	origins := make(map[string]string)

	for _, param := range op.Parameters {
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		if name == "" {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter with no name (possibly an unresolved components.parameters $ref)", op.OperationID)
		}

		schemaRaw, _ := param["schema"].(map[string]any)
		if schemaRaw == nil {
			schemaRaw = map[string]any{"type": "string"}
		}
		resolved, err := resolveSchema(op.Schemas, schemaRaw, map[string]int{}, 0)
		if err != nil {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter %q: %w", op.OperationID, name, err)
		}

		var origin string
		var required bool
		switch in {
		case "path":
			origin, required = "path", true // OpenAPI mandates path parameters are always required
		case "query":
			origin = "query"
			required, _ = param["required"].(bool)
		default:
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter %q: location %q is not supported (only path and query parameters flatten into a field)", op.OperationID, name, in)
		}

		description := resolved.Description
		if d, ok := param["description"].(string); ok && d != "" {
			description = d // a parameter's own description takes precedence over its schema's
		}

		if existing, dup := origins[name]; dup {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: %q is contributed by both %s and %s", op.OperationID, name, existing, origin)
		}
		origins[name] = origin

		fields = append(fields, FieldShape{
			JSONName:    name,
			Type:        resolved.Type,
			Required:    required,
			Enum:        stringifyEnum(resolved.Enum),
			Description: description,
			Origin:      origin,
		})
	}

	bodySchema, err := requestBodySchemaNode(op)
	if err != nil {
		return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: %w", op.OperationID, err)
	}
	if bodySchema != nil {
		resolved, err := resolveSchema(op.Schemas, bodySchema, map[string]int{}, 0)
		if err != nil {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: %w", op.OperationID, err)
		}

		propNames := make([]string, 0, len(resolved.Properties))
		for name := range resolved.Properties {
			propNames = append(propNames, name)
		}
		sort.Strings(propNames)

		wireNames := make(map[string]string, len(propNames)) // jsonName -> raw property name, for the collision check below
		for _, rawName := range propNames {
			propResolved, err := resolveSchema(op.Schemas, resolved.Properties[rawName], map[string]int{}, 0)
			if err != nil {
				return OperationShape{}, fmt.Errorf("shape from spec for %s: request body property %q: %w", op.OperationID, rawName, err)
			}

			// A body property's own name is rendered through bodyJSONTag, the
			// identical transform cmd/gen_action_inputs used when it generated
			// this operation's Input struct: see naming.go's doc comment for
			// why comparing the raw spec name against ShapeFromCatalog's
			// JSONName would make every body field incomparable.
			jsonName := bodyJSONTag(splitWords(rawName))

			// bodyJSONTag is not injective (see naming.go): two distinct raw
			// property names can render to the same JSON tag. The generator
			// refuses this at generation time (assembleFields's identical
			// check) rather than silently letting one shadow the other, and
			// ShapeFromSpec must refuse it too — a silent shadow here would
			// make this operation's shape depend on map iteration order.
			if existingRaw, dup := wireNames[jsonName]; dup {
				return OperationShape{}, fmt.Errorf("shape from spec for %s: body properties %q and %q both render as JSON field %q", op.OperationID, existingRaw, rawName, jsonName)
			}
			wireNames[jsonName] = rawName

			if existing, dup := origins[jsonName]; dup {
				return OperationShape{}, fmt.Errorf("shape from spec for %s: %q is contributed by both %s and body", op.OperationID, jsonName, existing)
			}
			origins[jsonName] = "body"

			fields = append(fields, FieldShape{
				JSONName:    jsonName,
				Type:        propResolved.Type,
				Required:    resolved.Required[rawName],
				Enum:        stringifyEnum(propResolved.Enum),
				Description: propResolved.Description,
				Origin:      "body",
			})
		}
	}

	sort.Slice(fields, func(i, j int) bool { return fields[i].JSONName < fields[j].JSONName })
	return OperationShape{OperationID: op.OperationID, Method: op.Method, Path: op.Path, Fields: fields}, nil
}

// httpMethods are the OpenAPI verbs that name an operation inside a path
// item, mirroring cmd/audit_1to1 and cmd/gen_action_inputs's identical list:
// a path item can also carry non-verb keys ("parameters", "summary"), which
// are not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// LoadSpecOperation decodes one vendored OpenAPI document and returns the
// named operation, resolved enough for ShapeFromSpec: its identity, its
// parameters, its request body, and the document's own components so
// resolveSchema can follow a $ref or requestBodySchemaNode a requestBody
// indirection.
//
// Reimplemented here, not moved from cmd/gen_action_inputs's near-identical
// loadDocument/operationsByDomain (spec.go), for two independent reasons.
// First, that package is `package main`; Go refuses to import a main package
// from anywhere, so its logic could not be reused without first moving it out
// of cmd/ regardless of preference. Second, most of what it does beyond
// "decode paths and components, find one operation" is generation-only
// concern this package has no use for — domain-tag grouping
// (operationsByDomain, checkDomainTagsCoverSpec), Summary/Description
// carried toward Title/Description derivation, credential and
// minimum-parameter bookkeeping. What this function reimplements is the
// small, genuinely shared fraction: decode a document's paths and
// components, and find one operation by operationId.
func LoadSpecOperation(data []byte, operationID string) (SpecOperation, error) {
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas       map[string]any `json:"schemas"`
			RequestBodies map[string]any `json:"requestBodies"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return SpecOperation{}, fmt.Errorf("decode spec: %w", err)
	}

	// Paths are iterated in sorted order so a document with more than one
	// operation carrying the same operationId (a spec defect, not something
	// this function should paper over silently) resolves deterministically
	// rather than depending on Go's randomised map iteration.
	paths := make([]string, 0, len(doc.Paths))
	for path := range doc.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		methods := doc.Paths[path]
		verbs := make([]string, 0, len(methods))
		for method := range methods {
			verbs = append(verbs, method)
		}
		sort.Strings(verbs)

		for _, method := range verbs {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var op struct {
				OperationID string           `json:"operationId"`
				Parameters  []map[string]any `json:"parameters"`
				RequestBody map[string]any   `json:"requestBody"`
			}
			if err := json.Unmarshal(methods[method], &op); err != nil {
				return SpecOperation{}, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID != operationID {
				continue
			}
			return SpecOperation{
				OperationID:   op.OperationID,
				Method:        strings.ToUpper(method),
				Path:          path,
				Parameters:    op.Parameters,
				RequestBody:   op.RequestBody,
				Schemas:       doc.Components.Schemas,
				RequestBodies: doc.Components.RequestBodies,
			}, nil
		}
	}
	return SpecOperation{}, fmt.Errorf("operation %q not found in spec", operationID)
}
