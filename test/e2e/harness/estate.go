package harness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EstateFileEnv names the environment variable carrying the path to the estate
// file. The scripts write it; TestMain reads it.
const EstateFileEnv = "PORTAINER_E2E_ESTATE"

// EdgeEnvFileEnv names the environment variable carrying the path to the
// derived edge environment file written alongside the estate (see
// WriteEdgeEnv). up.sh sources it with --env-file so docker compose can start
// the edge agent without anything parsing the estate's JSON to reach three
// strings out of it.
const EdgeEnvFileEnv = "PORTAINER_E2E_EDGE_ENV"

// The well-known Environments names cmd/provision/main.go registers today:
// EnvironmentDocker for each edition's own Docker-in-Docker endpoint,
// EnvironmentAgent for the agent endpoint registered against the Community
// Edition server only, and EnvironmentK3D for the Kubernetes leg. Exported so
// a domain reading one of them by name — roughly twenty P3 domains will —
// gets a typo caught at compile time instead of Environment silently
// returning ok = false, which a domain action could too easily treat as "no
// environment configured" rather than "the caller misspelled the name".
const (
	EnvironmentDocker = "docker"
	EnvironmentAgent  = "agent"
	EnvironmentK3D    = "k3d"
)

// Server is one provisioned Portainer.
type Server struct {
	Edition string      `json:"edition"`
	BaseURL string      `json:"base_url"`
	Creds   Credentials `json:"credentials"`

	// InstanceID is this server's own Server Instance ID, read from GET
	// /system/status at provisioning time (see harness.WaitReady's
	// ReadyStatus and cmd/provision/main.go's provisionServer) and persisted
	// here rather than used once and discarded. Portainer writes this value
	// into its own /data volume, so it is stable across a restart of the
	// same container — unlike the administrator credentials, which
	// harness.Provision now mints fresh every run and which therefore cannot
	// serve as a "this is the server we provisioned" marker any more.
	// cleanupOrphans (test/e2e/suite/fixtures_test.go) re-reads this field
	// live from whatever server an estate file points at and refuses its
	// destructive sweep unless the two agree.
	InstanceID string `json:"instance_id"`

	// ConflictingLicenceKeys carries what POST /licenses/add named as
	// "conflictingKeys" when this server's licence was attached, if it was
	// non-null. A clean attach leaves this empty. A non-empty value is the
	// first observable sign that Portainer's own licensing service tracks
	// activations across ephemeral runs (see plan/carry-forward.md); it is
	// recorded here, rather than only logged once at attach time, so it
	// survives past that one stderr line for whoever inspects the estate
	// afterwards. Values are redacted the same way harness.ApplyLicence
	// redacts its own errors.
	ConflictingLicenceKeys []string `json:"conflicting_licence_keys,omitempty"`

	// Environments maps a locally-meaningful name ("docker", "agent", "k3d",
	// ...) to the numeric endpoint id Portainer assigned when
	// cmd/provision/main.go registered it against this server.
	//
	// This replaces the single Estate.AgentID field this harness used to
	// carry: that field recorded exactly one environment, could not be
	// extended without growing Estate a new field per name, and nothing ever
	// read it back. Every P3 action that takes an environment id — the
	// containers, stacks, volumes, images, kubernetes, webhooks and agent-
	// proxy domains all take one — reads it from here, by name, instead of
	// hardcoding 1 or 2 on the assumption that those numbers only ever mean
	// what they meant on an empty database at creation time.
	//
	// It is a map keyed by a caller-chosen name, not a fixed pair of fields,
	// specifically so a domain that needs two Docker environments (endpoint
	// groups, edge groups, tag assignment, migration all do) can register a
	// second one under a name of its own choosing without Estate or Server
	// growing a new field for every domain that needs more than one.
	Environments map[string]int `json:"environments,omitempty"`
}

// Environment returns the numeric endpoint id registered under name on this
// server, and whether one was found. name is whatever
// cmd/provision/main.go's provisionServer or runKubernetes called
// WithEnvironment with — EnvironmentDocker and, on the Community Edition leg
// only, EnvironmentAgent today.
func (s Server) Environment(name string) (int, bool) {
	id, ok := s.Environments[name]
	return id, ok
}

