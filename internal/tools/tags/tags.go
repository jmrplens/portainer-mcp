// Package tags declares the Portainer environment-tag actions.
//
// Three actions: list, create, delete. It is the smallest domain in the
// catalog that includes a genuinely destructive action, which makes it the
// pilot for that classification — tags.delete removes a tag outright, with no
// softer alternative the way some domains offer a stop/disable short of
// deletion.
//
// All three actions — tags.list, tags.create, tags.delete — run on
// cmd/gen_action_inputs's generated code (actions.gen.go). TagList takes no
// input, its handler is a bare resp.JSON200 passthrough with no hand-shaped
// acknowledgement, and no existing test pins a guard clause a generated
// handler would not have. TagCreate's and TagDelete's only existing tests
// that called the handler directly
// (TestTagCreate_MissingName_ReturnsErrorWithoutCallingAPI,
// TestTagDelete_InvalidID_ReturnsErrorWithoutCallingAPI) now route through
// tools.Execute instead: the required-field check ("name is required") is
// exactly what tools.Execute's central schema validation enforces for every
// surface, and the non-positive-id check is now enforced the identical way,
// via a generated JSON Schema "minimum": 1 on TagDelete's own "id" path
// parameter (toolutil.MinimumParams, cmd/gen_action_inputs/fields.go's
// isIdentifierPathParam — this project's own addition, not the
// specification's; see that generator's doc comment for why). Neither test
// is weaker for it — the property under test (invalid input refused before
// the API is ever called) is unchanged, only where that refusal actually
// happens. PortainerTag (every one of these three operations' response type)
// carries nothing credential-shaped, so none needed a redaction wrapper the
// way registries.list/create/inspect/update did (see task-4b's report).
package tags

import (
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every tag action: all three now come from generatedSpecs().
func Specs() []toolutil.ActionSpec {
	return generatedSpecs()
}

// narrative supplies TagList's and TagCreate's ActionSpec narrative fields to
// generatedSpecs() (see actions.gen.go): only Title/Description, preserving
// the exact wording this domain hand-authored before each's swap to
// generated code, rather than letting it silently degrade to the vendored
// specification's own terser summary/description. Every other operationId
// returns the zero toolutil.ActionNarrative — nothing else in this domain is
// generated today.
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "TagList":
		return toolutil.ActionNarrative{
			Title:       "List tags",
			Description: "Returns every environment tag defined on this Portainer server.",
		}
	case "TagCreate":
		return toolutil.ActionNarrative{
			Title:       "Create a tag",
			Description: "Creates a new environment tag with the given name.",
		}
	case "TagDelete":
		return toolutil.ActionNarrative{
			Title:       "Delete a tag",
			Description: "Permanently removes a tag, unassigning it from every environment and environment group that carries it. This cannot be undone.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}

// tagCreate and tagDelete are no longer declared here: both run on
// cmd/gen_action_inputs's generated code (actions.gen.go) — see this file's
// package doc. PortainerTag carries nothing credential-shaped, so neither
// needed a redaction wrapper to become eligible.
