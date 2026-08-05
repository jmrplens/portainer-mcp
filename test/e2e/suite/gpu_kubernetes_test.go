//go:build e2e

package suite

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// gpuAdvertisementProblem is TestE2E_GPU_KubernetesNodeAdvertisesTheCard's
// own assertion against a node's capacity map, factored out as a pure
// function (no *testing.T, no live cluster) so both of its branches can be
// pinned by TestUnit_GPUAdvertisementProblem_DistinguishesMissingCapacityFromZero
// without a running estate.
//
// The two branches are deliberately not the same severity: a capacity map
// that carries no "nvidia.com/gpu" key at all means the device plugin never
// registered anything — fatal is true, matching the caller's original
// t.Fatalf. A capacity map that DOES carry the key but reports "0" means the
// plugin registered and found no card — a real but softer problem, matching
// the caller's original t.Errorf (fatal is false). A live review found the
// second branch exercised by nothing: every hardware run to date happened to
// have a card once the plugin was healthy, so "the key exists but says
// zero" had no test driving it, hardware or otherwise.
func gpuAdvertisementProblem(capacity map[string]string) (msg string, fatal bool) {
	gpus, ok := capacity["nvidia.com/gpu"]
	if !ok {
		return fmt.Sprintf("the node advertises no nvidia.com/gpu; its capacity was %v", capacity), true
	}
	if gpus == "0" {
		return fmt.Sprintf("nvidia.com/gpu = %q, want at least one: the device plugin registered but found no card", gpus), false
	}
	return "", false
}

// TestUnit_GPUAdvertisementProblem_DistinguishesMissingCapacityFromZero pins both branches of gpuAdvertisementProblem
// without a cluster: the missing-key case (fatal) and the present-but-zero
// case (not fatal) — the latter is the one the live review found nothing
// exercised, since a real GPU host with a healthy plugin never produces it.
func TestUnit_GPUAdvertisementProblem_DistinguishesMissingCapacityFromZero(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		capacity  map[string]string
		wantMsg   bool
		wantFatal bool
	}{
		{
			name:     "gpu present and non-zero: no problem",
			capacity: map[string]string{"nvidia.com/gpu": "1", "cpu": "12"},
		},
		{
			name:      "key absent entirely: fatal, the device plugin never registered anything",
			capacity:  map[string]string{"cpu": "12"},
			wantMsg:   true,
			wantFatal: true,
		},
		{
			name:     "key present but zero: not fatal, but still a real problem -- the branch the review found untested",
			capacity: map[string]string{"nvidia.com/gpu": "0"},
			wantMsg:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg, fatal := gpuAdvertisementProblem(tt.capacity)
			if (msg != "") != tt.wantMsg {
				t.Errorf("gpuAdvertisementProblem(%v) message = %q, want present=%v", tt.capacity, msg, tt.wantMsg)
			}
			if fatal != tt.wantFatal {
				t.Errorf("gpuAdvertisementProblem(%v) fatal = %v, want %v", tt.capacity, fatal, tt.wantFatal)
			}
		})
	}
}

// TestE2E_GPU_KubernetesNodeAdvertisesTheCard asserts the thing Portainer's
// GetKubernetesGPUInfo actually reads: a node whose capacity carries
// nvidia.com/gpu. Without the device plugin the resource simply does not
// exist, and that operation returns an empty summary that looks identical to
// a healthy cluster with no GPUs.
//
// It reads the node through Kubernetes rather than through Portainer because
// the kubernetes domain is not in the catalog yet; when it lands, the
// assertion moves to kubernetes.gpu_info and this test becomes its fixture.
//
// Gated on estate.HasKubernetesGPU(), NOT estate.HasGPU(): HasGPU records the
// COMPOSE leg's Docker host, which k3d-up.sh never touches (see
// harness.Estate.GPU's own doc), while k3d-up.sh records this leg's own
// capability separately once its device plugin DaemonSet rollout succeeds.
// The two can legitimately disagree — README.md calls a different Docker
// host per leg a supported combination — and gating on the wrong one either
// skips a run that could have passed (compose host has no card, Kubernetes
// leg's host does) or fails a legitimately GPU-less Kubernetes leg (compose
// host has a card, Kubernetes leg's host does not).
func TestE2E_GPU_KubernetesNodeAdvertisesTheCard(t *testing.T) {
	if !estate.HasKubernetes() {
		t.Skip("no Kubernetes leg provisioned in this estate: run `make e2e-k8s-up` first")
	}
	if !estate.HasKubernetesGPU() {
		t.Skip("no GPU on this estate's kubernetes node: bring the kubernetes leg up with `make e2e-k8s-up-remote` against a host with an NVIDIA card")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	capacity := kubernetesNodeCapacity(ctx, t)
	if msg, fatal := gpuAdvertisementProblem(capacity); msg != "" {
		if fatal {
			t.Fatal(msg)
		}
		t.Error(msg)
	}
}
