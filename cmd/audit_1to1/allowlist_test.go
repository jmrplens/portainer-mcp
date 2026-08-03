package main

import (
	"strings"
	"testing"
)

func TestUnit_ParseAllowList_ValidEntries_Parses(t *testing.T) {
	t.Parallel()
	data := []byte(`
- operation_id: WebsocketExec
  reason: "MCP cannot carry a websocket upgrade."
  added: 2026-08-03
`)

	entries, err := parseAllowList(data)
	if err != nil {
		t.Fatalf("parseAllowList() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseAllowList() = %v, want 1 entry", entries)
	}
	if entries[0].OperationID != "WebsocketExec" {
		t.Errorf("parseAllowList()[0].OperationID = %q, want %q", entries[0].OperationID, "WebsocketExec")
	}
}

func TestUnit_ParseAllowList_EmptyList_ReturnsNoEntriesNotError(t *testing.T) {
	t.Parallel()
	entries, err := parseAllowList([]byte("[]\n"))
	if err != nil {
		t.Fatalf("parseAllowList() error = %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("parseAllowList() = %v, want no entries", entries)
	}
}

func TestUnit_ParseAllowList_MissingOperationID_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`
- reason: "some reason"
  added: 2026-08-03
`)
	if _, err := parseAllowList(data); err == nil {
		t.Fatal("parseAllowList() = nil error, want error for missing operation_id")
	}
}

func TestUnit_ParseAllowList_MissingReason_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`
- operation_id: WebsocketExec
  added: 2026-08-03
`)
	if _, err := parseAllowList(data); err == nil {
		t.Fatal("parseAllowList() = nil error, want error for missing reason")
	}
}

func TestUnit_ParseAllowList_MissingAdded_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`
- operation_id: WebsocketExec
  reason: "some reason"
`)
	if _, err := parseAllowList(data); err == nil {
		t.Fatal("parseAllowList() = nil error, want error for missing added date")
	}
}

func TestUnit_ParseAllowList_InvalidAddedDate_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`
- operation_id: WebsocketExec
  reason: "some reason"
  added: "3rd August 2026"
`)
	err := mustError(t, data)
	if !strings.Contains(err.Error(), "added") {
		t.Errorf("parseAllowList() error = %v, want it to name the added field", err)
	}
}

func TestUnit_ParseAllowList_DuplicateOperationID_IsError(t *testing.T) {
	t.Parallel()
	data := []byte(`
- operation_id: WebsocketExec
  reason: "first"
  added: 2026-08-03
- operation_id: WebsocketExec
  reason: "second"
  added: 2026-08-03
`)
	err := mustError(t, data)
	if !strings.Contains(err.Error(), "WebsocketExec") {
		t.Errorf("parseAllowList() error = %v, want it to name the duplicated id", err)
	}
}

func TestUnit_ParseAllowList_MalformedYAML_IsError(t *testing.T) {
	t.Parallel()
	if _, err := parseAllowList([]byte("not: [valid, yaml")); err == nil {
		t.Fatal("parseAllowList() = nil error, want a decode error")
	}
}

func mustError(t *testing.T, data []byte) error {
	t.Helper()
	_, err := parseAllowList(data)
	if err == nil {
		t.Fatal("parseAllowList() = nil error, want an error")
	}
	return err
}