// WithEnvironment returns s with one more environment identity recorded
// under name, leaving every existing entry — and every other field — intact.
//
// It returns a new Server rather than mutating the receiver, the same rule
// Estate.MergeKubernetes already follows for the identical reason: two
// callers holding a Server obtained before this call must not silently
// observe each other's addition through a map both happen to share. The
// underlying map is copied on every call rather than mutated in place, so
// the Server this was called on keeps reporting exactly the entries it had
// before.
func (s Server) WithEnvironment(name string, id int) Server {
	next := make(map[string]int, len(s.Environments)+1)
	for k, v := range s.Environments {
		next[k] = v
	}
	next[name] = id
	s.Environments = next
	return s
}

// GPU describes the graphics card the estate's Docker daemon can hand to a
// container, as discovered on the Docker host by up.sh.
//
// CDIDevice, not a flag: the estate does not merely record that a GPU exists,
// it records the exact device string a container has to ask for. That string
// is a CDI identifier ("nvidia.com/gpu=all") rather than a --gpus request,
// because the estate's dind is Alpine and the NVIDIA container toolkit a
// nested --gpus needs is glibc-only. Recording the identifier the estate
// actually offers keeps that decision in one place instead of duplicated into
// every test that wants a GPU.
type GPU struct {
	Name      string `json:"name,omitempty"`
	CDIDevice string `json:"cdi_device,omitempty"`
}

// Estate is everything a suite needs to reach a running world.
//
// It is serialised to a gitignored file because the thing that provisions the
// estate (a script, once) is not the thing that consumes it (a test process,
// many times). It carries an API key: never commit it, never print it.
type Estate struct {
	CE         Server `json:"ce"`
	EE         Server `json:"ee"`
	Kubernetes Server `json:"kubernetes"`

	// GPU is the graphics card the estate's Docker daemon can offer, empty on
	// a host without one. Suites skip rather than fail in that case, exactly
	// as they do without a Business Edition licence.
	//
	// This describes the COMPOSE leg's Docker host only — up.sh is the only
	// script that ever populates it (see cmd/provision/main.go's run). It is
	// deliberately not touched by the Kubernetes leg's own provisioning
	// (runKubernetes): the two legs can run on different Docker hosts (the
	// split-host combination README.md calls legitimate), so a GPU on one
	// says nothing about the other. KubernetesGPU below is that leg's own,
	// independent field.
	GPU GPU `json:"gpu,omitzero"`

	// KubernetesGPU reports whether the k3d node advertises nvidia.com/gpu —
	// the Kubernetes leg's own GPU capability, set by k3d-up.sh
	// (PORTAINER_E2E_K8S_GPU) once the device plugin's DaemonSet rollout has
	// succeeded. It is a bare bool, unlike GPU's Name/CDIDevice pair: the
	// Kubernetes leg has no CDI device string of its own to record — a pod
	// simply requests the nvidia.com/gpu resource — so there is nothing here
	// for a second struct field to add.
	//
	// A test asserting on the Kubernetes leg's own GPU (see
	// TestE2E_GPU_KubernetesNodeAdvertisesTheCard) must gate on this, not on
	// HasGPU(): gating on the compose leg's field skips a passing run when
	// only the Kubernetes leg's host has a card, and fails a legitimately
	// GPU-less Kubernetes leg when only the compose leg's host does.
	KubernetesGPU bool `json:"kubernetes_gpu,omitempty"`

	// SwarmServiceID is the id of the long-lived fixture Swarm service
	// (busybox sleep, named swarm_fixture_service_name in
	// test/e2e/scripts/lib.sh) that up.sh creates on the compose leg's own
	// dind once it has put that daemon into Swarm mode. Empty means either
	// Swarm could not be enabled on this host or the fixture service could
	// not be confirmed -- both optional, exactly like GPU, and a
	// Swarm-dependent suite (docker.service_image_status today) must skip
	// with a named reason rather than fail when this is empty.
	//
	// It is the service's real, Swarm-assigned alphanumeric id, read back
	// from `docker service inspect` rather than assigned by the script --
	// see docs/api-divergences.md's "The cheat this is written down to
	// forbid" for why a hand-labelled small integer would not actually prove
	// anything about ServiceImageStatus's string-typed serviceId.
	//
	// Like GPU, this describes the COMPOSE leg's dind only: both CE and EE
	// register their own "docker" environment against the SAME dind daemon
	// (dindDaemonURL in cmd/provision/main.go), so this one fixture service
	// is reachable through either server's own environment id.
	SwarmServiceID string `json:"swarm_service_id,omitempty"`

	// EdgeEndpointID, EdgeAgentID and EdgeKey identify the edge environment
	// registered against EE, present only when a licence was available: edge
	// domains are Business Edition only. up.sh reads them back to start the
	// edge agent container once they exist, since they cannot be known
	// before the server issues them.
	//
	// EdgeEndpointID and EdgeAgentID are not interchangeable even though both
	// are "the id of the same environment": EdgeEndpointID is Portainer's
	// ordinary numeric database id (used in URLs like /api/endpoints/{id});
	// EdgeAgentID is the UUID the agent container needs as its EDGE_ID
	// environment variable. Passing the wrong one produces a real, distinct
	// failure — "invalid Edge identifier" — not a silent one; see
	// harness.EdgeCredentials.
	EdgeEndpointID int    `json:"edge_endpoint_id"`
	EdgeAgentID    string `json:"edge_agent_id"`
	EdgeKey        string `json:"edge_key"`
}

