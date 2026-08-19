// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package resource_controls

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// resourceControlCreate is the generated handler for operation ResourceControlCreate.
func resourceControlCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.ResourceControlCreateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("ResourceControlCreate: parse request body: %w", err)
	}
	resp, err := c.API.ResourceControlCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("ResourceControlCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("ResourceControlCreate: %w", err)
	}
	return resp.JSON200, nil
}

// resourceControlDelete is the generated handler for operation ResourceControlDelete.
func resourceControlDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params resourceControlDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("ResourceControlDelete: parse input: %w", err)
	}
	resp, err := c.API.ResourceControlDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("ResourceControlDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("ResourceControlDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// resourceControlUpdate is the generated handler for operation ResourceControlUpdate.
func resourceControlUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params resourceControlUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("ResourceControlUpdate: parse input: %w", err)
	}
	var body apigen.ResourceControlUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("ResourceControlUpdate: parse request body: %w", err)
	}
	resp, err := c.API.ResourceControlUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("ResourceControlUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("ResourceControlUpdate: %w", err)
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
			Name: "resource_controls.create", Domain: "resource_controls", OperationID: "ResourceControlCreate",
			Title:       "Create a new resource control",
			Description: "Create a new resource control to restrict access to a Docker resource.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     resourceControlCreate,
			Input:       resourceControlCreateInput{},
		}, narrative("ResourceControlCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "resource_controls.delete", Domain: "resource_controls", OperationID: "ResourceControlDelete",
			Title:       "Remove a resource control",
			Description: "Remove a resource control.",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     resourceControlDelete,
			Input:       resourceControlDeleteInput{},
		}, narrative("ResourceControlDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "resource_controls.update", Domain: "resource_controls", OperationID: "ResourceControlUpdate",
			Title:       "Update a resource control",
			Description: "Update a resource control",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     resourceControlUpdate,
			Input:       resourceControlUpdateInput{},
		}, narrative("ResourceControlUpdate")),
	}
}
