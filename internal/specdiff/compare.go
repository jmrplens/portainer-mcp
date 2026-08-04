package specdiff

import (
	"fmt"
	"slices"
	"sort"
)

// Compare reports every difference between before and after, one FieldChange
// per (field, kind) pair — never a single blanket "this operation changed".
// At a hundred-plus changed operations per minor Portainer release (see
// plan/research/version-delta-analysis.md), a report that cannot tell a
// reworded description from a swapped parameter type will not be read; this
// function is what lets a consumer group by consequence instead.
//
// A field present on only one side is ChangeAdded or ChangeRemoved and
// nothing else — its type, requiredness, enum, description and origin are
// not compared against an absent field. A field present on both sides is
// checked against every kind in turn, independently: a field can carry more
// than one FieldChange (a type swap that also picked up a reworded
// description reports both), because collapsing simultaneous, unrelated
// changes into one entry would hide one of them from a reader filtering by
// kind.
//
// Origin is compared only when both sides state one. ShapeFromCatalog cannot
// always know a field's origin (see FieldShape.Origin's doc comment), and an
// unstated origin is not evidence that the origin differs from a stated one
// — reporting ChangeOrigin every time a catalog-derived shape (Origin == "")
// meets a spec-derived one would manufacture a false positive for every
// single field of every single operation, which is worse than not checking
// origin at all.
//
// Title and Description are compared too, once each per call rather than
// once per field: they are operation-level facts (see OperationShape's doc
// comment), not something a caller supplies as a parameter. A model reads
// them to decide whether this action is the one it wants, so a hand-
// maintained Title or Description drifting from the specification it was
// generated from is exactly the same class of defect as a parameter's type
// silently widening — just one level up.
//
// The result is sorted by JSONName, then by Kind, so two calls over the same
// inputs — and two consumers calling this engine for drift and for delta —
// produce byte-identical output.
func Compare(before, after OperationShape) []FieldChange {
	beforeByName := indexFields(before.Fields)
	afterByName := indexFields(after.Fields)

	var changes []FieldChange
	for name, b := range beforeByName {
		a, ok := afterByName[name]
		if !ok {
			changes = append(changes, FieldChange{JSONName: name, Kind: ChangeRemoved, Before: describeField(b), After: ""})
			continue
		}
		changes = append(changes, compareField(name, b, a)...)
	}
	for name, a := range afterByName {
		if _, ok := beforeByName[name]; !ok {
			changes = append(changes, FieldChange{JSONName: name, Kind: ChangeAdded, Before: "", After: describeField(a)})
		}
	}

	// Title and Description are operation-level facts, not fields — see
	// OperationShape's doc comment — so they are compared once here, outside
	// the per-field loop above, using the sentinel JSONNames TitleSentinel
	// and DescriptionSentinel rather than any real field's wire name (see
	// those constants' own doc comment for why a collision there would be
	// dangerous, not merely cosmetic).
	if before.Title != after.Title {
		changes = append(changes, FieldChange{
			JSONName: TitleSentinel, Kind: ChangeTitle, Before: before.Title, After: after.Title,
			AfterOverridden: after.TitleOverridden,
		})
	}
	if before.Description != after.Description {
		changes = append(changes, FieldChange{
			JSONName: DescriptionSentinel, Kind: ChangeOperationDescription, Before: before.Description, After: after.Description,
			AfterOverridden: after.DescriptionOverridden,
		})
	}

	sort.Slice(changes, func(i, j int) bool {
		if changes[i].JSONName != changes[j].JSONName {
			return changes[i].JSONName < changes[j].JSONName
		}
		return changes[i].Kind < changes[j].Kind
	})
	return changes
}

// indexFields keys fields by JSONName. An operation shape is not expected to
// declare the same JSON name twice (cmd/gen_action_inputs's own
// assembleOperationFields refuses that at generation time), so the last
// occurrence winning on a hypothetical duplicate is not a correctness
// decision this function needs to defend — there is nothing in either
// producer that emits one.
func indexFields(fields []FieldShape) map[string]FieldShape {
	byName := make(map[string]FieldShape, len(fields))
	for _, f := range fields {
		byName[f.JSONName] = f
	}
	return byName
}

// compareField checks one field present on both sides against every kind of
// change this engine recognises, independently — see Compare's doc comment
// for why more than one FieldChange can come back for a single field.
func compareField(name string, before, after FieldShape) []FieldChange {
	var changes []FieldChange
	if before.Type != after.Type {
		changes = append(changes, FieldChange{JSONName: name, Kind: ChangeType, Before: before.Type, After: after.Type})
	}
	if before.Required != after.Required {
		changes = append(changes, FieldChange{JSONName: name, Kind: ChangeRequiredness, Before: fmt.Sprint(before.Required), After: fmt.Sprint(after.Required)})
	}
	if !enumsEqual(before.Enum, after.Enum) {
		changes = append(changes, FieldChange{JSONName: name, Kind: ChangeEnum, Before: fmt.Sprint(before.Enum), After: fmt.Sprint(after.Enum)})
	}
	if before.Description != after.Description {
		changes = append(changes, FieldChange{JSONName: name, Kind: ChangeDescription, Before: before.Description, After: after.Description})
	}
	if before.Origin != "" && after.Origin != "" && before.Origin != after.Origin {
		changes = append(changes, FieldChange{JSONName: name, Kind: ChangeOrigin, Before: before.Origin, After: after.Origin})
	}
	return changes
}

// enumsEqual compares two enum value sets, ignoring order: the vendored
// specification's own enum arrays have shown harmless reordering across
// versions elsewhere in this project's research (see
// plan/research/version-delta-analysis.md's enum de-duplication note), and
// what is model-facing is the set of allowed values, not the sequence they
// happen to be declared in.
func enumsEqual(before, after []string) bool {
	if len(before) != len(after) {
		return false
	}
	if len(before) == 0 {
		return true
	}
	b := slices.Clone(before)
	a := slices.Clone(after)
	sort.Strings(b)
	sort.Strings(a)
	return slices.Equal(b, a)
}

// describeField renders a field that exists on only one side, for
// ChangeAdded/ChangeRemoved's Before/After — compact enough to read inline in
// a work list, not a dump of every FieldShape attribute.
func describeField(f FieldShape) string {
	required := "optional"
	if f.Required {
		required = "required"
	}
	if f.Origin == "" {
		return fmt.Sprintf("%s (%s)", f.Type, required)
	}
	return fmt.Sprintf("%s (%s, %s)", f.Type, required, f.Origin)
}
