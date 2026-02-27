// Package models defines the local data structures used by the MCP server.
// These models are simplified representations of Portainer API resources,
// containing only fields relevant to MCP operations. Each model type includes
// a Convert function to transform from raw API models (client-api-go SDK).
package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// AccessGroup represents a Portainer endpoint group used to manage access to multiple environments.
type AccessGroup struct {
	ID             int            `json:"id"`
	Name           string         `json:"name"`
	EnvironmentIds []int          `json:"environment_ids"`
	UserAccesses   map[int]string `json:"user_accesses"`
	TeamAccesses   map[int]string `json:"team_accesses"`
}

// ConvertEndpointGroupToAccessGroup converts a raw Portainer endpoint group and its associated
// endpoints into a simplified AccessGroup model.
func ConvertEndpointGroupToAccessGroup(rawGroup *apimodels.PortainerEndpointGroup, rawEndpoints []*apimodels.PortainereeEndpoint) AccessGroup {
	if rawGroup == nil {
		return AccessGroup{}
	}

	environmentIds := make([]int, 0)
	for _, env := range rawEndpoints {
		if env == nil {
			continue
		}
		if env.GroupID == rawGroup.ID {
			environmentIds = append(environmentIds, int(env.ID))
		}
	}

	return AccessGroup{
		ID:             int(rawGroup.ID),
		Name:           rawGroup.Name,
		EnvironmentIds: environmentIds,
		UserAccesses:   convertAccesses(rawGroup.UserAccessPolicies),
		TeamAccesses:   convertAccesses(rawGroup.TeamAccessPolicies),
	}
}
