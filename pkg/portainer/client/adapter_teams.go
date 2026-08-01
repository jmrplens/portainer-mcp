package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/teams"
)

// DeleteTeam deletes a team by ID using the low-level Swagger client.
func (a *portainerAPIAdapter) DeleteTeam(id int64) error {
	params := teams.NewTeamDeleteParams().WithID(id)
	_, err := a.swagger.Teams.TeamDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete team: %w", err)
	}
	return nil
}
