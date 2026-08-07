package harness

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// daemonSetManifest is the small slice of the DaemonSet shape this file
// actually reads out of test/e2e/k8s/nvidia-device-plugin.yaml — its own
// identity (kind, name, namespace), tolerations, and its single container's
// image, privilege, env vars and volume mounts — not a general-purpose
// Kubernetes type.
//
// Kind/Metadata/Tolerations/container Name/Image/SecurityContext were added
// for I7: k3d-up.sh's own rollout-status call
// (`kubectl -n kube-system rollout status daemonset/nvidia-device-plugin`)
// hard-codes the namespace and name this manifest is supposed to carry, and
// nothing on either side pinned that the two actually agree. A drift on
// either side makes that call hang the full 180s and fail, on exactly the
// GPU hosts this branch exists for.
type daemonSetManifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name      string `yaml:"name"`
		Namespace string `yaml:"namespace"`
	} `yaml:"metadata"`
	Spec struct {
		Template struct {
			Spec struct {
				Tolerations []struct {
					Operator string `yaml:"operator"`
				} `yaml:"tolerations"`
				Containers []struct {
					Name            string `yaml:"name"`
					Image           string `yaml:"image"`
					SecurityContext struct {
						Privileged bool `yaml:"privileged"`
					} `yaml:"securityContext"`
					Env []struct {
						Name  string `yaml:"name"`
						Value string `yaml:"value"`
					} `yaml:"env"`
					VolumeMounts []struct {
						Name      string `yaml:"name"`
						MountPath string `yaml:"mountPath"`
						ReadOnly  bool   `yaml:"readOnly"`
					} `yaml:"volumeMounts"`
				} `yaml:"containers"`
				Volumes []struct {
					Name     string `yaml:"name"`
					HostPath *struct {
						Path string `yaml:"path"`
					} `yaml:"hostPath"`
				} `yaml:"volumes"`
			} `yaml:"spec"`
		} `yaml:"template"`
	} `yaml:"spec"`
}

// loadNvidiaDevicePluginManifest reads and parses
// test/e2e/k8s/nvidia-device-plugin.yaml, resolved relative to this
// package's own directory (../k8s/...) rather than the process's working
// directory, so this test finds the file the same way whether `go test` is
// invoked from the repository root or from inside this package.
func loadNvidiaDevicePluginManifest(t *testing.T) daemonSetManifest {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "k8s", "nvidia-device-plugin.yaml"))
	if err != nil {
		t.Fatalf("resolving nvidia-device-plugin.yaml path: %v", err)
	}
	data, err := os.ReadFile(path) //nolint:gosec // fixed, repo-relative path; not user input
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	var manifest daemonSetManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	if len(manifest.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("%s: %d containers, want exactly 1", path, len(manifest.Spec.Template.Spec.Containers))
	}
	return manifest
}

// TestUnit_NvidiaDevicePluginManifest_CarriesEveryHardwareMeasuredValue guards the
// checked-in DaemonSet's fragile values — the ones no static check (bash -n,
// shellcheck, go vet, a YAML/schema validator) can tell apart from an
// equally well-formed WRONG value, because every one of them was only ever
// established by running against real GPU hardware and watching it fail
// without it.
//
// What this test does NOT claim: it would not have caught task 8's actual
// defect. LD_LIBRARY_PATH and the host-libs volume/mount were entirely
// absent from both the manifest and this test at the time the brief this
// task first transcribed was written — the brief itself omitted them, so
// the "expected" values baked into a test written against that brief would
// have been wrong in exactly the same way. A test can only pin a value
// someone has already discovered is required; it cannot discover a missing
// one by itself.
//
// What it DOES prevent, demonstrated directly: a live review reverted
// NVIDIA_DRIVER_ROOT from "/" back to "/driver-root" — a plausible-looking
// "simplification" to match CONTAINER_DRIVER_ROOT — and the rest of the
// suite (bash -n, shellcheck, go vet, every other test) stayed green with
// that mutation in place. Only a test that actually reads this value back
// out of the manifest catches a silent revert of something a hardware run
// took real time to establish.
func TestUnit_NvidiaDevicePluginManifest_CarriesEveryHardwareMeasuredValue(t *testing.T) {
	manifest := loadNvidiaDevicePluginManifest(t)
	container := manifest.Spec.Template.Spec.Containers[0]

	env := make(map[string]string, len(container.Env))
	for _, e := range container.Env {
		env[e.Name] = e.Value
	}
	// Each of these four was arrived at by watching the previous value fail
	// against real hardware — see the manifest's own inline comments for the
	// specific failure each one closes.
	wantEnv := map[string]string{
		"DEVICE_LIST_STRATEGY":  "cdi-cri",
		"NVIDIA_DRIVER_ROOT":    "/",
		"CONTAINER_DRIVER_ROOT": "/driver-root",
		"LD_LIBRARY_PATH":       "/host-libs",
	}
	for name, want := range wantEnv {
		got, ok := env[name]
		if !ok {
			t.Errorf("env %s is absent, want %q", name, want)
			continue
		}
		if got != want {
			t.Errorf("env %s = %q, want %q", name, got, want)
		}
	}

	volumeHostPaths := make(map[string]string, len(manifest.Spec.Template.Spec.Volumes))
	for _, v := range manifest.Spec.Template.Spec.Volumes {
		if v.HostPath != nil {
			volumeHostPaths[v.Name] = v.HostPath.Path
		}
	}
	if got := volumeHostPaths["driver-root"]; got != "/" {
		t.Errorf(`volume "driver-root" hostPath = %q, want "/" -- this is what makes CONTAINER_DRIVER_ROOT mean anything`, got)
	}
	if got, ok := volumeHostPaths["host-libs"]; !ok || got != "/usr/lib/x86_64-linux-gnu" {
		t.Errorf(`volume "host-libs" hostPath = (%q, present=%v), want "/usr/lib/x86_64-linux-gnu" -- without it `+
			`the plugin cannot load NVML and fails "unable to validate flags: CDI --device-list-strategy options `+
			`are only supported on NVML-based systems"`, got, ok)
	}

	mountByName := make(map[string]struct {
		path     string
		readOnly bool
	}, len(container.VolumeMounts))
	for _, m := range container.VolumeMounts {
		mountByName[m.Name] = struct {
			path     string
			readOnly bool
		}{m.MountPath, m.ReadOnly}
	}
	if m, ok := mountByName["host-libs"]; !ok || m.path != "/host-libs" || !m.readOnly {
		t.Errorf(`volumeMount "host-libs" = (%+v, present=%v), want {path: /host-libs, readOnly: true}`, m, ok)
	}
}

