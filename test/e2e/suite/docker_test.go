//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// TestDocker_ServiceImageStatus_AgainstARealSwarmService is Wave 1 Stage A's
// closing proof: docker.service_image_status (internal/tools/docker/
// handlers.go) is the catalog's first action that only means anything
// against a real Docker Swarm service. Before test/e2e/scripts/up.sh grew
// its own swarm_init/swarm_fixture_service_id steps, the estate's dind was
// never in Swarm mode, so this action always answered 500 — invisibly,
// because nothing ever called it. See harness.Estate.HasSwarm and its own
// doc for what "confirmed" means here: both Swarm mode AND the fixture
// service must have been set up successfully.
//
// It reproduces the exact sequence measured by hand against a live 2.44.0
// server (see the plan's own progress ledger, "SWARM GAP"): first
// docker.service_image_status_clear as the real precondition the manual
// probe used (POST .../services/image_status/clear, which answers 500
// without Swarm and 204 with it), then docker.service_image_status itself
// against the fixture's REAL, Swarm-assigned alphanumeric service id —
// never a hand-labelled integer, which docs/api-divergences.md's "cheat
// this is written down to forbid" warns against by name: a service id this
// test invented itself would make the string-typed path parameter look
// correct without proving anything about a real Swarm-assigned one.
//
// refresh:true is not optional here — it is what makes this call actually
// discriminate. Measured directly against a live estate: without it,
// Portainer answers a stale, CACHED {"Status":"updated"} for a service id
// it successfully resolved on some earlier call, even after that service was
// deleted and even after Swarm mode itself was left entirely — a version of
// this test that omitted refresh would keep passing after either of those
// regressions and prove nothing. With refresh:true the handler performs a
// live `docker service inspect` on every call: it answers 500 when Swarm is
// not active ("This node is not a swarm manager"), 500 when Swarm is active
// but this exact service is gone ("service ... not found"), and 200 only
// when both are true — exactly the three states this test needs to tell
// apart.
//
// docker.service_image_status and docker.service_image_status_clear are
// both Business-Edition-only (edition.EE — see handWrittenSpecs and
// generatedSpecs in internal/tools/docker), so this only ever runs against
// the EE leg, across every surface.
func TestDocker_ServiceImageStatus_AgainstARealSwarmService(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("no confirmed swarm leg on this estate's docker daemon: run `make e2e-up` on a host where `docker swarm init` succeeds")
	}
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: docker.service_image_status is EE-only")
	}

	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate records a swarm fixture but the EE server has no %q environment", harness.EnvironmentDocker)
	}

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")

			// The real precondition the manual probe used: clearing the
			// cached status is the same POST that answered 500 before this
			// estate had a Swarm leg and 204 once it did. Called for real
			// here (not through the safe-mode session), exactly like the
			// hand probe.
			callAction[map[string]any](t, session, surface, "docker.service_image_status_clear", map[string]any{
				"environmentId": envID,
			})

			out := callAction[map[string]any](t, session, surface, "docker.service_image_status", map[string]any{
				"environmentId": envID,
				"serviceId":     estate.SwarmServiceID,
				"refresh":       true,
			})

			// The real, discriminating assertion. A regression that
			// reintroduced Swarm's absence (fixture torn down, swarm left)
			// makes the underlying GET answer 500 again, which callAction
			// itself turns into a t.Fatalf before this line is ever
			// reached. What is checked here is the shape of a genuine
			// success: images.StatusResponse always carries one of a fixed
			// six-value enum (docs: processing/outdated/updated/skipped/
			// preparing/error) for Status, and "error" specifically means
			// Portainer itself could not determine the image's status
			// against a real, existing service — not a value this test
			// should treat as an acceptable pass.
			status, ok := out["Status"].(string)
			if !ok || status == "" {
				t.Fatalf("docker.service_image_status returned no usable Status: %v", out)
			}
			if status == "error" {
				t.Fatalf("docker.service_image_status reported Status=%q (Message=%v) against a real fixture service; want a real status, not an error",
					status, out["Message"])
			}
		})
	}
}

