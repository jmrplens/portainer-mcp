// Package tools holds the single execution path every tool surface routes
// through, and the surface interface they implement.
package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Deps are what an action needs at execution time.
type Deps struct {
	Client   *portainer.Client
	Logger   *slog.Logger
	SafeMode bool
}

// Surface projects a catalog onto an MCP server.
type Surface interface {
	Register(server *mcp.Server, catalog *actioncatalog.Catalog, deps Deps) error
}

// Execute runs one action, applying safe mode.
//
// Every surface routes through here rather than calling a handler directly.
// That is deliberate: read-only mode is enforced by the catalog, which never
// contains a mutating action when it is set, and safe mode is enforced here.
// A surface that called handlers itself would bypass both, which is exactly
// how a server ends up advertising protections it does not apply.
func Execute(ctx context.Context, spec toolutil.ActionSpec, deps Deps, input json.RawMessage) (*mcp.CallToolResult, error) {
	// Normalized here, once, rather than in each surface: a surface that
	// marshals a caller-omitted input map (nil) produces the JSON literal
	// null, not {}. A handler's own json.Unmarshal treats null as leaving the
	// target unchanged, so a nil-tolerant handler would not necessarily
	// notice — but the field list safe-mode reports below would still differ
	// by surface, which is the exact class of divergence this catalog exists
	// to prevent. Normalizing centrally means no future surface can omit it.
	input = normalizeInput(input)

	if deps.SafeMode && spec.Mutating {
		return safeModePreview(spec, input), nil
	}

	// Validated here, once, for the same reason input is normalized here: the
	// MCP SDK only validates a typed tool's arguments against its published
	// InputSchema automatically, and the individual surface is the only one
	// that registers a typed tool. A surface that instead accepts a generic
	// {action, input} wrapper — meta and dynamic, the two the owner considers
	// primary — publishes a permissive schema for that wrapper, so the SDK
	// never sees the chosen action's own schema and never rejects anything
	// against it. Checking it here, before any surface's difference in
	// registration can matter, is what makes a missing required field refused
	// identically everywhere instead of only on whichever surface happens to
	// register a typed tool.
	//
	// Deliberately after the safe-mode check, not before: a safe-mode preview
	// never runs the handler, so an input that would fail schema validation is
	// no more harmful to preview than a valid one, and TestExecute_SafeMode_
	// MalformedInput_StillReportsSafeMode pins exactly that — safe mode must
	// keep its own framing even over input this check would otherwise refuse.
	if err := spec.ValidateInput(input); err != nil {
		return errorResult(fmt.Sprintf(
			"%s. Call portainer_find_action to see this action's schema (on the dynamic surface), "+
				"or read its published input schema on this surface, then retry with corrected input.",
			err,
		)), nil
	}

	// A nil client means the server was wired wrong. Every handler would
	// dereference it and panic, and a panic inside a tool call takes the whole
	// MCP server down — a tool error does not. Execute is the single path all
	// surfaces route through, so this is the one place worth checking.
	if deps.Client == nil {
		return errorResult(fmt.Sprintf("%s: no Portainer client is configured", spec.Name)), nil
	}

	if deps.Logger != nil {
		deps.Logger.Debug("executing action", "action", spec.Name, "mutating", spec.Mutating)
	}

	out, err := spec.Handler(ctx, deps.Client, input)
	if err != nil {
		if deps.Logger != nil {
			deps.Logger.Error("action failed", "action", spec.Name, "error", err)
		}
		return errorResult(fmt.Sprintf("%s: %v", spec.Name, err)), nil
	}

	encoded, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("%s: encode result: %v", spec.Name, err)), nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}, nil
}

// normalizeInput replaces an omitted action input with an empty JSON object.
//
// A surface that marshals a caller-omitted map[string]any input produces the
// JSON literal null (json.Marshal(nil map) == "null"), not {}. Handling that
// here, once, guarantees every surface delivers the same {} to a handler for
// the same omitted input, rather than depending on each surface to normalize
// it individually before calling Execute.
func normalizeInput(input json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(input)) == 0 || bytes.Equal(bytes.TrimSpace(input), []byte("null")) {
		return json.RawMessage("{}")
	}
	return input
}

// safeModePreview describes what the action would have done, without doing it.
func safeModePreview(spec toolutil.ActionSpec, input json.RawMessage) *mcp.CallToolResult {
	kind := "mutating"
	if spec.Destructive {
		kind = "destructive"
	}
	preview := map[string]any{
		"safe_mode": true,
		"action":    spec.Name,
		"kind":      kind,
		"would_call": map[string]any{
			"operation_id": spec.OperationID,
			// Field names only, never values: this preview travels back through
			// the tool response, where transcripts and observability treat it
			// less carefully than the request, and several domains carry
			// credentials in their input.
			"input_fields": inputFieldNames(input),
		},
		"note": "Safe mode is enabled, so this " + kind + " action was not executed. " +
			"Restart the server without --safe-mode to apply it.",
	}
	encoded, err := json.MarshalIndent(preview, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("%s: encode safe-mode preview: %v", spec.Name, err))
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(encoded)}}}
}

// inputFieldNames lists the top-level keys of a JSON object input, sorted.
//
// It never returns values. It also never fails: an input that is not a JSON
// object — malformed, empty, or a bare scalar — yields nil rather than an
// error, because losing the preview's framing is worse than losing the field
// list.
func inputFieldNames(input json.RawMessage) []string {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return nil
	}
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func errorResult(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: message}},
	}
}

// AnnotationsFor derives the MCP annotations a client uses to decide whether to
// confirm an action with the user.
func AnnotationsFor(spec toolutil.ActionSpec) *mcp.ToolAnnotations {
	a := &mcp.ToolAnnotations{
		Title:          spec.Title,
		ReadOnlyHint:   !spec.Mutating,
		IdempotentHint: spec.Idempotent,
	}
	if spec.Mutating {
		destructive := spec.Destructive
		a.DestructiveHint = &destructive
	}
	return a
}
