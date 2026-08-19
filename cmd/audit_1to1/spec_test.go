package main

import (
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specnaming"
)

func TestUnit_ParseSpecOperations_ValidSpec_ExtractsOperations(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"paths": {
			"/tags": {
				"get": {"operationId": "tagList", "tags": ["tags"]},
				"post": {"operationId": "tagCreate", "tags": ["tags"]}
			},
			"/tags/{id}": {
				"delete": {"operationId": "tagDelete", "tags": ["tags"], "deprecated": true}
			}
		}
	}`)

	doc, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	ops := doc.Operations
	if len(ops) != 3 {
		t.Fatalf("parseSpecOperations() = %v, want 3 operations", ops)
	}

	list, ok := ops["TagList"]
	if !ok {
		t.Fatalf("parseSpecOperations() missing TagList: %v", ops)
	}
	if list.Method != http.MethodGet || list.Path != "/tags" || list.Domain != "tags" || list.Deprecated {
		t.Errorf("parseSpecOperations()[TagList] = %+v, unexpected fields", list)
	}

	del, ok := ops["TagDelete"]
	if !ok || !del.Deprecated {
		t.Errorf("parseSpecOperations()[TagDelete] = %+v, want Deprecated=true", del)
	}
}

// TestUnit_ParseSpecOperations_NoOperationIDAndNoTableEntry_IsReportedNotDropped
// replaces a test that asserted only "it is skipped". That assertion is still
// true of the operation map, and it was the whole of what the old test
// checked — which is exactly why it stayed green through the defect this
// change fixes: a route nothing names was skipped, and a test named
// "SkipsOperationsWithNoOperationID" said so approvingly.
//
// What must hold now is both halves at once. GET /websocket/exec has no
// operationId in either document and no entry in internal/specnaming's
// table, so it still cannot be counted — there is no key to count it
// against. But it must come back in Unnamed, because the report is what
// makes it visible, and an assertion on the map alone cannot tell "not
// counted" from "not there".
func TestUnit_ParseSpecOperations_NoOperationIDAndNoTableEntry_IsReportedNotDropped(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"paths": {
			"/websocket/exec": {
				"get": {"tags": ["websocket"]}
			},
			"/tags": {
				"get": {"operationId": "tagList", "tags": ["tags"]}
			}
		}
	}`)

	doc, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(doc.Operations) != 1 {
		t.Fatalf("parseSpecOperations().Operations = %v, want exactly the one operation with an operationId", doc.Operations)
	}
	if _, ok := doc.Operations["TagList"]; !ok {
		t.Errorf("parseSpecOperations().Operations = %v, want TagList", doc.Operations)
	}
	want := []unnamedOperation{{Method: http.MethodGet, Path: "/websocket/exec", Domain: "websocket"}}
	if !reflect.DeepEqual(doc.Unnamed, want) {
		t.Errorf("parseSpecOperations().Unnamed = %+v, want %+v: a route nothing names must be reported, not dropped", doc.Unnamed, want)
	}
}

