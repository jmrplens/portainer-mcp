// Package meta projects the catalog as one MCP tool per domain.
//
// Each tool takes an action name and that action's input. This trades a
// discoverability cost — the model must read the description to learn what
// actions exist — for a tool list two orders of magnitude smaller than the
// individual surface.
package meta

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Surface registers one tool per domain.
type Surface struct{}

// Input is what a meta-tool accepts.
//
// Input's own field is typed as map[string]any rather than json.RawMessage.
// The MCP SDK infers each tool's input schema by reflection, and
// json.RawMessage — being a []byte under the hood — infers to a JSON Schema
// array of integers, not an arbitrary object; a real object argument then
// fails schema validation before the handler ever runs. map[string]any
// infers to a permissive object schema instead, since each action validates
// its own parameters once inside the handler.
type Input struct {
	Action string         `json:"action" jsonschema:"the canonical action name, such as system.info"`
	Input  map[string]any `json:"input,omitempty" jsonschema:"the chosen action's own parameters"`
}

// Register adds one tool per domain in the catalog.
func (Surface) Register(server *mcp.Server, catalog *actioncatalog.Catalog, deps tools.Deps) error {
	for _, domain := range catalog.Domains() {
		actions := catalog.ByDomain(domain)
		byName := make(map[string]toolutil.ActionSpec, len(actions))
		for _, spec := range actions {
			byName[spec.Name] = spec
		}
		names := make([]string, 0, len(actions))
		for _, spec := range actions {
			names = append(names, spec.Name)
		}

		mcp.AddTool(server, &mcp.Tool{
			Name:        "portainer_" + domain,
			Title:       describeDomain(domain),
			Description: describeActions(domain, actions),
			Annotations: domainAnnotations(actions),
		}, func(ctx context.Context, _ *mcp.CallToolRequest, in Input) (*mcp.CallToolResult, any, error) {
			spec, ok := byName[in.Action]
			if !ok {
				return &mcp.CallToolResult{
					IsError: true,
					Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
						"unknown action %q for domain %q. Valid actions: %s",
						in.Action, domain, strings.Join(names, ", "))}},
				}, nil, nil
			}
			payload := in.Input
			if payload == nil {
				payload = map[string]any{}
			}
			input, err := json.Marshal(payload)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: encode input: %w", in.Action, err)
			}
			res, err := tools.Execute(ctx, spec, deps, input)
			if err != nil {
				return nil, nil, fmt.Errorf("%s: %w", spec.Name, err)
			}
			return res, nil, nil
		})
	}
	return nil
}

func describeDomain(domain string) string {
	return "Portainer " + strings.ReplaceAll(domain, "_", " ")
}

// describeActions enumerates the domain's actions in the tool description,
// because on this surface the description is the only place a model can
// discover what the tool can do.
func describeActions(domain string, actions []toolutil.ActionSpec) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Performs %s operations on Portainer. Choose one with the `action` parameter and pass that action's parameters in `input`.\n\nAvailable actions:\n", domain)
	for _, spec := range actions {
		marker := ""
		switch {
		case spec.Destructive:
			marker = " [destructive]"
		case spec.Mutating:
			marker = " [mutating]"
		}
		fmt.Fprintf(&b, "- `%s`%s — %s\n", spec.Name, marker, spec.Description)
	}
	return b.String()
}

// domainAnnotations describe the whole tool, so they must reflect the most
// dangerous action reachable through it: a tool that can delete must not be
// annotated read-only.
func domainAnnotations(actions []toolutil.ActionSpec) *mcp.ToolAnnotations {
	a := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := false
	for _, spec := range actions {
		if spec.Mutating {
			a.ReadOnlyHint = false
		}
		if spec.Destructive {
			destructive = true
		}
	}
	if !a.ReadOnlyHint {
		a.DestructiveHint = &destructive
	}
	return a
}
