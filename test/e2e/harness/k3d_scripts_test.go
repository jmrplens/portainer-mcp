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

// This file runs the COMMITTED test/e2e/scripts/k3d-up.sh and k3d-down.sh
// against stub ssh/docker/k3d/kubectl/helm/go binaries, inside an isolated
// fake repository these tests fully control (their own .env, their own
// working directory) — never the real repository root's .env, and never
// any real network destination: the SSH "destination" used throughout is
// an invented hostname that resolves nowhere.
//
// What a green run here proves, and what it does NOT: for each thing it
// actually asserts on — which marker is touched, whether --gpus all is
// passed, the exact `-L` spec tunnel_add_forward is asked for, which
// destination k3d-down.sh tears down against — a stub records precisely
// what it was asked to do, so a pass means the script really made that
// decision. It is NOT a blanket claim that every ssh/kubectl/k3d invocation
// is checked in full: only what an assertion below names. It does NOT
// prove that a real k3d cluster, a real kubectl, or a real SSH connection
// to a real Docker host behaves the way the stubs assume. That is what the
// controller's live run against the real remote host is for (see
// task-7-report.md); mistaking a green run here for "remote execution
// verified" would be exactly the mistake this comment exists to prevent.
//
// A review round found this claim overstated for one specific call: the
// original version here checked the resulting PORTAINER_E2E_K8S_URL but
// never the arguments tunnel_add_forward was actually invoked with. Since
// K8S_URL is built from local_port alone and never reflects the remote
// target, two mutations at the k3d-up.sh call site — forwarding to
// 127.0.0.1 instead of $server_ip, and pointing the remote end at $api_port
// instead of $nodeport — both produced a live local port, a confident
// "forward confirmed", and an unchanged, still-correct-looking K8S_URL.
// The fix is TestUnit_K3DUpScript_RemoteRunRecordsItsOwnMarkerAndForwardsTheNodePort's
// own assertion on the exact `-O forward -L <port>:<server_ip>:<port>`
// invocation, below — the same thing tunnel_test.go's
// TestUnit_TunnelAddForward_RequestsTheForwardAndConfirmsItIsLive already
// does for the underlying function; this does it at the call site too.
//
// These tests exist because a prior review round mutated the committed
// scripts directly — dropping the "kubernetes" leg argument from either
// record_docker_host call, deleting the whole tunnel_add_forward/k8s_url
// remote branch, making --gpus all unconditional again — and every one of
// bash -n, shellcheck -x, go build ./... and go test ./test/e2e/harness/...
// stayed green throughout, because none of them actually ran the scripts'
// own remote-branch logic. CI never sets PORTAINER_E2E_REMOTE, so without
// this file the remote branch had no regression coverage at all beyond a
// manual, throwaway stub harness.

// k3dFakeRepo is an isolated copy of the repository shape k3d-up.sh and
// k3d-down.sh expect (a gitignored .env two directories above
// test/e2e/scripts, since that is where repo_root resolves to from the
// scripts' own BASH_SOURCE-derived path), built from the CURRENTLY
// COMMITTED scripts — copied fresh for every test, so a regression
// introduced into the real files is exercised the next time these tests
// run, not frozen at whatever version was copied once.
type k3dFakeRepo struct {
	e2eDir  string // .../fakerepo/test/e2e — the scripts' own working directory
	binDir  string
	logFile string
}

