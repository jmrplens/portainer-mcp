package harness

import (
	"errors"
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
	// PORTAINER_E2E_REMOTE="" overrides whatever the developer's own shell
	// happens to export, so a test's own inline "PORTAINER_E2E_REMOTE=1 some_fn
	// ..." prefix (a bash per-command assignment, which always wins over the
	// inherited environment for that one command regardless of what is set
	// here) is still the only way any of these calls sees it set. Without
	// this, TestUnit_DockerSSHDest_NeedsBothTheFlagAndTheKey's own
	// "key present but no flag yields local" case — the guard the repository
	// owner asked for by name — silently flips to reporting a destination the
	// moment PORTAINER_E2E_REMOTE=1 is exported in the shell running `go test`.
	cmd.Env = append(os.Environ(), "PORTAINER_E2E_REMOTE=")
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
	// See sourceLib's own comment: this call's own inline
	// "PORTAINER_E2E_REMOTE=1" prefix already wins over whatever this clears,
	// but leaving the developer's shell to flow in here uncleared would still
	// be one more untested assumption about what this exec.Cmd actually runs
	// with, at exactly the site the review named by name.
	cmd.Env = append(os.Environ(), "PORTAINER_E2E_REMOTE=")
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

// TestUnit_RefuseDockerHostSwitch_NoExistingMarkerAlwaysProceeds pins the
// case every first run depends on: with nothing recorded yet for this leg,
// there is nothing to switch away from, so both a local destination and any
// remote one must be accepted regardless of what is being requested.
func TestUnit_RefuseDockerHostSwitch_NoExistingMarkerAlwaysProceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cd := "cd " + dir + "\n"
	for _, dest := range []string{"", "truenas"} {
		if _, err := runLib(t, cd+`refuse_docker_host_switch "`+dest+`"`); err != nil {
			t.Errorf("refuse_docker_host_switch(%q) with no existing marker failed: %v", dest, err)
		}
	}
}

// TestUnit_RefuseDockerHostSwitch_SameDestinationProceeds proves the guard is
// not a blanket refusal: up.sh documents itself as idempotent ("running it
// twice replaces the estate rather than accumulating one"), and a second run
// against the SAME destination an earlier one already recorded must not be
// mistaken for a switch.
func TestUnit_RefuseDockerHostSwitch_SameDestinationProceeds(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cd := "cd " + dir + "\n"
	sourceLib(t, cd+`record_docker_host "truenas"`)
	if _, err := runLib(t, cd+`refuse_docker_host_switch "truenas"`); err != nil {
		t.Errorf("refuse_docker_host_switch(\"truenas\") against a marker already naming \"truenas\" failed: %v", err)
	}
}

// TestUnit_RefuseDockerHostSwitch_DifferentDestinationFails is the guard's
// whole reason to exist, exercised at the function level rather than only
// through a full script run: an existing marker naming one destination and a
// request for a DIFFERENT one — including the empty (local) destination a
// plain `make e2e-up` always resolves to — must fail loudly rather than let
// the caller go on to call record_docker_host, which would otherwise silently
// delete or overwrite the only record of where the earlier estate actually
// is.
func TestUnit_RefuseDockerHostSwitch_DifferentDestinationFails(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		recorded, dest string
	}{
		{"remote recorded, local requested", "truenas", ""},
		{"remote recorded, a different remote requested", "truenas", "some-other-host"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			cd := "cd " + dir + "\n"
			sourceLib(t, cd+`record_docker_host "`+tc.recorded+`"`)
			out, err := runLib(t, cd+`refuse_docker_host_switch "`+tc.dest+`"`)
			if err == nil {
				t.Fatalf("refuse_docker_host_switch(%q) against a marker naming %q succeeded, want a non-zero exit", tc.dest, tc.recorded)
			}
			if !strings.Contains(out, "refusing to continue") {
				t.Errorf("refuse_docker_host_switch's failure carried no diagnostic; stderr was %q", out)
			}
		})
	}
}

// TestUnit_RefuseDockerHostSwitch_LegArgumentChecksThatLegsOwnMarker proves
// the leg argument is honoured: a mismatch recorded under the "kubernetes"
// leg must not be raised against a call checking the compose leg (leg ""),
// and vice versa — the same independence
// TestUnit_DockerHostMarker_LegsAreIndependent already proves for
// record_docker_host/recorded_docker_host themselves.
func TestUnit_RefuseDockerHostSwitch_LegArgumentChecksThatLegsOwnMarker(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cd := "cd " + dir + "\n"
	sourceLib(t, cd+`record_docker_host "truenas" "kubernetes"`)
	// The compose leg's own marker is untouched, so a compose-leg check
	// against any destination must proceed regardless of the kubernetes
	// leg's recorded value.
	if _, err := runLib(t, cd+`refuse_docker_host_switch "some-other-host"`); err != nil {
		t.Errorf("refuse_docker_host_switch for the compose leg was refused by the kubernetes leg's own marker: %v", err)
	}
	// The kubernetes leg's own check against a genuinely different
	// destination must still refuse.
	if _, err := runLib(t, cd+`refuse_docker_host_switch "some-other-host" "kubernetes"`); err == nil {
		t.Error("refuse_docker_host_switch for the kubernetes leg succeeded despite its own marker naming a different host")
	}
}

