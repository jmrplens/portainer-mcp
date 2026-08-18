package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

// specOperation is one operation documented in a vendored OpenAPI spec.
type specOperation struct {
	// OperationID is the exported Go identifier oapi-codegen derives from the
	// raw operationId, e.g. "SystemStatus" for "systemStatus". This is the
	// same form toolutil.ActionSpec.OperationID uses, so the two are
	// comparable without a translation step at every call site.
	OperationID string
	Method      string
	Path        string
	// Domain is the operation's first declared tag, or "" if it carries none.
	// It is descriptive only, for the report; coverage is decided by
	// OperationID alone.
	Domain string
	// Deprecated marks a route the spec itself flags as withdrawn. It is
	// still a real operation that must be covered or explicitly allow-listed
	// — "deprecated" is a legitimate reason to allow-list an operation, not a
	// license to drop it from the count uncounted.
	Deprecated bool
}

// unnamedOperation is a route a vendored document declares without an
// operationId and that internal/specnaming's table does not name either.
//
// It has no key to audit coverage against — that is what an operationId is
// for, and the whole audit is keyed by one — so it cannot be counted as
// covered or uncovered. It is carried out of the parser anyway, and named in
// the report, because the alternative is what this type exists to end: an
// operation that is simply not there, in a report whose bottom line says
// every operation is accounted for.
type unnamedOperation struct {
	Method string
	Path   string
	// Domain is the operation's first declared tag, or "" if it carries
	// none. Purely descriptive, as on specOperation.
	Domain string
}

// specDocument is everything one vendored OpenAPI document declares that
// this audit needs: the operations it can key by an operationId, and the
// routes it cannot.
//
// The two travel together, from one pass over one document, deliberately. A
// separate "list the unnamed ones" function would be a second traversal that
// could disagree with the first about what the document contains, which is a
// smaller version of the very defect this pairing exists to prevent.
type specDocument struct {
	// Operations is keyed by exported operationId, real or synthetic.
	Operations map[string]specOperation
	// Unnamed is sorted by path then method so the report is stable.
	Unnamed []unnamedOperation
}

// httpMethods are the OpenAPI verbs that name an operation. A path item can
// also carry "parameters", "summary" and other non-verb keys, and those are
// not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// parseSpecOperations decodes one vendored OpenAPI document and returns
// every operation it declares: the ones it can key by an operationId, and
// the ones it cannot.
//
// Both vendored documents carry entries with no operationId at all —
// fourteen of Community's 265 operations, one of Business's 442 — inherited
// from the upstream source. This function used to `continue` past every one
// of them, and that is the defect this pairing exists to fix. Skipped is not
// the same as uncovered: an operation that never enters the map never enters
// the denominator either, so it appears in no coverage figure, in no list of
// gaps, and in no failure. A route Portainer genuinely serves was dropped
// from this project's plan with every gate green, precisely because nothing
// could see it. Those fourteen and one are also why the probed totals read
// 251 and 441 rather than 265 and 442 (docs/api-divergences.md §6.2).
//
// So the skip is now the last resort rather than the first move.
// internal/specnaming's explicit table is consulted first, and a route it
// names enters the map under that name — the same name cmd/gen_applicability
// writes into internal/apiversion's operationIDs index, which is why the
// rule lives in a package both commands can import rather than in either
// one. A route nothing names is still not counted, because there is no name
// to count it against, but it is returned in Unnamed and printed by
// buildReport. An uncovered operation is honest; an invisible one is not.
func parseSpecOperations(data []byte) (specDocument, error) {
	// Each path item's value is decoded as raw JSON per key, rather than
	// straight into a fixed operation struct, because a path item can carry
	// non-verb keys too — "parameters" holds a JSON array, "summary" a plain
	// string — and decoding those into an object-shaped struct would fail the
	// whole document instead of simply being skipped as the non-operations
	// they are.
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return specDocument{}, fmt.Errorf("decode spec: %w", err)
	}

	ops := make(map[string]specOperation)
	var unnamed []unnamedOperation
	for path, methods := range doc.Paths {
		for method, raw := range methods {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var op struct {
				OperationID string   `json:"operationId"`
				Tags        []string `json:"tags"`
				Deprecated  bool     `json:"deprecated"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return specDocument{}, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			domain := ""
			if len(op.Tags) > 0 {
				domain = op.Tags[0]
			}
			name := exportedName(op.OperationID)
			if name == "" {
				synthetic, ok := specnaming.SyntheticOperationID(method, path)
				if !ok {
					unnamed = append(unnamed, unnamedOperation{Method: strings.ToUpper(method), Path: path, Domain: domain})
					continue
				}
				name = synthetic
			}
			// A synthetic name colliding with one the document publishes is
			// caught here, by the check that was already guarding against a
			// document naming two routes the same. Refusing outright is the
			// point: internal/specnaming's table records that each of its
			// entries collided with nothing when it was written, and this is
			// what keeps that claim true against a future respec.
			if existing, dup := ops[name]; dup {
				declared := fmt.Sprintf("operationId %q", op.OperationID)
				if op.OperationID == "" {
					declared = "the name internal/specnaming's table gives this otherwise unnamed route"
				}
				return specDocument{}, fmt.Errorf(
					"%s (exported %q) is declared for both %s %s and %s %s",
					declared, name, existing.Method, existing.Path, strings.ToUpper(method), path)
			}
			ops[name] = specOperation{
				OperationID: name,
				Method:      strings.ToUpper(method),
				Path:        path,
				Domain:      domain,
				Deprecated:  op.Deprecated,
			}
		}
	}
	sort.Slice(unnamed, func(i, j int) bool {
		if unnamed[i].Path != unnamed[j].Path {
			return unnamed[i].Path < unnamed[j].Path
		}
		return unnamed[i].Method < unnamed[j].Method
	})
	return specDocument{Operations: ops, Unnamed: unnamed}, nil
}

// exportedName converts a raw OpenAPI operationId (e.g. "systemStatus") into
// the exported Go identifier oapi-codegen derives from it ("SystemStatus").
//
// This mirrors cmd/gen_applicability's identical transform, which is what
// populates toolutil.ActionSpec.OperationID's expected form throughout this
// project: every operationId in the vendored specs is plain ASCII camelCase
// or PascalCase with no separators, so upper-casing the first rune is the
// whole transformation.
func exportedName(id string) string {
	if id == "" {
		return id
	}
	r := []rune(id)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
