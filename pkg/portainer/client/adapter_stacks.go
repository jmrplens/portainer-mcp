package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/stacks"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListRegularStacks retrieves all regular (non-edge) stacks.
func (a *portainerAPIAdapter) ListRegularStacks() ([]*apimodels.PortainereeStack, error) {
	params := stacks.NewStackListParams()
	resp, respNoContent, err := a.swagger.Stacks.StackList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list regular stacks: %w", err)
	}
	if respNoContent != nil {
		return []*apimodels.PortainereeStack{}, nil
	}
	return resp.Payload, nil
}

// StackInspect retrieves details of a specific stack by ID.
func (a *portainerAPIAdapter) StackInspect(id int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackInspectParams().WithID(id)
	resp, err := a.swagger.Stacks.StackInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect stack: %w", err)
	}
	return resp.Payload, nil
}

// StackDelete removes a stack by ID.
func (a *portainerAPIAdapter) StackDelete(id int64, endpointID int64, removeVolumes bool) error {
	params := stacks.NewStackDeleteParams().WithID(id).WithEndpointID(endpointID).WithRemoveVolumes(&removeVolumes)
	_, err := a.swagger.Stacks.StackDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}
	return nil
}

// StackFileInspect retrieves the compose file content for a stack.
func (a *portainerAPIAdapter) StackFileInspect(id int64) (string, error) {
	params := stacks.NewStackFileInspectParams().WithID(id)
	resp, err := a.swagger.Stacks.StackFileInspect(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to inspect stack file: %w", err)
	}
	return resp.Payload.StackFileContent, nil
}

// StackUpdateGit updates the git configuration of a stack.
func (a *portainerAPIAdapter) StackUpdateGit(id int64, endpointID int64, body *apimodels.StacksStackGitUpdatePayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackUpdateGitParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackUpdateGit(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to update stack git: %w", err)
	}
	return resp.Payload, nil
}

// StackGitRedeploy triggers a git-based redeployment of a stack.
func (a *portainerAPIAdapter) StackGitRedeploy(id int64, endpointID int64, body *apimodels.StacksStackGitRedployPayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackGitRedeployParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackGitRedeploy(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to redeploy stack: %w", err)
	}
	return resp.Payload, nil
}

// StackStart starts a stopped stack.
func (a *portainerAPIAdapter) StackStart(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackStartParams().WithID(id).WithEndpointID(endpointID)
	resp, err := a.swagger.Stacks.StackStart(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to start stack: %w", err)
	}
	return resp.Payload, nil
}

// StackStop stops a running stack.
func (a *portainerAPIAdapter) StackStop(id int64, endpointID int64) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackStopParams().WithID(id).WithEndpointID(endpointID)
	resp, err := a.swagger.Stacks.StackStop(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to stop stack: %w", err)
	}
	return resp.Payload, nil
}

// StackMigrate migrates a stack to another environment.
func (a *portainerAPIAdapter) StackMigrate(id int64, endpointID int64, body *apimodels.StacksStackMigratePayload) (*apimodels.PortainereeStack, error) {
	params := stacks.NewStackMigrateParams().WithID(id).WithEndpointID(&endpointID).WithBody(body)
	resp, err := a.swagger.Stacks.StackMigrate(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to migrate stack: %w", err)
	}
	return resp.Payload, nil
}
