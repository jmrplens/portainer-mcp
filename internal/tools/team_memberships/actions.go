// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package team_memberships

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// teamMembershipCreate is the generated handler for operation TeamMembershipCreate.
func teamMembershipCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.TeamMembershipCreateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("TeamMembershipCreate: parse request body: %w", err)
	}
	resp, err := c.API.TeamMembershipCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("TeamMembershipCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamMembershipCreate: %w", err)
	}
	return resp.JSON200, nil
}

// teamMembershipDelete is the generated handler for operation TeamMembershipDelete.
func teamMembershipDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamMembershipDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamMembershipDelete: parse input: %w", err)
	}
	resp, err := c.API.TeamMembershipDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("TeamMembershipDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamMembershipDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// teamMembershipList is the generated handler for operation TeamMembershipList.
func teamMembershipList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.TeamMembershipListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("TeamMembershipList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamMembershipList: %w", err)
	}
	return resp.JSON200, nil
}

// teamMembershipUpdate is the generated handler for operation TeamMembershipUpdate.
func teamMembershipUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamMembershipUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamMembershipUpdate: parse input: %w", err)
	}
	var body apigen.TeamMembershipUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("TeamMembershipUpdate: parse request body: %w", err)
	}
	resp, err := c.API.TeamMembershipUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("TeamMembershipUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamMembershipUpdate: %w", err)
	}
	return resp.JSON200, nil
}

// teamMemberships is the generated handler for operation TeamMemberships.
func teamMemberships(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params teamMembershipsInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("TeamMemberships: parse input: %w", err)
	}
	resp, err := c.API.TeamMembershipsWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("TeamMemberships: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("TeamMemberships: %w", err)
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
			Name: "team_memberships.create", Domain: "team_memberships", OperationID: "TeamMembershipCreate",
			Title:       "Create a new team membership",
			Description: "Create a new team memberships. Access is only available to administrators or leaders of the associated team.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     teamMembershipCreate,
			Input:       teamMembershipCreateInput{},
		}, narrative("TeamMembershipCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "team_memberships.delete", Domain: "team_memberships", OperationID: "TeamMembershipDelete",
			Title:       "Remove a team membership",
			Description: "Remove a team membership. Access is only available to administrators or leaders of the associated team.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     teamMembershipDelete,
			Input:       teamMembershipDeleteInput{},
		}, narrative("TeamMembershipDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "team_memberships.list", Domain: "team_memberships", OperationID: "TeamMembershipList",
			Title:       "List team memberships",
			Description: "List team memberships. Access is only available to administrators and team leaders. Team leaders only see memberships of teams they lead.",
			Edition:     edition.CE,
			Handler:     teamMembershipList,
		}, narrative("TeamMembershipList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "team_memberships.update", Domain: "team_memberships", OperationID: "TeamMembershipUpdate",
			Title:       "Update a team membership",
			Description: "Update a team membership. Access is only available to administrators or leaders of the associated team.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     teamMembershipUpdate,
			Input:       teamMembershipUpdateInput{},
		}, narrative("TeamMembershipUpdate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "team_memberships.list_for_team", Domain: "team_memberships", OperationID: "TeamMemberships",
			Title:       "List team memberships",
			Description: "List team memberships. Access is only available to administrators and team leaders.",
			Edition:     edition.CE,
			Handler:     teamMemberships,
			Input:       teamMembershipsInput{},
		}, narrative("TeamMemberships")),
	}
}
