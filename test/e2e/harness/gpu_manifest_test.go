package harness

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// daemonSetManifest is the small slice of the DaemonSet shape this file
// actually reads out of test/e2e/k8s/nvidia-device-plugin.yaml — env vars,
// volume mounts and hostPath volumes on the single container this manifest
// carries — not a general-purpose Kubernetes type.
type daemonSetManifest struct {
	Spec struct {
		Template struct {
			Spec struct {
				Containers []struct {
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

// TestNvidiaDevicePluginManifest_CarriesEveryHardwareMeasuredValue guards the
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
func TestNvidiaDevicePluginManifest_CarriesEveryHardwareMeasuredValue(t *testing.T) {
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
