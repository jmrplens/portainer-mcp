package models

import (
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// BackupStatus represents the status of the last backup
type BackupStatus struct {
	Failed       bool   `json:"failed"`
	TimestampUTC string `json:"timestampUTC"`
}

// S3BackupSettings represents S3 backup configuration
type S3BackupSettings struct {
	AccessKeyID      string `json:"accessKeyID"`
	BucketName       string `json:"bucketName"`
	CronRule         string `json:"cronRule"`
	Password         string `json:"password"`
	Region           string `json:"region"`
	S3CompatibleHost string `json:"s3CompatibleHost"`
	SecretAccessKey  string `json:"secretAccessKey"`
}

// ConvertToBackupStatus converts a raw BackupBackupStatus to a local BackupStatus
func ConvertToBackupStatus(raw *apimodels.BackupBackupStatus) BackupStatus {
	if raw == nil {
		return BackupStatus{}
	}

	return BackupStatus{
		Failed:       raw.Failed,
		TimestampUTC: raw.TimestampUTC,
	}
}

// ConvertToS3BackupSettings converts a raw PortainereeS3BackupSettings to a local S3BackupSettings
func ConvertToS3BackupSettings(raw *apimodels.PortainereeS3BackupSettings) S3BackupSettings {
	if raw == nil {
		return S3BackupSettings{}
	}

	return S3BackupSettings{
		AccessKeyID:      raw.AccessKeyID,
		BucketName:       raw.BucketName,
		CronRule:         raw.CronRule,
		Password:         raw.Password,
		Region:           raw.Region,
		S3CompatibleHost: raw.S3CompatibleHost,
		SecretAccessKey:  raw.SecretAccessKey,
	}
}
