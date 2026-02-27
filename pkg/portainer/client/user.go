package client

import (
	"fmt"

	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// GetUsers retrieves all users from the Portainer server.
//
// Returns:
//   - A slice of User objects containing user information
//   - An error if the operation fails
func (c *PortainerClient) GetUsers() ([]models.User, error) {
	portainerUsers, err := c.cli.ListUsers()
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	users := make([]models.User, len(portainerUsers))
	for i, user := range portainerUsers {
		users[i] = models.ConvertToUser(user)
	}

	return users, nil
}

// CreateUser creates a new user on the Portainer server.
//
// Parameters:
//   - username: The username for the new user
//   - password: The password for the new user
//   - role: The role for the new user. Must be one of: admin, user, edge_admin
//
// Returns:
//   - The ID of the created user
//   - An error if the operation fails
func (c *PortainerClient) CreateUser(username, password, role string) (int, error) {
	roleInt := convertRole(role)
	if roleInt == 0 {
		return 0, fmt.Errorf("invalid role: must be admin, user or edge_admin")
	}

	id, err := c.cli.CreateUser(username, password, roleInt)
	if err != nil {
		return 0, fmt.Errorf("failed to create user: %w", err)
	}

	return int(id), nil
}

// GetUser retrieves a single user by ID from the Portainer server.
//
// Parameters:
//   - id: The ID of the user to retrieve
//
// Returns:
//   - A User object containing user information
//   - An error if the operation fails
func (c *PortainerClient) GetUser(id int) (models.User, error) {
	portainerUser, err := c.cli.GetUser(id)
	if err != nil {
		return models.User{}, fmt.Errorf("failed to get user: %w", err)
	}

	return models.ConvertToUser(portainerUser), nil
}

// DeleteUser deletes a user from the Portainer server.
//
// Parameters:
//   - id: The ID of the user to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteUser(id int) error {
	err := c.cli.DeleteUser(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// UpdateUserRole updates the role of a user.
//
// Parameters:
//   - id: The ID of the user to update
//   - role: The new role for the user. Must be one of: admin, user, edge_admin
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) UpdateUserRole(id int, role string) error {
	roleInt := convertRole(role)
	if roleInt == 0 {
		return fmt.Errorf("invalid role: must be admin, user or edge_admin")
	}

	return c.cli.UpdateUserRole(id, roleInt)
}

// convertRole convert role.
func convertRole(role string) int64 {
	switch role {
	case models.UserRoleAdmin:
		return models.UserRoleIDAdmin
	case models.UserRoleUser:
		return models.UserRoleIDUser
	case models.UserRoleEdgeAdmin:
		return models.UserRoleIDEdgeAdmin
	default:
		return 0
	}
}
