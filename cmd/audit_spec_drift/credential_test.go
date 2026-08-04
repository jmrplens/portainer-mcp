package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/specdiff"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/internal/wiring"
)

func jsonSchemaResponses(t *testing.T, schemaJSON string) map[string]map[string]any {
	t.Helper()
	var schema map[string]any
	if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
		t.Fatalf("decode fixture schema: %v", err)
	}
	return map[string]map[string]any{
		"200": {
			"content": map[string]any{
				"application/json": map[string]any{
					"schema": schema,
				},
			},
		},
	}
}

// TestUnit_ResponseCredentialFields_DirectProperty is the ordinary case: a
// credential-shaped property named directly in the response schema.
func TestUnit_ResponseCredentialFields_DirectProperty(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields DirectProperty", func(t *testing.T) {
		responses := jsonSchemaResponses(t, `{
			"type": "object",
			"properties": {
				"Id": {"type": "integer"},
				"Password": {"type": "string"}
			}
		}`)
		fields, err := responseCredentialFields(responses, nil)
		if err != nil {
			t.Fatalf("responseCredentialFields() error = %v", err)
		}
		if len(fields) != 1 || fields[0] != "Password" {
			t.Errorf("responseCredentialFields() = %v, want [Password]", fields)
		}
	})
}

// TestUnit_ResponseCredentialFields_NestedViaRef proves resolution follows a
// $ref down into a nested object, mirroring how PortainereeRegistry's
// ManagementConfiguration (itself a $ref) carries Password/AccessToken one
// level below the top-level response schema in the real vendored spec.
func TestUnit_ResponseCredentialFields_NestedViaRef(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields NestedViaRef", func(t *testing.T) {
		schemas := map[string]any{
			"Inner": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"AccessToken": map[string]any{"type": "string"},
				},
			},
		}
		responses := jsonSchemaResponses(t, `{
			"type": "object",
			"properties": {
				"Config": {"$ref": "#/components/schemas/Inner"}
			}
		}`)
		fields, err := responseCredentialFields(responses, schemas)
		if err != nil {
			t.Fatalf("responseCredentialFields() error = %v", err)
		}
		if len(fields) != 1 || fields[0] != "AccessToken" {
			t.Errorf("responseCredentialFields() = %v, want [AccessToken]", fields)
		}
	})
}

// TestUnit_ResponseCredentialFields_ArrayItems proves a list response (the
// real RegistryList shape: an array of registry objects) is walked into its
// items, not skipped because the top level is an array rather than an
// object.
func TestUnit_ResponseCredentialFields_ArrayItems(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields ArrayItems", func(t *testing.T) {
		responses := jsonSchemaResponses(t, `{
			"type": "array",
			"items": {
				"type": "object",
				"properties": {"Secret": {"type": "string"}}
			}
		}`)
		fields, err := responseCredentialFields(responses, nil)
		if err != nil {
			t.Fatalf("responseCredentialFields() error = %v", err)
		}
		if len(fields) != 1 || fields[0] != "Secret" {
			t.Errorf("responseCredentialFields() = %v, want [Secret]", fields)
		}
	})
}

// TestUnit_ResponseCredentialFields_NoJSONBody_ReturnsNil covers a 204 (or a
// 200 with no content) exactly like RegistryConfigure's real response in the
// vendored spec: nothing to walk, so nothing is reported, rather than an
// error about a missing schema.
func TestUnit_ResponseCredentialFields_NoJSONBody_ReturnsNil(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields NoJSONBody ReturnsNil", func(t *testing.T) {
		responses := map[string]map[string]any{"204": {"description": "Success"}}
		fields, err := responseCredentialFields(responses, nil)
		if err != nil {
			t.Fatalf("responseCredentialFields() error = %v", err)
		}
		if fields != nil {
			t.Errorf("responseCredentialFields() = %v, want nil", fields)
		}
	})
}

// TestUnit_ResponseCredentialFields_NoCredentialShapedField_ReturnsNil is
// the negative case: an ordinary response with no credential-shaped name
// anywhere reports nothing.
func TestUnit_ResponseCredentialFields_NoCredentialShapedField_ReturnsNil(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields NoCredentialShapedField ReturnsNil", func(t *testing.T) {
		responses := jsonSchemaResponses(t, `{
			"type": "object",
			"properties": {"Id": {"type": "integer"}, "Name": {"type": "string"}}
		}`)
		fields, err := responseCredentialFields(responses, nil)
		if err != nil {
			t.Fatalf("responseCredentialFields() error = %v", err)
		}
		if fields != nil {
			t.Errorf("responseCredentialFields() = %v, want nil", fields)
		}
	})
}

