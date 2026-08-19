// Package resource_controls declares the Portainer resource-control actions.
//
// Three actions for three routes, all generated (actions.go): create, update
// and delete. A resource control is what makes ONE Docker or Kubernetes
// resource — a container, a volume, a network, a secret, a stack, a config,
// a custom template — visible to some users and teams and not others. It is
// the per-resource half of the access model whose per-environment half is
// endpoints' and endpoint_groups' access policies, and whose subjects are
// internal/tools/teams and the users a team is built from.
//
// # There is no route that reads a resource control back
//
// This is the fact that shapes all three narratives, and it is a limitation
// of the Portainer API rather than an omission of this wave. Neither
// vendored document declares a GET under /resource_controls, and a live
// 2.44.0 server of each edition answers 405 Method Not Allowed to both
// GET /resource_controls and GET /resource_controls/{id} (measured). So:
//
//   - A model cannot list resource controls, and cannot fetch one by id.
//   - resource_controls.create's response is the ONLY place a new control's
//     Id is ever published. Losing it means losing the ability to update or
//     delete that control through this catalog, because both take the id and
//     nothing derives it from the resource.
//   - resource_controls.update's response is the only direct confirmation of
//     what a change stored.
//
// A control is otherwise visible only INLINE, on the resource it guards, and
// only for the resource kinds this catalog can already read: stacks.list,
// stacks.inspect, custom_templates.list and custom_templates.inspect return
// objects carrying a ResourceControl field. For a container, volume,
// network, secret or config there is no such action in this catalog today,
// so a control over one of those is not observable here at all once created.
// (Portainer itself does expose it — a volume read through its Docker proxy
// carries Portainer.ResourceControl, which is what test/e2e/suite's raw
// read-backs use — but that proxy route has no action in this catalog, so
// it is background rather than something a caller of these actions can
// reach.) Each narrative says this in the terms its own caller meets.
//
// # ResourceID is Portainer's identifier for the resource, not its name
//
// Measured on Community Edition: creating a volume named V through
// Portainer's own Docker proxy answered with ResourceID
// "V_dcm81sn24aeeuf36q9cbktjkw" — the volume's name with a node/cluster
// suffix — and auto-created a resource control keyed on exactly that
// string. A create naming the plain volume name instead was accepted with
// 200 and produced a SECOND, unrelated control that governs nothing,
// because Portainer never checks that the named resource exists. A create
// naming the composite id was refused 409, correctly, because the volume
// already had one. Both halves are in resource_controls.create's narrative:
// the silent no-op is the expensive failure here, not the 409.
//
// # Three refusals come from this catalog, not from Portainer
//
// docs/domain-wave-checklist.md's rule is that a narrative describes the
// catalog, and these three are why it matters in this domain. All measured
// through the dynamic surface against the live Community leg:
//
//	resource_controls.create: validating root: required: missing properties: ["type"]
//	resource_controls.create: validating root: validating /properties/type: enum: 99 does not equal any of: [1 2 3 4 5 6 7 8 9]
//	resource_controls.create: validating root: unexpected additional properties ["subResourceIds"]
//
// None of those calls reached Portainer. The third is the one a model will
// meet by accident: the create payload's SubResourceIDs property becomes the
// published wire name "subResourceIdS", with a capital S. The natural
// spelling "subResourceIds" is refused by this action's own schema before any
// call is made, so the create narrative gives the exact spelling.
//
// That capital S is a catalog-wide defect in bodyJSONTag, not something this
// domain chose, and it is worth naming precisely because the obvious guess is
// wrong: the rule does NOT live in internal/specnaming, which holds only the
// parameter/body collision rule and the synthetic-operationId rule and
// contains neither function involved. The derivation, confirmed by running
// the two functions over the real names:
//
//	SubResourceIDs  splitWords=["Sub" "Resource" "ID" "s"]  goFieldName="SubResourceIDs"  bodyJSONTag="subResourceIdS"
//	TagIDs          splitWords=["Tag" "ID" "s"]             goFieldName="TagIDs"          bodyJSONTag="tagIdS"
//	TagIds          splitWords=["Tag" "Ids"]                goFieldName="TagIDs"          bodyJSONTag="tagIds"
//
// splitWords is right: a fully-matched initialism run leaves the pluralising
// "s" as its own word. goFieldName special-cases that lone "s"
// (isPluralSuffixWord), and its own doc comment at
// cmd/gen_action_inputs/naming.go already spells out the exact failure being
// avoided — "this lone 's' upper-cases to 'S', rendering 'TagIDS'".
// bodyJSONTag has no such branch: its loop is title(w) for every word after
// the first, so "s" becomes "S". The fix landed in one function and not its
// neighbour. Both live in cmd/gen_action_inputs/naming.go and are mirrored in
// internal/specdiff/naming.go, and the two copies must be changed together —
// TestUnit_WireNames_MatchSpecdiffOnEveryRealOperation pins them to each
// other over every operation in both vendored documents.
//
// Note the third line above: the defect fires only when the wire name spells
// the initialism in full. "TagIds" renders correctly as "tagIds"; "TagIDs"
// does not. Five published fields across three domains are affected today —
// subResourceIdS here, tagIdS twice in endpoint_groups, and tagIdS plus
// endpointIdS in endpoints, the last two already on main. Renaming one
// domain's field in isolation is not the fix: it would make
// cmd/audit_spec_drift report one field added and one removed on an operation
// nobody touched, which is the bogus-allowlist trap that test's own doc
// comment warns about (measured — see this task's report).
package resource_controls

