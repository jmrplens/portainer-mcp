package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
)

// GetEdgeJobs retrieves all edge jobs from the Portainer server.
//
// Returns:
//   - A slice of EdgeJob objects
//   - An error if the operation fails
func (c *PortainerClient) GetEdgeJobs() ([]models.EdgeJob, error) {
	rawJobs, err := c.cli.ListEdgeJobs()
	if err != nil {
		return nil, fmt.Errorf("failed to list edge jobs: %w", err)
	}

	jobs := make([]models.EdgeJob, len(rawJobs))
	for i, raw := range rawJobs {
		jobs[i] = models.ConvertEdgeJobToLocal(raw)
	}

	return jobs, nil
}

// GetEdgeJob retrieves a specific edge job by ID.
//
// Parameters:
//   - id: The ID of the edge job
//
// Returns:
//   - An EdgeJob object
//   - An error if the operation fails
func (c *PortainerClient) GetEdgeJob(id int) (models.EdgeJob, error) {
	raw, err := c.cli.GetEdgeJob(int64(id))
	if err != nil {
		return models.EdgeJob{}, fmt.Errorf("failed to get edge job: %w", err)
	}

	return models.ConvertEdgeJobToLocal(raw), nil
}

// GetEdgeJobFile retrieves the file content of an edge job.
//
// Parameters:
//   - id: The ID of the edge job
//
// Returns:
//   - The file content as a string
//   - An error if the operation fails
func (c *PortainerClient) GetEdgeJobFile(id int) (string, error) {
	content, err := c.cli.GetEdgeJobFile(int64(id))
	if err != nil {
		return "", fmt.Errorf("failed to get edge job file: %w", err)
	}

	return content, nil
}

// CreateEdgeJob creates a new edge job on the Portainer server.
//
// Parameters:
//   - name: The name of the edge job
//   - cronExpression: The cron expression for scheduling
//   - fileContent: The script content
//   - endpoints: The environment IDs to target
//   - edgeGroups: The edge group IDs to target
//   - recurring: Whether the job is recurring
//
// Returns:
//   - The ID of the created edge job
//   - An error if the operation fails
func (c *PortainerClient) CreateEdgeJob(name, cronExpression, fileContent string, endpoints []int, edgeGroups []int, recurring bool) (int, error) {
	endpointIds := make([]int64, len(endpoints))
	for i, e := range endpoints {
		endpointIds[i] = int64(e)
	}

	edgeGroupIds := make([]int64, len(edgeGroups))
	for i, g := range edgeGroups {
		edgeGroupIds[i] = int64(g)
	}

	payload := &apimodels.EdgejobsEdgeJobCreateFromFileContentPayload{
		Name:           name,
		CronExpression: cronExpression,
		FileContent:    fileContent,
		Endpoints:      endpointIds,
		EdgeGroups:     edgeGroupIds,
		Recurring:      recurring,
	}

	id, err := c.cli.CreateEdgeJob(payload)
	if err != nil {
		return 0, fmt.Errorf("failed to create edge job: %w", err)
	}

	return int(id), nil
}

// DeleteEdgeJob deletes an edge job from the Portainer server.
//
// Parameters:
//   - id: The ID of the edge job to delete
//
// Returns:
//   - An error if the operation fails
func (c *PortainerClient) DeleteEdgeJob(id int) error {
	err := c.cli.DeleteEdgeJob(int64(id))
	if err != nil {
		return fmt.Errorf("failed to delete edge job: %w", err)
	}

	return nil
}

// GetEdgeUpdateSchedules retrieves all edge update schedules from the Portainer server.
//
// Returns:
//   - A slice of EdgeUpdateSchedule objects
//   - An error if the operation fails
func (c *PortainerClient) GetEdgeUpdateSchedules() ([]models.EdgeUpdateSchedule, error) {
	rawSchedules, err := c.cli.ListEdgeUpdateSchedules()
	if err != nil {
		return nil, fmt.Errorf("failed to list edge update schedules: %w", err)
	}

	schedules := make([]models.EdgeUpdateSchedule, len(rawSchedules))
	for i, raw := range rawSchedules {
		schedules[i] = models.ConvertEdgeUpdateScheduleToLocal(raw)
	}

	return schedules, nil
}
