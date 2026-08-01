// Package client provides adapter_test.go which tests all passthrough methods
// in adapter.go using a mock http.RoundTripper to intercept HTTP calls at the
// transport layer. This covers the 62 adapter methods that delegate to the
// Swagger-generated client or use httpTransport.Submit directly.
package client

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	httptransport "github.com/go-openapi/runtime/client"
	swaggerclient "github.com/portainer/client-api-go/v2/pkg/client"
	apimodels "github.com/portainer/client-api-go/v2/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockRoundTripper is a mock http.RoundTripper that returns canned responses.
// It captures the last request for optional inspection.
type mockRoundTripper struct {
	statusCode int
	body       string
	err        error
	lastReq    *http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return &http.Response{
		StatusCode: m.statusCode,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(m.body)),
	}, nil
}

// newTestAdapter creates a portainerAPIAdapter backed by a mock HTTP transport.
// The adapter can be used to test all 62 passthrough methods without a real API.
func newTestAdapter(rt http.RoundTripper) *portainerAPIAdapter {
	transport := httptransport.New("localhost", "/api", []string{"http"})
	transport.Transport = rt
	swagger := swaggerclient.New(transport, nil)
	return &portainerAPIAdapter{
		swagger:       swagger,
		httpTransport: transport,
	}
}

// errTransport is a sentinel error for transport-level failures.
var errTransport = fmt.Errorf("transport error")

// ---------------------------------------------------------------------------
// parseHostScheme & newPortainerAPIAdapter – partial coverage improvements
// ---------------------------------------------------------------------------

func TestParseHostScheme(t *testing.T) {
	tests := []struct {
		name       string
		host       string
		wantScheme string
		wantHost   string
	}{
		{"https default", "portainer.example.com", "https", "portainer.example.com"},
		{"explicit http", "http://portainer.local", "http", "portainer.local"},
		{"explicit HTTP uppercase", "HTTP://portainer.local", "http", "portainer.local"},
		{"https explicit", "https://portainer.example.com", "https", "portainer.example.com"},
		{"http with port", "http://192.168.0.40:31017", "http", "192.168.0.40:31017"},
		{"bare host with port", "192.168.0.40:31017", "https", "192.168.0.40:31017"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scheme, cleanHost := parseHostScheme(tc.host)
			assert.Equal(t, tc.wantScheme, scheme)
			assert.Equal(t, tc.wantHost, cleanHost)
		})
	}
}

func TestNewHTTPTransport(t *testing.T) {
	t.Run("skip TLS verify true", func(t *testing.T) {
		tr := newHTTPTransport(true)
		require.NotNil(t, tr.TLSClientConfig)
		assert.True(t, tr.TLSClientConfig.InsecureSkipVerify)
	})
	t.Run("skip TLS verify false", func(t *testing.T) {
		tr := newHTTPTransport(false)
		require.NotNil(t, tr.TLSClientConfig)
		assert.False(t, tr.TLSClientConfig.InsecureSkipVerify)
	})
}

func TestNewPortainerAPIAdapter(t *testing.T) {
	t.Run("https host", func(t *testing.T) {
		a := newPortainerAPIAdapter("portainer.example.com", "test-key", false)
		require.NotNil(t, a)
		assert.NotNil(t, a.swagger)
		assert.NotNil(t, a.httpTransport)
		assert.NotNil(t, a.PortainerClient)
	})
	t.Run("http host", func(t *testing.T) {
		a := newPortainerAPIAdapter("http://portainer.local", "test-key", true)
		require.NotNil(t, a)
		assert.NotNil(t, a.swagger)
	})
}

// ---------------------------------------------------------------------------
// Tag operations
// ---------------------------------------------------------------------------

func TestAdapterDeleteTag(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteTag(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteTag(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete tag")
	})
}

// ---------------------------------------------------------------------------
// Team operations
// ---------------------------------------------------------------------------

func TestAdapterDeleteTeam(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteTeam(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteTeam(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete team")
	})
}

// ---------------------------------------------------------------------------
// User operations
// ---------------------------------------------------------------------------

func TestAdapterDeleteUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteUser(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteUser(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete user")
	})
}

// ---------------------------------------------------------------------------
// Endpoint operations
// ---------------------------------------------------------------------------

func TestAdapterDeleteEndpoint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteEndpoint(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteEndpoint(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete endpoint")
	})
}

func TestAdapterSnapshotEndpoint(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.SnapshotEndpoint(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.SnapshotEndpoint(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to snapshot endpoint")
	})
}

