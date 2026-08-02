package harness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// AdminUsername and AdminPassword are the credentials every ephemeral estate
// is provisioned with. They are fixed rather than generated because a fixed
// value can be printed in a failure message without leaking anything: the
// estate is disposable and unreachable from outside the test network.
const (
	AdminUsername = "admin"
	AdminPassword = "e2e-Passw0rd-2026!"
)

// Credentials are what a provisioned Portainer will accept.
type Credentials struct {
	Username string
	Password string
	APIKey   string
	JWT      string
}

// EndpointSpec describes one environment to register.
//
// CreationType follows Portainer's own enumeration: 1 local Docker, 2 agent,
// 3 Azure, 4 edge agent, 5 local Kubernetes.
type EndpointSpec struct {
	Name                string
	CreationType        int
	URL                 string
	TLS                 bool
	TLSSkipVerify       bool
	TLSSkipClientVerify bool
}

// Provision takes a freshly started Portainer from empty to usable.
//
// setupToken is empty when the server was started with --no-setup-token, and
// carries the 64-hex token printed at startup otherwise. Portainer 2.44.0
// refuses to create the first administrator without it, which the design
// specification does not mention.
func Provision(ctx context.Context, c *http.Client, baseURL, setupToken string) (Credentials, error) {
	creds := Credentials{Username: AdminUsername, Password: AdminPassword}

	initBody := map[string]string{"Username": creds.Username, "Password": creds.Password}
	headers := map[string]string{}
	if setupToken != "" {
		headers["X-Setup-Token"] = setupToken
	}
	if err := postJSON(ctx, c, baseURL+"/api/users/admin/init", initBody, headers, nil); err != nil {
		return Credentials{}, fmt.Errorf("initialise administrator: %w", err)
	}

	var auth struct {
		JWT string `json:"jwt"`
	}
	if err := postJSON(ctx, c, baseURL+"/api/auth", initBody, nil, &auth); err != nil {
		return Credentials{}, fmt.Errorf("authenticate: %w", err)
	}
	if auth.JWT == "" {
		return Credentials{}, fmt.Errorf("authenticate: response carried no jwt")
	}
	creds.JWT = auth.JWT

	// The raw key is returned exactly once and never again. Losing it means
	// re-provisioning the estate.
	var token struct {
		RawAPIKey string `json:"rawAPIKey"`
	}
	tokenBody := map[string]string{"description": "e2e", "password": creds.Password}
	bearer := map[string]string{"Authorization": "Bearer " + creds.JWT}
	if err := postJSON(ctx, c, baseURL+"/api/users/1/tokens", tokenBody, bearer, &token); err != nil {
		return Credentials{}, fmt.Errorf("create api key: %w", err)
	}
	if token.RawAPIKey == "" {
		return Credentials{}, fmt.Errorf("create api key: response carried no rawAPIKey")
	}
	creds.APIKey = token.RawAPIKey

	return creds, nil
}

// CreateEndpoint registers one environment and returns its identifier.
//
// The request is multipart/form-data; Portainer rejects JSON here. Optional
// fields are omitted rather than sent empty, because a blank URL is not the
// same as an absent one for a local endpoint.
func CreateEndpoint(ctx context.Context, c *http.Client, baseURL, apiKey string, e EndpointSpec) (int, error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)

	fields := [][2]string{
		{"Name", e.Name},
		{"EndpointCreationType", strconv.Itoa(e.CreationType)},
	}
	if e.URL != "" {
		fields = append(fields, [2]string{"URL", e.URL})
	}
	// The agent always serves TLS with a certificate valid only for localhost,
	// so both skip flags are needed together. With TLSSkipVerify alone the
	// server answers 400 "Invalid certificate file", which names neither.
	if e.TLS {
		fields = append(fields, [2]string{"TLS", "true"})
	}
	if e.TLSSkipVerify {
		fields = append(fields, [2]string{"TLSSkipVerify", "true"})
	}
	if e.TLSSkipClientVerify {
		fields = append(fields, [2]string{"TLSSkipClientVerify", "true"})
	}
	for _, f := range fields {
		if err := form.WriteField(f[0], f[1]); err != nil {
			return 0, fmt.Errorf("write field %s: %w", f[0], err)
		}
	}
	if err := form.Close(); err != nil {
		return 0, fmt.Errorf("close multipart form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/endpoints", &buf)
	if err != nil {
		return 0, fmt.Errorf("build endpoint request: %w", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-API-Key", apiKey)

	var created struct {
		ID int `json:"Id"`
	}
	if err := do(c, req, &created); err != nil {
		return 0, fmt.Errorf("create endpoint %q: %w", e.Name, err)
	}
	return created.ID, nil
}

// ApplyLicence installs a Business Edition licence.
//
// The key never appears in the returned error. It comes from a gitignored .env
// and a CI log is the one place it must not reach.
func ApplyLicence(ctx context.Context, c *http.Client, baseURL, jwt, key string) error {
	body := map[string]string{"key": key}
	bearer := map[string]string{"Authorization": "Bearer " + jwt}
	if err := postJSON(ctx, c, baseURL+"/api/licenses/add", body, bearer, nil); err != nil {
		return fmt.Errorf("apply licence: %w", err)
	}
	return nil
}

// LicenceNodes reports the node allowance of the installed licence.
func LicenceNodes(ctx context.Context, c *http.Client, baseURL, jwt string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/api/licenses", nil)
	if err != nil {
		return 0, fmt.Errorf("build licence request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+jwt)

	var licences []struct {
		Nodes int `json:"nodes"`
	}
	if err := do(c, req, &licences); err != nil {
		return 0, fmt.Errorf("read licences: %w", err)
	}
	if len(licences) == 0 {
		return 0, fmt.Errorf("read licences: none installed")
	}
	return licences[0].Nodes, nil
}

func postJSON(ctx context.Context, c *http.Client, url string, body any, headers map[string]string, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return do(c, req, out)
}

// maxErrorBody bounds what an error message quotes back from the server, so a
// server that answers with a megabyte of HTML cannot produce an unreadable
// failure.
const maxErrorBody = 512

func do(c *http.Client, req *http.Request, out any) error {
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
		// A 403 from this specific endpoint means exactly one thing: the
		// server demands a setup token and did not get one. That is true
		// regardless of what the server's own message happens to say, so
		// the detection keys on the request, not on sniffing the response
		// body for wording no version of Portainer is contractually bound
		// to use. Naming the remedy here saves the next reader a trip
		// through Portainer's source.
		if resp.StatusCode == http.StatusForbidden && strings.HasSuffix(req.URL.Path, "/users/admin/init") {
			return fmt.Errorf("%s %s: http 403: this server requires a setup token; "+
				"start it with --no-setup-token or pass the token from its startup logs",
				req.Method, req.URL.Path)
		}
		return fmt.Errorf("%s %s: http %d: %s", req.Method, req.URL.Path, resp.StatusCode, bytes.TrimSpace(snippet))
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s %s: decode response: %w", req.Method, req.URL.Path, err)
	}
	return nil
}
