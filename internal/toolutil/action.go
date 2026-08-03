package toolutil

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
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
	// Input is a zero value of the action's parameter struct, or nil when the
	// action takes none. The JSON Schema published by every surface is
	// reflected from this type, and each handler unmarshals into the same
	// type — one declaration, so the published shape and the parsed shape
	// cannot drift.
	Input any

	// Aliases are alternative names a model might reach for instead of Name,
	// such as a REST verb or a synonym used elsewhere in Portainer's own
	// documentation. Purely discovery metadata: no surface resolves an
	// alias to the action, it only helps a model recognise the right one.
	Aliases []string
	// Tags group actions by concept across domains — e.g. "kubernetes" or
	// "audit-log" — independent of Domain, which groups them by meta-tool.
	Tags []string
	// Usage is one or two sentences of model-facing guidance on when to
	// reach for this action instead of a neighbouring one, complementing
	// Description rather than repeating it.
	Usage string
	// RelatedActions names other actions' Name values a model is likely to
	// need next to, or instead of, this one.
	RelatedActions []string
	// ParameterGuidance carries per-parameter discovery metadata, keyed by
	// the parameter's JSON field name. Populate it directly for anything
	// genuinely confusable; FillScopeParameterGuidance fills the rest from a
	// central table of Portainer's well-known identifier names.
	ParameterGuidance map[string]ParameterGuidance
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
	local, qualified := strings.CutPrefix(s.Name, s.Domain+".")
	if !qualified || local == "" {
		return fail("Name must be domain-qualified as %q with a non-empty action part", s.Domain+".<action>")
	}
	// MCP tool names allow only [A-Za-z0-9_.-], and the SDK merely logs a bad
	// name through a logger that discards by default, then registers it anyway.
	// Catching it here means no surface can produce an unusable tool.
	for _, r := range s.Name {
		if !isLegalToolNameRune(r) {
			return fail("Name contains %q, which MCP tool names disallow; use only letters, digits, underscore, dot or hyphen", r)
		}
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
	if s.Input != nil {
		t := reflect.TypeOf(s.Input)
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct {
			return fail("Input must be a struct or nil, got %s: the schema every surface publishes is reflected from it", t.Kind())
		}
	}
	if len(s.ParameterGuidance) > 0 {
		// A typo or a renamed field otherwise leaves a stale ParameterGuidance
		// entry that silently does nothing — the same class of drift this
		// project already refuses for EnumParams (see schema.go/schema_test.go)
		// for the stated reason: a loud refusal beats a struct that is
		// silently wrong. jsonFieldNames is the same resolution
		// FillScopeParameterGuidance already uses, so a name valid there is
		// valid here.
		known := make(map[string]bool, len(s.ParameterGuidance))
		for _, name := range jsonFieldNames(s.Input) {
			known[name] = true
		}
		for name := range s.ParameterGuidance {
			if !known[name] {
				return fail("ParameterGuidance names %q, which is not a JSON field of Input", name)
			}
		}
	}
	return nil
}

// isLegalToolNameRune reports whether r is one of the characters MCP tool
// names allow: letters, digits, underscore, dot or hyphen.
func isLegalToolNameRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z':
	case r >= 'A' && r <= 'Z':
	case r >= '0' && r <= '9':
	case r == '_' || r == '.' || r == '-':
	default:
		return false
	}
	return true
}

// compile-time proof that the Handler signature matches what domains write.
var _ Handler = func(context.Context, *portainer.Client, json.RawMessage) (any, error) { return nil, nil }
