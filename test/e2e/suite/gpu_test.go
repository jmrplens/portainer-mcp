//go:build e2e

package suite

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// gpuWorkloadImage is pulled inside the estate's dind, not on the Docker host.
// Ubuntu rather than a CUDA image because the driver libraries are supplied by
// the CDI specification, so nothing CUDA-specific has to be downloaded, and
// the image is an order of magnitude smaller.
const gpuWorkloadImage = "ubuntu:24.04"

// gpuWorkloadCommand runs ldconfig before nvidia-smi. That is not incidental:
// the estate's CDI specification has its hooks stripped (see
// test/e2e/scripts/lib.sh's strip_cdi_hooks — they invoke a glibc binary the
// Alpine dind cannot run), and one of those hooks is what normally creates
// the SONAME symlinks. Without ldconfig, nvidia-smi fails with "couldn't find
// libnvidia-ml.so" even though the library is mounted. Measured directly on
// 2026-08-05.
var gpuWorkloadCommand = []string{"sh", "-c", "ldconfig; nvidia-smi --query-gpu=name --format=csv,noheader"}

// dockerProxy issues a request against Portainer's Docker proxy for the named
// environment. The proxy is a passthrough and is not in the vendored
// specification, so it has no generated client method; this is raw HTTP by
// necessity, not by preference.
func dockerProxy(ctx context.Context, t *testing.T, srv harness.Server, envID int, method, path string, body any) *http.Response {
	t.Helper()
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("encoding the %s %s payload: %v", method, path, err)
		}
		payload = bytes.NewReader(encoded)
	}
	url := fmt.Sprintf("%s/api/endpoints/%d/docker%s", strings.TrimRight(srv.BaseURL, "/"), envID, path)
	req, err := http.NewRequestWithContext(ctx, method, url, payload)
	if err != nil {
		t.Fatalf("building the %s %s request: %v", method, url, err)
	}
	req.Header.Set("X-API-Key", srv.Creds.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	return resp
}

// readBody returns the response body and closes it, so callers can assert on
// a failed status without leaking the connection.
func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close() //nolint:errcheck // read-only, and the test fails on the status anyway
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading the response body: %v", err)
	}
	return string(data)
}

// demuxDockerLogs strips the multiplexed stream framing the Docker Engine API
// applies to a non-TTY container's logs (see gpuWorkloadCommand: the GPU
// workload container is created without "Tty": true, so this framing always
// applies to it) and returns the concatenated stdout/stderr payload with no
// header bytes left in it.
//
// Each frame is an 8-byte header — stream type in byte 0, a big-endian
// uint32 payload length in bytes 4-7 — followed by that many payload bytes,
// repeated back to back for as long as the container wrote output. Reading
// the raw response body with a plain strings.Contains, as an earlier draft of
// this test did, happens to work only because the workload's entire one-line
// output is short enough to land inside a single frame; it is not something
// this test can rely on by construction, since the frame boundary is decided
// by how the container's own stdout buffered its writes; not by where the
// expected text happens to sit. See TestDemuxDockerLogs for the case where
// naive Contains would fail and this does not.
func demuxDockerLogs(t *testing.T, raw []byte) string {
	t.Helper()
	var out strings.Builder
	for len(raw) > 0 {
		if len(raw) < 8 {
			t.Fatalf("docker log stream: %d trailing byte(s) shorter than an 8-byte frame header: %q", len(raw), raw)
		}
		size := binary.BigEndian.Uint32(raw[4:8])
		raw = raw[8:]
		if uint64(len(raw)) < uint64(size) {
			t.Fatalf("docker log stream: frame declares a %d-byte payload but only %d byte(s) remain", size, len(raw))
		}
		out.Write(raw[:size]) //nolint:errcheck // strings.Builder.Write never returns a non-nil error
		raw = raw[size:]
	}
	return out.String()
}

