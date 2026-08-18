// Package teams declares the Portainer team actions.
//
// Five actions for five routes, all generated (actions.go): list, create,
// inspect, update and delete. A team is one half of Portainer's access
// model — the other half is internal/tools/team_memberships, the join
// between a user and a team that carries the user's role inside it. Nothing
// in this package adds or removes a member of a team after the team exists;
// that is entirely team_memberships.create's and team_memberships.delete's
// job, with one measured exception recorded in TeamCreate's narrative below
// (the create payload's own TeamLeaders array does create memberships, at
// creation time only).
//
// Two facts about this domain were measured against a live server of each
// edition rather than read off the vendored documents, because neither
// document states them:
//
//   - Deleting a team deletes that team's memberships with it. Neither
//     document says what becomes of them, and the two readings — cascade,
//     or orphaned rows pointing at a team id that no longer resolves — are
//     very different things to tell a model. It is a cascade; see
//     TeamDelete's narrative for the transcript this claim rests on.
//   - DenyPortainerAccess exists in the Business Edition's create/update
//     payload schemas and not in Community's, and a live Community server
//     accepts the field and silently ignores it rather than rejecting it.
//     The Input structs are scaffolded from the Business document, so the
//     field is offered on both editions; TeamCreate's and TeamUpdate's
//     narratives say what Community actually does with it.
package teams

import (
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every team action. All five are generated (see actions.go);
// this domain has no hand-written action and no redaction wrapper — no
// operation here returns a credential-shaped field, portainer.Team being
// Id, Name and DenyPortainerAccess and nothing else.
//
// All five route through toolutil.WithNarrative rather than assigning
// Title/Description directly in an ActionSpec literal, the way
// internal/tools/endpoint_groups/endpoint_groups.go's own Specs() does and
// for the same reason: it is what lets cmd/audit_spec_drift recognise each
// deliberate improvement on the vendored specification's own wording as
// deliberate instead of gating on it as accidental drift.
func Specs() []toolutil.ActionSpec {
	return generatedSpecs()
}

// narrative supplies every action's ActionSpec narrative fields. Every
// operationId this domain declares appears here, routed through
// toolutil.WithNarrative rather than assigned in an ActionSpec literal —
// see docs/domain-wave-checklist.md, "Narrative overrides are structural".
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "TeamList":
		return toolutil.ActionNarrative{
			Title:       "List teams",
			Description: "Returns every team this caller may see, each as Id, Name and DenyPortainerAccess — never its members, which are read from team_memberships.list_for_team instead. Two optional filters narrow the list, and BOTH were measured to return an empty array for an administrator on both editions, including for a team the administrator holds a Role 1 (team leader) membership in: onlyLedTeams true returned [] where the unfiltered call returned that same led team, and environmentId 1 returned [] where environment 1 exists. Treat either filter as unreliable for an administrator and filter the unfiltered list yourself.",
		}
	case "TeamCreate":
		return toolutil.ActionNarrative{
			Title:       "Create a team",
			Description: "Creates a team with the given name and returns it. A duplicate name is refused with 409 rather than silently accepted, so the name is effectively the team's unique key. The optional TeamLeaders array is the one and only way a team action itself creates memberships: measured on both editions, creating a team with TeamLeaders [1] left a membership for user 1 in that team with Role 1 (team leader). Every membership after creation — leader or member — goes through team_memberships.create instead. DenyPortainerAccess is Business Edition only: Community accepts the field and ignores it, always reporting the team's DenyPortainerAccess as false (measured on both editions).",
		}
	case "TeamInspect":
		return toolutil.ActionNarrative{
			Title:       "Inspect one team",
			Description: "Returns one team by id: Id, Name and DenyPortainerAccess. No membership is returned — not a count, not a user id — so read team_memberships.list_for_team for that. An unknown id answers 404, which makes this the cheapest way to ask whether a team still exists; note that team_memberships.list_for_team is NOT an alternative for that question, because it answers 200 with an empty array for a team id that never existed (measured on both editions).",
		}
	case "TeamUpdate":
		return toolutil.ActionNarrative{
			Title:       "Update a team",
			Description: "Updates a team's name, its DenyPortainerAccess flag, or both. Every field is optional and only the ones supplied change: measured on both editions, a PUT carrying DenyPortainerAccess alone left the team's name exactly as it was. This does not touch membership at all — no user joins or leaves a team through this action. DenyPortainerAccess is Business Edition only: on Business a PUT setting it true made the team report DenyPortainerAccess true, while on Community the identical PUT answered 200 and left it false (measured on both editions).",
		}
	case "TeamDelete":
		return toolutil.ActionNarrative{
			Title:       "Delete a team",
			Description: "Permanently deletes a team AND every membership that team held: this cascades, it does not orphan. Measured on both editions — create a team, add a membership for user 1, confirm GET /team_memberships returns it, delete the team, and GET /team_memberships answers 200 with an empty array; the membership id is gone, not left pointing at a team that no longer resolves. No user is deleted, only their membership of this team. Deleting a team that is already gone answers 404 rather than 204, so a repeat call reports an error even though the end state it wanted is the one already in place.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