// HasCommunityEdition reports whether the compose Community Edition leg was
// provisioned — the CE twin of HasBusinessEdition and HasKubernetes below,
// requiring both fields for the same reason they do: a base URL with no API
// key names a server nothing can actually call.
//
// It exists because the Community Edition leg stopped being unconditional.
// The single Business Edition licence this project runs on permits one
// Portainer instance at a time, so `.github/workflows/e2e.yml` brings the
// compose legs and the Kubernetes leg up in two separate, sequential jobs,
// each with the estate to itself. The Kubernetes job's estate therefore
// carries a Kubernetes leg and no compose leg at all — a legitimately
// provisioned estate, not an interrupted one, and every caller that used to
// treat "CE" as always present has to ask instead.
func (e Estate) HasCommunityEdition() bool {
	return e.CE.BaseURL != "" && e.CE.Creds.APIKey != ""
}

// HasBusinessEdition reports whether the Business Edition leg was provisioned.
// It is false when no licence was available, and suites skip rather than fail
// in that case — a contributor without the licence must still be able to run
// the Community Edition suites.
func (e Estate) HasBusinessEdition() bool {
	return e.EE.BaseURL != "" && e.EE.Creds.APIKey != ""
}

// HasKubernetes reports whether the k3d leg was provisioned.
func (e Estate) HasKubernetes() bool {
	return e.Kubernetes.BaseURL != "" && e.Kubernetes.Creds.APIKey != ""
}

// HasGPU reports whether the COMPOSE leg's Docker daemon can hand a real GPU
// to a container. Both fields are required: a name without a device string
// names hardware no container can request. See GPU's own doc for why this is
// scoped to the compose leg specifically, and HasKubernetesGPU for the
// Kubernetes leg's own, independent field.
func (e Estate) HasGPU() bool {
	return e.GPU.Name != "" && e.GPU.CDIDevice != ""
}

// HasSwarm reports whether the compose leg's Docker daemon has Swarm mode
// enabled with a confirmed fixture service running on it, as set up by
// up.sh's swarm_init/swarm_fixture_service_id (test/e2e/scripts/lib.sh). It
// is false whenever that setup could not be completed -- Swarm mode refused,
// or the fixture service could not be created or confirmed -- and a
// Swarm-dependent suite must skip with a named reason in that case rather
// than fail, exactly as HasGPU's callers do without a card.
func (e Estate) HasSwarm() bool {
	return e.SwarmServiceID != ""
}

// HasKubernetesGPU reports whether the k3d node advertises nvidia.com/gpu.
// See KubernetesGPU's own doc for why this is a separate field from GPU/
// HasGPU rather than the same one reused: the two legs can run on different
// Docker hosts.
func (e Estate) HasKubernetesGPU() bool {
	return e.KubernetesGPU
}

// Leg is one provisioned server in an Estate, named the way this harness's
// own tests and cleanup routines already address it ("CE", "EE" or
// "Kubernetes"), paired with the Server it identifies.
type Leg struct {
	Name   string
	Server Server
}

