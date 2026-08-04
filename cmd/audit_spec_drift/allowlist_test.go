package main

import (
	"strings"
	"testing"
)

func TestUnit_ParseAllowList_ValidEntries_Decodes(t *testing.T) {
	t.Parallel()
	const doc = `
- operation_id: SystemUpgrade
  field: edgeAgentCheckinInterval
  reason: "hand-written pilot narrows this to a fixed type on purpose."
  added: 2026-08-04
`
	entries, err := parseAllowList([]byte(doc))
	if err != nil {
		t.Fatalf("parseAllowList() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseAllowList() = %d entries, want 1", len(entries))
	}
	e := entries[0]
	if e.OperationID != "SystemUpgrade" || e.Field != "edgeAgentCheckinInterval" || e.Added != "2026-08-04" {
		t.Errorf("parseAllowList() = %+v, want the fixture's fields verbatim", e)
	}
}

func TestUnit_ParseAllowList_Empty_ReturnsNoEntries(t *testing.T) {
	t.Parallel()
	entries, err := parseAllowList([]byte("[]\n"))
	if err != nil {
		t.Fatalf("parseAllowList() error = %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("parseAllowList() = %v, want empty", entries)
	}
}

// TestUnit_ParseAllowList_RejectsMalformedEntries proves every required
// field is actually enforced, one omission at a time, mirroring
// cmd/audit_1to1's identical table for its own allow-list.
func TestUnit_ParseAllowList_RejectsMalformedEntries(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		doc  string
	}{
		{"missing operation_id", `- field: x
  reason: "r"
  added: 2026-08-04
`},
		{"missing field", `- operation_id: X
  reason: "r"
  added: 2026-08-04
`},
		{"missing reason", `- operation_id: X
  field: x
  added: 2026-08-04
`},
		{"missing added", `- operation_id: X
  field: x
  reason: "r"
`},
		{"unparseable date", `- operation_id: X
  field: x
  reason: "r"
  added: "not a date"
`},
		{"duplicate operation+field pair", `
- operation_id: X
  field: x
  reason: "r"
  added: 2026-08-04
- operation_id: X
  field: x
  reason: "r2"
  added: 2026-08-04
`},
		{"malformed YAML", `not: [valid`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := parseAllowList([]byte(tc.doc)); err == nil {
				t.Errorf("parseAllowList(%q) = nil error, want an error", tc.name)
			}
		})
	}
}

// TestUnit_ParseAllowList_SameOperationDifferentField_IsNotADuplicate proves
// the uniqueness key is the (operation_id, field) pair, not operation_id
// alone: two entries for different fields of the same operation are both
// legitimate.
func TestUnit_ParseAllowList_SameOperationDifferentField_IsNotADuplicate(t *testing.T) {
	t.Parallel()
	const doc = `
- operation_id: X
  field: a
  reason: "r"
  added: 2026-08-04
- operation_id: X
  field: b
  reason: "r2"
  added: 2026-08-04
`
	entries, err := parseAllowList([]byte(doc))
	if err != nil {
		t.Fatalf("parseAllowList() error = %v, want two entries for the same operation on different fields to be accepted", err)
	}
	if len(entries) != 2 {
		t.Errorf("parseAllowList() = %d entries, want 2", len(entries))
	}
}

// TestUnit_ParseAllowList_ErrorNamesTheOffendingEntry is a light regression
// guard: an error a human has to act on should name what is wrong, not just
// that something is.
func TestUnit_ParseAllowList_ErrorNamesTheOffendingEntry(t *testing.T) {
	t.Parallel()
	const doc = `- operation_id: SystemUpgrade
  field: x
  reason: ""
  added: 2026-08-04
`
	_, err := parseAllowList([]byte(doc))
	if err == nil {
		t.Fatal("parseAllowList() = nil error, want an error for an empty reason")
	}
	if !strings.Contains(err.Error(), "SystemUpgrade") {
		t.Errorf("parseAllowList() error = %v, want it to name the offending operation_id", err)
	}
}
