package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterMetaTools builds and registers all meta-tools on the MCP server.
// In read-only mode, write actions are excluded from the action enum and
// their handlers are not registered. If a meta-tool has no available
// actions after filtering (e.g. all are write-only and read-only is on),
// it is silently skipped.
func (s *PortainerMCPServer) RegisterMetaTools() {
	defs := metaToolDefinitions()
	for _, def := range defs {
		s.registerOneMetaTool(def)
	}
}

// registerOneMetaTool builds a single meta-tool from its definition,
// filtering actions by read-only mode, and registers it.
func (s *PortainerMCPServer) registerOneMetaTool(def metaToolDef) {
	// Filter actions based on read-only mode
	available := make([]metaAction, 0, len(def.actions))
	for _, a := range def.actions {
		if s.readOnly && !a.readOnly {
			continue
		}
		available = append(available, a)
	}

	if len(available) == 0 {
		return
	}

	// Build action enum values and handler dispatch map
	actionNames := make([]string, len(available))
	handlers := make(map[string]server.ToolHandlerFunc, len(available))
	for i, a := range available {
		actionNames[i] = a.name
		handlers[a.name] = a.handler(s)
	}

	// Compute annotation: if ALL remaining actions are read-only, mark the
	// meta-tool as read-only. Otherwise use the definition's annotation.
	annotation := def.annotation
	allReadOnly := true
	for _, a := range available {
		if !a.readOnly {
			allReadOnly = false
			break
		}
	}
	if allReadOnly {
		annotation.ReadOnlyHint = boolPtr(true)
		annotation.DestructiveHint = boolPtr(false)
	}

	// Build the MCP tool programmatically
	tool := mcp.NewTool(def.name,
		mcp.WithDescription(def.description),
		mcp.WithToolAnnotation(annotation),
		mcp.WithString("action",
			mcp.Required(),
			mcp.Description(fmt.Sprintf("The operation to perform. Available actions: %s", strings.Join(actionNames, ", "))),
			mcp.Enum(actionNames...),
		),
	)

	// Register the meta-tool with a routing handler
	s.srv.AddTool(tool, makeMetaHandler(def.name, handlers))
}

// makeMetaHandler creates a ToolHandlerFunc that routes to the correct
// sub-handler based on the "action" parameter.
func makeMetaHandler(metaToolName string, handlers map[string]server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		actionRaw, ok := request.GetArguments()["action"]
		if !ok {
			return mcp.NewToolResultError("missing required parameter: action"), nil
		}

		action, ok := actionRaw.(string)
		if !ok || action == "" {
			return mcp.NewToolResultError("parameter 'action' must be a non-empty string"), nil
		}

		handler, ok := handlers[action]
		if !ok {
			available := make([]string, 0, len(handlers))
			for k := range handlers {
				available = append(available, k)
			}
			return mcp.NewToolResultError(fmt.Sprintf(
				"unknown action '%s' for tool '%s'. Available actions: %s",
				action, metaToolName, strings.Join(available, ", "),
			)), nil
		}

		return handler(ctx, request)
	}
}
