package client

import (
	"errors"
	"fmt"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestGetEdgeJobs verifies get edge jobs behavior.
func TestGetEdgeJobs(t *testing.T) {
	tests := []struct {
		name          string
		mockJobs      []*apimodels.PortainerEdgeJob
		mockError     error
		expectedJobs  []models.EdgeJob
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockJobs: []*apimodels.PortainerEdgeJob{
				{ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true, EdgeGroups: []int64{}},
				{ID: 2, Name: "Job 2", CronExpression: "0 0 * * *", Recurring: false, EdgeGroups: []int64{1}},
			},
			mockError: nil,
			expectedJobs: []models.EdgeJob{
				{ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true, EdgeGroups: []int{}},
				{ID: 2, Name: "Job 2", CronExpression: "0 0 * * *", Recurring: false, EdgeGroups: []int{1}},
			},
			expectedError: false,
		},
		{
			name:          "empty list",
			mockJobs:      []*apimodels.PortainerEdgeJob{},
			mockError:     nil,
			expectedJobs:  []models.EdgeJob{},
			expectedError: false,
		},
		{
			name:          "api error",
			mockJobs:      nil,
			mockError:     fmt.Errorf("api error"),
			expectedJobs:  nil,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListEdgeJobs").Return(tt.mockJobs, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			jobs, err := client.GetEdgeJobs()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedJobs, jobs)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetEdgeJob verifies get edge job behavior.
func TestGetEdgeJob(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockJob       *apimodels.PortainerEdgeJob
		mockError     error
		expectedJob   models.EdgeJob
		expectedError bool
	}{
		{
			name:      "successful retrieval",
			id:        1,
			mockJob:   &apimodels.PortainerEdgeJob{ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true, EdgeGroups: []int64{1, 2}},
			mockError: nil,
			expectedJob: models.EdgeJob{
				ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true, EdgeGroups: []int{1, 2},
			},
			expectedError: false,
		},
		{
			name:          "api error",
			id:            99,
			mockJob:       nil,
			mockError:     fmt.Errorf("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetEdgeJob", int64(tt.id)).Return(tt.mockJob, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			job, err := client.GetEdgeJob(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedJob, job)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetEdgeJobFile verifies get edge job file behavior.
func TestGetEdgeJobFile(t *testing.T) {
	tests := []struct {
		name            string
		id              int
		mockContent     string
		mockError       error
		expectedContent string
		expectedError   bool
	}{
		{
			name:            "successful retrieval",
			id:              1,
			mockContent:     "#!/bin/bash\necho hello",
			mockError:       nil,
			expectedContent: "#!/bin/bash\necho hello",
			expectedError:   false,
		},
		{
			name:          "api error",
			id:            99,
			mockContent:   "",
			mockError:     fmt.Errorf("not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetEdgeJobFile", int64(tt.id)).Return(tt.mockContent, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			content, err := client.GetEdgeJobFile(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedContent, content)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateEdgeJob verifies create edge job behavior.
func TestCreateEdgeJob(t *testing.T) {
	tests := []struct {
		name          string
		jobName       string
		cron          string
		fileContent   string
		endpoints     []int
		edgeGroups    []int
		recurring     bool
		mockID        int
		mockError     error
		expectedID    int
		expectedError bool
	}{
		{
			name:        "successful creation",
			jobName:     "My Job",
			cron:        "* * * * *",
			fileContent: "#!/bin/bash\necho hello",
			endpoints:   []int{1, 2},
			edgeGroups:  []int{3},
			recurring:   true,
			mockID:      42,
			mockError:   nil,
			expectedID:  42,
		},
		{
			name:          "api error",
			jobName:       "Fail",
			cron:          "0 0 * * *",
			fileContent:   "content",
			endpoints:     []int{},
			edgeGroups:    []int{},
			recurring:     false,
			mockError:     fmt.Errorf("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateEdgeJob", mock.AnythingOfType("*models.EdgejobsEdgeJobCreateFromFileContentPayload")).Return(tt.mockID, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			id, err := client.CreateEdgeJob(tt.jobName, tt.cron, tt.fileContent, tt.endpoints, tt.edgeGroups, tt.recurring)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedID, id)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteEdgeJob verifies delete edge job behavior.
func TestDeleteEdgeJob(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockError     error
		expectedError bool
	}{
		{
			name: "successful deletion",
			id:   1,
		},
		{
			name:          "delete error",
			id:            1,
			mockError:     errors.New("failed to delete edge job"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("DeleteEdgeJob", int64(tt.id)).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			err := client.DeleteEdgeJob(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetEdgeUpdateSchedules verifies get edge update schedules behavior.
func TestGetEdgeUpdateSchedules(t *testing.T) {
	tests := []struct {
		name              string
		mockSchedules     []*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule
		mockError         error
		expectedSchedules []models.EdgeUpdateSchedule
		expectedError     bool
	}{
		{
			name: "successful retrieval",
			mockSchedules: []*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule{
				{ID: 1, Name: "Schedule 1", Type: 1, ScheduledTime: "2024-01-01T00:00:00Z", EdgeGroupIds: []int64{}},
				{ID: 2, Name: "Schedule 2", Type: 2, ScheduledTime: "2024-02-01T00:00:00Z", EdgeGroupIds: []int64{1}},
			},
			mockError: nil,
			expectedSchedules: []models.EdgeUpdateSchedule{
				{ID: 1, Name: "Schedule 1", Type: 1, ScheduledTime: "2024-01-01T00:00:00Z", EdgeGroupIds: []int{}},
				{ID: 2, Name: "Schedule 2", Type: 2, ScheduledTime: "2024-02-01T00:00:00Z", EdgeGroupIds: []int{1}},
			},
			expectedError: false,
		},
		{
			name:              "empty list",
			mockSchedules:     []*apimodels.EdgeupdateschedulesDecoratedUpdateSchedule{},
			mockError:         nil,
			expectedSchedules: []models.EdgeUpdateSchedule{},
			expectedError:     false,
		},
		{
			name:              "api error",
			mockSchedules:     nil,
			mockError:         fmt.Errorf("api error"),
			expectedSchedules: nil,
			expectedError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListEdgeUpdateSchedules").Return(tt.mockSchedules, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			schedules, err := client.GetEdgeUpdateSchedules()

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSchedules, schedules)
			}

			mockAPI.AssertExpectations(t)
		})
	}
}
