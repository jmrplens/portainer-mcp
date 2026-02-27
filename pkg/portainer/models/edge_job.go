package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// EdgeJob represents a simplified edge job for the MCP application.
type EdgeJob struct {
	ID             int    `json:"id"`
	Name           string `json:"name"`
	CronExpression string `json:"cronExpression"`
	Recurring      bool   `json:"recurring"`
	Created        int64  `json:"created,omitempty"`
	Version        int    `json:"version,omitempty"`
	EdgeGroups     []int  `json:"edgeGroups,omitempty"`
}

// ConvertEdgeJobToLocal converts a raw SDK edge job to a local EdgeJob model.
//
// Parameters:
//   - raw: The raw SDK edge job
//
// Returns:
//   - A local EdgeJob model
func ConvertEdgeJobToLocal(raw *apimodels.PortainerEdgeJob) EdgeJob {
	if raw == nil {
		return EdgeJob{}
	}

	edgeGroups := make([]int, len(raw.EdgeGroups))
	for i, g := range raw.EdgeGroups {
		edgeGroups[i] = int(g)
	}

	return EdgeJob{
		ID:             int(raw.ID),
		Name:           raw.Name,
		CronExpression: raw.CronExpression,
		Recurring:      raw.Recurring,
		Created:        raw.Created,
		Version:        int(raw.Version),
		EdgeGroups:     edgeGroups,
	}
}

// EdgeUpdateSchedule represents a simplified edge update schedule for the MCP application.
type EdgeUpdateSchedule struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Type          int    `json:"type"`
	ScheduledTime string `json:"scheduledTime,omitempty"`
	Status        int    `json:"status,omitempty"`
	StatusMessage string `json:"statusMessage,omitempty"`
	Created       int64  `json:"created,omitempty"`
	CreatedBy     int    `json:"createdBy,omitempty"`
	EdgeGroupIds  []int  `json:"edgeGroupIds,omitempty"`
}

// ConvertEdgeUpdateScheduleToLocal converts a raw SDK edge update schedule to a local model.
//
// Parameters:
//   - raw: The raw SDK edge update schedule
//
// Returns:
//   - A local EdgeUpdateSchedule model
func ConvertEdgeUpdateScheduleToLocal(raw *apimodels.EdgeupdateschedulesDecoratedUpdateSchedule) EdgeUpdateSchedule {
	if raw == nil {
		return EdgeUpdateSchedule{}
	}

	edgeGroupIds := make([]int, len(raw.EdgeGroupIds))
	for i, g := range raw.EdgeGroupIds {
		edgeGroupIds[i] = int(g)
	}

	return EdgeUpdateSchedule{
		ID:            int(raw.ID),
		Name:          raw.Name,
		Type:          int(raw.Type),
		ScheduledTime: raw.ScheduledTime,
		Status:        int(raw.Status),
		StatusMessage: raw.StatusMessage,
		Created:       raw.Created,
		CreatedBy:     int(raw.CreatedBy),
		EdgeGroupIds:  edgeGroupIds,
	}
}
