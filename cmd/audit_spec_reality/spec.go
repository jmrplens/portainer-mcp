package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode"
)

// specOperation is one operation documented in a vendored OpenAPI spec.
type specOperation struct {
	// OperationID is the exported Go identifier oapi-codegen derives from
	// the raw operationId, e.g. "SystemStatus" for "systemStatus".
	OperationID string
	Method      string
	Path        string
	// Domain is the operation's first declared tag, or "" if it carries
	// none. Descriptive only, for the report.
	Domain string
	// Public is true when the operation declares no security requirement at
	// all — neither the document nor the operation names one, so nothing
	// checks a credential before the handler runs. Portainer's own router
	// calls these PublicAccess routes.
	//
	// This is derived from the document rather than from a hand-written list
	// of paths, deliberately. A list would be a second declaration of a fact
	// the specification already states, and it would go stale silently the
	// next time Portainer publishes a spec that makes a route public — which
	// is exactly the class of drift this project has already been caught by.
	// The vendored EE 2.44.0 document declares 24 such operations and CE 12;
	// see auditLeg for what the audit does about them.
	Public bool
}

// httpMethods are the OpenAPI verbs that name an operation. A path item can
// also carry "parameters", "summary" and other non-verb keys.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// parseSpecOperations decodes one vendored OpenAPI document and returns
// every operation it declares, keyed by its exported operationId.
//
// An operation with no operationId is skipped rather than treated as an
// error, the same rule cmd/audit_1to1's parser of the same name applies to
// the same documents (see that package's doc comment for the exact count
// this drops in each vendored spec) — this tool cannot name what it has no
// name for, and there is nothing to look up in a Go client or a catalog for
// it either.
//
// This is a deliberate duplicate of cmd/audit_1to1/spec.go's function of the
// same name and shape, not a shared import: cmd/gen_action_inputs,
// cmd/audit_1to1 and cmd/gen_applicability each already carry their own copy
// of this exact parsing step, and this command follows that existing
// convention rather than introducing the first shared "read an OpenAPI
// document" package four tools in.
func parseSpecOperations(data []byte) (map[string]specOperation, error) {
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
				OperationID string                 `json:"operationId"`
				Tags        []string               `json:"tags"`
				Security    *[]map[string][]string `json:"security"`
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
				// Public is derived from the document, never from a
				// hand-maintained list of paths: see specOperation.Public.
				Public: op.Security == nil || len(*op.Security) == 0,
			}
		}
	}
	return ops, nil
}

// exportedName converts a raw OpenAPI operationId (e.g. "systemStatus") into
// the exported Go identifier oapi-codegen derives from it ("SystemStatus").
func exportedName(id string) string {
	if id == "" {
		return id
	}
	r := []rune(id)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}
