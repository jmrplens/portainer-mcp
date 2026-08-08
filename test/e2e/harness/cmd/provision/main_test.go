package main

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// fakeBusinessEditionServer stands in for a real Business Edition Portainer
// server, answering exactly what provisionServer, harness.ApplyLicence, and
// harness.LicenceNodes each need to reach the licence-attached state. Every
// handler is unconditional: the test that fails a specific call does so by
// overriding just that one path after building the mux, not by adding
// branching logic here.
func fakeBusinessEditionServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"Id": 1})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestProvisionBusinessEdition_LicenceNodesFailure_ReturnsServerAlongsideError
// is the mutation-tested proof for the critical fix: harness.ApplyLicence
// above has already succeeded by the time GET /api/licenses (LicenceNodes)
// fails, so this server carries a real activation. Discarding it here — the
// defect this test guards against — would mean provisionBusinessEdition's
// caller (run, in main.go) has nothing to persist, and the estate file would
// never name the server or its API key that -release-licence needs to find
// and release it.
//
// Reverting the fix (restoring `return harness.Server{}, fmt.Errorf(...)` in
// provisionBusinessEdition's LicenceNodes branch) makes this test fail on
// both assertions below: BaseURL and Creds.APIKey come back empty. Verified
// by hand while writing this test, not merely asserted.
func TestProvisionBusinessEdition_LicenceNodesFailure_ReturnsServerAlongsideError(t *testing.T) {
	t.Parallel()
	server := fakeBusinessEditionServer(t)
	mux := server.Config.Handler.(*http.ServeMux)
	// The one failure this test is about: ApplyLicence above has already
	// succeeded, and now reading the licence back fails outright, exactly
	// like a timeout, a 500, or a transient network blip against a real
	// server would.
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})

	client := &http.Client{Timeout: kubernetesClientTimeout}
	got, err := provisionBusinessEdition(context.Background(), client, server.URL, "fake-licence")
	if err == nil {
		t.Fatal("provisionBusinessEdition() error = nil, want an error from the failing LicenceNodes call")
	}
	if got.BaseURL == "" {
		t.Error("provisionBusinessEdition() returned a Server with an empty BaseURL alongside the error: " +
			"the caller has nothing to persist, and the licence this call just attached is now unreachable " +
			"from the estate file")
	}
	if got.Creds.APIKey == "" {
		t.Error("provisionBusinessEdition() returned a Server with an empty API key alongside the error: " +
			"a later -release-licence call has no key to authenticate a release with")
	}
}

// TestProvisionBusinessEdition_Success_ReturnsTheProvisionedServer is the
// success-path sibling: with every call answering cleanly, the function
// returns no error and a fully populated Server, so the failure-path test
// above is verified against a handler difference of exactly one endpoint,
// not a mux that never worked to begin with.
func TestProvisionBusinessEdition_Success_ReturnsTheProvisionedServer(t *testing.T) {
	t.Parallel()
	server := fakeBusinessEditionServer(t)
	mux := server.Config.Handler.(*http.ServeMux)
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"nodes": 5}})
	})

	client := &http.Client{Timeout: kubernetesClientTimeout}
	got, err := provisionBusinessEdition(context.Background(), client, server.URL, "fake-licence")
	if err != nil {
		t.Fatalf("provisionBusinessEdition() error = %v, want nil", err)
	}
	if got.BaseURL != server.URL {
		t.Errorf("BaseURL = %q, want %q", got.BaseURL, server.URL)
	}
	if got.Creds.APIKey != "the-api-key" {
		t.Errorf("Creds.APIKey = %q, want %q", got.Creds.APIKey, "the-api-key")
	}
}

// TestRecoverStrandedLicence_LicenceNodesTransportFailure_IsReportedAsAFailure
// is the mutation-tested proof for the MAJOR fix at this file's own
// recoverStrandedLicence: the verification after ReleaseLicence must accept
// only a confirmed empty licence list as proof of release, never any error
// at all. Here GET /licenses fails outright (a 500) after a real,
// successful release call — recoverStrandedLicence must report that the
// release could not be confirmed, not print its "safe to reuse" success
// message. Reverting recoverStrandedLicence's switch to the old
// `if _, err := harness.LicenceNodes(...); err == nil { return
// fmt.Errorf(...) }` shape makes this test fail: that check treats this
// exact 500 as confirmation and returns nil. Verified by hand while writing
// this fix.
func TestRecoverStrandedLicence_LicenceNodesTransportFailure_IsReportedAsAFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	mux.HandleFunc("/api/licenses/remove", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	// The one failure this test is about: the release call above succeeded,
	// but confirming it (GET /licenses) fails outright.
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	t.Setenv(licenceEnv, "fake-licence")
	t.Setenv(recoverURLEnv, server.URL)

	err := recoverStrandedLicence()
	if err == nil {
		t.Fatal("recoverStrandedLicence() error = nil, want a failure: the release was never confirmed")
	}
	const wantSubstring = "could not confirm the licence was released"
	if !strings.Contains(err.Error(), wantSubstring) {
		t.Errorf("recoverStrandedLicence() error = %q, want it to contain %q", err.Error(), wantSubstring)
	}
}

