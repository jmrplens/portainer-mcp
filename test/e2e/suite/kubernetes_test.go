//go:build e2e

package suite

import (
	"log/slog"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
)

// wantKubernetesEdition and wantServerVersion are what k3d-up.sh's own
// override deploys: `--set enterpriseEdition.image.tag=2.44.0` against a
// chart whose own app version is five minor versions behind
// (ee-2.39.5). Asserting both, not just "a Version came back non-empty", is
// what makes that override's own regression visible: a chart upgrade or a
// dropped --set flag would silently redeploy the chart's stale default, and a
// test that only checked "Version is non-empty" would still pass against it.
//
// wantServerVersion is shared with system_test.go, which asserts the same
// Portainer version against the compose legs: it is a package-level
// constant, not a local one, exactly so a bump of the estate image needs one
// edit rather than two that can drift apart.
const (
	wantKubernetesEdition = "EE"
	wantServerVersion     = "2.44.0"
)

// TestKubernetesLeg_ReachableThroughEveryMCPSurface is the one test this
// file carries. P2 shipped no Kubernetes domain, so there is nothing
// Kubernetes-specific yet to exercise; this proves the leg itself — a real
// Portainer server deployed into k3d by test/e2e/scripts/k3d-up.sh — is
// reachable end to end through every MCP surface, which is the seam P3's
// Kubernetes wave plugs its own domain suite into.
//
// It builds its own sessions rather than using the package's shared
// Sessions: that type only ever carries Community and Business Edition
// legs (see newSessions in sessions_test.go), since the Kubernetes leg is
// provisioned by a separate script invocation the estate may or may not
// carry. skipTLSVerify is true here and nowhere else in this package: the
// Kubernetes leg's Helm chart deploys a self-signed certificate, and unlike
// the provisioner (harness.ClientWithCA), the estate file this test process
// reads back carries no pinned CA to verify it against — only a base URL and
// credentials.
//
// It calls system.version, not system.status, and checks both ServerEdition
// and ServerVersion against fixed expectations, not merely "some Version
// string came back non-empty". A whole-branch review measured that the
// looser check passes identically whether this session is actually talking
// to the Kubernetes leg or was, by some wiring mistake, pointed at
// estate.CE or estate.EE instead — any of the three answers a non-empty
// Version, so the test proved nothing about which server it was actually
// reaching. It also could not have caught k3d-up.sh's own
// `--set enterpriseEdition.image.tag=2.44.0` silently regressing to the
// chart's stale default (`ee-2.39.5`): asserting the specific version this
// estate is supposed to run is what makes that regression visible. Measured
// against a live in-cluster server: system.version returns exactly
// {"ServerVersion":"2.44.0","ServerEdition":"EE","DatabaseVersion":"2.44.0"}.
func TestKubernetesLeg_ReachableThroughEveryMCPSurface(t *testing.T) {
	// A skip, not a failure, and now the ordinary outcome rather than the
	// unusual one: CI's compose job (see .github/workflows/e2e.yml) never
	// brings this leg up, because the two legs share one single-instance
	// Business Edition licence and take it in turns. The message names both
	// what is absent and what provisions it, and TestMain's own summary line
	// names this leg among the ABSENT ones, so a green compose run cannot be
	// mistaken for one that measured the Kubernetes leg too.
	if !estate.HasKubernetes() {
		t.Skip("no Kubernetes leg provisioned in this estate: `make e2e-k8s-up` provisions it, " +
			"and CI's own Kubernetes job runs this suite against it separately")
	}

	logger := logging.New(slog.LevelWarn)
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session, err := buildSession(estate.Kubernetes, config.ToolSurface(surface), false, false, true, logger)
			if err != nil {
				t.Fatalf("build session against the Kubernetes leg: %v", err)
			}
			defer func() { _ = session.Close() }()

			out := callAction[map[string]any](t, session, surface, "system.version", nil)
			serverEdition, _ := out["ServerEdition"].(string)
			if serverEdition != wantKubernetesEdition {
				t.Errorf("system.version through the %s surface against the Kubernetes leg: ServerEdition = %q, want %q",
					surface, serverEdition, wantKubernetesEdition)
			}
			serverVersion, _ := out["ServerVersion"].(string)
			if serverVersion != wantServerVersion {
				t.Errorf("system.version through the %s surface against the Kubernetes leg: ServerVersion = %q, want %q",
					surface, serverVersion, wantServerVersion)
			}
		})
	}
}
