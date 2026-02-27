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

// TestHandleGetBackupStatus verifies the HandleGetBackupStatus MCP tool handler.
func TestHandleGetBackupStatus(t *testing.T) {
	tests := []struct {
		name        string
		mockStatus  models.BackupStatus
		mockError   error
		expectError bool
	}{
		{
			name: "successful status retrieval",
			mockStatus: models.BackupStatus{
				Failed:       false,
				TimestampUTC: "2024-01-01T00:00:00Z",
			},
			expectError: false,
		},
		{
			name:        "api error",
			mockStatus:  models.BackupStatus{},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetBackupStatus").Return(tt.mockStatus, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleGetBackupStatus()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var status models.BackupStatus
				err = json.Unmarshal([]byte(textContent.Text), &status)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockStatus, status)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleGetBackupS3Settings verifies the HandleGetBackupS3Settings MCP tool handler.
func TestHandleGetBackupS3Settings(t *testing.T) {
	tests := []struct {
		name         string
		mockSettings models.S3BackupSettings
		mockError    error
		expectError  bool
	}{
		{
			name: "successful settings retrieval",
			mockSettings: models.S3BackupSettings{
				AccessKeyID: "AKID123",
				BucketName:  "my-bucket",
				Region:      "us-east-1",
			},
			expectError: false,
		},
		{
			name:         "api error",
			mockSettings: models.S3BackupSettings{},
			mockError:    fmt.Errorf("api error"),
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("GetBackupS3Settings").Return(tt.mockSettings, tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}
			handler := server.HandleGetBackupS3Settings()
			result, err := handler(context.Background(), mcp.CallToolRequest{})

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)

				var settings models.S3BackupSettings
				err = json.Unmarshal([]byte(textContent.Text), &settings)
				assert.NoError(t, err)
				assert.Equal(t, tt.mockSettings, settings)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleCreateBackup verifies the HandleCreateBackup MCP tool handler.
func TestHandleCreateBackup(t *testing.T) {
	tests := []struct {
		name        string
		password    string
		mockError   error
		expectError bool
	}{
		{
			name:        "successful backup creation",
			password:    "secret",
			expectError: false,
		},
		{
			name:        "successful backup without password",
			password:    "",
			expectError: false,
		},
		{
			name:        "api error",
			password:    "secret",
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}
			mockClient.On("CreateBackup", tt.password).Return(tt.mockError)

			server := &PortainerMCPServer{cli: mockClient}

			args := map[string]any{}
			if tt.password != "" {
				args["password"] = tt.password
			}
			request := CreateMCPRequest(args)

			handler := server.HandleCreateBackup()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, tt.mockError.Error())
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "Backup created successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleBackupToS3 verifies the HandleBackupToS3 MCP tool handler.
func TestHandleBackupToS3(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockError   error
		expectError bool
	}{
		{
			name: "successful backup to S3",
			args: map[string]any{
				"accessKeyID":     "AKID123",
				"secretAccessKey": "secret",
				"bucketName":      "my-bucket",
				"region":          "us-east-1",
			},
			expectError: false,
		},
		{
			name:        "missing required parameter",
			args:        map[string]any{},
			expectError: true,
		},
		{
			name: "api error",
			args: map[string]any{
				"accessKeyID":     "AKID123",
				"secretAccessKey": "secret",
				"bucketName":      "my-bucket",
			},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}

			if _, ok := tt.args["accessKeyID"]; ok && tt.mockError != nil {
				mockClient.On("BackupToS3", models.S3BackupSettings{
					AccessKeyID:     tt.args["accessKeyID"].(string),
					SecretAccessKey: tt.args["secretAccessKey"].(string),
					BucketName:      tt.args["bucketName"].(string),
				}).Return(tt.mockError)
			} else if _, ok := tt.args["accessKeyID"]; ok {
				region, _ := tt.args["region"].(string)
				mockClient.On("BackupToS3", models.S3BackupSettings{
					AccessKeyID:     tt.args["accessKeyID"].(string),
					SecretAccessKey: tt.args["secretAccessKey"].(string),
					BucketName:      tt.args["bucketName"].(string),
					Region:          region,
				}).Return(tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}
			request := CreateMCPRequest(tt.args)

			handler := server.HandleBackupToS3()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "Backup to S3 completed successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// TestHandleRestoreFromS3 verifies the HandleRestoreFromS3 MCP tool handler.
func TestHandleRestoreFromS3(t *testing.T) {
	tests := []struct {
		name        string
		args        map[string]any
		mockError   error
		expectError bool
	}{
		{
			name: "successful restore from S3",
			args: map[string]any{
				"accessKeyID":     "AKID123",
				"secretAccessKey": "secret",
				"bucketName":      "my-bucket",
				"filename":        "backup.tar.gz",
			},
			expectError: false,
		},
		{
			name:        "missing required parameter",
			args:        map[string]any{},
			expectError: true,
		},
		{
			name: "api error",
			args: map[string]any{
				"accessKeyID":     "AKID123",
				"secretAccessKey": "secret",
				"bucketName":      "my-bucket",
				"filename":        "backup.tar.gz",
			},
			mockError:   fmt.Errorf("api error"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockPortainerClient{}

			if _, ok := tt.args["accessKeyID"]; ok {
				mockClient.On("RestoreFromS3",
					tt.args["accessKeyID"].(string),
					tt.args["bucketName"].(string),
					tt.args["filename"].(string),
					"", // password
					"", // region
					"", // s3CompatibleHost
					tt.args["secretAccessKey"].(string),
				).Return(tt.mockError)
			}

			server := &PortainerMCPServer{cli: mockClient}
			request := CreateMCPRequest(tt.args)

			handler := server.HandleRestoreFromS3()
			result, err := handler(context.Background(), request)

			if tt.expectError {
				assert.NoError(t, err)
				assert.True(t, result.IsError)
			} else {
				assert.NoError(t, err)
				textContent, ok := result.Content[0].(mcp.TextContent)
				assert.True(t, ok)
				assert.Contains(t, textContent.Text, "Restore from S3 completed successfully")
			}

			mockClient.AssertExpectations(t)
		})
	}
}
