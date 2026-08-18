package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/specnaming"
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
	originPath  = specnaming.OriginPath
	originQuery = specnaming.OriginQuery
	originBody  = specnaming.OriginBody
)

// mapBodyFieldName is the one wire name a map-shaped request body
// contributes — see assembleOperationFields' own map-body branch for why
// "namespace". Named as a constant rather than written twice because
// bodyWireNames must report the identical name to internal/specnaming: a
// name the body occupies but the disambiguation rule does not know about is
// exactly the silent shadow that rule exists to prevent.
const mapBodyFieldName = "namespace"

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
	// WireName is the name the specification itself declares for a top-level
	// path or query parameter, which is also the name the generated client's
	// own query-parameters struct tags it with. Equal to JSONName for every
	// operation but the handful where a request-body property already took
	// that name and internal/specnaming qualified this parameter by its
	// origin instead ("endpointId" -> "endpointIdQuery" for StackMigrate).
	// Left "" for a body field and for a nested struct's own fields, whose
	// wire name is bodyJSONTag's rendering of the specification's name and
	// so is never expected to be equal to it.
	//
	// Carried so buildHandlerSpec can see that a query parameter was
	// renamed. That matters because the generated handler distributes query
	// parameters by unmarshalling the caller's raw input straight into
	// apigen.<Operation>Params, which matches on this name and not on
	// JSONName — see handler.go's refusal, and its package doc for why the
	// whole distribution is a JSON round trip rather than field-by-field
	// assignment.
	WireName string
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
	// RequiresEdition is non-empty (always edition.EE, the only value that
	// means anything — see toolutil.FieldEditions) when this field is present
	// in the Business Edition operation's resolved schema and absent from the
	// Community one: a shared operation whose *shape* still is not shared.
	// Set by ceEEFieldDiff/applyFieldEditionGate in main.go's per-operation
	// loop, never by assembleFields/assembleOperationFields themselves, which
	// have no notion of a second, Community-side resolution at all. Rendered
	// as an `edition:"EE"` struct tag by fieldTag below, read back by
	// toolutil.FieldEditions.
	//
	// applyFieldEditionGate itself is nested-inclusive — it stamps this tag
	// onto a nested structSpec's own Fields exactly the way it stamps the
	// top-level Input struct's, since both are plain []fieldSpec — but
	// pruning is not: toolutil.FieldEditions, which actioncatalog.Build reads
	// to decide what to prune (see that package's Catalog.InputSchema doc
	// comment), inspects only a struct's own top-level fields. A nested
	// struct reached through a field that is *itself* untagged is never
	// recursed into, so a tag landing on one of its fields is inert unless
	// some ancestor field on the path back to the top-level Input struct also
	// carries the tag (in which case the whole subtree, tag included, is
	// already dropped as one property — see pruneInputSchemaByEdition). Measured
	// directly against a full scratch regeneration of both vendored specs at
	// the time this note was written: seven real operations hit this —
	// GitOpsSourcesTest, GitOpsSourcesTestById, CreateKubernetesNamespace,
	// UpdateKubernetesNamespace, UpdateKubernetesNamespaceDeprecated (domain
	// kubernetes), LDAPCheck (domain ldap) and UserUpdate (domain users) —
	// none of them in wave 1. See docs/api-divergences.md for the fuller
	// account and the precise field lists; a wave scaffolding one of those
	// seven must either implement nested pruning or hand-verify each nested
	// tag it emits is subsumed by an already-gated ancestor.
	RequiresEdition edition.Edition
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
//   - containerId on three operations, and serviceId on a fourth, below:
//     declared "integer" in the vendored spec, but Portainer reads each as a
//     string and passes it straight through to Docker (a hex container ID)
//     or Docker Swarm (an alphanumeric service ID). This is the same
//     documented-wrong-type defect "repositoryName" is excluded for, and it
//     was simply never examined when the suffix rule was written. Neither
//     shape is reliably parseable as an integer at all, so a numeric lower
//     bound asserts a guarantee about a value that is not a number; this
//     generator refuses to mislabel an upstream defect as a constraint. All
//     four also carry the identical entry in pathParamTypeOverrides above,
//     which is what changes their rendered Go/JSON Schema type from "int" to
//     "string" in the first place — a field whose published type is no
//     longer a number cannot meaningfully publish a numeric "minimum" either,
//     which is the only reason serviceId is named here at all: unlike the
//     containerId trio, nothing about serviceId's own zero-vs-positive range
//     is in question, only that "minimum" on a string is a constraint
//     encoding/jsonschema silently ignores (see pathParamTypeOverrides'
//     own doc comment for the fuller reasoning and the cheat this guards
//     against).
var pathParamMinimumExceptions = map[pathParamKey]string{
	{OperationID: "EndpointDockerhubStatus", ParamName: "registryId"}:     "registryId 0 is Portainer's sentinel for the anonymous DockerHub registry (GET /endpoints/{id}/dockerhub/{registryId}); a positive lower bound would refuse the one value guaranteed to resolve",
	{OperationID: "DockerContainerGpusInspect", ParamName: "containerId"}: "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/gpus); no numeric bound is meaningful",
	{OperationID: "ContainerImageStatus", ParamName: "containerId"}:       "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/image_status); no numeric bound is meaningful",
	{OperationID: "SnapshotContainerInspect", ParamName: "containerId"}:   "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/snapshot/containers/{containerId}); no numeric bound is meaningful",
	{OperationID: "ServiceImageStatus", ParamName: "serviceId"}:           "serviceId now publishes \"string\" (pathParamTypeOverrides); a numeric \"minimum\" on a string field is a constraint JSON Schema ignores, so the vestige is dropped rather than left to outlive its reason",
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

// pathParamTypeOverrides are the (operationId, path parameter) pairs where the
// vendored specification's declared "integer" type is simply wrong about what
// Portainer accepts on the wire: three of these are the identical
// containerId trio pathParamMinimumExceptions above already carves out of the
// minimum bound (declared "integer" upstream, but Portainer reads it as a
// string and passes it straight through to Docker as a 64-character hex
// container ID — no integer can represent one), and the fourth,
// ServiceImageStatus's serviceId, is Docker Swarm's own service ID: an
// alphanumeric token such as "9mnpnzenvg8p8tdbtq4wvbkcz", never a number
// either. Left at the generator's ordinary "integer" -> "int" rendering, all
// four actions are uncallable: tools.Execute's schema validation (via
// toolutil.ActionSpec.ValidateInput) rejects the only values that could ever
// work, since neither a hex container ID nor a Swarm service ID round-trips
// through Go's int at all, let alone the specific one Docker assigned.
//
// This is not the same decision pathParamMinimumExceptions records. That
// table suppresses a constraint this generator adds on top of the spec's own
// type (a positive lower bound); this one overrides the spec's own type
// itself, because the spec's declared type cannot be called against the real
// server no matter what bound is or is not stamped on it. A field named here
// keeps node.Type == "integer" for every other purpose (isIdentifierPathParam,
// the minimum-exception cross-check test, and the doc comment three lines
// above every entry below) — only the rendered Go field type and therefore
// the published JSON Schema "type" change to "string". See docs/api-
// divergences.md for the fuller account, including why a test that only
// proves a realistic string identifier validates is not enough: Docker
// chooses containerId, so nothing this project controls could fake one, but
// serviceId's own probe container can be labelled with any string a test
// author picks — including the digit "1", which would also satisfy a schema
// that had wrongly stayed "integer". Only asserting that an *integer* is
// refused proves the type actually changed.
var pathParamTypeOverrides = map[pathParamKey]string{
	{OperationID: "DockerContainerGpusInspect", ParamName: "containerId"}: "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/gpus); Docker assigns a 64-character hex string no int can carry",
	{OperationID: "ContainerImageStatus", ParamName: "containerId"}:       "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/containers/{containerId}/image_status); Docker assigns a 64-character hex string no int can carry",
	{OperationID: "SnapshotContainerInspect", ParamName: "containerId"}:   "containerId is a Docker hex container ID declared \"integer\" upstream (GET /docker/{environmentId}/snapshot/containers/{containerId}); Docker assigns a 64-character hex string no int can carry",
	{OperationID: "ServiceImageStatus", ParamName: "serviceId"}:           "serviceId is a Docker Swarm service ID declared \"integer\" upstream (GET /docker/{environmentId}/services/{serviceId}/image_status); Swarm assigns an alphanumeric token no int can carry",
}

// pathParamGoType returns the Go type assembleOperationFields renders for one
// required path parameter: defaultGoType (typeOf's ordinary derivation from
// the resolved schema node) unless pathParamTypeOverrides names this exact
// (operationId, parameter) pair, in which case it is always "string" — the
// only override this table has ever needed, since every entry today excuses
// the identical defect (an opaque identifier Portainer never parses as a
// number, whatever the vendored specification calls it).
func pathParamGoType(operationID, paramName, defaultGoType string) string {
	if _, overridden := pathParamTypeOverrides[pathParamKey{OperationID: operationID, ParamName: paramName}]; overridden {
		return "string"
	}
	return defaultGoType
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
//
// requiresEdition, when non-empty, appends an `edition:"EE"` keyword —
// toolutil.FieldEditions' own tag, read back at catalog-build time to prune
// this field from a Community catalog's published schema (see
// actioncatalog.Catalog.InputSchema). Appended last, after "json" and
// "jsonschema", so an existing field's tag only ever grows a new trailing
// keyword rather than reordering what is already there.
func fieldTag(name string, required bool, description string, requiresEdition edition.Edition) string {
	content := "json:" + strconv.Quote(jsonNameValue(name, required))
	if description != "" {
		content += " jsonschema:" + strconv.Quote(description)
	}
	if requiresEdition != "" {
		content += " edition:" + strconv.Quote(string(requiresEdition))
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

// operationParameters projects op's raw parameter list onto the pairs
// internal/specnaming's disambiguation rule reads: the wire name each
// parameter contributes and the location it comes from. A parameter whose
// location this generator does not support is projected verbatim rather than
// filtered out — assembleOperationFields' own loop is what refuses it, with
// the message it has always used, and pre-filtering here would mean the rule
// disambiguated against an incomplete set of contributors.
func operationParameters(op operation) []specnaming.Parameter {
	out := make([]specnaming.Parameter, 0, len(op.Parameters))
	for _, param := range op.Parameters {
		name, _ := param["name"].(string)
		in, _ := param["in"].(string)
		out = append(out, specnaming.Parameter{Name: name, Origin: in})
	}
	return out
}

// bodyWireNames returns every top-level wire name a resolved request body
// node contributes, which is what internal/specnaming needs in order to
// decide whether a parameter's own name collides with one. It mirrors
// assembleOperationFields' own body branches exactly, in the same
// precedence: named properties first (rendered by the identical
// bodyJSONTag(splitWords(...)) call assembleFields makes for the same
// property — one function called twice with one input, not a second
// implementation of the rule), then a map-shaped body's single synthesised
// name, then nothing for a body neither branch can flatten.
func bodyWireNames(node *schemaNode) []string {
	switch {
	case node == nil:
		return nil
	case len(node.Properties) > 0:
		out := make([]string, 0, len(node.Properties))
		for _, p := range node.Properties {
			out = append(out, bodyJSONTag(splitWords(p.Name)))
		}
		return out
	case node.MapValue != nil:
		return []string{mapBodyFieldName}
	default:
		return nil
	}
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
// {path, query, body} that internal/specnaming's rule cannot disambiguate —
// silently letting the later source shadow the earlier one would mean a
// caller's value for one silently governs both.
//
// The cross-origin case of that collision is disambiguated rather than
// refused (internal/specnaming: the body keeps the plain name, the parameter
// carries its origin), because refusing it is not free. internal/specdiff's
// ShapeFromSpec applies the identical rule, and cmd/audit_spec_drift turns
// its refusal into a returned error before any FieldChange exists, so no
// api/spec-drift-allowlist.yaml entry can excuse one — a single unshapeable
// operation fails the drift gate for every domain in the catalog. What is
// still refused, here and in ShapeFromSpec, is a collision with no
// principled winner: two *body* properties rendering to one JSON tag
// (assembleFields, above), and a qualified name that some third contributor
// already holds (internal/specnaming's own refusal).
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

	// The request body is resolved before the parameter loop rather than
	// after it, because internal/specnaming's disambiguation rule cannot
	// decide whether a parameter's own name collides with the body until it
	// knows every name the body contributes. Only the resolution moved: the
	// body block below still raises each of its own refusals exactly where it
	// always did.
	bodySchema, err := doc.requestBodySchema(op.RequestBody)
	if err != nil {
		return nil, nil, fmt.Errorf("request body: %w", err)
	}
	var bodyNode *schemaNode
	if bodySchema != nil {
		bodyNode, err = res.resolve(bodySchema, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("request body: %w", err)
		}
	}

	// One wire name contributed by both a parameter and a body property is
	// disambiguated by the single rule internal/specdiff applies too
	// (internal/specnaming): the body keeps the plain name, the parameter
	// carries its origin. Both sides must produce the identical name, or the
	// drift audit reports one field added and another removed on an operation
	// nobody touched — see TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation,
	// which runs both derivations over every operation in both vendored
	// specifications for exactly that reason.
	paramNames, err := specnaming.ResolveParameters(operationParameters(op), bodyWireNames(bodyNode))
	if err != nil {
		return nil, nil, fmt.Errorf("merging parameters with the request body: %w", err)
	}

	for i, param := range op.Parameters {
		rawName, _ := param["name"].(string)
		in, _ := param["in"].(string)
		if rawName == "" {
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
			return nil, nil, fmt.Errorf("parameter %q: %w", rawName, err)
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
			return nil, nil, fmt.Errorf("parameter %q: location %q is not supported; only path and query parameters merge into a flat Input", rawName, in)
		}

		// name is the wire name this parameter actually publishes: its own,
		// unless the request body already took it (paramNames above). Every
		// refusal in this loop still names rawName, the identifier the
		// specification declares and the only one a reader can grep the
		// document for, and so do pathParamGoType/pathParamTakesMinimum
		// below, whose tables are keyed by what the specification says.
		name := paramNames[i]
		goName := goFieldName(splitWords(name))
		goType, err := typeOf(node, required, structPrefix+goName, structs)
		if err != nil {
			return nil, nil, fmt.Errorf("%s %q: %w", origin, rawName, err)
		}
		if in == "path" {
			// pathParamTypeOverrides only ever applies to the four opaque
			// identifiers documented on it — node.Type itself is left alone
			// (still "integer", exactly as the vendored spec declares it) so
			// every other use of this node's type (the Enum check just below,
			// the Minimum check further down, and the cross-check test that
			// keeps pathParamTypeOverrides honest against the real spec) still
			// sees what the specification actually says.
			goType = pathParamGoType(op.OperationID, rawName, goType)
		}
		f := fieldSpec{
			GoName:      goName,
			GoType:      goType,
			JSONName:    name, // path/query names are already the lower-camel-case OpenAPI declares; used verbatim unless disambiguated
			Required:    required,
			Description: node.Description,
			Origin:      fieldOrigin,
			WireName:    rawName,
		}
		if len(node.Enum) > 0 && isEnumScalar(node.Type) {
			f.Enum = node.Enum
			f.EnumScalarType = node.Type
		}
		if in == "path" && node.Type == "integer" && pathParamTakesMinimum(op.OperationID, rawName) {
			one := 1
			f.Minimum = &one
		}
		if err := add(origin, f); err != nil {
			return nil, nil, err
		}
	}

	if bodySchema != nil {
		node := bodyNode
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
				JSONName:    mapBodyFieldName,
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

// ceOperationsByID flattens operationsByDomain's per-tag grouping of the
// vendored Community Edition specification into the flat, by-operationId
// index ceEEFieldDiff needs to find one EE operation's CE counterpart.
// Mirrors ceOperationIDSet's identical flattening in actionspec.go, which
// only needed presence (map[string]bool); this needs the operation itself,
// to resolve its fields.
func ceOperationsByID(ceByTag map[string][]operation) map[string]operation {
	out := make(map[string]operation)
	for _, ops := range ceByTag {
		for _, op := range ops {
			out[op.OperationID] = op
		}
	}
	return out
}

// ceEEFieldDiff resolves ceOp's own fields (against the Community
// specification's resolver and document) and compares them, structurally,
// against eeStructName/eeFields/eeNested — the identical operation's already-
// resolved Business Edition shape — returning the set of fields present on
// the Business side and absent from the Community one.
//
// The comparison is nested-inclusive, matching this generator's own
// deterministic nested-struct naming (typeOf/assembleFields: a nested
// struct's name is always structPrefix+GoFieldName, independent of which
// specification produced it): a struct is looked up by that same name on
// both sides. A struct absent from the Community side entirely — an object
// property Business Edition added wholesale, not merely widened — marks
// every one of its own fields Business-only; a struct present on both sides
// is compared field by field, by JSON name, within that struct alone. The
// returned set is keyed "structName.jsonName", which is unique across one
// operation's whole tree because structSpec names already are (see
// duplicateStructName).
//
// ceRes must resolve against ceDoc (the Community specification's own
// resolver/document pair), never the Business Edition one eeRes/eeDoc used
// to produce eeFields/eeNested — comparing an operation's Community shape
// against itself, resolved from the wrong document, would silently report
// no divergence at all.
func ceEEFieldDiff(eeStructName string, eeFields []fieldSpec, eeNested []structSpec, ceOp operation, ceRes *resolver, ceDoc *document) (map[string]bool, error) {
	var ceNested []structSpec
	ceFields, _, err := assembleOperationFields(ceOp, ceRes, ceDoc, eeStructName, &ceNested)
	if err != nil {
		return nil, fmt.Errorf("resolve Community Edition shape: %w", err)
	}

	ceByStruct := map[string]structSpec{eeStructName: {Name: eeStructName, Fields: ceFields}}
	for _, s := range ceNested {
		ceByStruct[s.Name] = s
	}

	eeByStruct := map[string]structSpec{eeStructName: {Name: eeStructName, Fields: eeFields}}
	for _, s := range eeNested {
		eeByStruct[s.Name] = s
	}

	eeOnly := map[string]bool{}
	for structName, eeStruct := range eeByStruct {
		ceStruct, sharedStruct := ceByStruct[structName]
		var ceFieldNames map[string]bool
		if sharedStruct {
			ceFieldNames = make(map[string]bool, len(ceStruct.Fields))
			for _, f := range ceStruct.Fields {
				ceFieldNames[f.JSONName] = true
			}
		}
		for _, f := range eeStruct.Fields {
			if !sharedStruct || !ceFieldNames[f.JSONName] {
				eeOnly[structName+"."+f.JSONName] = true
			}
		}
	}
	return eeOnly, nil
}

// applyFieldEditionGate stamps RequiresEdition = edition.EE onto every field
// of eeFields (the operation's own top-level Input fields, keyed
// eeStructName+"."+JSONName in eeOnly) and every nested struct's own fields
// in eeNested (keyed structName+"."+JSONName) that eeOnly names — mutating
// both in place, the same way assembleOperationFields' own caller already
// relies on fields/nested being mutable shared state (see main.go's
// commitInput closure, which captures fields by reference).
func applyFieldEditionGate(eeStructName string, eeFields []fieldSpec, eeNested []structSpec, eeOnly map[string]bool) {
	for i := range eeFields {
		if eeOnly[eeStructName+"."+eeFields[i].JSONName] {
			eeFields[i].RequiresEdition = edition.EE
		}
	}
	for i := range eeNested {
		for j := range eeNested[i].Fields {
			if eeOnly[eeNested[i].Name+"."+eeNested[i].Fields[j].JSONName] {
				eeNested[i].Fields[j].RequiresEdition = edition.EE
			}
		}
	}
}
