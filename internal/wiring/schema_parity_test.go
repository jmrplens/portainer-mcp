package wiring

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// parityInput is a fixture action's parameter shape with one required field
// ("name" carries no omitempty, so google/jsonschema-go's reflector marks it
// required — see jsonschema-go's infer.go). It stands in for a real case this
// regression is named after: registries.create's "authentication", corrected
// from hand-declared-optional to required because both the vendored
// specification and the independently generated wire client agree it is.
type parityInput struct {
	Name string `json:"name"`
}

// parityCatalog builds a tiny, real catalog carrying exactly one mutating
// action whose Input has one required field. A dedicated fixture action,
// rather than a real domain such as tags.create, keeps this test from ever
// needing to change when a real action's schema does — the property under
// test is structural (does Execute enforce the schema, not what any
// particular domain's schema happens to require).
func parityCatalog(t *testing.T) *actioncatalog.Catalog {
	t.Helper()
	specs := []toolutil.ActionSpec{{
		// OperationID borrows a real operation ("TagCreate") that both
		// vendored specifications declare: actioncatalog.Build resolves every
		// OperationID against the real applicability tables and refuses one
		// that "resolves in neither edition", so a made-up ID cannot be used
		// here even though this fixture action's own name and Input are
		// otherwise entirely synthetic.
		Name: "parity.create", Domain: "parity", OperationID: "TagCreate",
		Title: "Create a parity fixture", Description: "d", Edition: edition.CE,
		Mutating: true,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
			return map[string]any{"created": true}, nil
		},
		Input: parityInput{},
	}}
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.CE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("actioncatalog.Build: %v", err)
	}
	return catalog
}

// parityParams builds the call arguments for parity.create on the given
// surface, carrying whatever input the caller supplies. It is the one place
// that knows how each surface's call shape differs (a bare tool per action
// versus the {action, input} wrapper), so the tests below can assert on
// outcome without repeating that per surface.
func parityParams(surface config.ToolSurface, input map[string]any) *mcp.CallToolParams {
	switch surface {
	case config.SurfaceIndividual:
		return &mcp.CallToolParams{Name: "portainer_parity_create", Arguments: input}
	case config.SurfaceMeta:
		return &mcp.CallToolParams{
			Name:      "portainer_parity",
			Arguments: map[string]any{"action": "parity.create", "input": input},
		}
	default:
		return &mcp.CallToolParams{
			Name:      "portainer_execute_action",
			Arguments: map[string]any{"action": "parity.create", "input": input},
		}
	}
}

// newParityServer wires a server for the given surface with deps, so tests
// can choose whether parity.create's handler actually needs to run (which
// needs a non-nil Deps.Client: Execute's nil-client guard fires regardless of
// whether the handler itself would dereference it) or never reaches that far
// (a call schema validation already refused).
func newParityServer(t *testing.T, surface config.ToolSurface, catalog *actioncatalog.Catalog, deps tools.Deps) (*mcp.ClientSession, context.Context) {
	t.Helper()
	cfg := &config.Config{URL: "https://portainer.example.com", Token: "ptr_abc", ToolSurface: surface}
	server, err := NewServer(cfg, catalog, deps, edition.CE, "2.44.0")
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return connect(t, server)
}

// TestSchemaValidation_MissingRequiredField_RefusedIdenticallyAcrossSurfaces
// is the regression test for the asymmetry this task exists to close: before
// tools.Execute validated input itself, the individual surface refused a call
// missing a required field (the MCP SDK validates a typed tool's arguments
// against its published InputSchema automatically) while meta and dynamic
// executed the identical call, because both accept a generic {action, input}
// wrapper whose own schema does not mention the chosen action's fields.
//
// Every surface must refuse, and every surface's message must name the
// missing field ("name") — the one thing a validation refusal produces that
// no other failure mode would (a missing client or an unknown action would
// not mention it), which is the discriminating assertion the standing
// warning about non-discriminating checks calls for.
//
// Only meta and dynamic are required to also name the action
// ("parity.create") and point at portainer_find_action: those two surfaces
// route the same generic tool through many actions, so their message is the
// only place a model can learn which one was rejected. The individual
// surface does not repeat that — its message is the SDK's own, produced by
// the schema validation built into a typed MCP tool, before tools.Execute's
// spec.ValidateInput ever runs — but it does not need to: on that surface the
// tool the model called (portainer_parity_create) already names the action.
//
// A nil Deps.Client is deliberate, not an oversight: Execute's nil-client
// guard only runs once a call clears schema validation, so if this test ever
// started passing only because of a missing client rather than a rejected
// schema, giving it a working client would make that failure visible instead
// of silently passing for the wrong reason.
func TestSchemaValidation_MissingRequiredField_RefusedIdenticallyAcrossSurfaces(t *testing.T) {
	t.Parallel()
	catalog := parityCatalog(t)

	for _, surface := range []config.ToolSurface{config.SurfaceIndividual, config.SurfaceMeta, config.SurfaceDynamic} {
		t.Run(string(surface), func(t *testing.T) {
			t.Parallel()
			session, ctx := newParityServer(t, surface, catalog, tools.Deps{})
			params := parityParams(surface, map[string]any{})

			res, err := session.CallTool(ctx, params)
			if err != nil {
				t.Fatalf("CallTool(%s): %v", params.Name, err)
			}

			if !res.IsError {
				t.Fatalf("surface %q: call missing the required \"name\" field succeeded, want a refusal", surface)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "name") {
				t.Errorf("surface %q: error = %q, want it to name the missing field \"name\"", surface, text)
			}
			if surface == config.SurfaceIndividual {
				return
			}
			if !strings.Contains(text, "parity.create") {
				t.Errorf("surface %q: error = %q, want it to name the action \"parity.create\"", surface, text)
			}
			if !strings.Contains(text, "portainer_find_action") {
				t.Errorf("surface %q: error = %q, want it to point at portainer_find_action to discover the schema", surface, text)
			}
		})
	}
}

// TestSchemaValidation_ValidInput_RunsOnEveryValidSurface is the positive
// counterpart: the same action, given input that does satisfy its schema,
// must actually run — and reach the real handler, not merely avoid an error —
// on every surface. Without this, a validator strict enough to refuse
// everything would still pass the test above.
func TestSchemaValidation_ValidInput_RunsOnEveryValidSurface(t *testing.T) {
	t.Parallel()
	catalog := parityCatalog(t)

	for _, surface := range []config.ToolSurface{config.SurfaceIndividual, config.SurfaceMeta, config.SurfaceDynamic} {
		t.Run(string(surface), func(t *testing.T) {
			t.Parallel()
			// A non-nil client: this proves the call reached parity.create's
			// handler, not merely that no error surfaced. Execute's own
			// nil-client guard would otherwise mask that distinction, exactly
			// as noted on the fixtures in internal/tools/register_test.go.
			session, ctx := newParityServer(t, surface, catalog, tools.Deps{Client: &portainer.Client{}})
			params := parityParams(surface, map[string]any{"name": "ok"})

			res, err := session.CallTool(ctx, params)
			if err != nil {
				t.Fatalf("CallTool(%s): %v", params.Name, err)
			}
			if res.IsError {
				t.Fatalf("surface %q: valid input was refused: %+v", surface, res.Content)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			if !strings.Contains(text, "created") {
				t.Errorf("surface %q: result = %q, want the handler's own \"created\" field, not just a non-error result", surface, text)
			}
		})
	}
}
