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
	cmd := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", "source "+lib+"\n"+script)
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
	cmd := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c",
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

// TestUnit_DockerHostMarker_NamedLegGetsItsOwnFile pins the shape of a named
// leg's marker path: a suffix distinct from the shared, no-argument default,
// so the compose leg's own marker (.docker-host) is never the file a named
// leg such as "kubernetes" writes to.
func TestUnit_DockerHostMarker_NamedLegGetsItsOwnFile(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, `unset PORTAINER_E2E_DOCKER_HOST_FILE; docker_host_marker kubernetes`), "\n")
	if !strings.HasSuffix(got, "/.docker-host-kubernetes") {
		t.Errorf("docker_host_marker kubernetes = %q, want suffix %q", got, "/.docker-host-kubernetes")
	}
}

// TestUnit_DockerHostMarker_NamedLegIgnoresTheOverrideEnvVar pins the
// deliberate asymmetry documented in lib.sh: PORTAINER_E2E_DOCKER_HOST_FILE
// redirects only the default (no-argument) marker. A test, or a future
// caller, pointing that override at a scratch file must not also divert a
// named leg's marker there — that would silently collapse every leg back
// onto one shared file, the exact problem the leg argument exists to avoid.
func TestUnit_DockerHostMarker_NamedLegIgnoresTheOverrideEnvVar(t *testing.T) {
	t.Parallel()
	override := filepath.Join(t.TempDir(), "wrong-file")
	env := "export PORTAINER_E2E_DOCKER_HOST_FILE=" + override + "\n"
	got := strings.TrimRight(sourceLib(t, env+`docker_host_marker kubernetes`), "\n")
	if got == override {
		t.Errorf("docker_host_marker kubernetes honoured PORTAINER_E2E_DOCKER_HOST_FILE; got %q", got)
	}
	if !strings.HasSuffix(got, "/.docker-host-kubernetes") {
		t.Errorf("docker_host_marker kubernetes = %q, want suffix %q", got, "/.docker-host-kubernetes")
	}
}

// TestUnit_DockerHostMarker_LegsAreIndependent is the coverage the review
// asked for by name: the compose leg's marker and a named leg's marker,
// recorded in the same directory, must be two different files, and clearing
// either one's record must leave the other's destination untouched. Without
// this, a single shared marker would let `make e2e-up-remote` (compose) and
// a same-machine Kubernetes leg silently clobber each other's recorded
// destination the moment the two legs are not colocated.
func TestUnit_DockerHostMarker_LegsAreIndependent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cd := "cd " + dir + "\n"

	sourceLib(t, cd+`record_docker_host "truenas"`)
	sourceLib(t, cd+`record_docker_host "k3d-host" "kubernetes"`)

	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host`), "\n"); got != "truenas" {
		t.Errorf("recorded_docker_host (compose leg) = %q, want %q", got, "truenas")
	}
	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host kubernetes`), "\n"); got != "k3d-host" {
		t.Errorf("recorded_docker_host kubernetes = %q, want %q", got, "k3d-host")
	}

	// Clearing the Kubernetes leg's marker must not touch the compose leg's.
	sourceLib(t, cd+`record_docker_host "" "kubernetes"`)
	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host kubernetes`), "\n"); got != "" {
		t.Errorf("recorded_docker_host kubernetes after clearing it = %q, want empty", got)
	}
	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host`), "\n"); got != "truenas" {
		t.Errorf("recorded_docker_host (compose leg) after clearing the kubernetes leg = %q, want %q", got, "truenas")
	}

	// And the reverse: clearing the compose leg's marker must not touch a
	// still-recorded named leg's.
	sourceLib(t, cd+`record_docker_host "k3d-host" "kubernetes"`)
	sourceLib(t, cd+`record_docker_host ""`)
	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host`), "\n"); got != "" {
		t.Errorf("recorded_docker_host (compose leg) after clearing it = %q, want empty", got)
	}
	if got := strings.TrimRight(sourceLib(t, cd+`recorded_docker_host kubernetes`), "\n"); got != "k3d-host" {
		t.Errorf("recorded_docker_host kubernetes after clearing the compose leg = %q, want %q", got, "k3d-host")
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
	// It also makes the nested "bash -c" inside on_docker_host's local branch
	// unresolvable, which fails on_docker_host outright rather than merely
	// failing "command -v nvidia-smi" inside it — both converge on the same
	// "if !" capture in detect_gpu_name, so either way the result is empty.
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

// TestUnit_DetectGPUName_DoesNotDiscardAGPUWhenTheQueryOutlivesTheFirstLine
// guards a regression this file's history already fixed once: a version of
// detect_gpu_name that piped "nvidia-smi ... | head -n1" with pipefail set
// discarded a real GPU's name almost every time on a multi-GPU host. head
// reads its first line and exits, closing the pipe; if nvidia-smi is still
// writing its next line at that moment it dies with SIGPIPE (141); pipefail
// promotes that 141 to the capture's exit status, and "if !" reads it as
// failure, discarding a raw value that was already correct.
//
// TestUnit_DetectGPUName_ReportsTheFirstGPUsName's stub writes both lines in
// one printf with no gap between them, so head is never in a position to
// close the pipe while nvidia-smi still has something left to write — that
// stub cannot reproduce the bug it looks like it should catch, and did not:
// reintroducing the pipefail+head form passed 20/20 against it. This stub
// pauses between the two writes long enough to force the collision
// deterministically instead.
func TestUnit_DetectGPUName_DoesNotDiscardAGPUWhenTheQueryOutlivesTheFirstLine(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "nvidia-smi")
	body := "#!/usr/bin/env bash\nprintf 'NVIDIA GeForce RTX 4060\\n'\nsleep 0.05\nprintf 'NVIDIA GeForce RTX 3090\\n'\n"
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
//
// The third check pins the literal "${PORTAINER_E2E_CDI_SPEC:?" guard form,
// not just the bare variable name: a bare "${PORTAINER_E2E_CDI_SPEC}" also
// contains the substring "PORTAINER_E2E_CDI_SPEC" and would have passed a
// check that only looked for that, silently losing the "must fail loudly
// rather than mount an empty path" behaviour the brief names by name.
func TestUnit_ComposeGPUOverride_GivesTheDindTheGPUAndTheStrippedSpec(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../docker-compose.gpu.yml")
	if err != nil {
		t.Fatalf("reading the GPU override: %v", err)
	}
	for _, want := range []string{
		"gpus: all",
		"/etc/cdi/nvidia.yaml:ro",
		"${PORTAINER_E2E_CDI_SPEC:?",
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
// gpu_cdi_spec returns before ever reaching strip_cdi_hooks on this path
// (the "if ! raw=$(...)" capture fails and returns early — either because
// "command -v nvidia-ctk" genuinely fails, or, with a PATH this empty,
// because the nested "bash -c" inside on_docker_host's local branch is
// itself unresolvable; both converge on the same capture failing), so a PATH
// containing nothing at all is safe here: unlike an earlier version of this
// function, awk is never invoked, so there is nothing to symlink onto PATH
// the way TestUnit_DetectGPUName_ReportsNothingWhenTheHostHasNoNvidiaSmi
// does for nvidia-smi's absence.
func TestUnit_GPUCDISpec_ReportsNothingWhenTheHostHasNoNvidiaCtk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
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

// TestUnit_GPUCDISpec_DiscardsAPartialSpecWhenNvidiaCtkFailsMidGeneration
// stubs nvidia-ctk to print a well-formed-looking but truncated document and
// then exit non-zero — the shape of a generator that dies partway through.
// An earlier version of gpu_cdi_spec forwarded that truncated document as if
// it were complete: "|| true" discards a failing command's exit status, not
// whatever it had already written to stdout. Task 4 mounts this output into
// the dind and only checks the file is non-empty, so a truncated document
// would pass that check and corrupt every nested GPU container silently.
// Getting nothing here is the only correct outcome.
func TestUnit_GPUCDISpec_DiscardsAPartialSpecWhenNvidiaCtkFailsMidGeneration(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "nvidia-ctk")
	body := "#!/usr/bin/env bash\ncat <<'CDIEOF'\n" +
		"cdiVersion: 0.7.0\n" +
		"kind: nvidia.com/gpu\n" +
		"containerEdits:\n" +
		"    deviceNodes:\n" +
		"        - path: /dev/nvidiactl\n" +
		"CDIEOF\n" +
		"exit 1\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing failing nvidia-ctk stub: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH gpu_cdi_spec ""`), "\n")
	if got != "" {
		t.Errorf("gpu_cdi_spec with a mid-generation failure = %q, want empty", got)
	}
}

