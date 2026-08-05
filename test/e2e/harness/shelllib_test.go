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
	root := writeEnv(t, "NOT_PORTAINER_LICENSE=wrong\nPORTAINER_LICENSE=\"lic-123\"\nPORTAINER_E2E_DOCKER_SSH=truenas\nOTHER=nope\nQUOTED_OTHER=\"quoted-value\"\n")
	for _, tc := range []struct{ name, key, want string }{
		// Deliberately a different key from the anchor case below: this row
		// and that one used to both assert on PORTAINER_LICENSE against the
		// same fixture, so they could only ever pass or fail together and
		// neither one's intent was pinned down on its own.
		{"quoted value has its quotes stripped", "QUOTED_OTHER", "quoted-value"},
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

// TestUnit_DockerHostMarker_DefaultsUnderCurrentDirectory pins the branch
// every other marker test overrides away: with
// PORTAINER_E2E_DOCKER_HOST_FILE unset, docker_host_marker must fall back to
// a path under the current directory rather than, say, an empty string or an
// absolute constant that ignores where the scripts are run from.
func TestUnit_DockerHostMarker_DefaultsUnderCurrentDirectory(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, `unset PORTAINER_E2E_DOCKER_HOST_FILE; docker_host_marker`), "\n")
	if !strings.HasSuffix(got, "/.docker-host") {
		t.Errorf("docker_host_marker with no override = %q, want suffix %q", got, "/.docker-host")
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

// TestUnit_WriteToDockerHost_EmptyDestinationWritesLocally proves the local
// branch actually writes the file's real content, not merely that it exits
// zero: a `cat > "$path"` swapped for `cat > /dev/null` would still succeed
// silently.
func TestUnit_WriteToDockerHost_EmptyDestinationWritesLocally(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "output.txt")
	sourceLib(t, `printf '%s' 'hello-content' | write_to_docker_host "" "`+path+`"`)
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading file written by write_to_docker_host: %v", err)
	}
	if string(got) != "hello-content" {
		t.Errorf("write_to_docker_host wrote %q, want %q", string(got), "hello-content")
	}
}

// TestUnit_WriteToDockerHost_NonEmptyDestinationInvokesSSH mirrors
// TestUnit_OnDockerHost_NonEmptyDestinationInvokesSSH's stub-ssh-on-PATH
// technique. It asserts both the destination and the remote path appear in
// the invocation: checking only that ssh ran would miss a function that
// invoked ssh but forwarded the wrong remote path.
func TestUnit_WriteToDockerHost_NonEmptyDestinationInvokesSSH(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "ssh")
	if err := os.WriteFile(stub, []byte("#!/usr/bin/env bash\necho \"ssh-called dest=$*\"\n"), 0o700); err != nil {
		t.Fatalf("writing ssh stub: %v", err)
	}
	got := sourceLib(t, `echo ignored | PATH=`+dir+`:$PATH write_to_docker_host "somehost" "/remote/cdi.yaml"`)
	if !strings.Contains(got, "ssh-called") {
		t.Fatalf("write_to_docker_host with a destination did not invoke ssh; got %q", got)
	}
	if !strings.Contains(got, "somehost") {
		t.Errorf("ssh was invoked without the destination; got %q", got)
	}
	if !strings.Contains(got, "/remote/cdi.yaml") {
		t.Errorf("ssh was invoked without the remote path; got %q", got)
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

// TestUnit_DetectGPUName_ReportsNothingWhenTheHostHasNoNvidiaSmi proves the
// GPU-less path yields empty rather than an error: a developer on a laptop
// must still be able to bring the estate up, with the GPU suites skipping.
func TestUnit_DetectGPUName_ReportsNothingWhenTheHostHasNoNvidiaSmi(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// A PATH containing only this empty directory guarantees nvidia-smi is
	// absent regardless of what the machine running the test actually has.
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+` detect_gpu_name ""`), "\n")
	if got != "" {
		t.Errorf("detect_gpu_name on a host without nvidia-smi = %q, want empty", got)
	}
}

// TestUnit_DetectGPUName_ReportsTheFirstGPUsName stubs nvidia-smi with the
// exact output shape the real one produces for
// --query-gpu=name --format=csv,noheader, including the trailing newline and
// the multi-GPU case: a two-GPU host must yield one name, not both joined.
func TestUnit_DetectGPUName_ReportsTheFirstGPUsName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "nvidia-smi")
	body := "#!/usr/bin/env bash\nprintf 'NVIDIA GeForce RTX 4060\\nNVIDIA GeForce RTX 3090\\n'\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing nvidia-smi stub: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH detect_gpu_name ""`), "\n")
	if got != "NVIDIA GeForce RTX 4060" {
		t.Errorf("detect_gpu_name = %q, want %q", got, "NVIDIA GeForce RTX 4060")
	}
}

