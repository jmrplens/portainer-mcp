package harness

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakePortainer is enough of a Portainer to drive the provisioning sequence.
// It records what it was asked so a test can assert on the request, which is
// the only way to catch a header or content type that the real server would
// reject but a lenient stub would not.
type fakePortainer struct {
	requireSetupToken string
	sawSetupToken     string
	sawAPIKeyHeader   string
	sawEndpointForm   map[string]string
	sawEndpointCT     string
}

func (f *fakePortainer) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"x"}`))
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, r *http.Request) {
		f.sawSetupToken = r.Header.Get("X-Setup-Token")
		if f.requireSetupToken != "" && f.sawSetupToken != f.requireSetupToken {
			// Deliberately shares no wording with the remediation text
			// Provision's 403 branch adds: the point of that branch is to
			// say something the server itself never says, and the test
			// must not be satisfiable by an echo of this body alone.
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"message":"Access denied","details":"Unauthorized"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Id":1,"Username":"admin"}`))
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"jwt":"jwt-value"}`))
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer jwt-value" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`{"rawAPIKey":"ptr_rawkey","apiKey":{"id":1}}`))
	})
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, r *http.Request) {
		f.sawAPIKeyHeader = r.Header.Get("X-API-Key")
		f.sawEndpointCT = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		f.sawEndpointForm = map[string]string{}
		for k, v := range r.MultipartForm.Value {
			f.sawEndpointForm[k] = v[0]
		}
		_, _ = w.Write([]byte(`{"Id":7,"Name":"local"}`))
	})
	return mux
}

func TestProvision_NoSetupToken_ReturnsUsableCredentials(t *testing.T) {
	t.Parallel()
	fake := &fakePortainer{}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	creds, err := Provision(context.Background(), server.Client(), server.URL, "")
	if err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if creds.APIKey != "ptr_rawkey" {
		t.Errorf("APIKey = %q, want the value from rawAPIKey", creds.APIKey)
	}
	if creds.JWT != "jwt-value" {
		t.Errorf("JWT = %q, want the value from /auth", creds.JWT)
	}
	if creds.Password == "" {
		t.Error("Password is empty: the caller cannot re-authenticate or mint a second key")
	}
}

func TestProvision_SetupTokenRequired_IsSent(t *testing.T) {
	t.Parallel()
	// This is the failure the design spec did not anticipate: without the
	// header, init answers 403 and the whole sequence collapses.
	fake := &fakePortainer{requireSetupToken: "deadbeef"}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	if _, err := Provision(context.Background(), server.Client(), server.URL, "deadbeef"); err != nil {
		t.Fatalf("Provision() error = %v", err)
	}
	if fake.sawSetupToken != "deadbeef" {
		t.Errorf("X-Setup-Token = %q, want it forwarded", fake.sawSetupToken)
	}
}

func TestProvision_SetupTokenRequiredButAbsent_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	fake := &fakePortainer{requireSetupToken: "deadbeef"}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	_, err := Provision(context.Background(), server.Client(), server.URL, "")
	if err == nil {
		t.Fatal("Provision() error = nil, want an error when the server demands a setup token")
	}
	// The server's own message says the token is missing. That is the
	// diagnosis, and echoing it back teaches nobody anything. The value of
	// this branch is the remedy, which the server never states: either start
	// the container with --no-setup-token, or read the token out of its
	// startup logs. Assert on the remedy — a generic echo of the response
	// body cannot produce it, which is what makes this test discriminate.
	for _, want := range []string{"--no-setup-token", "startup logs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error = %q, want it to contain %q", err, want)
		}
	}
}

func TestAgentEndpoint_SetsBothSkipFlags(t *testing.T) {
	t.Parallel()
	// The Portainer agent always serves TLS with a self-signed certificate
	// valid only for "localhost", so a server reaching it by container name
	// cannot verify it. TLSSkipVerify alone is not enough: the real server
	// answers 400 "Invalid certificate file. Ensure that the file is uploaded
	// correctly", which names neither field and sends you looking for a file
	// that does not exist.
	spec := AgentEndpoint("agent", "portainer-agent")

	if spec.CreationType != 2 {
		t.Errorf("CreationType = %d, want 2 (agent environment)", spec.CreationType)
	}
	if spec.URL != "tcp://portainer-agent:9001" {
		t.Errorf("URL = %q, want the agent's tcp address on the compose network", spec.URL)
	}
	if !spec.TLS {
		t.Error("TLS = false: the agent serves TLS unconditionally")
	}
	if !spec.TLSSkipVerify {
		t.Error("TLSSkipVerify = false: the certificate is valid only for localhost")
	}
	if !spec.TLSSkipClientVerify {
		t.Error("TLSSkipClientVerify = false: without it Portainer answers 400 Invalid certificate file")
	}
}

func TestCreateEndpoint_SendsMultipartWithEveryTLSField(t *testing.T) {
	t.Parallel()
	// Registering an agent needs TLSSkipClientVerify as well as TLSSkipVerify.
	// With only the latter, the real server answers 400 "Invalid certificate
	// file" — a message that names neither field.
	fake := &fakePortainer{}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	id, err := CreateEndpoint(context.Background(), server.Client(), server.URL, "ptr_rawkey", EndpointSpec{
		Name: "agent", CreationType: 2, URL: "tcp://agent:9001",
		TLS: true, TLSSkipVerify: true, TLSSkipClientVerify: true,
	})
	if err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if id != 7 {
		t.Errorf("id = %d, want 7", id)
	}
	if !strings.HasPrefix(fake.sawEndpointCT, "multipart/form-data") {
		t.Errorf("Content-Type = %q, want multipart/form-data: the endpoint rejects JSON", fake.sawEndpointCT)
	}
	if fake.sawAPIKeyHeader != "ptr_rawkey" {
		t.Errorf("X-API-Key = %q, want the raw key", fake.sawAPIKeyHeader)
	}
	for field, want := range map[string]string{
		"Name": "agent", "EndpointCreationType": "2", "URL": "tcp://agent:9001",
		"TLS": "true", "TLSSkipVerify": "true", "TLSSkipClientVerify": "true",
	} {
		if got := fake.sawEndpointForm[field]; got != want {
			t.Errorf("form[%q] = %q, want %q", field, got, want)
		}
	}
}

func TestCreateEndpoint_LocalDocker_OmitsEmptyOptionalFields(t *testing.T) {
	t.Parallel()
	fake := &fakePortainer{}
	server := httptest.NewServer(fake.handler())
	t.Cleanup(server.Close)

	if _, err := CreateEndpoint(context.Background(), server.Client(), server.URL, "k", EndpointSpec{
		Name: "local", CreationType: 1,
	}); err != nil {
		t.Fatalf("CreateEndpoint() error = %v", err)
	}
	if _, present := fake.sawEndpointForm["URL"]; present {
		t.Error("URL was sent for a local Docker endpoint; an empty URL must be omitted, not sent blank")
	}
	if _, present := fake.sawEndpointForm["TLS"]; present {
		t.Error("TLS was sent when false; the field must be omitted so the server applies its own default")
	}
}

func TestCreateEdgeEndpoint_ReturnsEndpointIDEdgeIDAndKey(t *testing.T) {
	t.Parallel()
	var sawForm map[string]string
	var sawCreationType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		sawForm = map[string]string{}
		for k, v := range r.MultipartForm.Value {
			sawForm[k] = v[0]
		}
		sawCreationType = sawForm["EndpointCreationType"]
		_, _ = w.Write([]byte(`{"Id":9,"Name":"edge","EdgeID":"feb87bad-9d1c-41ed-86f0-f51d03abde3a","EdgeKey":"aGVsbG8="}`))
	}))
	t.Cleanup(server.Close)

	creds, err := CreateEdgeEndpoint(context.Background(), server.Client(), server.URL, "ptr_rawkey", "edge")
	if err != nil {
		t.Fatalf("CreateEdgeEndpoint() error = %v", err)
	}
	// EndpointID and EdgeID are deliberately checked against different
	// values: the field that matters for an agent's EDGE_ID environment
	// variable is the UUID, not the ordinary numeric database id, and a test
	// that used the same value for both would not catch the two being
	// swapped.
	if creds.EndpointID != 9 {
		t.Errorf("EndpointID = %d, want 9", creds.EndpointID)
	}
	if creds.EdgeID != "feb87bad-9d1c-41ed-86f0-f51d03abde3a" {
		t.Errorf("EdgeID = %q, want the UUID carried in the response's EdgeID field", creds.EdgeID)
	}
	if creds.Key != "aGVsbG8=" {
		t.Errorf("Key = %q, want the EdgeKey carried in the response", creds.Key)
	}
	if sawCreationType != "4" {
		t.Errorf("EndpointCreationType = %q, want 4 (edge agent)", sawCreationType)
	}
	if _, present := sawForm["URL"]; present {
		t.Error("URL was sent for an edge environment; the server derives it from its own settings")
	}
}

func TestCreateEdgeEndpoint_NoEdgeKeyInResponse_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	// A server that accepts the creation but, for whatever reason, answers
	// without a key. Silently returning an EdgeCredentials no agent can ever
	// use would be a worse failure than an explicit error here.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Id":9,"Name":"edge","EdgeID":"feb87bad-9d1c-41ed-86f0-f51d03abde3a"}`))
	}))
	t.Cleanup(server.Close)

	_, err := CreateEdgeEndpoint(context.Background(), server.Client(), server.URL, "k", "edge")
	if err == nil {
		t.Fatal("CreateEdgeEndpoint() error = nil, want an error when the response carries no EdgeKey")
	}
}

