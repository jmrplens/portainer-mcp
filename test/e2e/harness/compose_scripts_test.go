package harness

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// This file runs the COMMITTED test/e2e/scripts/up.sh and down.sh against
// stub ssh/docker/go binaries, inside an isolated fake repository these tests
// fully control — never the real repository root's .env, and never any real
// network destination.
//
// It exists for the reason k3d_scripts_test.go's own top-of-file comment
// gives for the Kubernetes leg's scripts: a live review mutated up.sh:21's
// `ssh_dest=$(docker_ssh_dest "$repo_root")` to
// `ssh_dest=$(read_env_var "$repo_root" PORTAINER_E2E_DOCKER_SSH)` — which
// sends a plain `make e2e-up` to whatever host .env names, exactly the
// failure mode the repository owner named by name — and down.sh:20's
// `ssh_dest=$(recorded_docker_host)` to `ssh_dest=""`, and `go test ./...`,
// `go build`, `go vet` and `bash -n` all stayed green throughout: nothing
// below cmd/portainer-mcp or internal/... ever runs these scripts, and
// shelllib_test.go only exercises docker_ssh_dest and recorded_docker_host as
// bare functions, never the two scripts' own use of them. Both mutations are
// pinned here, red, before being reverted.

// composeFakeRepo is composeFakeRepo's own isolated repository shape, built
// exactly like k3dFakeRepo for the identical reason: a fresh copy of the
// CURRENTLY COMMITTED scripts, so a regression introduced into the real files
// is exercised the next time these tests run.
type composeFakeRepo struct {
	e2eDir  string // .../fakerepo/test/e2e — the scripts' own working directory
	binDir  string
	logFile string
}

// newComposeFakeRepo builds the fake repository. envContents lets a caller
// seed a specific .env shape (a destination present or absent) without every
// test needing its own copy of the boilerplate around it.
func newComposeFakeRepo(t *testing.T, envContents string) *composeFakeRepo {
	t.Helper()
	root := t.TempDir()
	e2eDir := filepath.Join(root, "fakerepo", "test", "e2e")
	scriptsDir := filepath.Join(e2eDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("creating fake scripts dir: %v", err)
	}
	for _, name := range []string{"up.sh", "down.sh", "lib.sh", "remote.sh"} {
		src, err := filepath.Abs(filepath.Join("..", "scripts", name))
		if err != nil {
			t.Fatalf("resolving %s: %v", name, err)
		}
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatalf("reading committed %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(scriptsDir, name), data, 0o755); err != nil {
			t.Fatalf("writing fake %s: %v", name, err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "fakerepo", ".env"), []byte(envContents), 0o644); err != nil {
		t.Fatalf("writing fake .env: %v", err)
	}

	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("creating fake bin dir: %v", err)
	}
	logFile := filepath.Join(root, "log.txt")
	writeStub := func(name, body string) {
		if err := os.WriteFile(filepath.Join(binDir, name), []byte(body), 0o755); err != nil {
			t.Fatalf("writing %s stub: %v", name, err)
		}
	}
	// docker: the only thing up.sh/down.sh ever call it for is `docker
	// compose ...` — logging the invocation together with DOCKER_HOST is
	// enough to prove which daemon a call was aimed at, exactly like
	// k3d_scripts_test.go's own docker stub does for the same purpose.
	writeStub("docker", `#!/usr/bin/env bash
echo "DOCKER_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>}" >> "$LOGFILE"
exit 0
`)
	// go: stands in for `go run ./harness/cmd/provision`. Never writes an
	// estate file or edge environment — these tests do not need either, only
	// that the provisioner was invoked, and up.sh's own "no edge endpoint
	// provisioned" branch handles a missing edge.env file gracefully.
	// PORTAINER_E2E_SWARM_SERVICE_ID is surfaced in the log the same way
	// k3d_scripts_test.go's own go stub surfaces PORTAINER_E2E_K8S_URL, so a
	// test can observe what up.sh actually passed through without needing a
	// real estate file.
	writeStub("go", `#!/usr/bin/env bash
echo "GO_CALL: $* | PORTAINER_E2E_SWARM_SERVICE_ID=${PORTAINER_E2E_SWARM_SERVICE_ID:-<unset>}" >> "$LOGFILE"
exit 0
`)
	// ssh: answers the GPU/CDI probes (nvidia-smi, nvidia-ctk) and the
	// leftover-CDI-file check (test -f) as absent/false — these tests are not
	// about the GPU path — and otherwise just logs and succeeds, the same
	// "reports 0 regardless" shape production ssh has (see remote.sh), which
	// is fine here: unlike the Kubernetes leg's tunnel_add_forward, up.sh only
	// ever calls tunnel_up/tunnel_down, neither of which polls for
	// confirmation.
	writeStub("ssh", `#!/usr/bin/env bash
echo "SSH_CALL: $*" >> "$LOGFILE"
case "$*" in
    *"nvidia-smi"*|*"nvidia-ctk"*|*"test -f"*)
        exit 1
        ;;
esac
exit 0
`)
	// No k3d/kubectl/helm on this PATH at all: down.sh's own
	// `command -v k3d` guard is what is supposed to make its Kubernetes-leg
	// probe a no-op on a machine that never installed it, and a fake
	// repository standing in for a compose-only checkout is exactly that
	// machine.

	return &composeFakeRepo{e2eDir: e2eDir, binDir: binDir, logFile: logFile}
}

