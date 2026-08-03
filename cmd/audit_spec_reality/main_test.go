package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// writeFixture writes body to dir/name, failing the test on error.
func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// estateJSON marshals a minimal harness.Estate-shaped document by hand
// rather than importing harness.Estate's own struct: this test exercises
// run's LoadEstate call as a real caller would (a JSON file on disk,
// unaware of anything but the documented shape), the same way
// cmd/audit_1to1's tests build fixtures for the specs it reads rather than
// depending on the packages that produce them.
func estateJSON(t *testing.T, ceURL, ceKey, eeURL, eeKey string) string {
	t.Helper()
	doc := map[string]any{
		"ce": map[string]any{"edition": "CE", "base_url": ceURL, "credentials": map[string]any{"APIKey": ceKey}},
	}
	if eeURL != "" {
		doc["ee"] = map[string]any{"edition": "EE", "base_url": eeURL, "credentials": map[string]any{"APIKey": eeKey}}
	}
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal estate fixture: %v", err)
	}
	return string(data)
}

// fixtureServer builds an httptest.Server whose /api/tags responds as a
// present, registered route (401, Portainer's real answer to an invalid
// credential) and leaves every other path to Go's real default not-found
// fallback.
func fixtureServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/tags", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Invalid JWT token","details":"Unauthorized"}`))
	})
	return httptest.NewServer(mux)
}

const oneOpSpec = `{"paths": {"/tags": {"get": {"operationId": "tagsList", "tags": ["tags"]}}}}`

// TestUnit_Run_CommunityEditionOnly_AuditsCEAndSkipsEE covers an estate
// with no Business Edition leg: run must still succeed, audit CE, and say
// plainly that EE was not probed rather than silently omitting it.
func TestUnit_Run_CommunityEditionOnly_AuditsCEAndSkipsEE(t *testing.T) {
	t.Parallel()
	srv := fixtureServer()
	defer srv.Close()

	dir := t.TempDir()
	writeFixture(t, dir, "estate.json", estateJSON(t, srv.URL, "k", "", ""))
	writeFixture(t, dir, "ce-1.0.0.json", oneOpSpec)
	writeFixture(t, dir, "ee-1.0.0.json", oneOpSpec)

	var out strings.Builder
	if err := run(&out, filepath.Join(dir, "estate.json"), dir, "1.0.0", 2*time.Second); err != nil {
		t.Fatalf("run() error = %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "CE leg: 1 documented operation") {
		t.Errorf("run() output = %q, want the CE leg audited", out.String())
	}
	if strings.Contains(out.String(), "EE leg:") {
		t.Errorf("run() output = %q, want no EE leg section: the estate carries no Business Edition credentials", out.String())
	}
	if !strings.Contains(out.String(), "no licence") {
		t.Errorf("run() output = %q, want it to say plainly that EE was not probed", out.String())
	}
}

// TestUnit_Run_BusinessEdition_AuditsBothLegs covers an estate with both
// legs present.
func TestUnit_Run_BusinessEdition_AuditsBothLegs(t *testing.T) {
	t.Parallel()
	ceSrv := fixtureServer()
	defer ceSrv.Close()
	eeSrv := fixtureServer()
	defer eeSrv.Close()

	dir := t.TempDir()
	writeFixture(t, dir, "estate.json", estateJSON(t, ceSrv.URL, "ce-key", eeSrv.URL, "ee-key"))
	writeFixture(t, dir, "ce-1.0.0.json", oneOpSpec)
	writeFixture(t, dir, "ee-1.0.0.json", oneOpSpec)

	var out strings.Builder
	if err := run(&out, filepath.Join(dir, "estate.json"), dir, "1.0.0", 2*time.Second); err != nil {
		t.Fatalf("run() error = %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "CE leg: 1 documented operation") || !strings.Contains(out.String(), "EE leg: 1 documented operation") {
		t.Errorf("run() output = %q, want both legs audited", out.String())
	}
}

// TestUnit_Run_MissingEstate_ReturnsError is the plumbing failure mode.
func TestUnit_Run_MissingEstate_ReturnsError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	var out strings.Builder
	err := run(&out, filepath.Join(dir, "does-not-exist.json"), dir, "1.0.0", time.Second)
	if err == nil {
		t.Fatal("run() error = nil, want an error for a missing estate file")
	}
}

// TestUnit_Run_MissingSpecFile_ReturnsError is the plumbing failure mode on
// the other input.
func TestUnit_Run_MissingSpecFile_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := fixtureServer()
	defer srv.Close()

	dir := t.TempDir()
	writeFixture(t, dir, "estate.json", estateJSON(t, srv.URL, "k", "", ""))
	var out strings.Builder
	err := run(&out, filepath.Join(dir, "estate.json"), dir, "9.9.9", time.Second)
	if err == nil {
		t.Fatal("run() error = nil, want an error: no ce-9.9.9.json/ee-9.9.9.json in dir")
	}
}

// TestUnit_Run_SelfTestFailure_PropagatesAsError proves a broken detector
// stops run outright rather than reporting a false-clean audit: this
// server answers every path — including the manufactured canary — as if it
// were a real, present route.
func TestUnit_Run_SelfTestFailure_PropagatesAsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	dir := t.TempDir()
	writeFixture(t, dir, "estate.json", estateJSON(t, srv.URL, "k", "", ""))
	writeFixture(t, dir, "ce-1.0.0.json", oneOpSpec)
	writeFixture(t, dir, "ee-1.0.0.json", oneOpSpec)

	var out strings.Builder
	err := run(&out, filepath.Join(dir, "estate.json"), dir, "1.0.0", 2*time.Second)
	if err == nil {
		t.Fatal("run() error = nil, want an error: the self-test canary was answered as a present route")
	}
}

// TestUnit_ApiBaseURL_TrimsTrailingSlashAndAppendsAPI mirrors
// internal/portainer.New's identical normalisation of the same estate field.
func TestUnit_ApiBaseURL_TrimsTrailingSlashAndAppendsAPI(t *testing.T) {
	t.Parallel()
	if got := apiBaseURL("http://host:9000/"); got != "http://host:9000/api" {
		t.Errorf("apiBaseURL(with trailing slash) = %q, want %q", got, "http://host:9000/api")
	}
	if got := apiBaseURL("http://host:9000"); got != "http://host:9000/api" {
		t.Errorf("apiBaseURL(no trailing slash) = %q, want %q", got, "http://host:9000/api")
	}
}

// TestUnit_EstateDefault_UsesEnvVarWhenSet proves the flag default reads the
// same environment variable test/e2e/suite's TestMain and up.sh already
// agree on, rather than a second name this command invented for itself.
func TestUnit_EstateDefault_UsesEnvVarWhenSet(t *testing.T) {
	t.Setenv(harness.EstateFileEnv, "/tmp/somewhere/estate.json")
	if got := estateDefault(); got != "/tmp/somewhere/estate.json" {
		t.Errorf("estateDefault() = %q, want the environment variable's value", got)
	}
}

// TestUnit_EstateDefault_FallsBackWhenUnset covers the other branch.
func TestUnit_EstateDefault_FallsBackWhenUnset(t *testing.T) {
	t.Setenv(harness.EstateFileEnv, "")
	if got := estateDefault(); got != defaultEstate {
		t.Errorf("estateDefault() = %q, want the fallback %q", got, defaultEstate)
	}
}

// TestUnit_ReadFileIn_ReadsFileWithinDir is the ordinary case.
func TestUnit_ReadFileIn_ReadsFileWithinDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeFixture(t, dir, "spec.json", `{"paths": {}}`)

	data, err := readFileIn(dir, "spec.json")
	if err != nil {
		t.Fatalf("readFileIn() error = %v", err)
	}
	if string(data) != `{"paths": {}}` {
		t.Errorf("readFileIn() = %q, want the fixture's content", data)
	}
}

// TestUnit_ReadFileIn_RefusesToEscapeDir proves the confinement check
// actually bites.
func TestUnit_ReadFileIn_RefusesToEscapeDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	rel, err := filepath.Rel(dir, outside)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if _, err := readFileIn(dir, rel); err == nil {
		t.Fatal("readFileIn() = nil error, want it to refuse a name that escapes dir")
	}
}