// TestUnit_DetectGPUName_DiscardsAPartialNameWhenNvidiaSmiFailsMidQuery mirrors
// the gpu_cdi_spec case at the level that matters here: nvidia-smi prints a
// name and then exits non-zero. Without pipefail inside the command string,
// "nvidia-smi ... | head -n1" reports only head's exit status, so this
// failure would otherwise go unnoticed and the name would still be returned.
func TestUnit_DetectGPUName_DiscardsAPartialNameWhenNvidiaSmiFailsMidQuery(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stub := filepath.Join(dir, "nvidia-smi")
	body := "#!/usr/bin/env bash\nprintf 'NVIDIA GeForce RTX 4060\\n'\nexit 1\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing failing nvidia-smi stub: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH detect_gpu_name ""`), "\n")
	if got != "" {
		t.Errorf("detect_gpu_name with a mid-query failure = %q, want empty", got)
	}
}

// TestUnit_GPUCDISpec_ReturnsEmptyWhenStripCDIHooksItselfFails guards the
// last path in either GPU function that could still kill a caller.
// strip_cdi_hooks's only dependency is awk; an earlier version of
// gpu_cdi_spec ran "printf ... | strip_cdi_hooks" bare, so a broken awk's
// exit status escaped as gpu_cdi_spec's own, and under a caller's
// set -euo pipefail that terminates the whole script. Verified directly:
// shadowing awk with a stub that exits 127, with nvidia-ctk otherwise
// succeeding, made a statement placed right after the call never run.
func TestUnit_GPUCDISpec_ReturnsEmptyWhenStripCDIHooksItselfFails(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	ctkStub := filepath.Join(dir, "nvidia-ctk")
	ctkBody := "#!/usr/bin/env bash\ncat <<'CDIEOF'\n" +
		"cdiVersion: 0.7.0\n" +
		"kind: nvidia.com/gpu\n" +
		"containerEdits:\n" +
		"    deviceNodes:\n" +
		"        - path: /dev/nvidiactl\n" +
		"CDIEOF\n"
	if err := os.WriteFile(ctkStub, []byte(ctkBody), 0o700); err != nil {
		t.Fatalf("writing nvidia-ctk stub: %v", err)
	}
	awkStub := filepath.Join(dir, "awk")
	if err := os.WriteFile(awkStub, []byte("#!/usr/bin/env bash\nexit 127\n"), 0o700); err != nil {
		t.Fatalf("writing failing awk stub: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH gpu_cdi_spec ""`), "\n")
	if got != "" {
		t.Errorf("gpu_cdi_spec with a failing awk = %q, want empty", got)
	}
}