// run executes one of the fake repo's own scripts (up.sh or down.sh) with the
// stub PATH prepended and the given extra environment entries, and returns
// the script's own combined stdout+stderr, its ssh/docker/go invocation log,
// and its exit code.
func (r *composeFakeRepo) run(t *testing.T, scriptName string, extraEnv ...string) (output, log string, exitCode int) {
	t.Helper()
	script := filepath.Join(r.e2eDir, "scripts", scriptName)
	cmd := exec.CommandContext(t.Context(), "bash", script)
	// PORTAINER_E2E_REMOTE="" is appended BEFORE extraEnv, deliberately: Go's
	// exec.Cmd keeps only the LAST occurrence of a duplicate key, so a test
	// that actually wants remote behaviour (extraEnv containing
	// "PORTAINER_E2E_REMOTE=1") still wins. Without this, a developer running
	// `PORTAINER_E2E_REMOTE=1 go test ./test/e2e/harness/...` in their own
	// shell would have that value flow into os.Environ() and silently reach
	// every "plain run" test below as if PORTAINER_E2E_REMOTE=1 had been
	// requested, even though CI never sets it and nothing here ever asked for
	// remote behaviour.
	env := append(os.Environ(), "PATH="+r.binDir+":"+os.Getenv("PATH"), "LOGFILE="+r.logFile, "PORTAINER_E2E_REMOTE=")
	env = append(env, extraEnv...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("running %s: %v\noutput:\n%s", scriptName, err, out)
		}
	}
	logData, readErr := os.ReadFile(r.logFile)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("reading invocation log: %v", readErr)
	}
	return string(out), string(logData), code
}

// marker reads the compose leg's own marker (up.sh/down.sh never touch a
// named leg's — that is the Kubernetes leg's own k3d_scripts_test.go's
// concern), the same shape k3dFakeRepo.marker uses for its own default case.
func (r *composeFakeRepo) marker() (string, bool) {
	data, err := os.ReadFile(filepath.Join(r.e2eDir, ".docker-host"))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(data), "\n"), true
}

// seedMarker writes the compose leg's marker directly, standing in for an
// earlier run's own record_docker_host call.
func (r *composeFakeRepo) seedMarker(t *testing.T, dest string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(r.e2eDir, ".docker-host"), []byte(dest+"\n"), 0o644); err != nil {
		t.Fatalf("seeding the compose marker: %v", err)
	}
}

const fakeRemoteHost = "fake-remote-host-invented-for-this-test.invalid"

