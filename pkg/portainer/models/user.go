package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// User represents a Portainer user account.
type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// User role string constants (used in MCP tool parameters)
const (
	UserRoleAdmin     = "admin"
	UserRoleUser      = "user"
	UserRoleEdgeAdmin = "edge_admin"
	UserRoleUnknown   = "unknown"
)

// User role ID constants as used by the Portainer API
const (
	UserRoleIDAdmin    int64 = 1
	UserRoleIDUser     int64 = 2
	UserRoleIDEdgeAdmin int64 = 3
)

// ConvertToUser converts a raw Portainer user into a simplified User model.
func ConvertToUser(rawUser *apimodels.PortainereeUser) User {
	if rawUser == nil {
		return User{}
	}

	return User{
		ID:       int(rawUser.ID),
		Username: rawUser.Username,
		Role:     convertUserRole(rawUser),
	}
}

func convertUserRole(rawUser *apimodels.PortainereeUser) string {
	switch rawUser.Role {
	case UserRoleIDAdmin:
		return UserRoleAdmin
	case UserRoleIDUser:
		return UserRoleUser
	case UserRoleIDEdgeAdmin:
		return UserRoleEdgeAdmin
	default:
		return UserRoleUnknown
	}
}
