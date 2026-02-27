package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/utils"
)

// GetStacks retrieves all stacks from the Portainer server.
// Stacks are the equivalent of Edge Stacks in Portainer.
//
// Returns:
//   - A slice of Stack objects
//   - An error if the operation fails
func (c *PortainerClient) GetStacks() ([]models.Stack, error) {
	edgeStacks, err := c.cli.ListEdgeStacks()
	if err != nil {
		return nil, fmt.Errorf("failed to list edge stacks: %w", err)
	}

	stacks := make([]models.Stack, len(edgeStacks))
	for i, es := range edgeStacks {
		stacks[i] = models.ConvertEdgeStackToStack(es)
	}

	return stacks, nil
}

// GetRegularStacks retrieves all regular (non-edge) stacks from the Portainer server.
// Regular stacks are Docker Compose or Swarm stacks deployed to specific environments.
//
// Returns:
//   - A slice of RegularStack objects
//   - An error if the operation fails
func (c *PortainerClient) GetRegularStacks() ([]models.RegularStack, error) {
	rawStacks, err := c.cli.ListRegularStacks()
	if err != nil {
		return nil, fmt.Errorf("failed to list regular stacks: %w", err)
	}

	result := make([]models.RegularStack, len(rawStacks))
	for i, s := range rawStacks {
		result[i] = models.ConvertRegularStack(s)
	}

	return result, nil
}

// GetStackFile retrieves the file content of a stack from the Portainer server.
// Stacks are the equivalent of Edge Stacks in Portainer.
//
// Parameters:
//   - id: The ID of the stack to retrieve
//
// Returns:
//   - The file content of the stack (Compose file)
//   - An error if the operation fails
func (c *PortainerClient) GetStackFile(id int) (string, error) {
	file, err := c.cli.GetEdgeStackFile(int64(id))
	if err != nil {
		return "", fmt.Errorf("failed to get edge stack file: %w", err)
	}

	return file, nil
}

// CreateStack creates a new stack on the Portainer server.
// This function specifically creates a Docker Compose stack.
// Stacks are the equivalent of Edge Stacks in Portainer.
//
// Parameters:
//   - name: The name of the stack
//   - file: The file content of the stack (Compose file)
//   - environmentGroupIds: A slice of environment group IDs to include in the stack
//
// Returns:
//   - The ID of the created stack
//   - An error if the operation fails
func (c *PortainerClient) CreateStack(name, file string, environmentGroupIds []int) (int, error) {
	id, err := c.cli.CreateEdgeStack(name, file, utils.IntToInt64Slice(environmentGroupIds))
	if err != nil {
		return 0, fmt.Errorf("failed to create edge stack: %w", err)
	}

	return int(id), nil
}

// UpdateStack updates an existing stack on the Portainer server.
// This function specifically updates a Docker Compose stack.
// Stacks are the equivalent of Edge Stacks in Portainer.
//
// Parameters:
//   - id: The ID of the stack to update
//   - file: The file content of the stack (Compose file)
//   - environmentGroupIds: A slice of environment group IDs to include in the stack
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateStack(id int, file string, environmentGroupIds []int) error {
	err := c.cli.UpdateEdgeStack(int64(id), file, utils.IntToInt64Slice(environmentGroupIds))
	if err != nil {
		return fmt.Errorf("failed to update edge stack: %w", err)
	}

	return nil
}

// InspectStack retrieves a regular (non-edge) stack by ID.
//
// Parameters:
//   - id: The ID of the stack to inspect
//
// Returns:
//   - A RegularStack object
//   - An error if the operation fails
func (c *PortainerClient) InspectStack(id int) (models.RegularStack, error) {
	raw, err := c.cli.StackInspect(int64(id))
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to inspect stack: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}

// DeleteStack deletes a regular (non-edge) stack by ID.
//
// Parameters:
//   - id: The ID of the stack to delete
//   - endpointID: The environment ID where the stack is deployed
//   - removeVolumes: Whether to remove associated volumes
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteStack(id int, endpointID int, removeVolumes bool) error {
	err := c.cli.StackDelete(int64(id), int64(endpointID), removeVolumes)
	if err != nil {
		return fmt.Errorf("failed to delete stack: %w", err)
	}

	return nil
}

