package main

import (
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// action is a small constructor for a toolutil.ActionSpec carrying only the
// fields clusterActions and needsAttention read (Name, Domain, Usage), so
// tests stay focused on the clustering logic rather than on satisfying
// ActionSpec.Validate.
func action(name, domain, usage string) toolutil.ActionSpec {
	return toolutil.ActionSpec{Name: name, Domain: domain, Usage: usage}
}

// TestUnit_NeedsAttention_IdenticalUsage_Flagged is the brief's first
// required case: two actions in the same cluster whose Usage text reads
// exactly the same cannot help a model tell them apart, and must be flagged
// — regardless of whether that shared text is empty or a real (but
// duplicated) sentence.
func TestUnit_NeedsAttention_IdenticalUsage_Flagged(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"both empty":    "",
		"both the same": "Use this to inspect one resource by id.",
	}
	for name, usage := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := cluster{
				Key: clusterKey{Domain: "widgets", Base: "list"},
				Actions: []toolutil.ActionSpec{
					action("widgets.list", "widgets", usage),
					action("widgets.list_all", "widgets", usage),
				},
			}
			groups := needsAttention(c)
			if len(groups) != 1 {
				t.Fatalf("needsAttention() = %d groups, want 1 (identical Usage must be flagged): %+v", len(groups), groups)
			}
			if len(groups[0].Actions) != 2 {
				t.Errorf("flagged group has %d actions, want both: %+v", len(groups[0].Actions), groups[0])
			}
		})
	}
}

// TestUnit_NeedsAttention_DistinguishingUsage_NotFlagged is the brief's
// second required case: two actions in the same cluster whose Usage text
// genuinely differs already tells a model them apart and must not be
// flagged.
func TestUnit_NeedsAttention_DistinguishingUsage_NotFlagged(t *testing.T) {
	t.Parallel()
	c := cluster{
		Key: clusterKey{Domain: "widgets", Base: "list"},
		Actions: []toolutil.ActionSpec{
			action("widgets.list", "widgets", "Lists only widgets owned by the caller."),
			action("widgets.list_all", "widgets", "Lists every widget across every owner; requires an administrator token."),
		},
	}
	groups := needsAttention(c)
	if len(groups) != 0 {
		t.Errorf("needsAttention() = %+v, want no groups: the two actions' Usage text genuinely differs", groups)
	}
}

// TestUnit_NeedsAttention_Singleton_NeverFlagged is the brief's third
// required case: a cluster with exactly one member is never flagged, empty
// Usage or not, because there is no sibling to disambiguate it from.
func TestUnit_NeedsAttention_Singleton_NeverFlagged(t *testing.T) {
	t.Parallel()
	c := cluster{
		Key:     clusterKey{Domain: "widgets", Base: "list"},
		Actions: []toolutil.ActionSpec{action("widgets.list", "widgets", "")},
	}
	if groups := needsAttention(c); groups != nil {
		t.Errorf("needsAttention() = %+v, want nil for a singleton cluster", groups)
	}
}

// TestUnit_NeedsAttention_ThreeMembersOneDistinct_OnlyTheDuplicatePairFlagged
// guards against a cluster-level yes/no that would either flag a member with
// genuinely distinguishing text just because it shares a cluster with a
// duplicate pair, or, in the other direction, let a real duplicate pair hide
// behind one distinct sibling.
func TestUnit_NeedsAttention_ThreeMembersOneDistinct_OnlyTheDuplicatePairFlagged(t *testing.T) {
	t.Parallel()
	c := cluster{
		Key: clusterKey{Domain: "widgets", Base: "list"},
		Actions: []toolutil.ActionSpec{
			action("widgets.list", "widgets", "same text"),
			action("widgets.list_all", "widgets", "same text"),
			action("widgets.list_mine", "widgets", "Lists only widgets owned by the caller."),
		},
	}
	groups := needsAttention(c)
	if len(groups) != 1 {
		t.Fatalf("needsAttention() = %d groups, want exactly 1: %+v", len(groups), groups)
	}
	got := append([]string(nil), groups[0].Actions...)
	want := []string{"widgets.list", "widgets.list_all"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("flagged group = %v, want %v", got, want)
	}
}

