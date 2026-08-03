package main

import (
	"net/http"
	"strings"
	"testing"
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

	ops, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
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

func TestUnit_ParseSpecOperations_SkipsOperationsWithNoOperationID(t *testing.T) {
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

	ops, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
	if len(ops) != 1 {
		t.Fatalf("parseSpecOperations() = %v, want exactly the one operation with an operationId", ops)
	}
	if _, ok := ops["TagList"]; !ok {
		t.Errorf("parseSpecOperations() = %v, want TagList", ops)
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

	ops, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}
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
