package harness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEstate_RoundTrips(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "estate.json")
	want := Estate{
		CE: Server{
			Edition: "CE", BaseURL: "http://127.0.0.1:19000",
			Creds: Credentials{Username: "admin", Password: "p", APIKey: "k", JWT: "j"},
		},
		AgentID: 2,
	}
	if err := want.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}
	got, err := LoadEstate(path)
	if err != nil {
		t.Fatalf("LoadEstate() error = %v", err)
	}
	if got.CE.Creds.APIKey != "k" || got.AgentID != 2 {
		t.Errorf("round trip lost data: %+v", got)
	}
}

func TestEstate_SaveTo_IsOwnerOnly(t *testing.T) {
	t.Parallel()
	// The file holds an API key. World-readable is not acceptable, least of
	// all on a shared CI runner.
	path := filepath.Join(t.TempDir(), "estate.json")
	if err := (Estate{CE: Server{BaseURL: "u"}}).SaveTo(path); err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %04o, want 0600", perm)
	}
}

func TestEstate_HasBusinessEdition_FalseWithoutCredentials(t *testing.T) {
	t.Parallel()
	// A contributor with no licence must be able to run the CE suites, so an
	// absent Business Edition is a skip signal, not a broken estate.
	e := Estate{CE: Server{BaseURL: "u", Creds: Credentials{APIKey: "k"}}}
	if e.HasBusinessEdition() {
		t.Error("HasBusinessEdition() = true with no EE server provisioned")
	}
	e.EE = Server{BaseURL: "u2"} // present but unprovisioned
	if e.HasBusinessEdition() {
		t.Error("HasBusinessEdition() = true for a server with no API key")
	}
}

func TestLoadEstate_MissingCommunityEdition_ReturnsInformativeError(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "estate.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadEstate(path)
	if err == nil {
		t.Fatal("LoadEstate() error = nil, want an error for an estate with no CE server")
	}
	if !strings.Contains(err.Error(), "Community Edition") {
		t.Errorf("error = %q, want it to name what is missing", err)
	}
}

func TestEstate_HasKubernetes_FalseWithoutCredentials(t *testing.T) {
	t.Parallel()
	e := Estate{CE: Server{BaseURL: "u"}}
	if e.HasKubernetes() {
		t.Error("HasKubernetes() = true with no Kubernetes server provisioned")
	}
	e.Kubernetes = Server{BaseURL: "u3", Creds: Credentials{APIKey: "k"}}
	if !e.HasKubernetes() {
		t.Error("HasKubernetes() = false for a fully provisioned Kubernetes server")
	}
}

func TestLoadEstate_RejectsPathTraversal(t *testing.T) {
	t.Parallel()
	_, err := LoadEstate("../../../../etc/passwd")
	if err == nil {
		t.Fatal("LoadEstate() error = nil, want an error for a relative path that climbs above its starting directory")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("error = %q, want it to name the traversal", err)
	}
}
