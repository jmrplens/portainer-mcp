package client

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/utils"
)

// GetEnvironments retrieves all environments from the Portainer server.
//
// Returns:
//   - A slice of Environment objects
//   - An error if the operation fails
func (c *PortainerClient) GetEnvironments() ([]models.Environment, error) {
	endpoints, err := c.cli.ListEndpoints()
	if err != nil {
		return nil, fmt.Errorf("failed to list endpoints: %w", err)
	}

	environments := make([]models.Environment, len(endpoints))
	for i, endpoint := range endpoints {
		environments[i] = models.ConvertEndpointToEnvironment(endpoint)
	}

	return environments, nil
}

// GetEnvironment retrieves a single environment by ID from the Portainer server.
//
// Parameters:
//   - id: The ID of the environment to retrieve
//
// Returns:
//   - An Environment object
//   - An error if the operation fails
func (c *PortainerClient) GetEnvironment(id int) (models.Environment, error) {
	endpoint, err := c.cli.GetEndpoint(int64(id))
	if err != nil {
		return models.Environment{}, fmt.Errorf("failed to get endpoint: %w", err)
	}

	return models.ConvertEndpointToEnvironment(endpoint), nil
}

// DeleteEnvironment deletes an environment by ID.
//
// Parameters:
//   - id: The ID of the environment to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteEnvironment(id int) error {
	err := c.cli.DeleteEndpoint(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete endpoint: %w", err)
	}
	return nil
}

// SnapshotEnvironment triggers a snapshot for a single environment.
//
// Parameters:
//   - id: The ID of the environment to snapshot
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) SnapshotEnvironment(id int) error {
	err := c.cli.SnapshotEndpoint(int64(id))
	if err != nil {
		return fmt.Errorf("failed to snapshot endpoint: %w", err)
	}
	return nil
}

// SnapshotAllEnvironments triggers a snapshot for all environments.
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) SnapshotAllEnvironments() error {
	err := c.cli.SnapshotAllEndpoints()
	if err != nil {
		return fmt.Errorf("failed to snapshot all endpoints: %w", err)
	}
	return nil
}

// UpdateEnvironmentTags updates the tags associated with an environment.
//
// Parameters:
//   - id: The ID of the environment to update
//   - tagIds: A slice of tag IDs to associate with the environment
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateEnvironmentTags(id int, tagIds []int) error {
	tags := utils.IntToInt64Slice(tagIds)
	err := c.cli.UpdateEndpoint(int64(id),
		&tags,
		nil,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to update environment tags: %w", err)
	}
	return nil
}

// UpdateEnvironmentUserAccesses updates the user access policies of an environment.
//
// Parameters:
//   - id: The ID of the environment to update
//   - userAccesses: Map of user IDs to their access level
//
// Valid access levels are:
//   - environment_administrator
//   - helpdesk_user
//   - standard_user
//   - readonly_user
//   - operator_user
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateEnvironmentUserAccesses(id int, userAccesses map[int]string) error {
	uac := utils.IntToInt64Map(userAccesses)
	err := c.cli.UpdateEndpoint(int64(id),
		nil,
		&uac,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to update environment user accesses: %w", err)
	}
	return nil
}

// UpdateEnvironmentTeamAccesses updates the team access policies of an environment.
//
// Parameters:
//   - id: The ID of the environment to update
//   - teamAccesses: Map of team IDs to their access level
//
// Valid access levels are:
//   - environment_administrator
//   - helpdesk_user
//   - standard_user
//   - readonly_user
//   - operator_user
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateEnvironmentTeamAccesses(id int, teamAccesses map[int]string) error {
	tac := utils.IntToInt64Map(teamAccesses)
	err := c.cli.UpdateEndpoint(int64(id),
		nil,
		nil,
		&tac,
	)
	if err != nil {
		return fmt.Errorf("failed to update environment team accesses: %w", err)
	}
	return nil
}
