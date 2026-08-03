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
	"log/slog"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Surface registers one tool per domain.
type Surface struct{}

// largeDescriptionThreshold is the rendered description size, in bytes, above
// which Register warns. paramSchemaFull (the default) embeds one complete
// JSON Schema per action into a single domain tool description, and every
// MCP client resends every tool description on every request — a domain with
// many actions carrying large generated inputs (registries, with its nested
// provider and access-policy objects, is the real example) can grow a
// description large enough that an operator would want to know, and switch
// that one domain — or PORTAINER_META_PARAM_SCHEMA globally — to summary or
// none. 16 KiB is comfortably above every pilot domain's current rendered
// size and comfortably below where a single tool description starts to
// dominate a model's context on its own.
const largeDescriptionThreshold = 16 * 1024

// warnIfDescriptionIsLarge logs a warning when description exceeds
// largeDescriptionThreshold, naming the domain and mode so an operator can
// tell which domain to reconfigure (or switch PORTAINER_META_PARAM_SCHEMA
// globally) without inspecting the description itself. logger may be nil —
// Register is exercised in tests and by editions of this surface that never
// configure one — in which case this is a no-op rather than a nil-pointer
// panic.
func warnIfDescriptionIsLarge(logger *slog.Logger, domain string, mode paramSchemaMode, description string) {
	if logger == nil || len(description) <= largeDescriptionThreshold {
		return
	}
	logger.Warn("domain tool description is large; every MCP client request re-sends it",
		"domain", domain, "mode", mode, "bytes", len(description),
		"hint", "set PORTAINER_META_PARAM_SCHEMA=summary or none to shrink it")
}

// paramSchemaEnvVar selects how much parameter detail describeActions embeds
// per action. This surface has only one place to publish a per-action shape —
// the domain tool's own input schema is the same {action, input} shape for
// every action, so it cannot carry it — which is why the description is what
// this switch controls.
const paramSchemaEnvVar = "PORTAINER_META_PARAM_SCHEMA"

// paramSchemaMode is one of the three settings paramSchemaEnvVar accepts.
type paramSchemaMode string

const (
	// paramSchemaFull embeds each action's complete JSON Schema plus its
	// required parameters. This is the default: the owner ruled that token
	// cost is not a constraint here, and hiding the schema — this surface's
	// only behaviour before this switch existed — is what P2 already
	// declared dishonest for portainer_find_action's description.
	paramSchemaFull paramSchemaMode = "full"
	// paramSchemaSummary embeds only the required parameter names, for a
	// caller working under a tight context budget who still wants to know
	// what a call must supply.
	paramSchemaSummary paramSchemaMode = "summary"
	// paramSchemaNone omits parameter detail entirely — this surface's
	// behaviour before schemas were reflected at all. Kept as the tightest
	// escape hatch, never the default.
	paramSchemaNone paramSchemaMode = "none"
)

// resolveParamSchemaMode reads paramSchemaEnvVar directly, rather than
// through internal/config, because this switch is local to how this one
// surface renders its descriptions: no other package needs to know about it,
// and threading it through Config and wiring.SurfaceFor would expose an
// implementation detail of this package everywhere a Surface is constructed.
// An empty value defaults to full, matching the owner's ruling that
// publishing schemas is the default, not an opt-in.
func resolveParamSchemaMode() (paramSchemaMode, error) {
	mode := paramSchemaMode(strings.ToLower(strings.TrimSpace(os.Getenv(paramSchemaEnvVar))))
	switch mode {
	case "":
		return paramSchemaFull, nil
	case paramSchemaFull, paramSchemaSummary, paramSchemaNone:
		return mode, nil
	default:
		return "", fmt.Errorf("invalid %s %q: want full, summary or none", paramSchemaEnvVar, mode)
	}
}

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
	mode, err := resolveParamSchemaMode()
	if err != nil {
		return fmt.Errorf("meta surface: %w", err)
	}

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

		description, err := describeActions(domain, actions, mode)
		if err != nil {
			return fmt.Errorf("meta surface: %s: %w", domain, err)
		}
		warnIfDescriptionIsLarge(deps.Logger, domain, mode, description)

		mcp.AddTool(server, &mcp.Tool{
			Name:        "portainer_" + domain,
			Title:       describeDomain(domain),
			Description: description,
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
			// A nil in.Input (the caller omitted it) is not special-cased here:
			// tools.Execute normalizes a marshaled-to-null input into {} for
			// every surface, so this surface does not need its own copy of
			// that normalization.
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
	}
	return nil
}

func describeDomain(domain string) string {
	return "Portainer " + strings.ReplaceAll(domain, "_", " ")
}

// describeActions enumerates the domain's actions in the tool description,
// because on this surface the description is the only place a model can
// discover what the tool can do — including, per mode, each action's
// parameter shape. paramSchemaNone reproduces this surface's behaviour before
// schemas were reflected at all; it is never the default.
func describeActions(domain string, actions []toolutil.ActionSpec, mode paramSchemaMode) (string, error) {
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

		if mode == paramSchemaNone {
			continue
		}
		schema, err := spec.InputSchema()
		if err != nil {
			return "", fmt.Errorf("%s: input schema: %w", spec.Name, err)
		}
		required := toolutil.RequiredParams(schema)

		switch mode {
		case paramSchemaSummary:
			fmt.Fprintf(&b, "  Required parameters: %s\n", requiredOrNone(required))
		case paramSchemaFull:
			encoded, err := json.Marshal(schema)
			if err != nil {
				return "", fmt.Errorf("%s: encode input schema: %w", spec.Name, err)
			}
			fmt.Fprintf(&b, "  Parameters: %s\n", encoded)
			fmt.Fprintf(&b, "  Required parameters: %s\n", requiredOrNone(required))
		}
	}
	return b.String(), nil
}

// requiredOrNone renders a required-parameter list for the description,
// saying "none" explicitly rather than leaving the line blank: a model must
// be able to tell "this action takes no required parameters" apart from "this
// line was never rendered".
func requiredOrNone(required []string) string {
	if len(required) == 0 {
		return "none"
	}
	return strings.Join(required, ", ")
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