import (
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// Specs declares every resource-control action. All three are generated (see
// actions.go); this domain has no hand-written action and no redaction
// wrapper — portainer.ResourceControl is Id, ResourceId, SubResourceIds,
// Type, UserAccesses, TeamAccesses, Public, AdministratorsOnly and System,
// none of them credential-shaped.
//
// All three route through toolutil.WithNarrative rather than assigning
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
	case "ResourceControlCreate":
		return toolutil.ActionNarrative{
			Title: "Restrict one resource to chosen users or teams",
			Description: "Creates the access control over ONE Docker or Kubernetes resource, naming who may see and use it. KEEP THE Id THIS RETURNS: nothing in this catalog, and no route on the Portainer server, reads a resource control back — GET /resource_controls and GET /resource_controls/{id} both answer 405 Method Not Allowed (measured on both editions) — so this response is the only place the new control's Id is ever published, and resource_controls.update and resource_controls.delete both take that id and nothing else. A control is otherwise visible only inline on the resource it guards, and only for resources this catalog can read: stacks.list, stacks.inspect, custom_templates.list and custom_templates.inspect carry a ResourceControl field. Over a container, volume, network, secret or config it is not observable through this catalog at all once created. " +
				"resourceId must be PORTAINER'S identifier for the resource, which is not always its name, and getting it wrong fails silently: Portainer does not check that the named resource exists, so a create over a name nothing has answers 200 and stores a control that governs nothing (measured on Community). For a Docker volume the identifier is the volume name with a node/cluster suffix — creating a volume through Portainer answered ResourceID \"myvolume_dcm81sn24aeeuf36q9cbktjkw\" — and Portainer had already auto-created a control keyed on exactly that string, so a create naming it is refused 409 \"A resource control is already associated to this resource\". One resource holds at most one control: when a resource already has one, use resource_controls.update with its id rather than creating a second. For a stack the vendored document states the identifier is the stack NAME rather than its numeric id — that one is quoted from the specification and was NOT measured here, unlike every other claim in this paragraph. " +
				"type is required and this action's own schema accepts only 1 to 9 — a value outside that range is refused here, naming the enum, before Portainer is called. 1 container, 2 service, 3 volume, 4 network, 5 secret, 6 stack, 7 config, 8 custom template, 9 azure container group. " +
				"At least one grant must actually be ON, or Portainer refuses the call with 400 \"Invalid payload: must specify Users, Teams, Public or AdministratorsOnly\" (measured on both editions): send a non-empty users, a non-empty teams, public true, or administratorsOnly true. Sending them all as false or empty is refused too — a control that grants nobody anything cannot be created. users takes user identifiers and teams takes team identifiers from teams.list. " +
				"The optional list of sub-resources is spelled subResourceIdS, with a capital S — that is the field name this action publishes, and the natural spelling subResourceIds is refused by this action's own schema with `unexpected additional properties [\"subResourceIds\"]` without ever reaching Portainer. THIS IS THE ONLY PLACE SUB-RESOURCES CAN BE SET: resource_controls.update publishes no sub-resource field and Portainer ignores one on that route, so the list is fixed at creation and immutable afterwards (measured). It is not wiped by a later update either, so there is never a reason to delete and re-create a control in order to keep it.",
		}
	case "ResourceControlUpdate":
		return toolutil.ActionNarrative{
			Title: "Replace who may access one controlled resource",
			Description: "Changes who may access the resource one existing control guards, addressed by that CONTROL's own id — never by the resource. This is a REPLACE across THE FOUR FIELDS IT TAKES, and those four only: users, teams, public and administratorsOnly. Any of the four you omit is cleared, not preserved — measured on both editions, a control holding users [1] and teams [T] came back with UserAccesses and TeamAccesses both empty after an update carrying public alone — so send the full intended set of all four every time, including the parts that are not changing. " +
				"SubResourceIds is NOT one of those four and is NOT wiped: it survives an update untouched (measured — a control created with sub-resources [\"sub-a\",\"sub-b\"] still reported both after an update carrying only public). It also cannot be changed here at all: this action publishes no sub-resource field, so sending subResourceIdS is refused by its own schema with `unexpected additional properties [\"subResourceIdS\"]` before Portainer is called, and Portainer ignores the field on this route even when it is sent directly. Sub-resources are fixed at resource_controls.create and are immutable afterwards. Do NOT delete and re-create a control to change or preserve them: deleting one widens access to the resource in the meantime, and it is never necessary here. " +
				"At least one grant must be ON in the result, or Portainer refuses with 400 \"Invalid payload: must specify Users, Teams, Public or AdministratorsOnly\" (measured on both editions) — an update carrying only empty users and teams is refused, so a control cannot be emptied into granting nobody anything. To take access away entirely, use resource_controls.delete instead. " +
				"You need the control's id, and this catalog has no way to look one up: nothing lists or inspects resource controls, and Portainer answers 405 to GET /resource_controls and GET /resource_controls/{id} (measured on both editions). Take the id from resource_controls.create's response, or from the ResourceControl field of the resource itself where this catalog can read it — stacks.list, stacks.inspect, custom_templates.list, custom_templates.inspect. An id no control has answers 404. " +
				"The response carries the whole stored control, and it is the only direct confirmation of what landed, so read it rather than assuming: an update quietly clearing a grant it was not asked about looks identical to a successful one otherwise. Repeating an identical update is safe — it converges on the same state, measured on both editions.",
		}
	case "ResourceControlDelete":
		return toolutil.ActionNarrative{
			Title: "Remove one resource's access restriction",
			Description: "Deletes a resource control by its own id. This WIDENS access rather than narrowing it: the resource itself — the container, volume, stack or template — is untouched and keeps running; only the restriction over it is removed, after which Portainer's default visibility applies. Nothing here deletes a user, a team, or the resource. " +
				"Deleting a control that is already gone answers 404 rather than 204, so a repeat call reports an error even though the end state it wanted is the one already in place; the action is marked idempotent because the request names the control and that end state does not change on a retry. " +
				"You need the control's id, and this catalog has no way to look one up: nothing lists or inspects resource controls, and Portainer answers 405 to GET /resource_controls and GET /resource_controls/{id} (measured on both editions). Take the id from resource_controls.create's response, or from the ResourceControl field of the resource itself where this catalog can read it — stacks.list, stacks.inspect, custom_templates.list, custom_templates.inspect. There is no delete-by-resource form: a lost id cannot be recovered through these actions. " +
				"The answer is a bare {\"status\":\"ok\"} — Portainer itself answers 204 with no body, and this action reports that success rather than passing an empty response on. Confirm the effect on the resource where this catalog can read it; over a container, volume, network, secret or config there is nothing here that shows the control is gone.",
		}
	default:
		return toolutil.ActionNarrative{}
	}
}