// TestUnit_BaseName documents the stripping rule with concrete examples,
// including the real pilot-domain names it must group or keep apart.
func TestUnit_BaseName(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, domain, want string
	}{
		{"widgets.list", "widgets", "list"},
		{"widgets.list_all", "widgets", "list"},
		{"widgets.list_mine", "widgets", "list"},
		{"registries.delete", "registries", "delete"},
		{"registries.repository_tags_delete", "registries", "delete"},
		// ecr_delete_repository and ecr_delete_tags share the recognised verb
		// "delete", but only in the middle of the name — neither the leading
		// nor the trailing token is a recognised CRUD verb or variant suffix
		// ("ecr" and "repository"/"tags" are resource nouns, not operations)
		// — so each keeps its own full name and neither clusters with plain
		// "delete".
		{"registries.ecr_delete_repository", "registries", "ecr_delete_repository"},
		{"registries.ecr_delete_tags", "registries", "ecr_delete_tags"},
		{"system.info", "system", "info"},
		{"system.status", "system", "status"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := baseName(tt.name, tt.domain); got != tt.want {
				t.Errorf("baseName(%q, %q) = %q, want %q", tt.name, tt.domain, got, tt.want)
			}
		})
	}
}

// TestUnit_ClusterActions_GroupsSiblingsAcrossTheRealPilotNamingPattern
// proves the clustering mechanism itself actually groups something, using
// the real registries domain's own action names rather than a fixture
// engineered to succeed. This is the check the brief warns is easy to get
// wrong: a clusterActions that never groups anything would make every
// downstream test of needsAttention vacuously pass (nothing ever reaches a
// multi-member cluster to be flagged or not), so this test fails loudly if
// that regresses, independent of whether any of the resulting clusters also
// need attention.
func TestUnit_ClusterActions_GroupsSiblingsAcrossTheRealPilotNamingPattern(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{
		action("registries.list", "registries", ""),
		action("registries.create", "registries", ""),
		action("registries.delete", "registries", ""),
		action("registries.ecr_delete_repository", "registries", ""),
		action("registries.ecr_delete_tags", "registries", ""),
		action("registries.repository_tags_delete", "registries", ""),
	}

	clusters := clusterActions(actions)

	var deleteCluster *cluster
	for i := range clusters {
		if clusters[i].Key.Domain == "registries" && clusters[i].Key.Base == "delete" {
			deleteCluster = &clusters[i]
		}
	}
	if deleteCluster == nil {
		t.Fatalf("clusterActions(%v) produced no (registries, \"delete\") cluster; clustering is not grouping anything: %+v", actions, clusters)
	}
	if len(deleteCluster.Actions) != 2 {
		t.Fatalf("(registries, \"delete\") cluster has %d members, want 2 (registries.delete, registries.repository_tags_delete): %+v",
			len(deleteCluster.Actions), deleteCluster.Actions)
	}
	names := []string{deleteCluster.Actions[0].Name, deleteCluster.Actions[1].Name}
	if names[0] != "registries.delete" || names[1] != "registries.repository_tags_delete" {
		t.Errorf("(registries, \"delete\") cluster members = %v, want [registries.delete registries.repository_tags_delete]", names)
	}

	// registries.create and registries.list have no sibling with the same
	// base — each must remain a singleton, or the clustering is too coarse
	// (e.g. collapsing an entire domain into one bucket) rather than
	// genuinely grouping only same-family variants.
	for _, base := range []string{"list", "create"} {
		for i := range clusters {
			if clusters[i].Key.Domain == "registries" && clusters[i].Key.Base == base && len(clusters[i].Actions) != 1 {
				t.Errorf("(registries, %q) cluster has %d members, want 1 (singleton): %+v", base, len(clusters[i].Actions), clusters[i].Actions)
			}
		}
	}
}

// TestUnit_ClusterActions_DomainBoundary asserts that two actions with an
// identical local name in different domains never cluster together — Domain
// is part of the cluster key, not just the base name.
func TestUnit_ClusterActions_DomainBoundary(t *testing.T) {
	t.Parallel()
	actions := []toolutil.ActionSpec{
		action("widgets.list", "widgets", ""),
		action("gadgets.list", "gadgets", ""),
	}
	clusters := clusterActions(actions)
	for _, c := range clusters {
		if len(c.Actions) != 1 {
			t.Errorf("cluster %+v has %d members, want singleton per domain", c.Key, len(c.Actions))
		}
	}
}