func TestCreateEdgeEndpoint_NoEdgeIDInResponse_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	// The distinct sibling of the case above: a key with no UUID is just as
	// useless to an agent, since EDGE_ID is what it needs to identify itself.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Id":9,"Name":"edge","EdgeKey":"aGVsbG8="}`))
	}))
	t.Cleanup(server.Close)

	_, err := CreateEdgeEndpoint(context.Background(), server.Client(), server.URL, "k", "edge")
	if err == nil {
		t.Fatal("CreateEdgeEndpoint() error = nil, want an error when the response carries no EdgeID")
	}
}

func TestEnableEdgeCompute_SendsTheAgentReachableAddresses(t *testing.T) {
	t.Parallel()
	// Without this call, edge endpoint creation fails with "API server URL
	// not set in Edge Compute settings" — verified against a live estate.
	var sawAPIKey string
	var sawBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("method = %s, want PUT", r.Method)
		}
		sawAPIKey = r.Header.Get("X-API-Key")
		_ = json.NewDecoder(r.Body).Decode(&sawBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	err := EnableEdgeCompute(context.Background(), server.Client(), server.URL, "ptr_rawkey",
		"http://portainer-ee:9000", "portainer-ee:8000")
	if err != nil {
		t.Fatalf("EnableEdgeCompute() error = %v", err)
	}
	if sawAPIKey != "ptr_rawkey" {
		t.Errorf("X-API-Key = %q, want the raw key", sawAPIKey)
	}
	if sawBody["EnableEdgeComputeFeatures"] != true {
		t.Errorf("EnableEdgeComputeFeatures = %v, want true", sawBody["EnableEdgeComputeFeatures"])
	}
	if sawBody["EdgePortainerUrl"] != "http://portainer-ee:9000" {
		t.Errorf("EdgePortainerUrl = %v, want the agent-reachable URL", sawBody["EdgePortainerUrl"])
	}
	edge, _ := sawBody["Edge"].(map[string]any)
	if edge == nil || edge["TunnelServerAddress"] != "portainer-ee:8000" {
		t.Errorf("Edge.TunnelServerAddress = %v, want the agent-reachable tunnel address", edge)
	}
}