// TestRecoverStrandedLicence_Success_ConfirmsReleaseAndReturnsNil is the
// clean-path sibling: with every call, including the post-release GET
// /licenses, answering as a real server would after a genuine release
// (an empty list), recoverStrandedLicence returns nil.
func TestRecoverStrandedLicence_Success_ConfirmsReleaseAndReturnsNil(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/licenses/add", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"conflictingKeys": nil})
	})
	mux.HandleFunc("/api/licenses/remove", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/api/licenses", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	t.Setenv(licenceEnv, "fake-licence")
	t.Setenv(recoverURLEnv, server.URL)

	if err := recoverStrandedLicence(); err != nil {
		t.Fatalf("recoverStrandedLicence() error = %v, want nil", err)
	}
}

// fakeKubernetesServer stands in for the Helm-deployed Kubernetes leg
// runKubernetes talks to: TLS, since kubernetesClient verifies a real
// certificate rather than skipping verification, with handlers for every
// call runKubernetes makes when no licence is supplied (WaitReady, Provision,
// CreateEndpoint). Its own certificate is written to a temporary CA file in
// the shape k3d-up.sh leaves one in, since that is what kubernetesClient
// reads.
func fakeKubernetesServer(t *testing.T) (server *httptest.Server, caFile string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0", "InstanceID": "k8s-instance"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Id":1,"Username":"admin"}`))
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"Id": 42})
	})
	tlsServer := httptest.NewTLSServer(mux)
	t.Cleanup(tlsServer.Close)

	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsServer.Certificate().Raw})
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, caPEM, 0o600); err != nil {
		t.Fatalf("write CA file: %v", err)
	}
	return tlsServer, path
}

// TestUnit_RunKubernetes_PreservesAlreadyRecordedComposeGPU is the regression
// proof for the deliberate omission next to runKubernetes' LoadEstate call:
// this invocation's own environment never carries gpuNameEnv or
// gpuCDIDeviceEnv (k3d-up.sh does not export them — only up.sh does, for the
// compose leg), so a version of runKubernetes that read them here and
// assigned estate.GPU unconditionally, mirroring what run() does, would
// silently overwrite an already-recorded GPU with a zero one on every
// Kubernetes-leg run. The estate seeded here already carries a GPU, as it
// would after a real compose leg ran first; the assertion is that it comes
// back unchanged after runKubernetes has loaded, merged and saved twice.
//
// Not folded into TestUnit_RunKubernetes_RecordsItsOwnGPUFromEnvironment
// below despite the overlapping scenario (that table's own "compose leg has
// a gpu, kubernetes leg does not" case seeds the same shape of estate): the
// two tests guard different mechanisms. That table proves
// HasKubernetesGPU() is derived from k8sGPUEnv independently of the compose
// leg's own field, checking only booleans; this test proves runKubernetes
// never reads gpuNameEnv/gpuCDIDeviceEnv (a compose-leg-only pair of
// variables the table never touches) and reassigns estate.GPU with them,
// which needs the stronger exact-value equality below plus the
// HasKubernetes() sanity check — neither of which the table's per-case
// shape carries. Folding would either weaken this test's own assertions
// down to the table's booleans or force an unrelated field onto every other
// case just to satisfy one of them; a single t.Run keeps the convention
// without either.
func TestUnit_RunKubernetes_PreservesAlreadyRecordedComposeGPU(t *testing.T) {
	t.Run("gpu already recorded by the compose leg survives a kubernetes-leg run", func(t *testing.T) {
		server, caFile := fakeKubernetesServer(t)

		estatePath := filepath.Join(t.TempDir(), "estate.json")
		wantGPU := harness.GPU{Name: "NVIDIA GeForce RTX 4060", CDIDevice: "nvidia.com/gpu=all"}
		seed := harness.Estate{
			CE:  harness.Server{Edition: "CE", BaseURL: "http://ce.example"},
			GPU: wantGPU,
		}
		if err := seed.SaveTo(estatePath); err != nil {
			t.Fatalf("seed estate: SaveTo() error = %v", err)
		}

		t.Setenv(k8sBaseURLEnv, server.URL)
		t.Setenv(envK8sSetup, "the-setup-token")
		t.Setenv(k8sCAFileEnv, caFile)
		t.Setenv(licenceEnv, "")

		if err := runKubernetes(estatePath); err != nil {
			t.Fatalf("runKubernetes() error = %v, want nil", err)
		}

		got, err := harness.LoadEstate(estatePath)
		if err != nil {
			t.Fatalf("LoadEstate() error = %v", err)
		}
		if got.GPU != wantGPU {
			t.Errorf("GPU after runKubernetes = %+v, want %+v unchanged", got.GPU, wantGPU)
		}
		if !got.HasGPU() {
			t.Error("HasGPU() = false after runKubernetes ran against an estate that already recorded one")
		}
		// Sanity: the Kubernetes leg itself must have actually been provisioned,
		// not merely have returned early without doing anything.
		if !got.HasKubernetes() {
			t.Error("HasKubernetes() = false: runKubernetes did not actually provision the leg this test exercises")
		}
	})
}

// TestUnit_RunKubernetes_RecordsItsOwnGPUFromEnvironment is I5's own
// regression test at the provisioner level: k8sGPUEnv (PORTAINER_E2E_K8S_GPU,
// set by k3d-up.sh once its device plugin DaemonSet rollout succeeds) must
// land on estate.KubernetesGPU, independently of whatever the compose leg's
// GPU field already carries — the split-host combination README.md calls
// legitimate means the two can disagree.
func TestUnit_RunKubernetes_RecordsItsOwnGPUFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name          string
		k8sGPUEnvVal  string
		wantK8sGPU    bool
		composeGPU    harness.GPU
		wantComposeOK bool
	}{
		{
			name:          "kubernetes leg has a gpu, compose leg does not",
			k8sGPUEnvVal:  "1",
			wantK8sGPU:    true,
			composeGPU:    harness.GPU{},
			wantComposeOK: false,
		},
		{
			name:          "neither leg has a gpu",
			k8sGPUEnvVal:  "",
			wantK8sGPU:    false,
			composeGPU:    harness.GPU{},
			wantComposeOK: false,
		},
		{
			name:          "compose leg has a gpu, kubernetes leg does not (split host)",
			k8sGPUEnvVal:  "",
			wantK8sGPU:    false,
			composeGPU:    harness.GPU{Name: "NVIDIA GeForce RTX 4060", CDIDevice: "nvidia.com/gpu=all"},
			wantComposeOK: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server, caFile := fakeKubernetesServer(t)
			estatePath := filepath.Join(t.TempDir(), "estate.json")
			seed := harness.Estate{
				CE:  harness.Server{Edition: "CE", BaseURL: "http://ce.example"},
				GPU: tc.composeGPU,
			}
			if err := seed.SaveTo(estatePath); err != nil {
				t.Fatalf("seed estate: SaveTo() error = %v", err)
			}

			t.Setenv(k8sBaseURLEnv, server.URL)
			t.Setenv(envK8sSetup, "the-setup-token")
			t.Setenv(k8sCAFileEnv, caFile)
			t.Setenv(licenceEnv, "")
			t.Setenv(k8sGPUEnv, tc.k8sGPUEnvVal)

			if err := runKubernetes(estatePath); err != nil {
				t.Fatalf("runKubernetes() error = %v, want nil", err)
			}

			got, err := harness.LoadEstate(estatePath)
			if err != nil {
				t.Fatalf("LoadEstate() error = %v", err)
			}
			if got.HasKubernetesGPU() != tc.wantK8sGPU {
				t.Errorf("HasKubernetesGPU() = %v, want %v", got.HasKubernetesGPU(), tc.wantK8sGPU)
			}
			if got.HasGPU() != tc.wantComposeOK {
				t.Errorf("HasGPU() (compose leg) = %v, want %v -- runKubernetes must never derive one leg's GPU from the other's", got.HasGPU(), tc.wantComposeOK)
			}
		})
	}
}

// fakeComposeServer stands in for the Community Edition server run()
// provisions: plain HTTP (the compose legs are reached over
// http://localhost, unlike the Kubernetes leg), answering WaitReady,
// Provision and every CreateEndpoint call (docker, agent) generically.
func fakeComposeServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/system/status", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"Version": "2.44.0", "InstanceID": "ce-instance"})
	})
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"Id":1,"Username":"admin"}`))
	})
	mux.HandleFunc("/api/auth", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"jwt": "the-jwt"})
	})
	mux.HandleFunc("/api/users/1/tokens", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"rawAPIKey": "the-api-key"})
	})
	mux.HandleFunc("/api/endpoints", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]int{"Id": 1})
	})
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server
}