func TestAdapterSnapshotAllEndpoints(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.SnapshotAllEndpoints()
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.SnapshotAllEndpoints()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to snapshot all endpoints")
	})
}

// ---------------------------------------------------------------------------
// Webhook operations
// ---------------------------------------------------------------------------

func TestAdapterListWebhooks(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListWebhooks()
		assert.NoError(t, err)
		require.Len(t, result, 1)
		assert.Equal(t, int64(1), result[0].ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListWebhooks()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to list webhooks")
	})
}

func TestAdapterCreateWebhook(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":42}`})
		id, err := a.CreateWebhook("res-1", 1, 1)
		assert.NoError(t, err)
		assert.Equal(t, int64(42), id)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		id, err := a.CreateWebhook("res-1", 1, 1)
		assert.Error(t, err)
		assert.Equal(t, int64(0), id)
	})
}

func TestAdapterDeleteWebhook(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 202, body: ""})
		err := a.DeleteWebhook(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteWebhook(1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to delete webhook")
	})
}

// ---------------------------------------------------------------------------
// Custom Template operations
// ---------------------------------------------------------------------------

func TestAdapterListCustomTemplates(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListCustomTemplates()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListCustomTemplates()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetCustomTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":5}`})
		result, err := a.GetCustomTemplate(5)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(5), result.ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetCustomTemplate(5)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetCustomTemplateFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"FileContent":"version: '3'"}`})
		content, err := a.GetCustomTemplateFile(1)
		assert.NoError(t, err)
		assert.Equal(t, "version: '3'", content)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		content, err := a.GetCustomTemplateFile(1)
		assert.Error(t, err)
		assert.Empty(t, content)
	})
}

func TestAdapterCreateCustomTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":10}`})
		payload := &apimodels.CustomtemplatesCustomTemplateFromFileContentPayload{}
		result, err := a.CreateCustomTemplate(payload)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(10), result.ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.CreateCustomTemplate(&apimodels.CustomtemplatesCustomTemplateFromFileContentPayload{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterDeleteCustomTemplate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteCustomTemplate(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteCustomTemplate(1)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Registry operations
// ---------------------------------------------------------------------------

func TestAdapterListRegistries(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListRegistries()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListRegistries()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetRegistryByID(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":3}`})
		result, err := a.GetRegistryByID(3)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(3), result.ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetRegistryByID(3)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterCreateRegistry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":7}`})
		id, err := a.CreateRegistry(&apimodels.RegistriesRegistryCreatePayload{})
		assert.NoError(t, err)
		assert.Equal(t, int64(7), id)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		id, err := a.CreateRegistry(&apimodels.RegistriesRegistryCreatePayload{})
		assert.Error(t, err)
		assert.Equal(t, int64(0), id)
	})
}

func TestAdapterUpdateRegistry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		err := a.UpdateRegistry(1, &apimodels.RegistriesRegistryUpdatePayload{})
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.UpdateRegistry(1, &apimodels.RegistriesRegistryUpdatePayload{})
		assert.Error(t, err)
	})
}

func TestAdapterDeleteRegistry(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteRegistry(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteRegistry(1)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Backup operations
// ---------------------------------------------------------------------------

func TestAdapterGetBackupStatus(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetBackupStatus()
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetBackupStatus()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetBackupSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetBackupSettings()
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetBackupSettings()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterCreateBackup(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		err := a.CreateBackup("password")
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.CreateBackup("password")
		assert.Error(t, err)
	})
}

func TestAdapterBackupToS3(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.BackupToS3(&apimodels.BackupS3BackupPayload{})
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.BackupToS3(&apimodels.BackupS3BackupPayload{})
		assert.Error(t, err)
	})
}

func TestAdapterRestoreFromS3(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		err := a.RestoreFromS3(&apimodels.BackupRestoreS3Settings{})
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.RestoreFromS3(&apimodels.BackupRestoreS3Settings{})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Role operations
// ---------------------------------------------------------------------------

func TestAdapterListRoles(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListRoles()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListRoles()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// MOTD (raw HTTP)
// ---------------------------------------------------------------------------

func TestAdapterGetMOTD(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Title":"Hello","Message":"Welcome"}`})
		result, err := a.GetMOTD()
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "Hello", result["Title"])
		assert.Equal(t, "Welcome", result["Message"])
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetMOTD()
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get MOTD")
	})
}

// ---------------------------------------------------------------------------
// Edge Job operations
// ---------------------------------------------------------------------------

