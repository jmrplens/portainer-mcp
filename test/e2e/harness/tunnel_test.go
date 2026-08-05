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
// returns the log's contents, so a test can assert on exactly how ssh was
// invoked without needing a reachable host.
func sourceRemote(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	log := filepath.Join(dir, "ssh.log")
	stub := filepath.Join(dir, "ssh")
	body := "#!/usr/bin/env bash\nprintf '%s\\n' \"$*\" >> " + log + "\nexit 0\n"
	if err := os.WriteFile(stub, []byte(body), 0o700); err != nil {
		t.Fatalf("writing ssh stub: %v", err)
	}
	remote, err := filepath.Abs("../scripts/remote.sh")
	if err != nil {
		t.Fatalf("resolving remote.sh: %v", err)
	}
	full := "export PATH=" + dir + ":$PATH\nexport PORTAINER_E2E_TUNNEL_SOCK=" + filepath.Join(dir, "t.sock") + "\nsource " + remote + "\n" + script
	cmd := exec.Command("bash", "-euo", "pipefail", "-c", full)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if _, err := cmd.Output(); err != nil {
		t.Fatalf("bash script failed: %v\nstderr:\n%s", err, stderr.String())
	}
	data, err := os.ReadFile(log)
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		t.Fatalf("reading ssh log: %v", err)
	}
	return string(data)
}

func TestUnit_TunnelUp_NoDestinationInvokesNoSSH(t *testing.T) {
	t.Parallel()
	if got := sourceRemote(t, `tunnel_up "" 19000 19001`); got != "" {
		t.Errorf("tunnel_up with no destination invoked ssh: %q", got)
	}
}

func TestUnit_TunnelUp_ForwardsEveryRequestedPortFromTheRemoteLoopback(t *testing.T) {
	t.Parallel()
	got := sourceRemote(t, `tunnel_up "somehost" 19000 19001`)
	for _, want := range []string{
		"-L 19000:127.0.0.1:19000",
		"-L 19001:127.0.0.1:19001",
		"somehost",
		// ExitOnForwardFailure turns "the port was already taken" into a
		// non-zero exit here, rather than a suite that starts and then fails
		// against whatever else is listening on 19000.
		"-o ExitOnForwardFailure=yes",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("tunnel_up did not pass %q to ssh; invocation was:\n%s", want, got)
		}
	}
}

func TestUnit_TunnelDown_NoDestinationInvokesNoSSH(t *testing.T) {
	t.Parallel()
	if got := sourceRemote(t, `tunnel_down ""`); got != "" {
		t.Errorf("tunnel_down with no destination invoked ssh: %q", got)
	}
}

func TestUnit_TunnelDown_AsksTheMasterToExit(t *testing.T) {
	t.Parallel()
	got := sourceRemote(t, `tunnel_down "somehost"`)
	if !strings.Contains(got, "-O exit") {
		t.Errorf("tunnel_down did not ask the control master to exit; invocation was:\n%s", got)
	}
}
