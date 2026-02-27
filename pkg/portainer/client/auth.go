package client

import (
	"fmt"

	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// AuthenticateUser authenticates a user against the Portainer server
// using a username and password, and returns the JWT token.
//
// Parameters:
//   - username: The username for authentication
//   - password: The password for authentication
//
// Returns:
//   - An AuthResponse containing the JWT token
//   - An error if the operation fails
func (c *PortainerClient) AuthenticateUser(username, password string) (models.AuthResponse, error) {
	resp, err := c.cli.AuthenticateUser(username, password)
	if err != nil {
		return models.AuthResponse{}, fmt.Errorf("failed to authenticate user: %w", err)
	}

	return models.AuthResponse{
		JWT: resp.Jwt,
	}, nil
}

// Logout logs out the current user session.
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) Logout() error {
	err := c.cli.Logout()
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}

	return nil
}