func TestAdapterListEdgeJobs(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListEdgeJobs()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListEdgeJobs()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetEdgeJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":5}`})
		result, err := a.GetEdgeJob(5)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(5), result.ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetEdgeJob(5)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetEdgeJobFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"FileContent":"#!/bin/bash"}`})
		content, err := a.GetEdgeJobFile(1)
		assert.NoError(t, err)
		assert.Equal(t, "#!/bin/bash", content)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		content, err := a.GetEdgeJobFile(1)
		assert.Error(t, err)
		assert.Empty(t, content)
	})
}

func TestAdapterCreateEdgeJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":12}`})
		id, err := a.CreateEdgeJob(&apimodels.EdgejobsEdgeJobCreateFromFileContentPayload{})
		assert.NoError(t, err)
		assert.Equal(t, int64(12), id)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		id, err := a.CreateEdgeJob(&apimodels.EdgejobsEdgeJobCreateFromFileContentPayload{})
		assert.Error(t, err)
		assert.Equal(t, int64(0), id)
	})
}

func TestAdapterDeleteEdgeJob(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteEdgeJob(1)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteEdgeJob(1)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// Settings operations
// ---------------------------------------------------------------------------

func TestAdapterUpdateSettingsAdapter(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		err := a.UpdateSettings(&apimodels.SettingsSettingsUpdatePayload{})
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.UpdateSettings(&apimodels.SettingsSettingsUpdatePayload{})
		assert.Error(t, err)
	})
}

func TestAdapterGetPublicSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetPublicSettings()
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetPublicSettings()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// SSL operations
// ---------------------------------------------------------------------------

func TestAdapterGetSSLSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetSSLSettings()
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetSSLSettings()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterUpdateSSLSettings(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.UpdateSSLSettings(&apimodels.SslSslUpdatePayload{})
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.UpdateSSLSettings(&apimodels.SslSslUpdatePayload{})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// App Template operations
// ---------------------------------------------------------------------------

func TestAdapterListAppTemplates(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"templates":[{"Id":1}]}`})
		result, err := a.ListAppTemplates()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListAppTemplates()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetAppTemplateFile(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"FileContent":"services:"}`})
		content, err := a.GetAppTemplateFile(1)
		assert.NoError(t, err)
		assert.Equal(t, "services:", content)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		content, err := a.GetAppTemplateFile(1)
		assert.Error(t, err)
		assert.Empty(t, content)
	})
}

// ---------------------------------------------------------------------------
// Edge Update Schedule operations
// ---------------------------------------------------------------------------

