package main

import (
	"strings"
	"testing"
)

// This file covers docs/api-divergences.md §6.6: the vendored Business Edition
// document declares portaineree.CustomTemplateRelativePathSettings's
// PerDeviceConfigsMatchType (and its Group sibling) with an inline
// ["file", " dir"] beside an allOf $ref to portainer.PerDevConfigsFilterType,
// whose own enum is the clean ["file", "dir"]. resolve deliberately lets the
// sibling keyword win, so without normaliseEnumValues the malformed spelling
// is the one baked into the generated EnumParams() methods and published to
// models as the only value the catalog admits.
//
// The trim is general, not a carve-out for that one schema, so the cases that
// are not a plain trim are pinned here too: an empty string, which is a real
// enum member in both vendored documents, must survive untouched, while a
// value that is non-empty and trims to nothing must be refused rather than
// silently turned into that same empty string.

// TestUnit_Resolve_EnumValueWithSurroundingWhitespace_IsTrimmed is the
// mechanism in isolation, on schemas small enough to read, including the exact
// allOf-plus-sibling shape §6.6 records.
func TestUnit_Resolve_EnumValueWithSurroundingWhitespace_IsTrimmed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		raw  map[string]any
		want []any
	}{
		{
			name: "leading space on a plain enum",
			raw:  map[string]any{"type": "string", "enum": []any{"file", " dir"}},
			want: []any{"file", "dir"},
		},
		{
			name: "trailing space",
			raw:  map[string]any{"type": "string", "enum": []any{"file ", "dir"}},
			want: []any{"file", "dir"},
		},
		{
			name: "tabs and newlines count as whitespace",
			raw:  map[string]any{"type": "string", "enum": []any{"\tfile", "dir\n"}},
			want: []any{"file", "dir"},
		},
		{
			name: "the §6.6 shape: a malformed sibling enum overlaying a clean $ref",
			raw: map[string]any{
				"allOf":       []any{map[string]any{"$ref": "#/components/schemas/PerDevConfigsFilterType"}},
				"description": "Per device configs match type",
				"enum":        []any{"file", " dir"},
			},
			want: []any{"file", "dir"},
		},
		{
			name: "an empty string is a real value and survives",
			raw:  map[string]any{"type": "string", "enum": []any{"processing", "warning", "error", ""}},
			want: []any{"processing", "warning", "error", ""},
		},
		{
			name: "non-string values are untouched",
			raw:  map[string]any{"type": "integer", "enum": []any{1, 2, 3}},
			want: []any{1, 2, 3},
		},
	}
	res := &resolver{doc: newDoc(map[string]any{
		"PerDevConfigsFilterType": map[string]any{"type": "string", "enum": []any{"file", "dir"}},
	})}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := res.resolve(tt.raw, 0)
			if err != nil {
				t.Fatalf("resolve() error = %v, want nil", err)
			}
			assertEnum(t, node.Enum, tt.want)
		})
	}
}

// TestUnit_Resolve_WhitespaceOnlyEnumValue_IsRefused pins the deliberate
// refusal. Emitting "" for a value the document spells " " would invent the
// one string that already means something else in this very specification
// (portaineree.EndpointOperationStatus's "done" member is literally ""), so
// the operation carrying it is refused instead.
func TestUnit_Resolve_WhitespaceOnlyEnumValue_IsRefused(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value any
	}{
		{name: "a single space", value: " "},
		{name: "a tab", value: "\t"},
		{name: "several spaces", value: "   "},
	}
	res := &resolver{doc: newDoc(nil)}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := res.resolve(map[string]any{"type": "string", "enum": []any{"file", tt.value}}, 0)
			if err == nil {
				t.Fatalf("resolve() error = nil, want a refusal: %q has no value left once trimmed", tt.value)
			}
			if !strings.Contains(err.Error(), "only whitespace") {
				t.Errorf("resolve() error = %q, want it to name the defect (\"only whitespace\")", err)
			}
		})
	}
}

// TestUnit_Resolve_EnumValuesCollidingAfterTrim_AreDeduplicated pins the other
// deliberate outcome: the trim can make two entries identical, and publishing
// the same value twice is the defect cmd/fetch_spec's deduplicateEnums already
// repairs for values that were exact duplicates as published.
func TestUnit_Resolve_EnumValuesCollidingAfterTrim_AreDeduplicated(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		enum []any
		want []any
	}{
		{
			name: "the trimmed value collides with a later clean one",
			enum: []any{"file", " dir", "dir"},
			want: []any{"file", "dir"},
		},
		{
			name: "first occurrence order is preserved",
			enum: []any{" dir", "file", "dir "},
			want: []any{"dir", "file"},
		},
	}
	res := &resolver{doc: newDoc(nil)}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := res.resolve(map[string]any{"type": "string", "enum": tt.enum}, 0)
			if err != nil {
				t.Fatalf("resolve() error = %v, want nil", err)
			}
			assertEnum(t, node.Enum, tt.want)
		})
	}
}

// TestUnit_AssembleOperationFields_CustomTemplateRelativePathSettings_PublishesCleanDir
// runs the real generation path over the real vendored document, because a
// synthetic schema proves the mechanism and only the document proves the
// mechanism matches the shape the document actually has. It asserts on the
// rendered source, which is what a domain's inputs.go is written from.
func TestUnit_AssembleOperationFields_CustomTemplateRelativePathSettings_PublishesCleanDir(t *testing.T) {
	t.Parallel()
	for _, operationID := range []string{"CustomTemplateCreateRepository", "CustomTemplateCreateString", "CustomTemplateUpdate"} {
		t.Run(operationID, func(t *testing.T) {
			t.Parallel()
			op, doc, res := realOperation(t, operationID)
			structName := inputStructName(op.OperationID)
			var nested []structSpec
			fields, _, err := assembleOperationFields(op, res, doc, structName, &nested)
			if err != nil {
				t.Fatalf("assembleOperationFields(%s) error = %v", operationID, err)
			}
			src, err := renderFile("custom_templates", "ee-2.44.0.json", append(nested, structSpec{Name: structName, Fields: fields}))
			if err != nil {
				t.Fatalf("renderFile() error = %v", err)
			}
			text := string(src)
			if strings.Contains(text, `" dir"`) {
				t.Errorf("generated source still carries the vendored document's malformed %q enum value:\n%s", " dir", text)
			}
			for _, want := range []string{
				`"perDeviceConfigsMatchType":      {"file", "dir"}`,
				`"perDeviceConfigsGroupMatchType": {"file", "dir"}`,
			} {
				if !strings.Contains(text, want) {
					t.Errorf("generated source does not declare %s:\n%s", want, text)
				}
			}
		})
	}
}

func assertEnum(t *testing.T, got, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("enum = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("enum[%d] = %#v, want %#v (full: %#v)", i, got[i], want[i], got)
		}
	}
}
