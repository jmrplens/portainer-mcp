package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// minimumFinding is one identifier-shaped path parameter (per the vendored
// specification's own parameter name and type) whose catalog action does not
// publish "minimum": 1 for it. Never allow-listed, for the identical reason
// credentialFinding is not: cmd/gen_action_inputs's four real carve-outs are
// already excluded structurally by pathParamMinimumExceptions, so anything
// this audit still reports is a real, un-excused gap, not a judgement call
// left to a human.
type minimumFinding struct {
	OperationID string
	ActionName  string
	ParamName   string
}

// minimumAuditResult is the outcome of auditIdentifierMinimum.
type minimumAuditResult struct {
	Findings []minimumFinding
	// ParamsChecked counts only path parameters the identifier-shape rule
	// actually applies to.
	ParamsChecked int
}

// HasGaps reports whether the build must fail: at least one identifier-shaped
// path parameter has no "minimum": 1 in the catalog.
func (r *minimumAuditResult) HasGaps() bool {
	return len(r.Findings) > 0
}

// auditIdentifierMinimum is cmd/gen_action_inputs's identifier-minimum
// guarantee, checked again here so a hand edit that quietly drops a
// MinimumParams entry (or the whole method) is caught — see minimum.go's
// package doc comment for why this cannot be specdiff.Compare's job.
//
// Sorted (OperationID order), mirroring auditDrift and
// auditCredentialRedaction, for the identical determinism reason: two runs
// over the same inputs must produce a byte-identical report.
func auditIdentifierMinimum(eeOps, ceOps map[string]specOperation, actions []toolutil.ActionSpec) (*minimumAuditResult, error) {
	sorted := make([]toolutil.ActionSpec, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].OperationID < sorted[j].OperationID })

	result := &minimumAuditResult{}
	for _, action := range sorted {
		specOp, _, ok := resolveSpecOperation(action.OperationID, eeOps, ceOps)
		if !ok {
			return nil, fmt.Errorf("action %q: OperationID %q resolves in neither vendored spec", action.Name, action.OperationID)
		}

		var pathParams []struct{ Name, Type string }
		for _, param := range specOp.Op.Parameters {
			in, _ := param["in"].(string)
			if in != "path" {
				continue
			}
			name, _ := param["name"].(string)
			schema, _ := param["schema"].(map[string]any)
			typ, _ := schema["type"].(string)
			pathParams = append(pathParams, struct{ Name, Type string }{name, typ})
		}

		if len(pathParams) == 0 {
			continue
		}

		schema, err := action.InputSchema()
		if err != nil {
			return nil, fmt.Errorf("action %q: %w", action.Name, err)
		}

		for _, p := range pathParams {
			if !pathParamRequiresMinimum(action.OperationID, p.Name, p.Type) {
				continue
			}
			result.ParamsChecked++

			minVal, present := catalogMinimumFor(schema, p.Name)
			if !present || minVal < 1 {
				result.Findings = append(result.Findings, minimumFinding{
					OperationID: action.OperationID,
					ActionName:  action.Name,
					ParamName:   p.Name,
				})
			}
		}
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].OperationID != result.Findings[j].OperationID {
			return result.Findings[i].OperationID < result.Findings[j].OperationID
		}
		return result.Findings[i].ParamName < result.Findings[j].ParamName
	})
	return result, nil
}

// # The canary, mirroring credential_audit.go's identical discipline
//
// canaryMinIDInput and canaryNoMinIDInput are real, compiled Input structs —
// one implementing toolutil.MinimumParams (matching a real generated
// identifier field) and one not (matching exactly what a hand edit that
// deletes a MinimumParams method leaves behind). verifyMinimumCanary proves
// auditIdentifierMinimum tells them apart before this command trusts
// anything it reports.

type canaryMinIDInput struct {
	ID int `json:"id"`
}

func (canaryMinIDInput) MinimumParams() map[string]int { return map[string]int{"id": 1} }

type canaryNoMinIDInput struct {
	ID int `json:"id"`
}

// canaryNoopHandler is a real, compiled toolutil.Handler used only to satisfy
// ActionSpec.Validate/InputSchema's plumbing; verifyMinimumCanary never calls
// it.
func canaryNoopHandler(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return nil, nil
}

// canaryMinimumSpecOps is a fabricated specOperation this canary invents and
// never reads from a vendored spec or the real catalog: one path parameter
// named "id", typed integer, which isIdentifierPathParam and
// pathParamRequiresMinimum both agree must carry "minimum": 1.
func canaryMinimumSpecOps() map[string]specOperation {
	return map[string]specOperation{
		"CanaryMinimumSelfTest": {
			Op: specdiff.SpecOperation{
				OperationID: "CanaryMinimumSelfTest",
				Parameters: []map[string]any{
					{"name": "id", "in": "path", "schema": map[string]any{"type": "integer"}},
				},
			},
		},
	}
}

// verifyMinimumCanary proves auditIdentifierMinimum can still tell an action
// that publishes "minimum": 1 on an identifier-shaped path parameter apart
// from one that does not, using two real compiled Input types rather than a
// synthetic report. Checked both directions for the identical reason every
// other canary in this command is: a detector that always says "missing"
// would pass the negative case while flagging every real action in the
// catalog, and one that always says "present" would hide a real gap.
func verifyMinimumCanary() error {
	ops := canaryMinimumSpecOps()

	present := []toolutil.ActionSpec{{
		Name: "canary.minimum_present", Domain: "canary", OperationID: "CanaryMinimumSelfTest",
		Title: "t", Description: "d", Edition: edition.CE,
		Handler: canaryNoopHandler, Input: canaryMinIDInput{},
	}}
	result, err := auditIdentifierMinimum(ops, map[string]specOperation{}, present)
	if err != nil {
		return fmt.Errorf("minimum canary self-test could not run: %w", err)
	}
	if result.HasGaps() {
		return fmt.Errorf(
			"minimum canary self-test failed: auditIdentifierMinimum reported a gap for canaryMinIDInput, "+
				"which does declare MinimumParams()[\"id\"]=1; the mechanism cannot be trusted this run: %+v", result.Findings)
	}

	absent := []toolutil.ActionSpec{{
		Name: "canary.minimum_absent", Domain: "canary", OperationID: "CanaryMinimumSelfTest",
		Title: "t", Description: "d", Edition: edition.CE,
		Handler: canaryNoopHandler, Input: canaryNoMinIDInput{},
	}}
	result, err = auditIdentifierMinimum(ops, map[string]specOperation{}, absent)
	if err != nil {
		return fmt.Errorf("minimum canary self-test could not run: %w", err)
	}
	if !result.HasGaps() {
		return fmt.Errorf(
			"minimum canary self-test failed: auditIdentifierMinimum reported no gap for canaryNoMinIDInput, " +
				"which declares no MinimumParams() at all; a detector that cannot say \"missing\" is as untrustworthy as one that cannot say \"present\"")
	}
	return nil
}
