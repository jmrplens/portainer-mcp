package main

import "strings"

// This file is the second of the two generator-time structural guarantees
// task-4's freeze must not lose (see credential.go's package doc comment for
// the first, credential redaction). cmd/gen_action_inputs stamps a JSON
// Schema "minimum": 1 onto every integer path parameter shaped like a
// Portainer resource identifier (isIdentifierPathParam below, minus the four
// carve-outs in pathParamMinimumExceptions) — its own addition, never the
// vendored specification's, which never declares "minimum" on a single one
// of the 366 path parameters in the Business Edition document today (checked
// directly). tools.Execute validates every action's published schema before
// any handler runs, so this is what refuses a non-positive identifier before
// a network call, in one place, rather than a hand-written guard clause
// repeated in every handler that takes one.
//
// # Why this cannot be specdiff.Compare's job
//
// specdiff.Compare finds disagreement between a spec-derived shape and a
// catalog-derived one. This guarantee has no spec-derived side to compare
// against: the vendored specification never states a minimum on a path
// parameter at all, so a catalog action that has never had one and a catalog
// action a hand edit just stripped it from look identical to a before/after
// diff against the spec — both simply have no minimum where the spec also
// has none. What must be checked instead is a fact about the catalog alone,
// measured against the spec only for which fields the identifier-shape rule
// applies to. That is exactly the shape credential.go's check took too (see
// its own doc comment on why it lives outside specdiff), and this follows
// the identical pattern rather than forcing an asymmetric fact through a
// symmetric comparison engine.
//
// # Why the exceptions table is reimplemented, not imported
//
// cmd/gen_action_inputs is `package main`; nothing outside it can import its
// pathParamMinimumExceptions. The table is small (four entries, unchanged
// since it was introduced) and each entry's own reasoning is specific to one
// (operationId, parameter) pair — see cmd/gen_action_inputs/fields.go's
// identical table for the full justification of each. A future entry added
// there without a matching entry here would make this audit gate on a
// parameter the generator itself already knows must not carry the bound; the
// mirrored table below is what a reviewer adding one must remember to update
// in both places, exactly as scaffold-time and audit-time already must agree
// on redactionWrapperName's naming convention.

// isIdentifierPathParam reports whether name is shaped like a Portainer
// resource identifier, mirroring cmd/gen_action_inputs's identical function
// exactly: a case-insensitive "id" suffix.
func isIdentifierPathParam(name string) bool {
	return len(name) >= 2 && strings.EqualFold(name[len(name)-2:], "id")
}

// pathParamKey identifies one path parameter of one operation, mirroring
// cmd/gen_action_inputs's identical type.
type pathParamKey struct {
	OperationID string
	ParamName   string
}

// pathParamMinimumExceptions mirrors cmd/gen_action_inputs's identical table:
// the (operationId, path parameter) pairs where isIdentifierPathParam's name
// rule matches but a positive lower bound would refuse a value the real
// server accepts. None of the four names a domain this catalog declares
// today (system, tags, registries) — see cmd/gen_action_inputs/fields.go's
// own doc comment for each entry's reasoning. It stops being inert with the
// docker domain, which declares three of the operations it excuses.
//
// It is a mirror, and mirrors drift: this table carried four of the
// generator's five entries until the docker wave was about to land, and the
// missing one ({ServiceImageStatus, serviceId}) would have failed CI with a
// finding that cannot be allow-listed. TestUnit_PathParamMinimumExceptions_MirrorsTheGeneratorsOwnTable
// now compares the two key sets in both directions, so neither table can
// gain an entry the other lacks.
var pathParamMinimumExceptions = map[pathParamKey]bool{
	{OperationID: "EndpointDockerhubStatus", ParamName: "registryId"}:     true,
	{OperationID: "DockerContainerGpusInspect", ParamName: "containerId"}: true,
	{OperationID: "ContainerImageStatus", ParamName: "containerId"}:       true,
	{OperationID: "SnapshotContainerInspect", ParamName: "containerId"}:   true,
	{OperationID: "ServiceImageStatus", ParamName: "serviceId"}:           true,
}

// pathParamRequiresMinimum decides whether the catalog action generated (or
// hand-written) for operationID must publish "minimum": 1 on its paramName
// path parameter: an integer type, a name shaped like an identifier, and no
// recorded carve-out.
func pathParamRequiresMinimum(operationID, paramName, paramType string) bool {
	if paramType != "integer" {
		return false
	}
	if !isIdentifierPathParam(paramName) {
		return false
	}
	return !pathParamMinimumExceptions[pathParamKey{OperationID: operationID, ParamName: paramName}]
}

// catalogMinimumFor returns the "minimum" JSON Schema keyword schema declares
// for paramName, and whether it is present at all — schema is the map
// toolutil.ActionSpec.InputSchema() returns, where applyMinimumParams writes
// a native Go int (never a json.Unmarshal-decoded float64: the value is set
// directly in Go, after the schema's own encode/decode round-trip already
// happened — see toolutil/schema.go's InputSchema).
func catalogMinimumFor(schema map[string]any, paramName string) (int, bool) {
	props, _ := schema["properties"].(map[string]any)
	propSchema, ok := props[paramName].(map[string]any)
	if !ok {
		return 0, false
	}
	switch v := propSchema["minimum"].(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}
