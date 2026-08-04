package main

import (
	"net/http"
	"testing"
)

const twoOpDeltaSpec = `{
  "paths": {
    "/tags": {
      "get": {
        "operationId": "tagList",
        "tags": ["tags"],
        "parameters": [
          {"name": "limit", "in": "query", "required": false, "schema": {"type": "integer"}, "description": "Max results"}
        ]
      }
    },
    "/tags/{id}": {
      "delete": {
        "operationId": "tagDelete",
        "tags": ["tags"],
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "Tag identifier"}
        ]
      }
    }
  }
}`

// TestUnit_ParseSpecOperations_DecodesEveryOperation is the ordinary case:
// every operation is decoded, keyed by its raw (not PascalCased)
// operationId, carrying its own raw tag and route.
func TestUnit_ParseSpecOperations_DecodesEveryOperation(t *testing.T) {
	t.Parallel()
	ops, err := parseSpecOperations([]byte(twoOpDeltaSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("parseSpecOperations() = %d operations, want 2", len(ops))
	}

	list, ok := ops["tagList"]
	if !ok {
		t.Fatal(`parseSpecOperations() has no "tagList" entry`)
	}
	if list.Tag != "tags" {
		t.Errorf("tagList: Tag = %q, want %q", list.Tag, "tags")
	}
	if list.Op.Method != http.MethodGet || list.Op.Path != "/tags" {
		t.Errorf("tagList: Method=%q Path=%q, want GET /tags", list.Op.Method, list.Op.Path)
	}

	del, ok := ops["tagDelete"]
	if !ok {
		t.Fatal(`parseSpecOperations() has no "tagDelete" entry`)
	}
	if del.Op.Method != http.MethodDelete || del.Op.Path != "/tags/{id}" {
		t.Errorf("tagDelete: Method=%q Path=%q, want DELETE /tags/{id}", del.Op.Method, del.Op.Path)
	}
}

// TestUnit_ParseSpecOperations_KeepsRawOperationIDCase proves this function
// does NOT PascalCase the operationId the way cmd/audit_spec_drift's
// identically-named function does: there is no generated Go identifier on
// either side of a spec-to-spec comparison to match against, so the raw
// spelling ("tagList", not "TagList") is what both sides must key by.
func TestUnit_ParseSpecOperations_KeepsRawOperationIDCase(t *testing.T) {
	t.Parallel()
	ops, err := parseSpecOperations([]byte(twoOpDeltaSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if _, ok := ops["TagList"]; ok {
		t.Error(`parseSpecOperations() has a "TagList" entry, want it to key by the raw operationId "tagList" only`)
	}
}

// TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID mirrors both
// vendored specs' own handful of webhook/websocket routes that carry no
// operationId at all: those can never be tracked across a version boundary
// by identity.
func TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID(t *testing.T) {
	t.Parallel()
	spec := `{"paths": {"/webhook": {"post": {"tags": ["webhooks"]}}}}`
	ops, err := parseSpecOperations([]byte(spec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("parseSpecOperations() = %v, want no entries for an operation with no operationId", ops)
	}
}

// TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError proves a
// spec defect (the same operationId declared twice) is refused rather than
// silently resolved by map iteration order.
func TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	spec := `{
	  "paths": {
	    "/a": {"get": {"operationId": "dup", "tags": ["x"]}},
	    "/b": {"get": {"operationId": "dup", "tags": ["x"]}}
	  }
	}`
	if _, err := parseSpecOperations([]byte(spec)); err == nil {
		t.Fatal("parseSpecOperations() error = nil, want an error for a duplicate operationId")
	}
}

// TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError is the plumbing
// failure mode.
func TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := parseSpecOperations([]byte("not json")); err == nil {
		t.Fatal("parseSpecOperations() error = nil, want an error for malformed JSON")
	}
}
