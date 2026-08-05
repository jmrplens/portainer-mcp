package harness

import (
	"errors"
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
// destination k3d-down.sh tears down against, whether the nvidia device
// plugin gets its shim and its DaemonSet applied — a stub records precisely
// what it was asked to do, so a pass means the script really made that
// decision. It is NOT a blanket claim that every ssh/kubectl/k3d invocation
// is checked in full: only what an assertion below names. It does NOT
// prove that a real k3d cluster, a real kubectl, or a real SSH connection
// to a real Docker host behaves the way the stubs assume — and, for the GPU
// branch specifically, it does NOT prove the DaemonSet itself works: a
// stubbed `kubectl apply` and `rollout status` both exit 0 unconditionally,
// so this can only prove k3d-up.sh DECIDES correctly whether to install it,
// never that the manifest itself makes a real node advertise nvidia.com/gpu.
// That is what the controller's live run against the real remote GPU host is
// for (see task-7-report.md and task-8-report.md); mistaking a green run
// here for "remote execution verified" or "the GPU is really advertised"
// would be exactly the mistake this comment exists to prevent.
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
	// ssh: answers the driver probe (on_docker_host's nvidia-smi check) per
	// STUB_GPU_PRESENT and the SEPARATE container-toolkit probe
	// (on_docker_host's nvidia-ctk check, via gpu_cdi_spec) per
	// STUB_TOOLKIT_PRESENT — deliberately two independent knobs, not one,
	// because that is exactly the distinction C2 exists to cover: nvidia-smi
	// ships with the driver alone, and a host can have a working driver with
	// no container toolkit installed at all. Everything else just logs and
	// succeeds — the same "reports 0 regardless" shape production ssh has for
	// -O forward (see remote.sh), so a script that trusted it blindly would
	// not be caught by this stub alone; tunnel_add_forward's own
	// preflight/poll are what have to do the real work, and are exercised for
	// real (not just logged) via the real listener these tests start.
	writeStub("ssh", `#!/usr/bin/env bash
echo "SSH_CALL: $*" >> "$LOGFILE"
if [[ "$*" == *"nvidia-smi"* ]]; then
    if [[ "${STUB_GPU_PRESENT:-0}" == "1" ]]; then
        echo "NVIDIA GeForce RTX FAKE"
        exit 0
    fi
    exit 1
fi
if [[ "$*" == *"nvidia-ctk"* ]]; then
    if [[ "${STUB_TOOLKIT_PRESENT:-0}" == "1" ]]; then
        printf 'cdiVersion: 0.7.0\nkind: nvidia.com/gpu\ndevices:\n    - name: all\n'
        exit 0
    fi
    exit 1
fi
if [[ "$*" == *"-O forward"* ]]; then
    touch "${LOGFILE}.forward-requested"
fi
exit 0
`)
	// docker: answers the k3d node's own GPU-device check (`docker exec ...
	// ls /dev/nvidia0`, k3d-up.sh's gate on installing the device plugin) with
	// BOTH STUB_GPU_PRESENT and STUB_TOOLKIT_PRESENT required — in reality the
	// node only gets the device file when --gpus all was actually passed to
	// `k3d cluster create`, and this harness's own k3d-up.sh now withholds
	// that flag unless the toolkit probe (nvidia-ctk, above) succeeded too, so
	// a stub answering on STUB_GPU_PRESENT alone would report a device the
	// real script's own gate never granted. Any OTHER "exec" call (the
	// nvidia-ctk shim write) falls through to the generic success below, so
	// it is only ever checked through the log, never made to fail here.
	//
	// The "libcuda.so" branch answers k3d-up.sh's own probe for the driver
	// library directory the device plugin manifest hardcodes
	// (/usr/lib/x86_64-linux-gnu). It succeeds by default — every existing
	// GPU-present test in this file expects the device plugin to be applied
	// — and only STUB_LIBCUDA_MISSING=1 makes it fail, standing in for a node
	// whose driver libraries live somewhere else entirely (arm64, a
	// non-Debian base image).
	writeStub("docker", `#!/usr/bin/env bash
echo "DOCKER_CALL: $* | DOCKER_HOST=${DOCKER_HOST:-<unset>}" >> "$LOGFILE"
if [[ "$1" == "inspect" ]]; then
    echo "${STUB_SERVER_IP:-10.99.0.5}"
    exit 0
fi
if [[ "$1" == "exec" && "$*" == *"nvidia0"* ]]; then
    if [[ "${STUB_GPU_PRESENT:-0}" == "1" && "${STUB_TOOLKIT_PRESENT:-0}" == "1" ]]; then
        exit 0
    fi
    exit 1
fi
if [[ "$1" == "exec" && "$*" == *"libcuda.so"* ]]; then
    if [[ "${STUB_LIBCUDA_MISSING:-0}" == "1" ]]; then
        exit 1
    fi
    exit 0
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
    # k3d-up.sh matches setup_token=[0-9a-f]{64}, so the stub has to emit a
    # real 64-hex-character token or the script would find nothing and the
    # test would pass against a broken regex. The value is assembled at
    # runtime rather than written out: a 64-character hex literal in the
    # source is indistinguishable from a leaked credential to a secret
    # scanner, and GitGuardian failed this branch's first CI run on exactly
    # that. Do not "simplify" this back to a literal.
    *"logs deploy/portainer"*) printf 'setup_token=%s\n' "$(printf 'abcdef01%.0s' 1 2 3 4 5 6 7 8)"; exit 0 ;;
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
	return r.runWithPath(t, scriptName, r.binDir+":"+os.Getenv("PATH"), extraEnv...)
}

// runWithPath is run's variant for a test that needs to control PATH itself
// — specifically TestUnit_K3DUpScript_MissingToolLeavesAnExistingMarkerUntouched,
// which has to simulate a tool genuinely ABSENT rather than merely shadowed:
// the machine running these tests has its own k3d/kubectl/helm installed
// (this repository's own e2e tooling needs them), so run()'s
// binDir-then-real-PATH order would still find the real one further along
// even with its stub removed from binDir.
func (r *k3dFakeRepo) runWithPath(t *testing.T, scriptName, path string, extraEnv ...string) (output, log string, exitCode int) {
	t.Helper()
	script := filepath.Join(r.e2eDir, "scripts", scriptName)
	cmd := exec.CommandContext(t.Context(), "bash", script)
	env := append(os.Environ(), "PATH="+path, "LOGFILE="+r.logFile)
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
	ctx := t.Context()
	go func() {
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(forwardMarker); err == nil {
				break
			}
			time.Sleep(5 * time.Millisecond)
		}
		var lc net.ListenConfig
		ln, err := lc.Listen(ctx, "tcp", fmt.Sprintf("127.0.0.1:%d", port))
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
//  6. With no GPU on the node (the docker exec ls /dev/nvidia0 probe stubbed
//     to fail — the default here), the nvidia device plugin is neither
//     applied nor given its nvidia-ctk shim, and the script says so on
//     stderr. TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate
//     below is this check's positive direction.
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

	// The device-plugin install is gated on the NODE's own device file
	// (docker exec ... ls /dev/nvidia0), stubbed to fail by default (see
	// newK3DFakeRepo's docker stub) — nothing in this run ever sets
	// STUB_GPU_PRESENT. TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate
	// is the run that proves the opposite branch is ever taken at all; this
	// half alone would pass even if that branch were deleted outright, the
	// same reason the sibling test's own doc comment gives for --gpus all.
	if strings.Contains(log, "nvidia-ctk") {
		t.Errorf("k3d-up.sh wrote the nvidia-ctk shim with no GPU on the node; log:\n%s", log)
	}
	if strings.Contains(log, "nvidia-device-plugin.yaml") {
		t.Errorf("k3d-up.sh applied the nvidia device plugin with no GPU on the node; log:\n%s", log)
	}
	if !strings.Contains(output, "no GPU on the kubernetes node: GPU suites will skip") {
		t.Errorf("k3d-up.sh did not report the no-GPU-on-node message; output:\n%s", output)
	}
}

// TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate is the
// positive direction of check 2 above: a GPU-less run proves the flag is
// absent, but only a GPU-present run proves it is EVER added at all — a
// version that dropped the whole conditional and always omitted the flag
// would still pass the GPU-less test. It sets BOTH STUB_GPU_PRESENT (the
// driver, via nvidia-smi) and STUB_TOOLKIT_PRESENT (the container toolkit,
// via nvidia-ctk cdi generate): this is the "everything a real GPU host
// needs" case, and TestUnit_K3DUpScript_DriverPresentButToolkitAbsentDoesNotPassGpusAll
// below is what proves the driver alone is not enough.
//
// STUB_GPU_PRESENT and STUB_TOOLKIT_PRESENT together also drive the docker
// stub's answer to the node's own `ls /dev/nvidia0` probe (see
// newK3DFakeRepo), so this same run is also the one place proving the
// device-plugin install itself: the nvidia-ctk shim is written on the node,
// the DaemonSet manifest is applied, its rollout is waited on, and the script
// says so on stderr. Without a GPU on the node (the sibling test above), none
// of the four should happen — that test's own negative assertions are what
// makes this one non-trivial: a version that installed the plugin
// unconditionally would still pass this test alone.
func TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		"STUB_GPU_PRESENT=1",
		"STUB_TOOLKIT_PRESENT=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (remote, GPU present) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if !strings.Contains(log, "--gpus all") {
		t.Errorf("k3d cluster create did not pass --gpus all despite a detected GPU; log:\n%s", log)
	}

	// The bare substring "nvidia-ctk" is deliberately not used here: it also
	// matches gpu_cdi_spec's own TOOLKIT PROBE over ssh (see
	// TestUnit_K3DUpScript_DriverPresentButToolkitAbsentDoesNotPassGpusAll's
	// own doc), so it cannot tell "the shim was written" apart from "the
	// probe merely ran". TestUnit_K3DUpScript_InstallsTheNvidiaCtkShimOnEveryNode
	// below is the one place that pins the actual shim-install invocations,
	// on both nodes.
	if !strings.Contains(log, "DOCKER_CALL: exec k3d-portainer-mcp-e2e-server-0 sh -c printf") {
		t.Errorf("k3d-up.sh did not write the nvidia-ctk shim on the server node despite a GPU on the node; log:\n%s", log)
	}
	if !strings.Contains(log, "apply -f ./k8s/nvidia-device-plugin.yaml") {
		t.Errorf("k3d-up.sh did not apply the nvidia device plugin despite a GPU on the node; log:\n%s", log)
	}
	// The exact invocation, not "rollout status" and "daemonset/nvidia-device-
	// plugin" checked independently: two SEPARATE kubectl calls (one carrying
	// "-n kube-system" for something else entirely, another doing "rollout
	// status daemonset/nvidia-device-plugin" with no namespace flag at all)
	// would satisfy two bare Contains checks while never actually running
	// `kubectl -n kube-system rollout status daemonset/nvidia-device-plugin`
	// -- the exact command that hangs 180s and fails on a real cluster if the
	// namespace or the DaemonSet's own name ever drifts from what this script
	// assumes (see gpu_manifest_test.go's own identity assertions for the
	// manifest side of this pairing).
	wantRollout := "KUBECTL_CALL: --context k3d-portainer-mcp-e2e -n kube-system rollout status daemonset/nvidia-device-plugin --timeout=180s"
	if !strings.Contains(log, wantRollout) {
		t.Errorf("k3d-up.sh did not wait for the device plugin daemonset's rollout with the expected invocation; want %q in log:\n%s", wantRollout, log)
	}
	if !strings.Contains(output, "gpu advertised to the kubernetes leg") {
		t.Errorf("k3d-up.sh did not report the gpu-advertised message despite a GPU on the node; output:\n%s", output)
	}
}

// TestUnit_K3DUpScript_InstallsTheNvidiaCtkShimOnEveryNode is I6's own
// regression test. The DaemonSet tolerates everything (`operator: Exists`,
// see test/e2e/k8s/nvidia-device-plugin.yaml) and is scheduled onto every
// node of the --agents 1 cluster this script creates: server-0 AND agent-0.
// An earlier version of this script wrote the nvidia-ctk shim only onto
// server-0. kubectl's own node listing is not guaranteed to sort the server
// node first — test/e2e/suite/fixtures_test.go's own `.items[0]` read sorts
// to agent-0 in practice — so a plugin pod scheduled on agent-0 would try to
// invoke a hook that was never installed there.
func TestUnit_K3DUpScript_InstallsTheNvidiaCtkShimOnEveryNode(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)

	_, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		"STUB_GPU_PRESENT=1",
		"STUB_TOOLKIT_PRESENT=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (remote, GPU present) exited %d; log:\n%s", code, log)
	}
	for _, node := range []string{"k3d-portainer-mcp-e2e-server-0", "k3d-portainer-mcp-e2e-agent-0"} {
		want := "DOCKER_CALL: exec " + node + " sh -c printf"
		if !strings.Contains(log, want) {
			t.Errorf("k3d-up.sh did not write the nvidia-ctk shim on %s; log:\n%s", node, log)
		}
	}
}

// TestUnit_K3DUpScript_MissingDriverLibrariesSkipsTheDevicePluginRatherThanAborting
// is the regression test for the node-library probe k3d-up.sh now runs before
// applying the device plugin. test/e2e/k8s/nvidia-device-plugin.yaml
// hardcodes /usr/lib/x86_64-linux-gnu (a Debian/amd64 path) as the node's
// driver library directory. Without this probe, a GPU-and-toolkit-detected
// run on a node whose driver libraries live elsewhere would apply the
// manifest anyway, then wait the full 180s on `rollout status` before
// aborting under set -euo pipefail — with the cluster (and, remotely, its
// SSH tunnel) left running, since `k3d cluster create` installs no cleanup
// trap.
//
// The driver, toolkit and /dev/nvidia0 stubs all report success — this is
// otherwise exactly TestUnit_K3DUpScript_GPUDetectedPassesGpusAllToClusterCreate's
// own setup — and only the libcuda probe is made to fail, isolating this
// specific gate from the others already covered above.
func TestUnit_K3DUpScript_MissingDriverLibrariesSkipsTheDevicePluginRatherThanAborting(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		"STUB_GPU_PRESENT=1",
		"STUB_TOOLKIT_PRESENT=1",
		"STUB_LIBCUDA_MISSING=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (gpu present, driver libraries missing on the node) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if strings.Contains(log, "apply -f ./k8s/nvidia-device-plugin.yaml") {
		t.Errorf("k3d-up.sh applied the nvidia device plugin despite the node missing its hardcoded driver library path; log:\n%s", log)
	}
	if strings.Contains(log, "rollout status") {
		t.Errorf("k3d-up.sh waited on the device plugin's rollout despite never applying it; log:\n%s", log)
	}
	for _, want := range []string{"/usr/lib/x86_64-linux-gnu", "nvidia-device-plugin.yaml"} {
		if !strings.Contains(output, want) {
			t.Errorf("k3d-up.sh's warning did not name %q so a reader knows where to look; output:\n%s", want, output)
		}
	}
	if strings.Contains(output, "gpu advertised to the kubernetes leg") {
		t.Errorf("k3d-up.sh reported the gpu as advertised despite skipping the device plugin; output:\n%s", output)
	}
}

// TestUnit_K3DUpScript_DriverPresentButToolkitAbsentDoesNotPassGpusAll is
// C2's own regression test: a host can have a working NVIDIA driver
// (nvidia-smi answers) with no NVIDIA Container Toolkit installed at all —
// nvidia-smi ships with the driver alone. On such a host `k3d cluster create
// --gpus all` fails outright with "failed to discover GPU vendor from CDI: no
// known GPU vendor found" and rolls back the whole cluster, which is exactly
// the "make e2e-k8s-up now breaks on a local host with a driver but no
// toolkit" regression the review named. STUB_GPU_PRESENT=1 alone (without
// STUB_TOOLKIT_PRESENT) reproduces that host: nvidia-smi answers, nvidia-ctk
// does not.
//
// The log is checked for the shim's own install command
// ("chmod 0755 /usr/bin/nvidia-ctk"), not the bare substring "nvidia-ctk":
// gpu_cdi_spec's own TOOLKIT PROBE also invokes nvidia-ctk over ssh (to find
// out whether it exists at all), so a bare substring check would find that
// probe and pass even if the gate were deleted outright — the probe runs on
// every GPU-detected host regardless of what it finds, and only the shim
// write is the thing this test must prove absent.
func TestUnit_K3DUpScript_DriverPresentButToolkitAbsentDoesNotPassGpusAll(t *testing.T) {
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
		t.Fatalf("k3d-up.sh (driver present, no toolkit) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if strings.Contains(log, "--gpus all") {
		t.Errorf("k3d cluster create passed --gpus all with the driver present but no container toolkit; log:\n%s", log)
	}
	if !strings.Contains(output, "no usable nvidia container toolkit found") {
		t.Errorf("k3d-up.sh did not report the driver-without-toolkit warning; output:\n%s", output)
	}
	if strings.Contains(log, "chmod 0755 /usr/bin/nvidia-ctk") {
		t.Errorf("k3d-up.sh wrote the nvidia-ctk shim onto the node despite no container toolkit on the docker host; log:\n%s", log)
	}
	if strings.Contains(log, "nvidia-device-plugin.yaml") {
		t.Errorf("k3d-up.sh applied the nvidia device plugin despite no container toolkit on the docker host; log:\n%s", log)
	}
}

// TestUnit_K3DUpScript_ExitsBeforeProvisioningWhenTheForwardIsNeverConfirmed
// pins the FAILURE branch of the tunnel_add_forward call in k3d-up.sh
// (`if ! tunnel_add_forward ...; then echo ...; exit 1; fi`), which none of
// this file's other tests exercise — they all supply a stand-in listener
// that DOES appear, so the poll always succeeds and this branch is never
// taken.
//
// Worth pinning specifically because an unconfirmed forward does not fail
// loudly on its own: 127.0.0.1:<nodeport> is then either dead (an obviously
// broken URL the provisioner would fail against immediately, loudly) or
// held by something foreign (a URL that LOOKS fine and routes the
// provisioner somewhere that is not Portainer) — the second case is
// exactly the "routes elsewhere" outcome the last three rounds of this
// task exist to eliminate, and it is indistinguishable from success unless
// the script actually stops before ever handing that URL to the
// provisioner.
//
// Both halves below are asserted because either alone is insufficient: a
// non-zero exit code alone would not catch a script that ran the
// provisioner FIRST and failed afterwards (still a live URL handed to a
// real process, just with an exit code appended at the end); the absence
// of the provisioner call alone would not catch a script that swallowed
// the failure and exited 0 anyway.
//
// A third thing is asserted for a reason that has nothing to do with the
// forward itself: the kubernetes marker must still be recorded even though
// this run fails, and the compose marker must still be untouched.
// record_docker_host "$ssh_dest" kubernetes runs early in the real script —
// before cluster creation, before any of this — specifically so a run that
// dies partway through remains tearable down against the right host. That
// ordering is not enforced by anything the language checks; it is a
// convention a later edit could disturb without any type error, any lint
// warning, or any OTHER assertion in this file noticing, because every
// other test here only exercises the SUCCESS path, where the marker gets
// written either way regardless of when the line runs. If it moved to just
// before the final "kubernetes leg ready" echo — a plausible-looking
// "consolidate the bookkeeping at the end" edit — a run that fails here,
// before ever reaching that echo, would leave NO marker at all. The
// consequence is not merely "a test would have caught this": with no
// marker, `k3d-down.sh` reads no destination, never exports DOCKER_HOST,
// and runs `k3d cluster delete` against the LOCAL daemon, where no such
// cluster exists — a silent no-op, exit 0, no warning. The real cluster,
// its Business Edition licence assignment, and the SSH master tunnel_up
// opened all stay orphaned on somebody's actual machine, and the teardown
// that was supposed to catch that reports success.
//
// No startListenerAfterForwardRequested call here — that omission is the
// whole point: nothing ever answers this port, so tunnel_add_forward's
// poll must time out. PORTAINER_E2E_TUNNEL_FORWARD_RETRIES is lowered so
// the deliberately-unconfirmed poll does not cost the real ~4s budget.
func TestUnit_K3DUpScript_ExitsBeforeProvisioningWhenTheForwardIsNeverConfirmed(t *testing.T) {
	repo := newK3DFakeRepo(t)
	port := reserveFreeTCPPort(t)

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
		"PORTAINER_E2E_TUNNEL_FORWARD_RETRIES=2",
	)
	if code == 0 {
		t.Fatalf("k3d-up.sh exited 0 despite the NodePort forward never being confirmed; output:\n%s\nlog:\n%s", output, log)
	}
	if strings.Contains(log, "GO_CALL") {
		t.Errorf("k3d-up.sh invoked the provisioner despite the NodePort forward never being confirmed; log:\n%s", log)
	}

	// The marker must survive this failure — see the comment above for why
	// its absence is not merely wrong bookkeeping, but an orphaned remote
	// cluster, licence and tunnel that teardown will silently fail to find.
	if dest, ok := repo.marker("kubernetes"); !ok || dest != "fake-remote-host-invented-for-this-test.invalid" {
		t.Errorf("kubernetes marker = (%q, %v) after a failed run, want the fake destination still recorded so teardown can find the right host", dest, ok)
	}
	if _, ok := repo.marker(""); ok {
		t.Errorf("k3d-up.sh wrote the compose leg's own marker (.docker-host) on a failed run; it must never touch that one")
	}
}

// TestUnit_K3DUpScript_RefusesAPlainRunWhenARemoteKubernetesMarkerExists is
// I2's own regression test for the Kubernetes leg, mirroring
// TestUnit_UpScript_RefusesAPlainRunWhenARemoteMarkerExists in
// compose_scripts_test.go for the compose leg: a `make e2e-k8s-up-remote`
// followed by a plain `make e2e-k8s-up` must refuse rather than silently
// orphan the still-recorded remote cluster, its licence and its open ssh
// master. record_docker_host's own "empty destination deletes the marker"
// rule would otherwise wipe the ONLY record of where that cluster is.
func TestUnit_K3DUpScript_RefusesAPlainRunWhenARemoteKubernetesMarkerExists(t *testing.T) {
	repo := newK3DFakeRepo(t)
	const existingHost = "existing-kubernetes-host.invalid"
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host-kubernetes"),
		[]byte(existingHost+"\n"), 0o644); err != nil {
		t.Fatalf("seeding the kubernetes marker: %v", err)
	}

	output, log, code := repo.run(t, "k3d-up.sh")
	if code == 0 {
		t.Fatalf("k3d-up.sh (plain run) succeeded despite an existing remote kubernetes marker; output:\n%s\nlog:\n%s", output, log)
	}
	if !strings.Contains(output, "refusing to continue") {
		t.Errorf("k3d-up.sh did not report a refusal; output:\n%s", output)
	}
	if strings.Contains(log, "K3D_CALL:") {
		t.Errorf("k3d-up.sh created a cluster despite refusing the host switch; log:\n%s", log)
	}
	if dest, ok := repo.marker("kubernetes"); !ok || dest != existingHost {
		t.Errorf("kubernetes marker changed after a refused run: (%q, %v), want it untouched at %q", dest, ok, existingHost)
	}
}