func TestAdapterListEdgeUpdateSchedules(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		result, err := a.ListEdgeUpdateSchedules()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListEdgeUpdateSchedules()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// Auth operations
// ---------------------------------------------------------------------------

func TestAdapterAuthenticateUser(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"jwt":"test-token"}`})
		result, err := a.AuthenticateUser("admin", "password")
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, "test-token", result.Jwt)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.AuthenticateUser("admin", "password")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterLogout(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.Logout()
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.Logout()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to logout")
	})
}

// ---------------------------------------------------------------------------
// Helm operations
// ---------------------------------------------------------------------------

func TestAdapterListHelmRepositories(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.ListHelmRepositories(1)
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListHelmRepositories(1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterCreateHelmRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.CreateHelmRepository(1, "https://charts.example.com")
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.CreateHelmRepository(1, "https://charts.example.com")
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterDeleteHelmRepository(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteHelmRepository(1, 2)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteHelmRepository(1, 2)
		assert.Error(t, err)
	})
}

func TestAdapterSearchHelmCharts(t *testing.T) {
	t.Run("success without chart filter", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `"chart-data"`})
		result, err := a.SearchHelmCharts("https://charts.example.com", nil)
		assert.NoError(t, err)
		assert.Equal(t, "chart-data", result)
	})
	t.Run("success with chart filter", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `"nginx"`})
		chart := "nginx"
		result, err := a.SearchHelmCharts("https://charts.example.com", &chart)
		assert.NoError(t, err)
		assert.Equal(t, "nginx", result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.SearchHelmCharts("https://charts.example.com", nil)
		assert.Error(t, err)
		assert.Empty(t, result)
	})
}

func TestAdapterInstallHelmChart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 201, body: `{"name":"my-release"}`})
		result, err := a.InstallHelmChart(1, &apimodels.HelmInstallChartPayload{})
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.InstallHelmChart(1, &apimodels.HelmInstallChartPayload{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterListHelmReleases(t *testing.T) {
	t.Run("success no filters", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		result, err := a.ListHelmReleases(1, nil, nil, nil)
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("success with filters", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		ns := "default"
		filter := "my-release"
		selector := "app=nginx"
		result, err := a.ListHelmReleases(1, &ns, &filter, &selector)
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListHelmReleases(1, nil, nil, nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterDeleteHelmRelease(t *testing.T) {
	t.Run("success without namespace", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.DeleteHelmRelease(1, "my-release", nil)
		assert.NoError(t, err)
	})
	t.Run("success with namespace", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		ns := "production"
		err := a.DeleteHelmRelease(1, "my-release", &ns)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.DeleteHelmRelease(1, "my-release", nil)
		assert.Error(t, err)
	})
}

func TestAdapterGetHelmReleaseHistory(t *testing.T) {
	t.Run("success without namespace", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		result, err := a.GetHelmReleaseHistory(1, "my-release", nil)
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("success with namespace", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		ns := "default"
		result, err := a.GetHelmReleaseHistory(1, "my-release", &ns)
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetHelmReleaseHistory(1, "my-release", nil)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// Docker Dashboard (raw HTTP)
// ---------------------------------------------------------------------------

func TestAdapterGetDockerDashboard(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		body := `{"containers":{"running":3,"stopped":2,"healthy":1},"images":{"total":10,"size":0},"networks":3,"volumes":2,"stacks":1,"services":0}`
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: body})
		result, err := a.GetDockerDashboard(1)
		assert.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, result.Containers)
		assert.Equal(t, int64(3), result.Containers.Running)
		assert.Equal(t, int64(3), result.Networks)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetDockerDashboard(1)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get docker dashboard")
	})
}

// ---------------------------------------------------------------------------
// Kubernetes Dashboard (raw HTTP)
// ---------------------------------------------------------------------------

func TestAdapterGetKubernetesDashboard(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetKubernetesDashboard(1)
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetKubernetesDashboard(1)
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "failed to get kubernetes dashboard")
	})
}

// ---------------------------------------------------------------------------
// Kubernetes operations (swagger)
// ---------------------------------------------------------------------------

func TestAdapterGetKubernetesNamespaces(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{}]`})
		result, err := a.GetKubernetesNamespaces(1)
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetKubernetesNamespaces(1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterGetKubernetesConfig(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{}`})
		result, err := a.GetKubernetesConfig(1)
		assert.NoError(t, err)
		assert.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.GetKubernetesConfig(1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

// ---------------------------------------------------------------------------
// Stack operations
// ---------------------------------------------------------------------------

func TestAdapterListRegularStacks(t *testing.T) {
	t.Run("success with items", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `[{"Id":1}]`})
		result, err := a.ListRegularStacks()
		assert.NoError(t, err)
		require.Len(t, result, 1)
	})
	t.Run("success no content", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		result, err := a.ListRegularStacks()
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.ListRegularStacks()
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackInspect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackInspect(1)
		assert.NoError(t, err)
		require.NotNil(t, result)
		assert.Equal(t, int64(1), result.ID)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackInspect(1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackDelete(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 204, body: ""})
		err := a.StackDelete(1, 1, false)
		assert.NoError(t, err)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		err := a.StackDelete(1, 1, false)
		assert.Error(t, err)
	})
}

func TestAdapterStackFileInspect(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"StackFileContent":"version: '3'"}`})
		content, err := a.StackFileInspect(1)
		assert.NoError(t, err)
		assert.Equal(t, "version: '3'", content)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		content, err := a.StackFileInspect(1)
		assert.Error(t, err)
		assert.Empty(t, content)
	})
}

func TestAdapterStackUpdateGit(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackUpdateGit(1, 1, &apimodels.StacksStackGitUpdatePayload{})
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackUpdateGit(1, 1, &apimodels.StacksStackGitUpdatePayload{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackGitRedeploy(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackGitRedeploy(1, 1, &apimodels.StacksStackGitRedployPayload{})
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackGitRedeploy(1, 1, &apimodels.StacksStackGitRedployPayload{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackStart(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackStart(1, 1)
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackStart(1, 1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackStop(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackStop(1, 1)
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackStop(1, 1)
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestAdapterStackMigrate(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{statusCode: 200, body: `{"Id":1}`})
		result, err := a.StackMigrate(1, 1, &apimodels.StacksStackMigratePayload{})
		assert.NoError(t, err)
		require.NotNil(t, result)
	})
	t.Run("transport error", func(t *testing.T) {
		a := newTestAdapter(&mockRoundTripper{err: errTransport})
		result, err := a.StackMigrate(1, 1, &apimodels.StacksStackMigratePayload{})
		assert.Error(t, err)
		assert.Nil(t, result)
	})
}
