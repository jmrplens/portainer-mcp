package main

import (
	"net/http"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

func TestUnit_ExportedName_UppercasesFirstRuneOnly(t *testing.T) {
	t.Parallel()
	t.Run("ExportedName UppercasesFirstRuneOnly", func(t *testing.T) {
		if got := exportedName("tagDelete"); got != "TagDelete" {
			t.Errorf("exportedName(%q) = %q, want %q", "tagDelete", got, "TagDelete")
		}
		if got := exportedName(""); got != "" {
			t.Errorf("exportedName(\"\") = %q, want empty", got)
		}
	})
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
	t.Run("ParseSpecOperations DecodesEveryOperation", func(t *testing.T) {
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
	})
}

// TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID mirrors both
// vendored specs' own handful of webhook/websocket routes that carry no
// operationId at all AND no entry in internal/specnaming's table: nothing
// names those, so they can never become a catalog action and there is
// nothing for this audit to compare their shape against.
//
// POST /webhooks/{id} is a real such route and deliberately one
// internal/specnaming does not name, so this stays a test of the skip rather
// than becoming a second test of the synthetic-name lookup below.
func TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID(t *testing.T) {
	t.Parallel()
	t.Run("ParseSpecOperations SkipsOperationsWithNoOperationID", func(t *testing.T) {
		const spec = `{"paths": {"/webhooks/{id}": {"post": {"tags": ["webhooks"]}}}}`
		ops, err := parseSpecOperations([]byte(spec))
		if err != nil {
			t.Fatalf("parseSpecOperations() error = %v", err)
		}
		if len(ops) != 0 {
			t.Errorf("parseSpecOperations() = %v, want no operations for a path with no operationId", ops)
		}
	})
}

// unnamedInspectSpec is GET /endpoint_groups/{id} as both vendored documents
// actually declare it: fully described — summary, description, both
// parameters, a response schema — and carrying no operationId at all. It is
// the one route internal/specnaming's table names, and the reason
// parseSpecOperations consults that table instead of skipping.
const unnamedInspectSpec = `{
  "paths": {
    "/endpoint_groups/{id}": {
      "get": {
        "tags": ["endpoint_groups"],
        "summary": "Inspect an Environment(Endpoint) group",
        "description": "Retrieve details about an environment(endpoint) group.\n**Access policy**: administrator",
        "parameters": [
          {"name": "id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "Environment(Endpoint) group identifier"},
          {"name": "size", "in": "query", "schema": {"type": "boolean"}, "description": "If true, include the number of environments and breakdown by type"}
        ]
      }
    }
  }
}`

// TestUnit_ParseSpecOperations_UnnamedRouteTakesItsNameFromSpecnaming is the
// guard for the seam this audit refused at.
//
// GET /endpoint_groups/{id} carries no operationId in either vendored
// document, so parseSpecOperations used to skip it outright — and
// auditDrift then refused to run at all the moment the catalog declared
// endpoint_groups.inspect against internal/specnaming's name for it:
// `action "endpoint_groups.inspect": OperationID "EndpointGroupInspect"
// resolves in neither vendored spec`. The documents describe the operation
// completely apart from its name; only the key to find it by was missing.
//
// Delete the specnaming lookup in parseSpecOperations and this fails on the
// length check, before any field is examined.
func TestUnit_ParseSpecOperations_UnnamedRouteTakesItsNameFromSpecnaming(t *testing.T) {
	t.Parallel()
	t.Run("ParseSpecOperations UnnamedRouteTakesItsNameFromSpecnaming", func(t *testing.T) {
		want, named := specnaming.SyntheticOperationID(http.MethodGet, "/endpoint_groups/{id}")
		if !named {
			t.Fatal("internal/specnaming no longer names GET /endpoint_groups/{id}; this test and cmd/audit_spec_drift's lookup both assume it does")
		}

		ops, err := parseSpecOperations([]byte(unnamedInspectSpec))
		if err != nil {
			t.Fatalf("parseSpecOperations() error = %v", err)
		}
		if len(ops) != 1 {
			t.Fatalf("parseSpecOperations() = %d operation(s) (%v), want 1: the route carries no operationId but internal/specnaming names it", len(ops), ops)
		}

		op, ok := ops[want]
		if !ok {
			t.Fatalf("parseSpecOperations() has no %q entry; got %v", want, ops)
		}
		if op.Op.OperationID != want {
			t.Errorf("OperationID = %q, want %q: the synthetic name must be carried on the operation too, not only used as the map key", op.Op.OperationID, want)
		}
		if op.Op.Method != http.MethodGet || op.Op.Path != "/endpoint_groups/{id}" {
			t.Errorf("Method=%q Path=%q, want GET /endpoint_groups/{id}", op.Op.Method, op.Op.Path)
		}
		if op.Domain != "endpoint_groups" {
			t.Errorf("Domain = %q, want %q", op.Domain, "endpoint_groups")
		}
		// The whole point of resolving the name is that there is a real shape
		// behind it to compare against. An entry keyed correctly but carrying
		// no parameters would satisfy every check above and audit nothing.
		if len(op.Op.Parameters) != 2 {
			t.Errorf("Parameters = %d, want 2 (id and size): the audit compares this shape against the catalog's Input struct", len(op.Op.Parameters))
		}
		if op.Op.Summary != "Inspect an Environment(Endpoint) group" {
			t.Errorf("Summary = %q, want the document's own summary", op.Op.Summary)
		}
	})
}

// TestUnit_ParseSpecOperations_SyntheticNameCollidingWithAPublishedOne_ReturnsError
// is what keeps internal/specnaming's "verified collision-free" claim true
// against a future respec, on this side of the fence: if some later document
// starts publishing EndpointGroupInspect for a different route, the two must
// not silently shadow each other — whichever Go's randomised map iteration
// reached first would win, and this audit would then compare the catalog
// action against the wrong operation's shape.
//
// The message must say which side is which, or a reader sent to fix it
// removes the wrong one.
func TestUnit_ParseSpecOperations_SyntheticNameCollidingWithAPublishedOne_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("ParseSpecOperations SyntheticNameCollidingWithAPublishedOne ReturnsError", func(t *testing.T) {
		synthetic, named := specnaming.SyntheticOperationID(http.MethodGet, "/endpoint_groups/{id}")
		if !named {
			t.Fatal("internal/specnaming no longer names GET /endpoint_groups/{id}")
		}
		spec := `{"paths": {
			"/endpoint_groups/{id}": {"get": {"tags": ["endpoint_groups"]}},
			"/somewhere/else": {"get": {"operationId": "` + synthetic + `", "tags": ["x"]}}
		}}`

		_, err := parseSpecOperations([]byte(spec))
		if err == nil {
			t.Fatalf("parseSpecOperations() = nil error, want a refusal: %q is both published and synthetic here", synthetic)
		}
		if !strings.Contains(err.Error(), synthetic) {
			t.Errorf("parseSpecOperations() error = %v, want it to name %q", err, synthetic)
		}
		if !strings.Contains(err.Error(), "/endpoint_groups/{id}") || !strings.Contains(err.Error(), "/somewhere/else") {
			t.Errorf("parseSpecOperations() error = %v, want it to name both colliding routes", err)
		}
		if !strings.Contains(err.Error(), "specnaming") && !strings.Contains(err.Error(), "operationId \""+synthetic+"\"") {
			t.Errorf("parseSpecOperations() error = %v, want it to say where the second name came from", err)
		}
	})
}

// TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError proves a
// spec defect (the same operationId declared for two routes) is refused
// rather than one silently shadowing the other, which would make this
// audit's report depend on Go's randomised map iteration order.
func TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("ParseSpecOperations DuplicateOperationID ReturnsError", func(t *testing.T) {
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
	})
}

// TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError is the plumbing
// failure mode.
func TestUnit_ParseSpecOperations_MalformedJSON_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("ParseSpecOperations MalformedJSON ReturnsError", func(t *testing.T) {
		if _, err := parseSpecOperations([]byte("not json")); err == nil {
			t.Fatal("parseSpecOperations() = nil error, want an error for malformed JSON")
		}
	})
}

// TestUnit_ResolveSpecOperation_PrefersEEOverCE proves the precedence
// matches cmd/gen_action_inputs's own source of truth (its -spec flag
// defaults to the Business Edition document): an operation declared in both
// must be compared against the EE document, since that is what actually
// generated its Input struct's fields.
func TestUnit_ResolveSpecOperation_PrefersEEOverCE(t *testing.T) {
	t.Parallel()
	t.Run("ResolveSpecOperation PrefersEEOverCE", func(t *testing.T) {
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
	})
}

// TestUnit_ResolveSpecOperation_FallsBackToCE covers the real case this
// fallback exists for: SystemUpgrade, declared only in the Community
// Edition document.
func TestUnit_ResolveSpecOperation_FallsBackToCE(t *testing.T) {
	t.Parallel()
	t.Run("ResolveSpecOperation FallsBackToCE", func(t *testing.T) {
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
	})
}

// TestUnit_ResolveSpecOperation_NotFound_ReturnsFalse is the negative case:
// an operationId in neither document is reported as such, not as a zero
// value the caller might mistake for a match.
func TestUnit_ResolveSpecOperation_NotFound_ReturnsFalse(t *testing.T) {
	t.Parallel()
	t.Run("ResolveSpecOperation NotFound ReturnsFalse", func(t *testing.T) {
		empty := map[string]specOperation{}
		if _, _, ok := resolveSpecOperation("NoSuchOperation", empty, empty); ok {
			t.Error("resolveSpecOperation() ok = true, want false for an operationId in neither map")
		}
	})
}
