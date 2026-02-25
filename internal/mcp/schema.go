package mcp

import "slices"

// Tool names as defined in the YAML file
const (
	ToolCreateEnvironmentGroup             = "createEnvironmentGroup"
	ToolListEnvironmentGroups              = "listEnvironmentGroups"
	ToolUpdateEnvironmentGroup             = "updateEnvironmentGroup"
	ToolCreateAccessGroup                  = "createAccessGroup"
	ToolListAccessGroups                   = "listAccessGroups"
	ToolUpdateAccessGroup                  = "updateAccessGroup"
	ToolAddEnvironmentToAccessGroup        = "addEnvironmentToAccessGroup"
	ToolRemoveEnvironmentFromAccessGroup   = "removeEnvironmentFromAccessGroup"
	ToolListEnvironments                   = "listEnvironments"
	ToolGetEnvironment                     = "getEnvironment"
	ToolDeleteEnvironment                  = "deleteEnvironment"
	ToolSnapshotEnvironment                = "snapshotEnvironment"
	ToolSnapshotAllEnvironments            = "snapshotAllEnvironments"
	ToolUpdateEnvironment                  = "updateEnvironment"
	ToolGetStackFile                       = "getStackFile"
	ToolCreateStack                        = "createStack"
	ToolListStacks                         = "listStacks"
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
