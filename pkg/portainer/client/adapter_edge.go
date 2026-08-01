package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/edge_jobs"
	"github.com/portainer/client-api-go/v2/pkg/client/edge_update_schedules"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// ListEdgeJobs lists all edge jobs.
func (a *portainerAPIAdapter) ListEdgeJobs() ([]*apimodels.PortainerEdgeJob, error) {
	params := edge_jobs.NewEdgeJobListParams()
	resp, err := a.swagger.EdgeJobs.EdgeJobList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge jobs: %w", err)
	}
	return resp.Payload, nil
}

// GetEdgeJob retrieves an edge job by ID.
func (a *portainerAPIAdapter) GetEdgeJob(id int64) (*apimodels.PortainerEdgeJob, error) {
	params := edge_jobs.NewEdgeJobInspectParams().WithID(id)
	resp, err := a.swagger.EdgeJobs.EdgeJobInspect(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get edge job: %w", err)
	}
	return resp.Payload, nil
}

// GetEdgeJobFile retrieves the file content of an edge job.
func (a *portainerAPIAdapter) GetEdgeJobFile(id int64) (string, error) {
	params := edge_jobs.NewEdgeJobFileParams().WithID(id)
	resp, err := a.swagger.EdgeJobs.EdgeJobFile(params, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get edge job file: %w", err)
	}
	return resp.Payload.FileContent, nil
}

// CreateEdgeJob creates a new edge job from file content.
func (a *portainerAPIAdapter) CreateEdgeJob(payload *apimodels.EdgejobsEdgeJobCreateFromFileContentPayload) (int64, error) {
	params := edge_jobs.NewEdgeJobCreateStringParams().WithBody(payload)
	resp, err := a.swagger.EdgeJobs.EdgeJobCreateString(params, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to create edge job: %w", err)
	}
	return resp.Payload.ID, nil
}

// DeleteEdgeJob deletes an edge job by ID.
func (a *portainerAPIAdapter) DeleteEdgeJob(id int64) error {
	params := edge_jobs.NewEdgeJobDeleteParams().WithID(id)
	_, err := a.swagger.EdgeJobs.EdgeJobDelete(params, nil)
	if err != nil {
		return fmt.Errorf("failed to delete edge job: %w", err)
	}
	return nil
}

// ListEdgeUpdateSchedules lists all edge update schedules.
func (a *portainerAPIAdapter) ListEdgeUpdateSchedules() ([]*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule, error) {
	params := edge_update_schedules.NewEdgeUpdateScheduleListParams()
	resp, err := a.swagger.EdgeUpdateSchedules.EdgeUpdateScheduleList(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to list edge update schedules: %w", err)
	}
	return resp.Payload, nil
}
