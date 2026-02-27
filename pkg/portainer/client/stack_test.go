package client

import (
	"errors"
	"testing"
	"time"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/utils"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/assert"
)

// TestGetStacks verifies get stacks behavior.
func TestGetStacks(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		mockStacks    []*apimodels.PortainereeEdgeStack
		mockError     error
		expected      []models.Stack
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockStacks: []*apimodels.PortainereeEdgeStack{
				{
					ID:           1,
					Name:         "stack1",
					CreationDate: now,
					EdgeGroups:   []int64{1, 2},
				},
				{
					ID:           2,
					Name:         "stack2",
					CreationDate: now,
					EdgeGroups:   []int64{3},
				},
			},
			expected: []models.Stack{
				{
					ID:                  1,
					Name:                "stack1",
					CreatedAt:           time.Unix(now, 0).Format(time.RFC3339),
					EnvironmentGroupIds: []int{1, 2},
				},
				{
					ID:                  2,
					Name:                "stack2",
					CreatedAt:           time.Unix(now, 0).Format(time.RFC3339),
					EnvironmentGroupIds: []int{3},
				},
			},
		},
		{
			name:       "empty stacks",
			mockStacks: []*apimodels.PortainereeEdgeStack{},
			expected:   []models.Stack{},
		},
		{
			name:          "list error",
			mockError:     errors.New("failed to list stacks"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListEdgeStacks").Return(tt.mockStacks, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			stacks, err := client.GetStacks()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, stacks)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetStackFile verifies get stack file behavior.
func TestGetStackFile(t *testing.T) {
	tests := []struct {
		name          string
		stackID       int
		mockFile      string
		mockError     error
		expected      string
		expectedError bool
	}{
		{
			name:     "successful retrieval",
			stackID:  1,
			mockFile: "version: '3'\nservices:\n  web:\n    image: nginx",
			expected: "version: '3'\nservices:\n  web:\n    image: nginx",
		},
		{
			name:          "get file error",
			stackID:       2,
			mockError:     errors.New("failed to get stack file"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetEdgeStackFile", int64(tt.stackID)).Return(tt.mockFile, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			file, err := client.GetStackFile(tt.stackID)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, file)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateStack verifies create stack behavior.
func TestCreateStack(t *testing.T) {
	tests := []struct {
		name                string
		stackName           string
		stackFile           string
		environmentGroupIds []int
		mockID              int64
		mockError           error
		expected            int
		expectedError       bool
	}{
		{
			name:                "successful creation",
			stackName:           "test-stack",
			stackFile:           "version: '3'\nservices:\n  web:\n    image: nginx",
			environmentGroupIds: []int{1, 2},
			mockID:              1,
			expected:            1,
		},
		{
			name:                "create error",
			stackName:           "test-stack",
			stackFile:           "version: '3'\nservices:\n  web:\n    image: nginx",
			environmentGroupIds: []int{1},
			mockError:           errors.New("failed to create stack"),
			expectedError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateEdgeStack", tt.stackName, tt.stackFile, utils.IntToInt64Slice(tt.environmentGroupIds)).Return(tt.mockID, tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			id, err := client.CreateStack(tt.stackName, tt.stackFile, tt.environmentGroupIds)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, id)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestUpdateStack verifies update stack behavior.
func TestUpdateStack(t *testing.T) {
	tests := []struct {
		name                string
		stackID             int
		stackFile           string
		environmentGroupIds []int
		mockError           error
		expectedError       bool
	}{
		{
			name:                "successful update",
			stackID:             1,
			stackFile:           "version: '3'\nservices:\n  web:\n    image: nginx:latest",
			environmentGroupIds: []int{1, 2},
		},
		{
			name:                "update error",
			stackID:             2,
			stackFile:           "version: '3'\nservices:\n  web:\n    image: nginx:latest",
			environmentGroupIds: []int{1},
			mockError:           errors.New("failed to update stack"),
			expectedError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("UpdateEdgeStack", int64(tt.stackID), tt.stackFile, utils.IntToInt64Slice(tt.environmentGroupIds)).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}

			err := client.UpdateStack(tt.stackID, tt.stackFile, tt.environmentGroupIds)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetRegularStacks verifies retrieval and conversion of regular stacks.
func TestGetRegularStacks(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		mockStacks    []*apimodels.PortainereeStack
		mockError     error
		expectedCount int
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockStacks: []*apimodels.PortainereeStack{
				{ID: 1, Name: "web-app", Status: 1, Type: 2, EndpointID: 1, CreationDate: now},
				{ID: 2, Name: "db-stack", Status: 1, Type: 2, EndpointID: 1, CreationDate: now},
			},
			expectedCount: 2,
		},
		{
			name:          "empty list",
			mockStacks:    []*apimodels.PortainereeStack{},
			expectedCount: 0,
		},
		{
			name:          "API error",
			mockError:     errors.New("connection refused"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("ListRegularStacks").Return(tt.mockStacks, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.GetRegularStacks()

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.Len(t, result, tt.expectedCount)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestInspectStack verifies inspection of a regular stack by ID.
func TestInspectStack(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		id            int
		mockStack     *apimodels.PortainereeStack
		mockError     error
		expectedError bool
	}{
		{
			name:      "successful inspection",
			id:        1,
			mockStack: &apimodels.PortainereeStack{ID: 1, Name: "web-app", Status: 1, Type: 2, EndpointID: 1, CreationDate: now},
		},
		{
			name:          "API error",
			id:            99,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackInspect", int64(tt.id)).Return(tt.mockStack, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.InspectStack(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestDeleteStack verifies deletion of a regular stack.
func TestDeleteStack(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		endpointID    int
		removeVolumes bool
		mockError     error
		expectedError bool
	}{
		{
			name:          "successful deletion with volumes",
			id:            1,
			endpointID:    1,
			removeVolumes: true,
		},
		{
			name:       "successful deletion without volumes",
			id:         2,
			endpointID: 1,
		},
		{
			name:          "API error",
			id:            99,
			endpointID:    1,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackDelete", int64(tt.id), int64(tt.endpointID), tt.removeVolumes).Return(tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			err := c.DeleteStack(tt.id, tt.endpointID, tt.removeVolumes)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestInspectStackFile verifies retrieval of a regular stack's compose file.
func TestInspectStackFile(t *testing.T) {
	tests := []struct {
		name          string
		id            int
		mockContent   string
		mockError     error
		expectedError bool
	}{
		{
			name:        "successful retrieval",
			id:          1,
			mockContent: "version: '3'\nservices:\n  web:\n    image: nginx",
		},
		{
			name:          "API error",
			id:            99,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackFileInspect", int64(tt.id)).Return(tt.mockContent, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.InspectStackFile(tt.id)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockContent, result)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestUpdateStackGit verifies updating the git configuration of a regular stack.
func TestUpdateStackGit(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		id            int
		endpointID    int
		referenceName string
		prune         bool
		mockResult    *apimodels.PortainereeStack
		mockError     error
		expectedError bool
	}{
		{
			name:          "successful update",
			id:            1,
			endpointID:    1,
			referenceName: "refs/heads/main",
			prune:         true,
			mockResult:    &apimodels.PortainereeStack{ID: 1, Name: "web-app", Status: 1, CreationDate: now},
		},
		{
			name:          "API error",
			id:            99,
			endpointID:    1,
			referenceName: "refs/heads/main",
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackUpdateGit", int64(tt.id), int64(tt.endpointID), mock.AnythingOfType("*models.StacksStackGitUpdatePayload")).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.UpdateStackGit(tt.id, tt.endpointID, tt.referenceName, tt.prune)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestRedeployStackGit verifies git-based redeployment of a regular stack.
func TestRedeployStackGit(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		id            int
		endpointID    int
		pullImage     bool
		prune         bool
		mockResult    *apimodels.PortainereeStack
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful redeploy",
			id:         1,
			endpointID: 1,
			pullImage:  true,
			prune:      true,
			mockResult: &apimodels.PortainereeStack{ID: 1, Name: "web-app", Status: 1, CreationDate: now},
		},
		{
			name:          "API error",
			id:            99,
			endpointID:    1,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackGitRedeploy", int64(tt.id), int64(tt.endpointID), mock.AnythingOfType("*models.StacksStackGitRedployPayload")).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.RedeployStackGit(tt.id, tt.endpointID, tt.pullImage, tt.prune)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestStartStack verifies starting a regular stack.
func TestStartStack(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		id            int
		endpointID    int
		mockResult    *apimodels.PortainereeStack
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful start",
			id:         1,
			endpointID: 1,
			mockResult: &apimodels.PortainereeStack{ID: 1, Name: "web-app", Status: 1, CreationDate: now},
		},
		{
			name:          "API error",
			id:            99,
			endpointID:    1,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackStart", int64(tt.id), int64(tt.endpointID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.StartStack(tt.id, tt.endpointID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestStopStack verifies stopping a regular stack.
func TestStopStack(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name          string
		id            int
		endpointID    int
		mockResult    *apimodels.PortainereeStack
		mockError     error
		expectedError bool
	}{
		{
			name:       "successful stop",
			id:         1,
			endpointID: 1,
			mockResult: &apimodels.PortainereeStack{ID: 1, Name: "web-app", Status: 1, CreationDate: now},
		},
		{
			name:          "API error",
			id:            99,
			endpointID:    1,
			mockError:     errors.New("stack not found"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackStop", int64(tt.id), int64(tt.endpointID)).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.StopStack(tt.id, tt.endpointID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestMigrateStack verifies migration of a regular stack to another environment.
func TestMigrateStack(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		name             string
		id               int
		endpointID       int
		targetEndpointID int
		stackName        string
		mockResult       *apimodels.PortainereeStack
		mockError        error
		expectedError    bool
	}{
		{
			name:             "successful migration with name",
			id:               1,
			endpointID:       1,
			targetEndpointID: 2,
			stackName:        "migrated-stack",
			mockResult:       &apimodels.PortainereeStack{ID: 1, Name: "migrated-stack", Status: 1, CreationDate: now},
		},
		{
			name:             "successful migration without name",
			id:               2,
			endpointID:       1,
			targetEndpointID: 3,
			mockResult:       &apimodels.PortainereeStack{ID: 2, Name: "web-app", Status: 1, CreationDate: now},
		},
		{
			name:             "API error",
			id:               99,
			endpointID:       1,
			targetEndpointID: 2,
			mockError:        errors.New("stack not found"),
			expectedError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("StackMigrate", int64(tt.id), int64(tt.endpointID), mock.AnythingOfType("*models.StacksStackMigratePayload")).Return(tt.mockResult, tt.mockError)

			c := &PortainerClient{cli: mockAPI}
			result, err := c.MigrateStack(tt.id, tt.endpointID, tt.targetEndpointID, tt.stackName)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.id, result.ID)
			}
			mockAPI.AssertExpectations(t)
		})
	}
}
