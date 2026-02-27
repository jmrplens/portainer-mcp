package client

import (
	"fmt"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
)

// GetBackupStatus retrieves the status of the last backup.
func (c *PortainerClient) GetBackupStatus() (models.BackupStatus, error) {
	raw, err := c.cli.GetBackupStatus()
	if err != nil {
		return models.BackupStatus{}, fmt.Errorf("failed to get backup status: %w", err)
	}

	return models.ConvertToBackupStatus(raw), nil
}

// GetBackupS3Settings retrieves the S3 backup settings.
func (c *PortainerClient) GetBackupS3Settings() (models.S3BackupSettings, error) {
	raw, err := c.cli.GetBackupSettings()
	if err != nil {
		return models.S3BackupSettings{}, fmt.Errorf("failed to get backup S3 settings: %w", err)
	}

	return models.ConvertToS3BackupSettings(raw), nil
}

// CreateBackup triggers a backup with an optional password.
func (c *PortainerClient) CreateBackup(password string) error {
	return c.cli.CreateBackup(password)
}

// BackupToS3 triggers a backup to S3.
func (c *PortainerClient) BackupToS3(settings models.S3BackupSettings) error {
	body := &apimodels.BackupS3BackupPayload{
		AccessKeyID:      settings.AccessKeyID,
		BucketName:       settings.BucketName,
		CronRule:         settings.CronRule,
		Password:         settings.Password,
		Region:           settings.Region,
		S3CompatibleHost: settings.S3CompatibleHost,
		SecretAccessKey:  settings.SecretAccessKey,
	}

	return c.cli.BackupToS3(body)
}

// RestoreFromS3 triggers a restore from S3.
func (c *PortainerClient) RestoreFromS3(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey string) error {
	body := &apimodels.BackupRestoreS3Settings{
		AccessKeyID:      accessKeyID,
		BucketName:       bucketName,
		Filename:         filename,
		Password:         password,
		Region:           region,
		S3CompatibleHost: s3CompatibleHost,
		SecretAccessKey:  secretAccessKey,
	}

	return c.cli.RestoreFromS3(body)
}
