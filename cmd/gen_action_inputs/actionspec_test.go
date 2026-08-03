package main

import (
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// ceDoc loads the real vendored Community Edition specification once per
// test process's need for it — used only to build ceOperationIDs, exactly
// the input editionOf needs beyond what this generator already computes
// from the EE specification it otherwise runs against.
func ceOperationIDsForTest(t *testing.T) map[string]bool {
	t.Helper()
	_, paths, err := loadDocument("../../api/specs/ce-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument(ce) error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain(ce) error = %v", err)
	}
	return ceOperationIDSet(byTag)
}

// TestUnit_CleanTitleAndDescription_StripsAccessPolicyLine is the common
// case across the vendored specifications: a description ending in
// "**Access policy**: ..." has that line removed, and the remaining prose
// survives untouched.
func TestUnit_CleanTitleAndDescription_StripsAccessPolicyLine(t *testing.T) {
	t.Parallel()
	op := operation{
		Summary:     "Create a new tag",
		Description: "Create a new tag.\n**Access policy**: administrator",
	}
	title, description, err := cleanTitleAndDescription(op)
	if err != nil {
		t.Fatalf("cleanTitleAndDescription() error = %v", err)
	}
	if title != "Create a new tag" {
		t.Errorf("title = %q, want %q", title, "Create a new tag")
	}
	if description != "Create a new tag." {
		t.Errorf("description = %q, want the access-policy line removed", description)
	}
	if strings.Contains(strings.ToLower(description), "access policy") {
		t.Errorf("description = %q, must not still mention access policy", description)
	}
}

// TestUnit_CleanTitleAndDescription_OnlyAccessPolicyLine_FallsBackToTitle is
// systemInfo's real shape in the vendored specification: its description is
// exactly "**Access policy**: authenticated", nothing else. Stripping that
// line must not publish an empty Description.
func TestUnit_CleanTitleAndDescription_OnlyAccessPolicyLine_FallsBackToTitle(t *testing.T) {
	t.Parallel()
	op := operation{
		Summary:     "Retrieve system info",
		Description: "**Access policy**: authenticated",
	}
	title, description, err := cleanTitleAndDescription(op)
	if err != nil {
		t.Fatalf("cleanTitleAndDescription() error = %v", err)
	}
	if description != title {
		t.Errorf("description = %q, want it to fall back to title %q when nothing else survives stripping", description, title)
	}
}

// TestUnit_CleanTitleAndDescription_AccessPolicyLineNotLast covers the two
// real operations (EdgeStackStatusUpdate is not one of them, but its shape
// is exercised here directly) where the boilerplate line is not the
// description's final line — the split-and-filter approach must remove it
// regardless of position, not just trim a trailing suffix.
func TestUnit_CleanTitleAndDescription_AccessPolicyLineNotLast(t *testing.T) {
	t.Parallel()
	op := operation{
		Summary:     "Do the thing",
		Description: "First line of prose.\n**Access policy**: restricted\nSecond line of prose.",
	}
	_, description, err := cleanTitleAndDescription(op)
	if err != nil {
		t.Fatalf("cleanTitleAndDescription() error = %v", err)
	}
	if strings.Contains(strings.ToLower(description), "access policy") {
		t.Errorf("description = %q, must not still mention access policy regardless of line position", description)
	}
	if !strings.Contains(description, "First line of prose.") || !strings.Contains(description, "Second line of prose.") {
		t.Errorf("description = %q, want both surrounding lines preserved", description)
	}
}

// TestUnit_CleanTitleAndDescription_EmptySummary_Refuses is the refuse-
// rather-than-guess case: every real operation in both vendored
// specifications has a non-empty summary (see
// TestUnit_CleanTitleAndDescription_EveryRealOperationHasANonEmptySummary
// below), so an empty one reaching this function is a specification defect
// that must fail generation loudly rather than publish an empty Title no
// model could act on.
func TestUnit_CleanTitleAndDescription_EmptySummary_Refuses(t *testing.T) {
	t.Parallel()
	_, _, err := cleanTitleAndDescription(operation{Summary: "  ", Description: "whatever"})
	if err == nil {
		t.Fatal("cleanTitleAndDescription() error = nil, want a refusal for a blank summary")
	}
}

// TestUnit_CleanTitleAndDescription_EveryRealOperationHasANonEmptySummary
// proves the refusal above is not exercised by any of the 442 combined
// operations across both vendored specifications today — a live check
// against the real specs, not merely asserted in this test's own fixture.
func TestUnit_CleanTitleAndDescription_EveryRealOperationHasANonEmptySummary(t *testing.T) {
	t.Parallel()
	for _, specPath := range []string{"../../api/specs/ee-2.44.0.json", "../../api/specs/ce-2.44.0.json"} {
		_, paths, err := loadDocument(specPath)
		if err != nil {
			t.Fatalf("loadDocument(%s) error = %v", specPath, err)
		}
		byTag, err := operationsByDomain(paths)
		if err != nil {
			t.Fatalf("operationsByDomain(%s) error = %v", specPath, err)
		}
		for _, ops := range byTag {
			for _, op := range ops {
				if _, _, err := cleanTitleAndDescription(op); err != nil {
					t.Errorf("%s: cleanTitleAndDescription(%s) error = %v", specPath, op.OperationID, err)
				}
			}
		}
	}
}

// TestUnit_DangerFlags_MatchesTheBriefsVerbTable is the exact table this
// task's brief specifies: GET/HEAD read-only, DELETE destructive and
// idempotent, PUT mutating and idempotent, POST/PATCH mutating and not
// idempotent.
//
// Both directions matter here, per this phase's own standing warning: a test
// asserting only "DELETE produces Destructive: true" would pass equally
// against a dangerFlags that marks every verb destructive. Every row below
// pins Mutating, Destructive AND Idempotent, so a mutation that flips any
// one flag for any one verb turns a row red — GET's row proves a read verb
// is not marked destructive, PUT and POST's rows (both Mutating, only PUT
// Idempotent) prove idempotence is the thing that differs between two verbs
// that otherwise look equally "mutating".
func TestUnit_DangerFlags_MatchesTheBriefsVerbTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		method                            string
		mutating, destructive, idempotent bool
	}{
		{"GET", false, false, false},
		{"HEAD", false, false, false},
		{"DELETE", true, true, true},
		{"PUT", true, false, true},
		{"POST", true, false, false},
		{"PATCH", true, false, false},
	} {
		mutating, destructive, idempotent, err := dangerFlags(tc.method)
		if err != nil {
			t.Errorf("dangerFlags(%q) error = %v", tc.method, err)
			continue
		}
		if mutating != tc.mutating || destructive != tc.destructive || idempotent != tc.idempotent {
			t.Errorf("dangerFlags(%q) = (%v, %v, %v), want (%v, %v, %v)",
				tc.method, mutating, destructive, idempotent, tc.mutating, tc.destructive, tc.idempotent)
		}
		if tc.destructive && !tc.mutating {
			t.Errorf("test table row for %q is internally inconsistent: Destructive implies Mutating (see toolutil.ActionSpec.Validate)", tc.method)
		}
	}
}

// TestUnit_DangerFlags_UnknownVerb_Refuses guards the "refuse rather than
// guess" rule for a verb this table has no row for.
func TestUnit_DangerFlags_UnknownVerb_Refuses(t *testing.T) {
	t.Parallel()
	if _, _, _, err := dangerFlags("OPTIONS"); err == nil {
		t.Error("dangerFlags(\"OPTIONS\") error = nil, want a refusal: this project has no rule for this verb, and none exist in the vendored specs today")
	}
}

// TestUnit_EditionOf_PresentInBoth_IsCE and its EE-only counterpart pin
// editionOf's "CE means both" convention (see toolutil.ActionSpec.Edition)
// against the two concrete shapes the pilot domains already exercise:
// RegistryList exists in both vendored specifications; EcrDeleteRepository
// exists only in the Business Edition one.
func TestUnit_EditionOf_PresentInBoth_IsCE(t *testing.T) {
	t.Parallel()
	ceOps := map[string]bool{"RegistryList": true}
	if got := editionOf("RegistryList", ceOps); got != edition.CE {
		t.Errorf("editionOf(RegistryList) = %v, want CE", got)
	}
}

func TestUnit_EditionOf_AbsentFromCE_IsEE(t *testing.T) {
	t.Parallel()
	ceOps := map[string]bool{"RegistryList": true} // does not include EcrDeleteRepository
	if got := editionOf("EcrDeleteRepository", ceOps); got != edition.EE {
		t.Errorf("editionOf(EcrDeleteRepository) = %v, want EE", got)
	}
}

// TestUnit_EditionOf_MatchesRealPilotEditionsAcrossBothVendoredSpecs checks
// editionOf against the real CE and EE specifications for every operationId
// the three pilot domains declare an Edition for (excluding SystemUpgrade,
// which does not exist in the EE specification at all — see this file's own
// editionOf doc comment — and is therefore never something this generator's
// EE-driven domain processing computes an Edition for).
func TestUnit_EditionOf_MatchesRealPilotEditionsAcrossBothVendoredSpecs(t *testing.T) {
	t.Parallel()
	ceOps := ceOperationIDsForTest(t)

	for _, tc := range []struct {
		operationID string
		want        edition.Edition
	}{
		{"TagList", edition.CE},
		{"TagCreate", edition.CE},
		{"TagDelete", edition.CE},
		{"SystemInfo", edition.CE},
		{"SystemStatus", edition.CE},
		{"SystemVersion", edition.CE},
		{"SystemNodesCount", edition.CE},
		{"SystemUpdate", edition.EE}, // EE-only in the real spec, matching system.go's hand-written Edition
		{"RegistryList", edition.CE},
		{"RegistryCreate", edition.CE},
		{"RegistryPing", edition.CE},
		{"RegistryInspect", edition.CE},
		{"RegistryUpdate", edition.CE},
		{"RegistryConfigure", edition.CE},
		{"RegistryDelete", edition.CE},
		{"EcrDeleteRepository", edition.EE},
		{"EcrDeleteTags", edition.EE},
		{"RepositoryTagsDelete", edition.EE},
	} {
		if got := editionOf(tc.operationID, ceOps); got != tc.want {
			t.Errorf("editionOf(%q, <real CE spec>) = %v, want %v (matching the hand-written pilot)", tc.operationID, got, tc.want)
		}
	}
}

// TestUnit_SuspectDangerMismatch_RealKnownCases pins suspectDangerMismatch
// against real operations from the vendored specifications: the documented
// system.upgrade precedent, a representative Kubernetes bulk delete, and
// three operations that must NOT be flagged — a GET whose path mentions
// "upgrade" (genuinely read-only), a DELETE (already Destructive, flagging
// it would be noise) and a POST that neither deletes nor is irreversible
// (registries.configure).
func TestUnit_SuspectDangerMismatch_RealKnownCases(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		op   operation
		want bool
	}{
		{
			name: "system.upgrade: POST, CE-only, irreversible edition conversion",
			op:   operation{Method: "POST", Path: "/system/upgrade", OperationID: "SystemUpgrade"},
			want: true,
		},
		{
			name: "Kubernetes bulk delete: POST verb, path says delete",
			op:   operation{Method: "POST", Path: "/kubernetes/{id}/cron_jobs/delete", OperationID: "DeleteCronJobs"},
			want: true,
		},
		{
			name: "endpoints bulk delete: POST verb, operationId says delete",
			op:   operation{Method: "POST", Path: "/endpoints/delete", OperationID: "EndpointDeleteBatch"},
			want: true,
		},
		{
			name: "omni upgrade status: GET, read-only regardless of the word \"upgrade\"",
			op:   operation{Method: "GET", Path: "/omni/{credentialID}/cluster/{name}/upgrade/status/k8s", OperationID: "OmniGetK8sVersionUpgradeStatus"},
			want: false,
		},
		{
			name: "tags.delete: DELETE verb already Destructive, not a mismatch",
			op:   operation{Method: "DELETE", Path: "/tags/{id}", OperationID: "TagDelete"},
			want: false,
		},
		{
			name: "registries.configure: POST, neither deletes nor is irreversible",
			op:   operation{Method: "POST", Path: "/registries/{id}/configure", OperationID: "RegistryConfigure"},
			want: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := suspectDangerMismatch(tc.op); got != tc.want {
				t.Errorf("suspectDangerMismatch(%+v) = %v, want %v", tc.op, got, tc.want)
			}
		})
	}
}

// TestUnit_DangerMismatchWarnings_AcrossBothVendoredSpecs_MatchesTheMeasuredSet
// runs dangerMismatchWarnings against every operation in the union of both
// real vendored specifications and pins the exact result: 17 operations,
// including system.upgrade and every Kubernetes bulk delete named in the
// task-3 report. Asserting the precise set (not merely a count) is what a
// mutation loosening or tightening dangerKeywords would actually be caught
// by — a bare length check would survive swapping which 17 operations
// matched.
func TestUnit_DangerMismatchWarnings_AcrossBothVendoredSpecs_MatchesTheMeasuredSet(t *testing.T) {
	t.Parallel()
	var all []operation
	for _, specPath := range []string{"../../api/specs/ee-2.44.0.json", "../../api/specs/ce-2.44.0.json"} {
		_, paths, err := loadDocument(specPath)
		if err != nil {
			t.Fatalf("loadDocument(%s) error = %v", specPath, err)
		}
		byTag, err := operationsByDomain(paths)
		if err != nil {
			t.Fatalf("operationsByDomain(%s) error = %v", specPath, err)
		}
		for _, ops := range byTag {
			all = append(all, ops...)
		}
	}

	warnings := dangerMismatchWarnings(all)

	// Both specs declare some operations in common (e.g. RegistryConfigure),
	// so de-duplicate by operationId before counting: dangerMismatchWarnings
	// itself has no way to know the two calls above cover overlapping
	// operations, and neither should it. This is computed independently,
	// straight off suspectDangerMismatch, rather than by reparsing warnings'
	// own string shape — the point is to check dangerMismatchWarnings'
	// underlying behaviour, not its formatting.
	seen := map[string]bool{}
	var wantIDs []string
	for _, op := range all {
		if suspectDangerMismatch(op) && !seen[op.OperationID] {
			seen[op.OperationID] = true
			wantIDs = append(wantIDs, op.OperationID)
		}
	}
	sort.Strings(wantIDs)
	wantSet := seen

	if len(wantIDs) != 17 {
		t.Fatalf("measured %d distinct flagged operationIds across both vendored specs, want 17 (see task-3 report for the enumerated list); got %v", len(wantIDs), wantIDs)
	}
	for _, must := range []string{
		"SystemUpgrade", "DeleteCronJobs", "DeleteJobs", "DeleteKubernetesIngresses",
		"DeleteKubernetesServices", "DeleteRoleBindings", "DeleteRoles", "DeleteServiceAccounts",
		"DeleteClusterRoleBindings", "DeleteClusterRoles", "DeleteKubernetesPersistentVolumeClaims",
		"DeleteKubernetesPersistentVolumes", "DeleteKubernetesStorageClasses",
		"EndpointDeleteBatch", "EndpointAssociationDelete",
	} {
		if !wantSet[must] {
			t.Errorf("flagged set is missing %q, want it present (measured directly against the real specs)", must)
		}
	}
	if len(warnings) == 0 {
		t.Fatal("dangerMismatchWarnings() returned nothing, want the measured 17-operation set rendered as report lines")
	}
}

// TestUnit_BuildActionSpecFields_ReproducesPilotDomains is Task 3's own
// Step 5: build the mechanical ActionSpec fields for every one of the 18
// pilot operations this generator's EE-driven processing actually reaches
// (SystemUpgrade is the 19th; it does not exist in the EE specification —
// see editionOf's doc comment — so it is not exercised here, exactly as it
// is never processed by main.go's real run today) and compare Domain,
// OperationID, Edition, Mutating and Destructive against what
// internal/tools/{tags,system,registries} declare by hand.
//
// Idempotent is asserted too, but against dangerFlags' own rule rather than
// the pilots' hand-written value: none of the 19 pilot ActionSpec literals
// set Idempotent at all (grep confirms it — the field was added to
// toolutil.ActionSpec after these three domains were written, and nobody
// backfilled it), so every mutating pilot action's hand-written Idempotent
// is the zero value, false, even tags.delete and registries.update, which
// are genuinely idempotent by the brief's own DELETE/PUT rule. That is a
// real gap in the pilots this generator closes, not a divergence to
// reconcile in the rule — see the task-3 report for the full field-by-field
// verdict.
func TestUnit_BuildActionSpecFields_ReproducesPilotDomains(t *testing.T) {
	t.Parallel()
	ceOps := ceOperationIDsForTest(t)

	for _, tc := range []struct {
		domain, operationID           string
		method                        string
		wantEdition                   edition.Edition
		wantMutating, wantDestructive bool
		wantIdempotent                bool // this generator's own rule, not the (unset) pilot value
	}{
		{"tags", "TagList", "GET", edition.CE, false, false, false},
		{"tags", "TagCreate", "POST", edition.CE, true, false, false},
		{"tags", "TagDelete", "DELETE", edition.CE, true, true, true},
		{"system", "SystemInfo", "GET", edition.CE, false, false, false},
		{"system", "SystemStatus", "GET", edition.CE, false, false, false},
		{"system", "SystemVersion", "GET", edition.CE, false, false, false},
		{"system", "SystemNodesCount", "GET", edition.CE, false, false, false},
		{"system", "SystemUpdate", "POST", edition.EE, true, false, false},
		{"registries", "RegistryList", "GET", edition.CE, false, false, false},
		{"registries", "RegistryCreate", "POST", edition.CE, true, false, false},
		{"registries", "RegistryPing", "POST", edition.CE, true, false, false},
		{"registries", "RegistryInspect", "GET", edition.CE, false, false, false},
		{"registries", "RegistryUpdate", "PUT", edition.CE, true, false, true},
		{"registries", "RegistryConfigure", "POST", edition.CE, true, false, false},
		{"registries", "RegistryDelete", "DELETE", edition.CE, true, true, true},
		{"registries", "EcrDeleteRepository", "DELETE", edition.EE, true, true, true},
		{"registries", "EcrDeleteTags", "DELETE", edition.EE, true, true, true},
		{"registries", "RepositoryTagsDelete", "DELETE", edition.EE, true, true, true},
	} {
		op := operation{
			OperationID: tc.operationID,
			Method:      tc.method,
			Summary:     "placeholder summary", // Title/Description are not compared here; see the task-3 report
		}
		got, err := buildActionSpecFields(tc.domain, op, ceOps)
		if err != nil {
			t.Fatalf("buildActionSpecFields(%q, %q) error = %v", tc.domain, tc.operationID, err)
		}
		if got.Domain != tc.domain {
			t.Errorf("%s: Domain = %q, want %q", tc.operationID, got.Domain, tc.domain)
		}
		if got.OperationID != tc.operationID {
			t.Errorf("%s: OperationID = %q, want %q", tc.operationID, got.OperationID, tc.operationID)
		}
		if got.Edition != tc.wantEdition {
			t.Errorf("%s: Edition = %v, want %v (matching the hand-written pilot)", tc.operationID, got.Edition, tc.wantEdition)
		}
		if got.Mutating != tc.wantMutating {
			t.Errorf("%s: Mutating = %v, want %v (matching the hand-written pilot)", tc.operationID, got.Mutating, tc.wantMutating)
		}
		if got.Destructive != tc.wantDestructive {
			t.Errorf("%s: Destructive = %v, want %v (matching the hand-written pilot)", tc.operationID, got.Destructive, tc.wantDestructive)
		}
		if got.Idempotent != tc.wantIdempotent {
			t.Errorf("%s: Idempotent = %v, want %v (this generator's own verb rule — the pilot never set this field)", tc.operationID, got.Idempotent, tc.wantIdempotent)
		}
	}
}

// TestUnit_BuildActionSpecFields_SystemUpgrade_WouldDivergeFromTheHandOverride
// is the known case the brief calls out by name: were SystemUpgrade ever fed
// through buildActionSpecFields (it is not, today, because it does not
// exist in the EE specification this generator's domain processing
// enumerates from), the mechanical POST-verb rule would compute
// Destructive: false — disagreeing with system.go's hand-written
// Destructive: true, which exists precisely because converting a Community
// instance to Business Edition cannot be undone. This test proves the
// disagreement is real (not assumed) and, together with
// TestUnit_SuspectDangerMismatch_RealKnownCases's system.upgrade case,
// proves this generator surfaces the need for that override via
// suspectDangerMismatch rather than silently producing a wrong Destructive
// value for it.
func TestUnit_BuildActionSpecFields_SystemUpgrade_WouldDivergeFromTheHandOverride(t *testing.T) {
	t.Parallel()
	op := operation{OperationID: "SystemUpgrade", Method: "POST", Summary: "Upgrade to Business Edition"}
	got, err := buildActionSpecFields("system", op, map[string]bool{})
	if err != nil {
		t.Fatalf("buildActionSpecFields() error = %v", err)
	}
	if got.Destructive {
		t.Fatal("buildActionSpecFields() computed Destructive: true from the POST verb alone; the whole point of this test is that the mechanical rule does NOT — update this test's expectation and re-check the surfacing mechanism if that ever changes")
	}
	if !suspectDangerMismatch(op) {
		t.Error("suspectDangerMismatch() = false for SystemUpgrade, want true: this is exactly the mismatch that must surface the hand override rather than silently accepting Destructive: false")
	}
}

