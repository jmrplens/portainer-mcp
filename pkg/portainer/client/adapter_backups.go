package client

import (
	"fmt"

	"github.com/portainer/client-api-go/v2/pkg/client/backup"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// GetBackupStatus retrieves the status of the last backup.
func (a *portainerAPIAdapter) GetBackupStatus() (*apimodels.BackupBackupStatus, error) {
	params := backup.NewBackupStatusFetchParams()
	resp, err := a.swagger.Backup.BackupStatusFetch(params)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup status: %w", err)
	}
	return resp.Payload, nil
}

// GetBackupSettings retrieves the S3 backup settings.
func (a *portainerAPIAdapter) GetBackupSettings() (*apimodels.PortainereeS3BackupSettings, error) {
	params := backup.NewBackupSettingsFetchParams()
	resp, err := a.swagger.Backup.BackupSettingsFetch(params, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup settings: %w", err)
	}
	return resp.Payload, nil
}

// CreateBackup triggers a backup with an optional password.
func (a *portainerAPIAdapter) CreateBackup(password string) error {
	body := &apimodels.BackupBackupPayload{
		Password: password,
	}
	params := backup.NewBackupParams().WithBody(body)
	_, err := a.swagger.Backup.Backup(params, nil)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	return nil
}

// BackupToS3 triggers a backup to S3.
func (a *portainerAPIAdapter) BackupToS3(body *apimodels.BackupS3BackupPayload) error {
	params := backup.NewBackupToS3Params().WithBody(body)
	_, err := a.swagger.Backup.BackupToS3(params, nil)
	if err != nil {
		return fmt.Errorf("failed to backup to S3: %w", err)
	}
	return nil
}

// RestoreFromS3 triggers a restore from S3.
func (a *portainerAPIAdapter) RestoreFromS3(body *apimodels.BackupRestoreS3Settings) error {
	params := backup.NewRestoreFromS3Params().WithBody(body)
	_, err := a.swagger.Backup.RestoreFromS3(params)
	if err != nil {
		return fmt.Errorf("failed to restore from S3: %w", err)
	}
	return nil
}
