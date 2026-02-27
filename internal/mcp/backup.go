package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/portainer/models"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddBackupFeatures registers the backup and restore management tools on the MCP server.
func (s *PortainerMCPServer) AddBackupFeatures() {
	s.addToolIfExists(ToolGetBackupStatus, s.HandleGetBackupStatus())
	s.addToolIfExists(ToolGetBackupS3Settings, s.HandleGetBackupS3Settings())

	if !s.readOnly {
		s.addToolIfExists(ToolCreateBackup, s.HandleCreateBackup())
		s.addToolIfExists(ToolBackupToS3, s.HandleBackupToS3())
		s.addToolIfExists(ToolRestoreFromS3, s.HandleRestoreFromS3())
	}
}

// HandleGetBackupStatus returns an MCP tool handler that retrieves backup status.
func (s *PortainerMCPServer) HandleGetBackupStatus() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		status, err := s.cli.GetBackupStatus()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get backup status", err), nil
		}

		return jsonResult(status, "failed to marshal backup status")
	}
}

// HandleGetBackupS3Settings returns an MCP tool handler that retrieves backup s3 settings.
func (s *PortainerMCPServer) HandleGetBackupS3Settings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		settings, err := s.cli.GetBackupS3Settings()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get backup S3 settings", err), nil
		}

		return jsonResult(settings, "failed to marshal backup S3 settings")
	}
}

// HandleCreateBackup returns an MCP tool handler that creates backup.
func (s *PortainerMCPServer) HandleCreateBackup() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		password, err := parser.GetString("password", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid password parameter", err), nil
		}

		err = s.cli.CreateBackup(password)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to create backup", err), nil
		}

		return mcp.NewToolResultText("Backup created successfully"), nil
	}
}

// HandleBackupToS3 returns an MCP tool handler that creates a backup to s3.
func (s *PortainerMCPServer) HandleBackupToS3() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		accessKeyID, err := parser.GetString("accessKeyID", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid accessKeyID parameter", err), nil
		}

		secretAccessKey, err := parser.GetString("secretAccessKey", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid secretAccessKey parameter", err), nil
		}

		bucketName, err := parser.GetString("bucketName", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid bucketName parameter", err), nil
		}

		region, err := parser.GetString("region", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid region parameter", err), nil
		}

		s3CompatibleHost, err := parser.GetString("s3CompatibleHost", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid s3CompatibleHost parameter", err), nil
		}

		password, err := parser.GetString("password", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid password parameter", err), nil
		}

		cronRule, err := parser.GetString("cronRule", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid cronRule parameter", err), nil
		}

		settings := models.S3BackupSettings{
			AccessKeyID:      accessKeyID,
			SecretAccessKey:  secretAccessKey,
			BucketName:       bucketName,
			Region:           region,
			S3CompatibleHost: s3CompatibleHost,
			Password:         password,
			CronRule:         cronRule,
		}

		err = s.cli.BackupToS3(settings)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to backup to S3", err), nil
		}

		return mcp.NewToolResultText("Backup to S3 completed successfully"), nil
	}
}

// HandleRestoreFromS3 returns an MCP tool handler that restores from a backup from s3.
func (s *PortainerMCPServer) HandleRestoreFromS3() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		accessKeyID, err := parser.GetString("accessKeyID", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid accessKeyID parameter", err), nil
		}

		secretAccessKey, err := parser.GetString("secretAccessKey", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid secretAccessKey parameter", err), nil
		}

		bucketName, err := parser.GetString("bucketName", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid bucketName parameter", err), nil
		}

		filename, err := parser.GetString("filename", true)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid filename parameter", err), nil
		}

		password, err := parser.GetString("password", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid password parameter", err), nil
		}

		region, err := parser.GetString("region", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid region parameter", err), nil
		}

		s3CompatibleHost, err := parser.GetString("s3CompatibleHost", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid s3CompatibleHost parameter", err), nil
		}

		err = s.cli.RestoreFromS3(accessKeyID, bucketName, filename, password, region, s3CompatibleHost, secretAccessKey)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to restore from S3", err), nil
		}

		return mcp.NewToolResultText("Restore from S3 completed successfully"), nil
	}
}
