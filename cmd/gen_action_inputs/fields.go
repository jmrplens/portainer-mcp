package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// structSpec is one Go struct this generator will emit: either the flat
// top-level Input struct for one operation, or a nested struct backing an
// object-typed body property (registries.registryCreatePayload's "Ecr",
// "Github", "Gitlab" and "Quay" are the real examples in the pilot domains).
type structSpec struct {
	Name   string
	Doc    string
	Fields []fieldSpec
}

// Origin values identify which part of an operation contributed a
// top-level field: see fieldSpec.Origin's doc comment for why this exists.
const (
	originPath  = "path"
	originQuery = "query"
	originBody  = "body"
)

// fieldSpec is one struct field.
type fieldSpec struct {
	GoName      string
	GoType      string
	JSONName    string
	Required    bool
	Description string
	// Origin is "path", "query" or "body" for a top-level field returned by
	// assembleOperationFields — set at the exact point that function already
	// decides which of the three it is contributing to the merge, so a
	// generated handler (cmd/gen_action_inputs/handler.go) can route each
	// field to the generated client's positional path arguments, its
	// optional query-params struct or its body struct without re-running
	// that classification itself. Two derivations of which bucket a field
	// belongs to is precisely the "one fact, derived twice" defect that lost
	// 127 operations in P3.0 (see toolutil.DomainTags's doc comment) — this
	// field is what keeps handler.go from repeating that mistake one level
	// down. Left "" for a nested struct's own fields (assembleFields' other
	// caller, typeOf), which are never routed individually: a handler
	// forwards a nested object whole, as one value, to whichever bucket its
	// parent field belongs to.
	Origin string
	// Enum is non-nil when the spec constrains this field to a fixed set of
	// values. google/jsonschema-go v0.4.3's reflector has no struct-tag
	// syntax for "enum" — see internal/toolutil/schema.go's EnumParams for
	// why this is carried separately and applied by an EnumParams() method
	// rather than a jsonschema tag.
	Enum           []any
	EnumScalarType string // "string", "integer", "number" or "boolean"
	// Minimum is non-nil when this field carries a lower-bound constraint —
	// see isIdentifierPathParam's doc comment: this is a constraint this
	// generator adds itself, never one the vendored specification declares.
	// Rendered by renderMinimumParams the same way Enum is rendered by
	// renderEnumParams: as a MinimumParams() method (toolutil.MinimumParams),
	// since google/jsonschema-go's reflector has no struct-tag syntax for
	// "minimum" either.
	Minimum *int
}

// isIdentifierPathParam reports whether name — a path parameter's own OpenAPI
// name, not yet through goFieldName/bodyJSONTag — is shaped like a Portainer
// resource identifier: it matches case-insensitively on the suffix "id"
// ("id", "environmentId", "credentialID", "endpoint_id", "containerId",
// "taskID", "endpointId", "registryId", "endpointID", "serviceId",
// "stackId", "jobID", "keyID", "repositoryID").
//
// This is a general rule about the *name*, and it is only half the decision.
// It deliberately guarantees nothing on its own about any particular
// operation: three names never match it at all, and a further four
// (operationId, parameter) pairs match it but must not carry the lower bound
// anyway. Those live in pathParamMinimumExceptions below — the single place
// any carve-out is recorded — and assembleOperationFields consults both. An
// earlier revision of this comment asserted a positivity guarantee over
// "every one of the 285 integer path parameters ... except three"; that count
// was never audited against the containerId trio the exception table now
// names, so the claim is not restated here. The two tables, not a headline
// number, are what is actually checked by
// TestUnit_IsIdentifierPathParam_AcceptsIdentifiersRejectsCounterexamples and
// the real-operation tests in minimum_test.go.
//
// Three names never match the suffix rule and so need no entry anywhere:
// "repositoryName" (EcrDeleteTags's — Portainer's own known upstream defect,
// a resource *name* declared "integer"; see registries.go's doc comment on
// ecrDeleteTagsInput), and "state" and "rpn", ordinary non-identifier
// integers (a Docker task state code, and Quay's "rpn" toggle) that
// legitimately accept zero.
//
// The lower bound itself is this project's own addition, not the
// specification's: a Portainer resource identifier is a positive integer, a
// zero or negative one is never a valid route for one, and the vendored
// specification simply never says so. Refusing a non-positive identifier once,
// in the published schema (which tools.Execute validates against before any
// handler runs and before any network call), is both earlier and more uniform
// than the same guard clause hand-written into every handler that takes one.
func isIdentifierPathParam(name string) bool {
	return len(name) >= 2 && strings.EqualFold(name[len(name)-2:], "id")
}

