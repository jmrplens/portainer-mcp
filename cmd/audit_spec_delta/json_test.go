package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/specdiff"
)

// TestUnit_ToJSONReport_CountsMatchTheDeltaResult proves the JSON rendering
// is a pure relabelling of deltaResult's own counts, not a second
// computation that could drift from what buildReport prints.
func TestUnit_ToJSONReport_CountsMatchTheDeltaResult(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		BeforeCount: 427, AfterCount: 442,
		AddedCount: 20, RemovedCount: 5, ChangedCount: 26, ChangedStructCount: 12,
	}
	report := toJSONReport("before.json", "after.json", result)

	if report.Before != "before.json" || report.After != "after.json" {
		t.Errorf("toJSONReport() Before/After = %q/%q, want the paths passed in", report.Before, report.After)
	}
	if report.Counts.Added != 20 || report.Counts.Removed != 5 || report.Counts.Changed != 26 || report.Counts.ChangedTouchStruct != 12 {
		t.Errorf("toJSONReport() Counts = %+v, want {Added:20 Removed:5 Changed:26 ChangedTouchStruct:12}", report.Counts)
	}
}

// TestUnit_ToJSONReport_RoundTripsThroughRealJSONMarshalling proves the
// struct tags actually produce valid, snake_case JSON a future consumer
// could parse — not merely that the Go struct has the right field values.
func TestUnit_ToJSONReport_RoundTripsThroughRealJSONMarshalling(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		AddedCount: 1,
		Domains: []domainGroup{{
			Domain: "widgets",
			Added:  []opRef{{OperationID: "WidgetCreate", Method: "POST", Path: "/widgets"}},
			ChangedStruct: []changedOp{{
				OperationID: "WidgetUpdate", Method: "PUT", Path: "/widgets/{id}",
				Changes:       []specdiff.FieldChange{{JSONName: "count", Kind: specdiff.ChangeType, Before: "integer", After: "string"}},
				TouchesStruct: true,
			}},
		}},
	}
	report := toJSONReport("a", "b", result)

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if _, ok := decoded["operation_id"]; ok {
		t.Error("decoded top level has \"operation_id\": that field belongs to nested ops, not the report root")
	}
	domains, ok := decoded["domains"].([]any)
	if !ok || len(domains) != 1 {
		t.Fatalf("decoded[\"domains\"] = %v, want exactly one domain", decoded["domains"])
	}
	dom, ok := domains[0].(map[string]any)
	if !ok {
		t.Fatalf("domains[0] = %v, want an object", domains[0])
	}
	added, ok := dom["added"].([]any)
	if !ok || len(added) != 1 {
		t.Fatalf("domains[0][\"added\"] = %v, want exactly one entry", dom["added"])
	}
	addedOp, _ := added[0].(map[string]any)
	if addedOp["operation_id"] != "WidgetCreate" {
		t.Errorf("added[0][\"operation_id\"] = %v, want %q", addedOp["operation_id"], "WidgetCreate")
	}

	changedStruct, ok := dom["changed_struct"].([]any)
	if !ok || len(changedStruct) != 1 {
		t.Fatalf("domains[0][\"changed_struct\"] = %v, want exactly one entry", dom["changed_struct"])
	}
	changedOpDecoded, _ := changedStruct[0].(map[string]any)
	if changedOpDecoded["touches_struct"] != true {
		t.Errorf("changed_struct[0][\"touches_struct\"] = %v, want true", changedOpDecoded["touches_struct"])
	}
	changes, ok := changedOpDecoded["changes"].([]any)
	if !ok || len(changes) != 1 {
		t.Fatalf("changed_struct[0][\"changes\"] = %v, want exactly one FieldChange", changedOpDecoded["changes"])
	}
	change, _ := changes[0].(map[string]any)
	if change["field"] != "count" || change["kind"] != "type" || change["before"] != "integer" || change["after"] != "string" {
		t.Errorf("changes[0] = %v, want field=count kind=type before=integer after=string", change)
	}
}

// TestUnit_ToJSONReport_EmptyDomainBuckets_OmittedNotNull proves the
// `omitempty` tags actually take effect: a domain with only additions must
// not carry an explicit "removed": null a consumer would have to
// special-case.
func TestUnit_ToJSONReport_EmptyDomainBuckets_OmittedNotNull(t *testing.T) {
	t.Parallel()
	result := &deltaResult{
		AddedCount: 1,
		Domains: []domainGroup{{
			Domain: "widgets",
			Added:  []opRef{{OperationID: "WidgetCreate", Method: "POST", Path: "/widgets"}},
		}},
	}
	encoded, err := json.Marshal(toJSONReport("a", "b", result))
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	got := string(encoded)
	for _, forbidden := range []string{`"removed":null`, `"changed_struct":null`, `"changed_cosmetic":null`} {
		if strings.Contains(got, forbidden) {
			t.Errorf("encoded JSON = %s, want empty buckets omitted rather than null (found %q)", got, forbidden)
		}
	}
}
