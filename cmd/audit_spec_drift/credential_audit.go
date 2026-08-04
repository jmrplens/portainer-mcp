package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// credentialFinding is one catalog action whose success response — per the
// vendored specification — can carry a credential-shaped field, but whose
// declared Handler never calls the redaction wrapper that field requires.
// Unlike driftFinding, this never has an allow-list entry: there is no
// legitimate reason for a real credential to reach a model, so nothing here
// is ever excused, only fixed.
type credentialFinding struct {
	OperationID string
	ActionName  string
	// Fields are the credential-shaped field names found in the operation's
	// resolved success-response schema — e.g. "Password", "AccessToken".
	Fields []string
	// WrapperName is the function this action's domain package must declare
	// and call — redactionWrapperName(OperationID) — before this finding
	// clears.
	WrapperName string
}

// credentialAuditResult is the outcome of auditCredentialRedaction.
type credentialAuditResult struct {
	Findings []credentialFinding
	// ActionsChecked counts only actions whose response is credential-shaped
	// at all — the denominator a report needs to say "checked N, N-len(Findings)
	// pass" rather than implying every action in the catalog was a candidate.
	ActionsChecked int
}

// HasLeaks reports whether the build must fail: at least one credential-
// shaped response has no handler proven to redact it.
func (r *credentialAuditResult) HasLeaks() bool {
	return len(r.Findings) > 0
}

// auditCredentialRedaction is task-4b's generation-time credential refusal,
// moved here to survive the freeze (see credential.go's package doc comment
// for the full reasoning). For every declared action whose OperationID
// resolves to an operation whose vendored success response can carry a
// credential-shaped field, it demands the action's Handler statically calls
// redactionWrapperName(OperationID) somewhere in its own body — the same
// naming convention cmd/gen_action_inputs's checkCredentialRedaction already
// requires at scaffold time, checked again here so a later hand edit that
// quietly drops the call is caught too.
//
// Sorted (OperationID order) for the identical determinism reason auditDrift
// sorts: two runs over the same inputs must produce a byte-identical report.
func auditCredentialRedaction(eeOps, ceOps map[string]specOperation, actions []toolutil.ActionSpec) (*credentialAuditResult, error) {
	sorted := make([]toolutil.ActionSpec, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OperationID < sorted[j].OperationID })

	result := &credentialAuditResult{}
	for _, action := range sorted {
		specOp, _, ok := resolveSpecOperation(action.OperationID, eeOps, ceOps)
		if !ok {
			return nil, fmt.Errorf("action %q: OperationID %q resolves in neither vendored spec", action.Name, action.OperationID)
		}

		fields, err := responseCredentialFields(specOp.Responses, specOp.Op.Schemas)
		if err != nil {
			return nil, fmt.Errorf("action %q: credential-shaped response check: %w", action.Name, err)
		}
		if len(fields) == 0 {
			continue
		}
		result.ActionsChecked++

		wrapper := redactionWrapperName(action.OperationID)
		called, err := handlerRedactsCredential(action.Handler, wrapper)
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", action.Name, err)
		}
		if !called {
			result.Findings = append(result.Findings, credentialFinding{
				OperationID: action.OperationID,
				ActionName:  action.Name,
				Fields:      fields,
				WrapperName: wrapper,
			})
		}
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		return result.Findings[i].OperationID < result.Findings[j].OperationID
	})
	return result, nil
}

// # The canary is mandatory, exactly like verifyCanary's for specdiff.Compare
//
// This repository has already shipped seventeen checks that could not
// discriminate; the credential mechanism above is new with this task and has
// shipped zero times, which is exactly the state a canary exists for rather
// than after the fact. canaryRedactingHandler and canaryLeakingHandler below
// are real, compiled, package-level functions — not a mock or a synthetic
// AST fragment — so verifyCredentialCanary exercises handlerRedactsCredential
// against the identical mechanism (reflect.ValueOf(handler).Pointer(),
// runtime.FuncForPC, go/parser) a real run uses against real handlers,
// removing any doubt that the canary is testing a different code path than
// production.

// canaryRedactWrapper is the fixture wrapper verifyCredentialCanary looks
// for; its own behaviour (a pass-through) is irrelevant, since this canary
// checks only whether it was *called*, not what it does — that half is
// cmd/gen_action_inputs's generated redaction-guard test's job.
func canaryRedactWrapper(v any) any { return v }

// canaryRedactingHandler calls canaryRedactWrapper, matching a real
// generated handler such as registries' registryInspect.
func canaryRedactingHandler(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return canaryRedactWrapper(42), nil
}

// canaryLeakingHandler never calls canaryRedactWrapper, matching exactly
// the P2 defect this whole mechanism exists to catch: a handler that returns
// a credential-shaped response untouched.
func canaryLeakingHandler(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return 42, nil
}

// verifyCredentialCanary proves handlerRedactsCredential can still tell a
// handler that calls a named wrapper apart from one that does not, before
// this command trusts anything auditCredentialRedaction reports. It checks
// both directions, for the identical reason verifyCanary does: a detector
// that always reports "called" would pass the positive case while making
// every real leak invisible, and one that always reports "not called" would
// fail every real handler in the catalog, which would have been noticed
// immediately rather than shipped silently — but "immediately noticed" is
// exactly the property a canary exists to guarantee rather than assume.
func verifyCredentialCanary() error {
	called, err := handlerRedactsCredential(canaryRedactingHandler, "canaryRedactWrapper")
	if err != nil {
		return fmt.Errorf("credential canary self-test could not run: %w", err)
	}
	if !called {
		return fmt.Errorf(
			"credential canary self-test failed: handlerRedactsCredential reported false for canaryRedactingHandler, " +
				"which does call canaryRedactWrapper; the mechanism cannot be trusted this run")
	}

	called, err = handlerRedactsCredential(canaryLeakingHandler, "canaryRedactWrapper")
	if err != nil {
		return fmt.Errorf("credential canary self-test could not run: %w", err)
	}
	if called {
		return fmt.Errorf(
			"credential canary self-test failed: handlerRedactsCredential reported true for canaryLeakingHandler, " +
				"which never calls canaryRedactWrapper; a detector that cannot say \"no\" is as untrustworthy as one that cannot say \"yes\"")
	}
	return nil
}