// dockerFixtureImage is pulled inside the estate's dind for every test below
// that needs a real container or a real image on record. busybox:1.36 rather
// than a bare "busybox" (which the Swarm fixture service already pulls, see
// swarm_fixture_service_id in test/e2e/scripts/lib.sh): a pinned tag makes
// TestDocker_ImagesList_ReturnsRealImages's assertion against a literal tag
// string exact, and keeps this file's own container fixtures independent of
// whether HasSwarm() happens to be true on a given host.
const dockerFixtureImage = "busybox:1.36"

// createDockerContainer creates and starts a container named uniqueName("docker")
// from dockerFixtureImage, inside srv's own dind, through Portainer's Docker
// proxy (dockerProxy, gpu_test.go) -- never through docker.dashboard/images_list
// or any other action this domain declares, since none of them can create
// anything. It registers a t.Cleanup that force-removes the container.
//
// This exists specifically for docker.container_gpus_inspect and
// docker.container_image_status: both take a real Docker container id, and
// docs/api-divergences.md's section 6.3 "cheat this is written down to
// forbid" warns against inventing one -- a hand-labelled id can look correct
// without being correct. Creating one for real, through the estate's own
// dind, is what that section asks for instead.
//
// The container is started, not merely created: docker.service_image_status
// (docs/api-divergences.md section 2.4) was measured caching a stale answer
// for a service with no running task, and nothing in this file's own
// probing ruled out the same being true of a created-but-never-started
// container, so this keeps the fixture in the state most likely to get a
// genuine answer rather than an edge case nothing here has verified.
func createDockerContainer(ctx context.Context, t *testing.T, srv harness.Server, envID int) string {
	t.Helper()
	pull := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/images/create?fromImage="+dockerFixtureImage, nil)
	if body := readBody(t, pull); pull.StatusCode != http.StatusOK {
		t.Fatalf("pulling %s inside the estate: status %d, body %s", dockerFixtureImage, pull.StatusCode, body)
	}

	name := uniqueName("docker")
	create := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/containers/create?name="+name, map[string]any{
		"Image": dockerFixtureImage,
		"Cmd":   []string{"sleep", "3600"},
	})
	createBody := readBody(t, create)
	if create.StatusCode != http.StatusCreated {
		t.Fatalf("creating container %s: status %d, body %s", name, create.StatusCode, createBody)
	}
	var created struct {
		ID string `json:"Id"`
	}
	if err := json.Unmarshal([]byte(createBody), &created); err != nil {
		t.Fatalf("decoding container create response %q: %v", createBody, err)
	}
	// cleanupOrphans has no Docker-proxy sweeper (see gpu_test.go's identical
	// t.Cleanup and its own comment on why), so this is the only thing that
	// removes this container; `make e2e-down` destroys the dind wholesale as
	// the net for a run killed mid-test.
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		resp := dockerProxy(cleanupCtx, t, srv, envID, http.MethodDelete, "/containers/"+created.ID+"?force=true", nil) //nolint:bodyclose // readBody closes it; bodyclose cannot follow the value out of this t.Cleanup closure
		_ = readBody(t, resp)
	})

	start := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/containers/"+created.ID+"/start", nil)
	if body := readBody(t, start); start.StatusCode != http.StatusNoContent {
		t.Fatalf("starting container %s: status %d, body %s", name, start.StatusCode, body)
	}
	return created.ID
}

// fabricatedContainerID is a well-formed (64 lowercase hex characters, the
// shape docker.container_gpus_inspect and docker.container_image_status both
// require) container id that Docker never assigned. It exists to prove the
// live server actually looked the identifier up rather than accepting any
// string that merely passes schema validation -- see docs/api-divergences.md
// section 6.3's "cheat this is written down to forbid": a test that only
// ever calls with a real id cannot tell "the server checked and found it"
// apart from "the server never checks at all".
const fabricatedContainerID = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

// assertActionFails calls action through surface and fails the test unless
// the result comes back as a tool error -- the same shape
// TestRegistries_Ping_AgainstUnreachableURL and
// TestRegistries_BusinessEditionOnlyActions_FailInformativelyAgainstAFakeBackend
// (registries_test.go) already use for "this call must fail against a real
// server" assertions, via actionCallParams and a direct CallTool (callTool
// itself cannot be reused here: it fails the test on any IsError result,
// which is exactly the outcome this assertion wants to see).
func assertActionFails(t *testing.T, s *mcp.ClientSession, surface, action string, in map[string]any) {
	t.Helper()
	toolName, args := actionCallParams(t, surface, action, in)
	res, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", toolName, err)
	}
	if !res.IsError {
		t.Errorf("%s against a fabricated identifier succeeded, want a failure: %s", action, toolResultText(res))
	}
}