// pathParamKey identifies one path parameter of one operation. A carve-out
// must be this specific: every name below also occurs on *other* operations
// where the suffix rule is exactly right, so excluding by name alone would
// silently drop a real constraint from unrelated routes.
type pathParamKey struct {
	OperationID string
	ParamName   string
}

// pathParamMinimumExceptions are the (operationId, path parameter) pairs where
// isIdentifierPathParam's name rule matches but stamping "minimum": 1 would
// make the published JSON Schema refuse a value the real server accepts — or
// assert a numeric range over something that is not a number at all. The value
// is the reason, kept next to the entry so the next reader does not have to
// re-derive the argument, exactly as actionNameOverrides' Reason field does.
//
// Two distinct defects, one mechanism, because they need the identical
// carve-out at the identical decision point:
//
//   - endpointDockerhubStatus's registryId: zero is Portainer's documented
//     sentinel for the anonymous, unauthenticated DockerHub registry, not an
//     absent identifier. Portainer's own handler reads
//     `if registryID == 0 { registry = &portainer.Registry{} }` before any
//     lookup. Confirmed live: GET /endpoints/1/dockerhub/0 succeeds past the
//     lookup, while GET /endpoints/1/dockerhub/1 returns "Unable to find a
//     registry". Zero is the value guaranteed to work, so "minimum": 1 would
//     refuse the one call that always succeeds. registryId keeps the bound
//     everywhere else it appears (endpointRegistryAccess, and the registries
//     domain's own routes), where zero really is invalid.
//
//   - containerId on the three operations below: declared "integer" in the
//     vendored spec, but Portainer reads it as a string and passes it
//     straight through to Docker as a hex container ID. This is the same
//     documented-wrong-type defect "repositoryName" is excluded for, and it
//     was simply never examined when the suffix rule was written. A hex ID is
//     not reliably parseable as an integer at all, so a numeric lower bound
//     asserts a guarantee about a value that is not a number; this generator
//     refuses to mislabel an upstream defect as a constraint.
var pathParamMinimumExceptions = map[pathParamKey]string{
	{OperationID: "EndpointDockerhubStatus", ParamName: "registryId"}:     "registryId 0 is Portainer's sentinel for the anonymous DockerHub registry (GET /endpoints/{id}/dockerhub/{registryId}); a positive lower bound would refuse the one value guaranteed to resolve",
	{OperationID: "DockerContainerGpusInspect", ParamName: "containerId"}: "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/gpus); no numeric bound is meaningful",
	{OperationID: "ContainerImageStatus", ParamName: "containerId"}:       "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/image_status); no numeric bound is meaningful",
	{OperationID: "SnapshotContainerInspect", ParamName: "containerId"}:   "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/snapshot/containers/{containerId}); no numeric bound is meaningful",
}

// pathParamTakesMinimum decides whether this generator stamps a positive lower
// bound on one integer path parameter of one operation: the general name rule,
// minus any per-operation carve-out recorded above.
func pathParamTakesMinimum(operationID, paramName string) bool {
	if !isIdentifierPathParam(paramName) {
		return false
	}
	_, excepted := pathParamMinimumExceptions[pathParamKey{OperationID: operationID, ParamName: paramName}]
	return !excepted
}