// TestUnit_Run_RecordsGPUFromEnvironment is the coverage Step 4 of the brief
// itself leaves out: the brief's own test plan (estate_test.go's two new
// tests) proves HasGPU and the JSON round trip, but neither one exercises
// run() actually reading gpuNameEnv/gpuCDIDeviceEnv and attaching the result
// to the estate before the final save — the one line this task adds to
// main.go's compose path. Without this, a future edit that deleted that
// assignment (or moved it above where estate is declared, or typo'd the env
// var name) would pass every other test in this plan and still ship a
// compose leg that never records a GPU.
func TestUnit_Run_RecordsGPUFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name         string
		gpuName      string
		gpuCDIDevice string
		wantHasGPU   bool
		wantGPU      harness.GPU
	}{
		{
			name: "gpu present", gpuName: "NVIDIA GeForce RTX 4060", gpuCDIDevice: "nvidia.com/gpu=all",
			wantHasGPU: true, wantGPU: harness.GPU{Name: "NVIDIA GeForce RTX 4060", CDIDevice: "nvidia.com/gpu=all"},
		},
		{
			name: "no gpu on the docker host", gpuName: "", gpuCDIDevice: "",
			wantHasGPU: false, wantGPU: harness.GPU{},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeComposeServer(t)

			estatePath := filepath.Join(t.TempDir(), "estate.json")
			edgeEnvPath := filepath.Join(t.TempDir(), ".edge.env")

			t.Setenv(harness.EstateFileEnv, estatePath)
			t.Setenv(harness.EdgeEnvFileEnv, edgeEnvPath)
			t.Setenv(ceBaseURLEnv, server.URL)
			t.Setenv(licenceEnv, "")
			t.Setenv(gpuNameEnv, tc.gpuName)
			t.Setenv(gpuCDIDeviceEnv, tc.gpuCDIDevice)

			if err := run(false, false, false); err != nil {
				t.Fatalf("run() error = %v, want nil", err)
			}

			got, err := harness.LoadEstate(estatePath)
			if err != nil {
				t.Fatalf("LoadEstate() error = %v", err)
			}
			if got.GPU != tc.wantGPU {
				t.Errorf("GPU = %+v, want %+v", got.GPU, tc.wantGPU)
			}
			if got.HasGPU() != tc.wantHasGPU {
				t.Errorf("HasGPU() = %v, want %v", got.HasGPU(), tc.wantHasGPU)
			}
		})
	}
}

