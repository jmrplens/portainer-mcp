package main

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
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

// TestUnit_OneEditionFieldCounts_PrunesOrKeepsEEOnlyFieldByEdition is
// table-driven over the two compatible edition-count scenarios that share
// the identical fixture action and differ only by target edition and the
// resulting Fields count: Community Edition must not count the
// edition:"EE"-tagged field (pruned), Business Edition must count both
// (kept). Without the discriminating EE case, an implementation that always
// dropped the tagged field regardless of target edition — or one that never
// pruned anything at all and coincidentally reported 1 for some unrelated
// reason — would still pass the CE case alone.
func TestUnit_OneEditionFieldCounts_PrunesOrKeepsEEOnlyFieldByEdition(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		edition    edition.Edition
		wantFields int
	}{
		{"community edition prunes the EE-only field", edition.CE, 1},
		{"business edition keeps the EE-only field", edition.EE, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			actions := []toolutil.ActionSpec{editionCountsFixtureAction()}

			counts, err := oneEditionFieldCounts(actions, tc.edition, "2.44.0")
			if err != nil {
				t.Fatalf("oneEditionFieldCounts(%s) error = %v", tc.edition, err)
			}
			if counts.Actions != 1 {
				t.Fatalf("oneEditionFieldCounts(%s) Actions = %d, want 1", tc.edition, counts.Actions)
			}
			if counts.Fields != tc.wantFields {
				t.Errorf("oneEditionFieldCounts(%s) Fields = %d, want %d", tc.edition, counts.Fields, tc.wantFields)
			}
		})
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
// every real run).
//
// It deliberately does not pin an absolute action or field count the way an
// earlier revision of this test did (EE{Actions:18,Fields:51},
// CE{Actions:15,Fields:40}): every wave of new domains changes both numbers,
// and a hard-coded literal here "went stale on every wave and failed the
// build with no note pointing back at itself" (docs/domain-wave-checklist.md
// Step 1.4, which lists the actual four places a new domain must be
// registered and explicitly does not name this test as a fifth — this is
// meant to need no manual update at all, the same fix Task 6 applied to
// server_test.go's own hard-coded meta tool list).
//
// Instead it derives, per action, what per-field pruning must remove,
// independently of oneEditionFieldCounts/pruneInputSchemaByEdition: for every
// action present in *both* a freshly-built Community and Business catalog
// (only those two catalogs can differ on a shared action's own field set;
// system.upgrade/system.update and registries' ECR/repository-tag actions
// are whole-action EE-or-CE-only and contribute nothing comparable here),
// wantGated sums toolutil.FieldEditions(Input type) — a plain struct-tag
// reflection, never a schema-property count — and the test asserts the
// Business schema's own property count minus the Community schema's is
// exactly that number, action by action. If per-field pruning silently
// stopped working, every shared action's Business and Community property
// counts would become equal (delta 0) while wantGated, computed from the
// struct tags alone, stays exactly what it always was — the mismatch this
// test now fails on, for whichever action regressed, however many domains
// have landed by the time it runs.
func TestUnit_AuditEditionFieldCounts_RealCatalog_MatchesMeasuredCounts(t *testing.T) {
	t.Parallel()
	actions := wiring.AllSpecs()

	result, err := auditEditionFieldCounts(actions, "2.44.0")
	if err != nil {
		t.Fatalf("auditEditionFieldCounts() error = %v", err)
	}
	if result.EE.Actions == 0 || result.EE.Fields == 0 {
		t.Fatalf("auditEditionFieldCounts() EE = %+v, want at least one action and one field from the real catalog", result.EE)
	}
	if result.CE.Actions == 0 || result.CE.Fields == 0 {
		t.Fatalf("auditEditionFieldCounts() CE = %+v, want at least one action and one field from the real catalog", result.CE)
	}

	ceCatalog, err := actioncatalog.Build(actions, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build(CE): %v", err)
	}
	eeCatalog, err := actioncatalog.Build(actions, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build(EE): %v", err)
	}

	totalGated := 0
	for _, ceSpec := range ceCatalog.Actions() {
		eeSpec, sharedByBothEditions := eeCatalog.Lookup(ceSpec.Name)
		if !sharedByBothEditions {
			continue
		}

		wantGated := 0
		if ceSpec.Input != nil {
			inputType := reflect.TypeOf(ceSpec.Input)
			for inputType.Kind() == reflect.Pointer {
				inputType = inputType.Elem()
			}
			fieldEditions, err := toolutil.FieldEditions(inputType)
			if err != nil {
				t.Fatalf("toolutil.FieldEditions(%s): %v", ceSpec.Name, err)
			}
			wantGated = len(fieldEditions)
		}

		ceSchema, err := ceCatalog.InputSchema(ceSpec)
		if err != nil {
			t.Fatalf("ceCatalog.InputSchema(%s): %v", ceSpec.Name, err)
		}
		eeSchema, err := eeCatalog.InputSchema(eeSpec)
		if err != nil {
			t.Fatalf("eeCatalog.InputSchema(%s): %v", eeSpec.Name, err)
		}
		ceProps, _ := ceSchema["properties"].(map[string]any)
		eeProps, _ := eeSchema["properties"].(map[string]any)

		if gotGated := len(eeProps) - len(ceProps); gotGated != wantGated {
			t.Errorf("%s: EE properties (%d) - CE properties (%d) = %d, want %d (the number of edition:\"EE\"-tagged top-level fields on this shared action's own Input type): if this dropped to 0 while the struct still carries the tag, per-field edition pruning has silently stopped for this action",
				ceSpec.Name, len(eeProps), len(ceProps), gotGated, wantGated)
		}
		totalGated += wantGated
	}

	if totalGated == 0 {
		t.Fatal("test setup bug: no action shared by both real catalogs declares an edition:\"EE\" field; this test cannot discriminate pruning stopping without at least one (see internal/tools/registries's registryCreateInput.Github and registryInspectInput.EndpointID)")
	}
}
