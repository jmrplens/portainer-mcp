package actioncatalog

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools/registries"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func handler(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil }

func spec(name, domain, operationID string, ed edition.Edition) toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: name, Domain: domain, OperationID: operationID,
		Title: "t", Description: "d", Edition: ed, Handler: handler,
	}
}

func eeOpts() Options {
	return Options{Edition: edition.EE, ServerVersion: "2.44.0"}
}

func TestBuild_ValidSpecs_AreIncluded(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{spec("tags.list", "tags", "TagList", edition.CE)}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := len(c.Actions()); got != 1 {
		t.Errorf("Actions() = %d, want 1", got)
	}
	if _, ok := c.Lookup("tags.list"); !ok {
		t.Error("Lookup(tags.list) = false, want the action present")
	}
}

// This is the guard the P1 carry-forward demanded: a hand-written OperationID
// that does not resolve must fail the build, because Available reports an
// unknown operation as unavailable and the action would vanish silently.
func TestBuild_UnresolvableOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{spec("tags.list", "tags", "NoSuchOperation", edition.CE)}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error for an OperationID absent from the applicability table")
	}
	if !strings.Contains(err.Error(), "NoSuchOperation") {
		t.Errorf("error = %q, want it to name the unresolvable operationId", err)
	}
}

// TestBuild_RenderedNameCollision_ReturnsError guards the individual
// surface's tool-name mapping (see RenderToolName): "." is mapped to "_", but
// a valid action name may already contain "_", so two distinct, individually
// valid names can render to the same MCP tool name. mcp.AddTool upserts by
// name, so a colliding pair would otherwise let one action's tool silently
// replace the other's, with no error anywhere — this is the same
// silent-collision hazard the duplicate-name check already guards against for
// exact name matches.
func TestBuild_RenderedNameCollision_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("tags.list_all", "tags", "TagList", edition.CE),
		spec("tags_list.all", "tags_list", "TagCreate", edition.CE),
	}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error: both names render as portainer_tags_list_all")
	}
	for _, want := range []string{"tags.list_all", "tags_list.all"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to name the colliding action %q", err, want)
		}
	}
}

func TestBuild_DuplicateName_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("tags.list", "tags", "TagList", edition.CE),
		spec("tags.list", "tags", "TagCreate", edition.CE),
	}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error for a duplicate action name")
	}
}

func TestBuild_InvalidSpec_ReturnsError(t *testing.T) {
	t.Parallel()
	bad := spec("tags.list", "tags", "TagList", edition.CE)
	bad.Description = ""
	if _, err := Build([]toolutil.ActionSpec{bad}, eeOpts()); err == nil {
		t.Fatal("Build() = nil, want the spec's own validation error to surface")
	}
}

// An EE-only action must not appear in a CE catalog. Getting this backwards
// offers the model operations its server does not implement.
//
// This used to deliberately use SystemInfo (shared by both editions) declared
// as Edition: EE, so that exclusion could only come from the Edition.Includes
// check rather than as a side effect of OperationID resolution. The
// Edition/index cross-check in Build now rejects that declaration outright —
// a shared operation may not be declared EE, because that is exactly the lie
// the cross-check exists to catch (see TestBuild_DeclaresEEForASharedOperation_ReturnsError).
// So this test now uses SystemUpdate, a genuinely EE-only operation: it is
// filtered out of a CE catalog by the OperationID branch before Edition.Includes
// is ever consulted, but the catalog's observable behaviour — the action is
// absent — is exactly what a caller depends on either way.
func TestBuild_EEActionOnCEServer_IsExcluded(t *testing.T) {
	t.Parallel()
	specs := []toolutil.ActionSpec{
		spec("tags.list", "tags", "TagList", edition.CE),
		spec("system.update", "system", "SystemUpdate", edition.EE),
	}
	c, err := Build(specs, Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := c.Lookup("system.update"); ok {
		t.Error("an EE-only action appears in a CE catalog")
	}
	if _, ok := c.Lookup("tags.list"); !ok {
		t.Error("a CE action is missing from a CE catalog")
	}
}

// A CE declaration for an operation that only Business Edition serves is
// documentation that lies, and the index would mask it.
func TestBuild_DeclaresCEForAnEEOnlyOperation_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("system.update", "system", "SystemUpdate", edition.CE),
	}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error: SystemUpdate exists only in EE")
	}
}

