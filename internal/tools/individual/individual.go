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
		mcp.AddTool(server, &mcp.Tool{
			Name:        ToolName(spec.Name),
			Title:       spec.Title,
			Description: spec.Description,
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
