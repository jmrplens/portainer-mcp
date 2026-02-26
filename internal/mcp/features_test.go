// Tests for MCP server feature registration (AddXxxFeatures functions) and server options.
// Run: go test ./internal/mcp/ -run "TestAdd.*Features|TestServer" -v
package mcp

import (
"testing"

"github.com/mark3labs/mcp-go/mcp"
"github.com/mark3labs/mcp-go/server"
"github.com/stretchr/testify/assert"
)

// allToolNames returns a map containing every known tool constant as a key,
// each mapped to a minimal mcp.Tool. This allows AddXxxFeatures methods to
// find every tool they try to register via addToolIfExists.
func allToolNames() map[string]mcp.Tool {
names := []string{
ToolCreateEnvironmentGroup, ToolListEnvironmentGroups,
ToolCreateAccessGroup, ToolListAccessGroups,
ToolAddEnvironmentToAccessGroup, ToolRemoveEnvironmentFromAccessGroup,
ToolListEnvironments, ToolGetEnvironment, ToolDeleteEnvironment,
ToolSnapshotEnvironment, ToolSnapshotAllEnvironments,
ToolGetStackFile, ToolCreateStack, ToolListStacks, ToolListRegularStacks,
ToolUpdateStack, ToolGetStack, ToolDeleteStack, ToolInspectStackFile,
ToolUpdateStackGit, ToolRedeployStackGit, ToolStartStack, ToolStopStack, ToolMigrateStack,
ToolCreateEnvironmentTag, ToolDeleteEnvironmentTag, ToolListEnvironmentTags,
ToolCreateTeam, ToolGetTeam, ToolDeleteTeam, ToolListTeams,
ToolUpdateTeamName, ToolUpdateTeamMembers,
ToolListUsers, ToolCreateUser, ToolGetUser, ToolDeleteUser, ToolUpdateUserRole,
ToolGetSettings, ToolUpdateSettings, ToolGetPublicSettings,
ToolGetSSLSettings, ToolUpdateSSLSettings,
ToolListAppTemplates, ToolGetAppTemplateFile,
ToolUpdateAccessGroupName, ToolUpdateAccessGroupUserAccesses, ToolUpdateAccessGroupTeamAccesses,
ToolUpdateEnvironmentTags, ToolUpdateEnvironmentUserAccesses, ToolUpdateEnvironmentTeamAccesses,
ToolUpdateEnvironmentGroupName, ToolUpdateEnvironmentGroupEnvironments, ToolUpdateEnvironmentGroupTags,
ToolDockerProxy, ToolGetDockerDashboard,
ToolKubernetesProxy, ToolKubernetesProxyStripped,
ToolGetKubernetesDashboard, ToolListKubernetesNamespaces, ToolGetKubernetesConfig,
ToolGetSystemStatus,
ToolListCustomTemplates, ToolGetCustomTemplate, ToolGetCustomTemplateFile,
ToolCreateCustomTemplate, ToolDeleteCustomTemplate,
ToolListRegistries, ToolGetRegistry, ToolCreateRegistry, ToolUpdateRegistry, ToolDeleteRegistry,
ToolGetBackupStatus, ToolGetBackupS3Settings, ToolCreateBackup, ToolBackupToS3, ToolRestoreFromS3,
ToolListRoles, ToolGetMOTD,
ToolListWebhooks, ToolCreateWebhook, ToolDeleteWebhook,
ToolListEdgeJobs, ToolGetEdgeJob, ToolGetEdgeJobFile, ToolCreateEdgeJob, ToolDeleteEdgeJob,
ToolListEdgeUpdateSchedules,
ToolAuthenticate, ToolLogout,
ToolListHelmRepositories, ToolAddHelmRepository, ToolRemoveHelmRepository,
ToolSearchHelmCharts, ToolInstallHelmChart, ToolListHelmReleases,
ToolDeleteHelmRelease, ToolGetHelmReleaseHistory,
}

tools := make(map[string]mcp.Tool, len(names))
for _, n := range names {
tools[n] = mcp.Tool{
Name:        n,
Description: "test tool " + n,
InputSchema: mcp.ToolInputSchema{Properties: map[string]any{}},
}
}
return tools
}

// newTestServer creates a PortainerMCPServer with a mock client and all tool
// definitions loaded, suitable for testing AddXxxFeatures methods.
func newTestServer(readOnly bool) *PortainerMCPServer {
return &PortainerMCPServer{
srv: server.NewMCPServer("Test", "0.0.1",
server.WithToolCapabilities(true),
server.WithLogging(),
),
cli:      new(MockPortainerClient),
tools:    allToolNames(),
readOnly: readOnly,
}
}

// TestAddAccessGroupFeatures verifies tool registration for access groups.
func TestAddAccessGroupFeatures(t *testing.T) {
t.Run("read-write mode registers all tools", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddAccessGroupFeatures() })
})
t.Run("read-only mode does not panic", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddAccessGroupFeatures() })
})
}

// TestAddAppTemplateFeatures verifies tool registration for app templates.
func TestAddAppTemplateFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddAppTemplateFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddAppTemplateFeatures() })
})
}

// TestAddAuthFeatures verifies tool registration for authentication.
func TestAddAuthFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddAuthFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddAuthFeatures() })
})
}

// TestAddBackupFeatures verifies tool registration for backup.
func TestAddBackupFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddBackupFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddBackupFeatures() })
})
}

