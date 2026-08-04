package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// fixtureRedactWrapper is the wrapper wrapperFixtureHandlerCallsWrapper below
// calls, and wrapperFixtureHandlerLeaksDirectly deliberately does not.
func fixtureRedactWrapper(v any) any { return v }

// wrapperFixtureHandlerCallsWrapper is the "safe" shape: a handler that
// returns the result of calling the required wrapper, exactly like a real
// generated handler (e.g. registries' registryInspect returning
// redactRegistryInspect(resp.JSON200), nil).
func wrapperFixtureHandlerCallsWrapper(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return fixtureRedactWrapper(1), nil
}

// wrapperFixtureHandlerLeaksDirectly is the P2 defect this whole mechanism
// exists to catch: a handler whose success path never calls the wrapper at
// all, only ever returning the raw value.
func wrapperFixtureHandlerLeaksDirectly(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return 1, nil
}

// wrapperFixtureHandlerCallsWrapperViaSelector calls the wrapper through a
// selector expression (pkg.Fn) rather than a bare identifier, proving
// handlerRedactsCredential's *ast.SelectorExpr branch, not only its
// *ast.Ident one. strings.ToUpper stands in for any qualified call; what
// matters is the call's Sel.Name, not what package it actually names.
func wrapperFixtureHandlerCallsWrapperViaSelector(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return strings.ToUpper("x"), nil
}

// TestUnit_HandlerRedactsCredential_HandlerCallsWrapper_ReportsTrue is the
// positive case: a real, compiled, package-level function whose body calls
// the named wrapper is recognised as doing so.
func TestUnit_HandlerRedactsCredential_HandlerCallsWrapper_ReportsTrue(t *testing.T) {
	t.Parallel()
	called, err := handlerRedactsCredential(wrapperFixtureHandlerCallsWrapper, "fixtureRedactWrapper")
	if err != nil {
		t.Fatalf("handlerRedactsCredential() error = %v", err)
	}
	if !called {
		t.Error("handlerRedactsCredential() = false, want true: wrapperFixtureHandlerCallsWrapper does call fixtureRedactWrapper")
	}
}

// TestUnit_HandlerRedactsCredential_HandlerLeaksDirectly_ReportsFalse is the
// mutation proof this task's brief asks for: wrapperFixtureHandlerLeaksDirectly
// is exactly wrapperFixtureHandlerCallsWrapper with the call to the
// redaction wrapper removed — the identical edit that shipped P2's original
// defect in registries' create/inspect/update handlers (a hand-written
// handler returning the API response directly instead of the redacted
// value). Before this task, nothing but a per-domain hand-written fixture
// test caught that edit; this proves the general mechanism now would too.
func TestUnit_HandlerRedactsCredential_HandlerLeaksDirectly_ReportsFalse(t *testing.T) {
	t.Parallel()
	called, err := handlerRedactsCredential(wrapperFixtureHandlerLeaksDirectly, "fixtureRedactWrapper")
	if err != nil {
		t.Fatalf("handlerRedactsCredential() error = %v", err)
	}
	if called {
		t.Error("handlerRedactsCredential() = true, want false: wrapperFixtureHandlerLeaksDirectly never calls fixtureRedactWrapper")
	}
}

// TestUnit_HandlerRedactsCredential_SelectorCall_IsRecognised proves the
// *ast.SelectorExpr branch: a call written as pkg.Fn(...), not a bare
// identifier, is still recognised when its selector name matches.
func TestUnit_HandlerRedactsCredential_SelectorCall_IsRecognised(t *testing.T) {
	t.Parallel()
	called, err := handlerRedactsCredential(wrapperFixtureHandlerCallsWrapperViaSelector, "ToUpper")
	if err != nil {
		t.Fatalf("handlerRedactsCredential() error = %v", err)
	}
	if !called {
		t.Error("handlerRedactsCredential() = false, want true: the handler calls strings.ToUpper, a selector call named ToUpper")
	}
}

// TestUnit_HandlerRedactsCredential_WrongWrapperName_ReportsFalse proves the
// match is by the wrapper's own name, not merely "some call happened": a
// handler that calls a real function, just not the one this operation
// requires, must not be mistaken for redacting.
func TestUnit_HandlerRedactsCredential_WrongWrapperName_ReportsFalse(t *testing.T) {
	t.Parallel()
	called, err := handlerRedactsCredential(wrapperFixtureHandlerCallsWrapper, "someOtherWrapperNameEntirely")
	if err != nil {
		t.Fatalf("handlerRedactsCredential() error = %v", err)
	}
	if called {
		t.Error("handlerRedactsCredential() = true, want false: the handler never calls someOtherWrapperNameEntirely")
	}
}

// TestUnit_VerifyCredentialCanary_RealMechanism_Passes proves the canary
// itself (the one run() calls before trusting anything
// auditCredentialRedaction reports) passes against the real
// handlerRedactsCredential — not a stand-in — the same discrimination
// verifyCanary already proves for specdiff.Compare.
func TestUnit_VerifyCredentialCanary_RealMechanism_Passes(t *testing.T) {
	t.Parallel()
	if err := verifyCredentialCanary(); err != nil {
		t.Fatalf("verifyCredentialCanary() error = %v, want nil", err)
	}
}
