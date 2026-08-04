package main

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// allowListEntry excuses one gating field-level finding from this audit's
// pass/fail gate — one field of one operation, never a whole operation:
// the four hand-written pilot actions that resisted generation each took a
// deliberately narrower type for one or two fields, not an entirely
// different shape, and forgiving the whole operation would also forgive any
// unrelated field that happens to drift later.
//
// Field also accepts specdiff.TitleSentinel ("$title") or
// specdiff.DescriptionSentinel ("$description") to excuse a deliberately
// hand-authored operation-level Title or Description — the tags and
// registries pilots' own ActionSpec literals, and registries/system's
// narrative() hooks, all replace the vendored specification's own summary/
// description with improved prose on purpose. This is the same mechanism
// excusing a parameter's narrower type, not a new one: toolutil.WithNarrative
// fully replaces Title/Description rather than tagging which fields it
// touched, so there is nothing left on toolutil.ActionSpec by the time
// ShapeFromCatalog reads it to recognise "this was a deliberate override"
// from — the allow-list is what records that decision instead, the same way
// it already does for a parameter.
//
// Every field is required, mirroring cmd/audit_1to1's allowListEntry
// exactly and for the identical reason: Reason is read by a human deciding
// whether the exclusion still makes sense, and Added dates the entry so
// staleness is discoverable by inspection rather than archaeology through
// git blame.
type allowListEntry struct {
	OperationID string `yaml:"operation_id"`
	Field       string `yaml:"field"`
	Reason      string `yaml:"reason"`
	Added       string `yaml:"added"`
}

// allowListDateFormat is the ISO 8601 calendar-date layout every entry's
// Added field must use, matching cmd/audit_1to1's identical constant.
const allowListDateFormat = "2006-01-02"

// parseAllowList decodes the drift allow-list and validates that every entry
// is well-formed: present fields, no duplicate (operation_id, field) pair,
// and a parseable date.
//
// It deliberately does not check here whether the named field actually
// diverges — that requires the audit result, and lives in auditDrift, where
// an entry matching no real, currently-gating finding is reported as stale
// rather than folded into a parse error with no access to what would explain
// it.
func parseAllowList(data []byte) ([]allowListEntry, error) {
	var entries []allowListEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("decode allow-list: %w", err)
	}

	seen := make(map[string]bool, len(entries))
	for i, e := range entries {
		if e.OperationID == "" {
			return nil, fmt.Errorf("allow-list entry %d: operation_id is required", i)
		}
		if e.Field == "" {
			return nil, fmt.Errorf("allow-list entry %d (%s): field is required", i, e.OperationID)
		}
		key := e.OperationID + "\x00" + e.Field
		if seen[key] {
			return nil, fmt.Errorf("allow-list entry %d: %q field %q is listed twice", i, e.OperationID, e.Field)
		}
		seen[key] = true
		if e.Reason == "" {
			return nil, fmt.Errorf("allow-list entry %q field %q: reason is required", e.OperationID, e.Field)
		}
		if e.Added == "" {
			return nil, fmt.Errorf("allow-list entry %q field %q: added date is required", e.OperationID, e.Field)
		}
		if _, err := time.Parse(allowListDateFormat, e.Added); err != nil {
			return nil, fmt.Errorf("allow-list entry %q field %q: added %q is not an ISO date (YYYY-MM-DD): %w",
				e.OperationID, e.Field, e.Added, err)
		}
	}
	return entries, nil
}
