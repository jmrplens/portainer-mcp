package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	sdkclient "github.com/portainer/client-api-go/v2/client"
	swaggerclient "github.com/portainer/client-api-go/v2/pkg/client"
)

const (
	// defaultHTTPTimeout is the default timeout for HTTP requests to the Portainer API.
	defaultHTTPTimeout = 30 * time.Second
)

// portainerAPIAdapter wraps the SDK PortainerClient and adds methods
// that are available in the Swagger-generated client but not exposed
// by the SDK's high-level client (e.g., delete operations).
type portainerAPIAdapter struct {
	*sdkclient.PortainerClient
	swagger       *swaggerclient.PortainerClientAPI
	httpTransport *httptransport.Runtime
	scheme        string
	cleanHost     string
	apiKey        string
	proxyClient   *http.Client
}

// newHTTPTransport creates a configured http.Transport with TLS settings.
func newHTTPTransport(skipTLSVerify bool) *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLSVerify,
		},
	}
}

// parseHostScheme extracts the scheme and clean host from a URL or host string.
// The clean host has any scheme prefix removed, suitable for go-openapi transports.
// Returns "http" if the host starts with "http://", otherwise defaults to "https".
func parseHostScheme(host string) (scheme, cleanHost string) {
	lower := strings.ToLower(host)
	if strings.HasPrefix(lower, "http://") {
		return "http", host[len("http://"):]
	}
	if strings.HasPrefix(lower, "https://") {
		return "https", host[len("https://"):]
	}
	return "https", host
}

// newPortainerAPIAdapter creates a new adapter that embeds the SDK high-level
// client and also holds a reference to the low-level Swagger client for
// operations not exposed by the SDK.
func newPortainerAPIAdapter(host, apiKey string, skipTLSVerify bool) *portainerAPIAdapter {
	scheme, cleanHost := parseHostScheme(host)
	sdkCli := sdkclient.NewPortainerClient(cleanHost, apiKey,
		sdkclient.WithSkipTLSVerify(skipTLSVerify),
		sdkclient.WithScheme(scheme),
	)

	httpClient := &http.Client{
		Timeout:   defaultHTTPTimeout,
		Transport: newHTTPTransport(skipTLSVerify),
	}
	transport := httptransport.NewWithClient(cleanHost, "/api", []string{scheme}, httpClient)
	apiKeyAuth := runtime.ClientAuthInfoWriterFunc(func(r runtime.ClientRequest, _ strfmt.Registry) error {
		return r.SetHeaderParam("x-api-key", apiKey)
	})
	transport.DefaultAuthentication = apiKeyAuth

	return &portainerAPIAdapter{
		PortainerClient: sdkCli,
		swagger:         swaggerclient.New(transport, nil),
		httpTransport:   transport,
		scheme:          scheme,
		cleanHost:       cleanHost,
		apiKey:          apiKey,
		proxyClient:     httpClient,
	}
}

// ProxyDockerRequest overrides the SDK method to use the correct scheme
// instead of the hardcoded "https://" in the upstream SDK.
func (a *portainerAPIAdapter) ProxyDockerRequest(environmentId int, opts sdkclient.ProxyRequestOptions) (*http.Response, error) {
	baseURL := fmt.Sprintf("%s://%s/api/endpoints/%d/docker%s", a.scheme, a.cleanHost, environmentId, opts.APIPath)
	return a.proxyRequest(baseURL, opts)
}

// ProxyKubernetesRequest overrides the SDK method to use the correct scheme
// instead of the hardcoded "https://" in the upstream SDK.
func (a *portainerAPIAdapter) ProxyKubernetesRequest(environmentId int, opts sdkclient.ProxyRequestOptions) (*http.Response, error) {
	baseURL := fmt.Sprintf("%s://%s/api/endpoints/%d/kubernetes%s", a.scheme, a.cleanHost, environmentId, opts.APIPath)
	return a.proxyRequest(baseURL, opts)
}

func (a *portainerAPIAdapter) proxyRequest(baseURL string, opts sdkclient.ProxyRequestOptions) (*http.Response, error) {
	req, err := http.NewRequest(opts.Method, baseURL, opts.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to create proxy request: %w", err)
	}
	if opts.QueryParams != nil {
		q := req.URL.Query()
		for k, v := range opts.QueryParams {
			q.Set(k, v)
		}
		req.URL.RawQuery = q.Encode()
	}
	req.Header.Set("x-api-key", a.apiKey)
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}
	resp, err := a.proxyClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send proxy request: %w", err)
	}
	return resp, nil
}