func TestApplyLicence_DoesNotLeakTheKeyIntoTheError(t *testing.T) {
	t.Parallel()
	// The licence is a secret in a gitignored .env. A failure that echoes it
	// puts it in a CI log, which is the one place it must never reach.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid licence"}`))
	}))
	t.Cleanup(server.Close)

	const secret = "3-SUPERSECRETLICENCEKEY=="
	_, err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
	if err == nil {
		t.Fatal("ApplyLicence() error = nil, want an error on a 400")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "SUPERSECRET") {
		t.Fatalf("the licence key leaked into the error: %v", err)
	}
}

func TestApplyLicence_ServerEchoesTheKey_ItIsRedacted(t *testing.T) {
	t.Parallel()
	const secret = "3-SUPERSECRETLICENCEKEY=="
	// A server that quotes back what it was sent. We have not observed
	// Portainer doing this, and that is the point: the guard must not depend
	// on having observed it.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"message":"invalid licence","details":%q}`, string(body))
	}))
	t.Cleanup(server.Close)

	_, err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
	if err == nil {
		t.Fatal("ApplyLicence() error = nil, want an error on a 400")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "SUPERSECRET") {
		t.Fatalf("the licence key reached the error through the server's own response: %v", err)
	}
	// The error must still be useful — redaction that erases the diagnosis
	// trades one problem for another.
	if !strings.Contains(err.Error(), "invalid licence") {
		t.Errorf("error = %q, want it to keep the server's diagnosis", err)
	}
}

func TestApplyLicence_CleanAttach_ReturnsNoConflictingKeys(t *testing.T) {
	t.Parallel()
	// The measured, normal case: a clean attach answers
	// {"conflictingKeys":null}.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"conflictingKeys":null}`))
	}))
	t.Cleanup(server.Close)

	conflicting, err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", "3-KEY")
	if err != nil {
		t.Fatalf("ApplyLicence() error = %v", err)
	}
	if len(conflicting) != 0 {
		t.Errorf("conflicting = %v, want none on a clean attach", conflicting)
	}
}

