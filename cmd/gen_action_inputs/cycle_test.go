package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This file covers I5: the vendored Business Edition document contains a
// genuine $ref cycle, and before these changes it took the whole generator
// down with it.
//
// portaineree.EdgeConfig declares "prev" as a $ref back to
// portaineree.EdgeConfig. resolve() had no cycle detection, so the only thing
// that terminated the recursion was maxSchemaDepth — which meant
// EdgeConfigInspect and EdgeConfigList could not be resolved at all, and
// checkCredentialRedaction's error propagated straight out of run()'s domain
// loop. One defect in one operation therefore stopped all 46 domain
// directories from regenerating.
//
// Two independent properties are proven here, because fixing either alone
// still leaves the other broken: the cycle itself now resolves (so the real
// operations work), and an operation whose response schema cannot be resolved
// *for any other reason* no longer aborts unrelated domains.

// TestUnit_Resolve_SelfReferentialRefTruncatesRatherThanRecursing is the
// mechanism in isolation, on a schema small enough to read. Before cycle
// detection this returned "schema nesting exceeds depth 40"; raising that
// bound could never have helped, because the graph is infinite rather than
// deep.
func TestUnit_Resolve_SelfReferentialRefTruncatesRatherThanRecursing(t *testing.T) {
	t.Parallel()
	res := &resolver{doc: &document{schemas: map[string]any{
		"Node": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"prev": map[string]any{"$ref": "#/components/schemas/Node"},
			},
		},
	}}}

	node, err := res.resolve(map[string]any{"$ref": "#/components/schemas/Node"}, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v, want a truncated node for a self-referential $ref", err)
	}

	var prev *schemaNode
	for _, p := range node.Properties {
		if p.Name == "prev" {
			prev = p.Schema
		}
	}
	if prev == nil {
		t.Fatal(`property "prev" not resolved at all`)
	}
	if prev.TruncatedRef != "#/components/schemas/Node" {
		t.Errorf("prev.TruncatedRef = %q, want the cycling ref: a cut edge must be marked, not silently returned as an empty object", prev.TruncatedRef)
	}
	if len(prev.Properties) != 0 {
		t.Errorf("prev has %d properties, want 0: a truncated node carries none by construction", len(prev.Properties))
	}

	// The resolving stack must unwind completely, or the second resolve of the
	// same document would truncate at the top level and return nothing.
	if len(res.resolving) != 0 {
		t.Errorf("resolver.resolving = %v after resolve() returned, want empty: the stack must unwind", res.resolving)
	}
	again, err := res.resolve(map[string]any{"$ref": "#/components/schemas/Node"}, 0)
	if err != nil {
		t.Fatalf("second resolve() error = %v", err)
	}
	if len(again.Properties) != 2 {
		t.Errorf("second resolve() returned %d properties, want 2: resolution must not be poisoned by the first call", len(again.Properties))
	}
}

// TestUnit_ResponseCredentialFields_EdgeConfigResolvesDespiteItsSelfReference
// is the same property against the two real operations that actually hit it.
// A synthetic cycle proves the mechanism; only the real document proves the
// mechanism matches the shape the real document has.
func TestUnit_ResponseCredentialFields_EdgeConfigResolvesDespiteItsSelfReference(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{"EdgeConfigInspect", "EdgeConfigList"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, _, res := realOperation(t, operationID)
			if _, err := responseCredentialFields(op, res); err != nil {
				t.Errorf("responseCredentialFields(%s) error = %v, want nil: portaineree.EdgeConfig's \"prev\" self-reference must truncate, not exhaust the depth bound", operationID, err)
			}
		})
	}
}

// TestUnit_Resolve_AllOfBranchThatIsARecycledRefKeepsItsTruncatedRefMark
// proves mergeSchema does not drop a cut-edge marker on the way through an
// allOf: an operation whose response schema composes an already-open $ref
// via allOf (rather than referencing it directly, the shape
// TestUnit_Resolve_SelfReferentialRefTruncatesRatherThanRecursing covers)
// must still come out marked as truncated, not silently as an empty object
// indistinguishable from a real one.
func TestUnit_Resolve_AllOfBranchThatIsARecycledRefKeepsItsTruncatedRefMark(t *testing.T) {
	t.Parallel()
	res := &resolver{doc: &document{schemas: map[string]any{
		"Node": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"prev": map[string]any{
					"allOf": []any{
						map[string]any{"$ref": "#/components/schemas/Node"},
					},
				},
			},
		},
	}}}

	node, err := res.resolve(map[string]any{"$ref": "#/components/schemas/Node"}, 0)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}

	var prev *schemaNode
	for _, p := range node.Properties {
		if p.Name == "prev" {
			prev = p.Schema
		}
	}
	if prev == nil {
		t.Fatal(`property "prev" not resolved at all`)
	}
	if prev.TruncatedRef != "#/components/schemas/Node" {
		t.Errorf("prev.TruncatedRef = %q, want the cycling ref preserved through the allOf merge, not dropped as an empty object", prev.TruncatedRef)
	}
}

