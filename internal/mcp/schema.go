package mcp

import "slices"

// Tool names as defined in the YAML file
const (
	ToolCreateEnvironmentGroup             = "createEnvironmentGroup"
	ToolListEnvironmentGroups              = "listEnvironmentGroups"
	ToolCreateAccessGroup                  = "createAccessGroup"
	ToolListAccessGroups                   = "listAccessGroups"
	ToolAddEnvironmentToAccessGroup        = "addEnvironmentToAccessGroup"
	ToolRemoveEnvironmentFromAccessGroup   = "removeEnvironmentFromAccessGroup"
	ToolListEnvironments                   = "listEnvironments"
	ToolGetEnvironment                     = "getEnvironment"
	ToolDeleteEnvironment                  = "deleteEnvironment"
	ToolSnapshotEnvironment                = "snapshotEnvironment"
	ToolSnapshotAllEnvironments            = "snapshotAllEnvironments"
	ToolGetStackFile                       = "getStackFile"
	ToolCreateStack                        = "createStack"
	ToolListStacks                         = "listStacks"
	ToolListRegularStacks                  = "listRegularStacks"
	ToolUpdateStack                        = "updateStack"
	ToolGetStack                           = "getStack"
	ToolDeleteStack                        = "deleteStack"
	ToolInspectStackFile                   = "inspectStackFile"
	ToolUpdateStackGit                     = "updateStackGit"
	ToolRedeployStackGit                   = "redeployStackGit"
	ToolStartStack                         = "startStack"
	ToolStopStack                          = "stopStack"
	ToolMigrateStack                       = "migrateStack"
	ToolCreateEnvironmentTag               = "createEnvironmentTag"
	ToolDeleteEnvironmentTag               = "deleteEnvironmentTag"
	ToolListEnvironmentTags                = "listEnvironmentTags"
	ToolCreateTeam                         = "createTeam"
	ToolGetTeam                            = "getTeam"
	ToolDeleteTeam                         = "deleteTeam"
	ToolListTeams                          = "listTeams"
	ToolUpdateTeamName                     = "updateTeamName"
	ToolUpdateTeamMembers                  = "updateTeamMembers"
	ToolListUsers                          = "listUsers"
	ToolCreateUser                         = "createUser"
	ToolGetUser                            = "getUser"
	ToolDeleteUser                         = "deleteUser"
	ToolUpdateUserRole                     = "updateUserRole"
	ToolGetSettings                        = "getSettings"
	ToolUpdateSettings                     = "updateSettings"
	ToolGetPublicSettings                  = "getPublicSettings"
	ToolGetSSLSettings                     = "getSSLSettings"
	ToolUpdateSSLSettings                  = "updateSSLSettings"
	ToolListAppTemplates                   = "listAppTemplates"
	ToolGetAppTemplateFile                 = "getAppTemplateFile"
	ToolUpdateAccessGroupName              = "updateAccessGroupName"
	ToolUpdateAccessGroupUserAccesses      = "updateAccessGroupUserAccesses"
	ToolUpdateAccessGroupTeamAccesses      = "updateAccessGroupTeamAccesses"
	ToolUpdateEnvironmentTags              = "updateEnvironmentTags"
	ToolUpdateEnvironmentUserAccesses      = "updateEnvironmentUserAccesses"
	ToolUpdateEnvironmentTeamAccesses      = "updateEnvironmentTeamAccesses"
	ToolUpdateEnvironmentGroupName         = "updateEnvironmentGroupName"
	ToolUpdateEnvironmentGroupEnvironments = "updateEnvironmentGroupEnvironments"
	ToolUpdateEnvironmentGroupTags         = "updateEnvironmentGroupTags"
	ToolDockerProxy                        = "dockerProxy"
	ToolGetDockerDashboard                 = "getDockerDashboard"
	ToolKubernetesProxy                    = "kubernetesProxy"
	ToolKubernetesProxyStripped            = "getKubernetesResourceStripped"
	ToolGetKubernetesDashboard             = "getKubernetesDashboard"
	ToolListKubernetesNamespaces           = "listKubernetesNamespaces"
	ToolGetKubernetesConfig                = "getKubernetesConfig"
	ToolGetSystemStatus                    = "getSystemStatus"
	ToolListCustomTemplates                = "listCustomTemplates"
	ToolGetCustomTemplate                  = "getCustomTemplate"
	ToolGetCustomTemplateFile              = "getCustomTemplateFile"
	ToolCreateCustomTemplate               = "createCustomTemplate"
	ToolDeleteCustomTemplate               = "deleteCustomTemplate"
	ToolListRegistries                     = "listRegistries"
	ToolGetRegistry                        = "getRegistry"
	ToolCreateRegistry                     = "createRegistry"
	ToolUpdateRegistry                     = "updateRegistry"
	ToolDeleteRegistry                     = "deleteRegistry"
	ToolGetBackupStatus                    = "getBackupStatus"
	ToolGetBackupS3Settings                = "getBackupS3Settings"
	ToolCreateBackup                       = "createBackup"
	ToolBackupToS3                         = "backupToS3"
	ToolRestoreFromS3                      = "restoreFromS3"
	ToolListRoles                          = "listRoles"
	ToolGetMOTD                            = "getMOTD"
	ToolListWebhooks                       = "listWebhooks"
	ToolCreateWebhook                      = "createWebhook"
	ToolDeleteWebhook                      = "deleteWebhook"
	ToolListEdgeJobs                       = "listEdgeJobs"
	ToolGetEdgeJob                         = "getEdgeJob"
	ToolGetEdgeJobFile                     = "getEdgeJobFile"
	ToolCreateEdgeJob                      = "createEdgeJob"
	ToolDeleteEdgeJob                      = "deleteEdgeJob"
	ToolListEdgeUpdateSchedules            = "listEdgeUpdateSchedules"
	ToolAuthenticate                       = "authenticate"
	ToolLogout                             = "logout"
	ToolListHelmRepositories               = "listHelmRepositories"
	ToolAddHelmRepository                  = "addHelmRepository"
	ToolRemoveHelmRepository               = "removeHelmRepository"
	ToolSearchHelmCharts                   = "searchHelmCharts"
	ToolInstallHelmChart                   = "installHelmChart"
	ToolListHelmReleases                   = "listHelmReleases"
	ToolDeleteHelmRelease                  = "deleteHelmRelease"
	ToolGetHelmReleaseHistory              = "getHelmReleaseHistory"
)

