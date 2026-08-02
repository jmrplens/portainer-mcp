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
	err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
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

	err := ApplyLicence(context.Background(), server.Client(), server.URL, "jwt", secret)
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
