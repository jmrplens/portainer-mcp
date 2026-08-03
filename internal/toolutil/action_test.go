package toolutil

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

func noopHandler(context.Context, *portainer.Client, json.RawMessage) (any, error) {
	return nil, nil
}

func validSpec() ActionSpec {
	return ActionSpec{
		Name:        "tags.list",
		Domain:      "tags",
		OperationID: "TagList",
		Title:       "List tags",
		Description: "Lists every tag defined on this Portainer instance.",
		Edition:     edition.CE,
		Handler:     noopHandler,
	}
}

func TestValidate_CompleteSpec_ReturnsNil(t *testing.T) {
	t.Parallel()
	if err := validSpec().Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestValidate_MissingFields_AreEachReported(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		corruptSpec func(*ActionSpec)
		wantIn      string
	}{
		"Name":        {func(s *ActionSpec) { s.Name = "" }, "Name is required"},
		"Domain":      {func(s *ActionSpec) { s.Domain = "" }, "Domain is required"},
		"OperationID": {func(s *ActionSpec) { s.OperationID = "" }, "OperationID is required"},
		"Title":       {func(s *ActionSpec) { s.Title = "" }, "Title is required"},
		"Description": {func(s *ActionSpec) { s.Description = "" }, "Description is required"},
		"Edition":     {func(s *ActionSpec) { s.Edition = "" }, "is not CE or EE"},
		"Handler":     {func(s *ActionSpec) { s.Handler = nil }, "Handler is required"},
	}
	for field, tc := range cases {
		spec := validSpec()
		tc.corruptSpec(&spec)

		err := spec.Validate()
		if err == nil {
			t.Errorf("Validate() with empty %s = nil, want an error", field)
			continue
		}
		// Asserting the specific message is what makes each case discriminate:
		// several guards would otherwise be satisfied by an unrelated one
		// firing as a side effect.
		if !strings.Contains(err.Error(), tc.wantIn) {
			t.Errorf("Validate() with empty %s = %q, want it to contain %q", field, err, tc.wantIn)
		}
	}
}

// A destructive action that is not mutating is a contradiction, and the one
// that matters: safe mode and read-only both key off Mutating, so a spec
// marked destructive but not mutating would slip past both.
func TestValidate_DestructiveButNotMutating_ReturnsError(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Destructive = true
	spec.Mutating = false
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error: destructive implies mutating")
	}
}

func TestValidate_UnknownEdition_ReturnsError(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Edition = "business"
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for an unknown edition")
	}
}

// The name must be domain-qualified so a catalog spanning 46 domains cannot
// collide, and so a model reading an action id can tell where it lives.
func TestValidate_NameNotDomainQualified_ReturnsError(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Name = "list"
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a name that is not domain-qualified")
	}
}

func TestValidate_NameWithEmptyActionPart_ReturnsError(t *testing.T) {
	t.Parallel()
	spec := validSpec()
	spec.Name = "tags."
	if err := spec.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a name with no action part after the dot")
	}
}

func TestValidate_NameWithIllegalCharacter_ReturnsError(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tags.list one", "tags.list/all", "tags.list:x"} {
		spec := validSpec()
		spec.Name = name
		if err := spec.Validate(); err == nil {
			t.Errorf("Validate() with name %q = nil, want an error", name)
		}
	}
}

func TestValidate_InputMustBeAStruct(t *testing.T) {
	t.Parallel()
	spec := ActionSpec{
		Name: "tags.list", Domain: "tags", OperationID: "TagList",
		Title: "T", Description: "D", Edition: edition.CE, Input: "not a struct",
		Handler: func(_ context.Context, _ *portainer.Client, _ json.RawMessage) (any, error) {
			return nil, nil
		},
	}
	if err := spec.Validate(); err == nil {
		t.Error("Validate() error = nil, want a refusal for a non-struct Input")
	}
}
