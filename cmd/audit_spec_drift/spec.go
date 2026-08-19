package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

// specOperation is one operation declared by a vendored OpenAPI document,
// carrying both what specdiff needs to flatten its parameter shape (Op) and
// what this audit's own report needs to describe it (Domain, Deprecated).
//
// Op's Schemas/RequestBodies point at the same maps for every operation
// decoded from one document: decoding those once per document, rather than
// once per operation (specdiff.LoadSpecOperation's own approach, built for a
// caller that wants exactly one named operation), is what keeps auditing all
// 441 operations across two ~1MB documents from re-parsing each document
// hundreds of times over.
type specOperation struct {
	Op         specdiff.SpecOperation
	Domain     string
	Deprecated bool
	// Responses is the operation's raw, undecoded "responses" object, keyed by
	// status code. It is not part of specdiff.SpecOperation (see that type's
	// own doc comment: specdiff is deliberately scoped to request parameter
	// shape, never response shape) but this audit needs it for one further
	// thing specdiff does not cover at all: whether an operation's success
	// response can carry a credential-shaped field — see credential.go.
	Responses map[string]map[string]any
}

// httpMethods are the OpenAPI verbs that name an operation inside a path
// item, mirroring cmd/audit_1to1 and internal/specdiff's identical list: a
// path item can also carry non-verb keys ("parameters", "summary"), which are
// not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// exportedName converts a raw OpenAPI operationId (e.g. "systemStatus") into
// the exported Go identifier oapi-codegen derives from it ("SystemStatus"),
// mirroring cmd/audit_1to1's identical transform: every operationId in the
// vendored specs is plain ASCII camelCase or PascalCase with no separators,
// so upper-casing the first rune is the whole transformation.
func exportedName(id string) string {
	if id == "" {
		return id
	}
	r := []rune(id)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

// parseSpecOperations decodes one vendored OpenAPI document and returns
// every operation it declares, keyed by its exported operationId, ready for
// specdiff.ShapeFromSpec.
//
// An operation with no operationId is looked up in internal/specnaming's
// table first, and skipped only if that table does not name it either —
// mirroring cmd/audit_1to1's parseSpecOperations, which was taught the same
// rule for the same reason, and cmd/gen_applicability's applySyntheticIDs,
// which writes the same names into internal/apiversion's operationIDs index.
//
// Skipping unconditionally, which this function used to do, is not merely a
// missing feature: it is the same invisibility internal/specnaming exists to
// end, one gate further along. A route neither document names can still be
// declared as a catalog action (the name comes from that table, and
// cmd/gen_applicability puts it in the edition index so actioncatalog can
// resolve it), and when one is, this audit refused to run at all —
// "OperationID ... resolves in neither vendored spec" — for an operation the
// documents describe completely apart from its name. Both parameters, the
// response schema and the descriptions are all there to compare against; only
// the key to find them by was missing.
//
// The one thing that must not happen is a synthetic name quietly displacing a
// published one. It cannot: a name arrived at either way goes through the same
// duplicate check below, which refuses outright rather than overwriting —
// exactly as cmd/audit_1to1's own collisionError does, and what keeps
// internal/specnaming's "verified collision-free" claim honest against a
// future respec.
func parseSpecOperations(data []byte) (map[string]specOperation, error) {
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas       map[string]any `json:"schemas"`
			RequestBodies map[string]any `json:"requestBodies"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}

	// Paths are iterated in sorted order so a document declaring the same
	// operationId twice (a spec defect, not something to paper over) reports
	// deterministically rather than depending on Go's randomised map
	// iteration for which occurrence "wins" the duplicate error.
	paths := make([]string, 0, len(doc.Paths))
	for path := range doc.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	ops := make(map[string]specOperation)
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
				OperationID string                    `json:"operationId"`
				Summary     string                    `json:"summary"`
				Description string                    `json:"description"`
				Tags        []string                  `json:"tags"`
				Deprecated  bool                      `json:"deprecated"`
				Parameters  []map[string]any          `json:"parameters"`
				RequestBody map[string]any            `json:"requestBody"`
				Responses   map[string]map[string]any `json:"responses"`
			}
			if err := json.Unmarshal(methods[method], &op); err != nil {
				return nil, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			name := exportedName(op.OperationID)
			if name == "" {
				synthetic, named := specnaming.SyntheticOperationID(method, path)
				if !named {
					continue
				}
				name = synthetic
			}
			domain := ""
			if len(op.Tags) > 0 {
				domain = op.Tags[0]
			}
			if existing, dup := ops[name]; dup {
				return nil, fmt.Errorf(
					"operationId %q is declared for both %s %s and %s %s (%s)",
					name, existing.Op.Method, existing.Op.Path, strings.ToUpper(method), path,
					nameOrigin(op.OperationID))
			}
			ops[name] = specOperation{
				Op: specdiff.SpecOperation{
					OperationID:   name,
					Method:        strings.ToUpper(method),
					Path:          path,
					Summary:       op.Summary,
					Description:   op.Description,
					Parameters:    op.Parameters,
					RequestBody:   op.RequestBody,
					Schemas:       doc.Components.Schemas,
					RequestBodies: doc.Components.RequestBodies,
				},
				Domain:     domain,
				Deprecated: op.Deprecated,
				Responses:  op.Responses,
			}
		}
	}
	return ops, nil
}

// nameOrigin describes where the second of two colliding names came from, so
// the refusal above tells a published operationId apart from one
// internal/specnaming's table supplied. Getting that backwards would send a
// reader to remove the wrong thing — the same reasoning cmd/audit_1to1's
// collisionError records for its own, richer version of this message.
func nameOrigin(rawOperationID string) string {
	if rawOperationID == "" {
		return "the second is named by internal/specnaming's table, the document leaving it unnamed"
	}
	return fmt.Sprintf("the second declares operationId %q", rawOperationID)
}

// resolveSpecOperation looks up operationID in eeOps first, falling back to
// ceOps, and reports which one supplied it ("EE" or "CE").
//
// This is not an arbitrary choice: it is the identical precedence
// cmd/gen_action_inputs itself uses as the source of truth for every
// generated Input struct (its -spec flag defaults to the Business Edition
// document; the Community Edition document is consulted only to classify
// which generated actions are CE-only — see that command's main.go). An
// action generated against the Business Edition specification must be
// compared against that same specification, not against whichever one this
// audit happened to check first, or the comparison would not be checking
// what actually generated the field this audit is trying to protect.
// SystemUpgrade is the one operation this falls back for: it exists only in
// the Community Edition document (see internal/tools/system's own doc
// comment), so it is never in eeOps at all.
func resolveSpecOperation(operationID string, eeOps, ceOps map[string]specOperation) (specOperation, string, bool) {
	if op, ok := eeOps[operationID]; ok {
		return op, "EE", true
	}
	if op, ok := ceOps[operationID]; ok {
		return op, "CE", true
	}
	return specOperation{}, "", false
}
