package main

import (
	"net/http"
	"testing"
)

// TestUnit_ParseSpecOperations_CapturesMethodPathDomain is the ordinary case.
func TestUnit_ParseSpecOperations_CapturesMethodPathDomain(t *testing.T) {
	t.Parallel()
	doc := `{"paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"]}}}}`
	ops, err := parseSpecOperations([]byte(doc))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	op, ok := ops["TagsList"]
	if !ok {
		t.Fatalf("parseSpecOperations() = %v, want a \"TagsList\" entry", ops)
	}
	if op.Method != http.MethodGet || op.Path != "/tags" || op.Domain != "tags" {
		t.Errorf("parseSpecOperations() TagsList = %+v, want Method=GET Path=/tags Domain=tags", op)
	}
}

// TestUnit_ParseSpecOperations_SkipsOperationsWithNoID mirrors
// cmd/audit_1to1's identical rule: an operation with no operationId cannot
// be named, so it is dropped rather than erroring the whole document.
func TestUnit_ParseSpecOperations_SkipsOperationsWithNoID(t *testing.T) {
	t.Parallel()
	doc := `{"paths": {"/websocket/exec": {"get": {"tags": ["websocket"]}}}}`
	ops, err := parseSpecOperations([]byte(doc))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 0 {
		t.Errorf("parseSpecOperations() = %v, want empty: the operation has no operationId", ops)
	}
}

// TestUnit_ParseSpecOperations_IgnoresNonVerbKeys proves a path item's
// "parameters" and "summary" keys (not HTTP methods) are not mistaken for
// operations.
func TestUnit_ParseSpecOperations_IgnoresNonVerbKeys(t *testing.T) {
	t.Parallel()
	doc := `{"paths": {"/tags": {
		"parameters": [{"name": "id", "in": "path"}],
		"get": {"operationId": "tagsList", "tags": ["tags"]}
	}}}`
	ops, err := parseSpecOperations([]byte(doc))
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 1 {
		t.Errorf("parseSpecOperations() = %d operation(s), want exactly 1 (parameters is not an operation)", len(ops))
	}
}

// TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError proves two
// path items declaring the same operationId is refused, not silently
// resolved by keeping one at random.
func TestUnit_ParseSpecOperations_DuplicateOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	doc := `{"paths": {
		"/a": {"get": {"operationId": "dup", "tags": ["x"]}},
		"/b": {"get": {"operationId": "dup", "tags": ["x"]}}
	}}`
	if _, err := parseSpecOperations([]byte(doc)); err == nil {
		t.Fatal("parseSpecOperations() error = nil, want an error for a duplicate operationId")
	}
}

// TestUnit_ParseSpecOperations_MalformedDocument_ReturnsError guards the
// plumbing failure mode.
func TestUnit_ParseSpecOperations_MalformedDocument_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := parseSpecOperations([]byte("not json")); err == nil {
		t.Fatal("parseSpecOperations() error = nil, want an error for malformed JSON")
	}
}

// TestUnit_ParseSpecOperations_DocumentLevelSecurityFallback proves an
// operation with no security field of its own inherits the document-level
// default, while an operation that explicitly declares an empty array
// overrides that default rather than inheriting it — the vendored specs
// carry no root-level security today, but a future one that does must not
// have every such operation misclassified as public.
func TestUnit_ParseSpecOperations_DocumentLevelSecurityFallback(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		doc        string
		wantPublic bool
	}{
		{
			name:       "operation declares no security, document requires one: not public",
			doc:        `{"security": [{"apiKey": []}], "paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"]}}}}`,
			wantPublic: false,
		},
		{
			name:       "operation explicitly declares empty security, document requires one: public",
			doc:        `{"security": [{"apiKey": []}], "paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"], "security": []}}}}`,
			wantPublic: true,
		},
		{
			name:       "neither operation nor document declares security: public",
			doc:        `{"paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"]}}}}`,
			wantPublic: true,
		},
		{
			name:       "operation declares its own security, document has none: not public",
			doc:        `{"paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"], "security": [{"apiKey": []}]}}}}`,
			wantPublic: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ops, err := parseSpecOperations([]byte(tc.doc))
			if err != nil {
				t.Fatalf("parseSpecOperations() error = %v", err)
			}
			op, ok := ops["TagsList"]
			if !ok {
				t.Fatalf("parseSpecOperations() = %v, want a \"TagsList\" entry", ops)
			}
			if op.Public != tc.wantPublic {
				t.Errorf("parseSpecOperations() TagsList.Public = %v, want %v", op.Public, tc.wantPublic)
			}
		})
	}
}

// TestUnit_ExportedName_UppercasesFirstRuneOnly matches
// cmd/audit_1to1's identical transform.
func TestUnit_ExportedName_UppercasesFirstRuneOnly(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		id   string
		want string
	}{
		{name: "lower camel case gets its first rune uppercased", id: "systemStatus", want: "SystemStatus"},
		{name: "empty input stays empty", id: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := exportedName(tc.id); got != tc.want {
				t.Errorf("exportedName(%q) = %q, want %q", tc.id, got, tc.want)
			}
		})
	}
}
