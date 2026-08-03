package harness

import (
	"os"
	"path/filepath"
	"reflect"
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

func TestWriteEdgeEnv_WritesKeyValuePairsComposeReads(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".edge.env")
	if err := WriteEdgeEnv(path, "edge-uuid", "the-key", 7); err != nil {
		t.Fatalf("WriteEdgeEnv() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	want := "EDGE_ID=edge-uuid\nEDGE_KEY=the-key\nEDGE_ENDPOINT_ID=7\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestWriteEdgeEnv_IsOwnerOnly(t *testing.T) {
	t.Parallel()
	// It carries an enrolment key: the same treatment as the estate's API key.
	path := filepath.Join(t.TempDir(), ".edge.env")
	if err := WriteEdgeEnv(path, "id", "key", 1); err != nil {
		t.Fatalf("WriteEdgeEnv() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("permissions = %04o, want 0600", perm)
	}
}

func TestRemoveEdgeEnv_DeletesAnExistingFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".edge.env")
	if err := WriteEdgeEnv(path, "id", "key", 1); err != nil {
		t.Fatalf("WriteEdgeEnv() error = %v", err)
	}
	if err := RemoveEdgeEnv(path); err != nil {
		t.Fatalf("RemoveEdgeEnv() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("file still exists after RemoveEdgeEnv(): err = %v", err)
	}
}

func TestRemoveEdgeEnv_MissingFile_IsNotAnError(t *testing.T) {
	t.Parallel()
	// The common case: no edge environment was provisioned this run, so
	// there was never a file to remove.
	path := filepath.Join(t.TempDir(), ".edge.env")
	if err := RemoveEdgeEnv(path); err != nil {
		t.Errorf("RemoveEdgeEnv() error = %v, want nil for a file that never existed", err)
	}
}

func TestSyncEdgeEnv_EdgeProvisioned_WritesFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), ".edge.env")
	e := Estate{
		CE:             Server{Edition: "CE", BaseURL: "http://ce"},
		EdgeEndpointID: 7,
		EdgeAgentID:    "edge-uuid",
		EdgeKey:        "the-key",
	}
	if err := SyncEdgeEnv(e, path); err != nil {
		t.Fatalf("SyncEdgeEnv() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v, want SyncEdgeEnv to have written the file", err)
	}
	want := "EDGE_ID=edge-uuid\nEDGE_KEY=the-key\nEDGE_ENDPOINT_ID=7\n"
	if string(got) != want {
		t.Errorf("content = %q, want %q", got, want)
	}
}

func TestSyncEdgeEnv_NoEdge_RemovesStaleFile(t *testing.T) {
	t.Parallel()
	// A file left over from an earlier run that DID provision an edge
	// environment. This run's estate carries none — CE only, no licence —
	// and the file must not survive: up.sh's second compose pass would
	// otherwise start an agent enrolled with a dead key against a server
	// that, for all this run knows, no longer exists.
	path := filepath.Join(t.TempDir(), ".edge.env")
	if err := WriteEdgeEnv(path, "stale-uuid", "stale-key", 3); err != nil {
		t.Fatalf("WriteEdgeEnv() error = %v", err)
	}
	e := Estate{CE: Server{Edition: "CE", BaseURL: "http://ce"}}
	if err := SyncEdgeEnv(e, path); err != nil {
		t.Fatalf("SyncEdgeEnv() error = %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("stale edge environment file still exists after SyncEdgeEnv(): err = %v", err)
	}
}

func TestMergeKubernetes_PreservesTheComposeLegsAlreadyWritten(t *testing.T) {
	t.Parallel()
	existing := Estate{
		CE:             Server{Edition: "CE", BaseURL: "http://ce", Creds: Credentials{APIKey: "ce-key"}},
		EE:             Server{Edition: "EE", BaseURL: "http://ee", Creds: Credentials{APIKey: "ee-key"}},
		AgentID:        2,
		EdgeEndpointID: 3,
		EdgeAgentID:    "edge-uuid",
		EdgeKey:        "edge-key",
	}
	k8s := Server{Edition: "Kubernetes", BaseURL: "https://k8s", Creds: Credentials{APIKey: "k8s-key"}}

	got := existing.MergeKubernetes(k8s)

	// Server now carries a []string field (ConflictingLicenceKeys), so it can
	// no longer be compared with !=; reflect.DeepEqual is the direct
	// replacement, not a weakening of the assertion.
	if !reflect.DeepEqual(got.CE, existing.CE) {
		t.Errorf("CE = %+v, want it unchanged at %+v", got.CE, existing.CE)
	}
	if !reflect.DeepEqual(got.EE, existing.EE) {
		t.Errorf("EE = %+v, want it unchanged at %+v", got.EE, existing.EE)
	}
	if got.AgentID != existing.AgentID {
		t.Errorf("AgentID = %d, want %d", got.AgentID, existing.AgentID)
	}
	if got.EdgeEndpointID != existing.EdgeEndpointID || got.EdgeAgentID != existing.EdgeAgentID || got.EdgeKey != existing.EdgeKey {
		t.Errorf("edge fields changed: got %+v, want the edge fields from %+v", got, existing)
	}
	if !reflect.DeepEqual(got.Kubernetes, k8s) {
		t.Errorf("Kubernetes = %+v, want %+v", got.Kubernetes, k8s)
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
