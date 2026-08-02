package portainer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/jmrplens/portainer-mcp/internal/config"
	portainerapi "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
)

// DefaultTimeout bounds a single API call. Long operations such as a stack
// redeploy that pulls multi-gigabyte images need more, and set their own
// deadline on the context they pass.
const DefaultTimeout = 60 * time.Second

// Client is the project's entry point to the Portainer API.
type Client struct {
	API *portainerapi.ClientWithResponses
}

// New builds a client for the given configuration.
func New(cfg *config.Config) (*Client, error) {
	if cfg == nil || cfg.URL == "" {
		return nil, errors.New("portainer: a server URL is required")
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.SkipTLSVerify {
		// Deliberate: self-signed certificates are common on self-hosted
		// Portainer, and the operator opts in explicitly.
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12} //nolint:gosec // opt-in via PORTAINER_SKIP_TLS_VERIFY
	}
	httpClient := &http.Client{Transport: transport, Timeout: DefaultTimeout}

	token := cfg.Token
	api, err := portainerapi.NewClientWithResponses(
		cfg.URL+"/api",
		portainerapi.WithHTTPClient(httpClient),
		portainerapi.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-API-Key", token)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("portainer: build client: %w", err)
	}
	return &Client{API: api}, nil
}
