package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/users"
)

// DeleteUser deletes a user by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteUser(id int64) error {
	params := users.NewUserDeleteParams().WithID(id)
	_, err := a.swagger.Users.UserDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}
	return nil
}
