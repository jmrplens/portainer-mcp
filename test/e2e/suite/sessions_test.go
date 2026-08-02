//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// surfaceNames lists every tool surface the matrix exercises. It is the
// single place that changes if a fourth surface is ever added.
var surfaceNames = []string{"individual", "meta", "dynamic"}

// Sessions holds one in-memory client/server pair per surface x edition,
// plus one read-only and one safe-mode pair per surface.
//
// Every server here is built through wiring.BuildCatalog and
// wiring.NewServer, the internal/wiring package cmd/portainer-mcp's own
// run() calls to build the binary a user actually runs (Go refuses to
// import a package main from elsewhere, which is why that construction
// path lives in its own package rather than in cmd/portainer-mcp itself). A
// harness that constructed servers a different way could hide a defect that
// only the real wiring has.
type Sessions struct {
	byKey    map[string]*mcp.ClientSession
	readOnly map[string]*mcp.ClientSession
	safeMode map[string]*mcp.ClientSession
}

// newSessions builds every session the matrix needs from a loaded estate.
//
// It builds a normal session per surface for the Community Edition leg
// (always present) and, when the estate carries one, the Business Edition
// leg. Read-only and safe-mode sessions are built once per surface against
// the Community Edition leg only: For's edition axis exists to let a
// licence-less contributor skip Business Edition suites, but the mode a
// session runs in does not depend on which edition backs it, and building
// against CE keeps that guaranteed present.
func newSessions(e harness.Estate) (*Sessions, error) {
	s := &Sessions{
		byKey:    make(map[string]*mcp.ClientSession),
		readOnly: make(map[string]*mcp.ClientSession),
		safeMode: make(map[string]*mcp.ClientSession),
	}

	logger := logging.New(slog.LevelWarn)

	legs := map[string]harness.Server{"CE": e.CE}
	if e.HasBusinessEdition() {
		legs["EE"] = e.EE
	}

	for ed, srv := range legs {
		for _, surface := range surfaceNames {
			session, err := buildSession(srv, config.ToolSurface(surface), false, false, logger)
			if err != nil {
				return nil, fmt.Errorf("build %s/%s session: %w", surface, ed, err)
			}
			s.byKey[surface+"/"+ed] = session
		}
	}

	for _, surface := range surfaceNames {
		ro, err := buildSession(e.CE, config.ToolSurface(surface), true, false, logger)
		if err != nil {
			return nil, fmt.Errorf("build %s read-only session: %w", surface, err)
		}
		s.readOnly[surface] = ro

		sm, err := buildSession(e.CE, config.ToolSurface(surface), false, true, logger)
		if err != nil {
			return nil, fmt.Errorf("build %s safe-mode session: %w", surface, err)
		}
		s.safeMode[surface] = sm
	}

	return s, nil
}

// buildSession constructs one Portainer client, catalog and MCP server the
// same way cmd/portainer-mcp's run() does, then connects a client session to
// it over mcp.NewInMemoryTransports(): no sockets, no subprocess, no port
// allocation, and a panic in a handler surfaces as a test failure rather
// than a hung read. The Portainer client underneath is real and talks to the
// estate over HTTP; only the MCP transport is in-memory.
func buildSession(srv harness.Server, surface config.ToolSurface, readOnly, safeMode bool, logger *slog.Logger) (*mcp.ClientSession, error) {
	cfg := &config.Config{
		URL:         srv.BaseURL,
		Token:       srv.Creds.APIKey,
		ToolSurface: surface,
		ReadOnly:    readOnly,
		SafeMode:    safeMode,
	}

	client, err := portainer.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("build portainer client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	catalog, resolvedEdition, serverVersion, err := wiring.BuildCatalog(ctx, cfg, client, logger)
	if err != nil {
		return nil, fmt.Errorf("build action catalog: %w", err)
	}

	deps := tools.Deps{Client: client, Logger: logger, SafeMode: safeMode}

	server, err := wiring.NewServer(cfg, catalog, deps, resolvedEdition, serverVersion)
	if err != nil {
		return nil, fmt.Errorf("build server: %w", err)
	}

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), serverTransport, nil); err != nil {
		return nil, fmt.Errorf("connect server: %w", err)
	}
	mcpClient := mcp.NewClient(&mcp.Implementation{Name: "portainer-mcp-e2e", Version: "test"}, nil)
	session, err := mcpClient.Connect(context.Background(), clientTransport, nil)
	if err != nil {
		return nil, fmt.Errorf("connect client: %w", err)
	}
	return session, nil
}

