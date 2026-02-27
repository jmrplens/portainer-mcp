package mcp

import (
"context"
"crypto/ecdsa"
"crypto/elliptic"
"crypto/rand"
"crypto/x509"
"crypto/x509/pkix"
"encoding/json"
"encoding/pem"
"math/big"
"testing"
"time"

"github.com/mark3labs/mcp-go/mcp"
"github.com/mark3labs/mcp-go/server"
"github.com/portainer/portainer-mcp/pkg/portainer/models"
"github.com/stretchr/testify/assert"
)

// generateTestCertAndKey creates a self-signed certificate and private key in PEM format for testing.
func generateTestCertAndKey(t *testing.T) (string, string) {
t.Helper()

privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
assert.NoError(t, err)

template := &x509.Certificate{
SerialNumber: big.NewInt(1),
Subject:      pkix.Name{CommonName: "test"},
NotBefore:    time.Now(),
NotAfter:     time.Now().Add(24 * time.Hour),
}

certDER, err := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
assert.NoError(t, err)

certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

keyDER, err := x509.MarshalECPrivateKey(privKey)
assert.NoError(t, err)
keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

return string(certPEM), string(keyPEM)
}

// TestHandleGetSSLSettings verifies the HandleGetSSLSettings MCP tool handler.
func TestHandleGetSSLSettings(t *testing.T) {
tests := []struct {
name          string
sslSettings   models.SSLSettings
mockError     error
expectError   bool
errorContains string
}{
{
name: "successful SSL settings retrieval",
sslSettings: models.SSLSettings{
CertPath:    "/certs/cert.pem",
KeyPath:     "/certs/key.pem",
HTTPEnabled: true,
SelfSigned:  false,
},
mockError:   nil,
expectError: false,
},
{
name:          "client error",
sslSettings:   models.SSLSettings{},
mockError:     assert.AnError,
expectError:   true,
errorContains: "failed to get SSL settings",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := new(MockPortainerClient)
mockClient.On("GetSSLSettings").Return(tt.sslSettings, tt.mockError)

srv := &PortainerMCPServer{
srv:   server.NewMCPServer("Test Server", "1.0.0"),
cli:   mockClient,
tools: make(map[string]mcp.Tool),
}

handler := srv.HandleGetSSLSettings()
result, err := handler(context.Background(), mcp.CallToolRequest{})

if tt.expectError {
assert.NoError(t, err)
assert.NotNil(t, result)
assert.True(t, result.IsError)
textContent, ok := result.Content[0].(mcp.TextContent)
assert.True(t, ok)
assert.Contains(t, textContent.Text, tt.errorContains)
} else {
assert.NoError(t, err)
assert.NotNil(t, result)
textContent, ok := result.Content[0].(mcp.TextContent)
assert.True(t, ok)

var settings models.SSLSettings
err = json.Unmarshal([]byte(textContent.Text), &settings)
assert.NoError(t, err)
assert.Equal(t, tt.sslSettings, settings)
}

mockClient.AssertExpectations(t)
})
}
}

// TestHandleUpdateSSLSettings verifies the HandleUpdateSSLSettings MCP tool handler.
func TestHandleUpdateSSLSettings(t *testing.T) {
httpEnabled := true
testCert, testKey := generateTestCertAndKey(t)

tests := []struct {
name          string
request       mcp.CallToolRequest
setupMock     func(*MockPortainerClient)
expectError   bool
errorContains string
}{
{
name: "successful SSL settings update with all params",
request: mcp.CallToolRequest{
Params: mcp.CallToolParams{
Arguments: map[string]any{
"cert":        testCert,
"key":         testKey,
"httpEnabled": true,
},
},
},
setupMock: func(m *MockPortainerClient) {
m.On("UpdateSSLSettings", testCert, testKey, &httpEnabled).Return(nil)
},
expectError: false,
},
{
name: "successful SSL settings update with cert and key only",
request: mcp.CallToolRequest{
Params: mcp.CallToolParams{
Arguments: map[string]any{
"cert": testCert,
"key":  testKey,
},
},
},
setupMock: func(m *MockPortainerClient) {
m.On("UpdateSSLSettings", testCert, testKey, (*bool)(nil)).Return(nil)
},
expectError: false,
},
{
name: "client error",
request: mcp.CallToolRequest{
Params: mcp.CallToolParams{
Arguments: map[string]any{
"cert": testCert,
"key":  testKey,
},
},
},
setupMock: func(m *MockPortainerClient) {
m.On("UpdateSSLSettings", testCert, testKey, (*bool)(nil)).Return(assert.AnError)
},
expectError:   true,
errorContains: "failed to update SSL settings",
},
{
name: "invalid cert PEM format",
request: mcp.CallToolRequest{
Params: mcp.CallToolParams{
Arguments: map[string]any{
"cert": "not-valid-pem",
"key":  testKey,
},
},
},
setupMock:     func(m *MockPortainerClient) {},
expectError:   true,
errorContains: "invalid cert parameter",
},
{
name: "invalid key PEM format",
request: mcp.CallToolRequest{
Params: mcp.CallToolParams{
Arguments: map[string]any{
"cert": testCert,
"key":  "not-valid-pem",
},
},
},
setupMock:     func(m *MockPortainerClient) {},
expectError:   true,
errorContains: "invalid key parameter",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
mockClient := new(MockPortainerClient)
tt.setupMock(mockClient)

srv := &PortainerMCPServer{
srv:   server.NewMCPServer("Test Server", "1.0.0"),
cli:   mockClient,
tools: make(map[string]mcp.Tool),
}

handler := srv.HandleUpdateSSLSettings()
result, err := handler(context.Background(), tt.request)

if tt.expectError {
assert.NoError(t, err)
assert.NotNil(t, result)
assert.True(t, result.IsError)
textContent, ok := result.Content[0].(mcp.TextContent)
assert.True(t, ok)
assert.Contains(t, textContent.Text, tt.errorContains)
} else {
assert.NoError(t, err)
assert.NotNil(t, result)
textContent, ok := result.Content[0].(mcp.TextContent)
assert.True(t, ok)
assert.Contains(t, textContent.Text, "SSL settings updated successfully")
}

mockClient.AssertExpectations(t)
})
}
}