// TestUnit_UpScript_PlainRunNeverTouchesTheHostNamedInEnv is the mutation-
// proof for up.sh:21. A destination sitting in .env, with no
// PORTAINER_E2E_REMOTE set (exactly what a plain `make e2e-up` gets), must
// leave the docker compose call untouched — local, no DOCKER_HOST — and must
// never invoke ssh at all. Replacing docker_ssh_dest with a bare
// read_env_var, as a live review did, makes this red: the destination sitting
// in .env would flow straight through with no flag required at all.
func TestUnit_UpScript_PlainRunNeverTouchesTheHostNamedInEnv(t *testing.T) {
	repo := newComposeFakeRepo(t, "PORTAINER_E2E_DOCKER_SSH="+fakeRemoteHost+"\n")

	output, log, code := repo.run(t, "up.sh")
	if code != 0 {
		t.Fatalf("up.sh (plain run) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if !strings.Contains(log, "DOCKER_CALL:") {
		t.Fatalf("up.sh never invoked docker compose; log:\n%s", log)
	}
	if strings.Contains(log, "DOCKER_HOST=ssh://") {
		t.Errorf("up.sh's docker compose call carried a remote DOCKER_HOST despite no PORTAINER_E2E_REMOTE; log:\n%s", log)
	}
	if !strings.Contains(log, "DOCKER_HOST=<unset>") {
		t.Errorf("up.sh's docker compose call did not target the local daemon; log:\n%s", log)
	}
	if strings.Contains(log, "SSH_CALL:") {
		t.Errorf("up.sh invoked ssh at all despite no PORTAINER_E2E_REMOTE; log:\n%s", log)
	}
	if dest, ok := repo.marker(); ok {
		t.Errorf("up.sh recorded a compose marker on a plain local run: (%q, %v)", dest, ok)
	}
}

// TestUnit_UpScript_RemoteFlagUsesSSHDockerHostAndRecordsTheMarker is the
// positive direction: PORTAINER_E2E_REMOTE=1 (what `make e2e-up-remote` sets)
// must actually reach the docker compose call as DOCKER_HOST=ssh://... and
// record the marker. Without this, the sibling test above could pass for the
// wrong reason — a version that deleted remote support outright would also
// never touch DOCKER_HOST or ssh.
func TestUnit_UpScript_RemoteFlagUsesSSHDockerHostAndRecordsTheMarker(t *testing.T) {
	repo := newComposeFakeRepo(t, "PORTAINER_E2E_DOCKER_SSH="+fakeRemoteHost+"\n")

	output, log, code := repo.run(t, "up.sh", "PORTAINER_E2E_REMOTE=1")
	if code != 0 {
		t.Fatalf("up.sh (remote) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if !strings.Contains(log, "DOCKER_HOST=ssh://"+fakeRemoteHost) {
		t.Errorf("up.sh's docker compose call did not target the remote daemon; log:\n%s", log)
	}
	if dest, ok := repo.marker(); !ok || dest != fakeRemoteHost {
		t.Errorf("compose marker = (%q, %v), want the remote destination recorded", dest, ok)
	}
}

// TestUnit_UpScript_RefusesAPlainRunWhenARemoteMarkerExists is I2's own
// regression test, at the level the review named: a `make e2e-up-remote`
// followed by a plain `make e2e-up` must refuse rather than silently orphan
// the still-recorded remote estate. Before this guard existed,
// record_docker_host's own unconditional "empty destination deletes the
// marker" rule meant this exact sequence deleted the ONLY record of where the
// remote estate, its Business Edition licence and its open ssh master
// actually are — and did so as a side effect of a run that otherwise
// completed and reported success.
func TestUnit_UpScript_RefusesAPlainRunWhenARemoteMarkerExists(t *testing.T) {
	repo := newComposeFakeRepo(t, "")
	repo.seedMarker(t, fakeRemoteHost)

	output, log, code := repo.run(t, "up.sh")
	if code == 0 {
		t.Fatalf("up.sh (plain run) succeeded despite an existing remote marker; output:\n%s\nlog:\n%s", output, log)
	}
	if !strings.Contains(output, "refusing to continue") {
		t.Errorf("up.sh did not report a refusal; output:\n%s", output)
	}
	if strings.Contains(log, "DOCKER_CALL:") {
		t.Errorf("up.sh brought the estate up despite refusing the host switch; log:\n%s", log)
	}
	if dest, ok := repo.marker(); !ok || dest != fakeRemoteHost {
		t.Errorf("compose marker changed after a refused run: (%q, %v), want it untouched at %q", dest, ok, fakeRemoteHost)
	}
}

// TestUnit_UpScript_RefusesSwitchingToADifferentRemoteHost is the same guard
// in its other direction: an existing remote marker and a NEW run naming a
// different remote host must also refuse, not silently redirect teardown to
// the new host while the old one's estate is still running unreachable.
func TestUnit_UpScript_RefusesSwitchingToADifferentRemoteHost(t *testing.T) {
	repo := newComposeFakeRepo(t, "PORTAINER_E2E_DOCKER_SSH="+fakeRemoteHost+"\n")
	const otherHost = "a-different-remote-host.invalid"
	repo.seedMarker(t, otherHost)

	output, log, code := repo.run(t, "up.sh", "PORTAINER_E2E_REMOTE=1")
	if code == 0 {
		t.Fatalf("up.sh (remote) succeeded despite an existing marker naming a different host; output:\n%s\nlog:\n%s", output, log)
	}
	if !strings.Contains(output, "refusing to continue") {
		t.Errorf("up.sh did not report a refusal; output:\n%s", output)
	}
	if strings.Contains(log, "DOCKER_CALL:") {
		t.Errorf("up.sh brought the estate up despite refusing the host switch; log:\n%s", log)
	}
	if dest, ok := repo.marker(); !ok || dest != otherHost {
		t.Errorf("compose marker changed after a refused run: (%q, %v), want it untouched at %q", dest, ok, otherHost)
	}
}

// TestUnit_UpScript_AllowsReRunningAgainstTheSameRecordedDestination proves
// the guard is not a blanket refusal: up.sh's own doc says it is idempotent
// ("running it twice replaces the estate rather than accumulating one"), and
// a second `make e2e-up-remote` against the SAME host an earlier run already
// recorded must succeed, not be mistaken for a host switch.
func TestUnit_UpScript_AllowsReRunningAgainstTheSameRecordedDestination(t *testing.T) {
	repo := newComposeFakeRepo(t, "PORTAINER_E2E_DOCKER_SSH="+fakeRemoteHost+"\n")
	repo.seedMarker(t, fakeRemoteHost)

	output, log, code := repo.run(t, "up.sh", "PORTAINER_E2E_REMOTE=1")
	if code != 0 {
		t.Fatalf("up.sh (remote, re-run against the same host) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if !strings.Contains(log, "DOCKER_HOST=ssh://"+fakeRemoteHost) {
		t.Errorf("up.sh's docker compose call did not target the remote daemon; log:\n%s", log)
	}
	if dest, ok := repo.marker(); !ok || dest != fakeRemoteHost {
		t.Errorf("compose marker = (%q, %v), want the remote destination still recorded", dest, ok)
	}
}

// TestUnit_DownScript_TearsDownAgainstTheRecordedMarker is the mutation-proof
// for down.sh:20. With a marker recorded by an earlier (simulated) `up`, a
// plain `make e2e-down` must tear down against THAT destination — the whole
// point of the marker existing at all — and must clear it afterward.
// Replacing `ssh_dest=$(recorded_docker_host)` with `ssh_dest=""`, as a live
// review did, makes this red: down.sh would tear down locally regardless of
// what the marker names.
func TestUnit_DownScript_TearsDownAgainstTheRecordedMarker(t *testing.T) {
	repo := newComposeFakeRepo(t, "")
	repo.seedMarker(t, fakeRemoteHost)

	output, log, code := repo.run(t, "down.sh")
	if code != 0 {
		t.Fatalf("down.sh exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if !strings.Contains(log, "DOCKER_HOST=ssh://"+fakeRemoteHost) {
		t.Errorf("down.sh's docker compose call did not target the recorded remote daemon; log:\n%s", log)
	}
	if strings.Contains(log, "DOCKER_HOST=<unset>") {
		t.Errorf("down.sh also (or instead) tore down the local daemon; log:\n%s", log)
	}
	if dest, ok := repo.marker(); ok {
		t.Errorf("down.sh left the compose marker behind: (%q, %v), want it cleared", dest, ok)
	}
}

// TestUnit_DownScript_KubernetesTeardownFailureDoesNotAbortComposeTeardown is
// the regression test for the Kubernetes-leg probe's own subshell in
// down.sh. That block runs under this script's own `set -e`, and a subshell
// that exits non-zero aborts the parent script exactly like any other failing
// command would — nothing about `( ... )` on its own protects the statements
// that follow it. Today every command inside k3d-down.sh that this probe can
// reach is individually guarded (`|| echo` on the licence release, `|| true`
// on `k3d cluster delete`), so the trigger is not reachable yet, but the
// coupling itself is the defect: one future unguarded command inside
// k3d-down.sh would silently turn into "the compose teardown below never
// runs", leaving the compose estate up on whatever host it lives on — the
// exact outcome this script's own header calls worse than a stranded
// licence. This test stands in for that future command with a stub
// k3d-down.sh that simply exits 1.
func TestUnit_DownScript_KubernetesTeardownFailureDoesNotAbortComposeTeardown(t *testing.T) {
	repo := newComposeFakeRepo(t, "")
	repo.seedMarker(t, fakeRemoteHost)

	// down.sh's own probe only calls ./scripts/k3d-down.sh when `command -v
	// k3d` succeeds AND `k3d cluster list -o json` reports a cluster named
	// exactly like the default E2E_K3D_CLUSTER ("portainer-mcp-e2e").
	if err := os.WriteFile(filepath.Join(repo.binDir, "k3d"), []byte(`#!/usr/bin/env bash
if [[ "$1" == "cluster" && "$2" == "list" ]]; then
    echo '[{"name":"portainer-mcp-e2e"}]'
    exit 0
fi
exit 0
`), 0o755); err != nil {
		t.Fatalf("writing k3d stub: %v", err)
	}
	// Stands in for a future, unguarded command inside the real
	// k3d-down.sh: this replacement always fails.
	if err := os.WriteFile(filepath.Join(repo.e2eDir, "scripts", "k3d-down.sh"), []byte("#!/usr/bin/env bash\nexit 1\n"), 0o755); err != nil {
		t.Fatalf("writing failing k3d-down.sh stub: %v", err)
	}

	output, log, code := repo.run(t, "down.sh")
	if code != 0 {
		t.Fatalf("down.sh exited %d despite the kubernetes teardown failure being handled; output:\n%s\nlog:\n%s", code, output, log)
	}
	if !strings.Contains(output, "warning: the kubernetes teardown failed") {
		t.Errorf("down.sh did not report the kubernetes teardown failure; output:\n%s", output)
	}
	if !strings.Contains(log, "DOCKER_CALL: compose -f docker-compose.yml") {
		t.Errorf("down.sh never reached the compose teardown after the kubernetes teardown failed; log:\n%s", log)
	}
	if dest, ok := repo.marker(); ok {
		t.Errorf("down.sh left the compose marker behind after the kubernetes teardown failed: (%q, %v), want it cleared", dest, ok)
	}
}

// TestUnit_DownScript_NoMarkerTearsDownLocally is the baseline (unmutated,
// no earlier remote run) direction: with no marker at all, down.sh must tear
// down the local daemon, exactly as it always has.
func TestUnit_DownScript_NoMarkerTearsDownLocally(t *testing.T) {
	repo := newComposeFakeRepo(t, "")

	output, log, code := repo.run(t, "down.sh")
	if code != 0 {
		t.Fatalf("down.sh exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if !strings.Contains(log, "DOCKER_HOST=<unset>") {
		t.Errorf("down.sh's docker compose call did not target the local daemon; log:\n%s", log)
	}
	if strings.Contains(log, "DOCKER_HOST=ssh://") {
		t.Errorf("down.sh's docker compose call targeted a remote daemon despite no recorded marker; log:\n%s", log)
	}
}

// singleQuoted extracts the content of the first '...'-quoted substring in s,
// failing the test if there is none. Both up.sh's spec-writing call and
// down.sh's teardown rm -f pass the CDI specification's path to a remote
// shell wrapped in single quotes (e.g. "cat > '$path'", "rm -f '$path'"), so
// this is what actually pulls the literal path each one used out of a logged
// ssh invocation.
func singleQuoted(t *testing.T, s string) string {
	t.Helper()
	m := regexp.MustCompile(`'([^']+)'`).FindStringSubmatch(s)
	if m == nil {
		t.Fatalf("no single-quoted substring found in %q", s)
	}
	return m[1]
}

// TestUnit_GPUBranch_UpAndDownAgreeOnTheSameCDISpecPath is C3's own coverage
// gap closed: no test in this file exercised up.sh's or down.sh's GPU branch
// at all, so a drift among the three places that have to agree on the CDI
// specification's path -- up.sh's own write, the PORTAINER_E2E_CDI_SPEC value
// it hands to `docker compose`, and down.sh's own `rm -f` during teardown --
// would leave every one of bash -n, shellcheck, go vet and every other test
// in this file green.
//
// Run with PORTAINER_E2E_REMOTE=1 so all three of those pass through the
// stub ssh (rather than a bare local `cat`/`rm`, which would leave nothing
// in the log to assert on): the stub logs its own exact command line,
// including the path quoted exactly as up.sh/down.sh pass it, making each
// literal directly observable instead of merely inferred from exit codes.
// The ssh and docker stubs used here report a GPU, a usable container
// toolkit generating a minimal but shape-valid CDI document, and success on
// every leftover/shape check (test -f, test -s, grep -q) up.sh and down.sh
// run over ssh -- the "everything reports a GPU" shape this test needs, as
// opposed to newComposeFakeRepo's own shared stubs, which deliberately
// report no GPU at all for every OTHER test in this file.
//
// The expected path itself is read out of lib.sh's own cdi_spec_path
// (sourceLib, from shelllib_test.go) rather than hard-coded a second time
// here: this test's job is to prove the three CALL SITES agree with each
// other and with that one shared definition, not to duplicate its value as
// a fourth place that could itself drift.
func TestUnit_GPUBranch_UpAndDownAgreeOnTheSameCDISpecPath(t *testing.T) {
	repo := newComposeFakeRepo(t, "PORTAINER_E2E_DOCKER_SSH="+fakeRemoteHost+"\n")

	if err := os.WriteFile(filepath.Join(repo.binDir, "ssh"), []byte(`#!/usr/bin/env bash
echo "SSH_CALL: $*" >> "$LOGFILE"
case "$*" in
    *"nvidia-smi"*)
        echo "NVIDIA GeForce RTX FAKE"
        exit 0
        ;;
    *"nvidia-ctk"*)
        printf 'cdiVersion: 0.7.0\nkind: nvidia.com/gpu\ndevices:\n    - name: all\n'
        exit 0
        ;;
    # write_to_docker_host's remote command ("cat > 'path'") is the one place
    # this stub is actually fed piped stdin (up.sh's own
    # "gpu_cdi_spec | write_to_docker_host" pipe) -- draining it here avoids
    # the writer taking a SIGPIPE from a reader that exited without ever
    # reading, which is exactly what happened before this branch existed.
    *"cat >"*)
        cat >/dev/null
        exit 0
        ;;
esac
exit 0
`), 0o755); err != nil {
		t.Fatalf("writing gpu-reporting ssh stub: %v", err)
	}
	// The shared docker stub only ever logs DOCKER_HOST; PORTAINER_E2E_CDI_SPEC
	// is exactly what up.sh's and down.sh's own `docker compose` calls carry
	// the CDI specification's path through, so this replacement surfaces it
	// too.
	if err := os.WriteFile(filepath.Join(repo.binDir, "docker"), []byte(`#!/usr/bin/env bash
echo "DOCKER_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>} | PORTAINER_E2E_CDI_SPEC=${PORTAINER_E2E_CDI_SPEC:-<unset>}" >> "$LOGFILE"
exit 0
`), 0o755); err != nil {
		t.Fatalf("writing cdi-spec-surfacing docker stub: %v", err)
	}

	upOutput, upLog, upCode := repo.run(t, "up.sh", "PORTAINER_E2E_REMOTE=1")
	if upCode != 0 {
		t.Fatalf("up.sh (gpu present) exited %d; output:\n%s\nlog:\n%s", upCode, upOutput, upLog)
	}
	if !strings.Contains(upOutput, "gpu detected on the docker host") {
		t.Fatalf("up.sh did not report a detected gpu; output:\n%s", upOutput)
	}

	downOutput, downLog, downCode := repo.run(t, "down.sh")
	if downCode != 0 {
		t.Fatalf("down.sh (gpu present) exited %d; output:\n%s\nlog:\n%s", downCode, downOutput, downLog)
	}

	wantPath := strings.TrimRight(sourceLib(t, "cdi_spec_path"), "\n")

	writeLine, _ := sshLineContaining(t, upLog, "cat >")
	writtenPath := singleQuoted(t, writeLine)
	if writtenPath != wantPath {
		t.Errorf("up.sh's spec-writing call used path %q, want %q (lib.sh's cdi_spec_path); ssh invocation:\n%s", writtenPath, wantPath, writeLine)
	}

	if !strings.Contains(upLog, "PORTAINER_E2E_CDI_SPEC="+wantPath) {
		t.Errorf("up.sh's compose call did not carry PORTAINER_E2E_CDI_SPEC=%s; log:\n%s", wantPath, upLog)
	}

	rmLine, _ := sshLineContaining(t, downLog, "rm -f")
	removedPath := singleQuoted(t, rmLine)
	if removedPath != wantPath {
		t.Errorf("down.sh's teardown rm -f used path %q, want %q (lib.sh's cdi_spec_path); ssh invocation:\n%s", removedPath, wantPath, rmLine)
	}

	if !strings.Contains(downLog, "-f docker-compose.gpu.yml") {
		t.Errorf("down.sh's compose invocation did not include -f docker-compose.gpu.yml; log:\n%s", downLog)
	}
}

// swarmAwareDockerStub writes a `docker` stub that, on top of the shared
// stub's own logging, answers the three swarm-specific invocations up.sh's
// new block makes: `compose ... ps -q docker` (the dind's container id),
// `exec <id> docker swarm init ...`, and `exec <id> docker service inspect/
// create ...` for the fixture service. swarmInitBehaviour and
// createBehaviour select a canned response so a test can force either the
// happy path or a degradation. Inspect is handled internally: the FIRST call
// (before create) always reports "not found" and the SECOND (after create)
// reports serviceID, mirroring the real idempotent-lookup shape
// swarm_fixture_service_id implements — a single shared stub cannot tell the
// two calls apart by arguments alone, since they are identical.
func swarmAwareDockerStub(t *testing.T, dir, swarmInitBehaviour, createBehaviour, serviceID string) {
	t.Helper()
	body := "#!/usr/bin/env bash\n" +
		"echo \"DOCKER_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>}\" >> \"$LOGFILE\"\n" +
		"case \"$*\" in\n" +
		"    *'compose -f docker-compose.yml ps -q docker'*)\n" +
		"        echo fake-dind-container-id\n" +
		"        exit 0\n" +
		"        ;;\n" +
		"    *'swarm init --advertise-addr 127.0.0.1'*)\n" +
		"        " + swarmInitBehaviour + "\n" +
		"        ;;\n" +
		"    *'service inspect '*'--format {{.ID}}'*)\n" +
		"        if [[ -f \"$LOGFILE.inspected\" ]]; then echo '" + serviceID + "'; exit 0; fi\n" +
		"        touch \"$LOGFILE.inspected\"\n" +
		"        exit 1\n" +
		"        ;;\n" +
		"    *'service create --detach --name '*'--replicas 1 busybox sleep 3600'*)\n" +
		"        " + createBehaviour + "\n" +
		"        ;;\n" +
		"esac\n" +
		"exit 0\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o755); err != nil {
		t.Fatalf("writing swarm-aware docker stub: %v", err)
	}
}

// TestUnit_SwarmBranch_UpScriptInitialisesSwarmAndForwardsTheFixtureServiceID
// is the up.sh-level proof that the pieces lib.sh's own unit tests cover in
// isolation (swarm_init, swarm_fixture_service_id) are actually wired
// together and reach the provisioner: without this, a version of up.sh that
// called the right functions but never exported
// PORTAINER_E2E_SWARM_SERVICE_ID, or exported the wrong variable, would pass
// every lib.sh unit test and still leave the estate with no recorded Swarm
// fixture.
func TestUnit_SwarmBranch_UpScriptInitialisesSwarmAndForwardsTheFixtureServiceID(t *testing.T) {
	repo := newComposeFakeRepo(t, "")
	swarmAwareDockerStub(t, repo.binDir, "exit 0", "exit 0", "wxyhlanc3nqz")

	output, log, code := repo.run(t, "up.sh")
	if code != 0 {
		t.Fatalf("up.sh (swarm available) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if !strings.Contains(log, "DOCKER_CALL: exec fake-dind-container-id docker swarm init --advertise-addr 127.0.0.1") {
		t.Errorf("up.sh did not initialise swarm on the estate's dind; log:\n%s", log)
	}
	if !strings.Contains(log, "DOCKER_CALL: exec fake-dind-container-id docker service create --detach --name portainer-mcp-e2e-swarm-probe --replicas 1 busybox sleep 3600") {
		t.Errorf("up.sh did not create the fixture service; log:\n%s", log)
	}
	if !strings.Contains(log, "GO_CALL:") || !strings.Contains(log, "PORTAINER_E2E_SWARM_SERVICE_ID=wxyhlanc3nqz") {
		t.Errorf("up.sh did not forward the fixture service id to the provisioner; log:\n%s", log)
	}
	if !strings.Contains(output, "swarm ready on the estate's docker daemon") {
		t.Errorf("up.sh did not report swarm readiness; output:\n%s", output)
	}
}

// TestUnit_SwarmBranch_UpScriptDegradesWithoutAbortingWhenSwarmCannotBeEnabled
// is the negative direction the brief calls out by name: a host where Swarm
// mode itself is refused must still bring up the rest of the estate, with a
// warning rather than a non-zero exit, and must forward no service id to the
// provisioner at all.
func TestUnit_SwarmBranch_UpScriptDegradesWithoutAbortingWhenSwarmCannotBeEnabled(t *testing.T) {
	repo := newComposeFakeRepo(t, "")
	swarmAwareDockerStub(t, repo.binDir, "echo 'Error response from daemon: swarm mode is not supported' >&2; exit 1", "exit 0", "wxyhlanc3nqz")

	output, log, code := repo.run(t, "up.sh")
	if code != 0 {
		t.Fatalf("up.sh (swarm unavailable) exited %d, want 0 (degrade, don't abort); output:\n%s\nlog:\n%s", code, output, log)
	}

	if !strings.Contains(output, "no swarm on the estate's docker daemon: swarm-dependent suites will skip") {
		t.Errorf("up.sh did not report the swarm degradation; output:\n%s", output)
	}
	if strings.Contains(log, "service create") {
		t.Errorf("up.sh attempted to create the fixture service despite swarm init failing; log:\n%s", log)
	}
	if !strings.Contains(log, "PORTAINER_E2E_SWARM_SERVICE_ID=<unset>") {
		t.Errorf("up.sh forwarded a swarm service id despite swarm being unavailable; log:\n%s", log)
	}
}