// runLib is sourceLib's sibling for a script whose FAILURE is the behaviour
// under test: sourceLib's t.Fatalf on any non-zero exit is exactly wrong
// here, since a refusal is the expected outcome for several of these tests.
// It returns the combined stdout+stderr (refuse_docker_host_switch's own
// diagnostic goes to stderr) and the command's error, letting the caller
// decide what a non-nil error means.
func runLib(t *testing.T, script string) (string, error) {
	t.Helper()
	lib, err := filepath.Abs("../scripts/lib.sh")
	if err != nil {
		t.Fatalf("resolving lib.sh: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", "source "+lib+"\n"+script)
	// See sourceLib's own comment: cleared here for the same reason, so this
	// helper's callers are not exposed to whatever the developer's own shell
	// happens to export.
	cmd.Env = append(os.Environ(), "PORTAINER_E2E_REMOTE=")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// licenceLockPath mirrors licence_lock_path's own construction (repo_root +
// "/test/e2e/.licence.lock"), so a test can read or stat the lock file
// directly without sourcing lib.sh a second time just to ask it for the
// path.
func licenceLockPath(repoRoot string) string {
	return filepath.Join(repoRoot, "test", "e2e", ".licence.lock")
}

// licenceLockRepoRoot returns a fresh temporary directory shaped the way
// every real caller's repo root already is: with test/e2e/ present, so
// take_licence_lock's own ">" write has somewhere to land. Every real
// invocation runs from inside an actual checkout, where that directory is
// simply part of the repository and always exists; a bare t.TempDir() does
// not have it, and take_licence_lock's atomic write then fails for a reason
// that has nothing to do with the lock itself (a missing parent directory,
// not an existing file) -- its own doc says as much: that failure is
// indistinguishable from "already locked" from inside the function, which is
// exactly why the test fixture must not manufacture it by accident.
func licenceLockRepoRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "test", "e2e"), 0o755); err != nil {
		t.Fatalf("creating test/e2e under the fixture repo root: %v", err)
	}
	return root
}

// licenceLockStubMode selects one of licence_lock_holder_running's three
// possible answers for a stubbed docker/k3d: the check ran cleanly and
// found the leg up, ran cleanly and found nothing, or could not run at all
// (a daemon that refuses the connection, or a "not installed" tool -- see
// licence_lock_holder_running's own doc for why round 2 exists specifically
// to keep this third case from reading the same as the second).
type licenceLockStubMode int

const (
	licenceLockStubRunning licenceLockStubMode = iota
	licenceLockStubAbsent
	licenceLockStubFails
)

// licenceLockDockerStub writes a stub `docker` binary onto PATH that answers
// licence_lock_holder_running's own `docker ps --filter
// label=com.docker.compose.project=portainer-mcp-e2e ...` call, so these
// tests never depend on what containers actually exist on the machine
// running `go test` -- exactly the ambient-state problem
// licence_lock_holder_running's own doc names as the reason it matches on
// the compose project label rather than a container name substring. Mirrors
// dockerSwarmStub's technique above. licenceLockStubFails exits non-zero
// with stderr text shaped like the real "Cannot connect to the Docker
// daemon" failure, exercising the branch round 2 added: a command that
// fails outright, not one that merely answers empty.
func licenceLockDockerStub(t *testing.T, dir string, mode licenceLockStubMode) {
	t.Helper()
	var answer string
	switch mode {
	case licenceLockStubRunning:
		answer = "echo portainer-mcp-e2e-portainer-ce-1"
	case licenceLockStubAbsent:
		answer = ":"
	case licenceLockStubFails:
		answer = `echo "Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?" >&2; exit 1`
	}
	body := "#!/usr/bin/env bash\n" +
		"case \"$*\" in\n" +
		"    *'com.docker.compose.project=portainer-mcp-e2e'*)\n" +
		"        " + answer + "\n" +
		"        ;;\n" +
		"    *)\n" +
		"        echo \"unexpected docker invocation: $*\" >&2\n" +
		"        exit 1\n" +
		"        ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o700); err != nil {
		t.Fatalf("writing docker stub: %v", err)
	}
}