// TestDemuxDockerLogs proves demuxDockerLogs actually undoes the Docker
// Engine API's log framing rather than merely stripping bytes that never
// mattered in the one live run this suite has been measured against.
//
// The "split across two frames" case is the one that matters: it builds two
// frames whose payloads are "NVIDIA GeForce RTX " and "4060\n", so the full
// GPU name appears in neither frame's payload alone, only once both are
// concatenated with their headers removed. The trailing check confirms the
// fixture is not accidentally easy — that the raw, still-framed bytes do not
// already contain the reassembled string — because without that check a
// demuxDockerLogs that regressed to a no-op ("return string(raw)") could
// still pass here by accident if the fixture's own header bytes happened not
// to land between the two halves of the name. With the check in place, a
// no-op implementation fails this test: the 8 header bytes between the two
// payloads keep "NVIDIA GeForce RTX " and "4060" apart in the raw bytes, so
// only a genuine demultiplexer reassembles the name.
func TestDemuxDockerLogs(t *testing.T) {
	frame := func(streamType byte, payload string) []byte {
		header := make([]byte, 8)
		header[0] = streamType
		binary.BigEndian.PutUint32(header[4:], uint32(len(payload)))
		return append(header, []byte(payload)...)
	}

	tests := []struct {
		name string
		raw  []byte
		want string
	}{
		{
			name: "single stdout frame returns its payload unchanged",
			raw:  frame(1, "hello\n"),
			want: "hello\n",
		},
		{
			name: "a payload split across two frames reassembles across the header between them",
			raw:  append(frame(1, "NVIDIA GeForce RTX "), frame(1, "4060\n")...),
			want: "NVIDIA GeForce RTX 4060\n",
		},
		{
			name: "stdout and stderr frames both concatenate, in order",
			raw:  append(frame(1, "out\n"), frame(2, "err\n")...),
			want: "out\nerr\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := demuxDockerLogs(t, tt.raw); got != tt.want {
				t.Fatalf("demuxDockerLogs(...) = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("the split-frame fixture does not contain the reassembled text unframed", func(t *testing.T) {
		raw := append(frame(1, "NVIDIA GeForce RTX "), frame(1, "4060\n")...)
		const reassembled = "NVIDIA GeForce RTX 4060"
		if strings.Contains(string(raw), reassembled) {
			t.Fatalf("test fixture is invalid: the raw, still-framed bytes already contain %q without "+
				"demuxing, so this fixture could not have discriminated a real fix from a no-op", reassembled)
		}
	})
}

// TestE2E_GPU_PortainerRunsAContainerOnTheRealCard is the whole point of the
// remote estate: a container created through Portainer, on the estate's own
// Docker daemon, that actually reaches the GPU and can name it.
//
// It asserts the card's name rather than merely "nvidia-smi exited zero",
// because the failure this guards against is a CDI specification that mounts
// device nodes without the libraries — which produces a running container, a
// zero exit from `ldconfig`, and no GPU at all.
func TestE2E_GPU_PortainerRunsAContainerOnTheRealCard(t *testing.T) {
	// This skip is the one condition in this file where "wrong" and
	// "invisible" coincide: an estate that genuinely has a GPU but skips
	// anyway looks, from a CI log, exactly like a healthy pass on a GPU-less
	// machine. estate.HasGPU() requires both GPU.Name and GPU.CDIDevice to be
	// non-empty (harness/estate.go), which is Task 5's own guard against a
	// name-without-a-device-string estate that no container could actually
	// request; nothing about that condition is re-derived here. What this
	// test file is responsible for is not skipping when it should not — see
	// the task report for how the skip path itself was verified, on a real
	// GPU-less estate, to produce SKIP rather than a silent pass or a hang.
	if !estate.HasGPU() {
		t.Skip("no GPU on this estate's docker host: set PORTAINER_E2E_DOCKER_SSH in .env to a host with an NVIDIA card, or run the GPU suites there")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	srv := estate.CE
	envID, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate records a GPU but no %q environment to run it on", harness.EnvironmentDocker)
	}

	// Pull inside the dind first: container create does not pull, and the
	// error it returns for a missing image ("No such image") reads like a
	// configuration fault rather than the ordinary first-run condition it is.
	pull := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/images/create?fromImage="+gpuWorkloadImage, nil)
	if body := readBody(t, pull); pull.StatusCode != http.StatusOK {
		t.Fatalf("pulling %s inside the estate: status %d, body %s", gpuWorkloadImage, pull.StatusCode, body)
	}

	name := uniqueName("gpu")
	create := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/containers/create?name="+name, map[string]any{
		"Image": gpuWorkloadImage,
		"Cmd":   gpuWorkloadCommand,
		"HostConfig": map[string]any{
			"DeviceRequests": []map[string]any{
				{"Driver": "cdi", "DeviceIDs": []string{estate.GPU.CDIDevice}},
			},
		},
	})
	createBody := readBody(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("creating the GPU container: status %d, body %s", create.StatusCode, createBody)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal([]byte(createBody), &created); err != nil {
		t.Fatalf("decoding the create response %q: %v", createBody, err)
	}
	// cleanupOrphans has no Docker-proxy sweeper (it only knows how to sweep
	// tags and registries through Portainer's own API — see fixtures_test.go),
	// so this t.Cleanup is the only thing that removes this container. A run
	// killed mid-test leaves it behind inside the estate's own dind, which
	// `make e2e-down` destroys wholesale; see the task report for why a
	// Docker-proxy sweeper is left as a separate task rather than added here.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cleanupCancel()
		resp := dockerProxy(cleanupCtx, t, srv, envID, http.MethodDelete, "/containers/"+created.ID+"?force=true", nil)
		_ = readBody(t, resp)
	})

	// The device request must survive as Portainer stored it. This is the
	// same field a future docker.container_gpus_inspect action would read, so
	// a regression here — Portainer's proxy silently dropping or rewriting
	// HostConfig.DeviceRequests, or this test sending the wrong CDI device
	// string — is a regression in the action that domain will ship, caught
	// here first.
	inspect := dockerProxy(ctx, t, srv, envID, http.MethodGet, "/containers/"+created.ID+"/json", nil)
	inspectBody := readBody(t, inspect)
	if inspect.StatusCode != http.StatusOK {
		t.Fatalf("inspecting the GPU container: status %d, body %s", inspect.StatusCode, inspectBody)
	}
	var inspected struct {
		HostConfig struct {
			DeviceRequests []struct {
				Driver    string   `json:"Driver"`
				DeviceIDs []string `json:"DeviceIDs"`
			} `json:"DeviceRequests"`
		} `json:"HostConfig"`
	}
	if err := json.Unmarshal([]byte(inspectBody), &inspected); err != nil {
		t.Fatalf("decoding the inspect response: %v", err)
	}
	if len(inspected.HostConfig.DeviceRequests) != 1 {
		t.Fatalf("DeviceRequests = %+v, want exactly one entry", inspected.HostConfig.DeviceRequests)
	}
	if got := inspected.HostConfig.DeviceRequests[0].Driver; got != "cdi" {
		t.Errorf("DeviceRequests[0].Driver = %q, want %q", got, "cdi")
	}
	if got := inspected.HostConfig.DeviceRequests[0].DeviceIDs; len(got) != 1 || got[0] != estate.GPU.CDIDevice {
		t.Errorf("DeviceRequests[0].DeviceIDs = %v, want [%q]", got, estate.GPU.CDIDevice)
	}

	start := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/containers/"+created.ID+"/start", nil)
	if body := readBody(t, start); start.StatusCode != http.StatusNoContent {
		t.Fatalf("starting the GPU container: status %d, body %s", start.StatusCode, body)
	}

	wait := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/containers/"+created.ID+"/wait", nil)
	waitBody := readBody(t, wait)
	if wait.StatusCode != http.StatusOK {
		t.Fatalf("waiting for the GPU container: status %d, body %s", wait.StatusCode, waitBody)
	}
	// A 200 from /wait only means the call itself succeeded; it says nothing
	// about the container's own exit code. Decoding StatusCode and requiring 0
	// is what tells "the shell ran ldconfig and nvidia-smi to completion"
	// apart from "the shell errored out before nvidia-smi ever ran" — the
	// latter is a container that also produces a 200 here, and quite possibly
	// no output at all, which without this check would be indistinguishable
	// from a demuxing bug in the log assertion below rather than a startup
	// failure in the workload itself.
	var waited struct {
		StatusCode int `json:"StatusCode"`
	}
	if err := json.Unmarshal([]byte(waitBody), &waited); err != nil {
		t.Fatalf("decoding the wait response %q: %v", waitBody, err)
	}
	if waited.StatusCode != 0 {
		t.Fatalf("the GPU container exited with status %d, want 0: body %s", waited.StatusCode, waitBody)
	}

	logs := dockerProxy(ctx, t, srv, envID, http.MethodGet, "/containers/"+created.ID+"/logs?stdout=true&stderr=true", nil)
	logBody := readBody(t, logs)
	if logs.StatusCode != http.StatusOK {
		t.Fatalf("reading the GPU container's logs: status %d, body %s", logs.StatusCode, logBody)
	}
	// The container was created without "Tty": true, so this response is the
	// multiplexed stream demuxDockerLogs exists to undo (see its own doc and
	// TestDemuxDockerLogs) — asserting against the raw bytes here would be
	// correct only by luck, not by construction.
	demuxed := demuxDockerLogs(t, []byte(logBody))
	if !strings.Contains(demuxed, estate.GPU.Name) {
		t.Errorf("the container did not report the estate's GPU %q; its demuxed output was:\n%s", estate.GPU.Name, demuxed)
	}
}