// TestDocker_Dashboard_ReturnsRealCounters closes one of the six e2e gaps
// `make audit-e2e-gaps` named for this domain before this test existed
// (docs/domain-wave-checklist.md's Step 5: "a name it reports as
// unreferenced is a real gap in this step, not a tool defect").
//
// It loops over composeLegs(estate) rather than a hardcoded edition string,
// per that same step's own instruction not to hardcode a
// []string{"CE","EE"} literal: docker.dashboard is declared edition.CE
// (available on both editions -- edition.Includes), and both the Community
// and, when licensed, Business Edition legs proxy the SAME underlying dind
// daemon through their own "docker" environment, so both must answer it.
//
// The four counter fields are asserted present by key, not merely "the call
// did not error": probed live against this estate (2026-08-08), Portainer's
// own dashboard handler emits stacks/services/networks/volumes even when
// their value is zero (`{"...,"volumes":0,"networks":5,"stacks":0}`) rather
// than omitting a zero field, so their absence is a genuine, discriminating
// signal that the response shape regressed -- not an artefact of an empty
// environment this assertion could mistake for a bug.
func TestDocker_Dashboard_ReturnsRealCounters(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID, ok := leg.Server.Environment(harness.EnvironmentDocker)
		if !ok {
			t.Fatalf("%s: estate has no %q environment", leg.Name, harness.EnvironmentDocker)
		}
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)
				out := callAction[map[string]any](t, session, surface, "docker.dashboard", map[string]any{
					"environmentId": envID,
				})
				for _, field := range []string{"stacks", "services", "networks", "volumes"} {
					if _, ok := out[field]; !ok {
						t.Errorf("docker.dashboard response carries no %q field: %v", field, out)
					}
				}
			})
		}
	}
}

// TestDocker_ImagesList_ReturnsRealImages is the images_list half of the
// same gap. It pulls dockerFixtureImage into the estate's dind itself,
// through the same Docker proxy createDockerContainer uses, rather than
// relying on an image some other test or the Swarm fixture happened to
// leave behind: the assertion below needs to know, for certain, that at
// least one real image with a known tag exists on this environment, on
// every host this runs on, whether or not HasSwarm() is true here.
func TestDocker_ImagesList_ReturnsRealImages(t *testing.T) {
	for _, leg := range composeLegs(estate) {
		envID, ok := leg.Server.Environment(harness.EnvironmentDocker)
		if !ok {
			t.Fatalf("%s: estate has no %q environment", leg.Name, harness.EnvironmentDocker)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		pull := dockerProxy(ctx, t, leg.Server, envID, http.MethodPost, "/images/create?fromImage="+dockerFixtureImage, nil)
		if body := readBody(t, pull); pull.StatusCode != http.StatusOK {
			cancel()
			t.Fatalf("%s: pulling %s inside the estate: status %d, body %s", leg.Name, dockerFixtureImage, pull.StatusCode, body)
		}
		cancel()

		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)
				out := callAction[[]map[string]any](t, session, surface, "docker.images_list", map[string]any{
					"environmentId": envID,
					"withUsage":     true,
				})
				if len(out) == 0 {
					t.Fatalf("docker.images_list returned no images despite %s having just been pulled", dockerFixtureImage)
				}
				found := false
				for _, image := range out {
					tags, _ := image["tags"].([]any)
					for _, tag := range tags {
						if tag == dockerFixtureImage {
							found = true
						}
					}
					if _, ok := image["id"]; !ok {
						t.Errorf("docker.images_list entry carries no %q field: %v", "id", image)
					}
				}
				if !found {
					t.Errorf("docker.images_list did not list the just-pulled %s among its tags: %v", dockerFixtureImage, out)
				}
			})
		}
	}
}

