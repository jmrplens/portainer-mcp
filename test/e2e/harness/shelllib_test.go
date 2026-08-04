package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sourceLib runs script with test/e2e/scripts/lib.sh already sourced and
// returns its stdout. Anything the script writes to stderr is surfaced on
// failure, never mixed into the returned value: lib.sh's own diagnostics go
// to stderr and a helper that swallowed them would make a broken function
// look like an empty one.
func sourceLib(t *testing.T, script string) string {
	t.Helper()
	lib, err := filepath.Abs("../scripts/lib.sh")
	if err != nil {
		t.Fatalf("resolving lib.sh: %v", err)
	}
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", "source "+lib+"\n"+script)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash script failed: %v\nscript:\n%s\nstderr:\n%s", err, script, stderr.String())
	}
	return string(out)
}

// writeEnv creates a repository-root-shaped temporary directory holding a
// .env with the given contents, and returns its path.
func writeEnv(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(contents), 0o600); err != nil {
		t.Fatalf("writing .env: %v", err)
	}
	return dir
}

func TestUnit_ReadEnvVar_ReadsOnlyTheRequestedKey(t *testing.T) {
	t.Parallel()
	root := writeEnv(t, "NOT_PORTAINER_LICENSE=wrong\nPORTAINER_LICENSE=\"lic-123\"\nPORTAINER_E2E_DOCKER_SSH=truenas\nOTHER=nope\n")
	for _, tc := range []struct{ name, key, want string }{
		{"quoted value has its quotes stripped", "PORTAINER_LICENSE", "lic-123"},
		{"unquoted value is read verbatim", "PORTAINER_E2E_DOCKER_SSH", "truenas"},
		{"absent key yields empty", "PORTAINER_E2E_NOT_SET", ""},
		// A key that is a SUFFIX of another key must not match that other
		// key's line. This is the case the "^" anchor exists for, and it is
		// the only shape that fails without it: an unanchored grep matches
		// NOT_PORTAINER_LICENSE=wrong, and head -n1 then returns "wrong"
		// because the decoy sits first in the fixture. An earlier version of
		// this table asked for "PORTAINER_LICEN" instead, which the trailing
		// "=" already blocks with or without the anchor — a test that could
		// not fail.
		{"a key that another key ends with is not confused for it", "PORTAINER_LICENSE", "lic-123"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := strings.TrimRight(sourceLib(t, "read_env_var "+root+" "+tc.key), "\n")
			if got != tc.want {
				t.Errorf("read_env_var(%q) = %q, want %q", tc.key, got, tc.want)
			}
		})
	}
}

func TestUnit_ReadEnvVar_MissingEnvFileYieldsEmpty(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, "read_env_var "+t.TempDir()+" PORTAINER_LICENSE"), "\n")
	if got != "" {
		t.Errorf("read_env_var with no .env = %q, want empty", got)
	}
}

// TestUnit_DockerSSHDest_NeedsBothTheFlagAndTheKey is the guard the
// repository owner asked for by name: a destination sitting in .env must not
// be enough to send a plain `make e2e-up` to their NAS.
func TestUnit_DockerSSHDest_NeedsBothTheFlagAndTheKey(t *testing.T) {
	t.Parallel()
	withKey := writeEnv(t, "PORTAINER_E2E_DOCKER_SSH=truenas\n")
	noKey := writeEnv(t, "PORTAINER_LICENSE=lic\n")
	for _, tc := range []struct{ name, env, root, want string }{
		{"key present but no flag yields local", "", withKey, ""},
		{"flag explicitly off yields local", "PORTAINER_E2E_REMOTE=0", withKey, ""},
		{"flag and key together yield the destination", "PORTAINER_E2E_REMOTE=1", withKey, "truenas"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := strings.TrimRight(sourceLib(t, tc.env+" docker_ssh_dest "+tc.root), "\n")
			if got != tc.want {
				t.Errorf("docker_ssh_dest = %q, want %q", got, tc.want)
			}
		})
	}
	// The flag with no key must fail, not fall back: a silent local run would
	// skip every GPU suite and report green.
	cmd := exec.Command("bash", "-euo", "pipefail", "-c",
		"source ../scripts/lib.sh\nPORTAINER_E2E_REMOTE=1 docker_ssh_dest "+noKey)
	if err := cmd.Run(); err == nil {
		t.Error("docker_ssh_dest with the flag set and no key succeeded; want a non-zero exit")
	}
}