// TestUnit_ResponseCredentialFields_OneOf_ReturnsError mirrors
// cmd/gen_action_inputs's identical refusal: a union type does not compose
// into a single shape to walk, so this refuses rather than silently picking
// a branch and possibly missing a credential in the branch it did not pick.
func TestUnit_ResponseCredentialFields_OneOf_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields OneOf ReturnsError", func(t *testing.T) {
		responses := jsonSchemaResponses(t, `{"oneOf": [{"type": "object"}, {"type": "string"}]}`)
		if _, err := responseCredentialFields(responses, nil); err == nil {
			t.Fatal("responseCredentialFields() error = nil, want an error for a oneOf schema")
		}
	})
}

// TestUnit_ResponseCredentialFields_UnresolvedRef_ReturnsError proves a $ref
// this audit cannot resolve is a loud failure, not a silent "no credential
// found".
func TestUnit_ResponseCredentialFields_UnresolvedRef_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("ResponseCredentialFields UnresolvedRef ReturnsError", func(t *testing.T) {
		responses := jsonSchemaResponses(t, `{"$ref": "#/components/schemas/DoesNotExist"}`)
		if _, err := responseCredentialFields(responses, map[string]any{}); err == nil {
			t.Fatal("responseCredentialFields() error = nil, want an error for an unresolved $ref")
		}
	})
}

// redactFixtureOp is the exact name redactionWrapperName("FixtureOp")
// derives, so fixtureCredentialHandlerRedacts below is recognised as
// redacting FixtureOp's response.
func redactFixtureOp(v any) any { return v }

func fixtureCredentialHandlerRedacts(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return redactFixtureOp(map[string]any{"Password": "x"}), nil
}

// fixtureCredentialHandlerLeaks is fixtureCredentialHandlerRedacts with the
// call to redactFixtureOp removed — the exact mutation that shipped P2's
// original defect (registries' create/inspect/update handlers returning the
// API response directly).
func fixtureCredentialHandlerLeaks(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
	return map[string]any{"Password": "x"}, nil
}

func fixtureCredentialSpecOps(t *testing.T, credentialShaped bool) map[string]specOperation {
	t.Helper()
	schema := `{"type": "object", "properties": {"Id": {"type": "integer"}}}`
	if credentialShaped {
		schema = `{"type": "object", "properties": {"Id": {"type": "integer"}, "Password": {"type": "string"}}}`
	}
	responses := jsonSchemaResponses(t, schema)
	return map[string]specOperation{
		"FixtureOp": {
			Op:        specdiff.SpecOperation{OperationID: "FixtureOp"},
			Responses: responses,
		},
	}
}

// TestUnit_AuditCredentialRedaction_CredentialShapedAndHandlerRedacts_NoFinding
// is the clean case: the operation's response can carry a credential, and the
// action's Handler does call the required wrapper (redact + OperationID).
func TestUnit_AuditCredentialRedaction_CredentialShapedAndHandlerRedacts_NoFinding(t *testing.T) {
	t.Parallel()
	t.Run("AuditCredentialRedaction CredentialShapedAndHandlerRedacts NoFinding", func(t *testing.T) {
		ops := fixtureCredentialSpecOps(t, true)
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "FixtureOp",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: fixtureCredentialHandlerRedacts,
		}}
		result, err := auditCredentialRedaction(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditCredentialRedaction() error = %v", err)
		}
		if result.HasLeaks() {
			t.Errorf("auditCredentialRedaction() findings = %v, want none: the handler calls redactFixtureOp", result.Findings)
		}
		if result.ActionsChecked != 1 {
			t.Errorf("auditCredentialRedaction() ActionsChecked = %d, want 1", result.ActionsChecked)
		}
	})
}

// TestUnit_AuditCredentialRedaction_CredentialShapedAndHandlerLeaks_Finding is
// this task's mutation proof at the audit level: the same operation, but the
// Handler never calls any wrapper at all — the P2 defect — must be reported,
// unconditionally (never allow-listable).
func TestUnit_AuditCredentialRedaction_CredentialShapedAndHandlerLeaks_Finding(t *testing.T) {
	t.Parallel()
	t.Run("AuditCredentialRedaction CredentialShapedAndHandlerLeaks Finding", func(t *testing.T) {
		ops := fixtureCredentialSpecOps(t, true)
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "FixtureOp",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: fixtureCredentialHandlerLeaks,
		}}
		result, err := auditCredentialRedaction(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditCredentialRedaction() error = %v", err)
		}
		if !result.HasLeaks() {
			t.Fatal("auditCredentialRedaction() reported no leaks, want one: the handler never calls redactFixtureOp")
		}
		if len(result.Findings) != 1 || result.Findings[0].OperationID != "FixtureOp" {
			t.Errorf("auditCredentialRedaction() findings = %+v, want one finding naming FixtureOp", result.Findings)
		}
		if result.Findings[0].WrapperName != "redactFixtureOp" {
			t.Errorf("auditCredentialRedaction() WrapperName = %q, want %q", result.Findings[0].WrapperName, "redactFixtureOp")
		}
	})
}

