package toolutil

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// Handler executes one action. Input arrives as raw JSON so a single signature
// serves every action regardless of its parameters; each handler unmarshals
// into its own typed struct.
type Handler func(ctx context.Context, c *portainer.Client, input json.RawMessage) (any, error)

// ActionSpec is the canonical declaration of one Portainer action.
//
// An action is declared exactly once, here, in its domain package. Every tool
// surface is a projection of these declarations — never a parallel
// registration — which is what keeps the surfaces from drifting apart.
type ActionSpec struct {
	// Name is the canonical, domain-qualified identifier, such as "tags.list".
	Name string
	// Domain groups actions and names the meta-tool they belong to.
	Domain string
	// OperationID is the OpenAPI operationId. It is the only machine-checkable
	// link between this declaration, the generated client method (oapi-codegen
	// names every method after it) and the version applicability table.
	OperationID string
	// Title and Description are model-facing.
	Title       string
	Description string
	// Edition is the minimum edition offering this action. CE means both.
	Edition edition.Edition
	// Mutating marks an action that changes server state. Read-only mode
	// refuses to register these, and safe mode intercepts them.
	Mutating bool
	// Destructive marks an action that removes or irreversibly alters state.
	// Implies Mutating.
	Destructive bool
	// Idempotent marks an action that can be repeated without additional
	// effect. Only meaningful when Mutating.
	Idempotent bool
	Handler    Handler
}

// Validate reports whether the spec is internally coherent.
//
// It deliberately does not check that OperationID resolves — that requires the
// applicability table and belongs to catalog-level validation, where the
// edition is known.
func (s ActionSpec) Validate() error {
	name := s.Name
	if name == "" {
		name = "(unnamed)"
	}
	fail := func(format string, args ...any) error {
		return fmt.Errorf("%s: %s", name, fmt.Sprintf(format, args...))
	}

	if s.Name == "" {
		return fail("Name is required")
	}
	if s.Domain == "" {
		return fail("Domain is required")
	}
	if !strings.HasPrefix(s.Name, s.Domain+".") {
		return fail("Name must be domain-qualified as %q", s.Domain+".<action>")
	}
	if s.OperationID == "" {
		return fail("OperationID is required: it is the only link to the generated client")
	}
	if s.Title == "" {
		return fail("Title is required")
	}
	if s.Description == "" {
		return fail("Description is required: it is what the model reads to choose this action")
	}
	switch s.Edition {
	case edition.CE, edition.EE:
	default:
		return fail("Edition %q is not CE or EE", s.Edition)
	}
	if s.Destructive && !s.Mutating {
		return fail("Destructive implies Mutating; read-only and safe mode both key off Mutating")
	}
	if s.Handler == nil {
		return fail("Handler is required")
	}
	return nil
}

// compile-time proof that the Handler signature matches what domains write.
var _ Handler = func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil }
