package main

import (
	"os"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// The real vendored documents, relative to this command's own directory.
const (
	realCESpec = "../../api/specs/ce-2.44.0.json"
	realEESpec = "../../api/specs/ee-2.44.0.json"
)

// loadRealSpecs parses both committed documents, so a test can check the real
// alias table against the real operations rather than against a fixture.
func loadRealSpecs(t *testing.T) (ce, ee map[string]specOperation) {
	t.Helper()
	load := func(path string) map[string]specOperation {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		ops, err := parseSpecOperations(data)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		return ops
	}
	return load(realCESpec), load(realEESpec)
}

// webhookRoute is the one route the two vendored documents name differently,
// as each declares it. Built here rather than read from the documents in the
// synthetic tests below so those tests state the shape they depend on.
func webhookRoute(operationID string) specOperation {
	return op(operationID, "POST", "/stacks/webhooks/{webhookID}", "stacks")
}

// TestUnit_OperationAliases_HoldAgainstTheVendoredDocuments is what keeps the
// real table honest, and it is the reason run and runRatchet take their
// aliases as a parameter rather than reading operationAliases directly: those
// two are driven by tests over two-operation fixture specs, where the real
// table would be stale by construction. Nothing else would ever check the
// entries this command actually ships with.
//
// It is the same guarantee cmd/gen_action_inputs's
// TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation gives that
// generator's own correction table: a table stating facts about the vendored
// documents is only worth having while something re-derives those facts.
func TestUnit_OperationAliases_HoldAgainstTheVendoredDocuments(t *testing.T) {
	t.Parallel()
	ce, ee := loadRealSpecs(t)

	if len(operationAliases) == 0 {
		t.Fatal("operationAliases is empty; this test would then assert nothing at all")
	}

	// Driven by the table rather than checking it in one call, so a stale
	// entry names itself instead of arriving inside one aggregate error. The
	// whole table is then re-checked together, because checkAliases also
	// rejects a pair that collides with another pair — a property no single
	// entry can carry on its own.
	for _, alias := range operationAliases {
		t.Run(alias.Business+"/"+alias.Community, func(t *testing.T) {
			if err := checkAliases([]operationAlias{alias}, ce, ee); err != nil {
				t.Errorf("alias %s/%s no longer names one route under two names: %v", alias.Business, alias.Community, err)
			}
		})
	}

	t.Run("the table as a whole", func(t *testing.T) {
		if err := checkAliases(operationAliases, ce, ee); err != nil {
			t.Errorf("checkAliases(operationAliases) = %v, want nil", err)
		}
	})
}

// TestUnit_AuditCoverage_CoveringEitherNameOfAnAliasedRoute_CoversBoth is the
// property the whole mechanism exists for.
//
// POST /stacks/webhooks/{webhookID} is one route the two documents name
// differently — StacksWebhookInvoke in Business Edition, WebhookInvoke in
// Community Edition — and the catalog, generated from the Business Edition
// document, can only ever declare the Business Edition spelling. Without the
// alias the Community Edition name is uncovered forever on a route the server
// exposes; the third case below is what states that, by running the identical
// inputs with no alias table at all.
//
// The unrelated Community-Edition-only operation in every case is the control.
// GetKubernetesConfig has no Business Edition counterpart and no alias, and it
// must stay uncovered throughout: an implementation that simply stopped
// reporting Community Edition gaps would satisfy every other assertion here.
func TestUnit_AuditCoverage_CoveringEitherNameOfAnAliasedRoute_CoversBoth(t *testing.T) {
	t.Parallel()

	ce := map[string]specOperation{
		"WebhookInvoke":       webhookRoute("WebhookInvoke"),
		"GetKubernetesConfig": op("GetKubernetesConfig", "GET", "/kubernetes/config", "kubernetes"),
	}
	ee := map[string]specOperation{
		"StacksWebhookInvoke": webhookRoute("StacksWebhookInvoke"),
	}
	aliases := []operationAlias{{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason:    "one route, renamed between the two documents",
		Added:     "2026-08-18",
	}}

	tests := []struct {
		name          string
		actions       []toolutil.ActionSpec
		aliases       []operationAlias
		wantUncovered []string
	}{
		{
			name:    "the Business Edition name is covered and the alias carries it to the Community Edition one",
			actions: []toolutil.ActionSpec{action("stacks.webhook_invoke", "StacksWebhookInvoke")},
			aliases: aliases,
			// GetKubernetesConfig and nothing else: WebhookInvoke is covered
			// through its alias.
			wantUncovered: []string{"GetKubernetesConfig"},
		},
		{
			name:          "and the other way round, so the alias is a relation and not a one-way rewrite",
			actions:       []toolutil.ActionSpec{action("stacks.webhook_invoke", "WebhookInvoke")},
			aliases:       aliases,
			wantUncovered: []string{"GetKubernetesConfig"},
		},
		{
			name:    "without the alias the Community Edition name is reported uncovered",
			actions: []toolutil.ActionSpec{action("stacks.webhook_invoke", "StacksWebhookInvoke")},
			aliases: nil,
			// This is the blindness the alias fixes, stated as a passing
			// assertion: remove the table and the gap comes back.
			wantUncovered: []string{"GetKubernetesConfig", "WebhookInvoke"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := auditCoverage(ce, ee, tt.actions, nil, tt.aliases)
			if err != nil {
				t.Fatalf("auditCoverage() error = %v", err)
			}
			var uncovered []string
			for _, o := range result.CE.Uncovered {
				uncovered = append(uncovered, o.OperationID)
			}
			if strings.Join(uncovered, ",") != strings.Join(tt.wantUncovered, ",") {
				t.Errorf("CE uncovered = %v, want %v", uncovered, tt.wantUncovered)
			}
			if len(result.EE.Uncovered) != 0 {
				t.Errorf("EE uncovered = %v, want none", result.EE.Uncovered)
			}
		})
	}
}

// TestUnit_AuditCoverage_AllowListedRouteIsExcusedUnderBothNames pins the
// second place the alias applies. An allow-list entry excuses a route, and a
// route does not stop being excused because the other document spells it
// differently — treating the two sets differently would leave the same
// blindness in the half of the mechanism nobody looked at.
func TestUnit_AuditCoverage_AllowListedRouteIsExcusedUnderBothNames(t *testing.T) {
	t.Parallel()

	ce := map[string]specOperation{"WebhookInvoke": webhookRoute("WebhookInvoke")}
	ee := map[string]specOperation{"StacksWebhookInvoke": webhookRoute("StacksWebhookInvoke")}
	allowList := []allowListEntry{{
		OperationID: "StacksWebhookInvoke",
		Reason:      "hypothetical: this test needs an entry, not a real exclusion",
		Added:       "2026-08-18",
	}}
	aliases := []operationAlias{{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason:    "one route, renamed between the two documents",
		Added:     "2026-08-18",
	}}

	result, err := auditCoverage(ce, ee, nil, allowList, aliases)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}
	if result.HasGap() {
		t.Fatalf("HasGap() = true, want false: %v / %v", result.CE.Uncovered, result.EE.Uncovered)
	}
	if len(result.CE.AllowListed) != 1 || result.CE.AllowListed[0].OperationID != "WebhookInvoke" {
		t.Errorf("CE allow-listed = %v, want exactly [WebhookInvoke]", result.CE.AllowListed)
	}
}

// TestUnit_BuildReport_NamesEveryAlias pins that the mechanism appears in the
// report. An alias makes one operationId count as covered because another is,
// and nothing in the coverage numbers alone would ever say so — the same
// argument that makes the allow-list count unconditional.
func TestUnit_BuildReport_NamesEveryAlias(t *testing.T) {
	t.Parallel()

	ce := map[string]specOperation{"WebhookInvoke": webhookRoute("WebhookInvoke")}
	ee := map[string]specOperation{"StacksWebhookInvoke": webhookRoute("StacksWebhookInvoke")}
	aliases := []operationAlias{{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason:    "one route, renamed between the two documents",
		Added:     "2026-08-18",
	}}

	result, err := auditCoverage(ce, ee, []toolutil.ActionSpec{action("stacks.webhook_invoke", "StacksWebhookInvoke")}, nil, aliases)
	if err != nil {
		t.Fatalf("auditCoverage() error = %v", err)
	}
	report := buildReport(result)
	for _, want := range []string{"Operation aliases:  1", "StacksWebhookInvoke (EE) = WebhookInvoke (CE)"} {
		if !strings.Contains(report, want) {
			t.Errorf("buildReport() does not contain %q:\n%s", want, report)
		}
	}
}

// TestUnit_CheckAliases_RefusesAnEntryThatNoLongerHolds covers every way an
// alias can decay, because an alias nobody re-checks is the failure mode both
// allow-list files were built to avoid: it keeps applying after the fact it
// asserted stopped being true, and a later, unrelated operation can inherit
// the forgiveness.
//
// The two rows that matter most are the last two. "Declared in both
// documents" is what makes aliasing safe at all — an entry is only accepted
// when no single document uses the two names together, so an alias can never
// mark a second, genuinely distinct operation covered. "Different routes" is
// the case where the ids still resolve and the entry is nonetheless a lie.
func TestUnit_CheckAliases_RefusesAnEntryThatNoLongerHolds(t *testing.T) {
	t.Parallel()

	valid := operationAlias{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason:    "one route, renamed between the two documents",
		Added:     "2026-08-18",
	}
	ce := map[string]specOperation{"WebhookInvoke": webhookRoute("WebhookInvoke")}
	ee := map[string]specOperation{"StacksWebhookInvoke": webhookRoute("StacksWebhookInvoke")}

	withEntry := func(mutate func(a *operationAlias)) []operationAlias {
		entry := valid
		mutate(&entry)
		return []operationAlias{entry}
	}

	tests := []struct {
		name    string
		aliases []operationAlias
		ce, ee  map[string]specOperation
		want    string
	}{
		{
			name:    "the entry that holds",
			aliases: []operationAlias{valid},
			ce:      ce, ee: ee,
			want: "",
		},
		{
			name:    "no reason",
			aliases: withEntry(func(a *operationAlias) { a.Reason = "" }),
			ce:      ce, ee: ee,
			want: "reason is required",
		},
		{
			name:    "no date",
			aliases: withEntry(func(a *operationAlias) { a.Added = "" }),
			ce:      ce, ee: ee,
			want: "added date is required",
		},
		{
			name:    "a date that is not an ISO date",
			aliases: withEntry(func(a *operationAlias) { a.Added = "18 August 2026" }),
			ce:      ce, ee: ee,
			want: "is not an ISO date",
		},
		{
			name:    "an operationId aliased to itself",
			aliases: withEntry(func(a *operationAlias) { a.Community = a.Business }),
			ce:      ce, ee: ee,
			want: "is aliased to itself",
		},
		{
			name: "one operationId in two entries",
			aliases: []operationAlias{valid, {
				Business:  "StacksWebhookInvoke",
				Community: "SomeOtherName",
				Reason:    "a second claim on the same name",
				Added:     "2026-08-18",
			}},
			ce: ce, ee: ee,
			want: "may name at most one route",
		},
		{
			name:    "the Business Edition name is gone from the Business Edition document",
			aliases: []operationAlias{valid},
			ce:      ce, ee: map[string]specOperation{},
			want: `"StacksWebhookInvoke" is not declared in the Business Edition document`,
		},
		{
			name:    "the Community Edition name is gone from the Community Edition document",
			aliases: []operationAlias{valid},
			ce:      map[string]specOperation{}, ee: ee,
			want: `"WebhookInvoke" is not declared in the Community Edition document`,
		},
		{
			name:    "both documents now declare the Business Edition name, so they are not one route under two names",
			aliases: []operationAlias{valid},
			ce: map[string]specOperation{
				"WebhookInvoke":       webhookRoute("WebhookInvoke"),
				"StacksWebhookInvoke": op("StacksWebhookInvoke", "POST", "/stacks/webhooks/other", "stacks"),
			},
			ee:   ee,
			want: "is declared in the Community Edition document too",
		},
		{
			name:    "the two ids no longer name the same route",
			aliases: []operationAlias{valid},
			ce:      map[string]specOperation{"WebhookInvoke": op("WebhookInvoke", "POST", "/webhooks/{id}", "webhooks")},
			ee:      ee,
			want:    "no longer the same route",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := checkAliases(tt.aliases, tt.ce, tt.ee)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("checkAliases() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkAliases() = nil error, want one naming %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("checkAliases() error = %v, want it to name %q", err, tt.want)
			}
		})
	}
}

// TestUnit_AuditCoverage_RefusesAStaleAlias proves the refusal is wired into
// the audit itself and not merely available to be called: a check nothing
// invokes gates nothing. It is checked before any coverage counting, like the
// allow-list's own staleness check, so a decayed alias fails the run rather
// than quietly changing what "covered" means.
func TestUnit_AuditCoverage_RefusesAStaleAlias(t *testing.T) {
	t.Parallel()

	ce := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	ee := map[string]specOperation{"TagList": op("TagList", "GET", "/tags", "tags")}
	aliases := []operationAlias{{
		Business:  "StacksWebhookInvoke",
		Community: "WebhookInvoke",
		Reason:    "neither name exists in these documents",
		Added:     "2026-08-18",
	}}

	_, err := auditCoverage(ce, ee, []toolutil.ActionSpec{action("tags.list", "TagList")}, nil, aliases)
	if err == nil {
		t.Fatal("auditCoverage() = nil error, want a refusal for the stale alias")
	}
	if !strings.Contains(err.Error(), "StacksWebhookInvoke") {
		t.Errorf("auditCoverage() error = %v, want it to name the stale entry", err)
	}
}
