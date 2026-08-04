package registries

import (
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
)

// TestUnit_EditionGating_GithubAndEndpointIDPruneFromCommunityCatalog is P3.3
// task 7's proof for its second, small fix: registries was frozen before
// P3.3 task 2 added per-field edition gating (an `edition:"EE"` struct tag,
// read by toolutil.FieldEditions and pruned per catalog edition by
// actioncatalog.Catalog.InputSchema), so the generator will never revisit
// this domain's inputs.go to tag the three fields that mechanism was built
// for. This test is also the first real-code exercise of that mechanism
// outside its own package's tests: registryCreateInput.Github,
// registryUpdateInput.Github and registryInspectInput.EndpointID are all
// genuinely Business-Edition-only (confirmed directly against
// api/specs/ce-2.44.0.json: registries.registryCreatePayload and
// registries.registryUpdatePayload declare no "Github" property at all, and
// RegistryInspect's Community parameter list is just ["id"], no
// "endpointId") — a Community Edition model must never be offered a
// GitHub-registry-provider configuration, or an endpointId policy override,
// its own server has never heard of.
//
// Both directions matter: a Community catalog must drop all three, and a
// Business catalog must keep them — a mechanism that pruned unconditionally,
// regardless of target edition, would pass a test that only checked the
// first half.
func TestUnit_EditionGating_GithubAndEndpointIDPruneFromCommunityCatalog(t *testing.T) {
	t.Parallel()

	cases := []struct {
		actionName string
		field      string
	}{
		{"registries.create", "github"},
		{"registries.update", "github"},
		{"registries.inspect", "endpointId"},
	}

	for _, tc := range cases {
		t.Run(tc.actionName, func(t *testing.T) {
			t.Parallel()

			ceCatalog, err := actioncatalog.Build(Specs(), actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
			if err != nil {
				t.Fatalf("actioncatalog.Build(CE): %v", err)
			}
			eeCatalog, err := actioncatalog.Build(Specs(), actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
			if err != nil {
				t.Fatalf("actioncatalog.Build(EE): %v", err)
			}

			spec, ok := ceCatalog.Lookup(tc.actionName)
			if !ok {
				t.Fatalf("action %q not found in the Community catalog", tc.actionName)
			}

			ceSchema, err := ceCatalog.InputSchema(spec)
			if err != nil {
				t.Fatalf("ceCatalog.InputSchema(%s): %v", tc.actionName, err)
			}
			ceProps, _ := ceSchema["properties"].(map[string]any)
			if _, present := ceProps[tc.field]; present {
				t.Errorf("%s: Community input schema still publishes %q, want it pruned (Business Edition only)", tc.actionName, tc.field)
			}

			eeSpec, ok := eeCatalog.Lookup(tc.actionName)
			if !ok {
				t.Fatalf("action %q not found in the Business catalog", tc.actionName)
			}
			eeSchema, err := eeCatalog.InputSchema(eeSpec)
			if err != nil {
				t.Fatalf("eeCatalog.InputSchema(%s): %v", tc.actionName, err)
			}
			eeProps, _ := eeSchema["properties"].(map[string]any)
			if _, present := eeProps[tc.field]; !present {
				t.Errorf("%s: Business input schema does not publish %q, want it kept: only Community should have this pruned", tc.actionName, tc.field)
			}
		})
	}
}