// Legs returns every server this Estate actually provisioned: the Community
// Edition leg always, the Business Edition and Kubernetes legs only when
// HasBusinessEdition and HasKubernetes respectively report one was
// provisioned. Order is fixed (CE, EE, Kubernetes) so iteration is
// deterministic.
//
// This is the single derivation of the edition/platform axis every suite
// used to hardcode as a literal []string{"CE", "EE"} — or, worse,
// structurally, as a switch or a two-entry map built by hand — none of which
// knew about the Kubernetes leg without a third case added to every one of
// them by hand. A caller that needs "every edition this estate provisions"
// ranges over this instead of repeating that list a P3 domain would
// otherwise have copied roughly 88 more times.
func (e Estate) Legs() []Leg {
	var legs []Leg
	// CE is derived like the other two rather than prepended unconditionally.
	// It was unconditional while every estate began with `make e2e-up`; the
	// CI split described on HasCommunityEdition made a Kubernetes-only estate
	// a normal shape, and an unconditional entry here would have handed every
	// caller a leg whose BaseURL and API key are both empty — a client that
	// builds and then fails on the first request, instead of a leg the caller
	// can see is absent.
	if e.HasCommunityEdition() {
		legs = append(legs, Leg{Name: "CE", Server: e.CE})
	}
	if e.HasBusinessEdition() {
		legs = append(legs, Leg{Name: "EE", Server: e.EE})
	}
	if e.HasKubernetes() {
		legs = append(legs, Leg{Name: "Kubernetes", Server: e.Kubernetes})
	}
	return legs
}

// MergeKubernetes returns e with its Kubernetes leg set to k8s, leaving every
// field the compose legs already wrote untouched.
//
// The Kubernetes leg is provisioned by a separate process invocation
// (k3d-up.sh, after up.sh has already written CE and, when licensed, EE) into
// an estate file that already exists. Replacing e outright with a fresh
// Estate carrying only the Kubernetes leg — the mistake this guards against —
// would discard the compose legs the moment the Kubernetes leg is added,
// leaving the file looking freshly provisioned while every earlier server it
// named is gone.
func (e Estate) MergeKubernetes(k8s Server) Estate {
	merged := e
	merged.Kubernetes = k8s
	return merged
}

// SaveTo writes the estate atomically, so a reader can never observe a
// half-written file, and with owner-only permissions, because it holds keys.
func (e Estate) SaveTo(path string) error {
	encoded, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("encode estate: %w", err)
	}
	return writeOwnerOnlyAtomic(path, "estate", encoded)
}

// WriteEdgeEnv writes path as a KEY=value file docker compose reads natively
// via --env-file: EDGE_ID, EDGE_KEY and EDGE_ENDPOINT_ID. It carries the same
// enrolment secret the estate does, so it is written through the same
// atomic, owner-only path — see writeOwnerOnlyAtomic — and belongs next to
// the estate in .gitignore.
func WriteEdgeEnv(path, edgeID, edgeKey string, endpointID int) error {
	content := fmt.Sprintf("EDGE_ID=%s\nEDGE_KEY=%s\nEDGE_ENDPOINT_ID=%d\n", edgeID, edgeKey, endpointID)
	return writeOwnerOnlyAtomic(path, "edge environment", []byte(content))
}

// RemoveEdgeEnv deletes the derived edge environment file at path, if
// present. The provisioner calls this unconditionally whenever a run does not
// provision an edge environment: a file left over from an earlier run would
// otherwise silently start an agent enrolled against a server that no longer
// exists.
func RemoveEdgeEnv(path string) error {
	cleaned := filepath.Clean(path)
	if err := rejectEscapingPath("edge environment", path, cleaned); err != nil {
		return err
	}
	if err := os.Remove(cleaned); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove edge environment file %s: %w", cleaned, err)
	}
	return nil
}

// SyncEdgeEnv writes or removes the derived edge environment file at path
// depending on whether e carries an edge environment: written when both
// EdgeAgentID and EdgeKey are present, removed otherwise.
//
// This is the stale-file guard by itself, factored out so it can be tested
// without a running estate: a run that provisions no edge environment this
// time must not leave standing a file written by an earlier run, since
// up.sh's second compose pass starts an agent from whatever this file
// contains, whether or not it still describes anything real.
func SyncEdgeEnv(e Estate, path string) error {
	if e.EdgeAgentID != "" && e.EdgeKey != "" {
		return WriteEdgeEnv(path, e.EdgeAgentID, e.EdgeKey, e.EdgeEndpointID)
	}
	return RemoveEdgeEnv(path)
}

