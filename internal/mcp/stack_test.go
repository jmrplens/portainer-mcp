package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/jmrplens/portainer-mcp-enhanced/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestHandleGetStacks verifies the HandleGetStacks MCP tool handler.
func TestHandleGetStacks(t *testing.T) {
	tests := []struct {
		name        string
		mockStacks  []models.Stack
		mockError   error
		expectError bool
	}{
		{
			name: "successful stacks retrieval",
			mockStacks: []models.Stack{
				{ID: 1, Name: "stack1"},
				{ID: 2, Name: "stack2"},
			},
			mockError:   nil,
			expectError: false,
		},
		{
			name:        "api error",
			mockStacks:  nil,
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetStacks").Return(tt.mockStacks, tt.mockError)

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			handler := server.HandleGetStacks()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
				}
			} else {
				assert.NoError(t, err)
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var stacks []models.Stack
				err = json.Unmarshal([]byte(textContent.Text), &stacks)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockStacks, stacks)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetStackFile verifies the HandleGetStackFile MCP tool handler.
func TestHandleGetStackFile(t *testing.T) {
	tests := []struct {
		name        string
		inputID     int
		mockContent string
		mockError   error
		expectError bool
		setupParams func(request *mcp.CallToolRequest)
	}{
		{
			name:        "successful file retrieval",
			inputID:     1,
			mockContent: "version: '3'\nservices:\n  web:\n    image: nginx",
			mockError:   nil,
			expectError: false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id": float64(1),
				}
			},
		},
		{
			name:        "api error",
			inputID:     1,
			mockContent: "",
			mockError:   fmt.Errorf("api error"),
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id": float64(1),
				}
			},
		},
		{
			name:        "missing id parameter",
			inputID:     0,
			mockContent: "",
			mockError:   nil,
			expectError: true,
			setupParams: func(request *mcp.CallToolRequest) {
				// No need to set any parameters as the request will be invalid
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("GetStackFile", tt.inputID).Return(tt.mockContent, tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleGetStackFile()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
				}
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

// TestHandleCreateStack verifies the HandleCreateStack MCP tool handler.
func TestHandleCreateStack(t *testing.T) {
	tests := []struct {
		name             string
		inputName        string
		inputFile        string
		inputEnvGroupIDs []int
		mockID           int
		mockError        error
		expectError      bool
		setupParams      func(request *mcp.CallToolRequest)
	}{
		{
			name:             "successful stack creation",
			inputName:        "test-stack",
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockID:           1,
			mockError:        nil,
			expectError:      false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":                "test-stack",
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "api error",
			inputName:        "test-stack",
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockID:           0,
			mockError:        fmt.Errorf("api error"),
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":                "test-stack",
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing name parameter",
			inputName:        "",
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockID:           0,
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing file parameter",
			inputName:        "test-stack",
			inputFile:        "",
			inputEnvGroupIDs: []int{1, 2},
			mockID:           0,
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name":                "test-stack",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing environmentGroupIds parameter",
			inputName:        "test-stack",
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: nil,
			mockID:           0,
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"name": "test-stack",
					"file": "version: '3'\nservices:\n  web:\n    image: nginx",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("CreateStack", tt.inputName, tt.inputFile, tt.inputEnvGroupIDs).Return(tt.mockID, tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleCreateStack()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
				}
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

// TestHandleUpdateStack verifies the HandleUpdateStack MCP tool handler.
func TestHandleUpdateStack(t *testing.T) {
	tests := []struct {
		name             string
		inputID          int
		inputFile        string
		inputEnvGroupIDs []int
		mockError        error
		expectError      bool
		setupParams      func(request *mcp.CallToolRequest)
	}{
		{
			name:             "successful stack update",
			inputID:          1,
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockError:        nil,
			expectError:      false,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":                  float64(1),
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "api error",
			inputID:          1,
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockError:        fmt.Errorf("api error"),
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":                  float64(1),
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing id parameter",
			inputID:          0,
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: []int{1, 2},
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"file":                "version: '3'\nservices:\n  web:\n    image: nginx",
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing file parameter",
			inputID:          1,
			inputFile:        "",
			inputEnvGroupIDs: []int{1, 2},
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":                  float64(1),
					"environmentGroupIds": []any{float64(1), float64(2)},
				}
			},
		},
		{
			name:             "missing environmentGroupIds parameter",
			inputID:          1,
			inputFile:        "version: '3'\nservices:\n  web:\n    image: nginx",
			inputEnvGroupIDs: nil,
			mockError:        nil,
			expectError:      true,
			setupParams: func(request *mcp.CallToolRequest) {
				request.Params.Arguments = map[string]any{
					"id":   float64(1),
					"file": "version: '3'\nservices:\n  web:\n    image: nginx",
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			if !tt.expectError || tt.mockError != nil {
				mockClient.On("UpdateStack", tt.inputID, tt.inputFile, tt.inputEnvGroupIDs).Return(tt.mockError)
			}

			server := &PortainerMCPServer{
				cli: mockClient,
			}

			request := CreateMCPRequest(map[string]any{})
			tt.setupParams(&request)

			handler := server.HandleUpdateStack()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.True(t, result.IsError, "result.IsError should be true for expected errors")
				assert.Len(t, result.Content, 1)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok, "Result content should be mcp.TextContent for errors")
				if tt.mockError != nil {
					assert.Contains(t, textContent.Text, tt.mockError.Error())
				} else {
					assert.NotEmpty(t, textContent.Text, "Error message should not be empty for parameter errors")
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

// TestHandleListRegularStacks verifies the HandleListRegularStacks MCP tool handler.
func TestHandleListRegularStacks(t *testing.T) {
tests := []struct {
name        string
mockStacks  []models.RegularStack
mockError   error
expectError bool
}{
{
name: "successful regular stacks retrieval",
mockStacks: []models.RegularStack{
{ID: 1, Name: "web-app", Status: 1, EndpointID: 2},
{ID: 2, Name: "db-stack", Status: 1, EndpointID: 3},
},
expectError: false,
},
{
name:        "empty list",
mockStacks:  []models.RegularStack{},
expectError: false,
},
{
name:        "api error",
mockError:   fmt.Errorf("connection refused"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
mockClient.On("GetRegularStacks").Return(tt.mockStacks, tt.mockError)

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleListRegularStacks()
result, err := handler(context.Background(), mcp.CallToolRequest{})

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
var stacks []models.RegularStack
textContent := result.Content[0].(mcp.TextContent)
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &stacks)
assert.NoError(t, unmarshalErr)
assert.Equal(t, len(tt.mockStacks), len(stacks))
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleInspectStack verifies the HandleInspectStack MCP tool handler.
func TestHandleInspectStack(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful inspect",
params:    map[string]any{"id": float64(1)},
mockStack: models.RegularStack{ID: 1, Name: "my-stack", Status: 1},
},
{
name:        "missing id",
params:      map[string]any{},
expectError: true,
},
{
name:        "invalid id zero",
params:      map[string]any{"id": float64(0)},
expectError: true,
},
{
name:        "negative id",
params:      map[string]any{"id": float64(-1)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1)},
mockError:   fmt.Errorf("not found"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
if idVal, ok := tt.params["id"]; ok && idVal.(float64) > 0 {
mockClient.On("InspectStack", int(idVal.(float64))).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleInspectStack()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
var stack models.RegularStack
textContent := result.Content[0].(mcp.TextContent)
unmarshalErr := json.Unmarshal([]byte(textContent.Text), &stack)
assert.NoError(t, unmarshalErr)
assert.Equal(t, tt.mockStack.ID, stack.ID)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleDeleteStack verifies the HandleDeleteStack MCP tool handler.
func TestHandleDeleteStack(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockError   error
expectError bool
}{
{
name:   "successful delete",
params: map[string]any{"id": float64(1), "environmentId": float64(2), "removeVolumes": true},
},
{
name:   "successful delete without removeVolumes",
params: map[string]any{"id": float64(1), "environmentId": float64(2)},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1)},
expectError: true,
},
{
name:        "invalid id zero",
params:      map[string]any{"id": float64(0), "environmentId": float64(2)},
expectError: true,
},
{
name:        "invalid environmentId zero",
params:      map[string]any{"id": float64(1), "environmentId": float64(0)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
mockError:   fmt.Errorf("forbidden"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
if hasID && hasEnv && idVal.(float64) > 0 && envVal.(float64) > 0 {
removeVolumes, _ := tt.params["removeVolumes"].(bool)
mockClient.On("DeleteStack", int(idVal.(float64)), int(envVal.(float64)), removeVolumes).Return(tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleDeleteStack()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
textContent := result.Content[0].(mcp.TextContent)
assert.Contains(t, textContent.Text, "successfully")
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleInspectStackFile verifies the HandleInspectStackFile MCP tool handler.
func TestHandleInspectStackFile(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockContent string
mockError   error
expectError bool
}{
{
name:        "successful file retrieval",
params:      map[string]any{"id": float64(1)},
mockContent: "version: '3'\nservices:\n  web:\n    image: nginx",
},
{
name:        "missing id",
params:      map[string]any{},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(0)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1)},
mockError:   fmt.Errorf("not found"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
if idVal, ok := tt.params["id"]; ok && idVal.(float64) > 0 {
mockClient.On("InspectStackFile", int(idVal.(float64))).Return(tt.mockContent, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleInspectStackFile()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
textContent := result.Content[0].(mcp.TextContent)
assert.Equal(t, tt.mockContent, textContent.Text)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleUpdateStackGit verifies the HandleUpdateStackGit MCP tool handler.
func TestHandleUpdateStackGit(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful update with all params",
params:    map[string]any{"id": float64(1), "environmentId": float64(2), "referenceName": "main", "prune": true},
mockStack: models.RegularStack{ID: 1, Name: "my-stack"},
},
{
name:      "successful update with minimal params",
params:    map[string]any{"id": float64(1), "environmentId": float64(2)},
mockStack: models.RegularStack{ID: 1, Name: "my-stack"},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1)},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(0), "environmentId": float64(2)},
expectError: true,
},
{
name:        "invalid environmentId",
params:      map[string]any{"id": float64(1), "environmentId": float64(-1)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
mockError:   fmt.Errorf("conflict"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
if hasID && hasEnv && idVal.(float64) > 0 && envVal.(float64) > 0 {
refName, _ := tt.params["referenceName"].(string)
prune, _ := tt.params["prune"].(bool)
mockClient.On("UpdateStackGit", int(idVal.(float64)), int(envVal.(float64)), refName, prune).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleUpdateStackGit()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleRedeployStackGit verifies the HandleRedeployStackGit MCP tool handler.
func TestHandleRedeployStackGit(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful redeploy with all params",
params:    map[string]any{"id": float64(1), "environmentId": float64(2), "pullImage": true, "prune": true},
mockStack: models.RegularStack{ID: 1, Name: "redeployed"},
},
{
name:      "successful redeploy minimal",
params:    map[string]any{"id": float64(1), "environmentId": float64(2)},
mockStack: models.RegularStack{ID: 1, Name: "redeployed"},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1)},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(0), "environmentId": float64(2)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
mockError:   fmt.Errorf("deploy error"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
if hasID && hasEnv && idVal.(float64) > 0 && envVal.(float64) > 0 {
pullImage, _ := tt.params["pullImage"].(bool)
prune, _ := tt.params["prune"].(bool)
mockClient.On("RedeployStackGit", int(idVal.(float64)), int(envVal.(float64)), pullImage, prune).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleRedeployStackGit()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleStartStack verifies the HandleStartStack MCP tool handler.
func TestHandleStartStack(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful start",
params:    map[string]any{"id": float64(1), "environmentId": float64(2)},
mockStack: models.RegularStack{ID: 1, Name: "started-stack", Status: 1},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1)},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(-5), "environmentId": float64(2)},
expectError: true,
},
{
name:        "invalid environmentId",
params:      map[string]any{"id": float64(1), "environmentId": float64(0)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
mockError:   fmt.Errorf("start failed"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
if hasID && hasEnv && idVal.(float64) > 0 && envVal.(float64) > 0 {
mockClient.On("StartStack", int(idVal.(float64)), int(envVal.(float64))).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleStartStack()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleStopStack verifies the HandleStopStack MCP tool handler.
func TestHandleStopStack(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful stop",
params:    map[string]any{"id": float64(1), "environmentId": float64(2)},
mockStack: models.RegularStack{ID: 1, Name: "stopped-stack", Status: 2},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1)},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(0), "environmentId": float64(2)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
mockError:   fmt.Errorf("stop failed"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
if hasID && hasEnv && idVal.(float64) > 0 && envVal.(float64) > 0 {
mockClient.On("StopStack", int(idVal.(float64)), int(envVal.(float64))).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleStopStack()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
}
mockClient.AssertExpectations(t)
})
}
}

// TestHandleMigrateStack verifies the HandleMigrateStack MCP tool handler.
func TestHandleMigrateStack(t *testing.T) {
tests := []struct {
name        string
params      map[string]any
mockStack   models.RegularStack
mockError   error
expectError bool
}{
{
name:      "successful migrate with name",
params:    map[string]any{"id": float64(1), "environmentId": float64(2), "targetEnvironmentId": float64(3), "name": "new-name"},
mockStack: models.RegularStack{ID: 1, Name: "new-name"},
},
{
name:      "successful migrate without name",
params:    map[string]any{"id": float64(1), "environmentId": float64(2), "targetEnvironmentId": float64(3)},
mockStack: models.RegularStack{ID: 1, Name: "original"},
},
{
name:        "missing id",
params:      map[string]any{"environmentId": float64(2), "targetEnvironmentId": float64(3)},
expectError: true,
},
{
name:        "missing environmentId",
params:      map[string]any{"id": float64(1), "targetEnvironmentId": float64(3)},
expectError: true,
},
{
name:        "missing targetEnvironmentId",
params:      map[string]any{"id": float64(1), "environmentId": float64(2)},
expectError: true,
},
{
name:        "invalid id",
params:      map[string]any{"id": float64(0), "environmentId": float64(2), "targetEnvironmentId": float64(3)},
expectError: true,
},
{
name:        "invalid environmentId",
params:      map[string]any{"id": float64(1), "environmentId": float64(-1), "targetEnvironmentId": float64(3)},
expectError: true,
},
{
name:        "invalid targetEnvironmentId",
params:      map[string]any{"id": float64(1), "environmentId": float64(2), "targetEnvironmentId": float64(0)},
expectError: true,
},
{
name:        "api error",
params:      map[string]any{"id": float64(1), "environmentId": float64(2), "targetEnvironmentId": float64(3)},
mockError:   fmt.Errorf("migration failed"),
expectError: true,
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := &MockPortainerClient{}
idVal, hasID := tt.params["id"]
envVal, hasEnv := tt.params["environmentId"]
targetVal, hasTarget := tt.params["targetEnvironmentId"]
if hasID && hasEnv && hasTarget && idVal.(float64) > 0 && envVal.(float64) > 0 && targetVal.(float64) > 0 {
name, _ := tt.params["name"].(string)
mockClient.On("MigrateStack", int(idVal.(float64)), int(envVal.(float64)), int(targetVal.(float64)), name).Return(tt.mockStack, tt.mockError)
}

s := &PortainerMCPServer{cli: mockClient}
handler := s.HandleMigrateStack()
req := mcp.CallToolRequest{}
req.Params.Arguments = tt.params
result, err := handler(context.Background(), req)

assert.NoError(t, err)
if tt.expectError {
assert.True(t, result.IsError)
} else {
assert.False(t, result.IsError)
}
mockClient.AssertExpectations(t)
})
}
}
