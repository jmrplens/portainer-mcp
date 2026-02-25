package models

import (
	"time"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/utils"
)

type Stack struct {
	ID                  int    `json:"id"`
	Name                string `json:"name"`
	CreatedAt           string `json:"created_at"`
	EnvironmentGroupIds []int  `json:"group_ids"`
}

func ConvertEdgeStackToStack(rawEdgeStack *apimodels.PortainereeEdgeStack) Stack {
	createdAt := time.Unix(rawEdgeStack.CreationDate, 0).Format(time.RFC3339)

	return Stack{
		ID:                  int(rawEdgeStack.ID),
		Name:                rawEdgeStack.Name,
		CreatedAt:           createdAt,
		EnvironmentGroupIds: utils.Int64ToIntSlice(rawEdgeStack.EdgeGroups),
	}
}

// RegularStack represents a regular (non-edge) stack in Portainer
type RegularStack struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	Type           int    `json:"type"`
	Status         int    `json:"status"`
	EndpointID     int    `json:"endpoint_id"`
	EntryPoint     string `json:"entry_point,omitempty"`
	SwarmID        string `json:"swarm_id,omitempty"`
	CreatedBy      string `json:"created_by,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	FilesystemPath string `json:"filesystem_path,omitempty"`
}

// ConvertRegularStack converts a raw PortainereeStack to a RegularStack
func ConvertRegularStack(raw *apimodels.PortainereeStack) RegularStack {
	createdAt := ""
	if raw.CreationDate > 0 {
		createdAt = time.Unix(raw.CreationDate, 0).Format(time.RFC3339)
	}

	return RegularStack{
		ID:             int(raw.ID),
		Name:           raw.Name,
		Type:           int(raw.Type),
		Status:         int(raw.Status),
		EndpointID:     int(raw.EndpointID),
		EntryPoint:     raw.EntryPoint,
		SwarmID:        raw.SwarmID,
		CreatedBy:      raw.CreatedBy,
		CreatedAt:      createdAt,
		FilesystemPath: raw.FilesystemPath,
	}
}
