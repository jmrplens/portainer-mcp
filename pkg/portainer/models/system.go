package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

type SystemStatus struct {
	Version    string `json:"version"`
	InstanceID string `json:"instanceID"`
}

func ConvertToSystemStatus(rawStatus *apimodels.GithubComPortainerPortainerEeAPIHTTPHandlerSystemStatus) SystemStatus {
	return SystemStatus{
		Version:    rawStatus.Version,
		InstanceID: rawStatus.InstanceID,
	}
}