func TestApplyLicence_ConflictingKeys_ReturnedAndRedacted(t *testing.T) {
	t.Parallel()
	// The early warning the plan asks to watch for: a non-null
	// conflictingKeys is the first observable sign the vendor tracks
	// activations across runs. It must not become an error (failing here
	// would only block investigating it). Every conflicting key returned must
	// be redacted, whether or not it happens to equal our own secret: a
	// conflicting key is, by construction, someone else's key, so a guard that
	// only redacts occurrences of our own key's text inside it is a
	// near-guaranteed no-op for exactly the value it exists to protect. The
	// second entry here ("3-someOtherKey") is a genuinely different key,
	// deliberately never equal to secret, so this test would have caught that
	// defect: an implementation that redacted only by searching for our own
	// key would leave it in the clear, and this assertion requires it to be
	// redacted by its own shape instead.
	const secret = "3-SUPERSECRETLICENCEKEY=="
	const otherKey = "3-someOtherKey"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"conflictingKeys":[%q,%q]}`, secret, otherKey)
	}))
	t.Cleanup(server.Close)

	conflicting, err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
	if err != nil {
		t.Fatalf("ApplyLicence() error = %v, want no error: a conflict is a warning, not a failure", err)
	}
	if len(conflicting) != 2 {
		t.Fatalf("conflicting = %v, want 2 entries", conflicting)
	}
	for _, k := range conflicting {
		if strings.Contains(k, secret) || strings.Contains(k, "SUPERSECRET") {
			t.Fatalf("a conflicting key leaked our own licence key unredacted: %v", conflicting)
		}
	}
	if conflicting[1] == otherKey {
		t.Errorf("conflicting[1] = %q, a genuinely different conflicting key was returned unredacted", conflicting[1])
	}
	if !strings.HasPrefix(conflicting[1], otherKey[:redactKeyPrefixLen]) || !strings.Contains(conflicting[1], redactedMarker) {
		t.Errorf("conflicting[1] = %q, want a short identifying prefix of %q followed by %q", conflicting[1], otherKey, redactedMarker)
	}
}

func TestReleaseLicence_DoesNotLeakTheKeyIntoTheError(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"invalid licence"}`))
	}))
	t.Cleanup(server.Close)

	const secret = "3-SUPERSECRETLICENCEKEY=="
	err := ReleaseLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
	if err == nil {
		t.Fatal("ReleaseLicence() error = nil, want an error on a 400")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "SUPERSECRET") {
		t.Fatalf("the licence key leaked into the error: %v", err)
	}
}

func TestReleaseLicence_ServerEchoesTheKey_ItIsRedacted(t *testing.T) {
	t.Parallel()
	const secret = "3-SUPERSECRETLICENCEKEY=="
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprintf(w, `{"message":"invalid licence","details":%q}`, string(body))
	}))
	t.Cleanup(server.Close)

	err := ReleaseLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
	if err == nil {
		t.Fatal("ReleaseLicence() error = nil, want an error on a 400")
	}
	if strings.Contains(err.Error(), secret) || strings.Contains(err.Error(), "SUPERSECRET") {
		t.Fatalf("the licence key reached the error through the server's own response: %v", err)
	}
	if !strings.Contains(err.Error(), "invalid licence") {
		t.Errorf("error = %q, want it to keep the server's diagnosis", err)
	}
}

func TestReleaseLicence_SendsTheKeyAsALicenseKeysArray(t *testing.T) {
	t.Parallel()
	const secret = "3-SOMELICENCEKEY=="
	var gotBody map[string][]string
	var gotAPIKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAPIKey = r.Header.Get("X-Api-Key")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	if err := ReleaseLicence(context.Background(), server.Client(), server.URL, "the-api-key", secret); err != nil {
		t.Fatalf("ReleaseLicence() error = %v", err)
	}
	// X-Api-Key, not a bearer JWT: see ReleaseLicence's own doc for why — a
	// JWT is a session token a server restart invalidates, an API key is not.
	if gotAPIKey != "the-api-key" {
		t.Errorf("X-Api-Key header = %q, want the-api-key", gotAPIKey)
	}
	want := []string{secret}
	if len(gotBody["LicenseKeys"]) != 1 || gotBody["LicenseKeys"][0] != want[0] {
		t.Errorf("LicenseKeys = %v, want %v", gotBody["LicenseKeys"], want)
	}
}

func TestRedactSecret_EmptySecret_LeavesTheErrorAlone(t *testing.T) {
	t.Parallel()
	// An unlicensed run passes an empty key. Replacing the empty string would
	// corrupt every message it touches.
	original := errors.New("something failed")
	if got := redactSecret(original, "").Error(); got != "something failed" {
		t.Errorf("redactSecret with an empty secret = %q, want the error unchanged", got)
	}
}

func TestLicenceNodes_ReadsTheNodeAllowance(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"company": "3 Nodes Free", "nodes": 3, "productEdition": 2},
		})
	}))
	t.Cleanup(server.Close)

	nodes, err := LicenceNodes(context.Background(), server.Client(), server.URL, "jwt")
	if err != nil {
		t.Fatalf("LicenceNodes() error = %v", err)
	}
	if nodes != 3 {
		t.Errorf("nodes = %d, want 3", nodes)
	}
}