// The converse: gating a shared operation to EE hides it from CE servers that
// can serve it perfectly well.
func TestBuild_DeclaresEEForASharedOperation_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("system.info", "system", "SystemInfo", edition.EE),
	}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error: SystemInfo exists in both editions")
	}
}

// The direction the original cross-check missed: declaring EE for an operation
// only Community Edition serves. It built cleanly and was filtered only as a
// side effect of Edition.Includes.
func TestBuild_DeclaresEEForACEOnlyOperation_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("system.upgrade", "system", "SystemUpgrade", edition.EE),
	}, Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err == nil {
		t.Fatal("Build() = nil, want an error: SystemUpgrade exists only in Community Edition")
	}
}

// The cross-check must be fatal regardless of which edition is being built,
// not only the one where the OperationID also happens to resolve. Before the
// fix, an EE-only operation declared Edition CE failed the build only when
// building EE (because the CE lookup missed and hit the neither-edition typo
// path); building CE for the very same mis-declared spec resolved via the
// EE-only branch and `continue`d past the cross-check entirely, so a lying
// spec passed CI in one edition and hard-failed a user's startup in the
// other. AddonDetail is genuinely EE-only.
func TestBuild_DeclaresCEForAnEEOnlyOperation_FailsOnCEBuildToo(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("addons.detail", "addons", "AddonDetail", edition.CE),
	}, Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err == nil {
		t.Fatal("Build() = nil, want an error: AddonDetail exists only in Business Edition, even when building CE")
	}
}

// The converse direction: a CE-only operation declared Edition EE must be
// fatal when building CE too, not only when building EE. WebhookInvoke is
// genuinely CE-only.
func TestBuild_DeclaresEEForACEOnlyOperation_FailsOnEEBuildToo(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{
		spec("stacks.webhook_invoke", "stacks", "WebhookInvoke", edition.EE),
	}, eeOpts())
	if err == nil {
		t.Fatal("Build() = nil, want an error: WebhookInvoke exists only in Community Edition, even when building EE")
	}
}

// A genuinely EE-only operation declared EE must still build.
func TestBuild_DeclaresEEForAnEEOnlyOperation_Succeeds(t *testing.T) {
	t.Parallel()
	if _, err := Build([]toolutil.ActionSpec{
		spec("system.update", "system", "SystemUpdate", edition.EE),
	}, eeOpts()); err != nil {
		t.Fatalf("Build() error = %v, want success for a correct EE declaration", err)
	}
}

// And a genuinely CE-only operation declared CE.
func TestBuild_DeclaresCEForACEOnlyOperation_Succeeds(t *testing.T) {
	t.Parallel()
	if _, err := Build([]toolutil.ActionSpec{
		spec("system.upgrade", "system", "SystemUpgrade", edition.CE),
	}, Options{Edition: edition.CE, ServerVersion: "2.44.0"}); err != nil {
		t.Fatalf("Build() error = %v, want success for a correct CE declaration", err)
	}
}