func newK3DFakeRepo(t *testing.T) *k3dFakeRepo {
	t.Helper()
	root := t.TempDir()
	e2eDir := filepath.Join(root, "fakerepo", "test", "e2e")
	scriptsDir := filepath.Join(e2eDir, "scripts")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("creating fake scripts dir: %v", err)
	}
	for _, name := range []string{"k3d-up.sh", "k3d-down.sh", "lib.sh", "remote.sh"} {
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
	// PORTAINER_E2E_DOCKER_SSH names an invented host that resolves nowhere —
	// never truenas, never a real address. docker_ssh_dest only ever reads
	// this string; nothing in these tests dials it, since ssh itself is
	// stubbed on PATH below.
	envContents := "PORTAINER_LICENSE=fake-licence-not-real\n" +
		"PORTAINER_E2E_DOCKER_SSH=fake-remote-host-invented-for-this-test.invalid\n"
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
	// ssh: answers the GPU probe (on_docker_host's nvidia-smi check) per
	// STUB_GPU_PRESENT, and otherwise just logs and succeeds — the same
	// "reports 0 regardless" shape production ssh has for -O forward
	// (see remote.sh), so a script that trusted it blindly would not be
	// caught by this stub alone; tunnel_add_forward's own preflight/poll are
	// what have to do the real work, and are exercised for real (not just
	// logged) via the real listener these tests start.
	writeStub("ssh", `#!/usr/bin/env bash
echo "SSH_CALL: $*" >> "$LOGFILE"
if [[ "$*" == *"nvidia-smi"* ]]; then
    if [[ "${STUB_GPU_PRESENT:-0}" == "1" ]]; then
        echo "NVIDIA GeForce RTX FAKE"
        exit 0
    fi
    exit 1
fi
if [[ "$*" == *"-O forward"* ]]; then
    touch "${LOGFILE}.forward-requested"
fi
exit 0
`)
	writeStub("docker", `#!/usr/bin/env bash
echo "DOCKER_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>}" >> "$LOGFILE"
if [[ "$1" == "inspect" ]]; then
    echo "${STUB_SERVER_IP:-10.99.0.5}"
fi
exit 0
`)
	writeStub("k3d", `#!/usr/bin/env bash
echo "K3D_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>}" >> "$LOGFILE"
exit 0
`)
	writeStub("kubectl", `#!/usr/bin/env bash
echo "KUBECTL_CALL: $*" >> "$LOGFILE"
args="$*"
case "$args" in
    *"config set-cluster"*) exit 0 ;;
    *"create namespace"*) printf 'apiVersion: v1\nkind: Namespace\nmetadata:\n  name: portainer\n'; exit 0 ;;
    *"apply -f -"*) cat > /dev/null; exit 0 ;;
    *"logs deploy/portainer"*) echo "setup_token=a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4a1b2c3d4"; exit 0 ;;
    *"get svc portainer"*) echo "${STUB_NODEPORT:-30443}"; exit 0 ;;
    *"get pod"*"app.kubernetes.io/name=portainer"*) echo "portainer-fake-pod-0"; exit 0 ;;
    *"debug -q"*) printf -- '-----BEGIN CERTIFICATE-----\nFAKECERTDATANOTREAL\n-----END CERTIFICATE-----\n'; exit 0 ;;
    *) exit 0 ;;
esac
`)
	writeStub("helm", `#!/usr/bin/env bash
echo "HELM_CALL: $*" >> "$LOGFILE"
exit 0
`)
	writeStub("go", `#!/usr/bin/env bash
echo "GO_CALL: $* | K8S_URL=${PORTAINER_E2E_K8S_URL:-<unset>}" >> "$LOGFILE"
exit 0
`)

	return &k3dFakeRepo{e2eDir: e2eDir, binDir: binDir, logFile: logFile}
}

