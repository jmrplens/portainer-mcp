package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
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

// httpMethods are the OpenAPI verbs that name an operation. A path item can
// also carry "parameters", "summary" and other non-verb keys, and those are
// not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// parseSpecOperations decodes one vendored OpenAPI document and returns every
// operation it declares, keyed by its exported operationId.
//
// An operation with no operationId is skipped rather than treated as an
// error: both vendored specs carry a handful of such entries (fourteen in the
// Community Edition document, one in Business Edition — mostly webhook and
// websocket routes) inherited from the upstream source, and none of them can
// ever become an oapi-codegen-generated method for the catalog to reference,
// so there is no name to audit coverage against.
func parseSpecOperations(data []byte) (map[string]specOperation, error) {
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
		return nil, fmt.Errorf("decode spec: %w", err)
	}

	ops := make(map[string]specOperation)
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
				return nil, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID == "" {
				continue
			}
			name := exportedName(op.OperationID)
			domain := ""
			if len(op.Tags) > 0 {
				domain = op.Tags[0]
			}
			if existing, dup := ops[name]; dup {
				return nil, fmt.Errorf(
					"operationId %q (exported %q) is declared for both %s %s and %s %s",
					op.OperationID, name, existing.Method, existing.Path, strings.ToUpper(method), path)
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
	return ops, nil
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