// Close shuts down every client session this Sessions built. TestMain calls
// it once, after m.Run returns.
func (s *Sessions) Close() {
	for _, m := range []map[string]*mcp.ClientSession{s.byKey, s.readOnly, s.safeMode} {
		for _, session := range m {
			_ = session.Close()
		}
	}
}

// For returns the session for one surface and edition, skipping when that
// edition is not part of the estate. A contributor without the Business
// Edition licence must still be able to run the Community Edition suites.
func (s *Sessions) For(t *testing.T, surface, edition string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.byKey[surface+"/"+edition]
	if !ok {
		t.Skipf("no %s session for %s: that edition is not provisioned in this estate", surface, edition)
	}
	return session
}

// ReadOnly returns the read-only-mode session for surface. Unlike For, this
// never skips: every estate carries a Community Edition leg, and that is
// what a read-only session is built against, so its absence is a harness
// defect rather than an unprovisioned edition.
func (s *Sessions) ReadOnly(t *testing.T, surface string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.readOnly[surface]
	if !ok {
		t.Fatalf("no read-only session was built for surface %q", surface)
	}
	return session
}

// SafeMode returns the safe-mode session for surface. See ReadOnly: it never
// skips, for the same reason.
func (s *Sessions) SafeMode(t *testing.T, surface string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.safeMode[surface]
	if !ok {
		t.Fatalf("no safe-mode session was built for surface %q", surface)
	}
	return session
}

// callTool calls name on s with in as its arguments, fails the test on a
// transport error or a tool-reported error, and decodes the result's text
// content as O. Every later suite calls tools through this rather than
// repeating the decode-and-check boilerplate at each call site.
func callTool[O any](t *testing.T, s *mcp.ClientSession, name string, in any) O {
	t.Helper()
	var out O

	res, err := s.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: in})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	if res.IsError {
		t.Fatalf("CallTool(%s) returned an error result: %s", name, toolResultText(res))
	}
	if len(res.Content) == 0 {
		return out
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("CallTool(%s): content[0] is %T, want *mcp.TextContent", name, res.Content[0])
	}
	if err := json.Unmarshal([]byte(text.Text), &out); err != nil {
		t.Fatalf("CallTool(%s): decode result: %v\nraw: %s", name, err, text.Text)
	}
	return out
}

// toolResultText extracts the first text content from res, for error
// messages. It never fails: a result with no text content yields an empty
// string rather than crashing the test that only wants to report a failure.
func toolResultText(res *mcp.CallToolResult) string {
	if len(res.Content) == 0 {
		return ""
	}
	text, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		return ""
	}
	return text.Text
}

// TestSessions_EveryProvisionedSurfaceListsItsTools is the matrix smoke test:
// it proves every session built above can list tools, and that each surface
// publishes the shape it is supposed to. Dynamic and meta publish a fixed,
// small number of tools; the individual surface publishes one per action, so
// it is checked as "more than a few" rather than a literal that would need
// updating every time a domain is added.
func TestSessions_EveryProvisionedSurfaceListsItsTools(t *testing.T) {
	wantTools := map[string]func(int) bool{
		"dynamic":    func(n int) bool { return n == 3 },
		"meta":       func(n int) bool { return n >= 2 },
		"individual": func(n int) bool { return n > 3 },
	}

	for _, ed := range []string{"CE", "EE"} {
		for _, surface := range surfaceNames {
			want := wantTools[surface]
			t.Run(ed+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, ed)
				res, err := session.ListTools(t.Context(), nil)
				if err != nil {
					t.Fatalf("ListTools: %v", err)
				}
				if !want(len(res.Tools)) {
					names := make([]string, 0, len(res.Tools))
					for _, tool := range res.Tools {
						names = append(names, tool.Name)
					}
					t.Errorf("%s/%s published %d tools: %v", ed, surface, len(res.Tools), names)
				}
			})
		}
	}
}

// TestSessions_ReadOnlyAndSafeModeSessions_ListTools proves the read-only
// and safe-mode sessions are wired and reachable for every surface, the same
// way the matrix test above proves it for the edition axis.
func TestSessions_ReadOnlyAndSafeModeSessions_ListTools(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run("read-only/"+surface, func(t *testing.T) {
			res, err := sessions.ReadOnly(t, surface).ListTools(t.Context(), nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if len(res.Tools) == 0 {
				t.Error("read-only session published no tools")
			}
		})
		t.Run("safe-mode/"+surface, func(t *testing.T) {
			res, err := sessions.SafeMode(t, surface).ListTools(t.Context(), nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			if len(res.Tools) == 0 {
				t.Error("safe-mode session published no tools")
			}
		})
	}
}