// run executes one of the fake repo's own scripts (k3d-up.sh or
// k3d-down.sh) with the stub PATH prepended and the given extra environment
// entries, and returns the script's own combined stdout+stderr, its ssh/
// docker/k3d/kubectl/helm/go invocation log, and its exit code.
func (r *k3dFakeRepo) run(t *testing.T, scriptName string, extraEnv ...string) (output, log string, exitCode int) {
	t.Helper()
	script := filepath.Join(r.e2eDir, "scripts", scriptName)
	cmd := exec.Command("bash", script)
	env := append(os.Environ(), "PATH="+r.binDir+":"+os.Getenv("PATH"), "LOGFILE="+r.logFile)
	env = append(env, extraEnv...)
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
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

func (r *k3dFakeRepo) marker(leg string) (string, bool) {
	name := ".docker-host"
	if leg != "" {
		name = ".docker-host-" + leg
	}
	data, err := os.ReadFile(filepath.Join(r.e2eDir, name))
	if os.IsNotExist(err) {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return strings.TrimRight(string(data), "\n"), true
}

// startListenerAfterForwardRequested starts a real TCP listener at port
// only once the stub ssh has actually logged an "-O forward" request
// (signalled by a marker file the stub touches the instant it sees that
// call), standing in for "the NodePort forward became live shortly after
// tunnel_add_forward requested it" — the same idea
// TestUnit_TunnelAddForward_RequestsTheForwardAndConfirmsItIsLive uses, and
// for the same reason: present only from partway through the call, so a
// pass here proves the confirmation poll actually ran, not merely that
// something happened to already be there.
//
// An earlier version used a fixed delay (time.Sleep(150ms)) instead of this
// marker. That raced tunnel_add_forward's own PREFLIGHT check under load:
// this test's baseline (correct, unmutated) run intermittently failed —
// reproduced directly, 1 failure in an 11-run sample under a loaded
// system — because the cumulative overhead of the dozen-odd stub process
// spawns k3d-up.sh makes before ever reaching tunnel_add_forward
// occasionally exceeded 150ms, so the listener came up BEFORE the
// preflight check ran, and the preflight correctly rejected it as a
// pre-existing occupant. Tying the listener's start to the actual
// -O forward request (which the preflight always precedes, by
// construction) removes the race instead of just narrowing it.
func startListenerAfterForwardRequested(t *testing.T, forwardMarker string, port int) {
	t.Helper()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(forwardMarker); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
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
}

// TestUnit_K3DUpScript_RemoteRunRecordsItsOwnMarkerAndForwardsTheNodePort is
// the baseline (unmutated) remote run of k3d-up.sh. It pins, in one
// end-to-end pass, five decisions a review found completely uncovered:
//
//  1. record_docker_host is called WITH the "kubernetes" leg argument — the
//     Kubernetes marker is written, and the compose marker (.docker-host,
//     with no suffix) is not.
//  2. --gpus all is passed to `k3d cluster create` only when the GPU probe
//     (stubbed here to report no GPU) found one — absent in this run.
//  3. tunnel_add_forward is asked for the RIGHT forward: `-L
//     <port>:<server_ip>:<port>`, not merely some forward. This is the
//     sharper check a first round of this test omitted: K8S_URL alone
//     (checked below too) is built from local_port alone and stays
//     identical whether the remote end names server_ip, 127.0.0.1, or the
//     API port — so checking only the resulting URL let a misaimed forward
//     through silently. See this file's own top-of-file comment for the
//     two mutations that exposed this.
//  4. PORTAINER_E2E_K8S_URL passed to the provisioner is the forwarded
//     127.0.0.1 address, not server_ip:nodeport directly.
//  5. The API port's kubeconfig rewrite (kubectl config set-cluster) runs.
func TestUnit_K3DUpScript_RemoteRunRecordsItsOwnMarkerAndForwardsTheNodePort(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)
	const serverIP = "172.30.5.7"

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP="+serverIP,
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (remote, no GPU) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if dest, ok := repo.marker("kubernetes"); !ok || dest != "fake-remote-host-invented-for-this-test.invalid" {
		t.Errorf("kubernetes marker = (%q, %v), want the fake destination recorded", dest, ok)
	}
	if _, ok := repo.marker(""); ok {
		t.Errorf("k3d-up.sh wrote the compose leg's own marker (.docker-host); it must never touch that one")
	}

	if strings.Contains(log, "--gpus all") {
		t.Errorf("k3d cluster create passed --gpus all with no GPU detected; log:\n%s", log)
	}

	// The exact forward request, not merely "a forward happened". A
	// misaimed forward (wrong remote host, or the API port's number instead
	// of the NodePort's) still produces a live local port, a confirmed
	// poll, and — since K8S_URL is built from local_port alone — an
	// UNCHANGED, still-correct-looking URL below. Only inspecting the
	// actual `-L` spec catches that.
	forward, _ := sshLineContaining(t, log, "-O forward")
	wantSpec := fmt.Sprintf("-L %d:%s:%d", port, serverIP, port)
	if !strings.Contains(forward, wantSpec) {
		t.Errorf("tunnel_add_forward was not asked to forward to the k3d server's own address and NodePort; want %q in its ssh invocation:\n%s", wantSpec, forward)
	}

	wantURL := fmt.Sprintf("K8S_URL=https://127.0.0.1:%d", port)
	if !strings.Contains(log, wantURL) {
		t.Errorf("provisioner was not given the forwarded NodePort address; want %q in log:\n%s", wantURL, log)
	}
	if strings.Contains(log, fmt.Sprintf("K8S_URL=https://%s:%d", serverIP, port)) {
		t.Errorf("provisioner was given the raw server_ip:nodeport address instead of the forwarded one; log:\n%s", log)
	}

	if !strings.Contains(log, "config set-cluster") {
		t.Errorf("kubectl config set-cluster was never invoked; log:\n%s", log)
	}
}

// TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate is the
// positive direction of check 2 above: a GPU-less run proves the flag is
// absent, but only a GPU-present run proves it is EVER added at all — a
// version that dropped the whole conditional and always omitted the flag
// would still pass the GPU-less test.
func TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		"STUB_GPU_PRESENT=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (remote, GPU present) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if !strings.Contains(log, "--gpus all") {
		t.Errorf("k3d cluster create did not pass --gpus all despite a detected GPU; log:\n%s", log)
	}
}

// TestUnit_K3DDownScript_ClearsItsOwnMarkerAndLeavesTheComposeMarkerAlone
// pins the teardown half of the marker split: with BOTH markers present and
// DIFFERENT, tearing down the Kubernetes leg must read and clear only its
// own, never the compose leg's — the exact cross-leg misdirection an
// earlier task's review caught in down.sh's own probe.
func TestUnit_K3DDownScript_ClearsItsOwnMarkerAndLeavesTheComposeMarkerAlone(t *testing.T) {
	repo := newK3DFakeRepo(t)
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host-kubernetes"),
		[]byte("fake-remote-host-invented-for-this-test.invalid\n"), 0o644); err != nil {
		t.Fatalf("seeding the kubernetes marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host"),
		[]byte("a-different-host-belonging-to-the-compose-leg.invalid\n"), 0o644); err != nil {
		t.Fatalf("seeding the compose marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".estate.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("seeding a placeholder estate file: %v", err)
	}

	output, log, code := repo.run(t, "k3d-down.sh")
	if code != 0 {
		t.Fatalf("k3d-down.sh exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}

	if _, ok := repo.marker("kubernetes"); ok {
		t.Errorf("k3d-down.sh did not clear its own (kubernetes) marker")
	}
	if dest, ok := repo.marker(""); !ok || dest != "a-different-host-belonging-to-the-compose-leg.invalid" {
		t.Errorf("k3d-down.sh disturbed the compose leg's marker; now (%q, %v), want it untouched", dest, ok)
	}
	if !strings.Contains(log, "DOCKER_HOST=ssh://fake-remote-host-invented-for-this-test.invalid") {
		t.Errorf("k3d-down.sh did not tear down against the kubernetes leg's OWN recorded destination; log:\n%s", log)
	}
}