// TestDocker_ContainerGpusInspect_AgainstARealContainer closes the third of
// the six gaps, and is one of the two the review named as needing a real
// container rather than a transcription of an existing test's shape (the
// other is TestDocker_ContainerImageStatus_AgainstARealContainer below).
//
// One container is created once, on the Community Edition leg's own "docker"
// environment, and reused across every (leg, surface) pair below: CE and EE
// each register their own environment against the SAME underlying dind
// daemon (see harness.Estate.SwarmServiceID's own doc for the identical
// fact about the Swarm fixture), so a container created through either
// proxy is reachable through the other's environment id too. Creating one
// container rather than one per leg keeps this test's footprint on the
// dind to exactly what it needs.
//
// Probed live against this estate (2026-08-08): a real container with no
// GPU device request answers `{"gpus":"none"}`, which is what the positive
// assertion below checks for, literally -- not merely "the call did not
// error", which a handler that silently swallowed a downstream failure
// could also satisfy.
func TestDocker_ContainerGpusInspect_AgainstARealContainer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	srv := estate.CE
	envID, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("estate has no %q environment", harness.EnvironmentDocker)
	}
	containerID := createDockerContainer(ctx, t, srv, envID)

	for _, leg := range composeLegs(estate) {
		legEnvID, ok := leg.Server.Environment(harness.EnvironmentDocker)
		if !ok {
			t.Fatalf("%s: estate has no %q environment", leg.Name, harness.EnvironmentDocker)
		}
		for _, surface := range surfaceNames {
			t.Run(leg.Name+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, leg.Name)
				out := callAction[map[string]any](t, session, surface, "docker.container_gpus_inspect", map[string]any{
					"environmentId": legEnvID,
					"containerId":   containerID,
				})
				if got, _ := out["gpus"].(string); got != "none" {
					t.Errorf(`docker.container_gpus_inspect = %v, want {"gpus":"none"} for a container with no device request`, out)
				}
			})
		}
	}

	// The discriminating half: a well-formed but fabricated id must fail
	// against the real server, not resolve to a false success -- see
	// fabricatedContainerID's own doc for why this is checked at all.
	t.Run("fabricated container id fails", func(t *testing.T) {
		assertActionFails(t, sessions.For(t, "dynamic", "CE"), "dynamic", "docker.container_gpus_inspect", map[string]any{
			"environmentId": envID,
			"containerId":   fabricatedContainerID,
		})
	})
}

// TestDocker_ContainerImageStatus_AgainstARealContainer is the second of the
// two gaps needing a real container: docker.container_image_status is
// Business-Edition-only (edition.EE -- handWrittenSpecs in
// internal/tools/docker/docker.go), so unlike container_gpus_inspect this
// only ever runs against the EE leg.
//
// refresh:true is passed for the same reason
// TestDocker_ServiceImageStatus_AgainstARealSwarmService passes it: the
// sibling operation was measured caching a stale answer
// (docs/api-divergences.md section 2.4), and although ContainerImageStatus's
// own caching was not itself measured, its narrative
// (internal/tools/docker/docker.go's narrative func) says explicitly that it
// "may cache the same way" -- refresh:true is what keeps this assertion
// honest either way rather than resting on an unverified assumption.
func TestDocker_ContainerImageStatus_AgainstARealContainer(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: docker.container_image_status is EE-only")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	srv := estate.EE
	envID, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}
	containerID := createDockerContainer(ctx, t, srv, envID)

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")
			out := callAction[map[string]any](t, session, surface, "docker.container_image_status", map[string]any{
				"environmentId": envID,
				"containerId":   containerID,
				"refresh":       true,
			})
			status, ok := out["Status"].(string)
			if !ok || status == "" {
				t.Fatalf("docker.container_image_status returned no usable Status: %v", out)
			}
			if status == "error" {
				t.Fatalf("docker.container_image_status reported Status=%q (Message=%v) against a real container; want a real status, not an error",
					status, out["Message"])
			}
		})
	}

	// Unlike docker.container_gpus_inspect (which 404s on the identical
	// shape of unknown id, see TestDocker_ContainerGpusInspect_
	// AgainstARealContainer above), a fabricated id here does NOT fail.
	// Measured live against this estate (2026-08-08): GET .../containers/
	// <fabricated-64-hex>/image_status?refresh=true answers 200
	// {"Status":"skipped","Message":""} -- an earlier version of this test
	// assumed a fabricated id would fail the same way
	// DockerContainerGpusInspect's does, and running it here is what caught
	// that assumption wrong. Recorded as a positive assertion on the real,
	// measured value rather than dropped silently: "skipped" is therefore
	// not proof a container's image check was genuinely skipped for policy
	// reasons -- it is also what a nonexistent container looks like -- and a
	// future change that starts erroring instead is worth noticing. See
	// docs/api-divergences.md section 2.4 and this operation's own narrative
	// (internal/tools/docker/docker.go).
	t.Run("fabricated container id answers skipped, not an error", func(t *testing.T) {
		session := sessions.For(t, "dynamic", "EE")
		out := callAction[map[string]any](t, session, "dynamic", "docker.container_image_status", map[string]any{
			"environmentId": envID,
			"containerId":   fabricatedContainerID,
			"refresh":       true,
		})
		if status, _ := out["Status"].(string); status != "skipped" {
			t.Errorf("docker.container_image_status against a fabricated container id = %v, want Status=%q (measured live behaviour)", out, "skipped")
		}
	})
}

