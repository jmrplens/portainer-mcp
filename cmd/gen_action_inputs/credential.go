package main

import (
	"errors"
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// This file is the generator's half of task-4b's finding: P2's review caught
// registries.create/inspect/update returning Password and AccessToken in a
// success response, and no fixture carried a password, so no test could have
// caught it. Measured against the vendored Business Edition specification,
// the same shape recurs across dozens of operations this generator has not
// written a domain for yet. Remembering to redact each one by hand, the same
// way registries.go's own redact/redactList do today, is exactly the kind of
// fact this project has already lost track of once (P3.0's "one fact,
// derived twice" defect, cited throughout this generator's own doc comments)
// — so this makes forgetting a structural impossibility instead of a review
// habit: an operation whose success response can carry a credential-shaped
// field never gets a bare generated handler. It gets one that calls a
// domain-supplied redaction wrapper, or generation refuses by name.
//
// "Credential-shaped" is defined exactly once, in
// toolutil.IsCredentialShapedName, and read by two consumers: the runtime
// reflective guard every domain's own test uses to walk a real, populated
// response value (toolutil.WalkForCredentialShapedFields), and
// responseCredentialFields below, which walks the *resolved OpenAPI schema*
// of an operation's success response — property names only, since generation
// time has no values to inspect. Both call the same function so the two can
// never quietly disagree about what counts.

// successResponseSchemas returns the raw (unresolved) JSON Schema node of
// every 2xx response op declares with an application/json body, skipping any
// 2xx entry that carries no body at all (a 204, or a 200 with no "content" —
// both real and common in the vendored specs). Multiple entries are possible
// (AddonInstall's JSON200/JSON201 is the real example — see handler.go's
// responseInfoFor, which refuses to guess between them at the Go-client
// level); this collects every one of them rather than picking one, since a
// credential reachable through *either* success shape is exactly as
// disqualifying as one reachable through both.
func successResponseSchemas(op operation) []map[string]any {
	var schemas []map[string]any
	// Iterated over a sorted key list, not map order: this generator's other
	// diagnostics (checkDomainTagsCoverSpec, dangerMismatchWarnings) all avoid
	// depending on Go's randomised map iteration for anything that reaches
	// stderr or an error message, and the field-path list this feeds into
	// generation errors is exactly that.
	codes := make([]string, 0, len(op.Responses))
	for code := range op.Responses {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		if len(code) == 0 || code[0] != '2' {
			continue
		}
		resp := op.Responses[code]
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

// credentialShapedFieldPaths walks node (already resolved — every $ref
// followed, every allOf branch merged, by the same resolver.resolve every
// other schema in this generator goes through) and returns the dotted/
// bracketed path of every property whose name matches
// toolutil.IsCredentialShapedName, at any depth reachable through an object's
// properties, an array's items or a map's value.
//
// depth is bounded by maxSchemaDepth, the same bound resolver.resolve already
// enforces while building node in the first place — resolve() cannot produce
// a cyclic node (a genuine $ref cycle fails resolution itself, loudly, before
// this function ever runs), so node is always a finite tree and this bound is
// pure defence in depth, not something expected to fire.
func credentialShapedFieldPaths(node *schemaNode, path string, depth int) []string {
	if node == nil || depth > maxSchemaDepth {
		return nil
	}
	var out []string
	for _, p := range node.Properties {
		fieldPath := path + "." + p.Name
		// Shape-aware: the resolved node carries the property's own JSON
		// Schema type, which is what rules out a boolean toggle or a
		// timestamp whose name happens to contain a marker word. See
		// toolutil.IsCredentialShapedField for the measured justification.
		shape := toolutil.ShapeUnknown
		if p.Schema != nil {
			shape = toolutil.FieldShapeOfJSONType(p.Schema.Type)
		}
		if toolutil.IsCredentialShapedField(p.Name, shape) {
			out = append(out, fieldPath)
		}
		out = append(out, credentialShapedFieldPaths(p.Schema, fieldPath, depth+1)...)
	}
	if node.Items != nil {
		out = append(out, credentialShapedFieldPaths(node.Items, path+"[]", depth+1)...)
	}
	if node.MapValue != nil {
		out = append(out, credentialShapedFieldPaths(node.MapValue, path+"{}", depth+1)...)
	}
	return out
}

// errResponseSchemaUnresolvable marks the one failure mode that must not
// abort the whole run: op's success-response schema could not be resolved at
// all, so whether it carries a credential is unknown.
//
// "Unknown" is still a refusal — this generator never emits a handler it
// cannot prove is safe — but it is a refusal about *one operation*, and
// run()'s domain loop treats it as such: the domain containing it is left
// unwritten and named, every other domain is generated normally, and the
// command still exits non-zero. Before this distinction existed, a single
// unresolvable schema returned a plain error that run() propagated
// immediately, so one defect in one operation stopped all 46 domain
// directories from regenerating and turned CI's regeneration-freshness check
// red for the entire repository.
var errResponseSchemaUnresolvable = errors.New("success response schema could not be resolved")

// responseCredentialFields resolves every success-response schema op
// declares and returns the sorted, de-duplicated union of every
// credential-shaped field path reachable in any of them, or nil when none
// is.
func responseCredentialFields(op operation, res *resolver) ([]string, error) {
	seen := map[string]bool{}
	var out []string
	for _, raw := range successResponseSchemas(op) {
		node, err := res.resolve(raw, 0)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errResponseSchemaUnresolvable, err)
		}
		for _, path := range credentialShapedFieldPaths(node, op.OperationID, 0) {
			if !seen[path] {
				seen[path] = true
				out = append(out, path)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// redactionWrapperName is the naming convention a domain's hand-written file
// must follow to supply a redaction wrapper for op: "redact" + OperationID,
// e.g. "redactRegistryList". checkCredentialRedaction below is the only
// caller that needs to know this name; renderHandlerFunc (handler.go) never
// re-derives it — it receives the resolved name (or "") on handlerSpec.RedactWith.
func redactionWrapperName(operationID string) string {
	return "redact" + operationID
}

// checkCredentialRedaction decides whether op needs a domain-supplied
// redaction wrapper before this generator will emit a handler for it, and
// which function name that wrapper must have.
//
// When op's success response can carry no credential-shaped field, it
// returns ("", nil): a bare generated handler, exactly as before this file
// existed. When it can, and the domain's own hand-written files (funcNames,
// from scanHandOverrides — the same AST scan that already detects a
// hand-declared ActionSpec or handler function) already declare a function
// named redactionWrapperName(op.OperationID), it returns that name: the
// generated handler will call it instead of returning the response body
// directly (see handler.go's renderHandlerFunc). When neither is true, it
// refuses — a generation-time error naming the operation and every flagged
// field path, not a silently-emitted handler a reviewer would have to notice
// carries no redaction. A domain author who wires the wrapper's name but
// gets its parameter or return type wrong instead gets a Go compile error
// from the reference renderHandlerFunc emits — the "generator refusal or a
// compile error" property task-4b's brief calls for either way.
//
// This deliberately does not know which of the flagged fields the wrapper
// actually redacts, only that at least one exists and a wrapper by the right
// name is present: which fields to scrub is a per-domain judgement (see
// registries.go's own redact, which also drops TLSConfig wholesale rather
// than field by field), not something this generator can decide from a
// schema alone.
// # Overridden operations
//
// This check runs for every operation, including one a domain already covers
// with its own hand-written handler. It used to run only for operations this
// generator was about to emit code for, because run()'s loop skipped past it
// the moment overrideReason reported a hand-written handler — which meant the
// one escape hatch from generation was also an escape hatch from the guard,
// reproducing P2's original defect (registries' hand-written create/inspect/
// update returning Password and AccessToken unredacted) in the exact code path
// built to make it impossible.
//
// The generator cannot inspect arbitrary hand-written code to confirm it
// scrubs anything, so it requires the domain to *state* that it does, using
// the same convention a generated handler already follows: a function named
// redactionWrapperName(op.OperationID). For an overridden operation that
// function is not dead weight — the hand-written handler is expected to call
// it, which is what makes the statement true rather than merely present. The
// generated reflective-guard test (see render.go) is what then checks the
// wrapper actually removes the fields, so a pass-through cannot satisfy this
// by name alone.
func checkCredentialRedaction(op operation, res *resolver, funcNames map[string]bool, overridden bool) (string, error) {
	fields, err := responseCredentialFields(op, res)
	if err != nil {
		// Wrapped, not reformatted: run() distinguishes
		// errResponseSchemaUnresolvable from an ordinary refusal and must
		// still be able to see it through this layer.
		return "", fmt.Errorf("%s: %w", op.OperationID, err)
	}
	if len(fields) == 0 {
		return "", nil
	}
	wrapper := redactionWrapperName(op.OperationID)
	if !funcNames[wrapper] {
		if overridden {
			return "", fmt.Errorf(
				"%s: success response can carry credential-shaped field(s) %v, and this operation is covered by a hand-written handler this generator cannot inspect; "+
					"declare func %s(<the response's success-field type>) any in this domain's own hand-written file and call it from that handler, "+
					"which is how a hand-written handler states that it redacts (see internal/tools/registries's redact/redactList for the pattern)",
				op.OperationID, fields, wrapper)
		}
		return "", fmt.Errorf(
			"%s: success response can carry credential-shaped field(s) %v; "+
				"declare func %s(<the response's success-field type>) any in this domain's own hand-written file before generating "+
				"(see toolutil.IsCredentialShapedName and internal/tools/registries's redact/redactList for the pattern)",
			op.OperationID, fields, wrapper)
	}
	return wrapper, nil
}