// TestUnit_TypeOf_RefusesACyclicSchema is the other half of the truncation
// contract. Truncating is sound for a response walk (a cycle repeats the same
// property names, so the set of distinct names is unchanged) but never for
// building a Go type, where the cut edge has no type to be given. Emitting the
// empty struct a truncated node would otherwise produce would silently drop
// every field past the cycle.
func TestUnit_TypeOf_RefusesACyclicSchema(t *testing.T) {
	t.Parallel()
	var structs []structSpec
	_, err := typeOf(&schemaNode{Type: "object", TruncatedRef: "#/components/schemas/Node"}, true, "probe", &structs)
	if err == nil {
		t.Fatal("typeOf() = nil error, want a refusal: a cyclic schema has no request-shaped Go type")
	}
	if !strings.Contains(err.Error(), "#/components/schemas/Node") {
		t.Errorf("error = %q, want it to name the cycling ref", err)
	}
}

// poisonedSpecDir copies the real vendored specs into a temporary directory,
// replacing the 200-response schema of operationID with one resolve() refuses
// (a oneOf, which composes into no single Go type). It returns the path of the
// EE fixture.
//
// Built by mutating the real document rather than hand-writing a small one:
// run() reflects every operation against the real generated client
// (pathArgTypesFor, responseInfoFor), so an invented operationId would be
// rejected long before reaching the code under test, and the test would prove
// nothing about the path it claims to cover.
func poisonedSpecDir(t *testing.T, operationID string) string {
	t.Helper()
	dir := t.TempDir()

	for _, edition := range []string{"ee", "ce"} {
		raw, err := os.ReadFile(filepath.Join("../../api/specs", edition+"-2.44.0.json"))
		if err != nil {
			t.Fatalf("read %s spec: %v", edition, err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatalf("decode %s spec: %v", edition, err)
		}

		if edition == "ee" {
			paths, _ := doc["paths"].(map[string]any)
			poisoned := false
			for _, item := range paths {
				methods, _ := item.(map[string]any)
				for method, opRaw := range methods {
					if !httpMethods[strings.ToLower(method)] {
						continue
					}
					op, _ := opRaw.(map[string]any)
					id, _ := op["operationId"].(string)
					if exportedName(id) != operationID {
						continue
					}
					op["responses"] = map[string]any{
						"200": map[string]any{
							"description": "Success",
							"content": map[string]any{
								"application/json": map[string]any{
									"schema": map[string]any{
										"oneOf": []any{
											map[string]any{"type": "string"},
											map[string]any{"type": "integer"},
										},
									},
								},
							},
						},
					}
					poisoned = true
				}
			}
			if !poisoned {
				t.Fatalf("operationId %q not found in the real EE spec; this fixture would prove nothing", operationID)
			}
		}

		out, err := json.Marshal(doc)
		if err != nil {
			t.Fatalf("encode %s fixture: %v", edition, err)
		}
		if err := os.WriteFile(filepath.Join(dir, edition+"-poisoned.json"), out, 0o600); err != nil {
			t.Fatalf("write %s fixture: %v", edition, err)
		}
	}
	return filepath.Join(dir, "ee-poisoned.json")
}

// copyDomainDir copies one real domain package's hand-written files into a
// temporary tools directory, so run() sees the same overrides and helper
// functions it would in the real tree — without writing generated output over
// the real one.
func copyDomainDir(t *testing.T, toolsDir, domain string) {
	t.Helper()
	dst := filepath.Join(toolsDir, domain)
	if err := os.MkdirAll(dst, 0o750); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	src := filepath.Join("../../internal/tools", domain)
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read %s: %v", src, err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), ".gen.go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dst, e.Name()), data, 0o600); err != nil {
			t.Fatalf("write %s: %v", e.Name(), err)
		}
	}
}