// TestAddCustomTemplateFeatures verifies tool registration for custom templates.
func TestAddCustomTemplateFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddCustomTemplateFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddCustomTemplateFeatures() })
})
}

// TestAddDockerProxyFeatures verifies tool registration for Docker proxy.
func TestAddDockerProxyFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddDockerProxyFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddDockerProxyFeatures() })
})
}

// TestAddEdgeJobFeatures verifies tool registration for edge jobs.
func TestAddEdgeJobFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddEdgeJobFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddEdgeJobFeatures() })
})
}

// TestAddEdgeUpdateScheduleFeatures verifies tool registration for edge update schedules.
func TestAddEdgeUpdateScheduleFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddEdgeUpdateScheduleFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddEdgeUpdateScheduleFeatures() })
})
}

// TestAddEnvironmentFeatures verifies tool registration for environments.
func TestAddEnvironmentFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddEnvironmentFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddEnvironmentFeatures() })
})
}

// TestAddEnvironmentGroupFeatures verifies tool registration for environment groups.
func TestAddEnvironmentGroupFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddEnvironmentGroupFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddEnvironmentGroupFeatures() })
})
}

// TestAddHelmFeatures verifies tool registration for Helm.
func TestAddHelmFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddHelmFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddHelmFeatures() })
})
}

// TestAddKubernetesProxyFeatures verifies tool registration for Kubernetes proxy.
func TestAddKubernetesProxyFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddKubernetesProxyFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddKubernetesProxyFeatures() })
})
}

// TestAddKubernetesNativeFeatures verifies tool registration for Kubernetes native.
func TestAddKubernetesNativeFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddKubernetesNativeFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddKubernetesNativeFeatures() })
})
}

// TestAddMotdFeatures verifies tool registration for MOTD.
func TestAddMotdFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddMotdFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddMotdFeatures() })
})
}

// TestAddRegistryFeatures verifies tool registration for registries.
func TestAddRegistryFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddRegistryFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddRegistryFeatures() })
})
}

// TestAddRoleFeatures verifies tool registration for roles.
func TestAddRoleFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddRoleFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddRoleFeatures() })
})
}

// TestAddSettingsFeatures verifies tool registration for settings.
func TestAddSettingsFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddSettingsFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddSettingsFeatures() })
})
}

// TestAddSSLFeatures verifies tool registration for SSL.
func TestAddSSLFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddSSLFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddSSLFeatures() })
})
}

// TestAddStackFeatures verifies tool registration for stacks.
func TestAddStackFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddStackFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddStackFeatures() })
})
}

// TestAddSystemFeatures verifies tool registration for system.
func TestAddSystemFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddSystemFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddSystemFeatures() })
})
}

// TestAddTagFeatures verifies tool registration for tags.
func TestAddTagFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddTagFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddTagFeatures() })
})
}

// TestAddTeamFeatures verifies tool registration for teams.
func TestAddTeamFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddTeamFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddTeamFeatures() })
})
}

// TestAddUserFeatures verifies tool registration for users.
func TestAddUserFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddUserFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddUserFeatures() })
})
}

// TestAddWebhookFeatures verifies tool registration for webhooks.
func TestAddWebhookFeatures(t *testing.T) {
t.Run("read-write", func(t *testing.T) {
s := newTestServer(false)
assert.NotPanics(t, func() { s.AddWebhookFeatures() })
})
t.Run("read-only", func(t *testing.T) {
s := newTestServer(true)
assert.NotPanics(t, func() { s.AddWebhookFeatures() })
})
}

// TestWithReadOnly verifies the WithReadOnly server option sets readOnly flag.
func TestWithReadOnly(t *testing.T) {
tests := []struct {
name     string
value    bool
expected bool
}{
{"enabled", true, true},
{"disabled", false, false},
}
for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
opts := &serverOptions{}
WithReadOnly(tt.value)(opts)
assert.Equal(t, tt.expected, opts.readOnly)
})
}
}

// TestWithGranularTools verifies the WithGranularTools server option.
func TestWithGranularTools(t *testing.T) {
tests := []struct {
name     string
value    bool
expected bool
}{
{"enabled", true, true},
{"disabled", false, false},
}
for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
opts := &serverOptions{}
WithGranularTools(tt.value)(opts)
assert.Equal(t, tt.expected, opts.granularTools)
})
}
}

// TestWithSkipTLSVerify verifies the WithSkipTLSVerify server option.
func TestWithSkipTLSVerify(t *testing.T) {
tests := []struct {
name     string
value    bool
expected bool
}{
{"enabled", true, true},
{"disabled", false, false},
}
for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
opts := &serverOptions{}
WithSkipTLSVerify(tt.value)(opts)
assert.Equal(t, tt.expected, opts.skipTLSVerify)
})
}
}

// TestNewPortainerMCPServerWithReadOnly verifies that the readOnly option is
// propagated to the server instance.
func TestNewPortainerMCPServerWithReadOnly(t *testing.T) {
mockClient := new(MockPortainerClient)
s, err := NewPortainerMCPServer("https://example.com", "tok",
"testdata/valid_tools.yaml",
WithClient(mockClient),
WithDisableVersionCheck(true),
WithReadOnly(true),
)
assert.NoError(t, err)
assert.True(t, s.readOnly)
}
