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

// TestUnit_HandlerRedactsCredential_RecognisesACallByNameNotByAccident
// covers handlerRedactsCredential's four discriminating cases in one table:
// each case names the exact real-world shape it stands in for, and each
// asserts a different value of "called" or "wrapperName" against the
// identical fixture handlers, so no single case could pass by accident.
func TestUnit_HandlerRedactsCredential_RecognisesACallByNameNotByAccident(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		handler    func(context.Context, *portainer.Client, json.RawMessage) (any, error)
		wrapper    string
		wantCalled bool
	}{
		{
			// The positive case: a real, compiled, package-level function
			// whose body calls the named wrapper is recognised as doing so.
			name: "handler calls the named wrapper", handler: wrapperFixtureHandlerCallsWrapper,
			wrapper: "fixtureRedactWrapper", wantCalled: true,
		},
		{
			// The mutation proof this task's brief asks for:
			// wrapperFixtureHandlerLeaksDirectly is exactly
			// wrapperFixtureHandlerCallsWrapper with the call to the
			// redaction wrapper removed — the identical edit that shipped
			// P2's original defect in registries' create/inspect/update
			// handlers (a hand-written handler returning the API response
			// directly instead of the redacted value). Before this task,
			// nothing but a per-domain hand-written fixture test caught
			// that edit; this proves the general mechanism now would too.
			name: "handler never calls any wrapper", handler: wrapperFixtureHandlerLeaksDirectly,
			wrapper: "fixtureRedactWrapper", wantCalled: false,
		},
		{
			// Proves the *ast.SelectorExpr branch: a call written as
			// pkg.Fn(...), not a bare identifier, is still recognised when
			// its selector name matches.
			name: "handler calls the wrapper via a selector expression", handler: wrapperFixtureHandlerCallsWrapperViaSelector,
			wrapper: "ToUpper", wantCalled: true,
		},
		{
			// Proves the match is by the wrapper's own name, not merely
			// "some call happened": a handler that calls a real function,
			// just not the one this operation requires, must not be
			// mistaken for redacting.
			name: "handler calls a real function, but not the required wrapper", handler: wrapperFixtureHandlerCallsWrapper,
			wrapper: "someOtherWrapperNameEntirely", wantCalled: false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called, err := handlerRedactsCredential(tc.handler, tc.wrapper)
			if err != nil {
				t.Fatalf("handlerRedactsCredential() error = %v", err)
			}
			if called != tc.wantCalled {
				t.Errorf("handlerRedactsCredential() = %v, want %v", called, tc.wantCalled)
			}
		})
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
