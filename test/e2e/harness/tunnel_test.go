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
		// -M opens a *controllable* master, and sock is the literal path this
		// test configured through PORTAINER_E2E_TUNNEL_SOCK (not a hardcoded
		// guess). This is the seam where a defect leaks a live background ssh
		// process and its forwarded ports on every teardown: without -M there
		// is no master for tunnel_down to address at all; if the socket that
		// travels with it were ever wrong, tunnel_down's `-O exit` would find
		// nothing to close. Either way tunnel_down's own `2>/dev/null ||
		// true` — there to let a legitimate no-op succeed — swallows the
		// evidence, so nothing would ever report the leak.
		"-M",
		sock,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tunnel_up did not pass %q to ssh; invocation was:\n%s", want, got)
		}
	}
	// tunnel_up closes any stale tunnel before opening a new master, so a
	// socket left behind by a crashed run does not make the fresh `ssh -M`
	// refuse to bind. Both the cleanup call and the master call mention their
	// flags somewhere in the full log regardless of order, so only checking
	// the relative order of the two logged lines actually pins that the
	// cleanup ran first.
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	exitLine, masterLine := -1, -1
	for i, line := range lines {
		if exitLine == -1 && strings.Contains(line, "-O exit") {
			exitLine = i
		}
		if masterLine == -1 && strings.Contains(line, "-M") {
			masterLine = i
		}
	}
	if exitLine == -1 || masterLine == -1 {
		t.Fatalf("expected both a cleanup (-O exit) and a master (-M) ssh invocation; got:\n%s", got)
	}
	if exitLine >= masterLine {
		t.Errorf("tunnel_up did not close a stale tunnel before opening a new master; -O exit logged at line %d, -M at line %d:\n%s", exitLine, masterLine, got)
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
