package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// printActionNames, when true, makes TestUnit_ActionName_PrintAllGeneratedNames
// write every one of the 441 names this rule would mint to stderr — never
// stdout, which CI's grep enforces stays exclusive to the MCP transport. It
// defaults off so a normal `go test` run stays quiet; a human reviewing the
// rule ahead of the P3.1 waves runs
// `go test ./cmd/gen_action_inputs/... -run PrintAllGeneratedNames -names -v`
// once to read the product this rule actually produces.
var printActionNames = flag.Bool("names", false, "print every action name ActionName would produce for the real spec, to stderr")

// namedOperation is one operation the real vendored specification declares,
// paired with the domain directory that claims it — everything
// TestActionName_NoCollisionAcrossTheEntireSpecification and the printing
// test below need, and nothing from naming.go itself.
type namedOperation struct {
	Domain      string
	OperationID string
}

// allOperations loads the real vendored EE specification and pairs every one
// of its 441 operations with the domain directory toolutil.DomainTags says
// claims it. It is built entirely from loadDocument, operationsByDomain and
// domainOperations — main.go's own spec-parsing and domain-aggregation code
// — plus toolutil.DomainTags, the curated table. None of that is naming
// code: this is deliberately not a list built by calling ActionName and
// checking its own output for duplicates, which would prove nothing about
// the rule and everything about round-tripping a list through itself — the
// standing trap named for this phase.
func allOperations(t *testing.T) []namedOperation {
	t.Helper()

	_, paths, err := loadDocument("../../api/specs/ee-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}
	if err := toolutil.ValidateDomainTags(toolutil.DomainTags); err != nil {
		t.Fatalf("toolutil.ValidateDomainTags() error = %v", err)
	}
	if err := checkDomainTagsCoverSpec(toolutil.DomainTags, byTag); err != nil {
		t.Fatalf("checkDomainTagsCoverSpec() error = %v, want the curated table to fully cover the real spec", err)
	}

	domains := make([]string, 0, len(toolutil.DomainTags))
	for domain := range toolutil.DomainTags {
		domains = append(domains, domain)
	}
	sort.Strings(domains)

	var all []namedOperation
	for _, domain := range domains {
		ops, err := domainOperations(domain, toolutil.DomainTags, byTag)
		if err != nil {
			t.Fatalf("domainOperations(%q) error = %v", domain, err)
		}
		for _, op := range ops {
			all = append(all, namedOperation{Domain: domain, OperationID: op.OperationID})
		}
	}
	return all
}

// TestActionName_ReproducesEveryHandWrittenPilotName is where the rule and
// the three hand-written, twice-reviewed pilot domains (tags, system,
// registries) have to agree or the disagreement gets resolved, not carried
// forward silently into the 423 names this rule is about to mint. There are
// 19 of these, not 18: internal/tools/{tags,system,registries} declare 3 + 6
// + 10 = 19 actions between them, one more than the task brief's own count of
// "18" — counted directly from the three Specs() functions rather than
// trusted from the brief, precisely because a miscounted fixture is exactly
// the kind of thing this phase's standing warning is about.
//
// One entry does not read "system.nodes": SystemNodesCount's hand-written
// name drops "Count" entirely, which every other pilot name preserves in
// some form (TagList -> tags.list keeps "List", RegistryConfigure ->
// registries.configure keeps "Configure"). The mechanical rule produces
// "system.nodes_count", which is what is asserted below — verdict: the
// pilot name is the one that is wrong, not the rule. "system.nodes" is
// shorter but not reversible by eye: read alone, it could equally have come
// from a hypothetical "SystemNodes" operation, and the brief's own
// requirement for this rule is that it be "reversible by eye". Fixed in
// internal/tools/system/system.go and test/e2e/suite/system_test.go
// alongside this test, rather than special-cased here to keep the test
// passing against a name this task itself found to be the error.
func TestActionName_ReproducesEveryHandWrittenPilotName(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ domain, operationID, want string }{
		// tags (internal/tools/tags/tags.go)
		{"tags", "TagList", "tags.list"},
		{"tags", "TagCreate", "tags.create"},
		{"tags", "TagDelete", "tags.delete"},
		// system (internal/tools/system/system.go)
		{"system", "SystemInfo", "system.info"},
		{"system", "SystemStatus", "system.status"},
		{"system", "SystemVersion", "system.version"},
		{"system", "SystemNodesCount", "system.nodes_count"}, // see doc comment: corrected from the pilot's "system.nodes"
		{"system", "SystemUpdate", "system.update"},
		{"system", "SystemUpgrade", "system.upgrade"},
		// registries (internal/tools/registries/registries.go)
		{"registries", "RegistryList", "registries.list"},
		{"registries", "RegistryCreate", "registries.create"},
		{"registries", "RegistryPing", "registries.ping"},
		{"registries", "RegistryInspect", "registries.inspect"},
		{"registries", "RegistryUpdate", "registries.update"},
		{"registries", "RegistryConfigure", "registries.configure"},
		{"registries", "RegistryDelete", "registries.delete"},
		{"registries", "EcrDeleteRepository", "registries.ecr_delete_repository"},
		{"registries", "EcrDeleteTags", "registries.ecr_delete_tags"},
		{"registries", "RepositoryTagsDelete", "registries.repository_tags_delete"},
	} {
		got, err := ActionName(tc.domain, tc.operationID)
		if err != nil {
			t.Errorf("ActionName(%q, %q) error = %v", tc.domain, tc.operationID, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ActionName(%q, %q) = %q, want %q", tc.domain, tc.operationID, got, tc.want)
		}
	}
}

// TestActionName_NoCollisionAcrossTheEntireSpecification is 441 operations
// minted at once: two producing the same name means one silently shadows
// the other in actioncatalog.Build, which refuses duplicates — surfacing as
// a build failure at wave time with no indication of which name to change.
// allOperations enumerates from the real spec via main.go's own
// spec-parsing code, independent of ActionName, so this cannot pass merely
// because the list it iterates was itself produced by the function under
// test.
func TestActionName_NoCollisionAcrossTheEntireSpecification(t *testing.T) {
	t.Parallel()
	ops := allOperations(t)
	if len(ops) != 441 {
		t.Fatalf("allOperations() returned %d operations, want 441 (the vendored EE spec's real total)", len(ops))
	}
	seen := make(map[string]string, len(ops))
	for _, op := range ops {
		name, err := ActionName(op.Domain, op.OperationID)
		if err != nil {
			// Refused rather than colliding is the expected shape for the
			// handful of operations identical to their own domain prefix
			// (domain "motd"'s "MOTD", domain "backup"'s bare "Backup",
			// domain "team_memberships"'s bare "TeamMemberships") — see
			// TestActionName_LocalPartIsNeverEmpty and this task's report.
			// Those still need a name from somewhere before their domain
			// package is written; they are not a collision, so they do not
			// fail this test, but they are not silently ignored either.
			t.Logf("%s.%s: %v (expected refusal for a same-as-prefix operationID; needs a hand-declared name when this domain is written)", op.Domain, op.OperationID, err)
			continue
		}
		if prior, dup := seen[name]; dup {
			t.Errorf("%q is produced by both %s.%s and %s", name, op.Domain, op.OperationID, prior)
		}
		seen[name] = op.Domain + "." + op.OperationID
	}
}

// TestActionName_LocalPartIsNeverEmpty is the synthetic edge the brief
// specifies directly: an operationID identical to its domain's own prefix
// leaves nothing after stripping. ActionSpec.Validate would refuse an empty
// local part too, but 423 actions later and with no clue where it came
// from — this is where it is caught instead, with the offending operationID
// named in the error.
func TestActionName_LocalPartIsNeverEmpty(t *testing.T) {
	t.Parallel()
	if _, err := ActionName("tags", "Tag"); err == nil {
		t.Error("ActionName() error = nil, want a refusal when nothing remains after the prefix")
	}
}

// TestActionName_RequiresNonEmptyDomainAndOperationID guards the two
// preconditions ActionName documents but the three tests above never
// exercise directly, both real ways to call this function wrong from a
// generator loop that has, say, an unpopulated operation.Domain.
func TestActionName_RequiresNonEmptyDomainAndOperationID(t *testing.T) {
	t.Parallel()
	if _, err := ActionName("", "TagList"); err == nil {
		t.Error(`ActionName("", "TagList") error = nil, want a refusal: domain is required`)
	}
	if _, err := ActionName("tags", ""); err == nil {
		t.Error(`ActionName("tags", "") error = nil, want a refusal: operationID is required`)
	}
}

// TestUnit_ActionName_PrintAllGeneratedNames is step 5 of the task brief made
// runnable: not a test of any property (every name is, by construction here,
// free of collisions and non-empty — those are proven above), just a human
// checkpoint. Skipped unless -names is passed, so a normal `go test ./...`
// run stays silent; run deliberately, once, before the P3.1 waves begin,
// with `go test ./cmd/gen_action_inputs/... -run PrintAllGeneratedNames
// -names -v` to read every one of the 441 names this rule would mint.
// Printing goes to t.Log / os.Stderr, never os.Stdout, which CI's
// repository-wide grep for stray writes to standard output does not
// distinguish from a doc comment.
func TestUnit_ActionName_PrintAllGeneratedNames(t *testing.T) {
	if !*printActionNames {
		t.Skip("run with -names to print every generated action name to stderr")
	}
	ops := allOperations(t)
	names := make([]string, 0, len(ops))
	refused := 0
	for _, op := range ops {
		name, err := ActionName(op.Domain, op.OperationID)
		if err != nil {
			refused++
			names = append(names, fmt.Sprintf("%s.%s -- REFUSED: %v", op.Domain, op.OperationID, err))
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintln(os.Stderr, n)
	}
	fmt.Fprintf(os.Stderr, "%d name(s), %d refusal(s), out of %d operation(s)\n", len(names)-refused, refused, len(ops))
}
