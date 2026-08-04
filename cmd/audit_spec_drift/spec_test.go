package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestUnit_ExportedName_UppercasesFirstRuneOnly(t *testing.T) {
	t.Parallel()
	if got := exportedName("tagDelete"); got != "TagDelete" {
		t.Errorf("exportedName(%q) = %q, want %q", "tagDelete", got, "TagDelete")
	}
	if got := exportedName(""); got != "" {
		t.Errorf("exportedName(\"\") = %q, want empty", got)
	}
}

const twoOpSpec = `{
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
        "deprecated": true,
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "Tag identifier"}
        ]
      }
    }
  }
}`

// TestUnit_ParseSpecOperations_DecodesEveryOperation is the ordinary case:
// every GET/DELETE-style operation is decoded, keyed by its exported
// operationId, carrying its own domain and deprecated flag.
func TestUnit_ParseSpecOperations_DecodesEveryOperation(t *testing.T) {
	t.Parallel()
	ops, err := parseSpecOperations([]byte(twoOpSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 2 {
		t.Fatalf("parseSpecOperations() = %d operations, want 2", len(ops))
	}

	list, ok := ops["TagList"]
	if !ok {
		t.Fatal(`parseSpecOperations() has no "TagList" entry`)
	}
	if list.Domain != "tags" || list.Deprecated {
		t.Errorf("TagList: Domain=%q Deprecated=%v, want %q false", list.Domain, list.Deprecated, "tags")
	}
	if list.Op.Method != http.MethodGet || list.Op.Path != "/tags" {
		t.Errorf("TagList: Method=%q Path=%q, want GET /tags", list.Op.Method, list.Op.Path)
	}

	del, ok := ops["TagDelete"]
	if !ok {
		t.Fatal(`parseSpecOperations() has no "TagDelete" entry`)
	}
	if !del.Deprecated {
		t.Error("TagDelete: Deprecated = false, want true (the fixture marks it deprecated)")
	}
}

// TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID mirrors both
// vendored specs' own handful of webhook/websocket routes that carry no
// operationId at all: those can never become a catalog action, so there is
// nothing for this audit to compare their shape against.
func TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID(t *testing.T) {
	t.Parallel()
	const spec = `{"paths": {"/webhooks/{id}": {"post": {"tags": ["webhooks"]}}}}`
	ops, err := parseSpecOperations([]byte(spec))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("parseSpecOperations() = %v, want no operations for a path with no operationId", ops)
	}
}

// TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError proves a
// spec defect (the same operationId declared for two routes) is refused
// rather than one silently shadowing the other, which would make this
// audit's report depend on Go's randomised map iteration order.
func TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	const spec = `{"paths": {
		"/a": {"get": {"operationId": "dup", "tags": ["x"]}},
		"/b": {"get": {"operationId": "dup", "tags": ["x"]}}
	}}`
	_, err := parseSpecOperations([]byte(spec))
	if err == nil {
		t.Fatal("parseSpecOperations() = nil error, want an error for a duplicate operationId")
	}
	if !strings.Contains(err.Error(), "dup") {
		t.Errorf("parseSpecOperations() error = %v, want it to name the duplicate operationId", err)
	}
}

// TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError is the plumbing
// failure mode.
func TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := parseSpecOperations([]byte("not json")); err == nil {
		t.Fatal("parseSpecOperations() = nil error, want an error for malformed JSON")
	}
}

// TestUnit_ResolveSpecOperation_PrefersEEOverCE proves the precedence
// matches cmd/gen_action_inputs's own source of truth (its -spec flag
// defaults to the Business Edition document): an operation declared in both
// must be compared against the EE document, since that is what actually
// generated its Input struct's fields.
func TestUnit_ResolveSpecOperation_PrefersEEOverCE(t *testing.T) {
	t.Parallel()
	eeOps, err := parseSpecOperations([]byte(twoOpSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations(ee) error = %v", err)
	}
	// A CE document declaring the same operationId with a visibly different
	// path — if resolveSpecOperation ever preferred CE, this test would catch
	// it by the returned Path not matching the EE fixture's "/tags".
	const ceSpec = `{"paths": {"/ce-only-path": {"get": {"operationId": "tagList", "tags": ["tags"]}}}}`
	ceOps, err := parseSpecOperations([]byte(ceSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations(ce) error = %v", err)
	}

	op, edition, ok := resolveSpecOperation("TagList", eeOps, ceOps)
	if !ok {
		t.Fatal("resolveSpecOperation() ok = false, want true: TagList exists in both")
	}
	if edition != "EE" {
		t.Errorf("resolveSpecOperation() edition = %q, want %q", edition, "EE")
	}
	if op.Op.Path != "/tags" {
		t.Errorf("resolveSpecOperation() Path = %q, want the EE document's %q, not the CE one", op.Op.Path, "/tags")
	}
}

// TestUnit_ResolveSpecOperation_FallsBackToCE covers the real case this
// fallback exists for: SystemUpgrade, declared only in the Community
// Edition document.
func TestUnit_ResolveSpecOperation_FallsBackToCE(t *testing.T) {
	t.Parallel()
	eeOps, err := parseSpecOperations([]byte(`{"paths": {}}`))
	if err != nil {
		t.Fatalf("parseSpecOperations(ee) error = %v", err)
	}
	ceOps, err := parseSpecOperations([]byte(twoOpSpec))
	if err != nil {
		t.Fatalf("parseSpecOperations(ce) error = %v", err)
	}

	op, edition, ok := resolveSpecOperation("TagDelete", eeOps, ceOps)
	if !ok {
		t.Fatal("resolveSpecOperation() ok = false, want true: TagDelete exists in ce")
	}
	if edition != "CE" {
		t.Errorf("resolveSpecOperation() edition = %q, want %q", edition, "CE")
	}
	if op.Op.OperationID != "TagDelete" {
		t.Errorf("resolveSpecOperation() OperationID = %q, want %q", op.Op.OperationID, "TagDelete")
	}
}

// TestUnit_ResolveSpecOperation_NotFound_ReturnsFalse is the negative case:
// an operationId in neither document is reported as such, not as a zero
// value the caller might mistake for a match.
func TestUnit_ResolveSpecOperation_NotFound_ReturnsFalse(t *testing.T) {
	t.Parallel()
	empty := map[string]specOperation{}
	if _, _, ok := resolveSpecOperation("NoSuchOperation", empty, empty); ok {
		t.Error("resolveSpecOperation() ok = true, want false for an operationId in neither map")
	}
}
