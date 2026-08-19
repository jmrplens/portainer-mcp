//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/logging"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
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
	byKey      map[string]*mcp.ClientSession
	readOnly   map[string]*mcp.ClientSession
	safeMode   map[string]*mcp.ClientSession
	safeModeEE map[string]*mcp.ClientSession
}

// newSessions builds every session the matrix needs from a loaded estate.
//
// It builds a normal session per surface for each compose leg the estate
// actually carries. Read-only and safe-mode sessions are built once per
// surface against the Community Edition leg only: For's edition axis exists
// to let a licence-less contributor skip Business Edition suites, but the
// mode a session runs in does not depend on which edition backs it, and CE
// is the leg any compose estate carries.
//
// "Any compose estate" is the load-bearing qualifier, and it used to read
// "always present". CI provisions the compose legs and the Kubernetes leg in
// two separate jobs, so an estate carrying only the Kubernetes leg is now a
// normal shape and builds no read-only or safe-mode session at all — see
// ReadOnly, which skips rather than fails in exactly that case.
//
// A Business Edition safe-mode session is the one exception, built only when
// a licence was provisioned: it exists specifically to exercise a
// Business-Edition-only destructive action (system.update, and P3 will add
// more) without ever letting its handler run — see SafeModeEE.
func newSessions(e harness.Estate) (*Sessions, error) {
	s := &Sessions{
		byKey:      make(map[string]*mcp.ClientSession),
		readOnly:   make(map[string]*mcp.ClientSession),
		safeMode:   make(map[string]*mcp.ClientSession),
		safeModeEE: make(map[string]*mcp.ClientSession),
	}

	logger := logging.New(slog.LevelWarn)

	// composeLegs (fixtures_test.go) derives this from e.Legs() rather than
	// a hardcoded map literal — task 7b's fix. The Kubernetes leg is filtered
	// out there for a documented reason (no pinned CA in this process to
	// verify its self-signed certificate against); see
	// TestKubernetesLeg_ReachableThroughEveryMCPSurface in kubernetes_test.go
	// for the session that leg is reached through instead.
	for _, leg := range composeLegs(e) {
		for _, surface := range surfaceNames {
			session, err := buildSession(leg.Server, config.ToolSurface(surface), false, false, false, logger)
			if err != nil {
				return nil, fmt.Errorf("build %s/%s session: %w", surface, leg.Name, err)
			}
			s.byKey[surface+"/"+leg.Name] = session
		}
	}

	// Guarded on the Community Edition leg actually being present, which it no
	// longer always is: CI provisions the compose legs and the Kubernetes leg
	// in two separate jobs (one licence, one Portainer instance at a time —
	// see .github/workflows/e2e.yml), so the Kubernetes job's estate carries
	// no compose leg at all. Building these unguarded against a zero
	// harness.Server would fail newSessions outright, and TestMain turns that
	// into an exit(1) before a single test runs — a whole suite reporting
	// nothing rather than a suite skipping visibly.
	if e.HasCommunityEdition() {
		for _, surface := range surfaceNames {
			ro, err := buildSession(e.CE, config.ToolSurface(surface), true, false, false, logger)
			if err != nil {
				return nil, fmt.Errorf("build %s read-only session: %w", surface, err)
			}
			s.readOnly[surface] = ro

			sm, err := buildSession(e.CE, config.ToolSurface(surface), false, true, false, logger)
			if err != nil {
				return nil, fmt.Errorf("build %s safe-mode session: %w", surface, err)
			}
			s.safeMode[surface] = sm
		}
	}

	if e.HasBusinessEdition() {
		for _, surface := range surfaceNames {
			smEE, err := buildSession(e.EE, config.ToolSurface(surface), false, true, false, logger)
			if err != nil {
				return nil, fmt.Errorf("build %s Business Edition safe-mode session: %w", surface, err)
			}
			s.safeModeEE[surface] = smEE
		}
	}

	return s, nil
}