// TestUnit_ParseSpecOperations_NoOperationIDButTableNamesIt_IsCounted is the
// other half: a route no document names, that internal/specnaming does name,
// enters the operation map under that name and therefore enters the
// denominator. Nothing is left in Unnamed for it, because it is no longer
// nameless.
func TestUnit_ParseSpecOperations_NoOperationIDButTableNamesIt_IsCounted(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"paths": {
			"/endpoint_groups/{id}": {
				"get": {"tags": ["endpoint_groups"]}
			}
		}
	}`)

	doc, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	got, ok := doc.Operations["EndpointGroupInspect"]
	if !ok {
		t.Fatalf("parseSpecOperations().Operations = %v, want EndpointGroupInspect from internal/specnaming's table", doc.Operations)
	}
	if got.Method != http.MethodGet || got.Path != "/endpoint_groups/{id}" || got.Domain != "endpoint_groups" {
		t.Errorf("parseSpecOperations().Operations[EndpointGroupInspect] = %+v, want the GET route it was named for, tag included", got)
	}
	if len(doc.Unnamed) != 0 {
		t.Errorf("parseSpecOperations().Unnamed = %+v, want empty: this route is named now", doc.Unnamed)
	}
}

// TestUnit_ParseSpecOperations_SyntheticNameCollidesWithADocumentedOne_IsError
// is what keeps internal/specnaming's "verified collision-free" claim true
// against a future respec rather than only on the day it was written. If a
// published document ever names some other route EndpointGroupInspect, the
// audit must refuse rather than keep whichever of the two it happened to
// reach second — silently dropping one operation is the failure mode this
// whole task exists to end.
func TestUnit_ParseSpecOperations_SyntheticNameCollidesWithADocumentedOne_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"paths": {
			"/endpoint_groups/{id}": {"get": {"tags": ["endpoint_groups"]}},
			"/endpoint_groups/{id}/details": {"get": {"operationId": "endpointGroupInspect", "tags": ["endpoint_groups"]}}
		}
	}`)

	// Run repeatedly: which of the two routes the parser meets first is Go's
	// map iteration order over the document's path items, which differs per
	// run. The message must be byte-identical every time, and must attribute
	// each name to the route it actually came from — an order-dependent
	// message would tell a reader to remove the published operationId rather
	// than the table entry, or the reverse. Asserting only that the id
	// appears somewhere in the text passes for both orderings and for a
	// message that credits the wrong route.
	const want = `two routes resolve to the exported operationId "EndpointGroupInspect": ` +
		`GET /endpoint_groups/{id} (named by internal/specnaming's table, the document leaving it unnamed) and ` +
		`GET /endpoint_groups/{id}/details (operationId "endpointGroupInspect" in the document)`
	for i := 0; i < 50; i++ {
		_, err := parseSpecOperations(data)
		if err == nil {
			t.Fatal("parseSpecOperations() = nil error, want a refusal for a synthetic name a document now uses")
		}
		if err.Error() != want {
			t.Fatalf("parseSpecOperations() error on run %d =\n  %v\nwant\n  %s", i, err, want)
		}
	}
}

// TestUnit_ParseSpecOperations_RealDocuments_AccountForEveryRoute is the
// property that makes recurrence impossible rather than merely unlikely: over
// both real vendored documents, every path-item verb must come back either as
// a counted operation or as a named unnamed route. Nothing may be silently
// dropped.
//
// Counted against a second, independent traversal of the same JSON, so this
// cannot be satisfied by a parser that agrees with itself. The per-edition
// numbers are asserted too — 265 routes for Community, 442 for Business, of
// which 13 and 0 respectively remain nameless once the table has been
// consulted — because "the two totals match" would still hold if both halves
// lost the same operation.
func TestUnit_ParseSpecOperations_RealDocuments_AccountForEveryRoute(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		path        string
		wantRoutes  int
		wantUnnamed int
	}{
		{name: "Community Edition", path: realCESpec, wantRoutes: 265, wantUnnamed: 13},
		{name: "Business Edition", path: realEESpec, wantRoutes: 442, wantUnnamed: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(tc.path)
			if err != nil {
				t.Fatalf("read %s: %v", tc.path, err)
			}
			doc, err := parseSpecOperations(data)
			if err != nil {
				t.Fatalf("parseSpecOperations(%s) error = %v", tc.path, err)
			}

			var raw struct {
				Paths map[string]map[string]json.RawMessage `json:"paths"`
			}
			if err := json.Unmarshal(data, &raw); err != nil {
				t.Fatalf("decode %s: %v", tc.path, err)
			}
			routes := 0
			for _, item := range raw.Paths {
				for method := range item {
					if httpMethods[strings.ToLower(method)] {
						routes++
					}
				}
			}

			if routes != tc.wantRoutes {
				t.Errorf("%s declares %d routes, want %d; if the document was refreshed, re-measure the numbers below before changing them", tc.path, routes, tc.wantRoutes)
			}
			if got := len(doc.Operations) + len(doc.Unnamed); got != routes {
				t.Errorf("parseSpecOperations(%s) accounted for %d of %d routes (%d counted, %d unnamed); %d were dropped without trace",
					tc.path, got, routes, len(doc.Operations), len(doc.Unnamed), routes-got)
			}
			if len(doc.Unnamed) != tc.wantUnnamed {
				t.Errorf("parseSpecOperations(%s).Unnamed = %d routes, want %d", tc.path, len(doc.Unnamed), tc.wantUnnamed)
			}
			if _, ok := doc.Operations["EndpointGroupInspect"]; !ok {
				t.Errorf("parseSpecOperations(%s) does not name GET /endpoint_groups/{id}; both editions serve it and neither document names it, which is what internal/specnaming's table is for", tc.path)
			}
		})
	}
}

