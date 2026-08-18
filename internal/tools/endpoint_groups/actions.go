// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package endpoint_groups

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// endpointGroupAddEndpoint is the generated handler for operation EndpointGroupAddEndpoint.
func endpointGroupAddEndpoint(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointGroupAddEndpointInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointGroupAddEndpoint: parse input: %w", err)
	}
	resp, err := c.API.EndpointGroupAddEndpointWithResponse(ctx, params.ID, params.EndpointID)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupAddEndpoint: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupAddEndpoint: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointGroupCreate is the generated handler for operation EndpointGroupCreate.
func endpointGroupCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.EndpointGroupCreateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointGroupCreate: parse request body: %w", err)
	}
	resp, err := c.API.EndpointGroupCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupCreate: %w", err)
	}
	return resp.JSON200, nil
}

// endpointGroupDelete is the generated handler for operation EndpointGroupDelete.
func endpointGroupDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointGroupDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointGroupDelete: parse input: %w", err)
	}
	resp, err := c.API.EndpointGroupDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointGroupDeleteEndpoint is the generated handler for operation EndpointGroupDeleteEndpoint.
func endpointGroupDeleteEndpoint(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointGroupDeleteEndpointInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointGroupDeleteEndpoint: parse input: %w", err)
	}
	resp, err := c.API.EndpointGroupDeleteEndpointWithResponse(ctx, params.ID, params.EndpointID)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupDeleteEndpoint: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupDeleteEndpoint: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// endpointGroupList is the generated handler for operation EndpointGroupList.
func endpointGroupList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.EndpointGroupListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("EndpointGroupList: parse query parameters: %w", err)
	}
	resp, err := c.API.EndpointGroupListWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupList: %w", err)
	}
	return resp.JSON200, nil
}

// endpointGroupUpdate is the generated handler for operation EndpointGroupUpdate.
func endpointGroupUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params endpointGroupUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EndpointGroupUpdate: parse input: %w", err)
	}
	var body apigen.EndpointGroupUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("EndpointGroupUpdate: parse request body: %w", err)
	}
	resp, err := c.API.EndpointGroupUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("EndpointGroupUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EndpointGroupUpdate: %w", err)
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
			Name: "endpoint_groups.add_endpoint", Domain: "endpoint_groups", OperationID: "EndpointGroupAddEndpoint",
			Title:       "Add an environment(endpoint) to an environment(endpoint) group",
			Description: "Add an environment(endpoint) to an environment(endpoint) group",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointGroupAddEndpoint,
			Input:       endpointGroupAddEndpointInput{},
		}, narrative("EndpointGroupAddEndpoint")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.create", Domain: "endpoint_groups", OperationID: "EndpointGroupCreate",
			Title:       "Create an Environment(Endpoint) Group",
			Description: "Create a new environment(endpoint) group.",
			// CE's vendored specification gives POST /endpoint_groups no
			// operationId — one of docs/api-divergences.md §6.2's fourteen —
			// so cmd/gen_action_inputs's editionOf, which keys strictly off
			// the vendored Community document's own operationId presence,
			// derives EE here. cmd/gen_applicability's borrowIDsAcrossEditions
			// separately proves the route itself is served by Community (its
			// own spans table has POST /endpoint_groups from 2.27.9 onward)
			// and borrows the Business name into apiversion.operationIDs[CE].
			// Declaring Edition: EE against that borrowed index is exactly
			// the case actioncatalog.Build refuses to build: "declares
			// Edition EE but ... is not exclusive to Business Edition".
			// Edition: CE is the correct declaration, hand-corrected from
			// what the generator derived.
			Edition:  edition.CE,
			Mutating: true,
			Handler:  endpointGroupCreate,
			Input:    endpointGroupCreateInput{},
		}, narrative("EndpointGroupCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.delete", Domain: "endpoint_groups", OperationID: "EndpointGroupDelete",
			Title:       "Remove an environment(endpoint) group",
			Description: "Remove an environment(endpoint) group.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     endpointGroupDelete,
			Input:       endpointGroupDeleteInput{},
		}, narrative("EndpointGroupDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.delete_endpoint", Domain: "endpoint_groups", OperationID: "EndpointGroupDeleteEndpoint",
			Title:       "Removes environment(endpoint) from an environment(endpoint) group",
			Description: "Removes environment(endpoint) from an environment(endpoint) group",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     endpointGroupDeleteEndpoint,
			Input:       endpointGroupDeleteEndpointInput{},
		}, narrative("EndpointGroupDeleteEndpoint")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.list", Domain: "endpoint_groups", OperationID: "EndpointGroupList",
			Title:       "List Environment(Endpoint) groups",
			Description: "List all environment(endpoint) groups based on the current user authorizations. Will\nreturn all environment(endpoint) groups if using an administrator account otherwise it will\nonly return authorized environment(endpoint) groups.",
			Edition:     edition.CE,
			Handler:     endpointGroupList,
			Input:       endpointGroupListInput{},
		}, narrative("EndpointGroupList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.update", Domain: "endpoint_groups", OperationID: "EndpointGroupUpdate",
			Title:       "Update an environment(endpoint) group",
			Description: "Update an environment(endpoint) group.",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     endpointGroupUpdate,
			Input:       endpointGroupUpdateInput{},
		}, narrative("EndpointGroupUpdate")),
	}
}