// fieldTag renders one field's full struct tag: the "json" tag and, when
// description is non-empty, a "jsonschema" tag carrying it.
//
// required decides omitempty on the "json" tag, and nothing else does:
// google/jsonschema-go's reflector derives a schema's "required" list purely
// from a field's own omitempty/omitzero tag (jsonschema-go@v0.4.3's infer.go:
// `if !info.settings["omitempty"] && !info.settings["omitzero"] { s.Required
// = append(...) }`), so this is the single place a spec's required/optional
// status becomes Go source — get this wrong and the published schema
// disagrees with the spec regardless of the field's Go type or its
// "jsonschema" tag.
//
// The "jsonschema" tag carries description verbatim and nothing else.
// Checked directly against the vendored module rather than assumed: this
// version's reflector (infer.go, same file, a few lines above the
// "required" check just cited) does exactly one thing with this tag —
// `if tag, ok := field.Tag.Lookup("jsonschema"); ok { fs.Description = tag }`
// — the *entire* tag value becomes the field's description, verbatim, with
// no comma-separated keyword syntax of its own. This project's own
// schema_test.go fixture (sampleInput) already demonstrates the trap of
// assuming the sibling gitlab-mcp-server project's convention
// (`jsonschema:"text,required"`) carries over: its ID field's tag ends
// "...the tag,required", and that whole ",required" suffix lands in the
// published description text, not in any required/optional decision — this
// function must never append anything past the description itself, or every
// generated field would carry the same pollution.
//
// Rendered as an ordinary (non-raw) Go string literal via %q whenever the
// tag content itself contains a backtick, rather than the backtick-delimited
// raw string this generator used before descriptions were added to the tag:
// 32 of the vendored Business Edition specification's descriptions contain a
// backtick, which would terminate a raw string literal early and either fail
// to compile or silently truncate the tag. %q escapes exactly what needs
// escaping (quotes, backslashes, newlines) for the common case; falling back
// to it only when a backtick is present keeps the common, backtick-free case
// rendering exactly as before.
func fieldTag(name string, required bool, description string) string {
	content := "json:" + strconv.Quote(jsonNameValue(name, required))
	if description != "" {
		content += " jsonschema:" + strconv.Quote(description)
	}
	if strings.ContainsRune(content, '`') {
		return fmt.Sprintf("%q", content)
	}
	return "`" + content + "`"
}

// jsonNameValue renders the value half of a field's "json" tag: the wire name
// alone when required, or the wire name plus ",omitempty" when not — see
// fieldTag's doc comment for why required is the only thing that decides
// this.
func jsonNameValue(name string, required bool) string {
	if required {
		return name
	}
	return name + ",omitempty"
}

// isEnumScalar reports whether t is a JSON Schema type this generator's enum
// mechanism (a plain Go value compared for equality) can represent. "object"
// and "array" enums do not occur in the vendored specs and are refused
// implicitly by never being applied — see assembleFields, which only reads
// Enum off a scalar-typed node.
func isEnumScalar(t string) bool {
	switch t {
	case "string", "integer", "number", "boolean":
		return true
	}
	return false
}

// typeOf renders node as a Go type expression, appending any nested struct
// definitions it needs (an object with properties, or a map whose value is
// itself an object) to *structs. namePrefix names any such nested struct.
//
// Optional scalars and optional single objects render as a pointer
// (*string, *int, *bool, *float64, *NestedStruct) so that "not provided" and
// "provided as the zero value" stay distinguishable — this project's own
// registries.create/ping/configure handlers already depend on that
// distinction for TLS and TLSSkipVerify (see registries_test.go's
// "ExplicitTLSFalse" tests) and the vendored client generated from the same
// spec (internal/portainer/gen) uses the identical convention: required
// fields are values, optional fields are pointers, with no exception. Arrays
// and maps are the one deliberate departure from that generated client's
// convention (which also pointers optional slices, e.g. `Tags *[]string`):
// a nil slice or nil map is already an unambiguous "not provided" sentinel,
// so wrapping either in a second pointer buys no information and only adds
// an extra indirection a handler must peel off.
func typeOf(node *schemaNode, required bool, namePrefix string, structs *[]structSpec) (string, error) {
	// A node the resolver truncated stands for the cut edge of a genuine $ref
	// cycle (see resolve's doc comment). There is no Go type for it: emitting
	// the empty struct its zero properties would otherwise produce would
	// silently drop every field beyond the cut. No request body in either
	// vendored spec is cyclic today, so this refuses rather than models it.
	if node.TruncatedRef != "" {
		return "", fmt.Errorf(
			"schema is cyclic through %s: a request-shaped Go type cannot represent it, and truncating would silently drop every property past the cycle",
			node.TruncatedRef)
	}
	switch node.Type {
	case "string":
		if required {
			return "string", nil
		}
		return "*string", nil
	case "integer":
		if required {
			return "int", nil
		}
		return "*int", nil
	case "number":
		if required {
			return "float64", nil
		}
		return "*float64", nil
	case "boolean":
		if required {
			return "bool", nil
		}
		return "*bool", nil
	case "array":
		if node.Items == nil {
			return "", fmt.Errorf("array schema has no items")
		}
		itemType, err := typeOf(node.Items, true, namePrefix+"Item", structs)
		if err != nil {
			return "", fmt.Errorf("array item: %w", err)
		}
		return "[]" + itemType, nil
	case "object":
		if len(node.Properties) > 0 {
			fields, err := assembleFields(node.Properties, namePrefix, structs)
			if err != nil {
				return "", err
			}
			doc := namePrefix + " is a nested object property."
			if node.Description != "" {
				doc = namePrefix + ": " + node.Description
			}
			*structs = append(*structs, structSpec{Name: namePrefix, Doc: doc, Fields: fields})
			if required {
				return namePrefix, nil
			}
			return "*" + namePrefix, nil
		}
		if node.MapValue != nil {
			valueType, err := typeOf(node.MapValue, true, namePrefix+"Value", structs)
			if err != nil {
				return "", fmt.Errorf("map value: %w", err)
			}
			return "map[string]" + valueType, nil
		}
		return "", fmt.Errorf("object schema has neither properties nor a typed additionalProperties — a free-form object is not expressible as a Go struct")
	case "":
		return "", fmt.Errorf("schema has no type")
	default:
		return "", fmt.Errorf("unsupported schema type %q", node.Type)
	}
}

