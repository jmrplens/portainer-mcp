//go:build e2e

package suite

import (
	"testing"

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