// TestUnit_SyntheticOperationIDs_EveryEntryIsStillNameless keeps
// internal/specnaming's table honest against the documents, the way
// cmd/gen_action_inputs's TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation
// keeps actionNameOverrides honest: an entry for a route Portainer has since
// named itself would have this project inventing a name over a published one,
// and an entry for a route no document declares at all is a judgement about
// nothing.
func TestUnit_SyntheticOperationIDs_EveryEntryIsStillNameless(t *testing.T) {
	t.Parallel()
	for _, entry := range specnaming.SyntheticOperationIDs() {
		t.Run(entry.Method+" "+entry.Path, func(t *testing.T) {
			t.Parallel()
			declaredSomewhere := false
			for _, path := range []string{realCESpec, realEESpec} {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatalf("read %s: %v", path, err)
				}
				var raw struct {
					Paths map[string]map[string]struct {
						OperationID string `json:"operationId"`
					} `json:"paths"`
				}
				if err := json.Unmarshal(data, &raw); err != nil {
					t.Fatalf("decode %s: %v", path, err)
				}
				for method, op := range raw.Paths[entry.Path] {
					if !strings.EqualFold(method, entry.Method) {
						continue
					}
					declaredSomewhere = true
					if op.OperationID != "" {
						t.Errorf("%s now names %s %s %q; remove the entry from internal/specnaming rather than inventing a name over a published one",
							path, entry.Method, entry.Path, op.OperationID)
					}
				}
			}
			if !declaredSomewhere {
				t.Errorf("no vendored document declares %s %s; this table entry names nothing", entry.Method, entry.Path)
			}
			// Checked here as well as in internal/specnaming's own test,
			// because this is the check that runs against the real
			// documents: an entry that has gone stale and an entry that
			// never said why it existed are the same problem to whoever has
			// to decide what to do about it.
			if strings.TrimSpace(entry.Reason) == "" {
				t.Errorf("the table entry for %s %s states no Reason; cmd/audit_1to1 refuses an allow-list entry without one, and this is the same kind of standing exception",
					entry.Method, entry.Path)
			}
		})
	}
}

func TestUnit_ParseSpecOperations_IgnoresNonVerbPathItemKeys(t *testing.T) {
	t.Parallel()
	data := []byte(`{
		"paths": {
			"/tags": {
				"parameters": [{"name": "id", "in": "query"}],
				"get": {"operationId": "tagList", "tags": ["tags"]}
			}
		}
	}`)

	doc, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	ops := doc.Operations
	if len(ops) != 1 {
		t.Fatalf("parseSpecOperations() = %v, want exactly one operation (the \"parameters\" key must not be read as a verb)", ops)
	}
}

func TestUnit_ParseSpecOperations_DuplicateExportedName_IsError(t *testing.T) {
	t.Parallel()
	// "tagList" and "TagList" both export to "TagList": a real spec should
	// never do this, but the parser must refuse rather than silently keep
	// only one of the two operations it names.
	data := []byte(`{
		"paths": {
			"/tags": {"get": {"operationId": "tagList", "tags": ["tags"]}},
			"/tags/other": {"get": {"operationId": "TagList", "tags": ["tags"]}}
		}
	}`)

	_, err := parseSpecOperations(data)
	if err == nil {
		t.Fatal("parseSpecOperations() = nil error, want error for colliding exported operationId")
	}
	if !strings.Contains(err.Error(), "TagList") {
		t.Errorf("parseSpecOperations() error = %v, want it to name the colliding id", err)
	}
}

func TestUnit_ParseSpecOperations_MalformedJSON_IsError(t *testing.T) {
	t.Parallel()
	_, err := parseSpecOperations([]byte(`{not json`))
	if err == nil {
		t.Fatal("parseSpecOperations() = nil error, want a decode error")
	}
}

func TestUnit_ExportedName_UppercasesFirstRuneOnly(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"":              "",
		"systemStatus":  "SystemStatus",
		"SystemStatus":  "SystemStatus",
		"tagList":       "TagList",
		"eCRDeleteTags": "ECRDeleteTags",
	}
	for in, want := range tests {
		if got := exportedName(in); got != want {
			t.Errorf("exportedName(%q) = %q, want %q", in, got, want)
		}
	}
}
