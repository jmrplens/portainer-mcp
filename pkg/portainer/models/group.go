package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/utils"
)

// Group represents a Portainer edge group used to organize edge environments.
type Group struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	EnvironmentIds []int  `json:"environment_ids"`
	TagIds         []int  `json:"tag_ids"`
}

// ConvertEdgeGroupToGroup converts a raw Portainer edge group into a simplified Group model.
func ConvertEdgeGroupToGroup(rawEdgeGroup *apimodels.EdgegroupsDecoratedEdgeGroup) Group {
	if rawEdgeGroup == nil {
		return Group{}
	}

	return Group{
		ID:             int(rawEdgeGroup.ID),
		Name:           rawEdgeGroup.Name,
		EnvironmentIds: utils.Int64ToIntSlice(rawEdgeGroup.Endpoints),
		TagIds:         utils.Int64ToIntSlice(rawEdgeGroup.TagIds),
	}
}
