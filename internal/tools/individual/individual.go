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
func ToolName(actionName string) string {
	out := make([]rune, 0, len(actionName)+10)
	out = append(out, []rune("portainer_")...)
	for _, r := range actionName {
		if r == '.' {
			r = '_'
		}
		out = append(out, r)
	}
	return string(out)
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
