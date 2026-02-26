package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// SystemStatus represents the Portainer server version and instance identifier.
type SystemStatus struct {
	Version    string `json:"version"`
	InstanceID string `json:"instanceID"`
}

// ConvertToSystemStatus converts raw Portainer system status into a simplified SystemStatus model.
func ConvertToSystemStatus(rawStatus *apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus) SystemStatus {
	if rawStatus == nil {
		return SystemStatus{}
	}

	return SystemStatus{
		Version:    rawStatus.Version,
		InstanceID: rawStatus.InstanceID,
	}
}
