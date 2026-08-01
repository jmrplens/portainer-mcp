package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// Role represents a Portainer role
type Role struct {
	ID             int             `json:"id"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Priority       int             `json:"priority"`
	Authorizations map[string]bool `json:"authorizations"`
}

// ConvertToRole converts a raw PortainereeRole to a local Role
func ConvertToRole(raw *apimodels.PortainereeRole) Role {
	if raw == nil {
		return Role{}
	}

	role := Role{}
	if raw.Authorizations != nil {
		role.Authorizations = map[string]bool(raw.Authorizations)
	}

	if raw.ID != nil {
		role.ID = int(*raw.ID)
	}
	if raw.Name != nil {
		role.Name = *raw.Name
	}
	if raw.Description != nil {
		role.Description = *raw.Description
	}
	if raw.Priority != nil {
		role.Priority = int(*raw.Priority)
	}

	return role
}
