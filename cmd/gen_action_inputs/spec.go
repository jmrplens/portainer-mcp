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

// domainOperations returns every operation belonging to domainName, per
// domainTags's curated mapping (see internal/toolutil.DomainTags),
// aggregated across every tag domainName covers and re-sorted by
// OperationID so the result is deterministic regardless of map iteration
// order.
//
// A domain directory absent from domainTags is a hard error, never a
// silent empty result: before this, a directory name that did not literally
// match an OpenAPI tag (12 of the vendored spec's 46 tags name a directory
// differently than their operations' own first path segment would suggest —
// see toolutil.DomainTags's doc comment) produced zero operations and this
// generator moved on to the next domain having written nothing, with no
// error anywhere and nothing for CI's freshness check to catch, because
// there was no file to be stale.
func domainOperations(domainName string, domainTags map[string][]string, byTag map[string][]operation) ([]operation, error) {
	tags, ok := domainTags[domainName]
	if !ok {
		return nil, fmt.Errorf("domain directory %q has no entry in toolutil.DomainTags; add one (naming the OpenAPI tag(s) it covers) before generating for it", domainName)
	}
	var ops []operation
	for _, tag := range tags {
		ops = append(ops, byTag[tag]...)
	}
	sort.Slice(ops, func(i, j int) bool { return ops[i].OperationID < ops[j].OperationID })
	return ops, nil
}

// checkDomainTagsCoverSpec cross-checks domainTags against byTag (every
// operation the vendored spec actually declares, grouped by its own first
// tag) in both directions:
//
//   - every tag domainTags names must have at least one operation in the
//     spec, catching a typo in the table itself — the same silent-empty-
//     generation defect domainOperations guards against, reintroduced one
//     level up;
//   - every tag with at least one operation in the spec must be named by
//     some domain in domainTags, catching a tag the table has simply never
//     been told about, which is how 127 operations across 12 tags were
//     found silently unreachable before this table existed.
//
// This runs once, independent of which domain directories currently exist
// under -tools-dir: it is a property of the table against the vendored spec,
// not of how much of P3's domain wave has been written yet.
func checkDomainTagsCoverSpec(domainTags map[string][]string, byTag map[string][]operation) error {
	// Domain names are iterated in sorted order, not map order: two domains
	// racing to claim the same tag would otherwise name each other in a
	// nondeterministic order across runs, which is exactly the kind of
	// flakiness this audit's own output must not have.
	sortedDomains := make([]string, 0, len(domainTags))
	for domain := range domainTags {
		sortedDomains = append(sortedDomains, domain)
	}
	sort.Strings(sortedDomains)

	coveredBy := map[string]string{}
	for _, domain := range sortedDomains {
		for _, tag := range domainTags[domain] {
			if other, dup := coveredBy[tag]; dup {
				// A tag claimed by two domains passes both single-direction
				// checks below silently: the forward check sees the tag has
				// operations, and the reverse check sees the tag is present in
				// coveredBy. Left undetected, domainOperations would return the
				// same operations for both domains, generating two Input
				// structs — and two catalog actions — for one operation.
				return fmt.Errorf("tag %q is claimed by both domain %q and domain %q; each tag must belong to exactly one domain or its operations generate twice", tag, other, domain)
			}
			coveredBy[tag] = domain
			if len(byTag[tag]) == 0 {
				return fmt.Errorf("domain %q names tag %q, which has no operations in the vendored spec (check for a typo)", domain, tag)
			}
		}
	}

	var missing []string
	for tag, ops := range byTag {
		if len(ops) == 0 {
			continue
		}
		if _, ok := coveredBy[tag]; !ok {
			missing = append(missing, fmt.Sprintf("%s (%d operation(s))", tag, len(ops)))
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("tag(s) with operations but no domain in toolutil.DomainTags: %s", strings.Join(missing, ", "))
	}
	return nil
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
	for kind, v := range content {
		entry, _ := v.(map[string]any)
		schema, _ := entry["schema"].(map[string]any)
		if schema == nil {
			// A content entry with no "schema" key must not be confused with
			// "the operation has no body" (the rb == nil / len(content) == 0
			// cases above): assembleOperationFields cannot tell those apart,
			// and would silently generate an Input struct with no body fields
			// for an operation that does declare one.
			return nil, fmt.Errorf("requestBody content %q declares no schema; this generator refuses to emit a body-less Input for an operation that has a body", kind)
		}
		return schema, nil
	}
	return nil, nil
}