// TestUnit_AuditCredentialRedaction_NotCredentialShaped_NoCheckPerformed
// proves an operation whose response carries nothing credential-shaped is
// never even asked whether its handler redacts — the overwhelming majority
// of real actions, and each one must cost this audit nothing.
func TestUnit_AuditCredentialRedaction_NotCredentialShaped_NoCheckPerformed(t *testing.T) {
	t.Parallel()
	t.Run("AuditCredentialRedaction NotCredentialShaped NoCheckPerformed", func(t *testing.T) {
		ops := fixtureCredentialSpecOps(t, false)
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "FixtureOp",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: fixtureCredentialHandlerLeaks,
		}}
		result, err := auditCredentialRedaction(ops, map[string]specOperation{}, actions)
		if err != nil {
			t.Fatalf("auditCredentialRedaction() error = %v", err)
		}
		if result.HasLeaks() {
			t.Errorf("auditCredentialRedaction() findings = %v, want none: this operation's response carries no credential-shaped field", result.Findings)
		}
		if result.ActionsChecked != 0 {
			t.Errorf("auditCredentialRedaction() ActionsChecked = %d, want 0", result.ActionsChecked)
		}
	})
}

// TestUnit_AuditCredentialRedaction_UnresolvedOperationID_ReturnsError
// mirrors auditDrift's identical refusal: an action naming an OperationID
// neither vendored spec declares is a fatal input error, not a finding.
func TestUnit_AuditCredentialRedaction_UnresolvedOperationID_ReturnsError(t *testing.T) {
	t.Parallel()
	t.Run("AuditCredentialRedaction UnresolvedOperationID ReturnsError", func(t *testing.T) {
		actions := []toolutil.ActionSpec{{
			Name: "fixture.op", Domain: "fixture", OperationID: "NoSuchOperation",
			Title: "t", Description: "d", Edition: edition.CE,
			Handler: fixtureCredentialHandlerLeaks,
		}}
		if _, err := auditCredentialRedaction(map[string]specOperation{}, map[string]specOperation{}, actions); err == nil {
			t.Fatal("auditCredentialRedaction() error = nil, want an error for an unresolved OperationID")
		}
	})
}

// TestUnit_RealCatalog_EveryCredentialShapedAction_HandlerRedacts is the
// integration proof against production code, not a fixture: every action
// wiring.AllSpecs() declares today whose vendored response is
// credential-shaped must have a Handler that calls its wrapper. This is
// exactly what TestUnit_Run_RealCatalogAgainstRealSpecs_ReturnsNil in
// main_test.go already covers end to end (run() succeeds); this test isolates
// just the credential half so a future failure here points straight at
// credential redaction rather than requiring a reader to first rule out
// ordinary parameter drift.
func TestUnit_RealCatalog_EveryCredentialShapedAction_HandlerRedacts(t *testing.T) {
	t.Parallel()
	t.Run("RealCatalog EveryCredentialShapedAction HandlerRedacts", func(t *testing.T) {
		ceData, err := readFileIn(realSpecsDir, "ce-2.44.0.json")
		if err != nil {
			t.Fatalf("read real ce spec: %v", err)
		}
		eeData, err := readFileIn(realSpecsDir, "ee-2.44.0.json")
		if err != nil {
			t.Fatalf("read real ee spec: %v", err)
		}
		ceOps, err := parseSpecOperations(ceData)
		if err != nil {
			t.Fatalf("parseSpecOperations(ce) error = %v", err)
		}
		eeOps, err := parseSpecOperations(eeData)
		if err != nil {
			t.Fatalf("parseSpecOperations(ee) error = %v", err)
		}

		result, err := auditCredentialRedaction(eeOps, ceOps, wiring.AllSpecs())
		if err != nil {
			t.Fatalf("auditCredentialRedaction() error = %v", err)
		}
		if result.ActionsChecked == 0 {
			t.Fatal("auditCredentialRedaction() ActionsChecked = 0, want at least registries.list/create/inspect/update to be credential-shaped")
		}
		if result.HasLeaks() {
			t.Errorf("auditCredentialRedaction() found %d real, unresolved credential-redaction finding(s): %+v",
				len(result.Findings), result.Findings)
		}
	})
}
