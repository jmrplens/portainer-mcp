package harness

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
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
		// AddressFamily=inet is what makes ExitOnForwardFailure's own promise
		// actually hold. Measured directly against a real sshd: without it, a
		// taken 127.0.0.1:<port> makes ssh silently retry the bind on ::1 and
		// exit 0 once THAT succeeds -- ExitOnForwardFailure never fires,
		// because as far as ssh is concerned the forward did not fail, it just
		// landed somewhere else. It also has to live here, on the master's own
		// connection, rather than on any later per-forward request: measured
		// separately, the same option passed to `ssh -O forward` has no
		// effect, because the address family is negotiated once, when this
		// connection is established.
		"-o AddressFamily=inet",
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

// sourceRemoteCapturingResult is sourceRemote's sibling for
// tunnel_add_forward, whose success or failure IS the behaviour under test —
// unlike tunnel_up/tunnel_down, which never fail against a stub ssh that
// always exits 0. sourceRemote's own script would abort the whole invocation
// under set -e the moment a called function returns non-zero, which is
// wrong here: the calling script instead reports the result explicitly
// (`if fn; then echo RESULT:success; else echo RESULT:failure; fi`, exempt
// from set -e the same way lib.sh's own capture-then-emit functions are),
// and this helper returns that stdout alongside the ssh log and the
// script's own stderr (where tunnel_add_forward's diagnostic goes).
func sourceRemoteCapturingResult(t *testing.T, script string) (log, sock, stdout, stderrOut string) {
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
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("bash script failed: %v\nstderr:\n%s", err, stderr.String())
	}
	data, readErr := os.ReadFile(logPath)
	if readErr != nil && !os.IsNotExist(readErr) {
		t.Fatalf("reading ssh log: %v", readErr)
	}
	return string(data), sock, string(out), stderr.String()
}

func TestUnit_TunnelAddForward_NoDestinationInvokesNoSSH(t *testing.T) {
	t.Parallel()
	log, _, out, _ := sourceRemoteCapturingResult(t,
		`if tunnel_add_forward "" 19999 1.2.3.4 5555; then echo RESULT:success; else echo RESULT:failure; fi`)
	if log != "" {
		t.Errorf("tunnel_add_forward with no destination invoked ssh: %q", log)
	}
	if !strings.Contains(out, "RESULT:success") {
		t.Errorf("tunnel_add_forward with no destination did not report success; stdout was %q", out)
	}
}

// TestUnit_TunnelAddForward_RequestsTheForwardAndConfirmsItIsLive proves both
// halves of the contract: the right ssh invocation is made (-O forward
// against the SAME control socket tunnel_up would have bound, requesting a
// forward to an arbitrary remote host:port, not the destination's own
// loopback), and the function actually confirms liveness rather than
// trusting ssh's exit code.
//
// The listener standing in for "the forward is now live" is started only
// AFTER a short delay, deliberately — not present when the call begins.
// That is what exercises both checks rather than either one alone: if it
// were listening from the start, this would pass even with the preflight
// check deleted (there would be nothing for the preflight to reject, since
// by construction it looks identical to a genuine forward already being
// up); started only after the request, it can only be observed by the
// POLL, proving that half of the function actually runs and actually
// waits rather than checking once and giving up.
func TestUnit_TunnelAddForward_RequestsTheForwardAndConfirmsItIsLive(t *testing.T) {
	t.Parallel()
	port := reserveFreeTCPPort(t)

	go func() {
		time.Sleep(150 * time.Millisecond)
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			return
		}
		defer func() { _ = ln.Close() }()
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
		}
	}()

	script := fmt.Sprintf(
		`if tunnel_add_forward "somehost" %d "10.0.0.5" %d; then echo RESULT:success; else echo RESULT:failure; fi`,
		port, port)
	log, sock, out, _ := sourceRemoteCapturingResult(t, script)

	if !strings.Contains(out, "RESULT:success") {
		t.Fatalf("tunnel_add_forward did not report success even though the forward became live; stdout was %q, ssh log:\n%s", out, log)
	}
	forward, _ := sshLineContaining(t, log, "-O forward")
	for _, want := range []string{
		"-S", sock,
		fmt.Sprintf("-L %d:10.0.0.5:%d", port, port),
		"somehost",
	} {
		if !strings.Contains(forward, want) {
			t.Errorf("tunnel_add_forward's ssh invocation missing %q; invocation was:\n%s", want, forward)
		}
	}
}