// assertClears204 issues method against path directly on srv's own API --
// bypassing the MCP tool layer entirely, through a raw portainer.Client the
// same way fixtures_test.go builds one -- and fails unless the response is
// EXACTLY 204. This is deliberately narrower than "2xx": probed live
// (2026-08-08) against this estate's Business Edition leg, both
// docker.containers_image_status_clear and docker.stacks_image_status_clear
// answer 204 with no body, and a test that accepted any 2xx would keep
// passing if the endpoint started answering 202 and doing nothing. Confirmed
// directly against a stand-in server answering 202: a `resp.StatusCode/100
// != 2` version of this check let it through silently, while the exact
// `!= http.StatusNoContent` check below correctly flagged it.
func assertClears204(ctx context.Context, t *testing.T, srv harness.Server, method, path string) {
	t.Helper()
	c, err := portainer.New(&config.Config{URL: srv.BaseURL, Token: srv.Creds.APIKey})
	if err != nil {
		t.Fatalf("building a raw client for %s: %v", srv.BaseURL, err)
	}
	resp, err := c.Do(ctx, method, path, nil)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("%s %s = %d, want exactly %d (No Content)", method, path, resp.StatusCode, http.StatusNoContent)
	}
}

// TestDocker_ContainersImageStatusClear_Answers204 closes the fifth gap.
// docker.containers_image_status_clear is a POST with no resource of its
// own to read back, so the discriminating assertion is the literal status
// code (assertClears204), not a body; the loop over surfaceNames afterward
// exercises the action itself through every surface, both so
// `make audit-e2e-gaps` sees it referenced and so a genuine schema/handler
// regression on any one surface still fails this test, not just the raw
// probe.
func TestDocker_ContainersImageStatusClear_Answers204(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: docker.containers_image_status_clear is EE-only")
	}
	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertClears204(ctx, t, estate.EE, http.MethodPost, fmt.Sprintf("/docker/%d/containers/image_status/clear", envID))

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")
			callAction[map[string]any](t, session, surface, "docker.containers_image_status_clear", map[string]any{
				"environmentId": envID,
			})
		})
	}
}

// TestDocker_StacksImageStatusClear_Answers204 is the sixth and last gap.
// stacksImageStatusClear's own path carries no environmentId (see
// internal/tools/docker/actions.go: it is a query-filter parameter, not a
// path segment -- StacksImageStatusClear's route is /stacks/image_status/clear,
// not /docker/{environmentId}/...), so the raw probe below takes no
// environment id either; environmentId is still passed to the action call
// afterward, matching what the existing docker.stacks_image_status_clear
// unit test (internal/tools/docker/docker_test.go) already exercises as a
// valid, forwarded filter.
func TestDocker_StacksImageStatusClear_Answers204(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate: docker.stacks_image_status_clear is EE-only")
	}
	envID, ok := estate.EE.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the estate's Business Edition server has no %q environment", harness.EnvironmentDocker)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	assertClears204(ctx, t, estate.EE, http.MethodPost, "/stacks/image_status/clear")

	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")
			callAction[map[string]any](t, session, surface, "docker.stacks_image_status_clear", map[string]any{
				"environmentId": envID,
			})
		})
	}
}
