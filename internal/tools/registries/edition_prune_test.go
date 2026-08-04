package registries

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/dynamic"
	"github.com/jmrplens/portainer-mcp/internal/tools/individual"
	"github.com/jmrplens/portainer-mcp/internal/tools/meta"
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

// connectEditionPruneTestSession wires an in-memory MCP client to server, the
// same pairing internal/wiring/server_test.go and internal/tools/individual's
// own tests use: no network or subprocess involved, just the SDK's paired
// transports.
func connectEditionPruneTestSession(t *testing.T, server *mcp.Server) (*mcp.ClientSession, context.Context) {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "edition-prune-test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

// TestUnit_CommunityCatalog_AllThreeSurfaces_NeverPublishGithubOrEndpointID is
// C1's surface-level proof, the one the fourteen-agent review demanded because
// every guard up to this point exercised pruning only through
// Catalog.InputSchema — a method with exactly one non-test caller in the
// whole repository (cmd/audit_spec_drift's own informational field count) —
// while individual, meta and dynamic each called the unpruned
// spec.InputSchema() directly. A Community server driven through any of the
// three surfaces a model actually talks to still saw "github"
// (registries.create/update) and "endpointId" (registries.inspect).
//
// This builds a real Community catalog from registries.Specs(), registers
// all three surfaces on real *mcp.Server instances connected over an
// in-memory MCP session (mirroring internal/wiring/server_test.go's own
// connect helper), and asserts the two Business-Edition-only fields appear in
// none of them: individual's own per-action tool InputSchema, meta's
// domain-tool Description (which embeds each action's complete JSON Schema by
// default — PORTAINER_META_PARAM_SCHEMA is unset in this process, so
// paramSchemaFull applies), and dynamic's portainer_find_action response.
func TestUnit_CommunityCatalog_AllThreeSurfaces_NeverPublishGithubOrEndpointID(t *testing.T) {
	t.Parallel()

	ceCatalog, err := actioncatalog.Build(Specs(), actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build(CE): %v", err)
	}

	// The two Business-Edition-only fields task 7 hand-tagged (see this
	// file's own first test above): "github" on registries.create/update,
	// "endpointId" on registries.inspect.
	gated := []string{"github", "endpointId"}

	t.Run("individual", func(t *testing.T) {
		t.Parallel()
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		if err := (individual.Surface{}).Register(server, ceCatalog, tools.Deps{}); err != nil {
			t.Fatalf("individual.Register: %v", err)
		}
		session, ctx := connectEditionPruneTestSession(t, server)
		res, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		for _, tool := range res.Tools {
			schema, ok := tool.InputSchema.(map[string]any)
			if !ok {
				continue
			}
			props, _ := schema["properties"].(map[string]any)
			for _, field := range gated {
				if _, present := props[field]; present {
					t.Errorf("individual tool %q publishes %q in its InputSchema, want it pruned for a Community catalog", tool.Name, field)
				}
			}
		}
	})

	t.Run("meta", func(t *testing.T) {
		t.Parallel()
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		if err := (meta.Surface{}).Register(server, ceCatalog, tools.Deps{}); err != nil {
			t.Fatalf("meta.Register: %v", err)
		}
		session, ctx := connectEditionPruneTestSession(t, server)
		res, err := session.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		found := false
		for _, tool := range res.Tools {
			if tool.Name != "portainer_registries" {
				continue
			}
			found = true
			for _, field := range gated {
				if strings.Contains(tool.Description, field) {
					t.Errorf("meta tool %q description contains %q, want it pruned for a Community catalog:\n%s", tool.Name, field, tool.Description)
				}
			}
		}
		if !found {
			t.Fatal("portainer_registries tool not found on the meta surface")
		}
	})

	t.Run("dynamic", func(t *testing.T) {
		t.Parallel()
		server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
		if err := (dynamic.Surface{}).Register(server, ceCatalog, tools.Deps{}); err != nil {
			t.Fatalf("dynamic.Register: %v", err)
		}
		session, ctx := connectEditionPruneTestSession(t, server)
		for _, query := range []string{"registries create", "registries update", "registries inspect"} {
			res, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "portainer_find_action",
				Arguments: map[string]any{"query": query},
			})
			if err != nil {
				t.Fatalf("CallTool(portainer_find_action, %q): %v", query, err)
			}
			text, ok := res.Content[0].(*mcp.TextContent)
			if !ok {
				t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
			}
			for _, field := range gated {
				if strings.Contains(text.Text, field) {
					t.Errorf("dynamic find_action(%q) response contains %q, want it pruned for a Community catalog:\n%s", query, field, text.Text)
				}
			}
		}
	})
}

// TestUnit_ValidateInput_CommunityCatalog_RejectsEEOnlyField is C1's second
// demanded proof: not just that "endpointId" is no longer advertised on a
// Community catalog, but that execution is protected too. Before this
// mechanism was wired to a live path, ValidateInput(registries.inspect,
// {"id":1,"endpointId":3}) accepted the call: resolvedInputSchema
// (toolutil/schema.go) built its resolved schema from spec.InputSchema()'s
// own unpruned map, in which "endpointId" was simply a legitimate optional
// property, so a Community caller could send it and have it forwarded.
//
// Once InputSchema() returns the catalog-baked, pruned shape,
// "endpointId" is not merely absent from "properties" — every struct-derived
// schema this project publishes also carries "additionalProperties": false
// (google/jsonschema-go's reflector sets it for every struct type; see
// toolutil/schema.go's InputSchema), so a payload naming a property the
// schema no longer declares is refused as an unexpected one, not silently
// accepted as "unadvertised but tolerated".
func TestUnit_ValidateInput_CommunityCatalog_RejectsEEOnlyField(t *testing.T) {
	t.Parallel()

	ceCatalog, err := actioncatalog.Build(Specs(), actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build(CE): %v", err)
	}
	ceSpec, ok := ceCatalog.Lookup("registries.inspect")
	if !ok {
		t.Fatal("registries.inspect not found in the Community catalog")
	}
	if err := ceSpec.ValidateInput(json.RawMessage(`{"id":1,"endpointId":3}`)); err == nil {
		t.Fatal("ValidateInput({id, endpointId}) on a Community catalog = nil error, want the Business-Edition-only \"endpointId\" refused")
	}

	// Discriminating half: the identical payload against the Business catalog,
	// where "endpointId" is a genuine parameter, must be accepted — a fix that
	// simply refused every unknown-looking field regardless of edition (or
	// broke ValidateInput generally) would pass the CE half above for the
	// wrong reason.
	eeCatalog, err := actioncatalog.Build(Specs(), actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build(EE): %v", err)
	}
	eeSpec, ok := eeCatalog.Lookup("registries.inspect")
	if !ok {
		t.Fatal("registries.inspect not found in the Business catalog")
	}
	if err := eeSpec.ValidateInput(json.RawMessage(`{"id":1,"endpointId":3}`)); err != nil {
		t.Errorf("ValidateInput({id, endpointId}) on a Business catalog = %v, want nil: endpointId is a legitimate Business Edition parameter", err)
	}
}
