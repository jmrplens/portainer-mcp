package models

import (
	"strconv"

	"github.com/rs/zerolog/log"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

func convertAccesses[T apimodels.PortainerUserAccessPolicies | apimodels.PortainerTeamAccessPolicies](rawPolicies T) map[int]string {
	accesses := make(map[int]string)
	for idStr, role := range rawPolicies {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			log.Warn().Str("id", idStr).Err(err).Msg("Skipping access policy with invalid ID")
			continue
		}
		accesses[id] = convertAccessPolicyRole(&role)
	}
	return accesses
}

func convertAccessPolicyRole(rawPolicy *apimodels.PortainerAccessPolicy) string {
	if rawPolicy == nil {
		return "unknown"
	}

	switch rawPolicy.RoleID {
	case 1:
		return "environment_administrator"
	case 2:
		return "helpdesk_user"
	case 3:
		return "standard_user"
	case 4:
		return "readonly_user"
	case 5:
		return "operator_user"
	default:
		return "unknown"
	}
}