// writeOwnerOnlyAtomic writes data to path atomically — a reader can never
// observe a half-written file — with owner-only (0600) permissions, since
// every file written this way (the estate, the derived edge environment
// file) carries a secret. label names what is being written in wrapped
// errors, so a failure reads "write estate" or "write edge environment"
// rather than a generic message that could be either.
func writeOwnerOnlyAtomic(path, label string, data []byte) error {
	cleaned := filepath.Clean(path)
	if err := rejectEscapingPath(label, path, cleaned); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(cleaned), ".portainer-e2e-*")
	if err != nil {
		return fmt.Errorf("create temporary %s file: %w", label, err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	// os.CreateTemp already documents 0600 as the mode it creates with; this
	// Chmod restates that guarantee rather than establishing it, so a reader
	// should not take it as the reason TestEstate_SaveTo_IsOwnerOnly passes.
	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict %s permissions: %w", label, err)
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("write %s: %w", label, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", label, err)
	}
	if err := os.Rename(tmp.Name(), cleaned); err != nil {
		return fmt.Errorf("install %s: %w", label, err)
	}
	return nil
}

// LoadEstate reads an estate written by SaveTo.
//
// path is cleaned and validated before it ever reaches os.ReadFile: gosec's
// G304 flags exactly this call with a caller-controlled path, and the global
// constraints forbid silencing it with a suppression. Resolving the taint
// instead of hiding the warning is the fix the linter is asking for.
func LoadEstate(path string) (Estate, error) {
	cleaned := filepath.Clean(path)
	if err := rejectEscapingPath("estate", path, cleaned); err != nil {
		return Estate{}, err
	}
	raw, err := os.ReadFile(cleaned)
	if err != nil {
		return Estate{}, fmt.Errorf("read estate %s: %w", cleaned, err)
	}
	var e Estate
	if err := json.Unmarshal(raw, &e); err != nil {
		return Estate{}, fmt.Errorf("decode estate %s: %w", cleaned, err)
	}
	// "No leg at all", not "no Community Edition leg". The stricter rule this
	// replaces predates the CI split described on HasCommunityEdition, which
	// made a Kubernetes-only estate a legitimate shape: under the old rule
	// k3d-down.sh's own licence release (releaseKubernetesLicence, which
	// loads the estate before it can name the server to release against)
	// read that shape as a broken estate, reported "nothing to release" and
	// returned successfully — stranding the Business Edition licence on a
	// server `k3d cluster delete` was about to make unreachable for good.
	//
	// What the old rule was actually protecting is kept: an estate naming no
	// server at all is still refused, so an interrupted provisioning run, or
	// a suite pointed at an empty file, still fails loudly instead of
	// passing by measuring nothing.
	if len(e.Legs()) == 0 {
		return Estate{}, fmt.Errorf(
			"estate %s provisions no server at all (no Community Edition, Business Edition or Kubernetes leg): was provisioning interrupted?",
			cleaned)
	}
	return e, nil
}

// ReadTrustedFile reads path the same way LoadEstate does: cleaned and
// validated before it ever reaches os.ReadFile, so gosec's G304 has nothing
// to flag, rather than being told to look away from a caller-controlled path.
//
// "Trusted" names what this is not for: path is expected to come from this
// harness's own scripts (a temp file k3d-up.sh wrote, for instance), not from
// anything an adversary controls. It is a general-purpose read, unlike
// LoadEstate, which additionally knows the shape of an Estate and refuses one
// that provisions no server at all.
func ReadTrustedFile(path string) ([]byte, error) {
	cleaned := filepath.Clean(path)
	if err := rejectEscapingPath("trusted file", path, cleaned); err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", cleaned, err)
	}
	return raw, nil
}

// rejectEscapingPath rejects the one shape that would let a caller escape an
// intended directory: a relative path that climbs above its starting point
// once lexically resolved. label names the file being validated (for
// example "estate", "edge environment", or "trusted file") so the resulting
// error identifies which of the three files this validator guards actually
// failed, rather than always naming the estate regardless of which one an
// operator misconfigured.
//
// An absolute path is trusted as given: every caller in this codebase (the
// provisioner, its tests, and Task 6's TestMain reading EstateFileEnv) passes
// either an absolute path or one rooted at the current directory, never one
// meant to be sandboxed beneath it.
func rejectEscapingPath(label, original, cleaned string) error {
	if original == "" {
		return fmt.Errorf("%s path is empty", label)
	}
	if !filepath.IsAbs(cleaned) && strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("%s path %q escapes its starting directory", label, original)
	}
	return nil
}
