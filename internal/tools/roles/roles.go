// Package roles declares the Portainer role actions.
//
// One action for one route, generated (actions.go): roles.list, GET /roles.
// The domain takes no parameters and mutates nothing. Its whole value is
// that the identifiers it returns are what an access policy's RoleId takes —
// endpoints.update's userAccessPolicies and teamAccessPolicies, and the same
// two fields on endpoint_groups' create and update.
//
// Two facts were measured against a live 2.44.0 server of each edition
// rather than read off the vendored documents, because neither document
// states either one:
//
//   - The route answers an EMPTY ARRAY on Community Edition and six roles on
//     Business. Both documents declare it, both servers serve it, and both
//     answer 200 — so the action is Edition: edition.CE and is genuinely
//     published and genuinely callable on Community. It simply has nothing
//     to list there: role-based access control is a Business Edition
//     feature and a Community server holds no roles at all. Nothing in this
//     catalog prunes, gates or rewrites the action per edition, so the
//     emptiness a Community caller sees is Portainer's answer arriving
//     unaltered, not a refusal manufactured here. That distinction is what
//     roles.list's narrative has to carry, because a model that gets [] and
//     cannot tell "no roles exist" from "this failed" will guess. Recorded
//     in docs/api-divergences.md §5 as an edition asymmetry of a kind that
//     section did not previously hold: not a field missing from a shared
//     schema, but a shared route whose answer is empty on one edition.
//
//   - Portainer does not validate a RoleId written into an access policy.
//     Measured on Business Edition: PUT /endpoint_groups/{id} carrying
//     userAccessPolicies {"1": {"RoleId": 99}} answered 200 and stored
//     RoleId 99, an id no role has. That is why roles.list is worth an
//     action of its own rather than a constant table in a description —
//     it is the only way to learn which identifiers are real, and nothing
//     downstream will object to one that is not.
//
// The two documents also disagree about who may call the route: Community's
// declares "**Access policy**: administrator" and Business's declares
// "**Access policy**: authenticated". Not measured — this estate has only
// its administrator — so no narrative here claims anything about a
// non-administrator caller.
package roles

import (
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every role action. The one action is generated (see
// actions.go); this domain has no hand-written action, no Input struct (the
// operation takes no parameters at all) and no redaction wrapper —
// portaineree.Role is Id, Name, Description, Priority, Scope and
// Authorizations, none of them credential-shaped.
//
// It routes through toolutil.WithNarrative rather than assigning
// Title/Description directly in an ActionSpec literal, the way every other
// domain's Specs() does and for the same reason: it is what lets
// cmd/audit_spec_drift recognise a deliberate improvement on the vendored
// specification's own wording as deliberate instead of gating on it as
// accidental drift.
func Specs() []toolutil.ActionSpec {
	return generatedSpecs()
}

// narrative supplies every action's ActionSpec narrative fields. Every
// operationId this domain declares appears here, routed through
// toolutil.WithNarrative rather than assigned in an ActionSpec literal —
// see docs/domain-wave-checklist.md, "Narrative overrides are structural".
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "RoleList":
		return toolutil.ActionNarrative{
			Title: "List the roles an access policy can name",
			Description: "Returns every role defined on this Portainer server. A role's Id is what an access policy's RoleId field takes: endpoints.update's userAccessPolicies and teamAccessPolicies, and the same two fields on endpoint_groups. Read this first when granting access, because Portainer does NOT validate RoleId — an access policy naming RoleId 99, which no role has, was accepted with 200 and stored as written (measured on Business Edition), so a wrong id fails silently rather than being refused. " +
				"THE ANSWER DIFFERS BY EDITION, and an empty answer is not an error. On Business Edition this returns six roles, measured through all three tool surfaces: 1 Environment Administrator, 2 Helpdesk User, 3 Standard User, 4 Read-only User, 5 Operator, 6 Namespace Operator. On Community Edition the identical call succeeds and returns an EMPTY ARRAY — also measured through all three surfaces. That is Portainer's own answer passed through unchanged, not a refusal from this catalog and not a permissions problem: role-based access control is a Business Edition feature, so a Community server has no roles to list. On Community, treat the empty list as final rather than retrying or looking for a role id elsewhere; access there is granted without one. " +
				"Each entry carries Id, Name, Description, Priority and Scope, plus Authorizations — a map of one boolean per privilege, and a large one: Environment Administrator alone carries 206 entries, and the whole Business Edition answer arrives here as roughly 24 KB of pretty-printed JSON (23,964 bytes; Portainer's own compact response body is 17,913, so a `curl` check will show about a third less). Ask for this once and keep the Id and Name; nothing here takes a filter or a page.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
