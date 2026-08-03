package main

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// TestDomainOperations_KnownDomain_AggregatesAcrossItsTags guards the normal
// case: a domain covering more than one tag (as "cloud" does in the real
// table, covering only "cloud_credentials", but tested here with a synthetic
// two-tag domain to prove aggregation itself, independent of the real
// table's current shape) gets every operation from every tag it names,
// merged and sorted by OperationID.
func TestDomainOperations_KnownDomain_AggregatesAcrossItsTags(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"cloud": {"cloud_credentials", "kaas"}}
	byTag := map[string][]operation{
		"cloud_credentials": {{OperationID: "SharedGitCreate"}, {OperationID: "CloudCredsGetByID"}},
		"kaas":              {{OperationID: "Upgrade"}},
	}

	ops, err := domainOperations("cloud", domainTags, byTag)
	if err != nil {
		t.Fatalf("domainOperations() error = %v", err)
	}
	if len(ops) != 3 {
		t.Fatalf("domainOperations() = %v, want 3 operations aggregated across both tags", ops)
	}
	var ids []string
	for _, op := range ops {
		ids = append(ids, op.OperationID)
	}
	if got, want := strings.Join(ids, ","), "CloudCredsGetByID,SharedGitCreate,Upgrade"; got != want {
		t.Errorf("operation order = %q, want %q (sorted by OperationID)", got, want)
	}
}

// TestDomainOperations_DomainDirectoryHasNoEntry_ReturnsError is C1's core
// mutation: a domain directory this table does not name must be a hard
// error. Before the fix, main.go's loop did `ops := byDomain[domainName];
// if len(ops) == 0 { continue }` — indistinguishable, to that code, from a
// domain that legitimately has zero fields to generate for. Proven for real
// against a directory named "edgestacks": the actual tag for those
// operations is "edge_stacks" (see toolutil.DomainTags), so a directory
// spelled without the underscore has no entry and must fail loudly instead
// of silently producing nothing.
func TestDomainOperations_DomainDirectoryHasNoEntry_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"edge_stacks": {"edge_stacks"}}
	byTag := map[string][]operation{
		"edge_stacks": {{OperationID: "EdgeStackList"}},
	}

	_, err := domainOperations("edgestacks", domainTags, byTag)
	if err == nil {
		t.Fatal("domainOperations() = nil error, want one: \"edgestacks\" has no entry in domainTags")
	}
	if !strings.Contains(err.Error(), "edgestacks") {
		t.Errorf("error = %q, want it to name the unmapped directory", err)
	}
}

// TestCheckDomainTagsCoverSpec_ValidTable_Succeeds is the baseline: every tag
// domainTags names has operations, and every tag with operations is named.
func TestCheckDomainTagsCoverSpec_ValidTable_Succeeds(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{
		"tags":   {"tags"},
		"backup": {"backup"},
	}
	byTag := map[string][]operation{
		"tags":   {{OperationID: "TagList"}},
		"backup": {{OperationID: "Backup"}, {OperationID: "Restore"}},
	}
	if err := checkDomainTagsCoverSpec(domainTags, byTag); err != nil {
		t.Fatalf("checkDomainTagsCoverSpec() error = %v, want nil for a fully-covered table", err)
	}
}

// TestCheckDomainTagsCoverSpec_TagWithOperationsHasNoEntry_ReturnsError is
// C1's reverse direction: a tag the vendored spec has real operations under,
// but that no domain in the table claims, must fail loudly. This is exactly
// how 127 operations across 12 tags went unreachable before this table
// existed — nothing ever noticed a tag the table had simply never been told
// about.
func TestCheckDomainTagsCoverSpec_TagWithOperationsHasNoEntry_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"tags": {"tags"}}
	byTag := map[string][]operation{
		"tags":   {{OperationID: "TagList"}},
		"backup": {{OperationID: "Backup"}}, // no domain claims "backup"
	}
	err := checkDomainTagsCoverSpec(domainTags, byTag)
	if err == nil {
		t.Fatal("checkDomainTagsCoverSpec() = nil error, want one: \"backup\" has operations but no domain entry")
	}
	if !strings.Contains(err.Error(), "backup") {
		t.Errorf("error = %q, want it to name the uncovered tag", err)
	}
}

// TestCheckDomainTagsCoverSpec_TableNamesATagAbsentFromSpec_ReturnsError
// guards a typo in the table itself: a domain naming a tag that has zero
// operations in the vendored spec would otherwise resolve to zero
// operations for that domain and be indistinguishable from a domain that
// legitimately generates nothing — reintroducing this table's own defect
// one level up, inside the table.
func TestCheckDomainTagsCoverSpec_TableNamesATagAbsentFromSpec_ReturnsError(t *testing.T) {
	t.Parallel()
	domainTags := map[string][]string{"backup": {"backupp"}} // typo
	byTag := map[string][]operation{
		"backup": {{OperationID: "Backup"}},
	}
	err := checkDomainTagsCoverSpec(domainTags, byTag)
	if err == nil {
		t.Fatal("checkDomainTagsCoverSpec() = nil error, want one: \"backupp\" does not exist in the vendored spec")
	}
	if !strings.Contains(err.Error(), "backupp") {
		t.Errorf("error = %q, want it to name the typo'd tag", err)
	}
}

// TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec cross-checks
// toolutil.DomainTags — the real, curated table — against the real vendored
// EE specification, the superset gen-action-inputs actually runs against
// (see Makefile's gen-action-inputs target). This is what proves the table
// was genuinely populated for all 46 tags, not merely asserted to have 46
// entries in internal/toolutil's own unit test.
func TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec(t *testing.T) {
	t.Parallel()
	if err := toolutil.ValidateDomainTags(toolutil.DomainTags); err != nil {
		t.Fatalf("toolutil.ValidateDomainTags(toolutil.DomainTags) error = %v", err)
	}

	_, paths, err := loadDocument("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}

	if err := checkDomainTagsCoverSpec(toolutil.DomainTags, byTag); err != nil {
		t.Fatalf("checkDomainTagsCoverSpec(toolutil.DomainTags, <real spec>) error = %v, want the curated table to fully cover the vendored spec's tags", err)
	}
}
