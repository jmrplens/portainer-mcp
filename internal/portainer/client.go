package portainer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jmrplens/portainer-mcp/internal/config"
	portainerapi "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
)

// DefaultCallTimeout is the deadline callers should apply to an ordinary API
// call. It is deliberately NOT set as http.Client.Timeout: that field is an
// absolute ceiling which a per-call context cannot raise, and this server must
// support operations — a stack redeploy that pulls multi-gigabyte images, a
// backup — that legitimately run far longer. Deadlines belong on the context
// each call carries, where the caller can choose them.
const DefaultCallTimeout = 60 * time.Second

// Client is the project's entry point to the Portainer API.
type Client struct {
	API        *portainerapi.ClientWithResponses
	baseURL    string
	token      string
	httpClient *http.Client
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
	httpClient := &http.Client{Transport: transport}

	// Trim defensively: New does not require the caller to have run Validate,
	// and a trailing slash would produce host//api/... which the generated
	// client does not collapse. Against a ServeMux that 301-redirects, and Go
	// converts a redirected POST or PUT into a GET with no body.
	baseURL := strings.TrimRight(cfg.URL, "/") + "/api"

	token := cfg.Token
	api, err := portainerapi.NewClientWithResponses(
		baseURL,
		portainerapi.WithHTTPClient(httpClient),
		portainerapi.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-API-Key", token)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("portainer: build client: %w", err)
	}
	return &Client{API: api, baseURL: baseURL, token: token, httpClient: httpClient}, nil
}

// Get issues a raw authenticated GET against a path below the API root. It
// exists for the handful of callers that run before the typed client is
// useful — edition detection, health checks — and for endpoints outside the
// spec.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", path, err)
	}
	req.Header.Set("X-API-Key", c.token)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get %s: %w", path, err)
	}
	return resp, nil
}
