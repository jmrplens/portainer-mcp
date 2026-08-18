package specdiff

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
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
	//
	// Title and Description are taken from spec directly, with no further
	// cleaning: they are already the catalog's final, model-facing text,
	// whether cmd/gen_action_inputs derived them mechanically from the spec
	// or a domain's narrative hook (toolutil.WithNarrative) replaced them
	// outright. Either way this is what a model is actually shown — the
	// same reason ShapeFromCatalog reads a reflected InputSchema instead of
	// re-deriving one.
	return OperationShape{
		OperationID: spec.OperationID, Title: spec.Title, Description: spec.Description, Fields: fields,
		TitleOverridden: spec.TitleOverridden, DescriptionOverridden: spec.DescriptionOverridden,
	}, nil
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
	// HasMapValue reports whether this node is an object schema with a typed
	// "additionalProperties" and no named "properties" — a map/dictionary
	// body, mirroring cmd/gen_action_inputs's identical schemaNode.MapValue
	// (fields.go). Only ever set when Properties is empty at the point this
	// is checked, the identical condition that function's resolver applies:
	// an object schema can only be flattened one way, and named properties
	// win when a schema author declares both (never observed in either
	// vendored specification, but not assumed impossible). This carries only
	// the fact that a map value exists, not the value schema itself: unlike
	// cmd/gen_action_inputs, ShapeFromSpec never needs to name the value's Go
	// type, because the field it synthesizes for a map body always reports
	// JSON Schema type "object" regardless of what the map's values are (see
	// ShapeFromSpec's own doc comment on synthesizeMapBodyField).
	HasMapValue bool
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

	// A typed "additionalProperties" only matters when nothing has already
	// contributed a named property — mirroring cmd/gen_action_inputs's
	// identical resolver.resolve, which checks this after (not instead of)
	// the "properties" handling above for the same reason: a schema
	// declaring both, however unlikely, flattens as named properties, not as
	// a map, because that is the shape a Go struct can actually express.
	if len(node.Properties) == 0 {
		if _, ok := raw["additionalProperties"].(map[string]any); ok {
			node.HasMapValue = true
			if node.Type == "" {
				node.Type = "object"
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
	if src.HasMapValue {
		dst.HasMapValue = true
	}
}

// resolvedRequestBody returns op's own "requestBody" node, resolving one
// components.requestBodies $ref first if the body itself is declared that
// way — the same indirection cmd/gen_action_inputs's requestBodySchema
// resolves. Returns nil, nil for an operation with no body at all. Both
// requestBodySchemaNode (which reads only its "content") and
// synthesizeMapBodyField (which reads only its own "required" and
// "description", never its content) resolve the identical indirection
// through this one function, rather than each following the $ref itself.
func resolvedRequestBody(op SpecOperation) (map[string]any, error) {
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
	return rb, nil
}

// requestBodySchemaNode returns op's request body's single schema node,
// mirroring cmd/gen_action_inputs's own requestBodySchema (spec.go) exactly:
// whichever one content type key "content" declares, not only
// "application/json". A multipart/form-data body — every wave-1 file-upload
// operation (EndpointCreate, CustomTemplateCreateFile, both
// StackCreateDockerSwarm/StandaloneFile, EndpointDockerBrowsePut, and eight
// more named in cmd/gen_action_inputs/fieldcount_crosscheck_test.go's own
// former knownFieldCountResidual entries) declares real top-level fields —
// the identical object schema an application/json body would declare, wrapped
// in multipart form fields rather than a JSON object body, "format":"binary"
// marking the one property that is a file upload rather than an ordinary
// string. Reading only "application/json" made every one of those fields
// invisible to this function: a multipart-only body resolved to nil here,
// indistinguishable from "no body at all", so once such an operation was
// declared by a catalog action, every one of its real fields rendered as a
// gating ChangeAdded. This function must read the identical content this
// generator reads, or the two silently diverge on this third dimension (content
// type) the same way C1 (non-object bodies) and C2 (map-shaped bodies)
// already diverged on two others.
//
// Returns nil, nil for an operation with no body at all, or one whose body's
// "content" is empty. Returns an error, mirroring requestBodySchema's
// identical refusal, when the body declares more than one content type —
// CustomTemplateCreate (application/json and multipart/form-data, the EE
// vendored spec only) is the one operation in either vendored specification
// that does: there is no single schema to prefer over the other without
// arbitrarily ignoring whichever one this function did not pick, so both
// sides now refuse identically rather than this function silently reading
// only the application/json variant the way it used to.
func requestBodySchemaNode(op SpecOperation) (map[string]any, error) {
	rb, err := resolvedRequestBody(op)
	if err != nil {
		return nil, err
	}
	if rb == nil {
		return nil, nil
	}
	content, _ := rb["content"].(map[string]any)
	if len(content) == 0 {
		return nil, nil
	}
	if len(content) > 1 {
		kinds := make([]string, 0, len(content))
		for k := range content {
			kinds = append(kinds, k)
		}
		sort.Strings(kinds)
		return nil, fmt.Errorf("requestBody declares %d content types (%s); no single schema to compare against a catalog action's flat shape", len(content), strings.Join(kinds, ", "))
	}
	for _, v := range content {
		entry, _ := v.(map[string]any)
		schema, _ := entry["schema"].(map[string]any)
		return schema, nil
	}
	return nil, nil // unreachable: len(content) == 1 guarantees the loop above returns
}

// mapBodyDefaultDescription is the fallback description ShapeFromSpec gives
// a map-shaped body whose requestBody itself carries none, verbatim from
// cmd/gen_action_inputs/fields.go's identical string — see
// synthesizeMapBodyField.
const mapBodyDefaultDescription = "Values keyed by Kubernetes namespace."

// synthesizeMapBodyField returns the single field ShapeFromSpec reports for
// a map-shaped request body (JSON Schema "type":"object" with a typed
// "additionalProperties" and no named "properties"), mirroring
// cmd/gen_action_inputs/fields.go's assembleOperationFields's identical
// synthesis for the seven Kubernetes bulk-delete operations exactly: JSON
// name "namespace" (a map body has no property name of its own to derive one
// from — every real instance in the vendored specification is "a map where
// the key is the namespace and the value is an array of <kind> to delete"),
// required exactly when the request body itself is required (not a
// property-level "required" list — a map body has no named properties to
// list one for), and described by the request body's own "description" when
// non-empty, falling back to mapBodyDefaultDescription otherwise
// (bodyDescription's identical fallback rule).
//
// Type is always "object", never a Go type string naming the map's value
// shape (cmd/gen_action_inputs's typeOf derives "map[string][]string" or
// similar, because it emits a Go struct field and must): a map's own JSON
// Schema type is "object" regardless of what its values are — verified
// directly, reflecting the exact generated field
// (`Namespace map[string][]string`) through google/jsonschema-go renders
// "namespace"'s own "type" as "object", carrying the value shape only in
// "additionalProperties", which OperationShape's flat Fields do not describe
// at all (see that type's own doc comment on why a nested shape is out of
// scope). Reporting anything other than "object" here would make this field
// permanently, spuriously incomparable against ShapeFromCatalog's identical
// "object" for the same reason a nullable-array type spelling would (see
// canonicalType's own doc comment).
func synthesizeMapBodyField(op SpecOperation) FieldShape {
	rb, _ := resolvedRequestBody(op) // already resolved once by requestBodySchemaNode above; a $ref error there already returned
	required, _ := rb["required"].(bool)
	description := mapBodyDefaultDescription
	if d, ok := rb["description"].(string); ok && d != "" {
		description = d
	}
	return FieldShape{
		JSONName:    mapBodyFieldName,
		Type:        "object",
		Required:    required,
		Description: description,
		Origin:      "body",
	}
}

// mapBodyFieldName is the single wire name a map-shaped request body
// contributes (see synthesizeMapBodyField for why "namespace"). Named as a
// constant rather than written twice because occupiedBodyNames must report
// the identical name to internal/specnaming: a body that occupies a name the
// disambiguation rule does not know about is exactly the silent shadow the
// rule exists to prevent.
const mapBodyFieldName = "namespace"

// bodyWireName pairs one top-level request-body property's raw
// specification name with the wire name it renders to.
type bodyWireName struct {
	raw  string
	json string
}

// bodyProperties renders every top-level property of a resolved request body
// to its wire name, in raw-name order (sorted, so the fields this function
// feeds are produced deterministically regardless of Go map iteration order).
//
// Rendered once, here, and consumed both by ShapeFromSpec's own property
// loop and by occupiedBodyNames below: computing a property's wire name in
// one place for the collision rule and again in another for the field it
// produces is the "one fact, derived twice" defect this whole package exists
// to catch, and it would be particularly perverse to commit it inside the
// function that catches it.
func bodyProperties(resolved resolvedNode) []bodyWireName {
	rawNames := make([]string, 0, len(resolved.Properties))
	for name := range resolved.Properties {
		rawNames = append(rawNames, name)
	}
	sort.Strings(rawNames)

	out := make([]bodyWireName, 0, len(rawNames))
	for _, raw := range rawNames {
		// A body property's own name is rendered through bodyJSONTag, the
		// identical transform cmd/gen_action_inputs used when it generated
		// this operation's Input struct: see naming.go's doc comment for
		// why comparing the raw spec name against ShapeFromCatalog's
		// JSONName would make every body field incomparable.
		out = append(out, bodyWireName{raw: raw, json: bodyJSONTag(splitWords(raw))})
	}
	return out
}

// occupiedBodyNames is every wire name this operation's request body takes,
// which is what internal/specnaming needs to decide whether a parameter's
// own name collides with one. A map-shaped body occupies one name too
// (mapBodyFieldName), even though it has no named properties at all.
func occupiedBodyNames(bodySchema map[string]any, resolved resolvedNode, props []bodyWireName) []string {
	if bodySchema == nil {
		return nil
	}
	names := make([]string, 0, len(props)+1)
	for _, p := range props {
		names = append(names, p.json)
	}
	if len(resolved.Properties) == 0 && resolved.HasMapValue {
		names = append(names, mapBodyFieldName)
	}
	return names
}

// specParameters projects op's raw parameter list onto the pairs
// internal/specnaming's rule reads: the wire name each contributes and the
// location it comes from. A parameter whose location this function cannot
// support is still projected verbatim rather than filtered out — the
// parameter loop below is what refuses it, with the message it has always
// used, and pre-filtering here would mean the rule silently disambiguated
// against an incomplete set of contributors.
func specParameters(op SpecOperation) []specnaming.Parameter {
	out := make([]specnaming.Parameter, 0, len(op.Parameters))
	for _, param := range op.Parameters {
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		out = append(out, specnaming.Parameter{Name: name, Origin: in})
	}
	return out
}

// ShapeFromSpec flattens op's path parameters, query parameters and
// top-level request-body properties into the single OperationShape both
// audit consumers compare — mirroring cmd/gen_action_inputs's
// assembleOperationFields's flattening exactly (same three sources, same
// "path parameters are always required" rule, same disambiguation of a wire
// name two sources both contribute — internal/specnaming, imported by both —
// and the same refusal for the collisions that rule deliberately does not
// resolve), because a divergence between how this package flattens an
// operation and how the generator does would reintroduce the exact defect
// this package exists to catch: one fact, derived twice.
func ShapeFromSpec(op SpecOperation) (OperationShape, error) {
	var fields []FieldShape
	origins := make(map[string]string)

	// The request body is resolved before the parameter loop rather than
	// after it, because internal/specnaming's disambiguation rule cannot
	// decide whether a parameter's own name collides with the body until it
	// knows every name the body contributes. Only the resolution moved: each
	// refusal the body block raises still fires exactly where it always did,
	// below, and the parameter loop still reports its own refusals first for
	// every operation whose body this document can resolve at all.
	bodySchema, err := requestBodySchemaNode(op)
	if err != nil {
		return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: %w", op.OperationID, err)
	}
	var bodyResolved resolvedNode
	if bodySchema != nil {
		bodyResolved, err = resolveSchema(op.Schemas, bodySchema, map[string]int{}, 0)
		if err != nil {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: %w", op.OperationID, err)
		}
	}
	bodyProps := bodyProperties(bodyResolved)

	// One wire name contributed by both a parameter and a body property is
	// disambiguated by the single rule cmd/gen_action_inputs applies too
	// (internal/specnaming): the body keeps the plain name, the parameter
	// carries its origin. Both sides must produce the identical name or the
	// drift audit reports one field added and another removed on an
	// operation nobody touched — see
	// cmd/gen_action_inputs's TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation,
	// which runs both derivations over every operation in both vendored
	// specifications for exactly that reason.
	paramNames, err := specnaming.ResolveParameters(specParameters(op), occupiedBodyNames(bodySchema, bodyResolved, bodyProps))
	if err != nil {
		return OperationShape{}, fmt.Errorf("shape from spec for %s: %w", op.OperationID, err)
	}

	for i, param := range op.Parameters {
		rawName, _ := param["name"].(string)
		in, _ := param["in"].(string)
		if rawName == "" {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter with no name (possibly an unresolved components.parameters $ref)", op.OperationID)
		}
		// name is the wire name this parameter actually publishes: its own,
		// unless the request body already took it (see paramNames above).
		// Every refusal below still names rawName, the identifier the
		// specification itself declares and the only one a reader can grep
		// the document for.
		name := paramNames[i]

		schemaRaw, _ := param["schema"].(map[string]any)
		if schemaRaw == nil {
			schemaRaw = map[string]any{"type": "string"}
		}
		resolved, err := resolveSchema(op.Schemas, schemaRaw, map[string]int{}, 0)
		if err != nil {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter %q: %w", op.OperationID, rawName, err)
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
			return OperationShape{}, fmt.Errorf("shape from spec for %s: parameter %q: location %q is not supported (only path and query parameters flatten into a field)", op.OperationID, rawName, in)
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

	if bodySchema != nil {
		resolved := bodyResolved

		// Mirrors cmd/gen_action_inputs/fields.go's identical refusal exactly
		// (same message, same condition): a non-object top-level body has no
		// named top-level fields for this function to read, and reading
		// only resolved.Properties for one silently produced zero fields
		// with no error at all — indistinguishable from a real operation
		// that genuinely has none. That is precisely the shape the
		// generator refuses rather than guesses (fields.go's
		// requestBodySchema caller), which after the freeze means an
		// operation like this renders as hand-written code with no
		// generator refusal ever standing over it again; this function's
		// refusal is the only net left. Verified against the vendored
		// specification: DeleteKubernetesNamespace's body is
		// `{"type":"array","items":{"type":"string"}}`, a required list of
		// namespace names this function must not silently report as "no
		// drift" against a catalog shape that also happens to have none.
		//
		// A map-shaped body (JSON Schema "type":"object" with a typed
		// "additionalProperties" and no named "properties") is deliberately
		// NOT refused here, even though resolved.Properties is empty for it
		// too: the generator does not refuse this shape either — it
		// synthesizes one field named "namespace" for it
		// (fields.go's assembleOperationFields, the seven Kubernetes
		// bulk-delete operations' real shape) — so refusing it here would be
		// wrong, not merely incomplete, the moment any of those seven is
		// scaffolded: DeleteCronJobs, DeleteJobs, DeleteKubernetesIngresses,
		// DeleteKubernetesServices, DeleteRoleBindings, DeleteRoles and
		// DeleteServiceAccounts would each report a spurious "namespace
		// added" drift finding against a catalog shape that, correctly,
		// also has it — the exact false positive this function exists to
		// prevent, arriving with the single largest wave this project has
		// scaffolded to date. See synthesizeMapBodyField below for the
		// matching synthesis.
		if resolved.Type != "" && resolved.Type != "object" {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: top-level type %q is not an object; only an object body flattens into named fields", op.OperationID, resolved.Type)
		}
		// A genuinely free-form object — "type":"object" with neither named
		// "properties" nor a typed "additionalProperties" (PolicyCreate and
		// PolicyConflicts in the vendored Business Edition specification are
		// the real cases) — is refused for the identical reason
		// cmd/gen_action_inputs/fields.go's typeOf refuses it ("a free-form
		// object is not expressible as a Go struct"): the generator can
		// never scaffold this shape, so it can never disagree with this
		// function's own field count for it by producing a real field this
		// function missed — the map-body and non-object cases above are
		// dangerous precisely because the generator *can* express them and
		// this function used to silently report a different count. Refusing
		// here instead of reporting zero fields keeps that same guarantee
		// for the one remaining shape neither side can flatten: both sides
		// now refuse, which specdiff's own field-count cross-check
		// (cmd/gen_action_inputs's TestUnit_FieldCounts_GeneratorAndSpecdiffAgree_AcrossBothVendoredSpecs)
		// counts as agreement, not disagreement.
		if resolved.Type == "object" && len(resolved.Properties) == 0 && !resolved.HasMapValue {
			return OperationShape{}, fmt.Errorf("shape from spec for %s: request body: object schema has neither named properties nor a typed additionalProperties; a free-form object has no top-level field to flatten", op.OperationID)
		}
		if len(resolved.Properties) == 0 && resolved.HasMapValue {
			mapField := synthesizeMapBodyField(op)
			// occupiedBodyNames already reported this name to
			// internal/specnaming, so a parameter that wanted it was
			// qualified before the loop above ran and this cannot fire for a
			// parameter/body collision any more. Kept as the net for the one
			// thing the rule does not police: a body that somehow occupies
			// this name twice.
			if existing, dup := origins[mapField.JSONName]; dup {
				return OperationShape{}, fmt.Errorf("shape from spec for %s: %q is contributed by both %s and body", op.OperationID, mapField.JSONName, existing)
			}
			origins[mapField.JSONName] = "body"
			fields = append(fields, mapField)
			// resolved.Properties is empty, so the property-flattening loop
			// below contributes nothing further for this operation; falling
			// through to it (rather than returning early) means this
			// function still sorts and returns through the identical single
			// path every other operation does, not a second, parallel one.
		}

		wireNames := make(map[string]string, len(bodyProps)) // jsonName -> raw property name, for the collision check below
		for _, prop := range bodyProps {
			rawName, jsonName := prop.raw, prop.json
			propResolved, err := resolveSchema(op.Schemas, resolved.Properties[rawName], map[string]int{}, 0)
			if err != nil {
				return OperationShape{}, fmt.Errorf("shape from spec for %s: request body property %q: %w", op.OperationID, rawName, err)
			}

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

			// Cross-origin collisions are disambiguated before the parameter
			// loop runs (internal/specnaming: the body keeps the plain name,
			// the parameter carries its origin), so reaching this means a
			// name the body contributes was taken by a parameter the rule
			// did not qualify — which it only declines to do when it cannot,
			// and it refuses outright then. Kept as a net rather than
			// deleted: it costs one map lookup, and the alternative to a net
			// that never fires is a silent shadow if one ever could.
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
	title, description := CleanTitleAndDescription(op.Summary, op.Description)
	return OperationShape{
		OperationID: op.OperationID, Method: op.Method, Path: op.Path,
		Title: title, Description: description, Fields: fields,
	}, nil
}

// accessPolicyPrefix is the boilerplate line every operation's own
// "description" in the vendored specification ends with — for all but two
// of the 442 operations across both specs, its very last line — of the form
// "**Access policy**: <who may call this>". Verbatim copy of
// cmd/gen_action_inputs/actionspec.go's identically-named constant; see
// CleanTitleAndDescription's doc comment for why this is a
// reimplementation, not an import.
const accessPolicyPrefix = "**access policy**"

// CleanTitleAndDescription derives an operation-level Title and
// Description from its raw "summary" and "description" exactly the way
// cmd/gen_action_inputs's cleanTitleAndDescription does: Title is the
// summary, trimmed; Description is the description with every
// "**Access policy**: ..." line stripped (see accessPolicyPrefix) and the
// remainder trimmed, falling back to Title when that leaves nothing.
//
// Reimplemented here, not imported, for the identical reason naming.go's
// bodyJSONTag and shape.go's own LoadSpecOperation are reimplemented rather
// than moved: cmd/gen_action_inputs is `package main`, and Go refuses to
// import a main package from anywhere. Keeping the two in step is not
// assumed — cmd/gen_action_inputs/actionspec_test.go and this package's own
// TestUnit_CleanOperationTitleAndDescription_MatchesTheGenerator both run
// the exact same real-spec fixture (SharedGitUpdate's summary/description,
// where the description restates the summary with only a capitalisation
// difference) through each implementation and assert on the identical
// result, so a future edit to either side that silently diverges from the
// other fails a test immediately rather than waiting for a report to
// disagree with itself.
//
// Unlike the generator's cleanTitleAndDescription, this function never
// refuses: the generator cannot catalogue an operation with no derivable
// name, but ShapeFromSpec's job is only to compare, and an operation
// genuinely missing a summary (never observed in either vendored
// specification, but not something this package should crash over) simply
// compares Title "" against whatever the other side has — a real,
// reportable difference, not a fatal one.
func CleanTitleAndDescription(summary, description string) (title, cleanedDescription string) {
	title = strings.TrimSpace(summary)
	cleanedDescription = stripAccessPolicyLines(description)
	if cleanedDescription == "" {
		cleanedDescription = title
	}
	return title, cleanedDescription
}

// stripAccessPolicyLines removes every line of raw starting with
// accessPolicyPrefix (case-insensitively, matching leading/trailing
// whitespace on the line) and trims the remainder. Verbatim copy of
// cmd/gen_action_inputs/actionspec.go's identically-named function; see
// CleanTitleAndDescription's doc comment for why reimplemented
// rather than imported.
func stripAccessPolicyLines(raw string) string {
	lines := strings.Split(raw, "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(line)), accessPolicyPrefix) {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
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
// (operationsByDomain, checkDomainTagsCoverSpec), credential and
// minimum-parameter bookkeeping. What this function reimplements is the
// small, genuinely shared fraction: decode a document's paths and
// components, find one operation by operationId, and (like the generator)
// carry its raw Summary/Description forward — here so ShapeFromSpec can
// clean them into Title/Description via CleanTitleAndDescription.
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
				Summary     string           `json:"summary"`
				Description string           `json:"description"`
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
				Summary:       op.Summary,
				Description:   op.Description,
				Parameters:    op.Parameters,
				RequestBody:   op.RequestBody,
				Schemas:       doc.Components.Schemas,
				RequestBodies: doc.Components.RequestBodies,
			}, nil
		}
	}
	return SpecOperation{}, fmt.Errorf("operation %q not found in spec", operationID)
}
