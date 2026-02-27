// Package main implements the portainer-mcp CLI application.
// It provides a Model Context Protocol (MCP) server that exposes
// Portainer container management capabilities as MCP tools.
package main

import (
	"flag"

	"github.com/portainer/portainer-mcp/internal/mcp"
	"github.com/portainer/portainer-mcp/internal/tooldef"
	"github.com/rs/zerolog/log"
)

// defaultToolsPath is the default file path for the tools YAML configuration.
const defaultToolsPath = "tools.yaml"

var (
	// Version is the version of the portainer-mcp application, set at build time.
	Version string
	// BuildDate is the date the portainer-mcp application was built, set at build time.
	BuildDate string
	// Commit is the git commit hash of the portainer-mcp application, set at build time.
	Commit string
)

func main() {
	log.Info().
		Str("version", Version).
		Str("build-date", BuildDate).
		Str("commit", Commit).
		Msg("Portainer MCP server")

	serverFlag := flag.String("server", "", "The Portainer server URL")
	tokenFlag := flag.String("token", "", "The authentication token for the Portainer server")
	toolsFlag := flag.String("tools", "", "The path to the tools YAML file")
	readOnlyFlag := flag.Bool("read-only", false, "Run in read-only mode")
	granularToolsFlag := flag.Bool("granular-tools", false, "Register all individual tools instead of grouped meta-tools")
	disableVersionCheckFlag := flag.Bool("disable-version-check", false, "Disable Portainer server version check")
	skipTLSVerifyFlag := flag.Bool("skip-tls-verify", false, "Skip TLS certificate verification (insecure, use only for self-signed certs)")

	flag.Parse()

	if *serverFlag == "" || *tokenFlag == "" {
		log.Fatal().Msg("Both -server and -token flags are required")
	}

	toolsPath := *toolsFlag
	if toolsPath == "" {
		toolsPath = defaultToolsPath
	}

	// We first check if the tools.yaml file exists
	// We'll create it from the embedded version if it doesn't exist
	exists, err := tooldef.CreateToolsFileIfNotExists(toolsPath)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create tools.yaml file")
	}

	if exists {
		log.Info().Msg("using existing tools.yaml file")
	} else {
		log.Info().Msg("created tools.yaml file")
	}

	log.Info().
		Str("portainer-host", *serverFlag).
		Str("tools-path", toolsPath).
		Bool("read-only", *readOnlyFlag).
		Bool("granular-tools", *granularToolsFlag).
		Bool("disable-version-check", *disableVersionCheckFlag).
		Bool("skip-tls-verify", *skipTLSVerifyFlag).
		Msg("starting MCP server")

	server, err := mcp.NewPortainerMCPServer(*serverFlag, *tokenFlag, toolsPath, mcp.WithReadOnly(*readOnlyFlag), mcp.WithGranularTools(*granularToolsFlag), mcp.WithDisableVersionCheck(*disableVersionCheckFlag), mcp.WithSkipTLSVerify(*skipTLSVerifyFlag))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create server")
	}

	if *granularToolsFlag {
		server.AddEnvironmentFeatures()
		server.AddEnvironmentGroupFeatures()
		server.AddTagFeatures()
		server.AddStackFeatures()
		server.AddSettingsFeatures()
		server.AddSSLFeatures()
		server.AddUserFeatures()
		server.AddTeamFeatures()
		server.AddAccessGroupFeatures()
		server.AddDockerProxyFeatures()
		server.AddKubernetesProxyFeatures()
		server.AddKubernetesNativeFeatures()
		server.AddSystemFeatures()
		server.AddWebhookFeatures()
		server.AddCustomTemplateFeatures()
		server.AddRegistryFeatures()
		server.AddBackupFeatures()
		server.AddRoleFeatures()
		server.AddMotdFeatures()
		server.AddAuthFeatures()
		server.AddEdgeJobFeatures()
		server.AddEdgeUpdateScheduleFeatures()
		server.AddAppTemplateFeatures()
		server.AddHelmFeatures()
	} else {
		server.RegisterMetaTools()
	}

	err = server.Start()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to start server")
	}
}