// TestUnit_K3DUpScript_RefusesSwitchingToADifferentRemoteKubernetesHost is the
// same guard's other direction: an existing remote kubernetes marker and a
// NEW run naming a different remote host must also refuse.
func TestUnit_K3DUpScript_RefusesSwitchingToADifferentRemoteKubernetesHost(t *testing.T) {
	repo := newK3DFakeRepo(t)
	const otherHost = "a-different-remote-kubernetes-host.invalid"
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host-kubernetes"),
		[]byte(otherHost+"\n"), 0o644); err != nil {
		t.Fatalf("seeding the kubernetes marker: %v", err)
	}

	output, log, code := repo.run(t, "k3d-up.sh", "PORTAINER_E2E_REMOTE=1")
	if code == 0 {
		t.Fatalf("k3d-up.sh (remote) succeeded despite an existing marker naming a different host; output:\n%s\nlog:\n%s", output, log)
	}
	if !strings.Contains(output, "refusing to continue") {
		t.Errorf("k3d-up.sh did not report a refusal; output:\n%s", output)
	}
	if strings.Contains(log, "K3D_CALL:") {
		t.Errorf("k3d-up.sh created a cluster despite refusing the host switch; log:\n%s", log)
	}
	if dest, ok := repo.marker("kubernetes"); !ok || dest != otherHost {
		t.Errorf("kubernetes marker changed after a refused run: (%q, %v), want it untouched at %q", dest, ok, otherHost)
	}
}

