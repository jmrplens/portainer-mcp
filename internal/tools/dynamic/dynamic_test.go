package dynamic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/tools/actioncatalog"
	"github.com/jmrplens/portainer-mcp/internal/tools/system"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func connect(t *testing.T, server *mcp.Server) (*mcp.ClientSession, context.Context) {
	t.Helper()
	st, ct := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = session.Close() })
	return session, ctx
}

func serverFor(t *testing.T, deps tools.Deps) *mcp.Server {
	t.Helper()
	catalog, err := actioncatalog.Build(system.Specs(), actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, deps); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return server
}

// The defining property of this surface: two tools, whatever the catalog size.
func TestRegister_ExposesExactlyTwoTools(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(res.Tools) != 2 {
		t.Fatalf("tools = %d, want exactly 2", len(res.Tools))
	}
	names := res.Tools[0].Name + " " + res.Tools[1].Name
	for _, want := range []string{"portainer_find_action", "portainer_execute_action"} {
		if !strings.Contains(names, want) {
			t.Errorf("tools = %q, want %q present", names, want)
		}
	}
}

func TestFindAction_MatchesOnName(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "system version"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(text, "system.version") {
		t.Errorf("find(system version) = %q, want system.version among the matches", text)
	}
}

func TestFindAction_MatchesOnDescription(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "connectivity check"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "system.status") {
		t.Error("a query matching only a description found nothing")
	}
}

func TestFindAction_NoMatch_SaysSoAndSuggests(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "zzzz-no-such-thing"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if !strings.Contains(strings.ToLower(text), "no action") {
		t.Errorf("result = %q, want it to say plainly that nothing matched", text)
	}
}

func TestExecuteAction_UnknownAction_IsRefused(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "system.nonsense"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatal("IsError = false, want true for an unknown action")
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "portainer_find_action") {
		t.Error("the error does not point the model at find, which is how it recovers")
	}
}

// TestExecuteAction_OmittedInput_HandlerReceivesEmptyObject guards against the
// surface-dependent null/{} divergence: a caller that omits "input" entirely
// must still reach the handler with {}, the same as every other surface,
// because tools.Execute normalizes it centrally.
func TestExecuteAction_OmittedInput_HandlerReceivesEmptyObject(t *testing.T) {
	t.Parallel()
	var gotInput json.RawMessage
	specs := []toolutil.ActionSpec{{
		Name: "system.echo", Domain: "system", OperationID: "SystemInfo",
		Title: "t", Description: "d", Edition: edition.CE,
		Handler: func(_ context.Context, _ *portainer.Client, in json.RawMessage) (any, error) {
			gotInput = in
			return map[string]any{"ok": true}, nil
		},
	}}
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, tools.Deps{Client: &portainer.Client{}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	session, ctx := connect(t, server)

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "system.echo"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("CallTool returned an error: %+v", res.Content)
	}
	if string(gotInput) != "{}" {
		t.Errorf("handler received input = %q, want {}", gotInput)
	}
}

// Safe mode must intercept here too.
func TestExecuteAction_SafeMode_Intercepts(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{SafeMode: true}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_execute_action",
		Arguments: map[string]any{"action": "system.update", "input": map[string]any{}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Content[0].(*mcp.TextContent).Text), "safe mode") {
		t.Error("safe mode did not intercept through the dynamic surface")
	}
}

// The catalog built for this test includes system.update, a mutating action,
// so portainer_execute_action must not advertise itself as read-only: a
// client deciding whether to warn a user, or auto-approve, reads this hint.
func TestRegister_ExecuteAction_AnnotatedNotReadOnly_WhenCatalogHasMutatingActions(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	var execTool *mcp.Tool
	for _, tool := range res.Tools {
		if tool.Name == "portainer_execute_action" {
			execTool = tool
		}
	}
	if execTool == nil {
		t.Fatal("portainer_execute_action was not registered")
	}
	if execTool.Annotations == nil || execTool.Annotations.ReadOnlyHint {
		t.Error("portainer_execute_action is annotated ReadOnlyHint = true, but the catalog contains system.update, a mutating action")
	}
}

// find must report the action's danger, because on this surface it is the
// model's only chance to see it before calling execute.
func TestFindAction_ReportsMutatingAndDestructive(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "portainer_find_action",
		Arguments: map[string]any{"query": "update"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var result findResult
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &result); err != nil {
		t.Fatalf("find did not return JSON: %v", err)
	}
	for _, m := range result.Matches {
		if m.Action == "system.update" {
			if !m.Mutating {
				t.Error("system.update is not reported as mutating")
			}
			return
		}
	}
	t.Error("system.update was not among the matches for \"update\"")
}