// TestUnit_NvidiaDevicePluginManifest_PinsIdentityAndOtherLoadBearingFields is
// I7's own regression test. Neither this file nor k3d_scripts_test.go read
// metadata or kind before this test existed, so the DaemonSet's name and
// namespace were unasserted on both sides of a pairing that has to agree:
// k3d-up.sh hard-codes
// `kubectl -n kube-system rollout status daemonset/nvidia-device-plugin`,
// and drift on either side makes that call hang the full 180s and fail —
// on exactly the GPU hosts this branch exists for.
//
// The image tag, `privileged: true`, the tolerations block, the
// driver-root mountPath, NVIDIA_VISIBLE_DEVICES and NVIDIA_DRIVER_CAPABILITIES
// are pinned alongside identity for the same reason
// TestUnit_NvidiaDevicePluginManifest_CarriesEveryHardwareMeasuredValue pins
// the four env vars above: every one of them is equally load-bearing and
// equally invisible to bash -n, shellcheck, go vet or a YAML/schema
// validator, which can all tell a well-formed manifest from a broken one but
// none can tell a well-formed WRONG value from the right one. The
// driver-root mountPath specifically has a partner that IS already pinned
// (CONTAINER_DRIVER_ROOT, asserted above) — leaving the mountPath itself
// unpinned means the two could silently disagree, with nothing here to
// notice.
func TestUnit_NvidiaDevicePluginManifest_PinsIdentityAndOtherLoadBearingFields(t *testing.T) {
	manifest := loadNvidiaDevicePluginManifest(t)
	container := manifest.Spec.Template.Spec.Containers[0]

	if manifest.Kind != "DaemonSet" {
		t.Errorf("kind = %q, want %q", manifest.Kind, "DaemonSet")
	}
	if manifest.Metadata.Name != "nvidia-device-plugin" {
		t.Errorf("metadata.name = %q, want %q -- k3d-up.sh's rollout-status call hard-codes this name", manifest.Metadata.Name, "nvidia-device-plugin")
	}
	if manifest.Metadata.Namespace != "kube-system" {
		t.Errorf("metadata.namespace = %q, want %q -- k3d-up.sh's rollout-status call hard-codes this namespace", manifest.Metadata.Namespace, "kube-system")
	}

	if container.Image != "nvcr.io/nvidia/k8s-device-plugin:v0.17.1" {
		t.Errorf("container image = %q, want %q", container.Image, "nvcr.io/nvidia/k8s-device-plugin:v0.17.1")
	}
	if !container.SecurityContext.Privileged {
		t.Error("container securityContext.privileged = false, want true -- the plugin cannot access the host's GPU devices without it")
	}

	foundExists := false
	for _, tol := range manifest.Spec.Template.Spec.Tolerations {
		if tol.Operator == "Exists" {
			foundExists = true
		}
	}
	if !foundExists {
		t.Error(`tolerations does not contain {operator: Exists} -- without it the DaemonSet is not scheduled onto every node ` +
			`(specifically the tainted ones), the exact gap I6's "installed on every node" fix depends on the scheduler actually doing`)
	}

	env := make(map[string]string, len(container.Env))
	for _, e := range container.Env {
		env[e.Name] = e.Value
	}
	for name, want := range map[string]string{
		"NVIDIA_VISIBLE_DEVICES":     "all",
		"NVIDIA_DRIVER_CAPABILITIES": "all",
	} {
		if got, ok := env[name]; !ok || got != want {
			t.Errorf("env %s = (%q, present=%v), want %q", name, got, ok, want)
		}
	}

	foundDriverRootMount := false
	for _, m := range container.VolumeMounts {
		if m.Name != "driver-root" {
			continue
		}
		foundDriverRootMount = true
		if m.MountPath != "/driver-root" {
			t.Errorf(`volumeMount "driver-root".mountPath = %q, want "/driver-root" -- CONTAINER_DRIVER_ROOT (pinned above) names this same path, and the two can silently disagree`, m.MountPath)
		}
		if !m.ReadOnly {
			t.Error(`volumeMount "driver-root".readOnly = false, want true`)
		}
	}
	if !foundDriverRootMount {
		t.Error(`no volumeMount named "driver-root" found`)
	}
}
