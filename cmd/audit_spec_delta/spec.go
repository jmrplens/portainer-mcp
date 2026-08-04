package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// httpMethods are the OpenAPI verbs that name an operation inside a path
// item, mirroring cmd/audit_1to1, cmd/audit_spec_drift and
// internal/specdiff's identical list: a path item can also carry non-verb
// keys ("parameters", "summary"), which are not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// specOperation is one operation declared by a vendored OpenAPI document,
// carrying both what specdiff needs to flatten its parameter shape (Op) and
// what this tool's own report needs to name and place it: the raw OpenAPI
// tag it declared, unresolved, so a domain can be looked up from it later
// through toolutil.DomainTags rather than assumed equal to it (see this
// package's domain.go for why that lookup, not the tag itself, is what a
// work list must group by).
type specOperation struct {
	Op  specdiff.SpecOperation
	Tag string
}

// parseSpecOperations decodes one vendored (or freshly bundled candidate)
// OpenAPI document and returns every operation it declares, keyed by its raw
// operationId exactly as the document spells it — unlike cmd/audit_spec_drift's
// identically-named function, this one does not PascalCase it, because there
// is no generated Go identifier on either side of a spec-to-spec comparison
// to match against.
//
// An operation with no operationId is skipped: the vendored specs' own
// handful of such entries (mostly webhook and websocket routes) can never be
// tracked across a version boundary by identity, so there is nothing for
// this tool to compare across two spec documents either.
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
				OperationID string           `json:"operationId"`
				Tags        []string         `json:"tags"`
				Parameters  []map[string]any `json:"parameters"`
				RequestBody map[string]any   `json:"requestBody"`
			}
			if err := json.Unmarshal(methods[method], &op); err != nil {
				return nil, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID == "" {
				continue
			}
			tag := ""
			if len(op.Tags) > 0 {
				tag = op.Tags[0]
			}
			if existing, dup := ops[op.OperationID]; dup {
				return nil, fmt.Errorf(
					"operationId %q is declared for both %s %s and %s %s",
					op.OperationID, existing.Op.Method, existing.Op.Path, strings.ToUpper(method), path)
			}
			ops[op.OperationID] = specOperation{
				Op: specdiff.SpecOperation{
					OperationID:   op.OperationID,
					Method:        strings.ToUpper(method),
					Path:          path,
					Parameters:    op.Parameters,
					RequestBody:   op.RequestBody,
					Schemas:       doc.Components.Schemas,
					RequestBodies: doc.Components.RequestBodies,
				},
				Tag: tag,
			}
		}
	}
	return ops, nil
}
