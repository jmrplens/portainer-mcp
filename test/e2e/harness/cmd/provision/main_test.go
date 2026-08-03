package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeBusinessEditionServer stands in for a real Business Edition Portainer
// server, answering exactly what provisionServer, harness.ApplyLicence, and
// harness.LicenceNodes each need to reach the licence-attached state. Every
// handler is unconditional: the test that fails a specific call does so by
// overriding just that one path after building the mux, not by adding
// branching logic here.
func fakeBusinessEditionServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"Id": 1})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestProvisionBusinessEdition_LicenceNodesFailure_ReturnsServerAlongsideError
// is the mutation-tested proof for the critical fix: harness.ApplyLicence
// above has already succeeded by the time GET /api/licenses (LicenceNodes)
// fails, so this server carries a real activation. Discarding it here — the
// defect this test guards against — would mean provisionBusinessEdition's
// caller (run, in main.go) has nothing to persist, and the estate file would
// never name the server or its API key that -release-licence needs to find
// and release it.
//
// Reverting the fix (restoring `return harness.Server{}, fmt.Errorf(...)` in
// provisionBusinessEdition's LicenceNodes branch) makes this test fail on
// both assertions below: BaseURL and Creds.APIKey come back empty. Verified
// by hand while writing this test, not merely asserted.
func TestProvisionBusinessEdition_LicenceNodesFailure_ReturnsServerAlongsideError(t *testing.T) {
	t.Parallel()
	server := fakeBusinessEditionServer(t)
	mux := server.Config.Handler.(*http.ServeMux)
	// The one failure this test is about: ApplyLicence above has already
	// succeeded, and now reading the licence back fails outright, exactly
	// like a timeout, a 500, or a transient network blip against a real
	// server would.
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	client := &http.Client{Timeout: kubernetesClientTimeout}
	got, err := provisionBusinessEdition(context.Background(), client, server.URL, "fake-licence")
	if err == nil {
		t.Fatal("provisionBusinessEdition() error = nil, want an error from the failing LicenceNodes call")
	}
	if got.BaseURL == "" {
		t.Error("provisionBusinessEdition() returned a Server with an empty BaseURL alongside the error: " +
			"the caller has nothing to persist, and the licence this call just attached is now unreachable " +
			"from the estate file")
	}
	if got.Creds.APIKey == "" {
		t.Error("provisionBusinessEdition() returned a Server with an empty API key alongside the error: " +
			"a later -release-licence call has no key to authenticate a release with")
	}
}

// TestProvisionBusinessEdition_Success_ReturnsTheProvisionedServer is the
// success-path sibling: with every call answering cleanly, the function
// returns no error and a fully populated Server, so the failure-path test
// above is verified against a handler difference of exactly one endpoint,
// not a mux that never worked to begin with.
func TestProvisionBusinessEdition_Success_ReturnsTheProvisionedServer(t *testing.T) {
	t.Parallel()
	server := fakeBusinessEditionServer(t)
	mux := server.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"nodes": 5}})
	})

	client := &http.Client{Timeout: kubernetesClientTimeout}
	got, err := provisionBusinessEdition(context.Background(), client, server.URL, "fake-licence")
	if err != nil {
		t.Fatalf("provisionBusinessEdition() error = %v, want nil", err)
	}
	if got.BaseURL != server.URL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, server.URL)
	}
	if got.Creds.APIKey != "the-api-key" {
		t.Errorf("Creds.APIKey = %q, want %q", got.Creds.APIKey, "the-api-key")
	}
}

// TestRecoverStrandedLicence_LicenceNodesTransportFailure_IsReportedAsAFailure
// is the mutation-tested proof for the MAJOR fix at this file's own
// recoverStrandedLicence: the verification after ReleaseLicence must accept
// only a confirmed empty licence list as proof of release, never any error
// at all. Here GET /licenses fails outright (a 500) after a real,
// successful release call — recoverStrandedLicence must report that the
// release could not be confirmed, not print its "safe to reuse" success
// message. Reverting recoverStrandedLicence's switch to the old
// `if _, err := harness.LicenceNodes(...); err == nil { return
// fmt.Errorf(...) }` shape makes this test fail: that check treats this
// exact 500 as confirmation and returns nil. Verified by hand while writing
// this fix.
func TestRecoverStrandedLicence_LicenceNodesTransportFailure_IsReportedAsAFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	mux.HandleFunc("/api/licenses/remove", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// The one failure this test is about: the release call above succeeded,
	// but confirming it (GET /licenses) fails outright.
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	t.Setenv(licenceEnv, "fake-licence")
	t.Setenv(recoverURLEnv, server.URL)

	err := recoverStrandedLicence()
	if err == nil {
		t.Fatal("recoverStrandedLicence() error = nil, want a failure: the release was never confirmed")
	}
	const wantSubstring = "could not confirm the licence was released"
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Errorf("recoverStrandedLicence() error = %q, want it to contain %q", err.Error(), wantSubstring)
	}
}

// TestRecoverStrandedLicence_Success_ConfirmsReleaseAndReturnsNil is the
// clean-path sibling: with every call, including the post-release GET
// /licenses, answering as a real server would after a genuine release
// (an empty list), recoverStrandedLicence returns nil.
func TestRecoverStrandedLicence_Success_ConfirmsReleaseAndReturnsNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	mux.HandleFunc("/api/licenses/remove", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	t.Setenv(licenceEnv, "fake-licence")
	t.Setenv(recoverURLEnv, server.URL)

	if err := recoverStrandedLicence(); err != nil {
		t.Fatalf("recoverStrandedLicence() error = %v, want nil", err)
	}
}
