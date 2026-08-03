package toolutil

import (
	"reflect"
	"testing"
)

func TestUnit_WithNarrative_ZeroNarrative_LeavesSpecUnchanged(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Title = "List tags"
	spec.Description = "Lists every tag defined on this Portainer instance."

	got := WithNarrative(spec, ActionNarrative{})
	if got.Title != spec.Title || got.Description != spec.Description {
		t.Errorf("Title/Description = %q/%q, want the mechanical values untouched by a zero narrative", got.Title, got.Description)
	}
	if got.Usage != "" || got.RelatedActions != nil || got.Aliases != nil || got.Tags != nil || got.ParameterGuidance != nil {
		t.Errorf("narrative-only fields = %+v, want every one at its zero value", got)
	}
}

func TestUnit_WithNarrative_NonEmptyTitleAndDescription_ReplaceRatherThanConcatenate(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Title = "List tags"
	spec.Description = "Lists every tag defined on this Portainer instance."

	got := WithNarrative(spec, ActionNarrative{Title: "Human title", Description: "Human description."})
	if got.Title != "Human title" {
		t.Errorf("Title = %q, want the narrative's override %q, with no trace of the mechanical value", got.Title, "Human title")
	}
	if got.Description != "Human description." {
		t.Errorf("Description = %q, want the narrative's override %q, with no trace of the mechanical value", got.Description, "Human description.")
	}
}

func TestUnit_WithNarrative_SuppliesUsageRelatedAliasesTagsAndParameterGuidance(t *testing.T) {
	t.Parallel()
	n := ActionNarrative{
		Usage:             "Prefer this over tags.list when you already know the id.",
		RelatedActions:    []string{"tags.list"},
		Aliases:           []string{"remove-tag"},
		Tags:              []string{"lifecycle"},
		ParameterGuidance: map[string]ParameterGuidance{"id": {SemanticRole: "the tag's numeric identifier"}},
	}
	got := WithNarrative(validSpec(), n)
	if got.Usage != n.Usage {
		t.Errorf("Usage = %q, want %q", got.Usage, n.Usage)
	}
	if !reflect.DeepEqual(got.RelatedActions, n.RelatedActions) {
		t.Errorf("RelatedActions = %v, want %v", got.RelatedActions, n.RelatedActions)
	}
	if !reflect.DeepEqual(got.Aliases, n.Aliases) {
		t.Errorf("Aliases = %v, want %v", got.Aliases, n.Aliases)
	}
	if !reflect.DeepEqual(got.Tags, n.Tags) {
		t.Errorf("Tags = %v, want %v", got.Tags, n.Tags)
	}
	if !reflect.DeepEqual(got.ParameterGuidance, n.ParameterGuidance) {
		t.Errorf("ParameterGuidance = %v, want %v", got.ParameterGuidance, n.ParameterGuidance)
	}
}

// TestUnit_WithNarrative_EmptyTitleOrDescriptionAlone_KeepsTheMechanicalOne
// proves the two fields are decided independently: a hook overriding only
// one of Title/Description must not blank out the other.
func TestUnit_WithNarrative_EmptyTitleOrDescriptionAlone_KeepsTheMechanicalOne(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Title = "List tags"
	spec.Description = "Lists every tag defined on this Portainer instance."

	got := WithNarrative(spec, ActionNarrative{Description: "Human description."})
	if got.Title != "List tags" {
		t.Errorf("Title = %q, want the mechanical value preserved when the narrative only overrides Description", got.Title)
	}
	if got.Description != "Human description." {
		t.Errorf("Description = %q, want the narrative's override", got.Description)
	}
}
