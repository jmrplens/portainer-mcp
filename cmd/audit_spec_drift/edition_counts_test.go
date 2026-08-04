package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

// editionCountsFixtureInput mirrors internal/tools/actioncatalog's own
// editionGatedInput fixture exactly (see that package's catalog_test.go):
// one plain field, one edition:"EE" field — the minimal shape that proves
// oneEditionFieldCounts is actually measuring Task 2's per-field pruning,
// not merely counting whatever a fixture happens to supply.
type editionCountsFixtureInput struct {
	Name    string `json:"name"`
	EdgeKey string `json:"edgeKey,omitempty" edition:"EE"`
}

// editionCountsFixtureAction names a real, resolvable OperationID
// (SystemInfo, shared by both editions in the real applicability table) —
// actioncatalog.Build refuses an OperationID that does not resolve there
// (see that function's own doc comment), so a fixture with an invented one
// (this package's own "Fixture" convention used elsewhere in this package)
// would fail before ever reaching the pruning this test exists to check.
func editionCountsFixtureAction() toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: "system.info", Domain: "system", OperationID: "SystemInfo",
		Title: "t", Description: "d", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
		Input:   editionCountsFixtureInput{},
	}
}

// TestUnit_OneEditionFieldCounts_PrunesEEOnlyFieldFromCE is the CE half:
// building the catalog for Community Edition from an action whose Input
// tags one field edition:"EE" must not count that field.
func TestUnit_OneEditionFieldCounts_PrunesEEOnlyFieldFromCE(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{editionCountsFixtureAction()}

	counts, err := oneEditionFieldCounts(actions, edition.CE, "2.44.0")
	if err != nil {
		t.Fatalf("oneEditionFieldCounts(CE) error = %v", err)
	}
	if counts.Actions != 1 {
		t.Fatalf("oneEditionFieldCounts(CE) Actions = %d, want 1", counts.Actions)
	}
	if counts.Fields != 1 {
		t.Errorf("oneEditionFieldCounts(CE) Fields = %d, want 1: the edition:\"EE\" field must be pruned from Community Edition", counts.Fields)
	}
}

// TestUnit_OneEditionFieldCounts_KeepsEEOnlyFieldForEE is the discriminating
// other half: the identical action, built for Business Edition, must count
// both fields. Without this half, an implementation that always dropped the
// tagged field regardless of target edition — or one that never pruned
// anything at all and coincidentally reported 1 for some unrelated reason —
// would still pass the CE-only test above.
func TestUnit_OneEditionFieldCounts_KeepsEEOnlyFieldForEE(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{editionCountsFixtureAction()}

	counts, err := oneEditionFieldCounts(actions, edition.EE, "2.44.0")
	if err != nil {
		t.Fatalf("oneEditionFieldCounts(EE) error = %v", err)
	}
	if counts.Fields != 2 {
		t.Errorf("oneEditionFieldCounts(EE) Fields = %d, want 2: Business Edition must publish both fields", counts.Fields)
	}
}

// TestUnit_OneEditionFieldCounts_UnresolvableOperationID_ReturnsError proves
// this function's error is not swallowed: actioncatalog.Build's own refusal
// for an OperationID absent from the applicability table must surface here,
// wrapped, not silently reported as zero actions and zero fields.
func TestUnit_OneEditionFieldCounts_UnresolvableOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{{
		Name: "fixture.action", Domain: "fixture", OperationID: "NoSuchOperation",
		Title: "t", Description: "d", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
	}}
	if _, err := oneEditionFieldCounts(actions, edition.CE, "2.44.0"); err == nil {
		t.Fatal("oneEditionFieldCounts() error = nil, want an error for an OperationID absent from the applicability table")
	}
}

// TestUnit_AuditEditionFieldCounts_BothEditionsBuilt proves
// auditEditionFieldCounts actually builds and reports both editions from
// the same action, not only the one it happens to be called with first.
func TestUnit_AuditEditionFieldCounts_BothEditionsBuilt(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{editionCountsFixtureAction()}

	result, err := auditEditionFieldCounts(actions, "2.44.0")
	if err != nil {
		t.Fatalf("auditEditionFieldCounts() error = %v", err)
	}
	if result.CE.Fields != 1 {
		t.Errorf("auditEditionFieldCounts() CE.Fields = %d, want 1", result.CE.Fields)
	}
	if result.EE.Fields != 2 {
		t.Errorf("auditEditionFieldCounts() EE.Fields = %d, want 2", result.EE.Fields)
	}
	if result.CE.Actions != 1 || result.EE.Actions != 1 {
		t.Errorf("auditEditionFieldCounts() Actions = (CE %d, EE %d), want (1, 1)", result.CE.Actions, result.EE.Actions)
	}
}

// TestUnit_AuditEditionFieldCounts_RealCatalog_MatchesMeasuredCounts pins
// this function's output against the real declared catalog
// (internal/wiring.AllSpecs, the same actions cmd/audit_spec_drift audits in
// every real run). The numbers are not invented: measured directly by
// building both real catalogs and summing each one's own
// Catalog.InputSchema property counts at the time this test was written —
// 18 actions/51 fields for Business Edition, 15 actions/40 fields for
// Community Edition (system.upgrade is Community-only and contributes no
// fields; system.update and registries' three ECR/repository-tag actions are
// Business-only; registries.create/update's "github" and
// registries.inspect's "endpointId" are the three fields Task 2's mechanism
// prunes from the 14 actions both editions share). If per-field pruning
// silently stopped working, Community's count would climb from 40 to 43 (the
// three pruned fields returning) with this test failing on that exact
// number, not merely "some number changed".
func TestUnit_AuditEditionFieldCounts_RealCatalog_MatchesMeasuredCounts(t *testing.T) {
	t.Parallel()
	result, err := auditEditionFieldCounts(wiring.AllSpecs(), "2.44.0")
	if err != nil {
		t.Fatalf("auditEditionFieldCounts() error = %v", err)
	}
	if result.EE.Actions != 18 || result.EE.Fields != 51 {
		t.Errorf("auditEditionFieldCounts() EE = %+v, want {Actions:18 Fields:51}", result.EE)
	}
	if result.CE.Actions != 15 || result.CE.Fields != 40 {
		t.Errorf("auditEditionFieldCounts() CE = %+v, want {Actions:15 Fields:40}: if this regressed to 43, per-field edition pruning has silently stopped", result.CE)
	}
}