// A broad query must not return the whole catalog, and must say how much it
// held back — a model that cannot tell it was truncated will assume it saw
// everything and pick badly.
func TestFindAction_ManyMatches_IsCappedAndSaysSo(t *testing.T) {
	t.Parallel()
	specs := make([]toolutil.ActionSpec, 0, 30)
	for i := range 30 {
		specs = append(specs, toolutil.ActionSpec{
			Name:        fmt.Sprintf("tags.list%02d", i),
			Domain:      "tags",
			OperationID: "TagList",
			Title:       "List tags",
			Description: "Lists every tag defined on this Portainer instance.",
			Edition:     edition.CE,
			Handler:     func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
		})
	}
	catalog, err := actioncatalog.Build(specs, actioncatalog.Options{Edition: edition.EE, ServerVersion: "2.44.0"})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "portainer-mcp", Version: "test"}, nil)
	if err := (Surface{}).Register(server, catalog, tools.Deps{Client: &portainer.Client{}}); err != nil {
		t.Fatalf("Register: %v", err)
	}
	session, ctx := connect(t, server)

	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "portainer_find_action", Arguments: map[string]any{"query": "tags"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	var got findResult
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &got); err != nil {
		t.Fatalf("find did not return the expected shape: %v", err)
	}
	if got.Matched != 30 {
		t.Errorf("matched = %d, want 30", got.Matched)
	}
	if len(got.Matches) != maxMatches {
		t.Errorf("returned %d matches, want them capped at %d", len(got.Matches), maxMatches)
	}
	if got.Note == "" {
		t.Error("a truncated result carries no note, so the model cannot tell it was truncated")
	}
}

// A result that fits must not claim truncation.
func TestFindAction_FewMatches_CarriesNoTruncationNote(t *testing.T) {
	t.Parallel()
	session, ctx := connect(t, serverFor(t, tools.Deps{Client: &portainer.Client{}}))
	res, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "portainer_find_action", Arguments: map[string]any{"query": "system version"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	var got findResult
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Note != "" {
		t.Errorf("note = %q, want empty when nothing was held back", got.Note)
	}
	if got.Matched != len(got.Matches) {
		t.Errorf("matched = %d but %d returned; they must agree when nothing is capped", got.Matched, len(got.Matches))
	}
}

// TestSearch_LongQuery_DoesNotLetDescriptionNoiseOutrankANameMatch is the
// regression guard for a defect the cap made worse: unbounded per-term
// description scoring let twenty-five noise actions outscore the one action
// actually named, and the cap then discarded it while claiming the survivors
// were the best matches.
func TestSearch_LongQuery_DoesNotLetDescriptionNoiseOutrankANameMatch(t *testing.T) {
	t.Parallel()
	const query = "please help me quickly and safely retrieve the current list right now today urgently immediately without delay"

	specs := []toolutil.ActionSpec{{
		Name: "tags.list", Domain: "tags", OperationID: "TagList",
		Title: "List tags", Description: "Lists tags.", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
	}}
	for i := range 25 {
		specs = append(specs, toolutil.ActionSpec{
			Name: fmt.Sprintf("tags.noise%02d", i), Domain: "tags", OperationID: "TagList",
			Title: "Noise", Description: query, Edition: edition.CE,
			Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil },
		})
	}

	matches := search(specs, query)
	if len(matches) == 0 {
		t.Fatal("search returned nothing")
	}
	if matches[0].Action != "tags.list" {
		t.Errorf("best match = %q, want tags.list: a name match must outrank description noise", matches[0].Action)
	}

	// Checking which action sorts first is not enough on its own: with 25 noise
	// actions named "tags.noise00".."tags.noise24", a tie at the same score
	// still sorts "tags.list" first purely because "list" < "noise"
	// alphabetically — a coincidence of these fixture names, not evidence the
	// scoring invariant holds. Compare the scores directly instead, since this
	// test runs in-package and can see the unexported field.
	var listScore, noiseScore int
	haveNoise := false
	for _, m := range matches {
		switch {
		case m.Action == "tags.list":
			listScore = m.score
		case !haveNoise && strings.HasPrefix(m.Action, "tags.noise"):
			noiseScore = m.score
			haveNoise = true
		}
	}
	if !haveNoise {
		t.Fatal("no noise action was present among the matches")
	}
	if listScore <= noiseScore {
		t.Errorf("tags.list score = %d, a noise action score = %d; a name match must strictly outscore description noise, not merely tie",
			listScore, noiseScore)
	}

	// And it must survive the cap the surface applies.
	within := false
	for i, m := range matches {
		if m.Action == "tags.list" {
			within = i < maxMatches
			break
		}
	}
	if !within {
		t.Error("tags.list fell outside the result cap, so the model would never see the correct action")
	}
}
