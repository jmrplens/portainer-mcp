// Package dynamic projects the catalog as two tools regardless of its size.
//
// A model calls portainer_find_action to discover an action, then
// portainer_execute_action to run it. This keeps the model-facing tool list at
// two entries whether the catalog holds five actions or 442, which is why it is
// the default surface.
package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Surface registers the find and execute pair.
type Surface struct{}

// FindInput is what portainer_find_action accepts.
type FindInput struct {
	Query string `json:"query" jsonschema:"what you want to do, in plain words — for example 'list stacks' or 'delete a tag'"`
}

// ExecuteInput is what portainer_execute_action accepts.
//
// Input is map[string]any, not json.RawMessage. The SDK infers a tool's input
// schema from this struct, and json.RawMessage is []byte, which infers to an
// array of integers 0-255 — so a client sending a real object would be
// rejected by schema validation before dispatch ever ran. Verified against
// go-sdk v1.7.0.
type ExecuteInput struct {
	Action string         `json:"action" jsonschema:"the canonical action name returned by portainer_find_action"`
	Input  map[string]any `json:"input,omitempty" jsonschema:"that action's parameters"`
}

// match is one search result.
type match struct {
	Action      string `json:"action"`
	Domain      string `json:"domain"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Mutating    bool   `json:"mutating"`
	Destructive bool   `json:"destructive"`
	score       int
}

// Register adds portainer_find_action and portainer_execute_action.
func (Surface) Register(server *mcp.Server, catalog *actioncatalog.Catalog, deps tools.Deps) error {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_find_action",
		Title:       "Find a Portainer action",
		Description: "Searches the available Portainer actions and returns those matching your query, with the parameters each one takes. Call this first, then pass the chosen action's name to portainer_execute_action. Results say whether an action mutates or destroys state.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(_ context.Context, _ *mcp.CallToolRequest, in FindInput) (*mcp.CallToolResult, any, error) {
		matches := search(catalog.Actions(), in.Query)
		if len(matches) == 0 {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
				"No action matched %q. Try broader words, or a domain name: %s",
				in.Query, strings.Join(catalog.Domains(), ", "))}}}, nil, nil
		}
		encoded, err := json.MarshalIndent(matches, "", "  ")
		if err != nil {
			return nil, nil, fmt.Errorf("encode matches: %w", err)
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "portainer_execute_action",
		Title:       "Execute a Portainer action",
		Description: "Runs an action discovered with portainer_find_action. Pass its canonical name and that action's parameters.",
		Annotations: executeAnnotations(catalog),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ExecuteInput) (*mcp.CallToolResult, any, error) {
		spec, ok := catalog.Lookup(in.Action)
		if !ok {
			return &mcp.CallToolResult{
				IsError: true,
				Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(
					"Unknown action %q. Call portainer_find_action to discover the available actions.", in.Action)}},
			}, nil, nil
		}
		input, err := json.Marshal(in.Input)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: encode input: %w", in.Action, err)
		}
		res, err := tools.Execute(ctx, spec, deps, input)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: %w", spec.Name, err)
		}
		return res, nil, nil
	})

	return nil
}

// search ranks actions against a query. Matching is deliberately simple: an
// exact name match beats a name substring, which beats a title or description
// hit. P3 can add synonyms and fuzzy matching once the catalog is large enough
// for the difference to be measurable.
func search(specs []toolutil.ActionSpec, query string) []match {
	terms := strings.Fields(strings.ToLower(strings.TrimSpace(query)))
	if len(terms) == 0 {
		return nil
	}

	var out []match
	for _, spec := range specs {
		name := strings.ToLower(spec.Name)
		haystack := strings.ToLower(spec.Name + " " + spec.Domain + " " + spec.Title + " " + spec.Description)

		score := 0
		for _, term := range terms {
			switch {
			case name == term:
				score += 100
			case strings.Contains(name, term):
				score += 10
			case strings.Contains(haystack, term):
				score++
			}
		}
		if score == 0 {
			continue
		}
		out = append(out, match{
			Action: spec.Name, Domain: spec.Domain, Title: spec.Title,
			Description: spec.Description, Mutating: spec.Mutating,
			Destructive: spec.Destructive, score: score,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].Action < out[j].Action
	})
	return out
}

// executeAnnotations describe the execute tool, which can reach anything in the
// catalog, so it must advertise the most dangerous action available.
func executeAnnotations(catalog *actioncatalog.Catalog) *mcp.ToolAnnotations {
	a := &mcp.ToolAnnotations{ReadOnlyHint: true}
	destructive := false
	for _, spec := range catalog.Actions() {
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
