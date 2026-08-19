// Package endpoint_groups declares the Portainer environment-group actions.
//
// Seven actions for seven routes. Six are generated (actions.go); the
// seventh, endpoint_groups.inspect, is hand-written in handlers.go and is
// the interesting one.
//
// GET /endpoint_groups/{id} carries no operationId in either vendored
// specification — the one case of docs/api-divergences.md §6.2's fourteen
// that cross-edition borrowing cannot repair, because there is no name in
// either document to borrow. The route is otherwise fully documented in
// both (summary, description, both parameters, a response schema) and, as
// measured against a live server of each edition, answers 200 on both. Only
// the name was missing.
//
// This domain shipped without it, on the strength of a static audit that
// could see no name for it. That was wrong twice over: it lost Community
// Edition functionality the server plainly serves, and it did so invisibly,
// because an operation nothing names is counted in no coverage figure and
// reported in no gap list. internal/specnaming's explicit table now names
// the route EndpointGroupInspect — the same name cmd/gen_applicability
// writes into internal/apiversion's operationIDs index for BOTH editions,
// which is what lets actioncatalog resolve this action's edition at all —
// and this package declares the action under it.
//
// A model wanting one group therefore reads it here rather than filtering
// endpoint_groups.list, and gets the group's own record whether or not the
// caller can see the whole list.
package endpoint_groups

import (
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every environment-group action: generatedSpecs()'s six
// entries (see actions.go) plus the one kept hand-written in handlers.go.
// This domain has no redaction wrapper — no operation here returns a
// credential-shaped field.
//
// All seven route through toolutil.WithNarrative rather than assigning
// Title/Description directly in an ActionSpec literal, the way
// internal/tools/endpoints/endpoints.go's own Specs() does and for the same
// reason: it is what lets cmd/audit_spec_drift recognise each deliberate
// improvement on the vendored specification's own wording as deliberate
// instead of gating on it as accidental drift.
func Specs() []toolutil.ActionSpec {
	return append(generatedSpecs(),
		toolutil.WithNarrative(toolutil.ActionSpec{
			Name: "endpoint_groups.inspect", Domain: "endpoint_groups", OperationID: "EndpointGroupInspect",
			// Verbatim from the Business document's own summary and
			// description for GET /endpoint_groups/{id} (its access-policy
			// line stripped, as cmd/gen_action_inputs strips it), so this
			// action's spec-derived wording is what the audit compares
			// against, exactly as for the six generated siblings. The
			// Community document says the same thing with a typo ("abont");
			// cmd/audit_spec_drift resolves an operationId against the
			// Business document first, so this follows Business.
			Title:       "Inspect an Environment(Endpoint) group",
			Description: "Retrieve details about an environment(endpoint) group.",
			// CE, and this line is the whole point of the action. Neither
			// vendored document names this route, so nothing could be
			// borrowed across editions the way EndpointGroupCreate's
			// comment above describes; internal/specnaming's table supplies
			// the name instead, and cmd/gen_applicability writes it into
			// internal/apiversion's operationIDs index for both editions
			// after proving from each edition's own spans table that each
			// serves GET /endpoint_groups/{id}. Measured directly besides:
			// both editions answer 200 (see the narrative).
			Edition: edition.CE,
			Handler: endpointGroupInspect,
			Input:   endpointGroupInspectInput{},
		}, narrative("EndpointGroupInspect")),
	)
}

// narrative supplies every action's ActionSpec narrative fields. Every
// operationId this domain declares appears here, routed through
// toolutil.WithNarrative rather than assigned in an ActionSpec literal —
// see docs/domain-wave-checklist.md, "Narrative overrides are structural".
func narrative(operationID string) toolutil.ActionNarrative {
	switch operationID {
	case "EndpointGroupList":
		return toolutil.ActionNarrative{
			Title:       "List environment groups",
			Description: "Returns every environment group on this Portainer server, without the environments each holds: Total and the TypeInfo breakdown are both 0 for every group unless size is passed true, which asks the server to compute them (measured: Community reports Total 0 for the sole group without size, Total 2 with it — no group's membership is ever returned by name or id, on either edition). A group is how access policies and tags are applied to several environments at once; every environment belongs to exactly one, and one that ends up in none lands in group 1, the built-in \"Unassigned\" group.",
		}
	case "EndpointGroupInspect":
		return toolutil.ActionNarrative{
			Title:       "Inspect one environment group",
			Description: "Returns one environment group by id: Id, Name, Description, the tag ids attached to it when it has any, Total and the TypeInfo breakdown — plus a Policies array on Business Edition, which Community omits entirely, so do not depend on it. Membership is NOT returned: no environment is named or numbered here, on either edition. Total and TypeInfo behave exactly as they do on endpoint_groups.list — both read 0 unless size is passed true, which asks the server to count (measured on both editions: without size a group holding one environment reports Total 0; with it, Total 1 and TypeInfo.Docker 1). An unknown id answers 404 rather than an empty group, which makes this the cheapest way to ask whether a group still exists.",
		}
	case "EndpointGroupCreate":
		return toolutil.ActionNarrative{
			Title:       "Create an environment group",
			Description: "Creates a new environment group with the given name, and optionally moves the listed environments into it. An environment can belong to only one group at a time, so any environment named here leaves whatever group it was previously in.",
		}
	case "EndpointGroupUpdate":
		return toolutil.ActionNarrative{
			Title:       "Update an environment group",
			Description: "Updates an environment group's name, description, tags or access policies. userAccessPolicies and teamAccessPolicies each map an identifier to a RoleId, and those role identifiers come from roles.list — read it first, because Portainer does not validate RoleId: measured on Business Edition, a policy naming RoleId 99 (an id no role has) was accepted with 200 and stored verbatim, so a wrong one grants nothing and reports nothing. On Community Edition roles.list answers an empty array, roles being a Business Edition feature. If associatedEndpoints is supplied, it REPLACES the group's membership outright rather than adding to it: every environment not named moves back to group 1 (\"Unassigned\"), discarding whatever access policies or tags it inherited from this group — measured against a live server: PUT with associatedEndpoints:[] evicted a group's only member, and omitting associatedEndpoints entirely left membership untouched. List the group's full current membership first (endpoint_groups.list does not return it; read it from endpoints.list's groupId) before adding to it, or an unlisted member is silently dropped.",
		}
	case "EndpointGroupDelete":
		return toolutil.ActionNarrative{
			Title:       "Delete an environment group",
			Description: "Permanently deletes an environment group. This does not delete any environment: every environment the group held moves back into group 1, the built-in \"Unassigned\" group (measured against a live server: creating a group, populating it with one environment, then deleting the group leaves that environment in group 1).",
		}
	case "EndpointGroupAddEndpoint":
		return toolutil.ActionNarrative{
			Title:       "Move an environment into a group",
			Description: "Moves one environment into this group. An environment belongs to only one group at a time — measured against a live server: adding it to a new group changes its own recorded group to this one, which is how its eviction from whatever group held it before was confirmed.",
		}
	case "EndpointGroupDeleteEndpoint":
		return toolutil.ActionNarrative{
			Title:       "Remove an environment from a group",
			Description: "Removes one environment from this group. This does not delete the environment: it moves back into group 1, the built-in \"Unassigned\" group, and a later read of it still answers normally (measured against a live server).",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