// TestUnit_ApplyNarrative_HookSuppliesFields proves the hook mechanism
// actually wires narrative fields through, rather than applyNarrative
// silently ignoring whatever hook returns.
func TestUnit_ApplyNarrative_HookSuppliesFields(t *testing.T) {
	t.Parallel()
	hook := func(operationID string) actionNarrative {
		if operationID != "TagCreate" {
			return actionNarrative{}
		}
		return actionNarrative{
			Usage:          "Prefer this over a raw API call when tagging environments interactively.",
			RelatedActions: []string{"tags.list", "tags.delete"},
			Aliases:        []string{"create-tag"},
			Tags:           []string{"lifecycle"},
			ParameterGuidance: map[string]toolutil.ParameterGuidance{
				"name": {SemanticRole: "The tag's display name."},
			},
		}
	}

	got := applyNarrative(actionSpecFields{OperationID: "TagCreate", Name: "tags.create"}, hook)
	if got.Usage == "" || len(got.RelatedActions) != 2 || len(got.Aliases) != 1 || len(got.Tags) != 1 || len(got.ParameterGuidance) != 1 {
		t.Errorf("applyNarrative() = %+v, want every narrative field populated from the hook", got)
	}
}

// TestUnit_ApplyNarrative_NilHook_EmptyNarrative is the default a domain
// with nothing hand-authored yet gets: no hook, no narrative, but the
// mechanical fields still come through untouched.
func TestUnit_ApplyNarrative_NilHook_EmptyNarrative(t *testing.T) {
	t.Parallel()
	got := applyNarrative(actionSpecFields{OperationID: "TagList", Name: "tags.list"}, nil)
	if got.Usage != "" || got.RelatedActions != nil || got.Aliases != nil || got.Tags != nil || got.ParameterGuidance != nil {
		t.Errorf("applyNarrative(nil hook) = %+v, want every narrative field at its zero value", got)
	}
	if got.Name != "tags.list" {
		t.Errorf("applyNarrative(nil hook) lost the mechanical Name field: got %q", got.Name)
	}
}

// TestUnit_ApplyNarrative_RegeneratingMechanicalFieldsLeavesHookAuthoredTextUntouched
// is the property this task's brief exists to prove: "regenerating never
// discards authored text". It simulates exactly what a real regeneration
// does — recomputing actionSpecFields' mechanical fields from the
// specification (here, changing Title, as if the spec's own summary had
// been edited) — while hook, standing in for a domain's hand-written file,
// is never touched at all. If applyNarrative's narrative fields ever
// depended on anything other than calling hook, this would be the test that
// catches it: change hook's returned Usage and this test must go red;
// change fields' mechanical values and it must not.
func TestUnit_ApplyNarrative_RegeneratingMechanicalFieldsLeavesHookAuthoredTextUntouched(t *testing.T) {
	t.Parallel()
	hook := func(operationID string) actionNarrative {
		if operationID != "TagCreate" {
			return actionNarrative{}
		}
		return actionNarrative{Usage: "authored by a human, once", Tags: []string{"lifecycle"}}
	}

	before := actionSpecFields{OperationID: "TagCreate", Name: "tags.create", Title: "Create a tag"}
	gotBefore := applyNarrative(before, hook)

	// The "regeneration": every mechanical field changes, hook does not.
	after := before
	after.Title = "Create a new environment tag" // simulates a spec summary edit
	after.Description = "regenerated description text"
	gotAfter := applyNarrative(after, hook)

	if gotAfter.Usage != gotBefore.Usage {
		t.Errorf("Usage changed across a mechanical-only regeneration: before %q, after %q — narrative text must survive untouched", gotBefore.Usage, gotAfter.Usage)
	}
	if len(gotAfter.Tags) != 1 || gotAfter.Tags[0] != "lifecycle" {
		t.Errorf("Tags = %v after regeneration, want [\"lifecycle\"] preserved from the hook", gotAfter.Tags)
	}
	if gotAfter.Title == gotBefore.Title {
		t.Fatal("test setup is broken: Title did not actually change across the simulated regeneration, so this test proves nothing about mechanical fields moving while narrative fields hold still")
	}

	// The converse: change what the hook itself returns (the hand file being
	// edited by its domain author) and the narrative output must move,
	// proving applyNarrative is not simply repeating a cached value.
	editedHook := func(operationID string) actionNarrative {
		if operationID != "TagCreate" {
			return actionNarrative{}
		}
		return actionNarrative{Usage: "the author revised this sentence", Tags: []string{"lifecycle", "provisioning"}}
	}
	gotEdited := applyNarrative(after, editedHook)
	if gotEdited.Usage == gotAfter.Usage {
		t.Error("editing the hook did not change Usage: applyNarrative must call the hook, not memoise its previous return value")
	}
	if len(gotEdited.Tags) != 2 {
		t.Errorf("Tags = %v after editing the hook, want the hook's updated 2-element list", gotEdited.Tags)
	}
}