// licenceLockK3DStub mirrors licenceLockDockerStub for the Kubernetes leg's
// own `k3d cluster list -o json` check. licenceLockStubFails answers
// `cluster list` itself failing (a daemon it cannot reach); the OTHER way
// this leg's check can be unable to run -- k3d missing from PATH entirely --
// is exercised separately (TestUnit_LicenceLockHolderRunning_KubernetesK3DNotInstalled),
// since that failure mode has no docker/k3d stub to write at all.
func licenceLockK3DStub(t *testing.T, dir string, mode licenceLockStubMode) {
	t.Helper()
	var body string
	switch mode {
	case licenceLockStubFails:
		body = "#!/usr/bin/env bash\n" +
			"case \"$*\" in\n" +
			"    *'cluster list -o json'*)\n" +
			"        echo 'Error: failed to connect to docker' >&2\n" +
			"        exit 1\n" +
			"        ;;\n" +
			"    *)\n" +
			"        echo \"unexpected k3d invocation: $*\" >&2\n" +
			"        exit 1\n" +
			"        ;;\n" +
			"esac\n"
	default:
		list := "[]"
		if mode == licenceLockStubRunning {
			list = `[{"name":"portainer-mcp-e2e"}]`
		}
		body = "#!/usr/bin/env bash\n" +
			"case \"$*\" in\n" +
			"    *'cluster list -o json'*)\n" +
			"        echo '" + list + "'\n" +
			"        ;;\n" +
			"    *)\n" +
			"        echo \"unexpected k3d invocation: $*\" >&2\n" +
			"        exit 1\n" +
			"        ;;\n" +
			"esac\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "k3d"), []byte(body), 0o700); err != nil {
		t.Fatalf("writing k3d stub: %v", err)
	}
}

// licenceLockHolderRunningExitCode runs licence_lock_holder_running(leg)
// under runLib and returns its exact exit code -- 0/1/2, not merely
// zero/non-zero -- via *exec.ExitError. A plain err==nil/err!=nil check
// would pass just as well whether the guard folded "unknown" into "not
// running" or kept the two apart, which is precisely the collapse round 2
// exists to catch; only the exact code proves it did not happen.
func licenceLockHolderRunningExitCode(t *testing.T, script string) int {
	t.Helper()
	lib, err := filepath.Abs("../scripts/lib.sh")
	if err != nil {
		t.Fatalf("resolving lib.sh: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), "bash", "-euo", "pipefail", "-c", "source "+lib+"\n"+script)
	cmd.Env = append(os.Environ(), "PORTAINER_E2E_REMOTE=")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("running %q did not fail with a process exit error: %v\noutput:\n%s", script, err, out)
	}
	return exitErr.ExitCode()
}

// TestUnit_LicenceLockHolderRunning_MatchesEachLegsOwnSignal proves all
// THREE outcomes licence_lock_holder_running reports for both legs it
// switches on -- running (0), confirmed not running (1), and could not be
// determined (2) -- each independently, with the real docker/k3d binaries
// never consulted. The third state is round 2's whole point: a re-review
// found that a stubbed docker/k3d failure read back identically to "not
// running" before this change, which is exactly the distinction this table
// exists to pin down going forward.
func TestUnit_LicenceLockHolderRunning_MatchesEachLegsOwnSignal(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		leg  string
		mode licenceLockStubMode
		want int
	}{
		{"compose running", "compose", licenceLockStubRunning, 0},
		{"compose confirmed not running", "compose", licenceLockStubAbsent, 1},
		{"compose check fails (daemon unreachable)", "compose", licenceLockStubFails, 2},
		{"kubernetes running", "kubernetes", licenceLockStubRunning, 0},
		{"kubernetes confirmed not running", "kubernetes", licenceLockStubAbsent, 1},
		{"kubernetes check fails (cluster list unreachable)", "kubernetes", licenceLockStubFails, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			licenceLockDockerStub(t, dir, tc.mode)
			licenceLockK3DStub(t, dir, tc.mode)
			script := "PATH=" + dir + ":$PATH\nlicence_lock_holder_running " + tc.leg
			got := licenceLockHolderRunningExitCode(t, script)
			if got != tc.want {
				t.Errorf("licence_lock_holder_running(%q) = exit %d, want %d", tc.leg, got, tc.want)
			}
		})
	}
}

// TestUnit_LicenceLockHolderRunning_KubernetesK3DNotInstalled covers the
// OTHER way the Kubernetes leg's check can be unable to run: k3d missing
// from PATH entirely, which says nothing about whether a cluster exists --
// only that this machine cannot ask. A PATH containing nothing at all
// guarantees k3d is absent regardless of what the machine running the test
// actually has, mirroring TestUnit_DetectGPUName_ReportsNothingWhenTheHostHasNoNvidiaSmi's
// own technique for the identical "tool absent" shape.
func TestUnit_LicenceLockHolderRunning_KubernetesK3DNotInstalled(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	got := licenceLockHolderRunningExitCode(t, "PATH="+dir+" licence_lock_holder_running kubernetes")
	if got != 2 {
		t.Errorf("licence_lock_holder_running(\"kubernetes\") with k3d missing from PATH = exit %d, want 2 (could not determine)", got)
	}
}

