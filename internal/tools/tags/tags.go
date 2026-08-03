// Package tags declares the Portainer environment-tag actions.
//
// Three actions: list, create, delete. It is the smallest domain in the
// catalog that includes a genuinely destructive action, which makes it the
// pilot for that classification — tags.delete removes a tag outright, with no
// softer alternative the way some domains offer a stop/disable short of
// deletion.
//
// tags.list and tags.create run on cmd/gen_action_inputs's generated code
// (actions.gen.go). TagList takes no input, its handler is a bare
// resp.JSON200 passthrough with no hand-shaped acknowledgement, and no
// existing test pins a guard clause a generated handler would not have.
// TagCreate's only existing test that calls the handler directly
// (TestTagCreate_MissingName_ReturnsErrorWithoutCallingAPI) asserts a
// required-field check — exactly what tools.Execute's central schema
// validation now enforces for every surface (see cmd/gen_action_inputs's own
// package doc: "No required-field checks ... a second hand-rolled check in
// each of 441 handlers would drift from the schema"), so that test now
// routes through tools.Execute instead of calling the handler directly: not
// a weaker check, the same check, asserted on the path every real caller
// actually takes. PortainerTag (TagCreate's and TagList's response type)
// carries nothing credential-shaped either, so neither needed a redaction
// wrapper the way registries.list/create did (see task-4b's report).
//
// tags.delete stays hand-written: its existing test
// (TestTagDelete_InvalidID_ReturnsErrorWithoutCallingAPI) asserts the handler
// refuses a non-positive id *before* the request reaches the network. That is
// not a required-field check — the id is present, just out of range — and
// tools.Execute's schema validation has no "minimum" keyword support (see
// internal/toolutil/schema.go's EnumParams doc comment), so there is no
// equivalent path to route this test through; swapping tags.delete to
// generated code would genuinely weaken this guard, not just relocate it.
package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every tag action: generatedSpecs()'s two entries (tags.list,
// tags.create) plus tags.delete, kept hand-written above.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), toolutil.ActionSpec{
		Name: "tags.delete", Domain: "tags", OperationID: "TagDelete",
		Title:       "Delete a tag",
		Description: "Permanently removes a tag, unassigning it from every environment and environment group that carries it. This cannot be undone.",
		Edition:     edition.CE,
		Mutating:    true,
		Destructive: true,
		Handler:     tagDelete,
		Input:       tagDeleteInput{},
	})
}

// narrative supplies TagList's and TagCreate's ActionSpec narrative fields to
// generatedSpecs() (see actions.gen.go): only Title/Description, preserving
// the exact wording this domain hand-authored before each's swap to
// generated code, rather than letting it silently degrade to the vendored
// specification's own terser summary/description. Every other operationId
// returns the zero toolutil.ActionNarrative — nothing else in this domain is
// generated today.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "TagList":
		return toolutil.ActionNarrative{
			Title:       "List tags",
			Description: "Returns every environment tag defined on this Portainer server.",
		}
	case "TagCreate":
		return toolutil.ActionNarrative{
			Title:       "Create a tag",
			Description: "Creates a new environment tag with the given name.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}

// tagCreate is no longer declared here: it runs on cmd/gen_action_inputs's
// generated code (actions.gen.go) — see this file's package doc. PortainerTag
// carries nothing credential-shaped, so unlike registries.list/create it
// needed no redaction wrapper to become eligible.

func tagDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params tagDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("tags delete: parse input: %w", err)
	}
	if params.ID <= 0 {
		return nil, fmt.Errorf("tags delete: id must be a positive integer, got %d", params.ID)
	}

	resp, err := c.API.TagDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("tags delete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("tags delete: %w", err)
	}
	return map[string]any{"deleted": true, "id": params.ID}, nil
}
