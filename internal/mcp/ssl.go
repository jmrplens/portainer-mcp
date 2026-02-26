package mcp

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/portainer/portainer-mcp/pkg/toolgen"
)

// AddSSLFeatures registers SSL-related tools.
func (s *PortainerMCPServer) AddSSLFeatures() {
	s.addToolIfExists(ToolGetSSLSettings, s.HandleGetSSLSettings())

	if !s.readOnly {
		s.addToolIfExists(ToolUpdateSSLSettings, s.HandleUpdateSSLSettings())
	}
}

// HandleGetSSLSettings handles the getSSLSettings tool call.
func (s *PortainerMCPServer) HandleGetSSLSettings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sslSettings, err := s.cli.GetSSLSettings()
		if err != nil {
			return mcp.NewToolResultErrorFromErr("failed to get SSL settings", err), nil
		}

		return jsonResult(sslSettings, "failed to marshal SSL settings")
	}
}

// HandleUpdateSSLSettings handles the updateSSLSettings tool call.
func (s *PortainerMCPServer) HandleUpdateSSLSettings() server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		parser := toolgen.NewParameterParser(request)

		cert, err := parser.GetString("cert", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid cert parameter", err), nil
		}

		key, err := parser.GetString("key", false)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("invalid key parameter", err), nil
		}

		var httpEnabled *bool
		if args := request.GetArguments(); args != nil {
			if val, ok := args["httpEnabled"]; ok && val != nil {
				boolVal, ok := val.(bool)
				if !ok {
					return mcp.NewToolResultErrorFromErr("invalid httpEnabled parameter", fmt.Errorf("httpEnabled must be a boolean")), nil
				}
				httpEnabled = &boolVal
			}
		}

		if cert != "" {
			block, _ := pem.Decode([]byte(cert))
			if block == nil {
				return mcp.NewToolResultErrorFromErr("invalid cert parameter", fmt.Errorf("certificate is not valid PEM format")), nil
			}
			if _, err := x509.ParseCertificate(block.Bytes); err != nil {
				return mcp.NewToolResultErrorFromErr("invalid cert parameter", fmt.Errorf("certificate is not a valid X.509 certificate: %w", err)), nil
			}
		}

		if key != "" {
			block, _ := pem.Decode([]byte(key))
			if block == nil {
				return mcp.NewToolResultErrorFromErr("invalid key parameter", fmt.Errorf("key is not valid PEM format")), nil
			}
		}

		if err := s.cli.UpdateSSLSettings(cert, key, httpEnabled); err != nil {
			return mcp.NewToolResultErrorFromErr("failed to update SSL settings", err), nil
		}

		return mcp.NewToolResultText("SSL settings updated successfully"), nil
	}
}