// TestUnit_Run_RecordsSwarmServiceIDFromEnvironment is
// TestUnit_Run_RecordsGPUFromEnvironment's sibling for the Swarm fixture:
// estate_test.go's own new tests prove HasSwarm and the JSON round trip in
// isolation, but neither exercises run() actually reading
// swarmServiceIDEnv and attaching it to the estate before the final save.
// Without this, a future edit that deleted that assignment, moved it above
// where estate is declared, or typo'd the env var name would pass every
// other test in this package and still ship a compose leg that never
// records a Swarm fixture, silently turning every future
// docker.service_image_status e2e test into a permanent skip.
func TestUnit_Run_RecordsSwarmServiceIDFromEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name           string
		swarmServiceID string
		wantHasSwarm   bool
	}{
		{name: "swarm fixture present", swarmServiceID: "wxyhlanc3nqz", wantHasSwarm: true},
		{name: "no swarm on the docker host", swarmServiceID: "", wantHasSwarm: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := fakeComposeServer(t)

			estatePath := filepath.Join(t.TempDir(), "estate.json")
			edgeEnvPath := filepath.Join(t.TempDir(), ".edge.env")

			t.Setenv(harness.EstateFileEnv, estatePath)
			t.Setenv(harness.EdgeEnvFileEnv, edgeEnvPath)
			t.Setenv(ceBaseURLEnv, server.URL)
			t.Setenv(licenceEnv, "")
			t.Setenv(swarmServiceIDEnv, tc.swarmServiceID)

			if err := run(false, false, false); err != nil {
				t.Fatalf("run() error = %v, want nil", err)
			}

			got, err := harness.LoadEstate(estatePath)
			if err != nil {
				t.Fatalf("LoadEstate() error = %v", err)
			}
			if got.SwarmServiceID != tc.swarmServiceID {
				t.Errorf("SwarmServiceID = %q, want %q", got.SwarmServiceID, tc.swarmServiceID)
			}
			if got.HasSwarm() != tc.wantHasSwarm {
				t.Errorf("HasSwarm() = %v, want %v", got.HasSwarm(), tc.wantHasSwarm)
			}
		})
	}
}
