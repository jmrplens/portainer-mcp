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
	res, err := Execute(context.Background(), readOnlySpec(), Deps{}, json.RawMessage(`{}`))
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
	text := res.Content[0].(*mcp.TextContent).Text
	for _, want := range []string{"safe mode", "tags.delete", "destructive"} {
		if !strings.Contains(strings.ToLower(text), strings.ToLower(want)) {
			t.Errorf("preview = %q, want it to mention %q", text, want)
		}
	}
}

func TestExecute_SafeMode_LeavesReadOnlyActionsAlone(t *testing.T) {
	t.Parallel()
	res, err := Execute(context.Background(), readOnlySpec(), Deps{SafeMode: true}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
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
	res, err := Execute(context.Background(), spec, Deps{}, json.RawMessage(`{}`))
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