// InspectStackFile retrieves the compose file content for a regular (non-edge) stack.
//
// Parameters:
//   - id: The ID of the stack
//
// Returns:
//   - The compose file content
//   - An error if the operation fails
func (c *PortainerClient) InspectStackFile(id int) (string, error) {
	content, err := c.cli.StackFileInspect(int64(id))
	if err != nil {
		return "", fmt.Errorf("failed to inspect stack file: %w", err)
	}

	return content, nil
}

// UpdateStackGit updates the git configuration of a regular (non-edge) stack.
//
// Parameters:
//   - id: The ID of the stack to update
//   - endpointID: The environment ID where the stack is deployed
//   - referenceName: The git reference name (branch/tag)
//   - prune: Whether to prune removed services
//
// Returns:
//   - The updated RegularStack
//   - An error if the operation fails
func (c *PortainerClient) UpdateStackGit(id int, endpointID int, referenceName string, prune bool) (models.RegularStack, error) {
	body := &apimodels.StacksStackGitUpdatePayload{
		RepositoryReferenceName: referenceName,
		Prune:                   prune,
	}

	raw, err := c.cli.StackUpdateGit(int64(id), int64(endpointID), body)
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to update stack git: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}

// RedeployStackGit triggers a git-based redeployment of a regular (non-edge) stack.
//
// Parameters:
//   - id: The ID of the stack to redeploy
//   - endpointID: The environment ID where the stack is deployed
//   - pullImage: Whether to pull the latest images
//   - prune: Whether to prune removed services
//
// Returns:
//   - The redeployed RegularStack
//   - An error if the operation fails
func (c *PortainerClient) RedeployStackGit(id int, endpointID int, pullImage bool, prune bool) (models.RegularStack, error) {
	body := &apimodels.StacksStackGitRedployPayload{
		PullImage: pullImage,
		Prune:     prune,
	}

	raw, err := c.cli.StackGitRedeploy(int64(id), int64(endpointID), body)
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to redeploy stack: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}

// StartStack starts a stopped regular (non-edge) stack.
//
// Parameters:
//   - id: The ID of the stack to start
//   - endpointID: The environment ID where the stack is deployed
//
// Returns:
//   - The started RegularStack
//   - An error if the operation fails
func (c *PortainerClient) StartStack(id int, endpointID int) (models.RegularStack, error) {
	raw, err := c.cli.StackStart(int64(id), int64(endpointID))
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to start stack: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}

// StopStack stops a running regular (non-edge) stack.
//
// Parameters:
//   - id: The ID of the stack to stop
//   - endpointID: The environment ID where the stack is deployed
//
// Returns:
//   - The stopped RegularStack
//   - An error if the operation fails
func (c *PortainerClient) StopStack(id int, endpointID int) (models.RegularStack, error) {
	raw, err := c.cli.StackStop(int64(id), int64(endpointID))
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to stop stack: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}

// MigrateStack migrates a regular (non-edge) stack to another environment.
//
// Parameters:
//   - id: The ID of the stack to migrate
//   - endpointID: The current environment ID where the stack is deployed
//   - targetEndpointID: The target environment ID to migrate to
//   - name: Optional new name for the migrated stack
//
// Returns:
//   - The migrated RegularStack
//   - An error if the operation fails
func (c *PortainerClient) MigrateStack(id int, endpointID int, targetEndpointID int, name string) (models.RegularStack, error) {
	targetID := int64(targetEndpointID)
	body := &apimodels.StacksStackMigratePayload{
		EndpointID: &targetID,
		Name:       name,
	}

	raw, err := c.cli.StackMigrate(int64(id), int64(endpointID), body)
	if err != nil {
		return models.RegularStack{}, fmt.Errorf("failed to migrate stack: %w", err)
	}

	return models.ConvertRegularStack(raw), nil
}