// Access levels for users and teams
const (
	// AccessLevelEnvironmentAdmin represents the environment administrator access level
	AccessLevelEnvironmentAdmin = "environment_administrator"
	// AccessLevelHelpdeskUser represents the helpdesk user access level
	AccessLevelHelpdeskUser = "helpdesk_user"
	// AccessLevelStandardUser represents the standard user access level
	AccessLevelStandardUser = "standard_user"
	// AccessLevelReadonlyUser represents the readonly user access level
	AccessLevelReadonlyUser = "readonly_user"
	// AccessLevelOperatorUser represents the operator user access level
	AccessLevelOperatorUser = "operator_user"
)

// User roles
const (
	// UserRoleAdmin represents an admin user role
	UserRoleAdmin = "admin"
	// UserRoleUser represents a regular user role
	UserRoleUser = "user"
	// UserRoleEdgeAdmin represents an edge admin user role
	UserRoleEdgeAdmin = "edge_admin"
)

// All available access levels
var AllAccessLevels = []string{
	AccessLevelEnvironmentAdmin,
	AccessLevelHelpdeskUser,
	AccessLevelStandardUser,
	AccessLevelReadonlyUser,
	AccessLevelOperatorUser,
}

// All available user roles
var AllUserRoles = []string{
	UserRoleAdmin,
	UserRoleUser,
	UserRoleEdgeAdmin,
}

// isValidAccessLevel checks if a given string is a valid access level
func isValidAccessLevel(access string) bool {
	return slices.Contains(AllAccessLevels, access)
}

// isValidUserRole checks if a given string is a valid user role
func isValidUserRole(role string) bool {
	return slices.Contains(AllUserRoles, role)
}

// isValidHTTPMethod checks if a given string is a valid HTTP method for proxy requests
func isValidHTTPMethod(method string) bool {
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "HEAD", "PATCH"}
	return slices.Contains(validMethods, method)
}

// Registry type constants as used by the Portainer API
const (
	RegistryTypeQuay      = 1 // Quay.io
	RegistryTypeAzure     = 2 // Azure Container Registry
	RegistryTypeCustom    = 3 // Custom registry
	RegistryTypeGitLab    = 4 // GitLab
	RegistryTypeProGet    = 5 // ProGet
	RegistryTypeDockerHub = 6 // DockerHub
	RegistryTypeECR       = 7 // Amazon ECR
)

// Template type constants as used by the Portainer API
const (
	TemplateTypeSwarm      = 1 // Swarm
	TemplateTypeCompose    = 2 // Compose
	TemplateTypeKubernetes = 3 // Kubernetes
)

// Webhook type constants.
const (
	WebhookTypeService   = 1 // Service webhook
	WebhookTypeContainer = 2 // Container webhook
)

// isValidRegistryType checks if a given integer is a valid registry type.
func isValidRegistryType(t int) bool {
	return t >= RegistryTypeQuay && t <= RegistryTypeECR
}

// isValidTemplateType checks if a given integer is a valid custom template type.
func isValidTemplateType(t int) bool {
	return t >= TemplateTypeSwarm && t <= TemplateTypeKubernetes
}

// isValidWebhookType checks if a given integer is a valid webhook type.
func isValidWebhookType(t int) bool {
	return t >= WebhookTypeService && t <= WebhookTypeContainer
}
