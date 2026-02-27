package models

import (
	"testing"

	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
)

// TestConvertFunctionsNilInput verifies that all conversion functions
// handle nil input gracefully (return zero-value) instead of panicking.
func TestConvertFunctionsNilInput(t *testing.T) {
	t.Run("ConvertEndpointGroupToAccessGroup", func(t *testing.T) {
		result := ConvertEndpointGroupToAccessGroup(nil, nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToAppTemplate", func(t *testing.T) {
		result := ConvertToAppTemplate(nil)
		if result.Title != "" {
			t.Error("expected empty Title")
		}
	})

	t.Run("ConvertToAppTemplates_nil_slice", func(t *testing.T) {
		result := ConvertToAppTemplates(nil)
		if len(result) != 0 {
			t.Error("expected empty result")
		}
	})

	t.Run("ConvertToBackupStatus", func(t *testing.T) {
		result := ConvertToBackupStatus(nil)
		if result.Failed {
			t.Error("expected Failed to be false")
		}
	})

	t.Run("ConvertToS3BackupSettings", func(t *testing.T) {
		result := ConvertToS3BackupSettings(nil)
		if result.BucketName != "" {
			t.Error("expected empty BucketName")
		}
	})

	t.Run("ConvertCustomTemplateToLocal", func(t *testing.T) {
		result := ConvertCustomTemplateToLocal(nil)
		if result.Title != "" {
			t.Error("expected empty Title")
		}
	})

	t.Run("ConvertDockerDashboardResponse", func(t *testing.T) {
		result := ConvertDockerDashboardResponse(nil)
		if result.Networks != 0 {
			t.Error("expected zero Networks")
		}
	})

	t.Run("ConvertEdgeJobToLocal", func(t *testing.T) {
		result := ConvertEdgeJobToLocal(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertEdgeUpdateScheduleToLocal", func(t *testing.T) {
		result := ConvertEdgeUpdateScheduleToLocal(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertEndpointToEnvironment", func(t *testing.T) {
		result := ConvertEndpointToEnvironment(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertEdgeGroupToGroup", func(t *testing.T) {
		result := ConvertEdgeGroupToGroup(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToHelmRepository", func(t *testing.T) {
		result := ConvertToHelmRepository(nil)
		if result.URL != "" {
			t.Error("expected empty URL")
		}
	})

	t.Run("ConvertToHelmRepositoryList", func(t *testing.T) {
		result := ConvertToHelmRepositoryList(nil)
		if len(result.UserRepositories) != 0 {
			t.Error("expected empty UserRepositories")
		}
	})

	t.Run("ConvertToHelmRelease", func(t *testing.T) {
		result := ConvertToHelmRelease(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToHelmReleaseDetails", func(t *testing.T) {
		result := ConvertToHelmReleaseDetails(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertK8sDashboard", func(t *testing.T) {
		result := ConvertK8sDashboard(nil)
		if result.NamespacesCount != 0 {
			t.Error("expected zero NamespacesCount")
		}
	})

	t.Run("ConvertK8sNamespace", func(t *testing.T) {
		result := ConvertK8sNamespace(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertRawRegistryToRegistry", func(t *testing.T) {
		result := ConvertRawRegistryToRegistry(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToRole", func(t *testing.T) {
		result := ConvertToRole(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertSettingsToPortainerSettings", func(t *testing.T) {
		result := ConvertSettingsToPortainerSettings(nil)
		if result.Authentication.Method != "" {
			t.Error("expected empty Authentication.Method")
		}
	})

	t.Run("ConvertToPublicSettings", func(t *testing.T) {
		result := ConvertToPublicSettings(nil)
		if result.AuthenticationMethod != "" {
			t.Error("expected empty AuthenticationMethod")
		}
	})

	t.Run("ConvertToSSLSettings", func(t *testing.T) {
		result := ConvertToSSLSettings(nil)
		if result.CertPath != "" {
			t.Error("expected empty CertPath")
		}
	})

	t.Run("ConvertEdgeStackToStack", func(t *testing.T) {
		result := ConvertEdgeStackToStack(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertRegularStack", func(t *testing.T) {
		result := ConvertRegularStack(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToSystemStatus", func(t *testing.T) {
		result := ConvertToSystemStatus(nil)
		if result.Version != "" {
			t.Error("expected empty Version")
		}
	})

	t.Run("ConvertTagToEnvironmentTag", func(t *testing.T) {
		result := ConvertTagToEnvironmentTag(nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToTeam", func(t *testing.T) {
		result := ConvertToTeam(nil, nil)
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToUser", func(t *testing.T) {
		result := ConvertToUser(nil)
		if result.Username != "" {
			t.Error("expected empty Username")
		}
	})

	t.Run("ConvertToWebhook", func(t *testing.T) {
		result := ConvertToWebhook(nil)
		if result.Token != "" {
			t.Error("expected empty Token")
		}
	})

	t.Run("ConvertToMOTDFromMap", func(t *testing.T) {
		result := ConvertToMOTDFromMap(nil)
		if result.Title != "" {
			t.Error("expected empty Title")
		}
	})

	// Verify nil elements within slices are handled
	t.Run("ConvertToAppTemplates_with_nil_element", func(t *testing.T) {
		result := ConvertToAppTemplates([]*apimodels.PortainerTemplate{nil})
		if len(result) != 1 {
			t.Error("expected 1 element")
		}
	})

	t.Run("ConvertEndpointGroupToAccessGroup_with_nil_endpoints", func(t *testing.T) {
		group := &apimodels.PortainerEndpointGroup{}
		result := ConvertEndpointGroupToAccessGroup(group, []*apimodels.PortainereeEndpoint{nil})
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})

	t.Run("ConvertToTeam_with_nil_memberships", func(t *testing.T) {
		team := &apimodels.PortainerTeam{}
		result := ConvertToTeam(team, []*apimodels.PortainerTeamMembership{nil})
		if result.Name != "" {
			t.Error("expected empty Name")
		}
	})
}