// TestUnit_Run_AnUnresolvableResponseSchemaLeavesOtherDomainsGenerated is I5's
// acceptance test, and the invariant this fix exists to establish: a response
// schema that cannot be resolved must cost the domain that contains it, and
// nothing else.
//
// Before the fix, checkCredentialRedaction's error was returned straight out
// of the domain loop, so the first unresolvable operation stopped every
// domain after it — and, because the loop runs in sorted order, which domains
// survived was decided by alphabetical accident. The run must still fail
// (an operation nobody can prove is credential-free never gets a handler),
// but it must fail having regenerated everything it legitimately could.
// Both orderings are exercised deliberately. run() walks domains in sorted
// order, so poisoning only "tags" would leave "registries" written even under
// the old abort-everything behaviour — the survivor would be an alphabetical
// accident rather than evidence of anything. Poisoning "registries" is the
// case that actually discriminates, and the pair together show the outcome
// does not depend on which name sorts first.
func TestUnit_Run_AnUnresolvableResponseSchemaLeavesOtherDomainsGenerated(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		operationID   string
		poisonDomain  string
		anotherDomain string
	}{
		{"poisoned domain sorts last", "TagCreate", "tags", "registries"},
		{"poisoned domain sorts first", "RegistryPing", "registries", "tags"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			specPath := poisonedSpecDir(t, tc.operationID)
			toolsDir := t.TempDir()
			copyDomainDir(t, toolsDir, "tags")
			copyDomainDir(t, toolsDir, "registries")

			err := run([]string{"-spec", specPath, "-tools-dir", toolsDir})
			if err == nil {
				t.Fatalf("run() = nil error, want a refusal: %s's response schema cannot be resolved", tc.operationID)
			}
			if !strings.Contains(err.Error(), "could not be checked for credential-shaped response fields") {
				t.Errorf("error = %q, want the deferred-refusal summary", err)
			}

			// The domain holding the unresolvable operation is left untouched...
			poisoned := filepath.Join(toolsDir, tc.poisonDomain, "actions.gen.go")
			if _, statErr := os.Stat(poisoned); !os.IsNotExist(statErr) {
				t.Errorf("%s/actions.gen.go exists (stat error %v), want it unwritten: nothing about a domain with an uncheckable operation can be trusted",
					tc.poisonDomain, statErr)
			}
			// ...and every other domain is generated exactly as it would have been.
			for _, name := range []string{"actions.gen.go", "inputs.gen.go"} {
				if _, statErr := os.Stat(filepath.Join(toolsDir, tc.anotherDomain, name)); statErr != nil {
					t.Errorf("%s/%s was not written (%v); one domain's defect must not block an unrelated domain's regeneration",
						tc.anotherDomain, name, statErr)
				}
			}
		})
	}
}

// TestUnit_ResponseCredentialFields_UnresolvableSchemaIsDistinguishable is what
// makes the invariant above enforceable rather than incidental: run() can only
// treat this failure specially if it can tell it apart from every other
// refusal, and errors.Is is how it does so.
func TestUnit_ResponseCredentialFields_UnresolvableSchemaIsDistinguishable(t *testing.T) {
	t.Parallel()
	op := operation{
		OperationID: "Synthetic",
		Responses: map[string]map[string]any{
			"200": {"content": map[string]any{"application/json": map[string]any{
				"schema": map[string]any{"oneOf": []any{map[string]any{"type": "string"}}},
			}}},
		},
	}
	res := &resolver{doc: &document{schemas: map[string]any{}}}

	_, err := responseCredentialFields(op, res)
	if err == nil {
		t.Fatal("responseCredentialFields() = nil error, want a refusal for a oneOf response schema")
	}
	if !errors.Is(err, errResponseSchemaUnresolvable) {
		t.Errorf("errors.Is(err, errResponseSchemaUnresolvable) = false for %v; run() would treat this as a fatal, run-aborting refusal", err)
	}

	// checkCredentialRedaction must not flatten the sentinel while wrapping.
	_, err = checkCredentialRedaction(op, res, map[string]bool{}, false)
	if !errors.Is(err, errResponseSchemaUnresolvable) {
		t.Errorf("errors.Is(checkCredentialRedaction(...), errResponseSchemaUnresolvable) = false for %v", err)
	}
}
