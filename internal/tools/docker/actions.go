// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package docker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// containersImageStatusClear is the generated handler for operation ContainersImageStatusClear.
func containersImageStatusClear(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params containersImageStatusClearInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("ContainersImageStatusClear: parse input: %w", err)
	}
	resp, err := c.API.ContainersImageStatusClearWithResponse(ctx, params.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("ContainersImageStatusClear: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("ContainersImageStatusClear: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// dockerDashboard is the generated handler for operation DockerDashboard.
func dockerDashboard(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params dockerDashboardInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("DockerDashboard: parse input: %w", err)
	}
	resp, err := c.API.DockerDashboardWithResponse(ctx, params.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("DockerDashboard: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("DockerDashboard: %w", err)
	}
	return resp.JSON200, nil
}

// dockerImagesList is the generated handler for operation DockerImagesList.
func dockerImagesList(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params dockerImagesListInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("DockerImagesList: parse input: %w", err)
	}
	var queryParams apigen.DockerImagesListParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("DockerImagesList: parse query parameters: %w", err)
	}
	resp, err := c.API.DockerImagesListWithResponse(ctx, params.EnvironmentID, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("DockerImagesList: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("DockerImagesList: %w", err)
	}
	return resp.JSON200, nil
}

// serviceImageStatusClear is the generated handler for operation ServiceImageStatusClear.
func serviceImageStatusClear(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var params serviceImageStatusClearInput
	if err := json.Unmarshal(input, &params); err != nil {
		return nil, fmt.Errorf("ServiceImageStatusClear: parse input: %w", err)
	}
	resp, err := c.API.ServiceImageStatusClearWithResponse(ctx, params.EnvironmentID)
	if err != nil {
		return nil, fmt.Errorf("ServiceImageStatusClear: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("ServiceImageStatusClear: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// stacksImageStatusClear is the generated handler for operation StacksImageStatusClear.
func stacksImageStatusClear(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error) {
	var queryParams apigen.StacksImageStatusClearParams
	if err := json.Unmarshal(input, &queryParams); err != nil {
		return nil, fmt.Errorf("StacksImageStatusClear: parse query parameters: %w", err)
	}
	resp, err := c.API.StacksImageStatusClearWithResponse(ctx, &queryParams)
	if err != nil {
		return nil, fmt.Errorf("StacksImageStatusClear: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return nil, fmt.Errorf("StacksImageStatusClear: %w", err)
	}
	return map[string]any{"status": "ok"}, nil
}

// generatedSpecs returns every ActionSpec this generator derives for this
// domain. The domain's own Specs() (in its own, non-generated file) combines
// this with whatever action this generator refused or the domain otherwise
// declares by hand.
func generatedSpecs() []toolutil.ActionSpec {
	return []toolutil.ActionSpec{
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.containers_image_status_clear", Domain: "docker", OperationID: "ContainersImageStatusClear",
			Title:       "Clear container image status cache",
			Description: "Clear the image status cache for all containers in the environment",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     containersImageStatusClear,
			Input:       containersImageStatusClearInput{},
		}, narrative("ContainersImageStatusClear")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.dashboard", Domain: "docker", OperationID: "DockerDashboard",
			Title:       "Get counters for the dashboard",
			Description: "Get counters for the dashboard",
			Edition:     edition.CE,
			Handler:     dockerDashboard,
			Input:       dockerDashboardInput{},
		}, narrative("DockerDashboard")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.images_list", Domain: "docker", OperationID: "DockerImagesList",
			Title:       "Fetch images",
			Description: "Fetch images",
			Edition:     edition.CE,
			Handler:     dockerImagesList,
			Input:       dockerImagesListInput{},
		}, narrative("DockerImagesList")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.service_image_status_clear", Domain: "docker", OperationID: "ServiceImageStatusClear",
			Title:       "Clear service image status cache",
			Description: "Clear the image status cache for all services in the environment",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     serviceImageStatusClear,
			Input:       serviceImageStatusClearInput{},
		}, narrative("ServiceImageStatusClear")),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "docker.stacks_image_status_clear", Domain: "docker", OperationID: "StacksImageStatusClear",
			Title:       "Clear stack image status cache",
			Description: "Clear the image status cache for all stacks in the environment",
			Edition:     edition.EE,
			Mutating:    true,
			Handler:     stacksImageStatusClear,
			Input:       stacksImageStatusClearInput{},
		}, narrative("StacksImageStatusClear")),
	}
}
