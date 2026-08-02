//go:build e2e

package suite

import (
	"log/slog"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
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
func TestKubernetesLeg_ReachableThroughEveryMCPSurface(t *testing.T) {
	if !estate.HasKubernetes() {
		t.Skip("no Kubernetes leg provisioned in this estate: run `make e2e-k8s-up` first")
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

			// system.status, not system.info: system.info's real response
			// (GithubComPortainerPortainerEeApiHttpHandlerSystemSystemInfoResponse
			// in internal/portainer/gen/types.gen.go) carries no version field
			// at all — see the comment on TestSystemInfo_AcrossSurfacesAndEditions
			// in system_test.go for how that was found. system.status's
			// response does carry one ("Version").
			out := callAction[map[string]any](t, session, surface, "system.status", nil)
			version, _ := out["Version"].(string)
			if version == "" {
				t.Errorf("system.status through the %s surface against the Kubernetes leg returned no Version", surface)
			}
		})
	}
}
