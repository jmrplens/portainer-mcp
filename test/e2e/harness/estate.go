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

// Server is one provisioned Portainer.
type Server struct {
	Edition string      `json:"edition"`
	BaseURL string      `json:"base_url"`
	Creds   Credentials `json:"credentials"`
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

// SaveTo writes the estate atomically, so a reader can never observe a
// half-written file, and with owner-only permissions, because it holds keys.
func (e Estate) SaveTo(path string) error {
	cleaned := filepath.Clean(path)
	if err := rejectEscapingPath(path, cleaned); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return fmt.Errorf("encode estate: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(cleaned), ".estate-*")
	if err != nil {
		return fmt.Errorf("create temporary estate file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()

	if err := tmp.Chmod(0o600); err != nil {
		return fmt.Errorf("restrict estate permissions: %w", err)
	}
	if _, err := tmp.Write(encoded); err != nil {
		return fmt.Errorf("write estate: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close estate: %w", err)
	}
	if err := os.Rename(tmp.Name(), cleaned); err != nil {
		return fmt.Errorf("install estate: %w", err)
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
	if err := rejectEscapingPath(path, cleaned); err != nil {
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

// rejectEscapingPath rejects the one shape that would let a caller escape an
// intended directory: a relative path that climbs above its starting point
// once lexically resolved.
//
// An absolute path is trusted as given: every caller in this codebase (the
// provisioner, its tests, and Task 6's TestMain reading EstateFileEnv) passes
// either an absolute path or one rooted at the current directory, never one
// meant to be sandboxed beneath it.
func rejectEscapingPath(original, cleaned string) error {
	if original == "" {
		return fmt.Errorf("estate path is empty")
	}
	if !filepath.IsAbs(cleaned) && strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("estate path %q escapes its starting directory", original)
	}
	return nil
}
