package mcp

import (
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// metaAction maps an action name to its handler and access metadata.
type metaAction struct {
	name     string
	handler  func(s *PortainerMCPServer) server.ToolHandlerFunc
	readOnly bool // true = always available; false = hidden in read-only mode
}

// metaToolDef describes a single grouped meta-tool.
type metaToolDef struct {
	name        string
	description string
	actions     []metaAction
	annotation  mcp.ToolAnnotation
}

// boolPtr is a convenience helper for creating *bool values.
func boolPtr(v bool) *bool { return &v }

// metaToolDefinitions returns the complete list of meta-tool groups.
// Each group aggregates several existing granular tools behind a single
// "action" enum parameter. Read-only mode filters write actions at
// registration time, so the enum only exposes permitted actions.
func metaToolDefinitions() []metaToolDef {
	return []metaToolDef{
		{
			name:        "manage_environments",
			description: "Manage Portainer environments, environment groups, and tags. Actions: list_environments, get_environment, delete_environment, snapshot_environment, snapshot_all_environments, update_environment_tags, update_environment_user_accesses, update_environment_team_accesses, list_environment_groups, create_environment_group, update_environment_group_name, update_environment_group_environments, update_environment_group_tags, list_environment_tags, create_environment_tag, delete_environment_tag. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_environments", handler: (*PortainerMCPServer).HandleGetEnvironments, readOnly: true},
				{name: "get_environment", handler: (*PortainerMCPServer).HandleGetEnvironment, readOnly: true},
				{name: "delete_environment", handler: (*PortainerMCPServer).HandleDeleteEnvironment, readOnly: false},
				{name: "snapshot_environment", handler: (*PortainerMCPServer).HandleSnapshotEnvironment, readOnly: false},
				{name: "snapshot_all_environments", handler: (*PortainerMCPServer).HandleSnapshotAllEnvironments, readOnly: false},
				{name: "update_environment_tags", handler: (*PortainerMCPServer).HandleUpdateEnvironmentTags, readOnly: false},
				{name: "update_environment_user_accesses", handler: (*PortainerMCPServer).HandleUpdateEnvironmentUserAccesses, readOnly: false},
				{name: "update_environment_team_accesses", handler: (*PortainerMCPServer).HandleUpdateEnvironmentTeamAccesses, readOnly: false},
				{name: "list_environment_groups", handler: (*PortainerMCPServer).HandleGetEnvironmentGroups, readOnly: true},
				{name: "create_environment_group", handler: (*PortainerMCPServer).HandleCreateEnvironmentGroup, readOnly: false},
				{name: "update_environment_group_name", handler: (*PortainerMCPServer).HandleUpdateEnvironmentGroupName, readOnly: false},
				{name: "update_environment_group_environments", handler: (*PortainerMCPServer).HandleUpdateEnvironmentGroupEnvironments, readOnly: false},
				{name: "update_environment_group_tags", handler: (*PortainerMCPServer).HandleUpdateEnvironmentGroupTags, readOnly: false},
				{name: "list_environment_tags", handler: (*PortainerMCPServer).HandleGetEnvironmentTags, readOnly: true},
				{name: "create_environment_tag", handler: (*PortainerMCPServer).HandleCreateEnvironmentTag, readOnly: false},
				{name: "delete_environment_tag", handler: (*PortainerMCPServer).HandleDeleteEnvironmentTag, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Environments",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_stacks",
			description: "Manage Docker stacks (Compose and Edge deployments). Actions: list_stacks, list_regular_stacks, get_stack, get_stack_file, inspect_stack_file, create_stack, update_stack, delete_stack, update_stack_git, redeploy_stack_git, start_stack, stop_stack, migrate_stack. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_stacks", handler: (*PortainerMCPServer).HandleGetStacks, readOnly: true},
				{name: "list_regular_stacks", handler: (*PortainerMCPServer).HandleListRegularStacks, readOnly: true},
				{name: "get_stack", handler: (*PortainerMCPServer).HandleInspectStack, readOnly: true},
				{name: "get_stack_file", handler: (*PortainerMCPServer).HandleGetStackFile, readOnly: true},
				{name: "inspect_stack_file", handler: (*PortainerMCPServer).HandleInspectStackFile, readOnly: true},
				{name: "create_stack", handler: (*PortainerMCPServer).HandleCreateStack, readOnly: false},
				{name: "update_stack", handler: (*PortainerMCPServer).HandleUpdateStack, readOnly: false},
				{name: "delete_stack", handler: (*PortainerMCPServer).HandleDeleteStack, readOnly: false},
				{name: "update_stack_git", handler: (*PortainerMCPServer).HandleUpdateStackGit, readOnly: false},
				{name: "redeploy_stack_git", handler: (*PortainerMCPServer).HandleRedeployStackGit, readOnly: false},
				{name: "start_stack", handler: (*PortainerMCPServer).HandleStartStack, readOnly: false},
				{name: "stop_stack", handler: (*PortainerMCPServer).HandleStopStack, readOnly: false},
				{name: "migrate_stack", handler: (*PortainerMCPServer).HandleMigrateStack, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Stacks",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_access_groups",
			description: "Manage access groups for environment-level permissions. Actions: list_access_groups, create_access_group, update_access_group_name, update_access_group_user_accesses, update_access_group_team_accesses, add_environment_to_access_group, remove_environment_from_access_group. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_access_groups", handler: (*PortainerMCPServer).HandleGetAccessGroups, readOnly: true},
				{name: "create_access_group", handler: (*PortainerMCPServer).HandleCreateAccessGroup, readOnly: false},
				{name: "update_access_group_name", handler: (*PortainerMCPServer).HandleUpdateAccessGroupName, readOnly: false},
				{name: "update_access_group_user_accesses", handler: (*PortainerMCPServer).HandleUpdateAccessGroupUserAccesses, readOnly: false},
				{name: "update_access_group_team_accesses", handler: (*PortainerMCPServer).HandleUpdateAccessGroupTeamAccesses, readOnly: false},
				{name: "add_environment_to_access_group", handler: (*PortainerMCPServer).HandleAddEnvironmentToAccessGroup, readOnly: false},
				{name: "remove_environment_from_access_group", handler: (*PortainerMCPServer).HandleRemoveEnvironmentFromAccessGroup, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Access Groups",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_users",
			description: "Manage Portainer user accounts and roles. Actions: list_users, get_user, create_user, delete_user, update_user_role. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_users", handler: (*PortainerMCPServer).HandleGetUsers, readOnly: true},
				{name: "get_user", handler: (*PortainerMCPServer).HandleGetUser, readOnly: true},
				{name: "create_user", handler: (*PortainerMCPServer).HandleCreateUser, readOnly: false},
				{name: "delete_user", handler: (*PortainerMCPServer).HandleDeleteUser, readOnly: false},
				{name: "update_user_role", handler: (*PortainerMCPServer).HandleUpdateUserRole, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Users",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_teams",
			description: "Manage Portainer teams and membership. Actions: list_teams, get_team, create_team, delete_team, update_team_name, update_team_members. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_teams", handler: (*PortainerMCPServer).HandleGetTeams, readOnly: true},
				{name: "get_team", handler: (*PortainerMCPServer).HandleGetTeam, readOnly: true},
				{name: "create_team", handler: (*PortainerMCPServer).HandleCreateTeam, readOnly: false},
				{name: "delete_team", handler: (*PortainerMCPServer).HandleDeleteTeam, readOnly: false},
				{name: "update_team_name", handler: (*PortainerMCPServer).HandleUpdateTeamName, readOnly: false},
				{name: "update_team_members", handler: (*PortainerMCPServer).HandleUpdateTeamMembers, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Teams",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_docker",
			description: "Interact with Docker environments via dashboards and proxy API calls. Actions: get_docker_dashboard, docker_proxy. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "get_docker_dashboard", handler: (*PortainerMCPServer).HandleGetDockerDashboard, readOnly: true},
				{name: "docker_proxy", handler: (*PortainerMCPServer).HandleDockerProxy, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Docker",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(true),
			},
		},
		{
			name:        "manage_kubernetes",
			description: "Interact with Kubernetes environments via dashboards, namespaces, kubeconfig, and proxy API calls. Actions: get_kubernetes_resource_stripped, get_kubernetes_dashboard, list_kubernetes_namespaces, get_kubernetes_config, kubernetes_proxy. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "get_kubernetes_resource_stripped", handler: (*PortainerMCPServer).HandleKubernetesProxyStripped, readOnly: true},
				{name: "get_kubernetes_dashboard", handler: (*PortainerMCPServer).HandleGetKubernetesDashboard, readOnly: true},
				{name: "list_kubernetes_namespaces", handler: (*PortainerMCPServer).HandleListKubernetesNamespaces, readOnly: true},
				{name: "get_kubernetes_config", handler: (*PortainerMCPServer).HandleGetKubernetesConfig, readOnly: true},
				{name: "kubernetes_proxy", handler: (*PortainerMCPServer).HandleKubernetesProxy, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Kubernetes",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(true),
			},
		},
		{
			name:        "manage_helm",
			description: "Manage Helm repositories, charts, and releases on Kubernetes environments. Actions: list_helm_repositories, search_helm_charts, list_helm_releases, get_helm_release_history, add_helm_repository, remove_helm_repository, install_helm_chart, delete_helm_release. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_helm_repositories", handler: (*PortainerMCPServer).HandleListHelmRepositories, readOnly: true},
				{name: "search_helm_charts", handler: (*PortainerMCPServer).HandleSearchHelmCharts, readOnly: true},
				{name: "list_helm_releases", handler: (*PortainerMCPServer).HandleListHelmReleases, readOnly: true},
				{name: "get_helm_release_history", handler: (*PortainerMCPServer).HandleGetHelmReleaseHistory, readOnly: true},
				{name: "add_helm_repository", handler: (*PortainerMCPServer).HandleAddHelmRepository, readOnly: false},
				{name: "remove_helm_repository", handler: (*PortainerMCPServer).HandleRemoveHelmRepository, readOnly: false},
				{name: "install_helm_chart", handler: (*PortainerMCPServer).HandleInstallHelmChart, readOnly: false},
				{name: "delete_helm_release", handler: (*PortainerMCPServer).HandleDeleteHelmRelease, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Helm",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_registries",
			description: "Manage container registries (Quay, Azure, DockerHub, GitLab, ECR, custom). Actions: list_registries, get_registry, create_registry, update_registry, delete_registry. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_registries", handler: (*PortainerMCPServer).HandleListRegistries, readOnly: true},
				{name: "get_registry", handler: (*PortainerMCPServer).HandleGetRegistry, readOnly: true},
				{name: "create_registry", handler: (*PortainerMCPServer).HandleCreateRegistry, readOnly: false},
				{name: "update_registry", handler: (*PortainerMCPServer).HandleUpdateRegistry, readOnly: false},
				{name: "delete_registry", handler: (*PortainerMCPServer).HandleDeleteRegistry, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Registries",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_templates",
			description: "Manage custom and application templates for stack deployment. Actions: list_custom_templates, get_custom_template, get_custom_template_file, create_custom_template, delete_custom_template, list_app_templates, get_app_template_file. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_custom_templates", handler: (*PortainerMCPServer).HandleListCustomTemplates, readOnly: true},
				{name: "get_custom_template", handler: (*PortainerMCPServer).HandleGetCustomTemplate, readOnly: true},
				{name: "get_custom_template_file", handler: (*PortainerMCPServer).HandleGetCustomTemplateFile, readOnly: true},
				{name: "create_custom_template", handler: (*PortainerMCPServer).HandleCreateCustomTemplate, readOnly: false},
				{name: "delete_custom_template", handler: (*PortainerMCPServer).HandleDeleteCustomTemplate, readOnly: false},
				{name: "list_app_templates", handler: (*PortainerMCPServer).HandleListAppTemplates, readOnly: true},
				{name: "get_app_template_file", handler: (*PortainerMCPServer).HandleGetAppTemplateFile, readOnly: true},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Templates",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_backups",
			description: "Manage Portainer server backups and restore (local and S3). Actions: get_backup_status, get_backup_s3_settings, create_backup, backup_to_s3, restore_from_s3. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "get_backup_status", handler: (*PortainerMCPServer).HandleGetBackupStatus, readOnly: true},
				{name: "get_backup_s3_settings", handler: (*PortainerMCPServer).HandleGetBackupS3Settings, readOnly: true},
				{name: "create_backup", handler: (*PortainerMCPServer).HandleCreateBackup, readOnly: false},
				{name: "backup_to_s3", handler: (*PortainerMCPServer).HandleBackupToS3, readOnly: false},
				{name: "restore_from_s3", handler: (*PortainerMCPServer).HandleRestoreFromS3, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Backups",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_webhooks",
			description: "Manage webhooks for container services and automated deployments. Actions: list_webhooks, create_webhook, delete_webhook. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_webhooks", handler: (*PortainerMCPServer).HandleListWebhooks, readOnly: true},
				{name: "create_webhook", handler: (*PortainerMCPServer).HandleCreateWebhook, readOnly: false},
				{name: "delete_webhook", handler: (*PortainerMCPServer).HandleDeleteWebhook, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Webhooks",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_edge",
			description: "Manage Edge compute jobs and update schedules for remote environments. Actions: list_edge_jobs, get_edge_job, get_edge_job_file, create_edge_job, delete_edge_job, list_edge_update_schedules. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "list_edge_jobs", handler: (*PortainerMCPServer).HandleListEdgeJobs, readOnly: true},
				{name: "get_edge_job", handler: (*PortainerMCPServer).HandleGetEdgeJob, readOnly: true},
				{name: "get_edge_job_file", handler: (*PortainerMCPServer).HandleGetEdgeJobFile, readOnly: true},
				{name: "create_edge_job", handler: (*PortainerMCPServer).HandleCreateEdgeJob, readOnly: false},
				{name: "delete_edge_job", handler: (*PortainerMCPServer).HandleDeleteEdgeJob, readOnly: false},
				{name: "list_edge_update_schedules", handler: (*PortainerMCPServer).HandleListEdgeUpdateSchedules, readOnly: true},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Edge",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(true),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_settings",
			description: "Manage Portainer server settings, public settings, and SSL configuration. Actions: get_settings, get_public_settings, update_settings, get_ssl_settings, update_ssl_settings. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "get_settings", handler: (*PortainerMCPServer).HandleGetSettings, readOnly: true},
				{name: "get_public_settings", handler: (*PortainerMCPServer).HandleGetPublicSettings, readOnly: true},
				{name: "update_settings", handler: (*PortainerMCPServer).HandleUpdateSettings, readOnly: false},
				{name: "get_ssl_settings", handler: (*PortainerMCPServer).HandleGetSSLSettings, readOnly: true},
				{name: "update_ssl_settings", handler: (*PortainerMCPServer).HandleUpdateSSLSettings, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage Settings",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(true),
				OpenWorldHint:   boolPtr(false),
			},
		},
		{
			name:        "manage_system",
			description: "Portainer system info, roles, MOTD, and authentication. Actions: get_system_status, list_roles, get_motd, authenticate, logout. Set 'action' parameter to choose.",
			actions: []metaAction{
				{name: "get_system_status", handler: (*PortainerMCPServer).HandleGetSystemStatus, readOnly: true},
				{name: "list_roles", handler: (*PortainerMCPServer).HandleListRoles, readOnly: true},
				{name: "get_motd", handler: (*PortainerMCPServer).HandleGetMOTD, readOnly: true},
				{name: "authenticate", handler: (*PortainerMCPServer).HandleAuthenticateUser, readOnly: true},
				{name: "logout", handler: (*PortainerMCPServer).HandleLogout, readOnly: false},
			},
			annotation: mcp.ToolAnnotation{
				Title:           "Manage System",
				ReadOnlyHint:    boolPtr(false),
				DestructiveHint: boolPtr(false),
				IdempotentHint:  boolPtr(false),
				OpenWorldHint:   boolPtr(false),
			},
		},
	}
}
