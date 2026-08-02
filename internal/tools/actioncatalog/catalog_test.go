package actioncatalog

import (
	"context"
	"encoding/json"
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
