package client

import (
	"errors"
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/stretchr/testify/assert"
)

// TestGetBackupStatus verifies get backup status behavior.
func TestGetBackupStatus(t *testing.T) {
	tests := []struct {
		name          string
		mockStatus    *apimodels.BackupBackupStatus
		mockError     error
		expected      models.BackupStatus
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockStatus: &apimodels.BackupBackupStatus{
				Failed:       false,
				TimestampUTC: "2024-01-01T00:00:00Z",
			},
			expected: models.BackupStatus{
				Failed:       false,
				TimestampUTC: "2024-01-01T00:00:00Z",
			},
		},
		{
			name:          "api error",
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetBackupStatus").Return(tt.mockStatus, tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			status, err := client.GetBackupStatus()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, status)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestGetBackupS3Settings verifies get backup s3 settings behavior.
func TestGetBackupS3Settings(t *testing.T) {
	tests := []struct {
		name          string
		mockSettings  *apimodels.PortainereeS3BackupSettings
		mockError     error
		expected      models.S3BackupSettings
		expectedError bool
	}{
		{
			name: "successful retrieval",
			mockSettings: &apimodels.PortainereeS3BackupSettings{
				AccessKeyID: "AKID123",
				BucketName:  "my-bucket",
				Region:      "us-east-1",
			},
			expected: models.S3BackupSettings{
				AccessKeyID: "AKID123",
				BucketName:  "my-bucket",
				Region:      "us-east-1",
			},
		},
		{
			name:          "api error",
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("GetBackupSettings").Return(tt.mockSettings, tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			settings, err := client.GetBackupS3Settings()

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, settings)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestCreateBackup verifies create backup behavior.
func TestCreateBackup(t *testing.T) {
	tests := []struct {
		name          string
		password      string
		mockError     error
		expectedError bool
	}{
		{
			name:     "successful backup",
			password: "secret",
		},
		{
			name:          "api error",
			password:      "secret",
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			mockAPI.On("CreateBackup", tt.password).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			err := client.CreateBackup(tt.password)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestBackupToS3 verifies backup to s3 behavior.
func TestBackupToS3(t *testing.T) {
	tests := []struct {
		name          string
		settings      models.S3BackupSettings
		mockError     error
		expectedError bool
	}{
		{
			name: "successful backup to S3",
			settings: models.S3BackupSettings{
				AccessKeyID:     "AKID123",
				SecretAccessKey: "secret",
				BucketName:      "my-bucket",
				Region:          "us-east-1",
			},
		},
		{
			name: "api error",
			settings: models.S3BackupSettings{
				AccessKeyID:     "AKID123",
				SecretAccessKey: "secret",
				BucketName:      "my-bucket",
			},
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			expectedBody := &apimodels.BackupS3BackupPayload{
				AccessKeyID:      tt.settings.AccessKeyID,
				SecretAccessKey:  tt.settings.SecretAccessKey,
				BucketName:       tt.settings.BucketName,
				Region:           tt.settings.Region,
				S3CompatibleHost: tt.settings.S3CompatibleHost,
				Password:         tt.settings.Password,
				CronRule:         tt.settings.CronRule,
			}
			mockAPI.On("BackupToS3", expectedBody).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			err := client.BackupToS3(tt.settings)

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}

// TestRestoreFromS3 verifies restore from s3 behavior.
func TestRestoreFromS3(t *testing.T) {
	tests := []struct {
		name          string
		mockError     error
		expectedError bool
	}{
		{
			name: "successful restore",
		},
		{
			name:          "api error",
			mockError:     errors.New("api error"),
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockAPI := new(MockPortainerAPI)
			expectedBody := &apimodels.BackupRestoreS3Settings{
				AccessKeyID:      "AKID123",
				SecretAccessKey:  "secret",
				BucketName:       "my-bucket",
				Filename:         "backup.tar.gz",
				Password:         "pass",
				Region:           "us-east-1",
				S3CompatibleHost: "minio.example.com",
			}
			mockAPI.On("RestoreFromS3", expectedBody).Return(tt.mockError)

			client := &PortainerClient{cli: mockAPI}
			err := client.RestoreFromS3("AKID123", "my-bucket", "backup.tar.gz", "pass", "us-east-1", "minio.example.com", "secret")

			if tt.expectedError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			mockAPI.AssertExpectations(t)
		})
	}
}
