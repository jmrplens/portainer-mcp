//go:build e2e

package suite

import (
	"context"
	"testing"
	"time"
)

// TestE2E_GPU_KubernetesNodeAdvertisesTheCard asserts the thing Portainer's
// GetKubernetesGPUInfo actually reads: a node whose capacity carries
// nvidia.com/gpu. Without the device plugin the resource simply does not
// exist, and that operation returns an empty summary that looks identical to
// a healthy cluster with no GPUs.
//
// It reads the node through Kubernetes rather than through Portainer because
// the kubernetes domain is not in the catalog yet; when it lands, the
// assertion moves to kubernetes.gpu_info and this test becomes its fixture.
func TestE2E_GPU_KubernetesNodeAdvertisesTheCard(t *testing.T) {
	if !estate.HasKubernetes() {
		t.Skip("no Kubernetes leg provisioned in this estate: run `make e2e-k8s-up` first")
	}
	if !estate.HasGPU() {
		t.Skip("no GPU on this estate's docker host: bring the estate up with `make e2e-up-remote` against a host with an NVIDIA card")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	capacity := kubernetesNodeCapacity(ctx, t)
	gpus, ok := capacity["nvidia.com/gpu"]
	if !ok {
		t.Fatalf("the node advertises no nvidia.com/gpu; its capacity was %v", capacity)
	}
	if gpus == "0" {
		t.Errorf("nvidia.com/gpu = %q, want at least one: the device plugin registered but found no card", gpus)
	}
}
