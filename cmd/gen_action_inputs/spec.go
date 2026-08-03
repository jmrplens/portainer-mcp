package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// readFileIn reads name from dir, refusing to read anything name resolves
// outside of it. Mirrors cmd/audit_1to1's identical helper and
// cmd/gen_applicability's operationsIn confinement: -spec is an operator- or
// CI-supplied flag value, and this bounds what it can ever cause this
// command to open.
func readFileIn(dir, name string) ([]byte, error) {
	path := filepath.Clean(filepath.Join(dir, name))
	if rel, err := filepath.Rel(dir, path); err != nil || strings.HasPrefix(rel, "..") {
		return nil, fmt.Errorf("refuse to read %s: escapes %s", path, dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return data, nil
}

// httpMethods are the OpenAPI verbs that name an operation, mirroring
// cmd/audit_1to1's identical list: a path item can also carry non-verb keys
// ("parameters", "summary"), which are not operations.
var httpMethods = map[string]bool{
	"get": true, "post": true, "put": true, "delete": true,
	"patch": true, "head": true, "options": true,
}

// document is a vendored OpenAPI specification, decoded generically so that
// arbitrary schema shapes ($ref, allOf, oneOf) can be navigated without a
// fixed Go type for every possible node.
type document struct {
	schemas       map[string]any
	requestBodies map[string]any
}

// loadDocument reads and decodes one vendored specification file.
func loadDocument(path string) (*document, map[string]map[string]json.RawMessage, error) {
	raw, err := readFileIn(filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, nil, err
	}
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas       map[string]any `json:"schemas"`
			RequestBodies map[string]any `json:"requestBodies"`
		} `json:"components"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return &document{schemas: doc.Components.Schemas, requestBodies: doc.Components.RequestBodies}, doc.Paths, nil
}

// operation is one operation declared in the vendored spec, with just enough
// decoded to build an Input struct: its identity, its domain, and its raw
// parameter and request-body nodes (left raw so schemaResolver can navigate
// $ref/allOf/oneOf without a fixed shape).
type operation struct {
	OperationID string // exported form, e.g. "TagCreate"
	Method      string
	Path        string
	Domain      string // first declared tag, "" if none
	Parameters  []map[string]any
	RequestBody map[string]any // nil if the operation has no body
}

// operationsByDomain decodes every operation in paths and groups it by
// domain (its first OpenAPI tag), keyed for deterministic iteration by the
// caller sorting the returned domain names itself.
func operationsByDomain(paths map[string]map[string]json.RawMessage) (map[string][]operation, error) {
	byDomain := map[string][]operation{}
	for path, methods := range paths {
		for method, raw := range methods {
			if !httpMethods[strings.ToLower(method)] {
				continue
			}
			var op struct {
				OperationID string           `json:"operationId"`
				Tags        []string         `json:"tags"`
				Parameters  []map[string]any `json:"parameters"`
				RequestBody map[string]any   `json:"requestBody"`
			}
			if err := json.Unmarshal(raw, &op); err != nil {
				return nil, fmt.Errorf("decode %s %s: %w", strings.ToUpper(method), path, err)
			}
			if op.OperationID == "" {
				continue
			}
			domain := ""
			if len(op.Tags) > 0 {
				domain = op.Tags[0]
			}
			o := operation{
				OperationID: exportedName(op.OperationID),
				Method:      strings.ToUpper(method),
				Path:        path,
				Domain:      domain,
				Parameters:  op.Parameters,
				RequestBody: op.RequestBody,
			}
			byDomain[domain] = append(byDomain[domain], o)
		}
	}
	for domain := range byDomain {
		sort.Slice(byDomain[domain], func(i, j int) bool {
			return byDomain[domain][i].OperationID < byDomain[domain][j].OperationID
		})
	}
	return byDomain, nil
}

// requestBodySchema returns the single application/json schema node of an
// operation's request body, resolving one components.requestBodies $ref
// first if the body itself is declared that way.
//
// requestBody.required (whether the body may be omitted altogether) is
// deliberately not returned: it does not loosen what the body schema's own
// "required" property list says, which is the only requiredness
// assembleOperationFields needs, and no operation this generator is run
// against today declares an optional body.
//
// It refuses — the caller surfaces this as a hard generation error, per this
// generator's rule of refusing rather than guessing — an operation whose body
// declares more than one content type: CustomTemplateCreate is the real
// example in the vendored spec (application/json and multipart/form-data),
// and there is no single Go type that faithfully represents both without
// picking one arbitrarily.
func (d *document) requestBodySchema(rb map[string]any) (map[string]any, error) {
	if rb == nil {
		return nil, nil
	}
	if ref, ok := rb["$ref"].(string); ok {
		name := strings.TrimPrefix(ref, "#/components/requestBodies/")
		resolved, ok := d.requestBodies[name].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unresolved requestBody $ref %q", ref)
		}
		rb = resolved
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
		return nil, fmt.Errorf("requestBody declares %d content types (%s); no single Go type represents more than one", len(content), strings.Join(kinds, ", "))
	}
	for _, v := range content {
		entry, _ := v.(map[string]any)
		schema, _ := entry["schema"].(map[string]any)
		return schema, nil
	}
	return nil, nil
}