// TestUnit_TunnelAddForward_FailsWhenNothingAnswersTheLocalPort is the
// discriminator for the whole reason this function polls rather than
// trusting `ssh -O forward`'s own exit code: measured directly (see
// remote.sh's own comment), that exit code can be 0 even when the requested
// forward never came up. With nothing ever listening on the chosen port, a
// version that trusted ssh's exit code would report success here; the real
// function must report failure, with a diagnostic explaining why.
// PORTAINER_E2E_TUNNEL_FORWARD_RETRIES is lowered so this does not cost the
// real-world ~4s budget on every run.
func TestUnit_TunnelAddForward_FailsWhenNothingAnswersTheLocalPort(t *testing.T) {
	t.Parallel()
	port := reserveFreeTCPPort(t)

	script := fmt.Sprintf(
		`export PORTAINER_E2E_TUNNEL_FORWARD_RETRIES=2
if tunnel_add_forward "somehost" %d "10.0.0.5" %d; then echo RESULT:success; else echo RESULT:failure; fi`,
		port, port)
	log, _, out, stderrOut := sourceRemoteCapturingResult(t, script)

	if !strings.Contains(out, "RESULT:failure") {
		t.Fatalf("tunnel_add_forward reported success with nothing listening on the local port; stdout was %q, ssh log:\n%s", out, log)
	}
	if !strings.Contains(stderrOut, "could not confirm the forward") {
		t.Errorf("tunnel_add_forward's failure carried no diagnostic; stderr was %q", stderrOut)
	}
}

// TestUnit_TunnelAddForward_FailsWhenSomethingAlreadyOccupiesTheLocalPort is
// the direct regression test for a false positive a live review reproduced
// against a real sshd: with an unrelated process already holding
// 127.0.0.1:<port> before this is ever called, ssh's own `-O forward` can
// silently satisfy the request on ::1 instead and still exit 0 (see
// tunnel_up's AddressFamily=inet comment for the mechanism) — and even once
// that is closed, `-O forward` against an already-occupied port simply
// fails without disturbing the pre-existing occupant, so a poll afterwards
// would still connect to it and report success for a forward that was
// never made. This is what the preflight check exists to catch, and it
// must catch it BEFORE ever asking ssh for anything: the stub ssh here
// answers every request with an unconditional exit 0 (exactly the trap),
// so if the preflight did not short-circuit first, this test would pass
// for the wrong reason.
func TestUnit_TunnelAddForward_FailsWhenSomethingAlreadyOccupiesTheLocalPort(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a local port: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	script := fmt.Sprintf(
		`if tunnel_add_forward "somehost" %d "10.0.0.5" %d; then echo RESULT:success; else echo RESULT:failure; fi`,
		port, port)
	log, _, out, stderrOut := sourceRemoteCapturingResult(t, script)

	if !strings.Contains(out, "RESULT:failure") {
		t.Fatalf("tunnel_add_forward reported success with an unrelated process already occupying the port; stdout was %q, ssh log:\n%s", out, log)
	}
	if !strings.Contains(stderrOut, "already listening") {
		t.Errorf("tunnel_add_forward's failure carried no diagnostic about the pre-existing occupant; stderr was %q", stderrOut)
	}
	if strings.Contains(log, "-O forward") {
		t.Errorf("tunnel_add_forward asked ssh for a forward despite the port already being occupied; ssh log:\n%s", log)
	}
}

// reserveFreeTCPPort returns a TCP port free at the moment of the call, by
// binding then immediately releasing it. The small window before a caller
// rebinds it is an accepted, ordinary risk in this style of test.
func reserveFreeTCPPort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a local port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ln.Close(); err != nil {
		t.Fatalf("freeing the reserved port: %v", err)
	}
	return port
}
