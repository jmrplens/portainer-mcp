package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
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

// actionNameCollisions resolves every operation in ops through the same
// override-first-then-mechanical rule ResolveActionName applies, against the
// supplied override table, and returns one message per name produced by more
// than one operationId.
//
// Taking the table as a parameter is what makes the test below discriminating
// rather than vacuous: the same scan runs over the real table (which must be
// clean) and over a synthetic table that reintroduces a known collision (which
// must be caught). A scan that could only ever see the real table would report
// "no collisions" just as convincingly if it compared nothing at all.
func actionNameCollisions(t *testing.T, overrides map[string]actionNameOverride, ops []namedOperation) []string {
	t.Helper()
	type source struct {
		domain, operationID string
		overridden          bool
	}
	seen := make(map[string]source, len(ops))
	var collisions []string
	for _, op := range ops {
		name, err := resolveActionName(overrides, op.Domain, op.OperationID)
		if err != nil {
			// Still a refusal for a same-as-prefix operationId with no
			// override yet — expected, and not a collision. Unlike the
			// ActionName-only test this one cannot quietly excuse an
			// *overridden* operation, because an override always resolves.
			continue
		}
		_, overridden := overrides[op.OperationID]
		if prior, dup := seen[name]; dup {
			collisions = append(collisions, fmt.Sprintf(
				"%q is produced by both %s.%s (overridden=%t) and %s.%s (overridden=%t)",
				name, op.Domain, op.OperationID, overridden,
				prior.domain, prior.operationID, prior.overridden))
			continue
		}
		seen[name] = source{domain: op.Domain, operationID: op.OperationID, overridden: overridden}
	}
	sort.Strings(collisions)
	return collisions
}

// TestUnit_ResolveActionName_NoCollisionAcrossTheEntireSpecification is the
// check TestActionName_NoCollisionAcrossTheEntireSpecification above
// structurally cannot make. That test calls ActionName directly, so an
// entry in actionNameOverrides is invisible to it twice over: the override's
// *produced* name is never computed, and the override's *source* operationId
// is exactly one of the same-as-prefix refusals it skips with a t.Logf. Every
// domain generator calls ResolveActionName, not ActionName, so the names that
// actually reach actioncatalog.Build were going unchecked.
//
// That was not hypothetical. The "TeamMemberships" override (GET
// /teams/{id}/memberships) was hand-picked as "team_memberships.list", which
// is byte-identical to the name ActionName mechanically mints for the
// different operationId TeamMembershipList (GET /team_memberships). Two
// actions, one name, and actioncatalog.Build refuses duplicates — a build
// failure at wave time naming neither culprit.
func TestUnit_ResolveActionName_NoCollisionAcrossTheEntireSpecification(t *testing.T) {
	t.Parallel()
	ops := allOperations(t)
	if len(ops) != 441 {
		t.Fatalf("allOperations() returned %d operations, want 441 (the vendored EE spec's real total)", len(ops))
	}
	for _, collision := range actionNameCollisions(t, actionNameOverrides, ops) {
		t.Error(collision)
	}
}

