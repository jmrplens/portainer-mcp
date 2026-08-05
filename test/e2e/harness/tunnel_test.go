package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sourceRemote runs script with test/e2e/scripts/remote.sh sourced, with a
// stub `ssh` earlier on PATH that appends its arguments to a log file. It
// returns the log's contents and the control-socket path this invocation was
// configured with (the same value exported as PORTAINER_E2E_TUNNEL_SOCK), so
// a test can assert not just that ssh was invoked, but that it was invoked
// against *this* socket, without needing a reachable host.
func sourceRemote(t *testing.T, script string) (log, sock string) {
	t.Helper()
	dir := t.TempDir()
	logPath := filepath.Join(dir, "ssh.log")
	stub := filepath.Join(dir, "ssh")
	body := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> " + logPath + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing ssh stub: %v", err)
	}
	remote, err := filepath.Abs("../scripts/remote.sh")
	if err != nil {
		t.Fatalf("resolving remote.sh: %v", err)
	}
	sock = filepath.Join(dir, "t.sock")
	full := "export PATH=" + dir + ":$PATH\nexport PORTAINER_E2E_TUNNEL_SOCK=" + sock + "\nsource " + remote + "\n" + script
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", full)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if _, err := cmd.Output(); err != nil {
		t.Fatalf("bash script failed: %v\nstderr:\n%s", err, stderr.String())
	}
	data, err := os.ReadFile(logPath)
	if os.IsNotExist(err) {
		return "", sock
	}
	if err != nil {
		t.Fatalf("reading ssh log: %v", err)
	}
	return string(data), sock
}

// sshLineContaining returns the single logged ssh invocation containing
// marker, together with its position among the logged lines (so a caller can
// also assert relative order between two invocations). It fails the test
// unless exactly one line matches: zero means the invocation never happened,
// and more than one would make "the line containing marker" itself
// ambiguous — a helper that tolerated either would reintroduce the same
// whole-log vagueness this exists to rule out. tunnel_up logs two
// invocations (a cleanup and a master), so a plain strings.Contains over the
// whole log cannot tell which one a flag or value belongs to; this pins an
// assertion to one specific call.
func sshLineContaining(t *testing.T, log, marker string) (line string, index int) {
	t.Helper()
	lines := strings.Split(strings.TrimRight(log, "\n"), "\n")
	found := -1
	for i, l := range lines {
		if strings.Contains(l, marker) {
			if found != -1 {
				t.Fatalf("more than one ssh invocation contained %q; log was:\n%s", marker, log)
			}
			found = i
			line = l
		}
	}
	if found == -1 {
		t.Fatalf("no ssh invocation contained %q; log was:\n%s", marker, log)
	}
	return line, found
}

func TestUnit_TunnelUp_NoDestinationInvokesNoSSH(t *testing.T) {
	t.Parallel()
	if got, _ := sourceRemote(t, `tunnel_up "" 19000 19001`); got != "" {
		t.Errorf("tunnel_up with no destination invoked ssh: %q", got)
	}
}

func TestUnit_TunnelUp_ForwardsEveryRequestedPortFromTheRemoteLoopback(t *testing.T) {
	t.Parallel()
	got, sock := sourceRemote(t, `tunnel_up "somehost" 19000 19001`)
	for _, want := range []string{
		"-L 19000:127.0.0.1:19000",
		"-L 19001:127.0.0.1:19001",
		"somehost",
		// ExitOnForwardFailure turns "the port was already taken" into a
		// non-zero exit here, rather than a suite that starts and then fails
		// against whatever else is listening on 19000.
		"-o ExitOnForwardFailure=yes",
		// BatchMode refuses to prompt and ConnectTimeout bounds the attempt:
		// without either, a missing or stale key turns a CI failure into a
		// hang until the job itself times out.
		"-o BatchMode=yes",
		"-o ConnectTimeout=10",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tunnel_up did not pass %q to ssh; invocation was:\n%s", want, got)
		}
	}

	// tunnel_up logs two ssh invocations: the pre-emptive cleanup call
	// (-O exit) and the new master (-M). A whole-log strings.Contains for the
	// socket path cannot tell which invocation carries it — if the master
	// bound a different (wrong) socket while the cleanup line still had the
	// right one, a whole-log check would be satisfied by the cleanup line
	// alone and the defect would pass silently. Pinning the socket to each
	// invocation individually is what actually proves the master binds the
	// configured control socket, and that a later tunnel_down addresses that
	// same one — the pairing whose absence lets ssh -M leak, unnoticed,
	// because tunnel_down's own `2>/dev/null || true` swallows the evidence.
	master, masterLine := sshLineContaining(t, got, "-M")
	if !strings.Contains(master, sock) {
		t.Errorf("tunnel_up's master invocation did not bind the configured socket %q; invocation was:\n%s", sock, master)
	}
	cleanup, cleanupLine := sshLineContaining(t, got, "-O exit")
	if !strings.Contains(cleanup, sock) {
		t.Errorf("tunnel_up's cleanup invocation did not address the configured socket %q; invocation was:\n%s", sock, cleanup)
	}

	// tunnel_up closes any stale tunnel before opening a new master, so a
	// socket left behind by a crashed run does not make the fresh `ssh -M`
	// refuse to bind. Only the relative order of the two logged lines pins
	// that the cleanup ran first.
	if cleanupLine >= masterLine {
		t.Errorf("tunnel_up did not close a stale tunnel before opening a new master; -O exit logged at line %d, -M at line %d:\n%s", cleanupLine, masterLine, got)
	}
}

func TestUnit_TunnelDown_NoDestinationInvokesNoSSH(t *testing.T) {
	t.Parallel()
	if got, _ := sourceRemote(t, `tunnel_down ""`); got != "" {
		t.Errorf("tunnel_down with no destination invoked ssh: %q", got)
	}
}

func TestUnit_TunnelDown_AsksTheMasterToExit(t *testing.T) {
	t.Parallel()
	got, sock := sourceRemote(t, `tunnel_down "somehost"`)
	for _, want := range []string{
		"-O exit",
		// Without -S pointing at this tunnel's own socket, `-O exit` would
		// address ssh's default control path instead of this one — it never
		// finds the master, and the `2>/dev/null || true` that exists so a
		// legitimate no-op can succeed silently swallows that mismatch too.
		// The result is a leaked background ssh process and its forwarded
		// ports on every teardown, invisibly.
		"-S",
		sock,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tunnel_down did not pass %q to ssh; invocation was:\n%s", want, got)
		}
	}
}
