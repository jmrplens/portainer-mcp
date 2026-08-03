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
			Creds:        Credentials{Username: "admin", Password: "p", APIKey: "k", JWT: "j"},
			Environments: map[string]int{"docker": 1, "agent": 2},
		},
	}
	if err := want.SaveTo(path); err != nil {
		t.Fatalf("SaveTo() error = %v", err)
	}
	got, err := LoadEstate(path)
	if err != nil {
		t.Fatalf("LoadEstate() error = %v", err)
	}
	if got.CE.Creds.APIKey != "k" {
		t.Errorf("round trip lost data: %+v", got)
	}
	dockerID, ok := got.CE.Environment("docker")
	if !ok || dockerID != 1 {
		t.Errorf("round trip: CE.Environment(\"docker\") = (%d, %v), want (1, true)", dockerID, ok)
	}
	agentID, ok := got.CE.Environment("agent")
	if !ok || agentID != 2 {
		t.Errorf("round trip: CE.Environment(\"agent\") = (%d, %v), want (2, true)", agentID, ok)
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

func TestSyncEdgeEnv_PartialEdgeCredentials_RemovesStaleFile(t *testing.T) {
	t.Parallel()
	// An interrupted run that recorded an agent id but never a key (or vice
	// versa) cannot enrol anything. The file must not survive, exactly as
	// for no edge at all. This is the one branch TestSyncEdgeEnv_
	// EdgeProvisioned_WritesFile and TestSyncEdgeEnv_NoEdge_RemovesStaleFile
	// do not cover between them: a change of SyncEdgeEnv's "&&" to "||"
	// passes both of those unchanged.
	for _, tt := range []struct {
		name    string
		agentID string
		key     string
	}{
		{name: "agent id only", agentID: "edge-uuid"},
		{name: "key only", key: "the-key"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), ".edge.env")
			if err := WriteEdgeEnv(path, "stale-uuid", "stale-key", 3); err != nil {
				t.Fatalf("WriteEdgeEnv() error = %v", err)
			}
			e := Estate{
				CE:          Server{Edition: "CE", BaseURL: "http://ce"},
				EdgeAgentID: tt.agentID,
				EdgeKey:     tt.key,
			}
			if err := SyncEdgeEnv(e, path); err != nil {
				t.Fatalf("SyncEdgeEnv() error = %v", err)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("stale edge environment file survived a partial edge state: err = %v", err)
			}
		})
	}
}