// TestUnit_BuildReport_NamesTheAttentionCountUnambiguously mirrors
// cmd/audit_e2e_gaps's own report test: asserting the tighter "N of M" phrase
// ties the check to the actual flagged/total relationship, not to a bare
// digit a report could satisfy by accident (a line number, a percentage, an
// unrelated count).
func TestUnit_BuildReport_NamesTheAttentionCountUnambiguously(t *testing.T) {
	t.Parallel()
	clusters := []cluster{
		{
			Key: clusterKey{Domain: "widgets", Base: "list"},
			Actions: []toolutil.ActionSpec{
				action("widgets.list", "widgets", "same"),
				action("widgets.list_all", "widgets", "same"),
			},
		},
		{
			Key: clusterKey{Domain: "widgets", Base: "create"},
			Actions: []toolutil.ActionSpec{
				action("widgets.create", "widgets", "Creates one widget."),
				action("widgets.create_bulk", "widgets", "Creates many widgets in one call."),
			},
		},
	}
	report := buildReport(clusters)
	if !strings.Contains(report, "1 of 2") {
		t.Errorf("report does not state how many sibling clusters need attention:\n%s", report)
	}
	if !strings.Contains(report, "widgets.list") || !strings.Contains(report, "widgets.list_all") {
		t.Errorf("report does not name the flagged cluster's members:\n%s", report)
	}
	if strings.Contains(report, "widgets.create_bulk share identical") {
		t.Errorf("report flagged the distinguishing-text cluster:\n%s", report)
	}
}

// TestUnit_BuildReport_NoAttentionNeeded_StatesSoExplicitly guards the other
// direction: an all-clear report must say so, not merely omit a section a
// reader could mistake for "the tool did not run this far".
func TestUnit_BuildReport_NoAttentionNeeded_StatesSoExplicitly(t *testing.T) {
	t.Parallel()
	clusters := []cluster{
		{
			Key: clusterKey{Domain: "widgets", Base: "list"},
			Actions: []toolutil.ActionSpec{
				action("widgets.list", "widgets", "Lists only widgets owned by the caller."),
				action("widgets.list_all", "widgets", "Lists every widget; requires an administrator token."),
			},
		},
	}
	report := buildReport(clusters)
	if !strings.Contains(report, "0 of 1") {
		t.Errorf("report does not state zero clusters needing attention:\n%s", report)
	}
	if !strings.Contains(report, "distinguishing Usage text") {
		t.Errorf("report does not explicitly state the all-clear result:\n%s", report)
	}
}

// TestUnit_Run_RealCatalog_FlagsTheGenuineRegistriesDeleteGap is Step 5 of
// the brief run against the actual code, not a synthetic fixture: it builds
// the real catalog (system, tags, registries — the three pilot domains) and
// asserts the tool surfaces the one real gap that exists in it today.
// registries.delete and registries.repository_tags_delete share the "delete"
// base (see TestUnit_BaseName) and neither has Usage text yet — Task 2 added
// the field, but no domain has been given narrative Usage yet, that is what
// this whole backlog is for — so their Usage is identical (both empty) and
// this cluster must be flagged. This is the concrete answer to the brief's
// "if it flags nothing, find out why" instruction: it does not flag nothing,
// and this test is how that was confirmed rather than assumed.
func TestUnit_Run_RealCatalog_FlagsTheGenuineRegistriesDeleteGap(t *testing.T) {
	t.Parallel()
	catalog, err := actioncatalog.Build(allSpecs(), actioncatalog.Options{Edition: edition.EE})
	if err != nil {
		t.Fatalf("actioncatalog.Build: %v", err)
	}
	clusters := clusterActions(catalog.Actions())

	var deleteCluster *cluster
	for i := range clusters {
		if clusters[i].Key.Domain == "registries" && clusters[i].Key.Base == "delete" {
			deleteCluster = &clusters[i]
		}
	}
	if deleteCluster == nil {
		t.Fatal("the real catalog no longer produces a (registries, \"delete\") cluster; this test's premise is stale")
	}
	groups := needsAttention(*deleteCluster)
	if len(groups) == 0 {
		t.Fatalf("(registries, \"delete\") cluster was not flagged; expected registries.delete and "+
			"registries.repository_tags_delete to share empty Usage text: %+v", deleteCluster.Actions)
	}

	var out strings.Builder
	out.WriteString(buildReport(clusters))
	if !strings.Contains(out.String(), "registries.delete") || !strings.Contains(out.String(), "registries.repository_tags_delete") {
		t.Errorf("report does not name the flagged pair:\n%s", out.String())
	}
}

// TestUnit_Run_WritesToTheGivenWriter checks run's plumbing: it builds the
// catalog and writes a non-empty report to whatever writer it is given,
// returning no error on the success path.
func TestUnit_Run_WritesToTheGivenWriter(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	if err := run(&out); err != nil {
		t.Fatalf("run: unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), "Portainer MCP discovery audit") {
		t.Errorf("run did not write the expected report header:\n%s", out.String())
	}
}
