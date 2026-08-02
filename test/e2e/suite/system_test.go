//go:build e2e

package suite

import (
	"strings"
	"testing"
)

// TestSystemInfo_AcrossSurfacesAndEditions is the template every P3 domain
// suite copies: one action, run across every surface and every edition this
// estate provisions.
//
// The task brief's own illustrative version of this test checks
// out["ServerVersion"] == "2.44.0" against system.info's result. That field
// does not exist on system.info's response: GET /system/info
// (GithubComPortainerPortainerEeApiHttpHandlerSystemSystemInfoResponse in
// internal/portainer/gen/types.gen.go) reports only addonsAvailable, agents,
// edgeAgents, edgeDevices and platform — ServerVersion belongs to
// system.version and system.status instead (see
// TestSystemStatusVersionNodes_AcrossSurfacesAndEditions below, which does
// check it against those). Checked directly against the generated type
// before writing this, rather than assumed from the brief, precisely because
// the standing instruction for this task is that an assertion matching
// something other than what it claims to test is the recurring failure mode.
// A literal port of the brief's version would have compiled, and every run
// would have failed with "ServerVersion = <nil>, want 2.44.0" — a real
// finding, not the read-back bite the task was probing for.
//
// agents is the fact asserted instead: task 6 registers a Docker environment
// and a Portainer agent as a second environment against both CE and EE (see
// up.sh's "registered docker endpoint 1" / "registered agent endpoint 2"),
// so the true count is never zero on either edition.
func TestSystemInfo_AcrossSurfacesAndEditions(t *testing.T) {
	for _, edition := range []string{"CE", "EE"} {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)
				out := callAction[map[string]any](t, session, surface, "system.info", nil)
				agents, ok := out["agents"].(float64)
				if !ok || agents < 1 {
					t.Errorf("system.info agents = %v, want at least 1 (the agent endpoint task 6 registers)", out["agents"])
				}
			})
		}
	}
}

// TestSystemStatusVersionNodes_AcrossSurfacesAndEditions covers the rest of
// system's read-only actions with the same shape as TestSystemInfo above,
// asserting a fact specific enough that a handler returning an empty or
// wrong-shaped body would be caught rather than merely "no error".
func TestSystemStatusVersionNodes_AcrossSurfacesAndEditions(t *testing.T) {
	for _, edition := range []string{"CE", "EE"} {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				status := callAction[map[string]any](t, session, surface, "system.status", nil)
				if status["Version"] != "2.44.0" {
					t.Errorf("system.status Version = %v, want 2.44.0", status["Version"])
				}

				version := callAction[map[string]any](t, session, surface, "system.version", nil)
				serverEdition, _ := version["ServerEdition"].(string)
				if !strings.EqualFold(serverEdition, edition) {
					t.Errorf("system.version ServerEdition = %v, want %s", version["ServerEdition"], edition)
				}

				// system.nodes counts nodes across every environment this
				// server manages. The estate registers at least the local
				// Docker environment Portainer creates at startup plus the
				// agent environment task 6 registers, so the true count is
				// never zero — asserting that is stronger than "returned
				// something" without hardcoding the exact figure, which
				// would need updating every time the estate's shape changes.
				nodes := callAction[map[string]any](t, session, surface, "system.nodes", nil)
				count, ok := nodes["nodes"].(float64)
				if !ok || count <= 0 {
					t.Errorf("system.nodes nodes = %v, want a positive count", nodes["nodes"])
				}
			})
		}
	}
}

// TestSystemUpgrade_SafeModePreview_AcrossSurfaces exercises system.upgrade
// (Community Edition's mutating, destructive fifth system action) through
// every surface without ever letting it run for real: its handler restarts
// the target Portainer server, and this estate's Community Edition server
// backs every other CE session in the matrix, so a real call here would take
// down every other test running against it. Safe mode intercepts a mutating
// action before its handler ever runs (see tools.Execute), so this proves
// the catalog membership, routing and input handling for real, on a session
// deliberately configured not to reach the handler.
//
// system.update (Business Edition's counterpart) has no equivalent test: it
// is Business-Edition-only, and this harness's Sessions only builds a
// safe-mode session against the Community Edition leg (see newSessions in
// sessions_test.go) — there is no safe-mode Business Edition session to
// intercept it. Calling it for real would restart the Business Edition
// server for the rest of this run. It is left unexercised; see the task
// report.
func TestSystemUpgrade_SafeModePreview_AcrossSurfaces(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.SafeMode(t, surface)
			out := callAction[map[string]any](t, session, surface, "system.upgrade", nil)
			if safeMode, _ := out["safe_mode"].(bool); !safeMode {
				t.Fatalf("system.upgrade under safe mode = %v, want a safe_mode preview, not a real call", out)
			}
			if out["action"] != "system.upgrade" {
				t.Errorf("safe-mode preview action = %v, want %q", out["action"], "system.upgrade")
			}
		})
	}
}
