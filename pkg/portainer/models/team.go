package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// Team represents a Portainer team with its member user IDs.
type Team struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	MemberIDs []int  `json:"members"`
}

// ConvertToTeam converts a raw Portainer team and its memberships into a simplified Team model.
func ConvertToTeam(rawTeam *apimodels.PortainerTeam, rawMemberships []*apimodels.PortainerTeamMembership) Team {
	if rawTeam == nil {
		return Team{}
	}

	memberIDs := make([]int, 0)
	for _, member := range rawMemberships {
		if member == nil {
			continue
		}
		if member.TeamID == rawTeam.ID {
			memberIDs = append(memberIDs, int(member.UserID))
		}
	}

	return Team{
		ID:        int(rawTeam.ID),
		Name:      rawTeam.Name,
		MemberIDs: memberIDs,
	}
}
