// Scaffolded by cmd/gen_action_inputs from api/specs/ee-2.44.0.json, then frozen: this
// domain owns this file from here on (see P3.2's freeze and docs/domain-wave-checklist.md).
// Hand-edit it like any other source file; run `make audit-spec-drift` after any change that
// touches a parameter's shape, a redaction wrapper, or an identifier's minimum bound.

package custom_templates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// redactionGuards is one table entry per redaction wrapper in this domain —
// including CustomTemplateList's, whose handler is hand-written — consulted by TestUnit_RedactionGuards_RemoveEveryCredentialShapedField and
// TestUnit_RedactionGuards_HandlerRedactsCredentialShapedFields.
var redactionGuards = []struct {
	funcName    string
	operationID string
	fn          any
	handler     toolutil.Handler
}{
	{funcName: "redactCustomTemplateCreateRepository", operationID: "CustomTemplateCreateRepository", fn: redactCustomTemplateCreateRepository, handler: customTemplateCreateRepository},
	{funcName: "redactCustomTemplateCreateString", operationID: "CustomTemplateCreateString", fn: redactCustomTemplateCreateString, handler: customTemplateCreateString},
	{funcName: "redactCustomTemplateInspect", operationID: "CustomTemplateInspect", fn: redactCustomTemplateInspect, handler: customTemplateInspect},
	{funcName: "redactCustomTemplateList", operationID: "CustomTemplateList", fn: redactCustomTemplateList, handler: customTemplateList},
	{funcName: "redactCustomTemplateUpdate", operationID: "CustomTemplateUpdate", fn: redactCustomTemplateUpdate, handler: customTemplateUpdate},
}

// TestUnit_RedactionGuards_RemoveEveryCredentialShapedField proves every
// generated redaction wrapper in this domain actually strips what it is
// required to strip. cmd/gen_action_inputs refuses to emit a handler for a
// credential-returning operation unless a function of the expected name
// exists; existing is not the same as working, and this is what tells the
// two apart.
func TestUnit_RedactionGuards_RemoveEveryCredentialShapedField(t *testing.T) {
	t.Parallel()
	for _, g := range redactionGuards {
		t.Run(g.operationID, func(t *testing.T) {
			t.Parallel()
			fn := reflect.ValueOf(g.fn)
			if fn.Type().NumIn() != 1 || fn.Type().NumOut() != 1 {
				t.Fatalf("%s must take exactly one argument and return exactly one value, got %d in and %d out",
					g.funcName, fn.Type().NumIn(), fn.Type().NumOut())
			}

			// Populated, not zero-valued: WalkForCredentialShapedFields only
			// reports fields that actually carry a value, so a guard run
			// against a zero response would pass whatever the wrapper did.
			arg := reflect.New(fn.Type().In(0)).Elem()
			toolutil.PopulateForCredentialAudit(arg)

			survived := toolutil.AssertRedacted(fn.Call([]reflect.Value{arg})[0].Interface(), g.operationID)
			if len(survived) > 0 {
				t.Errorf("%s left %d credential-shaped field(s) populated: %v", g.funcName, len(survived), survived)
			}
		})
	}
}

// TestUnit_RedactionGuards_HandlerRedactsCredentialShapedFields proves the
// generated *handler* — not merely its redaction wrapper checked in
// isolation above — never returns a credential-shaped field to a caller.
// The wrapper guard above can pass while this fails: a handler that calls
// its wrapper and discards the result, that calls it from an unreachable
// branch, or that calls the wrong wrapper entirely all satisfy the wrapper
// guard (the wrapper itself, called directly, still works) and
// checkCredentialRedaction's static AST scan (the call site exists) while
// still leaking. Running the actual handler end to end against a synthetic
// response built the same way the wrapper guard's fixture is (a real,
// non-zero credential value, not an empty one — see PopulateForCredentialAudit's
// own doc comment for why an empty fixture proves nothing) is what a static
// scan or an isolated wrapper call cannot.
//
// An entry whose handler is nil (an overridden operation whose handler is
// hand-declared under a name this generator did not choose) is skipped:
// there is no mechanically-known function to call, and the domain author's
// own hand-written test — registries' TestRegistryInspect_ResponseWithPassword_IsRedacted
// is the precedent this generalises from — is what covers it instead.
func TestUnit_RedactionGuards_HandlerRedactsCredentialShapedFields(t *testing.T) {
	t.Parallel()
	for _, g := range redactionGuards {
		if g.handler == nil {
			continue
		}
		t.Run(g.operationID, func(t *testing.T) {
			t.Parallel()
			fn := reflect.ValueOf(g.fn)
			arg := reflect.New(fn.Type().In(0)).Elem()
			toolutil.PopulateForCredentialAudit(arg)

			body, err := json.Marshal(arg.Interface())
			if err != nil {
				t.Fatalf("marshal synthetic response: %v", err)
			}

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write(body)
			}))
			t.Cleanup(server.Close)

			c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
			if err != nil {
				t.Fatalf("portainer.New: %v", err)
			}

			out, err := g.handler(context.Background(), c, json.RawMessage("{}"))
			if err != nil {
				t.Fatalf("%s: handler error = %v", g.operationID, err)
			}

			survived := toolutil.AssertRedacted(out, g.operationID)
			if len(survived) > 0 {
				t.Errorf("%s's handler returned %d credential-shaped field(s) to a caller: %v", g.operationID, len(survived), survived)
			}
		})
	}
}