// TestUnit_ComposeGPUOverride_GivesTheDindTheGPUAndTheStrippedSpec reads the
// override file as text rather than parsing YAML: the point is that these
// four decisions are present and reviewable, and a YAML round-trip would not
// make the assertion stronger.
func TestUnit_ComposeGPUOverride_GivesTheDindTheGPUAndTheStrippedSpec(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../docker-compose.gpu.yml")
	if err != nil {
		t.Fatalf("reading the GPU override: %v", err)
	}
	for _, want := range []string{
		"gpus: all",
		"/etc/cdi/nvidia.yaml:ro",
		"PORTAINER_E2E_CDI_SPEC",
		"docker:",
	} {
		if !strings.Contains(string(data), want) {
			t.Errorf("docker-compose.gpu.yml is missing %q", want)
		}
	}
}

// TestUnit_CDIDeviceID_ReturnsTheConstantNvidiaGPUAllDevice pins the literal
// value: the compose override and up.sh both need the exact same string, and
// a typo here would silently make the estate ask the wrong daemon for a
// device that does not exist.
func TestUnit_CDIDeviceID_ReturnsTheConstantNvidiaGPUAllDevice(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, `cdi_device_id`), "\n")
	if got != "nvidia.com/gpu=all" {
		t.Errorf("cdi_device_id = %q, want %q", got, "nvidia.com/gpu=all")
	}
}

// TestUnit_GPUCDISpec_ReportsNothingWhenTheHostHasNoNvidiaCtk mirrors
// detect_gpu_name's GPU-less path: a host without the NVIDIA container
// toolkit must yield empty, not an error, so bringing the estate up without a
// GPU still works.
//
// gpu_cdi_spec pipes unconditionally through strip_cdi_hooks, even on this
// empty path, so a PATH containing nothing at all (the trick
// TestUnit_DetectGPUName_ReportsNothingWhenTheHostHasNoNvidiaSmi uses) would
// make awk itself unresolvable and fail the script for the wrong reason.
// Symlinking only awk into an otherwise empty directory keeps that guarantee
// — nvidia-ctk is absent regardless of what the machine running the test
// actually has — without breaking the pipe.
func TestUnit_GPUCDISpec_ReportsNothingWhenTheHostHasNoNvidiaCtk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	awkPath, err := exec.LookPath("awk")
	if err != nil {
		t.Fatalf("locating awk on this machine: %v", err)
	}
	if err := os.Symlink(awkPath, filepath.Join(dir, "awk")); err != nil {
		t.Fatalf("linking awk into the test PATH: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+` gpu_cdi_spec ""`), "\n")
	if got != "" {
		t.Errorf("gpu_cdi_spec on a host without nvidia-ctk = %q, want empty", got)
	}
}

// TestUnit_GPUCDISpec_StripsTheHooksFromWhatNvidiaCtkGenerates stubs
// nvidia-ctk with the same hook-bearing shape used by
// TestUnit_StripCDIHooks_RemovesHookBlocksAndKeepsEverythingElse. Without this
// test, gpu_cdi_spec piping through strip_cdi_hooks is not exercised anywhere
// — TestUnit_ComposeGPUOverride only checks the compose file's text, and
// TestUnit_StripCDIHooks only checks the filter in isolation, so a version of
// gpu_cdi_spec that dropped the pipe entirely would have left every other
// test in this file green.
func TestUnit_GPUCDISpec_StripsTheHooksFromWhatNvidiaCtkGenerates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "nvidia-ctk")
	body := "#!/usr/bin/env bash\ncat <<'CDIEOF'\n" +
		"cdiVersion: 0.7.0\n" +
		"kind: nvidia.com/gpu\n" +
		"containerEdits:\n" +
		"    deviceNodes:\n" +
		"        - path: /dev/nvidiactl\n" +
		"    hooks:\n" +
		"        - hookName: createContainer\n" +
		"          path: /usr/bin/nvidia-cdi-hook\n" +
		"          args:\n" +
		"            - nvidia-cdi-hook\n" +
		"            - update-ldcache\n" +
		"devices:\n" +
		"    - name: all\n" +
		"CDIEOF\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing nvidia-ctk stub: %v", err)
	}
	got := sourceLib(t, `PATH=`+dir+`:$PATH gpu_cdi_spec ""`)
	if strings.Contains(got, "hookName") || strings.Contains(got, "nvidia-cdi-hook") {
		t.Errorf("gpu_cdi_spec left hook content behind:\n%s", got)
	}
	for _, want := range []string{"cdiVersion: 0.7.0", "/dev/nvidiactl", "name: all"} {
		if !strings.Contains(got, want) {
			t.Errorf("gpu_cdi_spec dropped %q; result:\n%s", want, got)
		}
	}
}
