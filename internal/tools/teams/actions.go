// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package teams

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// teamCreate is the generated handler for operation TeamCreate.
func teamCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.TeamCreateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("TeamCreate: parse request body: %w", err)
	}
	resp, err := c.API.TeamCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("TeamCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamCreate: %w", err)
	}
	return resp.JSON200, nil
}

// teamDelete is the generated handler for operation TeamDelete.
func teamDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamDelete: parse input: %w", err)
	}
	resp, err := c.API.TeamDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("TeamDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// teamInspect is the generated handler for operation TeamInspect.
func teamInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamInspect: parse input: %w", err)
	}
	resp, err := c.API.TeamInspectWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("TeamInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamInspect: %w", err)
	}
	return resp.JSON200, nil
}

// teamList is the generated handler for operation TeamList.
func teamList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.TeamListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("TeamList: parse query parameters: %w", err)
	}
	resp, err := c.API.TeamListWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("TeamList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamList: %w", err)
	}
	return resp.JSON200, nil
}

// teamUpdate is the generated handler for operation TeamUpdate.
func teamUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamUpdate: parse input: %w", err)
	}
	var body apigen.TeamUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("TeamUpdate: parse request body: %w", err)
	}
	resp, err := c.API.TeamUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("TeamUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamUpdate: %w", err)
	}
	return resp.JSON200, nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "teams.create", Domain: "teams", OperationID: "TeamCreate",
			Title:       "Create a new team",
			Description: "Create a new team.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     teamCreate,
			Input:       teamCreateInput{},
		}, narrative("TeamCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "teams.delete", Domain: "teams", OperationID: "TeamDelete",
			Title:       "Remove a team",
			Description: "Remove a team.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     teamDelete,
			Input:       teamDeleteInput{},
		}, narrative("TeamDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "teams.inspect", Domain: "teams", OperationID: "TeamInspect",
			Title:       "Inspect a team",
			Description: "Retrieve details about a team. Access is only available for administrator and leaders of that team.",
			Edition:     edition.CE,
			Handler:     teamInspect,
			Input:       teamInspectInput{},
		}, narrative("TeamInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "teams.list", Domain: "teams", OperationID: "TeamList",
			Title:       "List teams",
			Description: "List teams. All authenticated users can list all teams (teams only expose ID and Name).",
			Edition:     edition.CE,
			Handler:     teamList,
			Input:       teamListInput{},
		}, narrative("TeamList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "teams.update", Domain: "teams", OperationID: "TeamUpdate",
			Title:       "Update a team",
			Description: "Update a team.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     teamUpdate,
			Input:       teamUpdateInput{},
		}, narrative("TeamUpdate")),
	}
}
