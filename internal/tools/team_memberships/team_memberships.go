// Package team_memberships declares the Portainer team-membership actions.
//
// Five actions for five routes, all generated (actions.go). A team
// membership is the join between a user and a team, and it carries that
// user's role *in that team*: Role 1 is team leader, Role 2 is an ordinary
// member. It is a first-class object with its own id, its own routes and
// its own lifecycle — team_memberships.create is how a user joins a team,
// and no action in internal/tools/teams adds one to a team that already
// exists (teams.create's own TeamLeaders array is the single exception, and
// only at creation time; see that action's narrative).
//
// Two of these five actions are called "list" something, and the difference
// between them is the whole reason the second one needed a name of its own:
//
//   - team_memberships.list (GET /team_memberships) returns EVERY membership
//     on the server, across all teams.
//   - team_memberships.list_for_team (GET /teams/{id}/memberships) returns
//     the memberships of ONE team.
//
// The second's operationId is the bare word "TeamMemberships", identical to
// this domain's own prefix, so cmd/gen_action_inputs's mechanical rule
// refuses it outright rather than mint "team_memberships.team_memberships";
// its actionNameOverrides table supplies "list_for_team" instead, and
// deliberately not the shorter "list", which the *different* operationId
// TeamMembershipList already holds. Both descriptions below say which is
// which, because the names alone are close enough to pick wrongly from.
//
// One fact worth carrying here rather than only in internal/tools/teams:
// deleting a team deletes that team's memberships with it, measured on both
// editions. A model that has just deleted a team does not need to clean up
// after it here, and a membership id it held before that deletion is gone,
// not merely dangling. See teams.delete's own narrative for the transcript.
package team_memberships

import (
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every team-membership action. All five are generated (see
// actions.go); this domain has no hand-written action and no redaction
// wrapper — portainer.TeamMembership is Id, UserID, TeamID and Role, none
// of them credential-shaped.
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
	case "TeamMembershipList":
		return toolutil.ActionNarrative{
			Title:       "List every team membership",
			Description: "Returns EVERY team membership on this Portainer server, across all teams — this is the server-wide list. For the memberships of one team, use team_memberships.list_for_team instead; the two names are close and this is the difference between them. Each entry is Id, UserID, TeamID and Role (1 team leader, 2 member), so this is also how a user's own team roles are discovered: filter by UserID. An administrator sees all of them; a team leader sees only the memberships of the teams they lead — measured on both editions: a non-administrator leading one of two teams got back that team's membership alone, not the other team's.",
		}
	case "TeamMembershipCreate":
		return toolutil.ActionNarrative{
			Title:       "Add a user to a team",
			Description: "Creates a membership: this is how a user joins a team, and there is no action on the team itself that adds one after the team exists. All three fields are required — UserID, TeamID, and Role, which is 1 for team leader and 2 for an ordinary member. A user may hold only one membership per team: a second create for the same UserID and TeamID pair is refused with 409 whichever of the two Roles it names, so raising a member to leader is team_memberships.update on the existing membership, not a second create (measured on both editions). Role accepts only 1 or 2 — any other value is refused by this action's own schema before the call is made, so that refusal comes from here and names the enum, not from Portainer.",
		}
	case "TeamMembershipUpdate":
		return toolutil.ActionNarrative{
			Title:       "Change a user's role in a team",
			Description: "Updates one membership by its own id — not by user and team — and its usual purpose is changing Role between 1 (team leader) and 2 (member); measured on both editions, a PUT with Role 2 on a Role 1 membership left it reporting Role 2. All three of UserID, TeamID and Role are required by the payload, so send the membership's current UserID and TeamID back unchanged alongside the Role being changed. Read the membership's id from team_memberships.list or team_memberships.list_for_team first: nothing here accepts a user and team pair.",
		}
	case "TeamMembershipDelete":
		return toolutil.ActionNarrative{
			Title:       "Remove a user from a team",
			Description: "Removes one membership by its own id, which is how a user leaves a team. The user is not deleted and the team is not deleted — only the join between them, and with it whatever access that team granted the user. Deleting a membership that is already gone answers 404 rather than 204, so a repeat call reports an error even though the end state it wanted is the one already in place. Deleting the membership's team removes it too, so there is nothing to clean up here after a teams.delete (measured on both editions).",
		}
	case "TeamMemberships":
		return toolutil.ActionNarrative{
			Title:       "List one team's memberships",
			Description: "Returns the memberships of ONE team, given that team's id — not the server-wide list, which is team_memberships.list; the two names are close and this is the difference between them. Each entry is Id, UserID, TeamID and Role (1 team leader, 2 member), so this is how a team's roster is read: internal/tools/teams returns no membership at all, not even a count. An id no team has answers 200 with an empty array rather than 404 (measured on both editions), so an empty result here does NOT prove the team exists — use teams.inspect, which does answer 404, for that question. An empty result also does not mean access was refused: a team leader asking for a team they do not lead gets 403 \"Access denied to team\", not an empty list (measured on both editions).",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