// assembleFields builds one struct's fields from a resolved property list,
// used both for a nested object's own properties and, via
// assembleOperationFields, for a request body's top-level properties.
//
// Field names use bodyJSONTag: every body property in the vendored specs is
// named in this Portainer API's own Go-struct-field style ("Name", "BaseURL",
// "TLSSkipVerify"), and bodyJSONTag is what converts that into the
// lower-camel-case this project's tool-facing schemas publish uniformly,
// matching the pilot domains' hand-written Input structs exactly
// (BaseURL -> "baseUrl", TLSSkipVerify -> "tlsSkipVerify").
func assembleFields(props []schemaProperty, structNamePrefix string, structs *[]structSpec) ([]fieldSpec, error) {
	fields := make([]fieldSpec, 0, len(props))
	// goFieldName and bodyJSONTag are not injective (see their doc comments):
	// two distinct wire property names can collapse onto one rendered Go
	// field name or one rendered JSON tag. go/format only reformats, and
	// go/types happily accepts two struct fields with distinct Go names that
	// both carry the same `json:"..."` tag — encoding/json then silently
	// drops both fields that share a tag at (un)marshal time, so the
	// published schema would disagree with what the handler actually parses.
	// Detected here, one struct at a time, so both the top-level Input and
	// every nested object catch a collision the moment it is assembled,
	// rather than only where a caller happens to merge fields from more than
	// one source (assembleOperationFields's own "add" already catches this
	// for the flat merge of path/query/body, but a nested object's own
	// properties never pass through that merge at all).
	jsonNames := make(map[string]string, len(props))
	goNames := make(map[string]string, len(props))
	for _, p := range props {
		words := splitWords(p.Name)
		goName := goFieldName(words)
		jsonName := bodyJSONTag(words)
		if existing, dup := jsonNames[jsonName]; dup {
			return nil, fmt.Errorf(
				"properties %q and %q of %s both render as JSON field %q: goFieldName/bodyJSONTag collision, refusing rather than silently dropping one at (un)marshal time",
				existing, p.Name, structNamePrefix, jsonName)
		}
		jsonNames[jsonName] = p.Name
		if existing, dup := goNames[goName]; dup {
			return nil, fmt.Errorf(
				"properties %q and %q of %s both render as Go field %q: goFieldName/bodyJSONTag collision, refusing rather than silently redeclaring one field",
				existing, p.Name, structNamePrefix, goName)
		}
		goNames[goName] = p.Name

		nestedName := structNamePrefix + goName
		goType, err := typeOf(p.Schema, p.Required, nestedName, structs)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", p.Name, err)
		}
		f := fieldSpec{
			GoName:      goName,
			GoType:      goType,
			JSONName:    jsonName,
			Required:    p.Required,
			Description: p.Schema.Description,
		}
		if len(p.Schema.Enum) > 0 && isEnumScalar(p.Schema.Type) {
			f.Enum = p.Schema.Enum
			f.EnumScalarType = p.Schema.Type
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// bodyDescription returns a request body's own "description" — a sibling of
// "content" and "required" in the OpenAPI document, describing the body as a
// whole rather than any single property of it — falling back to def when rb
// carries none (an anonymous $ref'd body, or one that simply omits it).
func bodyDescription(rb map[string]any, def string) string {
	if d, ok := rb["description"].(string); ok && d != "" {
		return d
	}
	return def
}

// mergeSource identifies which part of the operation a field came from, for
// the collision error assembleOperationFields raises when two sources both
// contribute the same wire name.
type mergeSource struct {
	origin string // "path parameter", "query parameter" or "body"
	field  fieldSpec
}

// assembleOperationFields merges an operation's path parameters, query
// parameters and request body into the single flat field list an Input
// struct needs — the "generate for the model, not for the wire" merge this
// generator exists to do, in place of the wire-shaped convention that keeps
// path/query in one Params struct and the body in a separate
// JSONRequestBody type (see internal/portainer/gen.EndpointListParams for
// that pattern).
//
// It refuses two things beyond what typeOf and the resolver already refuse:
// a path/query parameter whose location is neither ("in": "header" or
// "cookie", which this generator does not support merging into a body-shaped
// struct), and a wire name contributed by more than one of
// {path, query, body} — the collision the brief calls out explicitly, since
// silently letting the later source shadow the earlier one would mean a
// caller's value for one silently governs both.
//
// It returns the operation's flat fields (sorted by
// JSON name, for deterministic struct rendering — unchanged from before this
// doc comment) and, separately, pathOrder: the JSON names of every path
// parameter in the order op.Parameters (and therefore the URL template and
// the generated client method's own positional arguments — see
// TestUnit_PathOrder_MatchesDeclarationOrderNotAlphabeticalOrder) declares
// them. That second return exists only because fields is sorted and path
// argument order is not alphabetical: /kubernetes/{id}/namespaces/{namespace}/configmaps/{configmap}
// calls GetKubernetesConfigMapWithResponse(ctx, id, namespace, configmap),
// not the alphabetical (configmap, id, namespace) — handler.go needs the real
// order to bind each positional argument correctly, and re-scanning
// op.Parameters a second time for it, independent of this function's own
// pass, is exactly the "two derivations of one fact" risk fieldSpec.Origin's
// doc comment already explains; both are produced by this one pass instead.
func assembleOperationFields(op operation, res *resolver, doc *document, structPrefix string, structs *[]structSpec) (fields []fieldSpec, pathOrder []string, err error) {
	byName := map[string]mergeSource{}
	var order []string
	add := func(origin string, f fieldSpec) error {
		if existing, dup := byName[f.JSONName]; dup {
			return fmt.Errorf("%q is contributed by both %s and %s: a tool input needs one flat object, and this generator refuses to guess which one wins",
				f.JSONName, existing.origin, origin)
		}
		byName[f.JSONName] = mergeSource{origin: origin, field: f}
		order = append(order, f.JSONName)
		if origin == "path parameter" {
			pathOrder = append(pathOrder, f.JSONName)
		}
		return nil
	}

	for _, param := range op.Parameters {
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		if name == "" {
			// A components.parameters $ref parameter object carries no "name"
			// of its own — the name lives on the referenced component, which
			// this generator does not resolve. Silently skipping it (as this
			// loop used to) would drop an unmet required path/query parameter
			// with no trace in the generated Input struct. None of the
			// vendored specs use a parameter-level $ref today (verified
			// against every operation in both specs), so this refusal is not
			// expected to fire; it exists so a future spec that does use one
			// fails generation loudly instead of publishing an incomplete
			// struct.
			return nil, nil, fmt.Errorf("parameter with no name (possibly a components.parameters $ref, which this generator does not resolve): %v", param)
		}
		schemaRaw, _ := param["schema"].(map[string]any)
		if schemaRaw == nil {
			schemaRaw = map[string]any{"type": "string"}
		}
		node, err := res.resolve(schemaRaw, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("parameter %q: %w", name, err)
		}
		if d, ok := param["description"].(string); ok {
			node.Description = d
		}

		var origin string
		var fieldOrigin string
		var required bool
		switch in {
		case "path":
			origin, fieldOrigin, required = "path parameter", originPath, true // OpenAPI mandates path parameters are always required
		case "query":
			origin, fieldOrigin = "query parameter", originQuery
			required, _ = param["required"].(bool)
		default:
			return nil, nil, fmt.Errorf("parameter %q: location %q is not supported; only path and query parameters merge into a flat Input", name, in)
		}

		goType, err := typeOf(node, required, structPrefix+goFieldName(splitWords(name)), structs)
		if err != nil {
			return nil, nil, fmt.Errorf("%s %q: %w", origin, name, err)
		}
		f := fieldSpec{
			GoName:      goFieldName(splitWords(name)),
			GoType:      goType,
			JSONName:    name, // path/query names are already the lower-camel-case OpenAPI declares; used verbatim
			Required:    required,
			Description: node.Description,
			Origin:      fieldOrigin,
		}
		if len(node.Enum) > 0 && isEnumScalar(node.Type) {
			f.Enum = node.Enum
			f.EnumScalarType = node.Type
		}
		if in == "path" && node.Type == "integer" && pathParamTakesMinimum(op.OperationID, name) {
			one := 1
			f.Minimum = &one
		}
		if err := add(origin, f); err != nil {
			return nil, nil, err
		}
	}

	bodySchema, err := doc.requestBodySchema(op.RequestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("request body: %w", err)
	}
	if bodySchema != nil {
		node, err := res.resolve(bodySchema, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("request body: %w", err)
		}
		if node.Type != "" && node.Type != "object" {
			return nil, nil, fmt.Errorf("request body: top-level type %q is not an object; only an object body flattens into named fields", node.Type)
		}

		switch {
		case len(node.Properties) > 0:
			// requestBody.required (bodyRequired) governs only whether the body
			// may be absent altogether; it does not loosen what the body
			// schema's own "required" list says about its properties, so each
			// field's Required flag (already set by assembleFields from that
			// list) is carried through unchanged.
			bodyFields, err := assembleFields(node.Properties, structPrefix, structs)
			if err != nil {
				return nil, nil, fmt.Errorf("request body: %w", err)
			}
			for _, f := range bodyFields {
				f.Origin = originBody
				if err := add("body", f); err != nil {
					return nil, nil, err
				}
			}
		case node.MapValue != nil:
			// The body itself is a map — the seven Kubernetes bulk-delete
			// operations (DeleteCronJobs, DeleteJobs, DeleteKubernetesIngresses,
			// DeleteKubernetesServices, DeleteRoleBindings, DeleteRoles,
			// DeleteServiceAccounts) are the real example, each documented as "a
			// map where the key is the namespace and the value is an array of
			// <kind> to delete" — not an object with named properties to
			// flatten. There is no property name to derive a field name from,
			// because the map *is* the whole body, so this names the field
			// "namespace" after what every instance of this shape in the
			// vendored spec actually holds — the same identifier
			// toolutil.scopeParameterDefaults already documents guidance for.
			valueType, err := typeOf(node.MapValue, true, structPrefix+"Value", structs)
			if err != nil {
				return nil, nil, fmt.Errorf("request body: map value: %w", err)
			}
			bodyRequired, _ := op.RequestBody["required"].(bool)
			f := fieldSpec{
				GoName:      "Namespace",
				GoType:      "map[string]" + valueType,
				JSONName:    "namespace",
				Required:    bodyRequired,
				Description: bodyDescription(op.RequestBody, "Values keyed by Kubernetes namespace."),
				Origin:      originBody,
			}
			if err := add("body", f); err != nil {
				return nil, nil, err
			}
		default:
			// Neither named properties nor a typed additionalProperties: route
			// the body node through typeOf so its free-form-object refusal —
			// the whole reason this generator exists rather than publishing a
			// vacuous struct for every operation — can actually fire for a
			// request body. Before this fix, a required body that resolved to
			// this shape (PolicyCreate, PolicyConflicts) silently contributed
			// zero fields instead of aborting generation: the refusal lived
			// inside typeOf, but nothing on this path ever called it, so an
			// operation whose whole point was "we cannot express this safely"
			// published the same empty-object schema as a genuinely
			// parameterless action.
			if _, err := typeOf(node, true, structPrefix, structs); err != nil {
				return nil, nil, fmt.Errorf("request body: %w", err)
			}
			return nil, nil, fmt.Errorf("request body: resolved to no fields even though a body is present; this generator has a gap for this schema shape")
		}
	}

	sort.Strings(order)
	fields = make([]fieldSpec, 0, len(order))
	for _, name := range order {
		fields = append(fields, byName[name].field)
	}
	return fields, pathOrder, nil
}
