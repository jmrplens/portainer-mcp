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
// system.update (Business Edition's counterpart) has its own equivalent
// test below, TestSystemUpdate_SafeModePreview_DoesNotActuallyRestart,
// through a Business Edition safe-mode session (Sessions.SafeModeEE).
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

// TestSystemUpdate_SafeModePreview_DoesNotActuallyRestart is system.update's
// equivalent of the test above, through a Business Edition safe-mode
// session (Sessions.SafeModeEE) rather than the Community Edition one:
// system.update is Business-Edition-only, so a Community Edition session,
// safe mode or not, never has it in its catalog to call.
//
// Only two things are asserted here: the call returns a preview, not an
// error, and the preview's own text identifies it as a safe-mode
// interception. Both are satisfied identically by a correct interception and
// by a handler that silently no-ops — a whole-branch review measured this
// directly: InstanceID (Portainer's own per-instance identity) survives both
// a `docker restart` of the container and an image replacement, so
// comparing it before and after system.update, which this test used to do,
// never actually detects a restart at all and proves nothing beyond what
// these two assertions already do. It has been removed rather than left
// alongside a comment claiming it discriminates the two, which it does not.
//
// The property "safe mode prevents the handler from ever running, not just
// the response from looking like a preview" is what actually needs an
// observable side effect to prove, and system.update — Portainer's own
// restart — is exactly the wrong action to prove it with: a real side effect
// here would take down every other test running against this Business
// Edition server. TestSafeMode_TagsCreate_DoesNotActuallyCreateATag in
// sessions_test.go proves that property instead, against tags.create, whose
// side effect (a new tag on the server) is both observable through a raw API
// read and safe to actually let happen if safe mode ever regressed.
func TestSystemUpdate_SafeModePreview_DoesNotActuallyRestart(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.SafeModeEE(t, surface)

			out := callAction[map[string]any](t, session, surface, "system.update", nil)
			if safeMode, _ := out["safe_mode"].(bool); !safeMode {
				t.Fatalf("system.update under safe mode = %v, want a safe_mode preview, not a real call", out)
			}
			if out["action"] != "system.update" {
				t.Errorf("safe-mode preview action = %v, want %q", out["action"], "system.update")
			}
			note, _ := out["note"].(string)
			if !strings.Contains(note, "Safe mode is enabled") {
				t.Errorf("safe-mode preview note = %q, want it to identify a safe-mode interception", note)
			}
		})
	}
}