// TestUnit_ResolveActionName_CollisionCheckCatchesAnOverrideShadowingAMechanicalName
// is the other half of the proof: it reintroduces the exact defect described
// above in a synthetic override table and requires the scan to catch it. Both
// tests are needed. The first alone would pass against a scan with the very
// blind spot this pair exists to close; this one alone would pass against a
// real table that still collides.
func TestUnit_ResolveActionName_CollisionCheckCatchesAnOverrideShadowingAMechanicalName(t *testing.T) {
	t.Parallel()
	ops := allOperations(t)

	synthetic := map[string]actionNameOverride{
		"TeamMemberships": {
			Domain: "team_memberships",
			Name:   "team_memberships.list", // the pre-fix name: collides with TeamMembershipList's mechanical one
			Reason: "synthetic: reproduces the collision C1 found, so the scan above is proven to detect it",
		},
	}
	collisions := actionNameCollisions(t, synthetic, ops)
	if len(collisions) == 0 {
		t.Fatal("actionNameCollisions() found nothing against a table that reintroduces the TeamMemberships/TeamMembershipList collision; the scan is not comparing override names against mechanical ones")
	}
	for _, c := range collisions {
		if !strings.Contains(c, "team_memberships.list") {
			t.Errorf("unexpected collision %q; want the synthetic TeamMemberships one", c)
		}
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

// TestUnit_ActionSplitter_ImprovesNamesWithoutChangingOldSplitterOutput is
// the two-sided proof the separation between splitActionWords and splitWords
// actually holds. Asserting only that splitActionWords now produces a good
// name would pass exactly as well if actionInitialisms' extra entries (GPU,
// RBAC, CSV, MTLS, CA) had been added to commonInitialisms instead of a
// table of their own — the coupling this task exists to remove. So every
// case here checks both functions against the same identifier: the new
// splitter's improved output, and — unchanged — splitWords' original,
// pre-existing output, exactly as it was before actionInitialisms existed.
// If a future edit ever merges the two tables, the second half of each case
// is what fails.
func TestUnit_ActionSplitter_ImprovesNamesWithoutChangingOldSplitterOutput(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		identifier          string
		wantActionLocalPart string   // splitActionWords, joined the way ActionName joins its local part
		wantOldSplitWords   []string // splitWords, exactly as goFieldName/bodyJSONTag still consume it
	}{
		{"GetKubernetesGPUInfo", "get_kubernetes_gpu_info", []string{"Get", "Kubernetes", "G", "P", "UI", "nfo"}},
		{"GetKubernetesRBACStatus", "get_kubernetes_rbac_status", []string{"Get", "Kubernetes", "R", "B", "A", "C", "Status"}},
		{"AuthLogsCSV", "auth_logs_csv", []string{"Auth", "Logs", "C", "S", "V"}},
		{"MTLSCertificate", "mtls_certificate", []string{"M", "TLS", "Certificate"}},
		{"ValidateOAuth", "validate_oauth", []string{"Validate", "O", "Auth"}},
		{"BackupToS3", "backup_to_s3", []string{"Backup", "To", "S", "3"}},
		{"UpdateK8sPodSecurityRule", "update_k8s_pod_security_rule", []string{"Update", "K", "8s", "Pod", "Security", "Rule"}},
	} {
		gotActionParts := splitActionWords(tc.identifier)
		lowered := make([]string, len(gotActionParts))
		for i, w := range gotActionParts {
			lowered[i] = lower(w)
		}
		if got := strings.Join(lowered, "_"); got != tc.wantActionLocalPart {
			t.Errorf("splitActionWords(%q) joined = %q, want %q", tc.identifier, got, tc.wantActionLocalPart)
		}

		gotOld := splitWords(tc.identifier)
		if strings.Join(gotOld, "|") != strings.Join(tc.wantOldSplitWords, "|") {
			t.Errorf("splitWords(%q) = %q, want %q unchanged — actionInitialisms must never affect the function goFieldName and bodyJSONTag call",
				tc.identifier, gotOld, tc.wantOldSplitWords)
		}
	}
}

// TestUnit_ActionSplitter_WireTagsStayByteIdentical is the strongest form of
// the same proof: it regenerates registries/inputs.gen.go and
// tags/inputs.gen.go — the only two domains with any Input struct today, so
// the only two with any wire tag to check — from the real vendored spec, the
// same way `make gen-action-inputs` does, and asserts the result is
// byte-for-byte what is already committed. actionInitialisms carrying five
// entries commonInitialisms does not (GPU, RBAC, CSV, MTLS, CA) is only safe
// if nothing renderFile emits ever consults that table; this is what would
// catch it if it ever did.
func TestUnit_ActionSplitter_WireTagsStayByteIdentical(t *testing.T) {
	t.Parallel()
	for _, domainName := range []string{"registries", "tags"} {
		committed, err := os.ReadFile("../../internal/tools/" + domainName + "/inputs.gen.go")
		if err != nil {
			t.Fatalf("read committed inputs.gen.go for %s: %v", domainName, err)
		}
		regenerated := regenerateInputsFile(t, domainName)
		if !bytes.Equal(committed, regenerated) {
			t.Errorf("regenerated %s/inputs.gen.go does not match the committed file byte for byte", domainName)
		}
	}
}

// regenerateInputsFile reproduces main.go's run() body for one domain
// exactly — load the real spec, aggregate that domain's operations, assemble
// each operation's Input struct fields, render — so
// TestUnit_ActionSplitter_WireTagsStayByteIdentical is regenerating through
// the real generator's own code path, not a reimplementation of it that
// could drift from what `make gen-action-inputs` actually runs.
func regenerateInputsFile(t *testing.T, domainName string) []byte {
	t.Helper()
	// loadPath is where the test process (cwd inside cmd/gen_action_inputs)
	// actually finds the file; specPath is what `make gen-action-inputs`
	// passes -spec from the repo root, and therefore what is already
	// embedded in the committed file's "DO NOT EDIT" header — renderFile
	// writes specPath verbatim into that header, so passing the load path
	// there instead would fail this comparison for a reason that has nothing
	// to do with actionInitialisms.
	const loadPath = "../../api/specs/ee-2.44.0.json"
	const specPath = "api/specs/ee-2.44.0.json"

	doc, paths, err := loadDocument(loadPath)
	if err != nil {
		t.Fatalf("loadDocument() error = %v", err)
	}
	byTag, err := operationsByDomain(paths)
	if err != nil {
		t.Fatalf("operationsByDomain() error = %v", err)
	}
	ops, err := domainOperations(domainName, toolutil.DomainTags, byTag)
	if err != nil {
		t.Fatalf("domainOperations(%q) error = %v", domainName, err)
	}

	res := &resolver{doc: doc}
	var allStructs []structSpec
	for _, op := range ops {
		structName := inputStructName(op.OperationID)
		var nested []structSpec
		fields, _, err := assembleOperationFields(op, res, doc, structName, &nested)
		if err != nil {
			t.Fatalf("assembleOperationFields(%s): %v", op.OperationID, err)
		}
		if len(fields) == 0 {
			continue
		}
		allStructs = append(allStructs, structSpec{
			Name: structName,
			Doc: fmt.Sprintf("%s is the parameter shape for operation %s (%s %s).",
				structName, op.OperationID, op.Method, op.Path),
			Fields: fields,
		})
		allStructs = append(allStructs, nested...)
	}

	source, err := renderFile(domainName, specPath, allStructs)
	if err != nil {
		t.Fatalf("renderFile(%q): %v", domainName, err)
	}
	return source
}

// TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation is
// actionNameOverrides' own honesty check, the same shape as
// TestUnit_DomainTags_CoversEveryTagInTheVendoredSpec in main_test.go: an
// override naming an operationId the real spec does not have (or naming the
// wrong domain for one it does have) is a name nobody is getting generated
// for them, indistinguishable from a correct entry until something checks it
// against the spec directly.
func TestUnit_ActionNameOverrides_EveryEntryMatchesARealOperation(t *testing.T) {
	t.Parallel()
	if err := validateActionNameOverrides(actionNameOverrides, allOperations(t)); err != nil {
		t.Fatalf("validateActionNameOverrides() error = %v", err)
	}
}

// TestValidateActionNameOverrides_StaleEntry_ReturnsError proves the
// direction that matters most in practice: a future respec that drops or
// renames one of these four operations must fail loudly, not leave a dead
// entry silently promising a name generation will never ask for again.
func TestValidateActionNameOverrides_StaleEntry_ReturnsError(t *testing.T) {
	t.Parallel()
	overrides := map[string]actionNameOverride{
		"NoLongerInTheSpec": {Domain: "motd", Name: "motd.get", Reason: "test fixture"},
	}
	ops := []namedOperation{{Domain: "motd", OperationID: "MOTD"}}
	err := validateActionNameOverrides(overrides, ops)
	if err == nil {
		t.Fatal("validateActionNameOverrides() error = nil, want one: the override names an operationId absent from ops")
	}
	if !strings.Contains(err.Error(), "NoLongerInTheSpec") {
		t.Errorf("error = %q, want it to name the stale operationId", err)
	}
}

// TestValidateActionNameOverrides_WrongDomain_ReturnsError guards the other
// mistake this table can make: an override correctly naming a real
// operationId but the wrong domain for it, which ResolveActionName's own
// domain check would also catch at call time — this is the same property
// checked ahead of time, against the whole table at once.
func TestValidateActionNameOverrides_WrongDomain_ReturnsError(t *testing.T) {
	t.Parallel()
	overrides := map[string]actionNameOverride{
		"MOTD": {Domain: "wrong_domain", Name: "motd.get", Reason: "test fixture"},
	}
	ops := []namedOperation{{Domain: "motd", OperationID: "MOTD"}}
	err := validateActionNameOverrides(overrides, ops)
	if err == nil {
		t.Fatal("validateActionNameOverrides() error = nil, want one: the override names the wrong domain")
	}
	if !strings.Contains(err.Error(), "MOTD") {
		t.Errorf("error = %q, want it to name the mismatched operationId", err)
	}
}

// TestResolveActionName_PrefersOverrideThenFallsBackToActionName is
// ResolveActionName's own contract: an operationId actionNameOverrides names
// gets that override's Name verbatim, regardless of what ActionName would
// have computed; anything else falls through to ActionName unchanged.
func TestResolveActionName_PrefersOverrideThenFallsBackToActionName(t *testing.T) {
	t.Parallel()

	name, err := ResolveActionName("motd", "MOTD")
	if err != nil {
		t.Fatalf("ResolveActionName(\"motd\", \"MOTD\") error = %v", err)
	}
	if want := actionNameOverrides["MOTD"].Name; name != want {
		t.Errorf("ResolveActionName(\"motd\", \"MOTD\") = %q, want the override's %q", name, want)
	}

	if _, err := ResolveActionName("motd", "MOTD"); err != nil {
		t.Errorf("ResolveActionName(\"motd\", \"MOTD\") error = %v, want nil: the override's own Domain matches", err)
	}
	if _, err := ResolveActionName("some_other_domain", "MOTD"); err == nil {
		t.Error("ResolveActionName(\"some_other_domain\", \"MOTD\") error = nil, want a refusal: the override is declared for domain \"motd\"")
	}

	name, err = ResolveActionName("tags", "TagList")
	if err != nil {
		t.Fatalf("ResolveActionName(\"tags\", \"TagList\") error = %v", err)
	}
	if name != "tags.list" {
		t.Errorf("ResolveActionName(\"tags\", \"TagList\") = %q, want ActionName's own \"tags.list\" (no override names this operationId)", name)
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