// An action whose operation does not exist on this server version must be
// excluded, so the catalog cannot offer a route that answers 404.
func TestBuild_ActionAbsentOnServerVersion_IsExcluded(t *testing.T) {
	t.Parallel()
	// SharedGitGetAll is GET /cloud/gitcredentials, withdrawn in 2.43.0.
	specs := []toolutil.ActionSpec{spec("cloud.gitcredentials_list", "cloud", "SharedGitGetAll", edition.EE)}

	on243, err := Build(specs, Options{Edition: edition.EE, ServerVersion: "2.43.0"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := on243.Lookup("cloud.gitcredentials_list"); ok {
		t.Error("an action withdrawn in 2.43.0 appears in a 2.43.0 catalog")
	}

	on244, err := Build(specs, Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := on244.Lookup("cloud.gitcredentials_list"); !ok {
		t.Error("the action is missing from a 2.44.0 catalog, where it exists")
	}
}

// Read-only is enforced here as well as at registration: a catalog built
// read-only must not even contain a mutating action.
func TestBuild_ReadOnly_ExcludesMutatingActions(t *testing.T) {
	t.Parallel()
	mutating := spec("tags.create", "tags", "TagCreate", edition.CE)
	mutating.Mutating = true
	specs := []toolutil.ActionSpec{spec("tags.list", "tags", "TagList", edition.CE), mutating}

	c, err := Build(specs, Options{Edition: edition.EE, ServerVersion: "2.44.0", ReadOnly: true})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if _, ok := c.Lookup("tags.create"); ok {
		t.Error("a mutating action appears in a read-only catalog")
	}
	if _, ok := c.Lookup("tags.list"); !ok {
		t.Error("a read-only action is missing from a read-only catalog")
	}
}

func TestByDomain_GroupsAndSorts(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{
		spec("tags.list", "tags", "TagList", edition.CE),
		spec("system.info", "system", "SystemInfo", edition.CE),
		spec("tags.create", "tags", "TagCreate", edition.CE),
	}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got := c.Domains(); len(got) != 2 || got[0] != "system" || got[1] != "tags" {
		t.Errorf("Domains() = %v, want [system tags] sorted", got)
	}
	tags := c.ByDomain("tags")
	if len(tags) != 2 || tags[0].Name != "tags.create" || tags[1].Name != "tags.list" {
		t.Errorf("ByDomain(tags) = %v, want create then list, sorted", tags)
	}
}

// One catalog is shared by every tool surface. If Actions, Domains or
// ByDomain handed out their backing slice, one surface sorting, filtering in
// place, or editing a returned spec would silently corrupt what every other
// surface sees.
func TestAccessors_ReturnCopies_SoCallersCannotCorruptTheCatalog(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{
		spec("tags.list", "tags", "TagList", edition.CE),
		spec("tags.create", "tags", "TagCreate", edition.CE),
	}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	got := c.Actions()
	got[0].Name = "MUTATED"
	if c.Actions()[0].Name == "MUTATED" {
		t.Error("mutating the slice returned by Actions corrupted the catalog")
	}

	domains := c.Domains()
	domains[0] = "MUTATED"
	if c.Domains()[0] == "MUTATED" {
		t.Error("mutating the slice returned by Domains corrupted the catalog")
	}

	byDomain := c.ByDomain("tags")
	byDomain[0].Name = "MUTATED"
	if c.ByDomain("tags")[0].Name == "MUTATED" {
		t.Error("mutating the slice returned by ByDomain corrupted the catalog")
	}
}

// specWithDiscoveryMetadata is the fixture TestCatalogAccessors_* needs and
// spec() alone cannot provide: every pilot domain (system, tags, registries)
// declares its specs with Aliases, Tags, RelatedActions and ParameterGuidance
// left unset, so a spec built from spec() alone has nil slices and a nil map.
// Asserting that mutating a returned Aliases, Tags, RelatedActions or
// ParameterGuidance (including its CommonConfusions) does not reach the
// catalog is meaningless against those zero values — append(nil-slice, "x")
// never aliases the original nil, and indexing into a nil map is a read, not
// a write a clone could fail to isolate. This fixture gives every one of
// those fields real, non-empty content so the mutations below actually
// exercise cloneActionSpec's slice- and map-copying for each field
// individually, rather than only the two (Aliases, ParameterGuidance's own
// map) the original fixture happened to populate — which is exactly why
// removing the deep-copy of Tags, RelatedActions or CommonConfusions alone
// used to survive every test here unnoticed.
func specWithDiscoveryMetadata(name, domain, operationID string, ed edition.Edition) toolutil.ActionSpec {
	s := spec(name, domain, operationID, ed)
	s.Aliases = []string{"original-alias"}
	s.Tags = []string{"original-tag"}
	s.RelatedActions = []string{"original-related"}
	// Input carries an "id" field so ParameterGuidance's "id" entry below
	// names a real JSON field of Input — Validate refuses a ParameterGuidance
	// key absent from Input's JSON fields, and this fixture's whole point is
	// a spec that passes validation while carrying real discovery metadata.
	s.Input = struct {
		ID int `json:"id"`
	}{}
	s.ParameterGuidance = map[string]toolutil.ParameterGuidance{
		"id": {
			SemanticRole:     "original",
			CommonConfusions: []string{"original-confusion"},
		},
	}
	return s
}

// TestCatalogAccessors_DoNotShareSliceOrMapMemory guards the knock-on Task 1
// introduced silently: ActionSpec grew slice and map fields, and Actions,
// ByDomain and Lookup used to return copies that only isolated scalar
// fields. A caller that grows a returned spec's Aliases or writes into its
// ParameterGuidance must never be able to reach what the catalog itself
// stores — the very isolation every other surface's shared-catalog
// assumption depends on.
func TestCatalogAccessors_DoNotShareSliceOrMapMemory(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{
		specWithDiscoveryMetadata("tags.list", "tags", "TagList", edition.CE),
	}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	// Sanity guard: if the fixture ever regresses to an empty Aliases, Tags,
	// RelatedActions, a nil ParameterGuidance or an empty CommonConfusions,
	// the corresponding mutation below would silently exercise nothing,
	// exactly the non-discriminating-test trap this task is warned about.
	actions := c.Actions()
	if len(actions[0].Aliases) == 0 {
		t.Fatal("fixture has no Aliases; the mutation below would prove nothing")
	}
	if len(actions[0].Tags) == 0 {
		t.Fatal("fixture has no Tags; the mutation below would prove nothing")
	}
	if len(actions[0].RelatedActions) == 0 {
		t.Fatal("fixture has no RelatedActions; the mutation below would prove nothing")
	}
	if actions[0].ParameterGuidance == nil {
		t.Fatal("fixture has no ParameterGuidance; the mutation below would prove nothing")
	}
	if len(actions[0].ParameterGuidance["id"].CommonConfusions) == 0 {
		t.Fatal("fixture's ParameterGuidance has no CommonConfusions; the mutation below would prove nothing")
	}

	actions[0].Aliases[0] = "corrupted"
	actions[0].Tags[0] = "corrupted"
	actions[0].RelatedActions[0] = "corrupted"
	actions[0].ParameterGuidance["injected"] = toolutil.ParameterGuidance{}
	actions[0].ParameterGuidance["id"].CommonConfusions[0] = "corrupted"

	again := c.Actions()
	if again[0].Aliases[0] == "corrupted" {
		t.Error("Actions() shares Aliases slice memory with the catalog")
	}
	if again[0].Tags[0] == "corrupted" {
		t.Error("Actions() shares Tags slice memory with the catalog")
	}
	if again[0].RelatedActions[0] == "corrupted" {
		t.Error("Actions() shares RelatedActions slice memory with the catalog")
	}
	if _, leaked := again[0].ParameterGuidance["injected"]; leaked {
		t.Error("Actions() shares ParameterGuidance map memory with the catalog")
	}
	if again[0].ParameterGuidance["id"].CommonConfusions[0] == "corrupted" {
		t.Error("Actions() shares ParameterGuidance's CommonConfusions slice memory with the catalog")
	}
}

// TestByDomainAccessor_DoesNotShareSliceOrMapMemory is the same guard for
// ByDomain, which Actions' own test cannot cover: ByDomain reads from a
// different backing slice (c.byDomain) than Actions does (c.ordered), so a
// clone applied to one and forgotten on the other would pass every Actions
// test while still leaking through ByDomain.
func TestByDomainAccessor_DoesNotShareSliceOrMapMemory(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{
		specWithDiscoveryMetadata("tags.list", "tags", "TagList", edition.CE),
	}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	byDomain := c.ByDomain("tags")
	if len(byDomain[0].Aliases) == 0 || len(byDomain[0].Tags) == 0 || len(byDomain[0].RelatedActions) == 0 ||
		byDomain[0].ParameterGuidance == nil || len(byDomain[0].ParameterGuidance["id"].CommonConfusions) == 0 {
		t.Fatal("fixture lost its Aliases, Tags, RelatedActions or ParameterGuidance; the mutation below would prove nothing")
	}

	byDomain[0].Aliases[0] = "corrupted"
	byDomain[0].Tags[0] = "corrupted"
	byDomain[0].RelatedActions[0] = "corrupted"
	byDomain[0].ParameterGuidance["injected"] = toolutil.ParameterGuidance{}
	byDomain[0].ParameterGuidance["id"].CommonConfusions[0] = "corrupted"

	again := c.ByDomain("tags")
	if again[0].Aliases[0] == "corrupted" {
		t.Error("ByDomain() shares Aliases slice memory with the catalog")
	}
	if again[0].Tags[0] == "corrupted" {
		t.Error("ByDomain() shares Tags slice memory with the catalog")
	}
	if again[0].RelatedActions[0] == "corrupted" {
		t.Error("ByDomain() shares RelatedActions slice memory with the catalog")
	}
	if _, leaked := again[0].ParameterGuidance["injected"]; leaked {
		t.Error("ByDomain() shares ParameterGuidance map memory with the catalog")
	}
	if again[0].ParameterGuidance["id"].CommonConfusions[0] == "corrupted" {
		t.Error("ByDomain() shares ParameterGuidance's CommonConfusions slice memory with the catalog")
	}
}

// TestLookupAccessor_DoesNotShareSliceOrMapMemory is the same guard for
// Lookup, which reads from yet another backing map (c.byName) that Build
// populates independently of c.ordered and c.byDomain.
func TestLookupAccessor_DoesNotShareSliceOrMapMemory(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{
		specWithDiscoveryMetadata("tags.list", "tags", "TagList", edition.CE),
	}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}

	found, ok := c.Lookup("tags.list")
	if !ok {
		t.Fatal("Lookup(tags.list) = false, want the action present")
	}
	if len(found.Aliases) == 0 || len(found.Tags) == 0 || len(found.RelatedActions) == 0 ||
		found.ParameterGuidance == nil || len(found.ParameterGuidance["id"].CommonConfusions) == 0 {
		t.Fatal("fixture lost its Aliases, Tags, RelatedActions or ParameterGuidance; the mutation below would prove nothing")
	}

	found.Aliases[0] = "corrupted"
	found.Tags[0] = "corrupted"
	found.RelatedActions[0] = "corrupted"
	found.ParameterGuidance["injected"] = toolutil.ParameterGuidance{}
	found.ParameterGuidance["id"].CommonConfusions[0] = "corrupted"

	again, ok := c.Lookup("tags.list")
	if !ok {
		t.Fatal("Lookup(tags.list) = false on the second call")
	}
	if again.Aliases[0] == "corrupted" {
		t.Error("Lookup() shares Aliases slice memory with the catalog")
	}
	if again.Tags[0] == "corrupted" {
		t.Error("Lookup() shares Tags slice memory with the catalog")
	}
	if again.RelatedActions[0] == "corrupted" {
		t.Error("Lookup() shares RelatedActions slice memory with the catalog")
	}
	if _, leaked := again.ParameterGuidance["injected"]; leaked {
		t.Error("Lookup() shares ParameterGuidance map memory with the catalog")
	}
	if again.ParameterGuidance["id"].CommonConfusions[0] == "corrupted" {
		t.Error("Lookup() shares ParameterGuidance's CommonConfusions slice memory with the catalog")
	}
}

func TestBuild_UnknownEdition_ReturnsError(t *testing.T) {
	t.Parallel()
	_, err := Build([]toolutil.ActionSpec{spec("tags.list", "tags", "TagList", edition.CE)},
		Options{Edition: edition.Edition("bogus"), ServerVersion: "2.44.0"})
	if err == nil {
		t.Fatal("Build() = nil, want an error: an unknown edition would silently empty the catalog")
	}
}

func TestBuild_EmptyEdition_ReturnsError(t *testing.T) {
	t.Parallel()
	if _, err := Build([]toolutil.ActionSpec{spec("tags.list", "tags", "TagList", edition.CE)},
		Options{ServerVersion: "2.44.0"}); err == nil {
		t.Fatal("Build() = nil, want an error for an empty edition")
	}
}

// editionGatedInput is the fixture every Catalog.InputSchema test below
// shares: one plain field and one `edition:"EE"` field, the exact shape
// Task 2's own brief specifies for its Step 1 tests.
//
// EdgeKey deliberately carries no "omitempty": google/jsonschema-go's
// reflector puts a field in the reflected schema's "required" list exactly
// when it lacks omitempty/omitzero (see toolutil/schema.go's InputSchema and
// cmd/gen_action_inputs/fields.go's fieldTag, which states the identical
// rule for the generator's own output), so an omitempty EdgeKey is never in
// "required" to begin with — TestUnit_CatalogInputSchema_CEBuild_
// DropsEEOnlyFieldAndFromRequired's own "required" assertion would then pass
// whether or not pruning touches "required" at all, and deleting that whole
// code path (catalog.go's `switch req := schema["required"].(type)` block)
// left every test in this file green. A genuinely required gated field is
// what makes the "required" half of that test — and of the mutation that
// proves it — actually exercise the code it claims to.
type editionGatedInput struct {
	Name    string `json:"name"`
	EdgeKey string `json:"edgeKey" edition:"EE"`
}

// editionGatedSpec is spec() plus an Input carrying editionGatedInput, so
// its schema has something for edition-based pruning to actually drop.
// system.info/SystemInfo is shared by both editions in the real
// applicability index, which every test below needs: Build's own
// Edition/index cross-check would refuse a CE declaration for an
// EE-exclusive operation before InputSchema is ever reached.
func editionGatedSpec() toolutil.ActionSpec {
	s := spec("system.info", "system", "SystemInfo", edition.CE)
	s.Input = editionGatedInput{}
	return s
}

// TestUnit_CatalogInputSchema_CEBuild_DropsEEOnlyFieldAndFromRequired is
// Task 2's Step 1 test, CE half: built for Community Edition, the plain
// field is published and the EE-only field is gone from both "properties"
// and "required".
func TestUnit_CatalogInputSchema_CEBuild_DropsEEOnlyFieldAndFromRequired(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{editionGatedSpec()}, Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	found, ok := c.Lookup("system.info")
	if !ok {
		t.Fatal("Lookup(system.info) = false, want the action present on a CE catalog")
	}

	schema, err := c.InputSchema(found)
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["name"]; !ok {
		t.Errorf("properties = %v, want the untagged \"name\" field present", props)
	}
	if _, ok := props["edgeKey"]; ok {
		t.Errorf("properties = %v, want the edition:\"EE\" \"edgeKey\" field pruned from a CE catalog", props)
	}
	required := toolutil.RequiredParams(schema)
	sawName := false
	for _, name := range required {
		if name == "edgeKey" {
			t.Error("required still names \"edgeKey\" after it was pruned from properties")
		}
		if name == "name" {
			sawName = true
		}
	}
	// Both required fields would trivially "pass" the edgeKey check above if
	// pruning simply emptied "required" altogether rather than removing only
	// the gated entry — this is the other half that catches that: "name" was
	// never gated and must still be required.
	if !sawName {
		t.Errorf("required = %v, want the untagged, still-required \"name\" field present", required)
	}
}

// TestUnit_CatalogInputSchema_EEBuild_KeepsEEOnlyField is Step 1's EE half:
// built for Business Edition, both fields appear.
func TestUnit_CatalogInputSchema_EEBuild_KeepsEEOnlyField(t *testing.T) {
	t.Parallel()
	c, err := Build([]toolutil.ActionSpec{editionGatedSpec()}, eeOpts())
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	found, ok := c.Lookup("system.info")
	if !ok {
		t.Fatal("Lookup(system.info) = false, want the action present on an EE catalog")
	}

	schema, err := c.InputSchema(found)
	if err != nil {
		t.Fatalf("InputSchema() error = %v", err)
	}
	props, _ := schema["properties"].(map[string]any)
	for _, name := range []string{"name", "edgeKey"} {
		if _, ok := props[name]; !ok {
			t.Errorf("properties = %v, want %q present on an EE catalog", props, name)
		}
	}
}

// TestUnit_PruneInputSchemaByEdition_NothingToDrop_ReturnsSchemaUnchanged is
// the discriminating half the brief calls out explicitly: an action with no
// `edition:"EE"` field (fieldEditions nil, the common case) must get back
// the exact same map, not a needless clone.
//
// spec.InputSchema itself already returns an independent deep copy on every
// call (see that method's own doc comment), so this property has to be
// tested at the pruning function's own level — Catalog.InputSchema's two
// stages (spec.InputSchema, then pruneInputSchemaByEdition) cannot be told
// apart by comparing two separately-obtained schemas, since the first stage
// alone already guarantees they are never the same map. Proven here by
// mutating schema *after* the call and checking the mutation is visible in
// got too — only true if the two are the same underlying map, which a
// clone would not be.
func TestUnit_PruneInputSchemaByEdition_NothingToDrop_ReturnsSchemaUnchanged(t *testing.T) {
	t.Parallel()
	schema := map[string]any{
		"type":       "object",
		"properties": map[string]any{"name": map[string]any{"type": "string"}},
	}

	got := pruneInputSchemaByEdition(schema, nil, edition.CE)
	schema["marker"] = "planted after the call"
	if _, ok := got["marker"]; !ok {
		t.Error("pruneInputSchemaByEdition returned a different map when fieldEditions was nil; want the identical map, not a clone")
	}

	// Same claim, the other way nothing-to-drop can arise: a fieldEditions
	// map that names a field, but whose required edition the target already
	// includes (an EE target facing an EE-only field), so drop ends up empty
	// even though fieldEditions itself is not nil.
	schema2 := map[string]any{
		"type":       "object",
		"properties": map[string]any{"edgeKey": map[string]any{"type": "string"}},
	}
	got2 := pruneInputSchemaByEdition(schema2, map[string]edition.Edition{"edgeKey": edition.EE}, edition.EE)
	schema2["marker"] = "planted after the call"
	if _, ok := got2["marker"]; !ok {
		t.Error("pruneInputSchemaByEdition cloned the schema even though the EE target already includes every gated field")
	}
}

// TestUnit_CatalogInputSchema_BuildingCEThenEE_DoesNotLoseFieldForEE is the
// trap named explicitly in the brief: a test that only ever builds a CE
// catalog cannot tell "prunes per catalog" apart from "mutates the shared
// cached schema", because both look identical from the CE side alone. This
// builds CE first, reads its (rightly) pruned schema, then builds a second,
// independent EE catalog for the exact same Input type and confirms the
// EE-only field is still there — which an implementation that cached the
// *pruned* result keyed only by reflect.Type (ignoring which edition it was
// pruned for) would fail, because the CE build would have poisoned that
// cache entry before the EE build ever read it.
func TestUnit_CatalogInputSchema_BuildingCEThenEE_DoesNotLoseFieldForEE(t *testing.T) {
	t.Parallel()
	ceCatalog, err := Build([]toolutil.ActionSpec{editionGatedSpec()}, Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("CE Build() error = %v", err)
	}
	ceSpec, _ := ceCatalog.Lookup("system.info")
	ceSchema, err := ceCatalog.InputSchema(ceSpec)
	if err != nil {
		t.Fatalf("CE InputSchema() error = %v", err)
	}
	ceProps, _ := ceSchema["properties"].(map[string]any)
	if _, leaked := ceProps["edgeKey"]; leaked {
		t.Fatal("CE catalog already publishes edgeKey; the fixture proves nothing about the cache trap")
	}

	eeCatalog, err := Build([]toolutil.ActionSpec{editionGatedSpec()}, eeOpts())
	if err != nil {
		t.Fatalf("EE Build() error = %v", err)
	}
	eeSpec, _ := eeCatalog.Lookup("system.info")
	eeSchema, err := eeCatalog.InputSchema(eeSpec)
	if err != nil {
		t.Fatalf("EE InputSchema() error = %v", err)
	}
	eeProps, _ := eeSchema["properties"].(map[string]any)
	if _, ok := eeProps["edgeKey"]; !ok {
		t.Error("EE catalog is missing edgeKey after a CE catalog for the same Input type was built first: the CE build's pruning leaked across catalogs")
	}
}

// The end-to-end property the registries pilot exists for: the same declared
// specs must yield different catalogs on CE and EE.
func TestBuild_RegistriesPilot_YieldsDifferentCatalogsPerEdition(t *testing.T) {
	t.Parallel()
	ceCatalog, err := Build(registries.Specs(), Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("CE Build: %v", err)
	}
	eeCatalog, err := Build(registries.Specs(), Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("EE Build: %v", err)
	}
	if len(ceCatalog.Actions()) >= len(eeCatalog.Actions()) {
		t.Errorf("CE has %d actions and EE has %d; EE must offer strictly more in this domain",
			len(ceCatalog.Actions()), len(eeCatalog.Actions()))
	}
}

// propertyNames returns schema's "properties" keys, sorted, so two schemas'
// published fields can be compared and printed deterministically.
func propertyNames(schema map[string]any) []string {
	props, _ := schema["properties"].(map[string]any)
	names := make([]string, 0, len(props))
	for name := range props {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// TestUnit_Catalog_RegistriesDivergentActions_PublishExactFieldsPerEdition is
// this task's per-edition catalog test. registries is used rather than the
// brief's own suggested endpoints.create/endpoints.list: this repository has
// not scaffolded a docker/endpoints domain yet (wave 1 is the next plan, and
// this task is expressly forbidden from scaffolding one — see this task's
// own "Notes for the executor"), whereas registries is real, already built,
// and already carries a genuine field-level divergence — task 7 hand-tagged
// registryCreateInput.Github, registryUpdateInput.Github and
// registryInspectInput.EndpointID with `edition:"EE"` (see
// internal/tools/registries/inputs.go and that domain's own
// edition_prune_test.go, the first real-code exercise of Task 2's mechanism
// outside actioncatalog's own package). This is the second: it builds the
// real catalog Task 2's mechanism actually runs inside
// (actioncatalog.Build + Catalog.InputSchema), rather than registries' own
// test, which is exactly as real but does not itself prove the *catalog*
// wiring — as opposed to some registries-local helper — is what performs the
// pruning.
//
// The trap this guards against (spelled out in this task's own brief): a
// test that only asserts "the CE count differs from the EE count" passes
// against an implementation that prunes the *wrong* field, or prunes at
// random, as long as it prunes something. Every case below asserts the
// specific field that must divarge, in both directions (present in EE,
// absent from CE) — never a bare count.
//
// It also survives an unrelated hand-edit to registries: rather than pinning
// to registries.create's current 11-CE/12-EE property counts (which a future
// field addition to registryCreateInput would break for a reason unrelated
// to edition gating), the third assertion below checks the invariant "EE's
// properties, minus exactly the named Business-only fields, equal CE's
// properties" — true regardless of how many other, ungated fields
// registries.create happens to declare on either side of this edit.
func TestUnit_Catalog_RegistriesDivergentActions_PublishExactFieldsPerEdition(t *testing.T) {
	t.Parallel()

	ceCatalog, err := Build(registries.Specs(), Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build(CE): %v", err)
	}
	eeCatalog, err := Build(registries.Specs(), Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build(EE): %v", err)
	}

	cases := []struct {
		actionName string
		// businessOnly names the field(s) this action's Input tags
		// edition:"EE": genuinely absent from the Community Edition
		// operation's own resolved schema (verified directly against
		// api/specs/ce-2.44.0.json — see registries/inputs.go's own doc
		// comments on registryCreateInput.Github, registryUpdateInput.Github
		// and registryInspectInput.EndpointID for the exact verification).
		businessOnly []string
	}{
		{"registries.create", []string{"github"}},
		{"registries.update", []string{"github"}},
		{"registries.inspect", []string{"endpointId"}},
	}

	for _, tc := range cases {
		t.Run(tc.actionName, func(t *testing.T) {
			t.Parallel()

			ceSpec, ok := ceCatalog.Lookup(tc.actionName)
			if !ok {
				t.Fatalf("%s: not found in the Community catalog", tc.actionName)
			}
			ceSchema, err := ceCatalog.InputSchema(ceSpec)
			if err != nil {
				t.Fatalf("CE InputSchema(%s): %v", tc.actionName, err)
			}

			eeSpec, ok := eeCatalog.Lookup(tc.actionName)
			if !ok {
				t.Fatalf("%s: not found in the Business catalog", tc.actionName)
			}
			eeSchema, err := eeCatalog.InputSchema(eeSpec)
			if err != nil {
				t.Fatalf("EE InputSchema(%s): %v", tc.actionName, err)
			}

			ceProps, _ := ceSchema["properties"].(map[string]any)
			eeProps, _ := eeSchema["properties"].(map[string]any)

			// Direction 1: every named field is genuinely published to
			// Business Edition. A pruning bug that dropped it from both
			// editions (rather than only Community) would still pass a test
			// that checked only direction 2 below.
			for _, field := range tc.businessOnly {
				if _, ok := eeProps[field]; !ok {
					t.Errorf("%s: Business Edition does not publish %q, want it present", tc.actionName, field)
				}
			}

			// Direction 2: every named field is genuinely pruned from
			// Community Edition. This is the direction Task 2's own
			// mechanism exists for.
			for _, field := range tc.businessOnly {
				if _, ok := ceProps[field]; ok {
					t.Errorf("%s: Community Edition still publishes %q, want it pruned", tc.actionName, field)
				}
			}

			// Direction 3, the specific-not-a-count check: dropping exactly
			// businessOnly's fields from EE's own property set must yield
			// precisely CE's — not merely a smaller set, and not pinned to
			// today's literal counts. A pruning implementation that dropped
			// some unrelated field (or dropped nothing and this domain's
			// fixture happened to already differ some other way) would fail
			// here even though directions 1 and 2 above both passed.
			want := make(map[string]bool, len(eeProps))
			for name := range eeProps {
				want[name] = true
			}
			for _, field := range tc.businessOnly {
				delete(want, field)
			}
			wantNames := make([]string, 0, len(want))
			for name := range want {
				wantNames = append(wantNames, name)
			}
			sort.Strings(wantNames)

			gotNames := propertyNames(ceSchema)
			if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
				t.Errorf("%s: CE properties = %v, want exactly EE's properties %v minus %v",
					tc.actionName, gotNames, propertyNames(eeSchema), tc.businessOnly)
			}
		})
	}
}
