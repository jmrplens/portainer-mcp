package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// fakeFetcher serves a two-file spec: a root that $refs into stacks.yaml, and
// stacks.yaml carrying both the path item and the schema it references.
func fakeFetcher(t *testing.T) fetcher {
	t.Helper()
	files := map[string]string{
		"ee-versions.json": `[{"id":"9.9.9","file":"versions/ee/9.9.9/openapi.yaml","name":"9.9.9"}]`,
		"versions/ee/9.9.9/openapi.yaml": `
openapi: 3.0.0
info: {title: t, version: 9.9.9}
paths:
  /stacks:
    $ref: stacks.yaml#/paths/~1stacks
components:
  securitySchemes:
    apiKey: {type: apiKey, name: X-API-KEY, in: header}
`,
		"versions/ee/9.9.9/stacks.yaml": `
paths:
  /stacks:
    get:
      operationId: StackList
      responses:
        "200":
          description: ok
          content:
            application/json:
              schema: {$ref: '#/components/schemas/Stack'}
components:
  schemas:
    Stack:
      type: object
      properties:
        Id: {type: integer}
`,
	}
	return func(rel string) ([]byte, error) {
		body, ok := files[rel]
		if !ok {
			return nil, fmt.Errorf("unexpected fetch: %s", rel)
		}
		return []byte(body), nil
	}
}

func TestBundle_SplitSpec_MergesPathsAndComponents(t *testing.T) {
	t.Parallel()
	spec, provenance, conflicts, err := bundle(fakeFetcher(t), "ee", "9.9.9")
	if err != nil {
		t.Fatalf("bundle() error = %v", err)
	}
	if len(conflicts) != 0 {
		t.Errorf("conflicts = %v, want none", conflicts)
	}

	paths, _ := spec["paths"].(map[string]any)
	if _, ok := paths["/stacks"]; !ok {
		t.Fatalf("paths = %v, want /stacks inlined", paths)
	}
	if got := provenance["/stacks"]; got != "stacks.yaml" {
		t.Errorf("provenance[/stacks] = %q, want %q", got, "stacks.yaml")
	}

	// The regression this test exists for: an earlier bundler inlined paths
	// but not components, leaving every internal $ref dangling.
	components, _ := spec["components"].(map[string]any)
	schemas, _ := components["schemas"].(map[string]any)
	if _, ok := schemas["Stack"]; !ok {
		t.Errorf("components.schemas = %v, want Stack merged from the sibling file", schemas)
	}
	if _, ok := components["securitySchemes"]; !ok {
		t.Error("components.securitySchemes from the root document was lost during the merge")
	}
}

func TestBundle_ConflictingComponent_IsReported(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"ee-versions.json": `[{"id":"9.9.9","file":"versions/ee/9.9.9/openapi.yaml","name":"9.9.9"}]`,
		"versions/ee/9.9.9/openapi.yaml": `
openapi: 3.0.0
info: {title: t, version: 9.9.9}
paths:
  /a: {$ref: 'a.yaml#/paths/~1a'}
  /b: {$ref: 'b.yaml#/paths/~1b'}
`,
		"versions/ee/9.9.9/a.yaml": `
paths: {/a: {get: {operationId: A, responses: {"200": {description: ok}}}}}
components: {schemas: {Shared: {type: object, properties: {x: {type: string}}}}}
`,
		"versions/ee/9.9.9/b.yaml": `
paths: {/b: {get: {operationId: B, responses: {"200": {description: ok}}}}}
components: {schemas: {Shared: {type: integer}}}
`,
	}
	fetch := func(rel string) ([]byte, error) {
		body, ok := files[rel]
		if !ok {
			return nil, fmt.Errorf("unexpected fetch: %s", rel)
		}
		return []byte(body), nil
	}

	_, _, conflicts, err := bundle(fetch, "ee", "9.9.9")
	if err != nil {
		t.Fatalf("bundle() error = %v", err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflicts = %v, want exactly one for schemas/Shared", conflicts)
	}
}

func TestBundle_SingleFileSpec_ReturnedUnchanged(t *testing.T) {
	t.Parallel()
	files := map[string]string{
		"ce-versions.json": `[{"id":"2.40.0","file":"versions/ce/2.40.0.yaml","name":"2.40.0"}]`,
		"versions/ce/2.40.0.yaml": `
openapi: 3.0.0
info: {title: t, version: 2.40.0}
paths: {/status: {get: {operationId: Status, responses: {"200": {description: ok}}}}}
`,
	}
	fetch := func(rel string) ([]byte, error) { return []byte(files[rel]), nil }

	spec, provenance, _, err := bundle(fetch, "ce", "2.40.0")
	if err != nil {
		t.Fatalf("bundle() error = %v", err)
	}
	if len(provenance) != 0 {
		t.Errorf("provenance = %v, want empty for a single-file spec", provenance)
	}
	paths, _ := spec["paths"].(map[string]any)
	if len(paths) != 1 {
		t.Errorf("paths = %v, want the single path preserved", paths)
	}
	_ = json.Marshal // keep the import honest if the test grows
}
