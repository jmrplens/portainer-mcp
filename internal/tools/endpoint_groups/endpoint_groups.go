// Package endpoint_groups declares the Portainer environment-group actions.
//
// Six actions for seven routes. GET /endpoint_groups/{id} — inspect one
// group — carries no operationId in either vendored specification, the one
// case of docs/api-divergences.md §6.2's fourteen that cross-edition
// borrowing cannot repair, because there is no name in either document to
// borrow. It is not declared here and cannot be; a model wanting one group
// reads it out of endpoint_groups.list.
package endpoint_groups

import "github.com/jmrplens/portainer-mcp/internal/toolutil"

// Specs declares every environment-group action. All six are generated;
// this domain has no hand-written handler and no redaction wrapper.
func Specs() []toolutil.ActionSpec { return generatedSpecs() }

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
	case "EndpointGroupCreate":
		return toolutil.ActionNarrative{
			Title:       "Create an environment group",
			Description: "Creates a new environment group with the given name, and optionally moves the listed environments into it. An environment can belong to only one group at a time, so any environment named here leaves whatever group it was previously in.",
		}
	case "EndpointGroupUpdate":
		return toolutil.ActionNarrative{
			Title:       "Update an environment group",
			Description: "Updates an environment group's name, description, tags or access policies. If associatedEndpoints is supplied, it REPLACES the group's membership outright rather than adding to it: every environment not named moves back to group 1 (\"Unassigned\"), discarding whatever access policies or tags it inherited from this group — measured against a live server: PUT with associatedEndpoints:[] evicted a group's only member, and omitting associatedEndpoints entirely left membership untouched. List the group's full current membership first (endpoint_groups.list does not return it; read it from endpoints.list's groupId) before adding to it, or an unlisted member is silently dropped.",
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
