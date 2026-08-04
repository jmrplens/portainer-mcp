// Package specdiff is the one comparison engine that decides whether an
// operation's parameter shape changed, regardless of what the two shapes
// being compared came from.
//
// Two consumers need the identical comparison for different operands: a
// drift audit compares the action catalog against the vendored
// specification it was generated from, and a delta tool compares the
// vendored specification against a candidate newer one. Deriving "did this
// operation's shape change" twice — once per consumer — is exactly the
// mistake that lost 127 operations in P3.0, where the domain-to-tag
// relationship was derived one way by the catalog and another way by the
// generator, and nothing noticed until an audit was built to cross-check
// them. This package exists so that never happens for parameter shape: both
// consumers call Compare on an OperationShape produced by ShapeFromCatalog or
// ShapeFromSpec, never a parallel derivation of their own.
package specdiff

// FieldShape describes one flattened, top-level field of an operation's
// parameter shape: a path parameter, a query parameter, or a top-level
// request-body property. It deliberately does not describe anything nested
// inside a body property — see OperationShape's doc comment for why a flat
// shape is what a model is actually told.
type FieldShape struct {
	// JSONName is the field's wire name: a path/query parameter's own OpenAPI
	// name, or a request body property's JSON name.
	JSONName string
	// Type is the JSON Schema "type" keyword for this field — "string",
	// "integer", "number", "boolean", "array" or "object" — never a
	// JSON-Schema-2020-12-style nullable array such as ["null", "string"]:
	// ShapeFromCatalog collapses that representation (the reflector's own
	// encoding of a Go pointer or slice field) down to the one substantive
	// type it names, because the vendored specification never expresses
	// optionality that way and a field that is optional on both sides must
	// not be reported as a type change purely because of how each side
	// spells "this may be absent".
	Type string
	// Required reports whether this field's JSON name appears in its parent
	// schema's "required" list (path parameters are always required, per
	// OpenAPI itself).
	Required bool
	// Enum is the field's allowed values, each rendered with fmt.Sprintf
	// ("%v", v) so a JSON number decoded as float64 and a JSON string compare
	// the same way regardless of which side produced them. Nil when the
	// field carries no enum constraint.
	Enum []string
	// Description is the field's model-facing prose: a parameter's own
	// "description", or a body property's resolved schema "description".
	Description string
	// Origin names which part of the operation this field came from: "path",
	// "query" or "body". ShapeFromSpec always sets this, because the
	// vendored specification states a parameter's "in" and which schema a
	// property was flattened out of. ShapeFromCatalog leaves it "": neither
	// toolutil.ActionSpec nor the reflected InputSchema it publishes records
	// which of the three a field was routed from — that routing is a
	// generation-time fact that never reaches the published schema a model
	// reads, so there is nothing for ShapeFromCatalog to read it back from.
	// Compare treats an empty Origin on either side as "not stated", not as a
	// value that differs from a stated one — see Compare's doc comment.
	Origin string
}

// OperationShape is one operation's flattened, model-facing parameter shape:
// exactly the fields a caller must supply, none of what is nested inside a
// request-body object property.
//
// Flat, not nested, on purpose: FieldShape has no field of its own type, so a
// change several levels inside a body object's nested schema is out of scope
// for this shape — it does not reach a generated Input struct's own fields
// (cmd/gen_action_inputs flattens only the top level into named fields; a
// nested object property renders as one Go field of a nested struct type,
// routed to a handler whole) and it is not part of what ShapeFromCatalog can
// even see: toolutil.ActionSpec's reflected InputSchema, like the generated
// Input struct it comes from, only names its own top-level properties.
type OperationShape struct {
	OperationID string
	Method      string
	Path        string
	Fields      []FieldShape
}

// ChangeKind classifies one FieldChange. See Compare's doc comment for which
// kind wins when several are true of the same field: exactly one, never a
// blanket "field changed".
type ChangeKind string

const (
	// ChangeAdded means the field exists in after but not before.
	ChangeAdded ChangeKind = "added"
	// ChangeRemoved means the field exists in before but not after.
	ChangeRemoved ChangeKind = "removed"
	// ChangeType means the field's JSON Schema type differs.
	ChangeType ChangeKind = "type"
	// ChangeRequiredness means the field moved between required and
	// optional.
	ChangeRequiredness ChangeKind = "requiredness"
	// ChangeEnum means the field's set of allowed values differs.
	ChangeEnum ChangeKind = "enum"
	// ChangeDescription means only the field's prose changed — the
	// consequential/cosmetic distinction the whole engine exists to draw:
	// this kind alone cannot make a caller send the wrong thing.
	ChangeDescription ChangeKind = "description"
	// ChangeOrigin means the field moved between path, query and body.
	ChangeOrigin ChangeKind = "origin"
)

// FieldChange is one detected difference between a before and an after
// FieldShape sharing the same JSONName (or, for ChangeAdded/ChangeRemoved,
// existing on only one side).
//
// Before and After carry only the value relevant to Kind — e.g. a ChangeType
// entry's Before/After are the two type strings, not the whole field — so a
// report can print exactly what changed without a reader reconstructing it
// from two full FieldShape values.
type FieldChange struct {
	JSONName string
	Kind     ChangeKind
	Before   string
	After    string
}

// SpecOperation is one operation as declared by a vendored OpenAPI document,
// carrying just enough of it — resolved against the same document's
// components — for ShapeFromSpec to flatten into an OperationShape.
//
// It is deliberately not the full richness cmd/gen_action_inputs's own
// internal operation/document types carry (nested struct assembly, Go naming,
// credential/minimum bookkeeping): that package is `package main`, so nothing
// outside it can import its types at all, and most of what it carries has no
// counterpart in a flat parameter-shape comparison. See LoadSpecOperation's
// doc comment for the reimplement-vs-move decision this type is the result
// of.
type SpecOperation struct {
	OperationID string
	Method      string
	Path        string
	// Parameters are the operation's raw, undecoded "parameters" array
	// entries (each a "name"/"in"/"required"/"schema"/"description" object),
	// left raw so ShapeFromSpec can resolve each entry's own schema node
	// against Schemas the same way a request-body property is resolved.
	Parameters []map[string]any
	// RequestBody is the operation's raw "requestBody" node, or nil when the
	// operation has none. It may itself be a $ref into RequestBodies, the
	// same indirection cmd/gen_action_inputs's requestBodySchema resolves.
	RequestBody map[string]any
	// Schemas is the document's components.schemas, needed to resolve a
	// $ref/allOf chain down to a concrete type/description/enum.
	Schemas map[string]any
	// RequestBodies is the document's components.requestBodies, needed only
	// when RequestBody is itself a $ref into it.
	RequestBodies map[string]any
}