func TestUnit_DockerHostMarker_CarriesTheDestinationFromUpToDown(t *testing.T) {
	t.Parallel()
	marker := filepath.Join(t.TempDir(), ".docker-host")
	env := "export PORTAINER_E2E_DOCKER_HOST_FILE=" + marker + "\n"
	if got := strings.TrimRight(sourceLib(t, env+`record_docker_host "truenas"; recorded_docker_host`), "\n"); got != "truenas" {
		t.Errorf("recorded_docker_host after recording = %q, want %q", got, "truenas")
	}
	// Recording an empty destination must clear a marker left by an earlier
	// remote run, or the next local teardown would be aimed at the wrong host.
	if got := strings.TrimRight(sourceLib(t, env+`record_docker_host "truenas"; record_docker_host ""; recorded_docker_host`), "\n"); got != "" {
		t.Errorf("recorded_docker_host after recording empty = %q, want empty", got)
	}
	if got := strings.TrimRight(sourceLib(t, env+`recorded_docker_host`), "\n"); got != "" {
		t.Errorf("recorded_docker_host with no marker = %q, want empty", got)
	}
}

func TestUnit_OnDockerHost_EmptyDestinationRunsLocally(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, `on_docker_host "" 'echo ran-locally'`), "\n")
	if got != "ran-locally" {
		t.Errorf("on_docker_host with no destination = %q, want %q", got, "ran-locally")
	}
}

// TestUnit_OnDockerHost_NonEmptyDestinationInvokesSSH proves the remote branch
// is taken without needing a reachable host: a stub `ssh` earlier on PATH
// records that it was called. Asserting only the local branch would leave the
// branch that matters completely unexercised.
func TestUnit_OnDockerHost_NonEmptyDestinationInvokesSSH(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "ssh")
	if err := os.WriteFile(stub, []byte("#!/usr/bin/env bash\necho \"ssh-called dest=$*\"\n"), 0o700); err != nil {
		t.Fatalf("writing ssh stub: %v", err)
	}
	got := sourceLib(t, `PATH=`+dir+`:$PATH on_docker_host "somehost" 'echo unused'`)
	if !strings.Contains(got, "ssh-called") {
		t.Fatalf("on_docker_host with a destination did not invoke ssh; got %q", got)
	}
	if !strings.Contains(got, "somehost") {
		t.Errorf("ssh was invoked without the destination; got %q", got)
	}
}

// TestUnit_StripCDIHooks_RemovesHookBlocksAndKeepsEverythingElse uses the
// exact shape `nvidia-ctk cdi generate` emits: a hooks: sequence nested under
// containerEdits, followed by sibling keys at the same indentation that must
// survive.
func TestUnit_StripCDIHooks_RemovesHookBlocksAndKeepsEverythingElse(t *testing.T) {
	t.Parallel()
	in := `cdiVersion: 0.7.0
kind: nvidia.com/gpu
containerEdits:
    deviceNodes:
        - path: /dev/nvidiactl
    hooks:
        - hookName: createContainer
          path: /usr/bin/nvidia-cdi-hook
          args:
            - nvidia-cdi-hook
            - update-ldcache
        - hookName: createContainer
          path: /usr/bin/nvidia-cdi-hook
          args:
            - nvidia-cdi-hook
            - create-symlinks
    mounts:
        - hostPath: /usr/bin/nvidia-smi
          containerPath: /usr/bin/nvidia-smi
devices:
    - name: all
`
	got := sourceLib(t, "cat <<'CDIEOF' | strip_cdi_hooks\n"+in+"CDIEOF")
	if strings.Contains(got, "hookName") || strings.Contains(got, "nvidia-cdi-hook") {
		t.Errorf("strip_cdi_hooks left hook content behind:\n%s", got)
	}
	for _, want := range []string{"cdiVersion: 0.7.0", "/dev/nvidiactl", "hostPath: /usr/bin/nvidia-smi", "name: all"} {
		if !strings.Contains(got, want) {
			t.Errorf("strip_cdi_hooks dropped %q; result:\n%s", want, got)
		}
	}
}
