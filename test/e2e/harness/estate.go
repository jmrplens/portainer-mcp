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

// Server is one provisioned Portainer.
type Server struct {
	Edition string      `json:"edition"`
	BaseURL string      `json:"base_url"`
	Creds   Credentials `json:"credentials"`

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
	AgentID    int    `json:"agent_endpoint_id"`

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
	if e.CE.BaseURL == "" {
		return Estate{}, fmt.Errorf("estate %s has no Community Edition server: was provisioning interrupted?", cleaned)
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
// missing a Community Edition server.
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
