package harness

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

// AdminUsername is the administrator username every ephemeral estate is
// provisioned with. It names no secret, so it stays fixed: reaching the
// estate still requires the password Provision mints fresh every run (see
// generateAdminPassword), which is never a constant and never logged.
const AdminUsername = "admin"

// adminPasswordBytes is how much raw entropy generateAdminPassword reads from
// crypto/rand for one provisioning run: 24 bytes is 192 bits, comfortably
// beyond anything guessable, encoded below into a password Portainer's own
// JSON body and this harness's HTTP calls can carry without escaping.
const adminPasswordBytes = 24

// generateAdminPassword mints a fresh, high-entropy administrator password
// for the estate Provision is about to bring up.
//
// crypto/rand, not math/rand: this is a credential a real Portainer instance
// will accept as administrator, and math/rand's output is predictable from
// its seed — exactly the property a generated-rather-than-guessable password
// exists to not have. Base64 URL-safe encoding avoids characters (notably
// '/' and '+') that would need escaping wherever this value travels — a JSON
// body, a compose env file, a shell command — none of which this function
// controls once the password leaves it.
func generateAdminPassword() (string, error) {
	raw := make([]byte, adminPasswordBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate administrator password: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

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

// agentPort is the port the Portainer agent serves on. It is not configurable
// in the compose stack, so it is a constant rather than a parameter.
const agentPort = 9001

// AgentEndpoint describes an agent environment reachable at host on the
// compose network.
//
// Both skip flags are set together and neither is optional. The agent serves
// TLS with a self-signed certificate whose subject is "localhost", so a server
// connecting by container name cannot verify it, and Portainer refuses to
// register an agent environment whose certificate it can neither verify nor be
// told to ignore.
func AgentEndpoint(name, host string) EndpointSpec {
	return EndpointSpec{
		Name:                name,
		CreationType:        2,
		URL:                 fmt.Sprintf("tcp://%s:%d", host, agentPort),
		TLS:                 true,
		TLSSkipVerify:       true,
		TLSSkipClientVerify: true,
	}
}

// Provision takes a freshly started Portainer from empty to usable.
//
// setupToken is empty when the server was started with --no-setup-token, and
// carries the 64-hex token printed at startup otherwise. Portainer 2.44.0
// refuses to create the first administrator without it, which the design
// specification does not mention.
func Provision(ctx context.Context, c *http.Client, baseURL, setupToken string) (Credentials, error) {
	password, err := generateAdminPassword()
	if err != nil {
		return Credentials{}, fmt.Errorf("provision: %w", err)
	}
	creds := Credentials{Username: AdminUsername, Password: password}

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
	// The vendored spec documents lowercase "username"/"password" and lists
	// apiKey as the required credential; live Portainer 2.44.0 disagrees with
	// its own spec and accepts the capitalised field names reused from init
	// above, because Go's JSON decoding on the server side is case-insensitive
	// on struct field matches. Verified against a live instance: this call
	// returns a 296-character JWT. Reusing initBody here is deliberate, not
	// an oversight of the spec's casing.
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
	// User ID 1 is not assumed, it is guaranteed: this is the first user
	// created in an empty database by admin/init immediately above, and
	// Portainer assigns ids sequentially from 1. Verified against a live
	// instance: this call succeeds immediately after init.
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

// EdgeCredentials identifies an edge environment to whatever agent enrols
// against it.
//
// EndpointID and EdgeID are not the same thing and are easy to conflate: the
// server's response carries both, they look interchangeable (a bare
// identifier each), and only one of them is what an edge agent's EDGE_ID
// environment variable means. EndpointID is Portainer's ordinary numeric
// database id for the environment — the one every other endpoint has, used
// in URLs like /api/endpoints/{id}. EdgeID is a UUID minted specifically for
// this environment's edge identity. Passing EndpointID as EDGE_ID to an
// agent fails distinctively, not silently: the agent polls, and the server
// answers "Permission denied to access environment. The device has not been
// trusted yet: Unauthorized Edge endpoint operation: invalid Edge
// identifier" — a real, first-run measurement, not a hypothetical. Key is a
// shared secret: whoever holds it can enrol as that environment, so it is
// handled with the same care as an API key.
type EdgeCredentials struct {
	EndpointID int
	EdgeID     string
	Key        string
}

// CreateEdgeEndpoint registers an edge environment and returns what an edge
// agent needs to enrol against it.
//
// It shares CreateEndpoint's request shape (the same multipart POST to
// /api/endpoints, EndpointCreationType 4) but is a separate function rather
// than a fourth CreateEndpoint case: only an edge environment's response
// carries an edge id and key, and threading those back through
// EndpointSpec — which every other creation type ignores — would make the
// common path answer for fields it never uses. Unlike an agent or local
// environment, no URL is sent: the server derives one from its own Edge
// Compute settings.
func CreateEdgeEndpoint(ctx context.Context, c *http.Client, baseURL, apiKey, name string) (EdgeCredentials, error) {
	var buf bytes.Buffer
	form := multipart.NewWriter(&buf)
	if err := form.WriteField("Name", name); err != nil {
		return EdgeCredentials{}, fmt.Errorf("write field Name: %w", err)
	}
	if err := form.WriteField("EndpointCreationType", "4"); err != nil {
		return EdgeCredentials{}, fmt.Errorf("write field EndpointCreationType: %w", err)
	}
	if err := form.Close(); err != nil {
		return EdgeCredentials{}, fmt.Errorf("close multipart form: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/endpoints", &buf)
	if err != nil {
		return EdgeCredentials{}, fmt.Errorf("build edge endpoint request: %w", err)
	}
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-API-Key", apiKey)

	var created struct {
		ID      int    `json:"Id"`
		EdgeID  string `json:"EdgeID"`
		EdgeKey string `json:"EdgeKey"`
	}
	if err := do(c, req, &created); err != nil {
		return EdgeCredentials{}, fmt.Errorf("create edge endpoint %q: %w", name, err)
	}
	if created.EdgeKey == "" {
		return EdgeCredentials{}, fmt.Errorf("create edge endpoint %q: response carried no EdgeKey", name)
	}
	if created.EdgeID == "" {
		return EdgeCredentials{}, fmt.Errorf("create edge endpoint %q: response carried no EdgeID", name)
	}
	return EdgeCredentials{EndpointID: created.ID, EdgeID: created.EdgeID, Key: created.EdgeKey}, nil
}

// EnableEdgeCompute turns on Edge Compute and tells the server how an edge
// agent should reach it. portainerURL is the address an agent polls for
// commands; tunnelAddr is where it opens its reverse tunnel. Both are
// addresses on the estate's own compose network — an edge agent enrolled by
// this harness never reaches the server through the port published to the
// host.
//
// Without this, endpoint creation for an edge environment fails outright:
// Portainer answers "API server URL not set in Edge Compute settings" rather
// than registering anything. Verified against a live estate.
func EnableEdgeCompute(ctx context.Context, c *http.Client, baseURL, apiKey, portainerURL, tunnelAddr string) error {
	body := map[string]any{
		"EnableEdgeComputeFeatures": true,
		"EdgePortainerUrl":          portainerURL,
		"Edge":                      map[string]string{"TunnelServerAddress": tunnelAddr},
	}
	headers := map[string]string{"X-API-Key": apiKey}
	if err := putJSON(ctx, c, baseURL+"/api/settings", body, headers, nil); err != nil {
		return fmt.Errorf("enable edge compute: %w", err)
	}
	return nil
}

// ApplyLicence installs a Business Edition licence and reports any
// conflicting keys the server names.
//
// Neither our own error text nor anything quoted back from the server may
// carry the key. It comes from a gitignored .env, it is registered to a
// personal email, and a CI log is the one place it must never reach — so the
// server's response is scrubbed before it is wrapped, on the assumption that a
// validation message may echo what it was sent.
//
// A clean attach answers {"conflictingKeys":null}. A non-null value is not a
// failure — this call still succeeds and returns it, never an error — but it
// is the first observable sign that Portainer's own licensing service tracks
// activations across ephemeral runs, which the local counters this harness
// can otherwise read do not show (see plan/carry-forward.md). The caller logs
// it prominently and carries it into the estate; failing here would only
// block the investigation that signal is for. The returned keys are redacted
// by shape (see redactKeyShape), since a conflicting key is exactly as
// sensitive as the one this harness holds, and by construction is *not* our
// own key's text — so it cannot be found by searching for our key inside it.
func ApplyLicence(ctx context.Context, c *http.Client, baseURL, jwt, key string) ([]string, error) {
	body := map[string]string{"key": key}
	bearer := map[string]string{"Authorization": "Bearer " + jwt}
	var resp struct {
		ConflictingKeys []string `json:"conflictingKeys"`
	}
	if err := postJSON(ctx, c, baseURL+"/api/licenses/add", body, bearer, &resp); err != nil {
		return nil, fmt.Errorf("apply licence: %w", redactSecret(err, key))
	}
	if len(resp.ConflictingKeys) == 0 {
		return nil, nil
	}
	conflicting := make([]string, len(resp.ConflictingKeys))
	for i, k := range resp.ConflictingKeys {
		conflicting[i] = redactKeyShape(k)
	}
	return conflicting, nil
}

// ReleaseLicence removes a previously applied Business Edition licence.
//
// Licence release is unconditional: every server this harness attaches a
// licence to must give it back before it is destroyed, the Kubernetes leg
// included, since its licence key is shared with a real account and a
// disposable estate is not where it belongs to remain.
//
// Authenticated with apiKey (sent as X-Api-Key), not a JWT: a JWT is a
// session token that a server restart invalidates, while an API key is
// persisted in Portainer's own database and survives one. Measured against a
// live estate: after restarting the Business Edition container, the JWT
// captured at provisioning time answers 401 on this same endpoint while the
// API key still answers 200. down.sh and k3d-down.sh call this best-effort
// and swallow a failure to avoid blocking teardown — authenticating with a
// token a restart destroys would silently reintroduce the exact licence
// stranding this function exists to prevent, on the very failure path meant
// to catch it (a daemon restart, a host reboot, or system.upgrade itself).
func ReleaseLicence(ctx context.Context, c *http.Client, baseURL, apiKey, key string) error {
	body := map[string][]string{"LicenseKeys": {key}}
	headers := map[string]string{"X-Api-Key": apiKey}
	if err := postJSON(ctx, c, baseURL+"/api/licenses/remove", body, headers, nil); err != nil {
		return fmt.Errorf("release licence: %w", redactSecret(err, key))
	}
	return nil
}

// redactedMarker replaces every occurrence of a secret in an error's text.
const redactedMarker = "[REDACTED]"

// redactString returns s with every occurrence of secret replaced by a fixed
// marker. It is a no-op when secret is empty, so an unlicensed run does not
// start replacing every empty string in everything it touches.
func redactString(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, redactedMarker)
}

// redactKeyPrefixLen is how much of a conflicting licence key redactKeyShape
// keeps, for identification (telling two conflicting keys apart in a log)
// without exposing enough to reuse the key.
const redactKeyPrefixLen = 4

// redactKeyShape redacts a licence key by its own shape: a short prefix kept,
// the rest elided.
//
// This exists because a conflicting key named back by the server is, by
// construction, not the text of our own key — redactString(k, ourKey) is a
// near-guaranteed no-op in exactly the case it was written for, since it only
// ever finds and replaces occurrences of ourKey's own text. Redacting the
// conflicting key by its own shape works regardless of what it actually
// contains.
func redactKeyShape(s string) string {
	if len(s) <= redactKeyPrefixLen {
		return redactedMarker
	}
	return s[:redactKeyPrefixLen] + "..." + redactedMarker
}

// redactSecret returns err with every occurrence of secret in its rendered
// text replaced by a fixed marker. It is a no-op when secret is empty, so an
// unlicensed run does not start replacing every empty string in every error
// it touches.
func redactSecret(err error, secret string) error {
	if err == nil || secret == "" {
		return err
	}
	return errors.New(redactString(err.Error(), secret))
}

// ErrNoLicenceInstalled reports that GET /licenses answered successfully with
// an empty list. It is a distinct sentinel, not a message a caller matches by
// substring, so a caller verifying a licence release (recoverStrandedLicence)
// can tell "the licence is confirmed gone" apart from "the request itself
// failed" — a timeout, a 500, a decode failure, or a 401 all reach
// LicenceNodes' other error path and must never be mistaken for this one.
var ErrNoLicenceInstalled = errors.New("read licences: none installed")

// LicenceNodes reports the node allowance of the installed licence. It
// returns ErrNoLicenceInstalled, matchable with errors.Is, specifically when
// the server answered but reported no licence at all; every other failure
// (transport, HTTP status, decode) is wrapped and returned as-is.
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
		return 0, ErrNoLicenceInstalled
	}
	return licences[0].Nodes, nil
}

func postJSON(ctx context.Context, c *http.Client, url string, body any, headers map[string]string, out any) error {
	return sendJSON(ctx, c, http.MethodPost, url, body, headers, out)
}

// putJSON is postJSON's PUT twin, added for EnableEdgeCompute: Portainer's
// settings resource is updated with PUT, not POST.
func putJSON(ctx context.Context, c *http.Client, url string, body any, headers map[string]string, out any) error {
	return sendJSON(ctx, c, http.MethodPut, url, body, headers, out)
}

func sendJSON(ctx context.Context, c *http.Client, method, url string, body any, headers map[string]string, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(encoded))
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
