package tools

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func readOnlySpec() toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: "tags.list", Domain: "tags", OperationID: "TagList",
		Title: "List tags", Description: "d", Edition: edition.CE,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
			return map[string]any{"tags": []string{"a"}}, nil
		},
	}
}

func destructiveSpec(executed *bool) toolutil.ActionSpec {
	return toolutil.ActionSpec{
		Name: "tags.delete", Domain: "tags", OperationID: "TagDelete",
		Title: "Delete tag", Description: "d", Edition: edition.CE,
		Mutating: true, Destructive: true,
		Handler: func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
			*executed = true
			return map[string]any{"deleted": true}, nil
		},
	}
}

func TestExecute_ReadOnlyAction_RunsTheHandler(t *testing.T) {
	t.Parallel()
	// A non-nil client: this test is about the handler running successfully,
	// not about client-nilness, which TestExecute_NilClient_* covers on its own.
	res, err := Execute(context.Background(), readOnlySpec(), Deps{Client: &portainer.Client{}}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.IsError {
		t.Fatalf("Execute() returned IsError: %+v", res.Content)
	}
}

// The guard that matters: in safe mode a destructive action must return a
// preview and its handler must never run.
func TestExecute_SafeMode_InterceptsMutatingActionWithoutRunningIt(t *testing.T) {
	t.Parallel()
	executed := false
	res, err := Execute(context.Background(), destructiveSpec(&executed), Deps{SafeMode: true}, json.RawMessage(`{"id":3}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executed {
		t.Fatal("safe mode ran the handler; it must only preview")
	}
	if res.IsError {
		t.Error("the safe-mode preview is reported as a tool error; it is a successful, informative result")
	}
	text := res.Content[0].(*mcp.TextContent).Text
	for _, want := range []string{"safe mode", "tags.delete", "destructive"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
			t.Errorf("preview = %q, want it to mention %q", text, want)
		}
	}
}

func TestExecute_SafeMode_LeavesReadOnlyActionsAlone(t *testing.T) {
	t.Parallel()
	// A non-nil client: without one, the nil-client guard would short-circuit
	// before the handler ever ran, and this test would pass regardless of
	// whether safe mode actually left the action alone.
	res, err := Execute(context.Background(), readOnlySpec(), Deps{SafeMode: true, Client: &portainer.Client{}}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if res.IsError {
		t.Fatalf("Execute() returned IsError: %+v", res.Content)
	}
	if strings.Contains(strings.ToLower(res.Content[0].(*mcp.TextContent).Text), "safe mode") {
		t.Error("safe mode intercepted a read-only action")
	}
}

func TestExecute_HandlerError_BecomesToolError(t *testing.T) {
	t.Parallel()
	spec := readOnlySpec()
	spec.Handler = func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
		return nil, errors.New("portainer said no")
	}
	// A non-nil client: this test is about the handler's own error propagating,
	// not about client-nilness, which TestExecute_NilClient_* covers on its own.
	res, err := Execute(context.Background(), spec, Deps{Client: &portainer.Client{}}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() returned a Go error, want a tool error result: %v", err)
	}
	if !res.IsError {
		t.Fatal("res.IsError = false, want true for a failing handler")
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "portainer said no") {
		t.Error("the tool error does not carry the handler's message")
	}
}

// Annotations are how a client decides whether to confirm with the user, so a
// destructive action that advertises itself as read-only is a safety defect.
func TestAnnotationsFor_DestructiveAction_IsNotMarkedReadOnly(t *testing.T) {
	t.Parallel()
	a := AnnotationsFor(destructiveSpec(new(bool)))
	if a.ReadOnlyHint {
		t.Error("ReadOnlyHint = true for a destructive action")
	}
	if a.DestructiveHint == nil || !*a.DestructiveHint {
		t.Error("DestructiveHint is not set for a destructive action")
	}
}

func TestAnnotationsFor_ReadOnlyAction_IsMarkedReadOnly(t *testing.T) {
	t.Parallel()
	a := AnnotationsFor(readOnlySpec())
	if !a.ReadOnlyHint {
		t.Error("ReadOnlyHint = false for a read-only action")
	}
}

// A malformed input must not cost the preview its framing: a model that reads
// only an encode error cannot tell safe mode blocked the call, and may retry.
func TestExecute_SafeMode_MalformedInput_StillReportsSafeMode(t *testing.T) {
	t.Parallel()
	executed := false
	res, err := Execute(context.Background(), destructiveSpec(&executed), Deps{SafeMode: true}, json.RawMessage(`{not json`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executed {
		t.Fatal("the handler ran despite safe mode")
	}
	if res.IsError {
		t.Error("a malformed input turned the preview into a tool error")
	}
	text := strings.ToLower(res.Content[0].(*mcp.TextContent).Text)
	if !strings.Contains(text, "safe mode") {
		t.Errorf("preview = %q, want it to still say safe mode blocked the call", text)
	}
}

func TestExecute_SafeMode_PreviewReportsFieldNamesNotValues(t *testing.T) {
	t.Parallel()
	executed := false
	res, err := Execute(context.Background(), destructiveSpec(&executed), Deps{SafeMode: true},
		json.RawMessage(`{"password":"hunter2","id":3}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	text := res.Content[0].(*mcp.TextContent).Text
	if strings.Contains(text, "hunter2") {
		t.Error("the preview echoed an input value back through the tool response")
	}
	if !strings.Contains(text, "password") {
		t.Error("the preview does not name the fields that would have been sent")
	}
}

// Safe mode previews need no client, so a nil one must not stop the preview.
func TestExecute_SafeModeWithNilClient_StillPreviews(t *testing.T) {
	t.Parallel()
	executed := false
	res, err := Execute(context.Background(), destructiveSpec(&executed), Deps{SafeMode: true}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if executed {
		t.Fatal("the handler ran")
	}
	if res.IsError {
		t.Error("a nil client turned a safe-mode preview into an error")
	}
	if !strings.Contains(strings.ToLower(res.Content[0].(*mcp.TextContent).Text), "safe mode") {
		t.Error("the preview lost its safe-mode framing")
	}
}

func TestExecute_NilClient_ReturnsToolErrorRatherThanPanicking(t *testing.T) {
	t.Parallel()
	spec := readOnlySpec()
	spec.Handler = func(_ context.Context, c *portainer.Client, _ json.RawMessage) (any, error) {
		// A real handler would dereference the client here and panic.
		return c.API, nil
	}

	res, err := Execute(context.Background(), spec, Deps{}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !res.IsError {
		t.Fatal("res.IsError = false, want a tool error for a missing client")
	}
	if !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "client") {
		t.Error("the error does not say a client is missing")
	}
}

func TestExecute_ForwardsContextClientAndInputToTheHandler(t *testing.T) {
	t.Parallel()
	type ctxKey struct{}
	wantCtx := context.WithValue(context.Background(), ctxKey{}, "sentinel")
	wantClient := &portainer.Client{}
	wantInput := json.RawMessage(`{"id":7}`)

	var gotCtx context.Context
	var gotClient *portainer.Client
	var gotInput json.RawMessage

	spec := readOnlySpec()
	spec.Handler = func(ctx context.Context, c *portainer.Client, in json.RawMessage) (any, error) {
		gotCtx, gotClient, gotInput = ctx, c, in
		return map[string]any{"ok": true}, nil
	}

	if _, err := Execute(wantCtx, spec, Deps{Client: wantClient}, wantInput); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if gotCtx == nil || gotCtx.Value(ctxKey{}) != "sentinel" {
		t.Error("the caller's context was not forwarded to the handler")
	}
	if gotClient != wantClient {
		t.Error("the configured client was not forwarded to the handler")
	}
	if string(gotInput) != string(wantInput) {
		t.Errorf("input forwarded as %q, want %q", gotInput, wantInput)
	}
}
