//go:build e2e

package suite

import (
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

// businessOnlySpecs returns every declared action whose Edition is Business
// Edition, read from wiring.AllSpecs — the same declaration list
// wiring.BuildCatalog builds every real catalog from — rather than a
// hardcoded slice of names.
//
// A hardcoded list is a list that goes stale silently: P3 grows the catalog
// from 18 actions to 441, and a Business-Edition-only action declared in
// that growth is covered by this test the day it is written only if the
// list it is checked against is derived, not maintained by hand alongside
// it.
func businessOnlySpecs(t *testing.T) []toolutil.ActionSpec {
	t.Helper()
	var out []toolutil.ActionSpec
	for _, spec := range wiring.AllSpecs() {
		if spec.Edition == edition.EE {
			out = append(out, spec)
		}
	}
	if len(out) == 0 {
		t.Fatal("businessOnlySpecs found no Business-Edition-only action; the pilot domains declare at least four (registries.ecr_delete_repository, registries.ecr_delete_tags, registries.repository_tags_delete, system.update) — this assumption is stale or the derivation is broken")
	}
	return out
}

// TestBusinessEditionActions_AreAbsentOnCommunity is the structural half of
// the edition negative matrix: an action declared Business-Edition-only must
// not be published as a tool on a Community Edition estate at all. This is
// checked on the individual surface because that is the surface where each
// action is its own, separately-named tool — the surface where "published"
// or "not published" is a directly observable fact about ListTools.
func TestBusinessEditionActions_AreAbsentOnCommunity(t *testing.T) {
	t.Parallel()
	session := sessions.For(t, "individual", "CE")
	res, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	published := map[string]bool{}
	for _, tool := range res.Tools {
		published[tool.Name] = true
	}
	for _, spec := range businessOnlySpecs(t) {
		name := actioncatalog.RenderToolName(spec.Name)
		if published[name] {
			t.Errorf("%s is Business Edition only but is published on a Community estate", name)
		}
	}
}

// TestBusinessEditionAction_CalledOnCommunity_FailsInformatively is the
// behavioural half: invoking a Business-Edition-only action by its canonical
// name, through the dynamic surface's find/execute pair, must fail with a
// message a model can act on rather than a bare 404.
//
// registries.ecr_delete_tags is the action under test, chosen because it is
// Business-Edition-only and requires no input to reach the "not in this
// edition's catalog" refusal — dynamic's execute handler looks the action up
// in the catalog before it ever reaches the handler that would validate
// input, so an empty input map never gets in the way of reaching that
// refusal.
//
// Two things were checked before trusting this test, because the standing
// warning for this whole plan is that a check can match something other than
// what it claims to:
//
//  1. Could a different failure produce the same text? A transport error
//     (session.CallTool returning err != nil) is caught separately above and
//     would fail the test with "CallTool: ..." before this assertion is ever
//     reached — it cannot silently satisfy it. A nil-client or other
//     Execute-level failure never runs here at all: dynamic's handler
//     (internal/tools/dynamic/dynamic.go) looks the action up in the catalog
//     first and returns this exact refusal without ever calling
//     tools.Execute when the lookup misses, which is exactly what happens
//     for an action absent from the Community Edition catalog. So the only
//     way to reach this text is the catalog lookup missing — which is what
//     "Business Edition only, called on Community" means.
//  2. The brief's own draft of this assertion checked for the lowercase
//     substring "unknown action". dynamic.go's actual refusal capitalises it
//     — "Unknown action %q. Call portainer_find_action..." — so a
//     case-sensitive check for the lowercase form would never match the
//     real message and this test would fail on a passing system, not report
//     a false pass. Matched here against the capitalised text the handler
//     actually emits, verified by reading internal/tools/dynamic/dynamic.go
//     directly rather than assumed from the brief.
func TestBusinessEditionAction_CalledOnCommunity_FailsInformatively(t *testing.T) {
	t.Parallel()
	session := sessions.For(t, "dynamic", "CE")
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "registries.ecr_delete_tags", "input": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("a Business Edition action succeeded against a Community estate")
	}
	text := toolResultText(res)
	// A model reading "404" learns nothing. It must learn that the action
	// exists, that this estate cannot run it, and why.
	for _, want := range []string{"registries.ecr_delete_tags", "Unknown action"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal = %q, want it to contain %q", text, want)
		}
	}
}