// TestUnit_LicenceLockHolderRunning_UnrecognisedLegIsConfirmedNotRunning
// pins the default case's own reasoning, not merely its return value: an
// unrecognised leg (a blank or garbled HOLDER, reachable only via an
// interrupted write) is 1 (confirmed not running), never 2 (unknown) --
// no leg by that name is ever a candidate to be running, so there is
// nothing uncertain to report.
func TestUnit_LicenceLockHolderRunning_UnrecognisedLegIsConfirmedNotRunning(t *testing.T) {
	t.Parallel()
	got := licenceLockHolderRunningExitCode(t, `licence_lock_holder_running ""`)
	if got != 1 {
		t.Errorf("licence_lock_holder_running(\"\") = exit %d, want 1 (confirmed not running)", got)
	}
}

// TestUnit_TakeLicenceLock_FirstCallWritesHolderTakenAtAndEstate is the
// success path: with no existing lock, take_licence_lock must record all
// three fields the refusal message (and licence-check.sh's staleness check)
// both depend on.
func TestUnit_TakeLicenceLock_FirstCallWritesHolderTakenAtAndEstate(t *testing.T) {
	t.Parallel()
	repoRoot := licenceLockRepoRoot(t)
	if _, err := runLib(t, `take_licence_lock `+repoRoot+` compose`); err != nil {
		t.Fatalf("take_licence_lock on an unlocked repo failed: %v", err)
	}
	data, err := os.ReadFile(licenceLockPath(repoRoot))
	if err != nil {
		t.Fatalf("reading the lock file: %v", err)
	}
	for _, want := range []string{"HOLDER=compose", "TAKEN_AT=", "ESTATE="} {
		if !strings.Contains(string(data), want) {
			t.Errorf("lock file missing %q; content:\n%s", want, data)
		}
	}
}

// TestUnit_TakeLicenceLock_SecondCallRefusesWhileHolderIsRunning is the
// guard's whole reason to exist: a second leg must not be able to take the
// lock while the first leg is still using it, and the refusal must name the
// running holder rather than report it as merely stale.
func TestUnit_TakeLicenceLock_SecondCallRefusesWhileHolderIsRunning(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	licenceLockDockerStub(t, dir, licenceLockStubRunning)
	script := "PATH=" + dir + ":$PATH\n" +
		"take_licence_lock " + repoRoot + " compose\n" +
		"take_licence_lock " + repoRoot + " compose"
	out, err := runLib(t, script)
	if err == nil {
		t.Fatalf("second take_licence_lock call succeeded; want a non-zero exit. output:\n%s", out)
	}
	if !strings.Contains(out, "already held by 'compose'") {
		t.Errorf("refusal did not name the running holder; output:\n%s", out)
	}
}

// TestUnit_TakeLicenceLock_StaleHolderRefusesAndLeavesTheLockOnDisk is Step
// 2's central rule, proven at the function level: a lock naming a leg that
// is not actually running must still refuse -- never silently clear itself
// -- and the refusal must point at `make e2e-licence-release` rather than
// any command that deletes the lock outright.
func TestUnit_TakeLicenceLock_StaleHolderRefusesAndLeavesTheLockOnDisk(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	// The first take below succeeds with no docker/k3d call at all (there is
	// no existing lock yet to check a holder against); this stub only has to
	// answer for the SECOND call, which evaluates licence_lock_holder_running
	// against the lock the first call just wrote.
	licenceLockDockerStub(t, dir, licenceLockStubAbsent)
	script := "PATH=" + dir + ":$PATH\n" +
		"take_licence_lock " + repoRoot + " compose\n" +
		"take_licence_lock " + repoRoot + " compose"
	out, err := runLib(t, script)
	if err == nil {
		t.Fatalf("take_licence_lock against a stale holder succeeded; want a non-zero exit. output:\n%s", out)
	}
	if !strings.Contains(out, "stale") || !strings.Contains(out, "make e2e-licence-release") {
		t.Errorf("stale refusal missing the expected diagnostic; output:\n%s", out)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); err != nil {
		t.Errorf("stale refusal removed the lock file; it must be reported, never auto-removed: %v", err)
	}
}

