// Package individual projects the catalog as one MCP tool per action.
//
// This is the compatibility surface: it produces the largest tool list and the
// highest token cost, and exists for clients that handle neither meta-tools nor
// the dynamic find/execute pair.
package individual

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
)

// Surface registers one tool per action.
type Surface struct{}

// ToolName renders an action's canonical name as an MCP tool name.
// "tags.list" becomes "portainer_tags_list".
//
// This delegates to actioncatalog.RenderToolName rather than keeping its own
// copy: Build's collision check runs against that exact rendering, and a
// second implementation here could silently drift from it, letting two
// actions collide on this surface without Build ever noticing.
func ToolName(actionName string) string {
	return actioncatalog.RenderToolName(actionName)
}

// Register adds every catalog action as its own tool.
func (Surface) Register(server *mcp.Server, catalog *actioncatalog.Catalog, deps tools.Deps) error {
	for _, spec := range catalog.Actions() {
		// The schema is set explicitly, rather than left for AddTool to infer
		// from the handler's map[string]any parameter, precisely because that
		// inference produces a permissive object schema — the placeholder
		// every action published before this schema was reflected from each
		// action's own Input type. AddTool only infers when InputSchema is
		// nil, so setting it here to the real, per-action shape is what
		// makes this surface publish something a model can act on instead of
		// guessing and reading the error.
		schema, err := spec.InputSchema()
		if err != nil {
			return fmt.Errorf("%s: input schema: %w", spec.Name, err)
		}
		// This surface deliberately keeps the generic mcp.AddTool, rather
		// than the server's low-level, non-validating form: the generic form
		// makes the SDK itself validate a typed tool's arguments against
		// InputSchema before the handler ever runs (see google/jsonschema-go's
		// Resolved.Validate, called from the SDK's applySchema). That check is
		// independent of tools.Execute's own spec.ValidateInput call below —
		// on this surface a rejected call never reaches Execute at all — which
		// is exactly what keeps this surface protected even if Execute's
		// validation is ever accidentally removed: meta and dynamic have no
		// schema check of their own to fall back on, so only they would start
		// silently executing invalid input; this surface would not. The two
		// paths do report a rejection in different words (the SDK's own
		// "validating \"arguments\": ..." here, spec.ValidateInput's message
		// through Execute on meta and dynamic) — accepted here rather than
		// suppressed, because on this surface the tool a model called already
		// names the action (there is exactly one tool per action), which
		// meta's and dynamic's generic {action, input} wrapper cannot say for
		// itself and so must state in the message.
		mcp.AddTool(server, &mcp.Tool{
			Name:        ToolName(spec.Name),
			Title:       spec.Title,
			Description: spec.Description,
			InputSchema: schema,
			Annotations: tools.AnnotationsFor(spec),
		}, func(ctx context.Context, _ *mcp.CallToolRequest, input map[string]any) (*mcp.CallToolResult, any, error) {
			raw, err := json.Marshal(input)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: encode input: %w", spec.Name, err)
			}
			res, err := tools.Execute(ctx, spec, deps, raw)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", spec.Name, err)
			}
			return res, nil, nil
		})
	}
	return nil
}