// TestUnit_K3DUpScript_AllowsReRunningAgainstTheSameRecordedKubernetesHost
// proves the guard is not a blanket refusal: a second `make e2e-k8s-up-remote`
// against the SAME host an earlier run already recorded must succeed.
func TestUnit_K3DUpScript_AllowsReRunningAgainstTheSameRecordedKubernetesHost(t *testing.T) {
	repo := newK3DFakeRepo(t)
	const sameHost = "fake-remote-host-invented-for-this-test.invalid" // matches newK3DFakeRepo's own .env
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host-kubernetes"),
		[]byte(sameHost+"\n"), 0o644); err != nil {
		t.Fatalf("seeding the kubernetes marker: %v", err)
	}
	port := reserveFreeTCPPort(t)
	startListenerAfterForwardRequested(t, repo.logFile+".forward-requested", port)

	output, log, code := repo.run(t, "k3d-up.sh",
		"PORTAINER_E2E_REMOTE=1",
		fmt.Sprintf("STUB_NODEPORT=%d", port),
		"STUB_SERVER_IP=172.30.5.7",
	)
	if code != 0 {
		t.Fatalf("k3d-up.sh (remote, re-run against the same host) exited %d; output:\n%s\nlog:\n%s", code, output, log)
	}
	if dest, ok := repo.marker("kubernetes"); !ok || dest != sameHost {
		t.Errorf("kubernetes marker = (%q, %v), want the same destination still recorded", dest, ok)
	}
}