// TestUnit_TakeLicenceLock_UnknownHolderRefusesWithoutAssertingStaleness is
// round 2's fix, proven at take_licence_lock's own refusal message: when
// licence_lock_holder_running cannot tell whether the holder is running at
// all (its docker/k3d check itself failed), the refusal must say so --
// never call the lock "stale", and never point at `make e2e-licence-release`
// the way the confirmed-stale message does, since that command is exactly
// what would delete a lock that may still be protecting a live estate.
func TestUnit_TakeLicenceLock_UnknownHolderRefusesWithoutAssertingStaleness(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	licenceLockDockerStub(t, dir, licenceLockStubFails)
	script := "PATH=" + dir + ":$PATH\n" +
		"take_licence_lock " + repoRoot + " compose\n" +
		"take_licence_lock " + repoRoot + " compose"
	out, err := runLib(t, script)
	if err == nil {
		t.Fatalf("second take_licence_lock call succeeded despite an unresolvable holder check; want a non-zero exit. output:\n%s", out)
	}
	if !strings.Contains(out, "could not be determined") {
		t.Errorf("refusal did not say the check itself could not be resolved; output:\n%s", out)
	}
	if strings.Contains(out, "reported as stale") {
		t.Errorf("refusal asserted staleness it never established; output:\n%s", out)
	}
	if strings.Contains(out, "run 'make e2e-licence-release'") {
		t.Errorf("refusal recommended the command that deletes the lock, on an answer that may not mean the holder is gone; output:\n%s", out)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); err != nil {
		t.Errorf("unknown-holder refusal removed the lock file; it must be reported, never auto-removed: %v", err)
	}
}

// licenceCheckLockTailScript extracts, verbatim, licence-check.sh's own
// lock-clearing tail -- from its "lock_path=$(licence_lock_path ...)" line
// through the matching top-level "fi" -- so a test exercises the REAL
// script's current decision logic rather than a hand-copied duplicate that
// could silently drift from it if that file changes. The rest of
// licence-check.sh (a real `docker run`, a real
// `go run ./harness/cmd/provision -recover-licence` against a throwaway
// server) cannot be run from a unit test at all -- and must not be, per
// this task's own standing constraint never to touch the live estate or its
// licence -- so this extracted tail is the only piece of the script's
// actual behavior a test here can exercise.
func licenceCheckLockTailScript(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("../scripts/licence-check.sh")
	if err != nil {
		t.Fatalf("reading licence-check.sh: %v", err)
	}
	lines := strings.Split(string(data), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, `lock_path=$(licence_lock_path`) {
			start = i
			break
		}
	}
	if start == -1 {
		t.Fatalf("licence-check.sh's lock_path=$(licence_lock_path ...) line was not found; has it moved or been renamed?")
	}
	end := -1
	for i := start; i < len(lines); i++ {
		if lines[i] == "fi" {
			end = i
			break
		}
	}
	if end == -1 {
		t.Fatalf("could not find the closing \"fi\" for licence-check.sh's lock-clearing tail")
	}
	return strings.Join(lines[start:end+1], "\n")
}

// TestUnit_LicenceCheckLockTail_RunningHolderWarnsAndKeepsTheLock,
// TestUnit_LicenceCheckLockTail_AbsentHolderClearsTheLock and
// TestUnit_LicenceCheckLockTail_UnknownHolderWarnsAndKeepsTheLock are the
// three cases round 2 asked for by name, run against the real script's own
// extracted tail (see licenceCheckLockTailScript): a stub that reports the
// holder running, one that reports it confirmed absent, and one that fails
// outright -- each must reach ITS OWN outcome. The failing case is the
// re-reviewer's exact reachable accident: `make e2e-licence-release` run
// while docker is unreachable (or DOCKER_HOST points at the wrong machine)
// used to read as "confirmed not running" and delete the lock out from
// under a genuinely live estate; it must now warn and leave the file on
// disk, identically to the running case, not identically to the absent one.

func TestUnit_LicenceCheckLockTail_RunningHolderWarnsAndKeepsTheLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	licenceLockDockerStub(t, dir, licenceLockStubRunning)
	script := "PATH=" + dir + ":$PATH\n" +
		"repo_root=" + repoRoot + "\n" +
		"take_licence_lock \"$repo_root\" compose\n" +
		licenceCheckLockTailScript(t)
	out, err := runLib(t, script)
	if err != nil {
		t.Fatalf("licence-check.sh's tail failed against a running holder: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "appears to be running right now") {
		t.Errorf("tail did not warn about the running holder; output:\n%s", out)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); err != nil {
		t.Errorf("licence-check.sh's tail deleted a running holder's lock: %v", err)
	}
}

