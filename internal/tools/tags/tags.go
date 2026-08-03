// Package tags declares the Portainer environment-tag actions.
//
// Three actions: list, create, delete. It is the smallest domain in the
// catalog that includes a genuinely destructive action, which makes it the
// pilot for that classification — tags.delete removes a tag outright, with no
// softer alternative the way some domains offer a stop/disable short of
// deletion.
//
// tags.list runs on cmd/gen_action_inputs's generated code (actions.gen.go):
// TagList takes no input, its handler is a bare resp.JSON200 passthrough with
// no hand-shaped acknowledgement, and no existing test pins a guard clause a
// generated handler would not have. tags.create and tags.delete stay
// hand-written here: both have an existing unit test
// (TestTagCreate_MissingName_ReturnsErrorWithoutCallingAPI,
// TestTagDelete_InvalidID_ReturnsErrorWithoutCallingAPI) that calls the
// handler directly, bypassing tools.Execute's own central schema validation,
// and asserts the handler itself refuses invalid input before ever reaching
// the network — a property cmd/gen_action_inputs's generated handlers
// deliberately do not have (see cmd/gen_action_inputs's own package doc: "No
// required-field checks ... a second hand-rolled check in each of 441
// handlers would drift from the schema"). Swapping either to generated code
// would either break that test or require weakening it, both of which this
// task's brief rules out ("every existing test must stay green ... do not
// adjust the test"); see plan/carry-forward.md for the full account.
package tags

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every tag action: generatedSpecs()'s one entry (tags.list)
// plus the two kept hand-written above.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(), toolutil.ActionSpec{
		Name: "tags.create", Domain: "tags", OperationID: "TagCreate",
		Title:       "Create a tag",
		Description: "Creates a new environment tag with the given name.",
		Edition:     edition.CE,
		Mutating:    true,
		Handler:     tagCreate,
		Input:       tagCreateInput{},
	}, toolutil.ActionSpec{
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

// narrative supplies TagList's ActionSpec narrative fields to
// generatedSpecs() (see actions.gen.go): only Title/Description, preserving
// the exact wording this domain hand-authored before the swap to generated
// code, rather than letting it silently degrade to the vendored
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
	default:
		return toolutil.ActionNarrative{}
	}
}

func tagCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params tagCreateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("tags create: parse input: %w", err)
	}
	if params.Name == "" {
		return nil, fmt.Errorf("tags create: name is required")
	}

	resp, err := c.API.TagCreateWithResponse(ctx, apigen.TagCreateJSONRequestBody{Name: params.Name})
	if err != nil {
		return nil, fmt.Errorf("tags create: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("tags create: %w", err)
	}
	return resp.JSON200, nil
}

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