func TestMergeKubernetes_PreservesTheComposeLegsAlreadyWritten(t *testing.T) {
	t.Parallel()
	existing := Estate{
		CE: Server{
			Edition: "CE", BaseURL: "http://ce",
			Creds:        Credentials{APIKey: "ce-key"},
			Environments: map[string]int{"docker": 1, "agent": 2},
		},
		EE:             Server{Edition: "EE", BaseURL: "http://ee", Creds: Credentials{APIKey: "ee-key"}},
		EdgeEndpointID: 3,
		EdgeAgentID:    "edge-uuid",
		EdgeKey:        "edge-key",
	}
	k8s := Server{Edition: "Kubernetes", BaseURL: "https://k8s", Creds: Credentials{APIKey: "k8s-key"}}

	got := existing.MergeKubernetes(k8s)

	// Server now carries a []string field (ConflictingLicenceKeys) and a map
	// field (Environments), so it can no longer be compared with !=;
	// reflect.DeepEqual is the direct replacement, not a weakening of the
	// assertion.
	if !reflect.DeepEqual(got.CE, existing.CE) {
		t.Errorf("CE = %+v, want it unchanged at %+v", got.CE, existing.CE)
	}
	if !reflect.DeepEqual(got.EE, existing.EE) {
		t.Errorf("EE = %+v, want it unchanged at %+v", got.EE, existing.EE)
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

// TestServer_Environment_UnrecordedName_ReportsAbsent proves the zero-value
// case before the "two environments" case below builds on it: a Server that
// has never had WithEnvironment called reports every name absent, not a
// zero id that could be confused with a real one.
func TestServer_Environment_UnrecordedName_ReportsAbsent(t *testing.T) {
	t.Parallel()
	var s Server
	if id, ok := s.Environment("docker"); ok {
		t.Errorf("Environment(\"docker\") = (%d, true), want ok = false on a Server with none recorded", id)
	}
}

// TestServer_WithEnvironment_CarriesMoreThanOneDockerEnvironment is the proof
// task 7a's own brief calls for by name: a domain needing two Docker
// environments (endpoint groups, edge groups, tag assignment and migration
// all do) has no way to express that with a single AgentID field, which is
// exactly what this task removes. The trap named in the brief is a test that
// passes trivially by only ever putting one environment in — so this
// constructs two, under different names, and requires each one to still
// report its own distinct id after the other was added, not the last id
// written clobbering the first.
func TestServer_WithEnvironment_CarriesMoreThanOneDockerEnvironment(t *testing.T) {
	t.Parallel()
	srv := Server{Edition: "CE", BaseURL: "http://ce"}

	withFirst := srv.WithEnvironment("docker", 3)
	withBoth := withFirst.WithEnvironment("docker-secondary", 9)

	firstID, ok := withBoth.Environment("docker")
	if !ok || firstID != 3 {
		t.Fatalf("Environment(\"docker\") = (%d, %v), want (3, true)", firstID, ok)
	}
	secondID, ok := withBoth.Environment("docker-secondary")
	if !ok || secondID != 9 {
		t.Fatalf("Environment(\"docker-secondary\") = (%d, %v), want (9, true)", secondID, ok)
	}
	// The one assertion that actually rules out the trivial pass: if adding
	// the second environment had silently overwritten or aliased the first
	// (for instance, by reusing the same underlying map without copying, or
	// by a WithEnvironment that ignored its receiver's existing entries),
	// both ids could come back equal, or the first could have vanished
	// entirely. Neither id is zero and they must differ.
	if firstID == secondID {
		t.Fatal("both environments report the same id: the second is not distinguishable from the first")
	}

	// WithEnvironment must not mutate the receiver's own map: withFirst,
	// built before withBoth existed, must still report only its own one
	// entry. A shared, mutated-in-place map would make this call see the
	// later addition too.
	if _, ok := withFirst.Environment("docker-secondary"); ok {
		t.Error("WithEnvironment mutated an earlier Server's map: withFirst sees the later call's addition")
	}
	// And srv itself, the Server neither WithEnvironment call was supposed to
	// touch, must still report nothing recorded at all.
	if _, ok := srv.Environment("docker"); ok {
		t.Error("WithEnvironment mutated the original receiver: srv sees an environment recorded on a value derived from it")
	}
}

// TestEstate_Legs_DerivesFromWhatWasActuallyProvisioned is 7b's proof: the
// edition/platform axis is read off the Estate's own HasBusinessEdition and
// HasKubernetes, not a literal every caller repeats and a fourth leg would
// need copied into by hand. An estate with only a Community Edition leg
// reports exactly one; adding a licensed Business Edition leg and a
// Kubernetes leg each grow the result by exactly one more, distinguishable
// by name.
func TestEstate_Legs_DerivesFromWhatWasActuallyProvisioned(t *testing.T) {
	t.Parallel()

	ceOnly := Estate{CE: Server{Edition: "CE", BaseURL: "http://ce"}}
	legs := ceOnly.Legs()
	if len(legs) != 1 || legs[0].Name != "CE" {
		t.Fatalf("Legs() on a Community-only estate = %+v, want exactly one leg named CE", legs)
	}

	full := Estate{
		CE:         Server{Edition: "CE", BaseURL: "http://ce"},
		EE:         Server{Edition: "EE", BaseURL: "http://ee", Creds: Credentials{APIKey: "ee-key"}},
		Kubernetes: Server{Edition: "Kubernetes", BaseURL: "https://k8s", Creds: Credentials{APIKey: "k8s-key"}},
	}
	legs = full.Legs()
	if len(legs) != 3 {
		t.Fatalf("Legs() on a fully provisioned estate returned %d legs, want 3: %+v", len(legs), legs)
	}
	names := []string{legs[0].Name, legs[1].Name, legs[2].Name}
	want := []string{"CE", "EE", "Kubernetes"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("Legs() names = %v, want %v", names, want)
	}
	if legs[1].Server.BaseURL != "http://ee" {
		t.Errorf("EE leg carries the wrong Server: %+v", legs[1].Server)
	}
	if legs[2].Server.BaseURL != "https://k8s" {
		t.Errorf("Kubernetes leg carries the wrong Server: %+v", legs[2].Server)
	}
}
