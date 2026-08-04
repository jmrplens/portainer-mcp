// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package registries

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// ecrDeleteRepository is the generated handler for operation EcrDeleteRepository.
func ecrDeleteRepository(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params ecrDeleteRepositoryInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("EcrDeleteRepository: parse input: %w", err)
	}
	resp, err := c.API.EcrDeleteRepositoryWithResponse(ctx, params.ID, params.RepositoryName)
	if err != nil {
		return nil, fmt.Errorf("EcrDeleteRepository: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("EcrDeleteRepository: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// registryCreate is the generated handler for operation RegistryCreate.
func registryCreate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.RegistryCreateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("RegistryCreate: parse request body: %w", err)
	}
	resp, err := c.API.RegistryCreateWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("RegistryCreate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryCreate: %w", err)
	}
	return redactRegistryCreate(resp.JSON200), nil
}

// registryDelete is the generated handler for operation RegistryDelete.
func registryDelete(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryDeleteInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("RegistryDelete: parse input: %w", err)
	}
	resp, err := c.API.RegistryDeleteWithResponse(ctx, params.ID)
	if err != nil {
		return nil, fmt.Errorf("RegistryDelete: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryDelete: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// registryInspect is the generated handler for operation RegistryInspect.
func registryInspect(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryInspectInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("RegistryInspect: parse input: %w", err)
	}
	var queryParams apigen.RegistryInspectParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("RegistryInspect: parse query parameters: %w", err)
	}
	resp, err := c.API.RegistryInspectWithResponse(ctx, params.ID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("RegistryInspect: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryInspect: %w", err)
	}
	return redactRegistryInspect(resp.JSON200), nil
}

// registryList is the generated handler for operation RegistryList.
func registryList(ctx context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
	resp, err := c.API.RegistryListWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("RegistryList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryList: %w", err)
	}
	return redactRegistryList(resp.JSON200), nil
}

// registryPing is the generated handler for operation RegistryPing.
func registryPing(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var body apigen.RegistryPingJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("RegistryPing: parse request body: %w", err)
	}
	resp, err := c.API.RegistryPingWithResponse(ctx, body)
	if err != nil {
		return nil, fmt.Errorf("RegistryPing: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryPing: %w", err)
	}
	return resp.JSON200, nil
}

// registryUpdate is the generated handler for operation RegistryUpdate.
func registryUpdate(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params registryUpdateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("RegistryUpdate: parse input: %w", err)
	}
	var body apigen.RegistryUpdateJSONRequestBody
	if err := json.Unmarshal(input, &body); err != nil {
		return nil, fmt.Errorf("RegistryUpdate: parse request body: %w", err)
	}
	resp, err := c.API.RegistryUpdateWithResponse(ctx, params.ID, body)
	if err != nil {
		return nil, fmt.Errorf("RegistryUpdate: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("RegistryUpdate: %w", err)
	}
	return redactRegistryUpdate(resp.JSON200), nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.ecr_delete_repository", Domain: "registries", OperationID: "EcrDeleteRepository",
			Title:       "Delete ECR repository",
			Description: "Delete ECR repository.",
			Edition:     edition.EE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     ecrDeleteRepository,
			Input:       ecrDeleteRepositoryInput{},
		}, narrative("EcrDeleteRepository")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.create", Domain: "registries", OperationID: "RegistryCreate",
			Title:       "Create a new registry",
			Description: "Create a new registry.",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryCreate,
			Input:       registryCreateInput{},
		}, narrative("RegistryCreate")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.delete", Domain: "registries", OperationID: "RegistryDelete",
			Title:       "Remove a registry",
			Description: "Remove a registry",
			Edition:     edition.CE,
			Mutating:    true,
			Destructive: true,
			Idempotent:  true,
			Handler:     registryDelete,
			Input:       registryDeleteInput{},
		}, narrative("RegistryDelete")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.inspect", Domain: "registries", OperationID: "RegistryInspect",
			Title:       "Inspect a registry",
			Description: "Retrieve details about a registry. If endpointId is provided, applies policy overrides for that environment.",
			Edition:     edition.CE,
			Handler:     registryInspect,
			Input:       registryInspectInput{},
		}, narrative("RegistryInspect")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.list", Domain: "registries", OperationID: "RegistryList",
			Title:       "List Registries",
			Description: "List all registries.\nAdministrators and edge-admins receive the full registry record (minus passwords).\nAll other authenticated users receive a scrubbed record containing only ID, Name, and Type.",
			Edition:     edition.CE,
			Handler:     registryList,
		}, narrative("RegistryList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.ping", Domain: "registries", OperationID: "RegistryPing",
			Title:       "Test registry connection",
			Description: "Test connection to a registry with provided credentials",
			Edition:     edition.CE,
			Mutating:    true,
			Handler:     registryPing,
			Input:       registryPingInput{},
		}, narrative("RegistryPing")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "registries.update", Domain: "registries", OperationID: "RegistryUpdate",
			Title:       "Update a registry",
			Description: "Update a registry",
			Edition:     edition.CE,
			Mutating:    true,
			Idempotent:  true,
			Handler:     registryUpdate,
			Input:       registryUpdateInput{},
		}, narrative("RegistryUpdate")),
	}
}