// TestUnit_K3DUpScript_MissingToolLeavesAnExistingMarkerUntouched is I2's own
// regression test for the reordering fix: the required-tool check
// (`command -v k3d kubectl helm`) now runs BEFORE refuse_docker_host_switch
// and record_docker_host, so a machine missing one of the three fails without
// ever touching this leg's marker. Before this fix, record_docker_host ran
// FIRST: on a machine missing a tool, a plain `make e2e-k8s-up` (empty
// destination) would delete an existing remote marker and then exit 1 having
// done nothing else — the exact "wipes the marker and exits 1 having done
// nothing" defect the review named, distinct from (and reachable even
// without) the "different destination" case the two tests above cover.
func TestUnit_K3DUpScript_MissingToolLeavesAnExistingMarkerUntouched(t *testing.T) {
	repo := newK3DFakeRepo(t)
	const existingHost = "existing-kubernetes-host.invalid"
	if err := os.WriteFile(filepath.Join(repo.e2eDir, ".docker-host-kubernetes"),
		[]byte(existingHost+"\n"), 0o644); err != nil {
		t.Fatalf("seeding the kubernetes marker: %v", err)
	}
	if err := os.Remove(filepath.Join(repo.binDir, "helm")); err != nil {
		t.Fatalf("removing the helm stub to simulate a machine missing it: %v", err)
	}

	// PATH deliberately excludes the real PATH's own /usr/local/bin (or
	// wherever this machine's real k3d/kubectl/helm live): appending the full
	// real PATH after binDir, as run() does, would still find the real helm
	// further along even with its stub removed here. /usr/bin and /bin are
	// where the ordinary utilities the scripts also shell out to (grep, awk,
	// cut, ...) live on this image.
	output, log, code := repo.runWithPath(t, "k3d-up.sh", repo.binDir+":/usr/bin:/bin")
	if code == 0 {
		t.Fatalf("k3d-up.sh succeeded despite a missing tool; output:\n%s\nlog:\n%s", output, log)
	}
	if !strings.Contains(output, "helm is required but not installed") {
		t.Errorf("k3d-up.sh did not report the missing tool; output:\n%s", output)
	}
	if strings.Contains(output, "refusing to continue") {
		t.Errorf("k3d-up.sh reached the host-switch guard before the tool check; output:\n%s", output)
	}
	if dest, ok := repo.marker("kubernetes"); !ok || dest != existingHost {
		t.Errorf("kubernetes marker changed after a missing-tool run: (%q, %v), want it untouched at %q", dest, ok, existingHost)
	}
	if strings.Contains(log, "K3D_CALL:") {
		t.Errorf("k3d-up.sh invoked k3d despite a missing tool; log:\n%s", log)
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
