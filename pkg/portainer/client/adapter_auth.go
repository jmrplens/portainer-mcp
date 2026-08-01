package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/auth"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// AuthenticateUser authenticates a user using the Swagger client.
func (a *portainerAPIAdapter) AuthenticateUser(username, password string) (*apimodels.AuthAuthenticateResponse, error) {
	params := auth.NewAuthenticateUserParams()
	params.Body = &apimodels.AuthAuthenticatePayload{
		Username: &username,
		Password: &password,
	}
	resp, err := a.swagger.Auth.AuthenticateUser(params)
	if err != nil {
		return nil, fmt.Errorf("failed to authenticate user: %w", err)
	}
	return resp.Payload, nil
}

// Logout logs out the current user session.
func (a *portainerAPIAdapter) Logout() error {
	params := auth.NewLogoutParams()
	_, err := a.swagger.Auth.Logout(params, nil)
	if err != nil {
		return fmt.Errorf("failed to logout: %w", err)
	}
	return nil
}