// buildSession constructs one Portainer client, catalog and MCP server the
// same way cmd/portainer-mcp's run() does, then connects a client session to
// it over mcp.NewInMemoryTransports(): no sockets, no subprocess, no port
// allocation, and a panic in a handler surfaces as a test failure rather
// than a hung read. The Portainer client underneath is real and talks to the
// estate over HTTP; only the MCP transport is in-memory.
//
// skipTLSVerify exists for the Kubernetes leg alone: its Helm chart deploys a
// self-signed certificate, and unlike the provisioner (see
// harness.ClientWithCA), the estate file that survives into this process
// carries no pinned CA to verify it against — only a base URL and
// credentials. Every compose leg is plain HTTP, so every other caller passes
// false.
func buildSession(srv harness.Server, surface config.ToolSurface, readOnly, safeMode, skipTLSVerify bool, logger *slog.Logger) (*mcp.ClientSession, error) {
	cfg := &config.Config{
		URL:           srv.BaseURL,
		Token:         srv.Creds.APIKey,
		ToolSurface:   surface,
		ReadOnly:      readOnly,
		SafeMode:      safeMode,
		SkipTLSVerify: skipTLSVerify,
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
	for _, m := range []map[string]*mcp.ClientSession{s.byKey, s.readOnly, s.safeMode, s.safeModeEE} {
		for _, session := range m {
			_ = session.Close()
		}
	}
}

// Editions returns the name of every edition a domain suite is expected to
// exercise — the editions the action catalog itself declares actions for,
// read from wiring.AllSpecs, sorted for deterministic iteration. It does not
// read this Sessions at all, which is the point, and is why its receiver is
// unnamed.
//
// It used to answer "the editions that were actually built", derived from
// byKey. That was task 7b's replacement for a []string{"CE", "EE"} literal
// repeated at the top of every per-domain suite, and it fixed the problem it
// was aimed at — but it introduced another: an estate with no Business
// Edition leg made every EE subtest cease to exist rather than skip. A suite
// that ranges over nothing passes, silently, and its output is
// indistinguishable from a run that measured that edition and found it
// correct. That was carried in docs/open-follow-ups.md until this change
// closed it.
//
// CI's split into two sequential jobs (see .github/workflows/e2e.yml) turned
// that from an inconvenience into a trap: each job now sees half an estate by
// design, so a derived-from-what-was-built axis would have both jobs report a
// full green pass between them while neither measured the other's half.
//
// Ranging over the catalog instead is the fix that follow-up prescribed for
// itself, and it is still derived rather than hand-maintained: an edition no action
// is declared for cannot appear here, and one declared for a third edition
// would appear without anything being edited. Sessions.For turns each edition
// this estate lacks into a named, visible skip — one "--- SKIP" line per
// edition per suite, which is exactly the signal that was missing.
func (*Sessions) Editions() []string {
	seen := map[string]bool{}
	for _, spec := range wiring.AllSpecs() {
		seen[string(spec.Edition)] = true
	}
	editions := make([]string, 0, len(seen))
	for ed := range seen {
		editions = append(editions, ed)
	}
	sort.Strings(editions)
	return editions
}

// For returns the session for one surface and edition, skipping when that
// edition is not part of the estate. A contributor without the Business
// Edition licence must still be able to run the Community Edition suites, and
// — since the CI split described on Editions — a job that provisioned only
// the Kubernetes leg must still be able to run the whole suite and have it
// say, out loud, what it did not measure.
//
// The skip message names the command that would provision the missing leg,
// because the two reasons an edition is absent have different remedies: no
// Business Edition licence in .env (nothing to do but supply one), or a
// compose estate that was never brought up at all.
func (s *Sessions) For(t *testing.T, surface, edition string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.byKey[surface+"/"+edition]
	if !ok {
		t.Skipf("no %s session for %s: that edition is not provisioned in this estate (%s)",
			surface, edition, provisioningHint(edition))
	}
	return session
}

// provisioningHint names what would have provisioned the edition an absent
// session was asked for, for the skip messages above. It is deliberately not
// a claim about why this particular run lacks it — the estate cannot tell
// "no licence" apart from "compose leg never started" from a missing map
// entry alone — only about what provisions it at all.
func provisioningHint(edition string) string {
	if edition == "EE" {
		return "`make e2e-up` with a PORTAINER_LICENSE in .env provisions it"
	}
	return "`make e2e-up` provisions it"
}

// ReadOnly returns the read-only-mode session for surface, skipping when
// this estate has no Community Edition leg — the leg every read-only and
// safe-mode session is built against — and failing when it has one and the
// session is missing anyway.
//
// The two cases must not be collapsed. "No compose leg was provisioned" is
// an ordinary shape since CI's split into two jobs (see Editions): the
// Kubernetes job's estate has none, and these suites have to say so rather
// than fail. "The estate has a Community Edition leg and newSessions still
// did not build this session" is a harness defect, and turning that into a
// skip would let a broken harness look like an unprovisioned leg — the exact
// substitution this whole change is about, in the opposite direction.
func (s *Sessions) ReadOnly(t *testing.T, surface string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.readOnly[surface]
	if !ok {
		if !estate.HasCommunityEdition() {
			t.Skipf("no %s read-only session: this estate provisions no Community Edition leg "+
				"(`make e2e-up` provisions it), and read-only mode is exercised against that leg", surface)
		}
		t.Fatalf("no read-only session was built for surface %q", surface)
	}
	return session
}

// SafeMode returns the safe-mode session for surface. See ReadOnly: it skips
// and fails for the same two distinct reasons.
func (s *Sessions) SafeMode(t *testing.T, surface string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.safeMode[surface]
	if !ok {
		if !estate.HasCommunityEdition() {
			t.Skipf("no %s safe-mode session: this estate provisions no Community Edition leg "+
				"(`make e2e-up` provisions it), and safe mode is exercised against that leg", surface)
		}
		t.Fatalf("no safe-mode session was built for surface %q", surface)
	}
	return session
}

// SafeModeEE returns the safe-mode session for surface built against the
// Business Edition leg, skipping when no licence was provisioned for this
// estate — the same skip behaviour as For, since a contributor without the
// licence must still be able to run every Community Edition suite.
//
// It exists so a Business-Edition-only destructive action (system.update
// today; P3 will add more) can be exercised through safe mode's interception
// rather than left unexercised for lack of a session that would otherwise
// let its handler run for real against the live Business Edition server.
func (s *Sessions) SafeModeEE(t *testing.T, surface string) *mcp.ClientSession {
	t.Helper()
	session, ok := s.safeModeEE[surface]
	if !ok {
		t.Skipf("no %s Business Edition safe-mode session: no licence was provisioned for this estate", surface)
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

// callAction is the one helper that hides the three surfaces' differences
// from a domain test, so a domain test is written once and runs three times:
//
//   - individual calls one tool per action, named by actioncatalog.RenderToolName.
//   - meta calls one tool per domain ("portainer_<domain>") with an "action"
//     argument naming which of that domain's actions to run.
//   - dynamic calls the fixed "portainer_execute_action" tool with an
//     "action" argument naming the canonical action, catalog-wide.
//
// It delegates the individual surface's tool name to
// actioncatalog.RenderToolName rather than reimplementing the "." -> "_"
// mapping: Task 9's audit imports that same function to recognise an
// individual-surface reference, and a second, hand-rolled copy here could
// silently drift from it.
func callAction[O any](t *testing.T, s *mcp.ClientSession, surface, action string, in map[string]any) O {
	t.Helper()
	toolName, args := actionCallParams(t, surface, action, in)
	return callTool[O](t, s, toolName, args)
}

// actionCallParams computes the tool name and arguments callAction would use
// for surface and action, without calling the tool.
//
// It exists so a test that must inspect a tool-error result directly — a
// call expected to fail, such as an input the handler itself rejects, or a
// Business-Edition-only action invoked against a Community estate — can
// still route through the same surface-to-tool-name mapping callAction uses,
// rather than keeping a second copy that could drift from it. callAction
// itself cannot serve that case: callTool underneath it fails the test on
// any IsError result, which is exactly what a "this call must fail" test
// needs to see rather than treat as a test-harness failure.
func actionCallParams(t *testing.T, surface, action string, in map[string]any) (string, any) {
	t.Helper()

	// wrapAction builds the {"action": ..., "input": ...} envelope meta and
	// dynamic both take. "input" is omitted entirely when in is nil, rather
	// than sent as an explicit JSON null: the MCP SDK infers both surfaces'
	// input field as a JSON Schema object (map[string]any, not
	// json.RawMessage — see meta.Input and dynamic.ExecuteInput's own
	// comments on why), and schema validation rejects null against an
	// object-typed field before the request ever reaches the handler that
	// would otherwise treat "no input" and "null input" the same way.
	wrapAction := func() map[string]any {
		args := map[string]any{"action": action}
		if in != nil {
			args["input"] = in
		}
		return args
	}

	switch surface {
	case "individual":
		return actioncatalog.RenderToolName(action), in
	case "meta":
		domain, _, ok := strings.Cut(action, ".")
		if !ok {
			t.Fatalf("actionCallParams: action %q is not domain-qualified", action)
		}
		return "portainer_" + domain, wrapAction()
	case "dynamic":
		return "portainer_execute_action", wrapAction()
	default:
		t.Fatalf("actionCallParams: unknown surface %q, want individual, meta or dynamic", surface)
		return "", nil
	}
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

	for _, ed := range sessions.Editions() {
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

// TestUnit_Editions_NamesEveryCatalogEditionEvenWhenNoneWereBuilt is the
// guard on the visible-skip fix: the edition axis every domain suite ranges
// over must come from what the catalog declares, never from what this
// particular estate happened to provision.
//
// It asks a Sessions that built nothing at all, because that is the one
// question that separates the two derivations no matter which estate the
// suite is run against. The implementation this replaced answered "no
// editions" here, and a domain suite ranging over no editions runs no
// subtests, reports nothing, and passes — the silent half-measurement the
// CI split would otherwise have made routine on both of its jobs at once.
//
// The second assertion guards the derivation rather than the call: an axis
// read from wiring.AllSpecs is only as wide as the specs declare, so if the
// catalog ever stopped declaring a Business Edition action this would shrink
// back to ["CE"] and take every EE subtest with it, silently. That is the
// same assumption edition_test.go's businessOnlySpecs already refuses to
// leave unstated.
func TestUnit_Editions_NamesEveryCatalogEditionEvenWhenNoneWereBuilt(t *testing.T) {
	t.Parallel()

	built := (&Sessions{byKey: map[string]*mcp.ClientSession{}}).Editions()
	if len(built) == 0 {
		t.Fatal("Editions() on a Sessions that built nothing = [], want the editions the catalog declares: " +
			"a suite ranging over this would run no subtests and pass by measuring nothing")
	}

	want := map[string]bool{"CE": true, "EE": true}
	for _, ed := range built {
		delete(want, ed)
	}
	if len(want) != 0 {
		t.Errorf("Editions() = %v, missing %v: the catalog declares actions for both editions "+
			"(see businessOnlySpecs in edition_test.go), so both must appear on the axis", built, want)
	}

	// And the live Sessions must answer identically to the empty one: the
	// axis is a property of the catalog, not of this estate.
	if live := sessions.Editions(); !reflect.DeepEqual(live, built) {
		t.Errorf("sessions.Editions() = %v, want %v: the axis still depends on what this estate provisioned", live, built)
	}
}

// TestSessions_For_AnAbsentEditionSkipsVisibly is the other half of the same
// fix, checked against whatever estate this run actually has: every edition
// on the axis must produce a subtest, and that subtest must either run or
// skip — never silently fail to exist.
//
// It reads the outcome through t.Cleanup rather than the bool t.Run returns,
// which cannot tell a skip from a pass: Skipf ends the subtest's goroutine,
// so a cleanup registered first is the only thing that still observes
// t.Skipped() afterwards.
//
// On a fully provisioned compose estate every row runs and this proves the
// axis is not skipping things that are present. On a half estate — either CI
// job, or any contributor without a licence — the rows for the missing leg
// skip, and the run's own output names them. That difference, visible in the
// log, is the whole point.
func TestSessions_For_AnAbsentEditionSkipsVisibly(t *testing.T) {
	editions := sessions.Editions()
	if len(editions) == 0 {
		t.Fatal("sessions.Editions() is empty: this test would pass by running nothing")
	}

	for _, ed := range editions {
		provisioned := ed == "CE" && estate.HasCommunityEdition() ||
			ed == "EE" && estate.HasBusinessEdition()

		var skipped bool
		ran := t.Run(ed, func(t *testing.T) {
			t.Cleanup(func() { skipped = t.Skipped() })
			session := sessions.For(t, "dynamic", ed)
			if session == nil {
				t.Fatalf("For(dynamic, %s) returned a nil session without skipping", ed)
			}
		})
		if !ran {
			t.Errorf("the %s row failed; it must either run against a provisioned leg or skip with a named reason", ed)
			continue
		}
		if skipped == provisioned {
			t.Errorf("the %s row skipped = %v on an estate where that leg provisioned = %v: "+
				"a provisioned leg must be measured and an absent one must say so", ed, skipped, provisioned)
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

// TestSafeMode_TagsCreate_DoesNotActuallyCreateATag proves safe mode
// discriminates a genuine interception from a silent no-op, the way
// TestSystemUpdate_SafeModePreview_DoesNotActuallyRestart in system_test.go
// no longer can (see that test's own comment): a whole-branch review measured
// that InstanceID, the signal that test used to rely on, survives both a
// `docker restart` of the container and an image replacement, so comparing it
// before and after a safe-mode call never actually detected a restart at all.
//
// tags.create is the right action to prove this property with instead of a
// system action: its side effect (a new tag on the server) is both
// observable through a raw API read and, unlike an actual system restart,
// harmless to let happen for real if safe mode ever regressed. A preview
// response and a safe_mode:true flag alone are satisfied identically by a
// correct interception and by a handler that silently no-ops before ever
// calling the Portainer API — this is the read-back that tells them apart,
// the same rule TestTags_CreateThenListThenDelete's own read-back follows for
// a real create.
//
// Built against the Community Edition safe-mode session (Sessions.SafeMode),
// not the Business Edition one: tags exists on both editions and this
// property has nothing to do with edition, so it is proven against the leg
// every estate always carries, licensed or not.
func TestSafeMode_TagsCreate_DoesNotActuallyCreateATag(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.SafeMode(t, surface)

			name := uniqueName("tag")
			out := callAction[map[string]any](t, session, surface, "tags.create", map[string]any{"name": name})
			if safeMode, _ := out["safe_mode"].(bool); !safeMode {
				t.Fatalf("tags.create under safe mode = %v, want a safe_mode preview, not a real call", out)
			}
			if out["action"] != "tags.create" {
				t.Errorf("safe-mode preview action = %v, want %q", out["action"], "tags.create")
			}

			assertTagAbsent(t, name, "safe mode intercepting tags.create")
		})
	}
}

// TestReadOnly_TagsCreate_IsHiddenRefusedAndNeverExecutes proves read-only
// mode's three claims together, on all three surfaces: the mutating action is
// hidden from that surface's own discovery mechanism, calling it by name is
// refused, and nothing was written.
//
// TestSessions_ReadOnlyAndSafeModeSessions_ListTools's own
// `len(res.Tools) == 0` check cannot prove any of this: it never names
// tags.create at all, so it passes identically whether read-only mode hides
// mutating actions correctly or the read-only sessions were built with
// ReadOnly: false by mistake — the exact mutation a whole-branch review
// applied, which left every existing assertion in this package green. The
// first two properties here are checked per surface, because "hidden" means
// something different on each: the individual surface omits the tool
// entirely from ListTools; the meta surface's per-domain tool description
// (describeActions) stops enumerating the action; the dynamic surface's
// portainer_find_action stops matching it, because actioncatalog.Build's
// ReadOnly filter removes the action from the catalog before any surface
// ever sees it. The third property — nothing was written — is what makes the
// first two trustworthy: either alone would pass against a server that hid
// the tool, or refused it with the right words, and executed it anyway.
func TestReadOnly_TagsCreate_IsHiddenRefusedAndNeverExecutes(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			ro := sessions.ReadOnly(t, surface)

			switch surface {
			case "individual":
				roRes, err := ro.ListTools(t.Context(), nil)
				if err != nil {
					t.Fatalf("ListTools: %v", err)
				}
				for _, tool := range roRes.Tools {
					if tool.Name == "portainer_tags_create" {
						t.Error("read-only individual surface published portainer_tags_create")
					}
				}
				normal := sessions.For(t, "individual", "CE")
				normalRes, err := normal.ListTools(t.Context(), nil)
				if err != nil {
					t.Fatalf("ListTools (normal session): %v", err)
				}
				if len(roRes.Tools) >= len(normalRes.Tools) {
					t.Errorf("read-only individual surface published %d tools, want fewer than the normal session's %d",
						len(roRes.Tools), len(normalRes.Tools))
				}
			case "meta":
				res, err := ro.ListTools(t.Context(), nil)
				if err != nil {
					t.Fatalf("ListTools: %v", err)
				}
				found := false
				for _, tool := range res.Tools {
					if tool.Name != "portainer_tags" {
						continue
					}
					found = true
					if strings.Contains(tool.Description, "tags.create") {
						t.Errorf("read-only portainer_tags description still advertises tags.create: %q", tool.Description)
					}
				}
				if !found {
					t.Fatal("read-only meta surface published no portainer_tags tool: this check would pass vacuously")
				}
			case "dynamic":
				res, err := ro.CallTool(t.Context(), &mcp.CallToolParams{
					Name: "portainer_find_action", Arguments: map[string]any{"query": "tags"},
				})
				if err != nil {
					t.Fatalf("CallTool(portainer_find_action): %v", err)
				}
				if res.IsError {
					t.Fatalf("portainer_find_action returned an error result: %s", toolResultText(res))
				}
				if text := toolResultText(res); strings.Contains(text, "tags.create") {
					t.Errorf("read-only dynamic surface's find_action for %q still surfaced tags.create: %s", "tags", text)
				}
			}

			name := uniqueName("tag")
			toolName, args := actionCallParams(t, surface, "tags.create", map[string]any{"name": name})
			res, err := ro.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			switch surface {
			case "individual":
				if err == nil {
					t.Fatal("CallTool(portainer_tags_create) on a read-only session succeeded, want a refusal")
				}
				const want = `unknown tool "portainer_tags_create"`
				if !strings.Contains(err.Error(), want) {
					t.Errorf("refusal = %q, want it to contain %q", err.Error(), want)
				}
			case "meta":
				if err != nil {
					t.Fatalf("CallTool: %v", err)
				}
				if !res.IsError {
					t.Fatal("tags.create through the read-only meta session succeeded, want a refusal")
				}
				const want = `unknown action "tags.create" for domain "tags"`
				if text := toolResultText(res); !strings.Contains(text, want) {
					t.Errorf("refusal = %q, want it to contain %q", text, want)
				}
			case "dynamic":
				if err != nil {
					t.Fatalf("CallTool: %v", err)
				}
				if !res.IsError {
					t.Fatal("tags.create through the read-only dynamic session succeeded, want a refusal")
				}
				const want = `Unknown action "tags.create"`
				if text := toolResultText(res); !strings.Contains(text, want) {
					t.Errorf("refusal = %q, want it to contain %q", text, want)
				}
			}

			assertTagAbsent(t, name, "read-only mode refusing tags.create")
		})
	}
}

// assertTagAbsent fails t if a tag named name exists on the Community
// Edition server, read directly through the Portainer API rather than
// through any MCP surface — the one check that cannot be satisfied by a
// surface that hides or refuses a call and executes it anyway. reason names,
// in the failure message, what was supposed to have prevented the tag from
// existing.
func assertTagAbsent(t *testing.T, name, reason string) {
	t.Helper()
	client := fixtureClient(t, "CE")
	resp, err := client.API.TagListWithResponse(t.Context())
	if err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if err := toolutil.Check(resp); err != nil {
		t.Fatalf("list tags: %v", err)
	}
	if resp.JSON200 == nil {
		return
	}
	for _, tag := range *resp.JSON200 {
		if tag.Name != nil && *tag.Name == name {
			t.Errorf("tag %q exists on the server despite %s", name, reason)
		}
	}
}
