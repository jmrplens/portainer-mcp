package main

import (
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// allowListEntry excuses one spec operation from audit_1to1's pass/fail gate.
//
// Every field is required. OperationID identifies exactly what is excused, in
// the same exported-Go form as toolutil.ActionSpec.OperationID and
// specOperation.OperationID, so the three are comparable directly. Reason is
// read by a human deciding whether the exclusion still makes sense. Added
// dates the entry so staleness is discoverable by inspection, not by
// archaeology through git blame.
type allowListEntry struct {
	OperationID string `yaml:"operation_id"`
	Reason      string `yaml:"reason"`
	Added       string `yaml:"added"`
}

// allowListDateFormat is the ISO 8601 calendar-date layout every entry's
// Added field must use.
const allowListDateFormat = "2006-01-02"

// parseAllowList decodes the coverage allow-list and validates that every
// entry is well-formed: present fields, no duplicate OperationID, and a
// parseable date.
//
// It deliberately does not check that OperationID resolves against either
// vendored spec here — that cross-check needs both specs loaded, and lives in
// auditCoverage, where "resolves against neither" is its own reported
// failure rather than folded into a parse error with no access to the specs
// that would explain it.
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
		if seen[e.OperationID] {
			return nil, fmt.Errorf("allow-list entry %d: %q is listed twice", i, e.OperationID)
		}
		seen[e.OperationID] = true
		if e.Reason == "" {
			return nil, fmt.Errorf("allow-list entry %q: reason is required", e.OperationID)
		}
		if e.Added == "" {
			return nil, fmt.Errorf("allow-list entry %q: added date is required", e.OperationID)
		}
		if _, err := time.Parse(allowListDateFormat, e.Added); err != nil {
			return nil, fmt.Errorf("allow-list entry %q: added %q is not an ISO date (YYYY-MM-DD): %w",
				e.OperationID, e.Added, err)
		}
	}
	return entries, nil
}
