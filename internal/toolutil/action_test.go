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
	cases := map[string]func(*ActionSpec){
		"Name":        func(s *ActionSpec) { s.Name = "" },
		"Domain":      func(s *ActionSpec) { s.Domain = "" },
		"OperationID": func(s *ActionSpec) { s.OperationID = "" },
		"Title":       func(s *ActionSpec) { s.Title = "" },
		"Description": func(s *ActionSpec) { s.Description = "" },
		"Edition":     func(s *ActionSpec) { s.Edition = "" },
		"Handler":     func(s *ActionSpec) { s.Handler = nil },
	}
	for field, corruptSpec := range cases {
		spec := validSpec()
		corruptSpec(&spec)
		err := spec.Validate()
		if err == nil {
			t.Errorf("Validate() with empty %s = nil, want an error", field)
			continue
		}
		if !strings.Contains(err.Error(), spec.Name+": ") && spec.Name != "" {
			t.Errorf("Validate() error for %s = %q, want it to name the action", field, err)
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
