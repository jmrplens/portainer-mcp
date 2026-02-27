package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestHandleListEdgeJobs verifies the HandleListEdgeJobs MCP tool handler.
func TestHandleListEdgeJobs(t *testing.T) {
	tests := []struct {
		name        string
		mockJobs    []models.EdgeJob
		mockError   error
		expectError bool
	}{
		{
			name: "successful retrieval",
			mockJobs: []models.EdgeJob{
				{ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true},
				{ID: 2, Name: "Job 2", CronExpression: "0 0 * * *", Recurring: false},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			mockJobs:    nil,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetEdgeJobs").Return(tt.mockJobs, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}

			handler := server.HandleListEdgeJobs()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var jobs []models.EdgeJob
				err = json.Unmarshal([]byte(textContent.Text), &jobs)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockJobs, jobs)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetEdgeJob verifies the HandleGetEdgeJob MCP tool handler.
func TestHandleGetEdgeJob(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockJob     models.EdgeJob
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:    "successful retrieval",
			inputID: 1,
			mockJob: models.EdgeJob{
				ID: 1, Name: "Job 1", CronExpression: "* * * * *", Recurring: true,
			},
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("not found"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.mockError != nil || !tt.expectError {
				mockClient.On("GetEdgeJob", tt.inputID).Return(tt.mockJob, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetEdgeJob()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var job models.EdgeJob
				err = json.Unmarshal([]byte(textContent.Text), &job)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockJob, job)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetEdgeJobFile verifies the HandleGetEdgeJobFile MCP tool handler.
func TestHandleGetEdgeJobFile(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockContent string
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful retrieval",
			inputID:     1,
			mockContent: "#!/bin/bash\necho hello",
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("not found"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.mockError != nil || tt.inputID != 0 && !tt.expectError {
				mockClient.On("GetEdgeJobFile", tt.inputID).Return(tt.mockContent, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetEdgeJobFile()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Equal(t, tt.mockContent, textContent.Text)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateEdgeJob verifies the HandleCreateEdgeJob MCP tool handler.
func TestHandleCreateEdgeJob(t *testing.T) {
	tests := []struct {
		name        string
		mockID      int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful creation",
			mockID:      42,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":           "My Job",
					"cronExpression": "* * * * *",
					"fileContent":    "#!/bin/bash\necho hello",
					"recurring":      true,
					"endpoints":      []any{float64(1), float64(2)},
					"edgeGroups":     []any{float64(3)},
				}
			},
		},
		{
			name:        "api error",
			mockID:      0,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":           "Fail",
					"cronExpression": "0 0 * * *",
					"fileContent":    "content",
				}
			},
		},
		{
			name:        "missing name parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"cronExpression": "* * * * *",
					"fileContent":    "content",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if tt.name == "successful creation" {
				mockClient.On("CreateEdgeJob", "My Job", "* * * * *", "#!/bin/bash\necho hello", []int{1, 2}, []int{3}, true).Return(tt.mockID, tt.mockError)
			} else if tt.name == "api error" {
				mockClient.On("CreateEdgeJob", "Fail", "0 0 * * *", "content", []int{}, []int{}, false).Return(tt.mockID, tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleCreateEdgeJob()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, fmt.Sprintf("ID: %d", tt.mockID))
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleDeleteEdgeJob verifies the HandleDeleteEdgeJob MCP tool handler.
func TestHandleDeleteEdgeJob(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful deletion",
			inputID:     1,
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{"id": float64(1)}
			},
		},
		{
			name:        "missing id parameter",
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("DeleteEdgeJob", tt.inputID).Return(tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleDeleteEdgeJob()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				if tt.mockError != nil {
					textContent, ok := result.Content[0].(mcp.TextContent)
					assert.True(t, ok)
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleListEdgeUpdateSchedules verifies the HandleListEdgeUpdateSchedules MCP tool handler.
func TestHandleListEdgeUpdateSchedules(t *testing.T) {
	tests := []struct {
		name          string
		mockSchedules []models.EdgeUpdateSchedule
		mockError     error
		expectError   bool
	}{
		{
			name: "successful retrieval",
			mockSchedules: []models.EdgeUpdateSchedule{
				{ID: 1, Name: "Schedule 1", Type: 1, ScheduledTime: "2024-01-01T00:00:00Z"},
				{ID: 2, Name: "Schedule 2", Type: 2, ScheduledTime: "2024-02-01T00:00:00Z"},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:          "api error",
			mockSchedules: nil,
			mockError:     fmt.Errorf("api error"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetEdgeUpdateSchedules").Return(tt.mockSchedules, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}

			handler := server.HandleListEdgeUpdateSchedules()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var schedules []models.EdgeUpdateSchedule
				err = json.Unmarshal([]byte(textContent.Text), &schedules)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockSchedules, schedules)
			}

			mockClient.AssertExpectations(t)
		})
	}
}