func TestUnit_LicenceCheckLockTail_AbsentHolderClearsTheLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	licenceLockDockerStub(t, dir, licenceLockStubAbsent)
	script := "PATH=" + dir + ":$PATH\n" +
		"repo_root=" + repoRoot + "\n" +
		"take_licence_lock \"$repo_root\" compose\n" +
		licenceCheckLockTailScript(t)
	out, err := runLib(t, script)
	if err != nil {
		t.Fatalf("licence-check.sh's tail failed against a confirmed-absent holder: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "cleared the stale licence lock") {
		t.Errorf("tail did not report clearing the lock; output:\n%s", out)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); !os.IsNotExist(err) {
		t.Errorf("licence-check.sh's tail left a confirmed-stale lock in place: err=%v", err)
	}
}

func TestUnit_LicenceCheckLockTail_UnknownHolderWarnsAndKeepsTheLock(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	repoRoot := licenceLockRepoRoot(t)
	licenceLockDockerStub(t, dir, licenceLockStubFails)
	script := "PATH=" + dir + ":$PATH\n" +
		"repo_root=" + repoRoot + "\n" +
		"take_licence_lock \"$repo_root\" compose\n" +
		licenceCheckLockTailScript(t)
	out, err := runLib(t, script)
	if err != nil {
		t.Fatalf("licence-check.sh's tail failed when the holder check itself failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "could not determine") {
		t.Errorf("tail did not say the check itself could not be resolved; output:\n%s", out)
	}
	if strings.Contains(out, "cleared the stale licence lock") {
		t.Errorf("tail cleared the lock on an unresolvable check; output:\n%s", out)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); err != nil {
		t.Errorf("licence-check.sh's tail deleted the lock when the holder check itself failed -- the exact accident round 2 exists to prevent: %v", err)
	}
}

// TestUnit_ReleaseLicenceLock_ToleratesAnAbsentLock is Ruling 1's central
// case: the very first `make e2e-down` on a machine where a lock was never
// taken (this task did not exist yet, or a Community-only run never created
// one) must warn and succeed, never fail teardown.
func TestUnit_ReleaseLicenceLock_ToleratesAnAbsentLock(t *testing.T) {
	t.Parallel()
	repoRoot := licenceLockRepoRoot(t)
	out, err := runLib(t, `release_licence_lock `+repoRoot+` compose`)
	if err != nil {
		t.Fatalf("release_licence_lock against an absent lock failed; want success: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "warning:") {
		t.Errorf("release_licence_lock against an absent lock did not warn; output:\n%s", out)
	}
}

// TestUnit_ReleaseLicenceLock_LeavesAForeignHolderInPlace proves a leg's own
// teardown can never remove a lock it does not own: down.sh releasing the
// compose leg's lock must not be able to clear one the Kubernetes leg
// holds, or vice versa.
func TestUnit_ReleaseLicenceLock_LeavesAForeignHolderInPlace(t *testing.T) {
	t.Parallel()
	repoRoot := licenceLockRepoRoot(t)
	if _, err := runLib(t, `take_licence_lock `+repoRoot+` kubernetes`); err != nil {
		t.Fatalf("setup: take_licence_lock failed: %v", err)
	}
	before, err := os.ReadFile(licenceLockPath(repoRoot))
	if err != nil {
		t.Fatalf("reading the lock file after setup: %v", err)
	}
	out, err := runLib(t, `release_licence_lock `+repoRoot+` compose`)
	if err != nil {
		t.Fatalf("release_licence_lock against a foreign holder failed; want a warned success: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "warning:") {
		t.Errorf("release_licence_lock against a foreign holder did not warn; output:\n%s", out)
	}
	after, err := os.ReadFile(licenceLockPath(repoRoot))
	if err != nil {
		t.Fatalf("lock file removed by a release call for a leg that does not own it: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("lock file content changed by a foreign release; before:\n%s\nafter:\n%s", before, after)
	}
}

// TestUnit_ReleaseLicenceLock_RemovesItsOwn is the ordinary teardown path:
// the leg that took the lock is the one releasing it.
func TestUnit_ReleaseLicenceLock_RemovesItsOwn(t *testing.T) {
	t.Parallel()
	repoRoot := licenceLockRepoRoot(t)
	if _, err := runLib(t, `take_licence_lock `+repoRoot+` compose`); err != nil {
		t.Fatalf("setup: take_licence_lock failed: %v", err)
	}
	if _, err := runLib(t, `release_licence_lock `+repoRoot+` compose`); err != nil {
		t.Fatalf("release_licence_lock for the owning leg failed: %v", err)
	}
	if _, err := os.Stat(licenceLockPath(repoRoot)); !os.IsNotExist(err) {
		t.Errorf("lock file still present after its own leg released it: err=%v", err)
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
// name and then exits non-zero. detect_gpu_name has no "| head -n1" in its
// remote command string and no pipefail (see its own doc for why: a
// still-writing nvidia-smi piped into head can take SIGPIPE and be
// misread as "no GPU"). Instead it captures the WHOLE command's output into a
// variable first ("if ! raw=$(...); then return 0; fi") and only takes the
// first line locally afterward, so a failing nvidia-smi has to be caught
// through on_docker_host's own exit status, not a pipe stage failing on its
// behalf. Without that capture-then-check shape, a bare "|| true" style
// implementation would forward whatever nvidia-smi had already written even
// though it went on to fail — the same trap gpu_cdi_spec's own doc describes
// for a truncated CDI specification, one severity level lower here because a
// partial display name is still just a name.
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

// TestUnit_SwarmFixtureServiceName_ReturnsAFixedPortainerMcpE2EPrefixedName
// pins the literal value: up.sh's swarm_init/swarm_fixture_service_id and any
// future orphan sweep must all agree on the same name, and a typo here would
// silently make a second `make e2e-up` create a duplicate service instead of
// finding the existing one.
func TestUnit_SwarmFixtureServiceName_ReturnsAFixedPortainerMcpE2EPrefixedName(t *testing.T) {
	t.Parallel()
	got := strings.TrimRight(sourceLib(t, `swarm_fixture_service_name`), "\n")
	if !strings.HasPrefix(got, "portainer-mcp-e2e-") {
		t.Errorf("swarm_fixture_service_name() = %q, want it prefixed like the estate's own compose project", got)
	}
}

// dockerSwarmStub writes a fake `docker` binary that only understands the
// handful of `exec <id> docker ...` invocations swarm_init and
// swarm_fixture_service_id make, so these tests never touch a real Docker
// daemon. swarmInitBehaviour and inspectBehaviour each select one of a small
// set of canned responses; createBehaviour does the same for `service
// create`. Any invocation these tests do not expect exits 1 with a message
// naming the unexpected arguments, so a function calling docker with the
// wrong arguments fails loudly here rather than silently getting a
// convenient default answer.
func dockerSwarmStub(t *testing.T, dir, swarmInitBehaviour, inspectBehaviour, createBehaviour string) {
	t.Helper()
	body := "#!/usr/bin/env bash\n" +
		"case \"$*\" in\n" +
		"    *'swarm init --advertise-addr 127.0.0.1'*)\n" +
		"        " + swarmInitBehaviour + "\n" +
		"        ;;\n" +
		"    *'service inspect '*'--format {{.ID}}'*)\n" +
		"        " + inspectBehaviour + "\n" +
		"        ;;\n" +
		"    *'service create --detach --name '*'--replicas 1 busybox sleep 3600'*)\n" +
		"        " + createBehaviour + "\n" +
		"        ;;\n" +
		"    *)\n" +
		"        echo \"unexpected docker invocation: $*\" >&2\n" +
		"        exit 1\n" +
		"        ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o700); err != nil {
		t.Fatalf("writing docker stub: %v", err)
	}
}

// TestUnit_SwarmInit_SucceedsOnAFreshDaemon is swarm_init's ordinary path: the
// underlying `docker swarm init` succeeds outright.
func TestUnit_SwarmInit_SucceedsOnAFreshDaemon(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dockerSwarmStub(t, dir, "exit 0", "exit 1", "exit 1")
	if _, err := runLib(t, `PATH=`+dir+`:$PATH swarm_init fake-dind-id`); err != nil {
		t.Errorf("swarm_init() on a fresh daemon failed: %v", err)
	}
}

// TestUnit_SwarmInit_AlreadyInASwarm_IsIdempotent is the mutation-proof for
// the exact failure mode the brief calls out by name: a second `make e2e-up`
// run, with no intervening `make e2e-down`, must not abort the whole estate
// because the dind kept its Swarm state from the first run.
func TestUnit_SwarmInit_AlreadyInASwarm_IsIdempotent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dockerSwarmStub(t, dir,
		`echo 'Error response from daemon: This node is already part of a swarm. Use "docker swarm leave" to leave this swarm and join another one.' >&2; exit 1`,
		"exit 1", "exit 1")
	if _, err := runLib(t, `PATH=`+dir+`:$PATH swarm_init fake-dind-id`); err != nil {
		t.Errorf("swarm_init() against an already-initialised daemon failed, want idempotent success: %v", err)
	}
}

// TestUnit_SwarmInit_OtherFailure_WarnsAndDegrades proves the third path:
// anything other than "already part of a swarm" is a plain, reported
// failure -- never fatal -- so a host where Swarm mode itself is unavailable
// still lets the rest of the estate come up.
func TestUnit_SwarmInit_OtherFailure_WarnsAndDegrades(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dockerSwarmStub(t, dir, `echo 'Error response from daemon: swarm mode is not supported' >&2; exit 1`, "exit 1", "exit 1")
	out, err := runLib(t, `PATH=`+dir+`:$PATH swarm_init fake-dind-id`)
	if err == nil {
		t.Fatal("swarm_init() with an unrecognised failure succeeded, want a non-zero exit")
	}
	if !strings.Contains(out, "warning:") || !strings.Contains(out, "swarm mode is not supported") {
		t.Errorf("swarm_init() did not report the underlying failure; output:\n%s", out)
	}
}

// TestUnit_SwarmFixtureServiceID_CreatesTheServiceWhenAbsent is the fresh
// path: no existing service (both inspect calls fail-then-succeed around a
// create), so the function creates it and reads its id back.
func TestUnit_SwarmFixtureServiceID_CreatesTheServiceWhenAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const wantID = "wxyhlanc3nqz9mnpnzenvg8p8"
	// The FIRST inspect (before create) must fail -- nothing exists yet --
	// and the SECOND (after create) must succeed. A single shared stub
	// cannot distinguish the two calls by arguments alone (they are
	// identical), so this stub counts invocations through a marker file.
	marker := filepath.Join(dir, "inspect-called")
	body := "#!/usr/bin/env bash\n" +
		"case \"$*\" in\n" +
		"    *'service inspect '*'--format {{.ID}}'*)\n" +
		"        if [[ -f '" + marker + "' ]]; then echo '" + wantID + "'; exit 0; fi\n" +
		"        touch '" + marker + "'\n" +
		"        exit 1\n" +
		"        ;;\n" +
		"    *'service create --detach --name '*'--replicas 1 busybox sleep 3600'*)\n" +
		"        echo '" + wantID + "'\n" +
		"        exit 0\n" +
		"        ;;\n" +
		"    *)\n" +
		"        echo \"unexpected docker invocation: $*\" >&2\n" +
		"        exit 1\n" +
		"        ;;\n" +
		"esac\n"
	if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(body), 0o700); err != nil {
		t.Fatalf("writing docker stub: %v", err)
	}
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH swarm_fixture_service_id fake-dind-id`), "\n")
	if got != wantID {
		t.Errorf("swarm_fixture_service_id() = %q, want %q", got, wantID)
	}
}

// TestUnit_SwarmFixtureServiceID_ReusesAnExistingService is the idempotency
// proof: a service the FIRST inspect already finds must be reused, and
// `docker service create` must never be invoked at all -- calling it would
// fail on Docker's own "name conflicts with an existing object" against a
// real daemon.
func TestUnit_SwarmFixtureServiceID_ReusesAnExistingService(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	const wantID = "existing-service-id-12345"
	dockerSwarmStub(t, dir, "exit 1", "echo '"+wantID+"'; exit 0", "exit 1")
	got := strings.TrimRight(sourceLib(t, `PATH=`+dir+`:$PATH swarm_fixture_service_id fake-dind-id`), "\n")
	if got != wantID {
		t.Errorf("swarm_fixture_service_id() with an already-existing service = %q, want %q", got, wantID)
	}
}

// TestUnit_SwarmFixtureServiceID_CreateFailure_WarnsAndDegrades proves the
// service, like Swarm itself, is optional: a create failure (image pull
// refused, no manager available, anything else) must be reported and
// returned as a plain failure, not a fatal one.
func TestUnit_SwarmFixtureServiceID_CreateFailure_WarnsAndDegrades(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dockerSwarmStub(t, dir, "exit 1", "exit 1", `echo 'Error response from daemon: no suitable node' >&2; exit 1`)
	out, err := runLib(t, `PATH=`+dir+`:$PATH swarm_fixture_service_id fake-dind-id`)
	if err == nil {
		t.Fatal("swarm_fixture_service_id() with a failing create succeeded, want a non-zero exit")
	}
	if !strings.Contains(out, "warning:") || !strings.Contains(out, "no suitable node") {
		t.Errorf("swarm_fixture_service_id() did not report the underlying failure; output:\n%s", out)
	}
}

// TestUnit_SwarmFixtureServiceID_CreatedButUnconfirmed_WarnsAndDegrades
// covers the one path the other tests cannot reach: create reports success
// but the follow-up inspect still cannot find it (a race, or a Docker
// version that names the service differently). This must degrade exactly
// like an outright create failure, not propagate an empty id as if it were
// a real one.
func TestUnit_SwarmFixtureServiceID_CreatedButUnconfirmed_WarnsAndDegrades(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	dockerSwarmStub(t, dir, "exit 1", "exit 1", "exit 0")
	out, err := runLib(t, `PATH=`+dir+`:$PATH swarm_fixture_service_id fake-dind-id`)
	if err == nil {
		t.Fatal("swarm_fixture_service_id() with an unconfirmable service succeeded, want a non-zero exit")
	}
	if !strings.Contains(out, "warning:") || !strings.Contains(out, "could not be read back") {
		t.Errorf("swarm_fixture_service_id() did not report the confirmation failure; output:\n%s", out)
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
