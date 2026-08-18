//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// rawEndpointGroups reads the environment group list straight from the
// Portainer API, bypassing every surface, through rawEndpointsJSON
// (endpoints_test.go) -- generic over the path rather than specific to
// /endpoints, so it serves this domain's own reads too rather than being
// reimplemented here.
//
// Read-backs for endpoint_groups.create and endpoint_groups.delete go
// through this, not through endpoint_groups.list (the action under test's
// own sibling): a create or delete that answered plausibly while writing
// nothing would pass a read-back performed through any action at all, and
// this task's brief is explicit that the group's bare existence is proven
// against the server directly.
func rawEndpointGroups(t *testing.T, ed string) []map[string]any {
	t.Helper()
	var out []map[string]any
	rawEndpointsJSON(t, ed, "/endpoint_groups", &out)
	return out
}

// endpointGroupIDByName finds an environment group by name in a raw GET
// /endpoint_groups, or -1.
//
// Existence-only, deliberately: EndpointGroupList never returns membership
// regardless of what is passed to it (Total and the TypeInfo breakdown read
// 0 unless size:true is asked for, and even then no group's membership is
// ever returned by name or id -- see this domain's own EndpointGroupList
// narrative, internal/tools/endpoint_groups/endpoint_groups.go). Nothing in
// this file uses this helper to answer "who is in this group": that
// question is always answered by reading the environment's own GroupId
// (rawEnvironment, endpoints_test.go) instead.
func endpointGroupIDByName(t *testing.T, ed, name string) int {
	t.Helper()
	for _, g := range rawEndpointGroups(t, ed) {
		if g["Name"] == name {
			if id, ok := g["Id"].(float64); ok {
				return int(id)
			}
		}
	}
	return -1
}

// createEndpointGroupFixture creates an environment group named name on
// edition ed's server directly through the Portainer API, not through
// endpoint_groups.create -- the action under test in this domain's own
// lifecycle test below. It exists for the safe-mode table, which needs a
// real, pre-existing group to preview a rename/delete/membership change
// against: building that fixture through the very action under test would
// let a broken create silently pass the test meant to catch it, the same
// reasoning tags_test.go's own doc comment gives for not using createTag to
// set up TestTags_CreateThenListThenDelete.
//
// Retried the same way createTag and createRegistry are (fixtures_test.go):
// every session in the matrix can create fixtures concurrently against the
// same server, and Portainer's own name-uniqueness check races under that
// concurrency.
func createEndpointGroupFixture(t *testing.T, ed, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create endpoint group %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp, err := client.API.EndpointGroupCreateWithResponse(ctx, apigen.EndpointGroupCreateJSONRequestBody{Name: name})
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil {
			return fmt.Errorf("create endpoint group %q: response carried no body", name)
		}
		id = resp.JSON200.Id
		return nil
	})

	newLedger(t).Register("endpoint_group", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEndpointGroupIfPresent(ctx, ed, id)
	})
	return id
}

// deleteEndpointGroupIfPresent deletes group id on edition ed's server only
// if a raw GET /endpoint_groups still reports it, so a caller that already
// deleted it (through the action under test, not through this helper) does
// not turn a passing test into a spurious cleanup failure -- the same
// reasoning as deleteTagIfPresent (fixtures_test.go).
func deleteEndpointGroupIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.EndpointGroupListWithResponse(ctx, &apigen.EndpointGroupListParams{})
	if err != nil {
		return fmt.Errorf("list endpoint groups before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list endpoint groups before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, g := range *resp.JSON200 {
			if g.Id == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.EndpointGroupDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	return toolutil.Check(delResp)
}

// rawAddEndpointToGroup moves an environment into a group directly through
// the Portainer API, bypassing endpoint_groups.add_endpoint -- the action
// under test elsewhere in this file.
//
// It exists to build a genuine, observable starting condition for the
// safe-mode delete_endpoint row below: that row's whole point is proving
// delete_endpoint does not evict a real member, so the member has to be
// real (and really in the group) before the safe-mode call runs. Using this
// domain's own add_endpoint action to put it there would make a defect in
// add_endpoint's handler indistinguishable from a defect in
// delete_endpoint's interception -- exactly the kind of self-certifying
// read-back this suite's own brief warns against.
func rawAddEndpointToGroup(t *testing.T, ed string, groupID, endpointID int) {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.API.EndpointGroupAddEndpointWithResponse(ctx, groupID, endpointID)
	if err != nil {
		t.Fatalf("raw move environment %d into group %d: %v", endpointID, groupID, err)
	}
	if err := toolutil.Check(resp); err != nil {
		t.Fatalf("raw move environment %d into group %d: %v", endpointID, groupID, err)
	}
}

// TestEndpointGroups_CreateAddRemoveThenDelete_AcrossSurfacesAndEditions is
// this domain's mutation template, following tags_test.go's: every mutating
// call is followed by a read-back, and the two that matter most are read
// through the raw Portainer API rather than through any action, exactly as
// the task brief for this suite specifies.
//
// The sequence is the brief's own: create a group, confirm it exists
// server-side, move a real environment into it, confirm that environment's
// own GroupId changed, move it back out, confirm the eviction landed it in
// group 1 ("Unassigned" -- measured fact recorded in this domain's own
// EndpointGroupDeleteEndpoint narrative), then delete the now-empty group
// and confirm it is gone.
//
// The environment this test moves around is one it creates and owns
// (createEnvironmentThroughSurface, endpoints_test.go) rather than the
// shared estate's own Docker environment: unlike the safe-mode table below,
// these calls are real, and reassigning the estate's own environment's
// group out from under every other, concurrently running test in this
// package would be exactly the shared-estate hazard this suite's own
// TestEndpoints_UpdateRelations_ReassignsAnEnvironmentsGroup was written to
// avoid for the identical reason.
func TestEndpointGroups_CreateAddRemoveThenDelete_AcrossSurfacesAndEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				envID := createEnvironmentThroughSurface(t, surface, ed, uniqueName("env-groups"))

				groupName := uniqueName("endpoint-group")
				// endpoint_groups.create is called through the MCP surface
				// under test, not through createEndpointGroupFixture: see
				// that helper's own doc comment, and tags_test.go's
				// identical note about tags.create -- relying on the raw
				// fixture here would let audit_e2e_gaps count
				// endpoint_groups.create as exercised with no test ever
				// actually invoking it through a tool.
				created := callAction[map[string]any](t, session, surface, "endpoint_groups.create", map[string]any{"name": groupName})
				idFloat, ok := created["Id"].(float64)
				if !ok {
					t.Fatalf("endpoint_groups.create response carried no Id: %v", created)
				}
				id := int(idFloat)
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteEndpointGroupIfPresent(ctx, ed, id); err != nil {
						t.Errorf("clean up endpoint group %q: %v", groupName, err)
					}
				})

				// Read back after create, raw: a create that returned a
				// plausible-looking body but wrote nothing would pass
				// without this.
				if got := endpointGroupIDByName(t, ed, groupName); got != id {
					t.Fatalf("group %q was created as %d but the server lists it as %d", groupName, id, got)
				}

				callAction[any](t, session, surface, "endpoint_groups.add_endpoint", map[string]any{
					"id": id, "endpointId": envID,
				})

				// Read back through the ENVIRONMENT, not the group:
				// EndpointGroupList never returns membership (see
				// endpointGroupIDByName's own doc comment), so this is the
				// only field a real move can be proven through. This is the
				// one line Step 4 of the task brief exists to prove bites:
				// stub endpoint_groups.add_endpoint's handler into a no-op,
				// and this assertion must fail on all three surfaces.
				if got, _ := rawEnvironment(t, ed, envID)["GroupId"].(float64); int(got) != id {
					t.Fatalf("environment %d GroupId = %v after endpoint_groups.add_endpoint, want %d", envID, got, id)
				}

				callAction[any](t, session, surface, "endpoint_groups.delete_endpoint", map[string]any{
					"id": id, "endpointId": envID,
				})

				// Removing an environment from a group does not delete it;
				// it lands back in group 1, the built-in "Unassigned" group
				// (this domain's own EndpointGroupDeleteEndpoint narrative).
				if got, _ := rawEnvironment(t, ed, envID)["GroupId"].(float64); int(got) != 1 {
					t.Errorf("environment %d GroupId = %v after endpoint_groups.delete_endpoint, want 1 (Unassigned)", envID, got)
				}

				callAction[any](t, session, surface, "endpoint_groups.delete", map[string]any{"id": id})

				// Read back again, raw: a delete that no-ops would pass
				// without this. This is Step 4's other guard: stub
				// endpoint_groups.delete's handler to return success without
				// calling the API, and this assertion must fail on all three
				// surfaces.
				if got := endpointGroupIDByName(t, ed, groupName); got != -1 {
					t.Errorf("group %q is still present as %d after endpoint_groups.delete", groupName, got)
				}
			})
		}
	}
}

// TestEndpointGroups_List_ReflectsAGroupCreatedDirectly proves
// endpoint_groups.list surfaces every group on the server, not only ones
// this same MCP session happens to have created:
// createEndpointGroupFixture creates directly against the Portainer API,
// the same relationship tags_test.go's own
// TestTags_List_ReflectsATagCreatedDirectly has with createTag, and
// endpoint_groups.list here must still find it.
func TestEndpointGroups_List_ReflectsAGroupCreatedDirectly(t *testing.T) {
	for _, edition := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				name := uniqueName("endpoint-group-list")
				createEndpointGroupFixture(t, edition, name)

				listed := callAction[[]map[string]any](t, session, surface, "endpoint_groups.list", nil)
				// Community Edition's vendored specification omits Policies
				// from EndpointGroupList's response shape, which the
				// Business Edition specification marks required
				// (docs/api-divergences.md §5.3) -- so this only checks the
				// group's own presence by name, never a Policies field,
				// which would fail on CE.
				if !slices.ContainsFunc(listed, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Errorf("group %q created directly does not appear in endpoint_groups.list", name)
				}
			})
		}
	}
}

// TestEndpointGroups_Inspect_ReturnsTheGroupItWasAskedFor exercises
// endpoint_groups.inspect, the seventh action of this domain and the one for
// a route neither vendored document names (internal/specnaming supplies the
// operationId EndpointGroupInspect; see this domain's package comment).
//
// The group is a fixture created directly against the Portainer API
// (createEndpointGroupFixture), never through endpoint_groups.create: the
// action under test here is inspect, and building its subject through a
// sibling action would let one defect mask the other, the same reasoning
// that helper's own doc comment gives.
//
// Three assertions, in increasing strength:
//
//  1. The answer is THIS group -- Id and Name both, not merely a 200. An
//     inspect that ignored its id parameter and always answered group 1
//     ("Unassigned", present on every estate) would satisfy a bare
//     success check and fail this one on both fields.
//  2. Without size, Total reads 0 even once the group really holds an
//     environment. That is the server's documented default and this
//     domain's measured behaviour, identical to endpoint_groups.list's.
//  3. With size true, Total reads 1 for that same group at that same
//     moment. This is the pair that proves the handler actually sends the
//     query parameter: a handler that dropped size would report 0 twice, and
//     one that hard-coded it would report 1 twice. Neither passes both rows.
//
// Policies is deliberately never touched: Business Edition returns it and
// Community Edition omits it entirely (docs/api-divergences.md 5.3), so an
// assertion on it would fail on CE -- the same restraint
// TestEndpointGroups_List_ReflectsAGroupCreatedDirectly already exercises.
//
// Deliberately NOT parallel across its own ed/surface subtests, for exactly
// the reason TestEndpointGroups_Update_AssociatedEndpointsReplacesMembership
// records below: it registers a real environment per subtest, and six
// concurrent registrations (three surfaces x CE and EE) push this package's
// peak Business Edition node usage past what the licence carries alongside
// endpoints_test.go's and stacks_test.go's own concurrent fixtures. Running
// serially caps this test's peak at one environment while still covering
// every surface and edition.
func TestEndpointGroups_Inspect_ReturnsTheGroupItWasAskedFor(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, ed)

				groupName := uniqueName("endpoint-group-inspect")
				groupID := createEndpointGroupFixture(t, ed, groupName)

				empty := callAction[map[string]any](t, session, surface, "endpoint_groups.inspect", map[string]any{
					"id": groupID,
				})
				if got, _ := empty["Id"].(float64); int(got) != groupID {
					t.Errorf("endpoint_groups.inspect(%d) returned Id %v, want %d: it answered about a different group", groupID, got, groupID)
				}
				if got, _ := empty["Name"].(string); got != groupName {
					t.Errorf("endpoint_groups.inspect(%d) returned Name %q, want %q: it answered about a different group", groupID, got, groupName)
				}

				// A real member, moved in through the raw Portainer API
				// rather than endpoint_groups.add_endpoint, so a defect in
				// that action cannot be mistaken for one in inspect
				// (rawAddEndpointToGroup's own doc comment).
				envID := createEnvironmentThroughSurface(t, surface, ed, uniqueName("env-groups-inspect"))
				rawAddEndpointToGroup(t, ed, groupID, envID)

				unsized := callAction[map[string]any](t, session, surface, "endpoint_groups.inspect", map[string]any{
					"id": groupID,
				})
				if got, ok := unsized["Total"].(float64); !ok || got != 0 {
					t.Errorf("endpoint_groups.inspect(%d) without size returned Total %v (present=%v), want 0 even though the group now holds environment %d",
						groupID, unsized["Total"], ok, envID)
				}

				sized := callAction[map[string]any](t, session, surface, "endpoint_groups.inspect", map[string]any{
					"id": groupID, "size": true,
				})
				if got, _ := sized["Id"].(float64); int(got) != groupID {
					t.Errorf("endpoint_groups.inspect(%d, size:true) returned Id %v, want %d", groupID, got, groupID)
				}
				if got, ok := sized["Total"].(float64); !ok || got != 1 {
					t.Errorf("endpoint_groups.inspect(%d, size:true) returned Total %v (present=%v), want 1: the group holds exactly environment %d, so size was not sent or not honoured",
						groupID, sized["Total"], ok, envID)
				}
			})
		}
	}
}

// TestEndpointGroups_Update_AssociatedEndpointsReplacesMembership exercises
// endpoint_groups.update against the measured behaviour its own narrative
// warns about: associatedEndpoints, when supplied, REPLACES the group's
// membership outright. A partial list evicts every unlisted member back to
// group 1, silently -- this proves both halves, by moving two environments
// in together and then asking for only one of them to stay.
//
// The group is a fixture (createEndpointGroupFixture), not the group this
// file's own lifecycle test creates: this test exercises update in
// isolation, on its own group, so a failure here cannot be confused with a
// failure in create, add_endpoint or delete.
//
// Deliberately NOT parallel across its own ed/surface subtests, unlike every
// other test in this file: it registers two fresh environments per subtest,
// and six concurrent creates (three surfaces x CE and EE) push this
// package's peak Business Edition node usage past what the licence already
// carries from endpoints_test.go's and stacks_test.go's own concurrent
// fixtures -- which is what made those PRE-EXISTING tests skip on "node
// allowance" nondeterministically once this test was added. Running
// serially caps this test's own peak at two environments while still
// covering every surface and edition; do not restore t.Parallel() to "speed
// it up" -- that is exactly the change that reintroduces the interference.
func TestEndpointGroups_Update_AssociatedEndpointsReplacesMembership(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				session := sessions.For(t, surface, ed)

				groupID := createEndpointGroupFixture(t, ed, uniqueName("endpoint-group-update"))
				keptID := createEnvironmentThroughSurface(t, surface, ed, uniqueName("env-update-kept"))
				evictedID := createEnvironmentThroughSurface(t, surface, ed, uniqueName("env-update-evicted"))

				callAction[map[string]any](t, session, surface, "endpoint_groups.update", map[string]any{
					"id": groupID, "associatedEndpoints": []int{keptID, evictedID},
				})
				if got, _ := rawEnvironment(t, ed, keptID)["GroupId"].(float64); int(got) != groupID {
					t.Fatalf("environment %d GroupId = %v after endpoint_groups.update, want %d", keptID, got, groupID)
				}
				if got, _ := rawEnvironment(t, ed, evictedID)["GroupId"].(float64); int(got) != groupID {
					t.Fatalf("environment %d GroupId = %v after endpoint_groups.update, want %d", evictedID, got, groupID)
				}

				// A second update names only keptID: evictedID is
				// unlisted, and the narrative's claim is that it is
				// silently evicted back to group 1 rather than left alone.
				callAction[map[string]any](t, session, surface, "endpoint_groups.update", map[string]any{
					"id": groupID, "associatedEndpoints": []int{keptID},
				})
				if got, _ := rawEnvironment(t, ed, keptID)["GroupId"].(float64); int(got) != groupID {
					t.Errorf("environment %d GroupId = %v after the second endpoint_groups.update, want %d (still named)", keptID, got, groupID)
				}
				if got, _ := rawEnvironment(t, ed, evictedID)["GroupId"].(float64); int(got) != 1 {
					t.Errorf("environment %d GroupId = %v after the second endpoint_groups.update, want 1 (Unassigned): "+
						"an unlisted member must be evicted, not left in place", evictedID, got)
				}
			})
		}
	}
}

// endpointGroupsSafeModeMutations is one entry per mutating action in this
// domain, following endpointsSafeModeMutations' shape (endpoints_test.go):
// inputs realistic enough that safe mode's interception is the only reason
// none of them execute.
var endpointGroupsSafeModeMutations = []struct {
	action string
	kind   string
	input  func(groupID, envID int) map[string]any
}{
	{"endpoint_groups.create", "mutating", func(int, int) map[string]any {
		return map[string]any{"name": uniqueName("endpoint-group-safe")}
	}},
	{"endpoint_groups.update", "destructive", func(groupID, _ int) map[string]any {
		return map[string]any{"id": groupID, "name": uniqueName("endpoint-group-safe-renamed")}
	}},
	{"endpoint_groups.delete", "destructive", func(groupID, _ int) map[string]any {
		return map[string]any{"id": groupID}
	}},
	{"endpoint_groups.add_endpoint", "mutating", func(groupID, envID int) map[string]any {
		return map[string]any{"id": groupID, "endpointId": envID}
	}},
	{"endpoint_groups.delete_endpoint", "destructive", func(groupID, envID int) map[string]any {
		return map[string]any{"id": groupID, "endpointId": envID}
	}},
}

// TestSafeMode_EndpointGroups_MutatingActionsArePreviewedAndNothingChanges
// proves tools.Execute intercepts every mutating action in this domain
// before its handler runs, following
// TestSafeMode_Endpoints_MutatingActionsArePreviewedAndNothingChanges'
// three properties: the answer is a preview naming the action and the kind
// its own ActionSpec flags imply, and nothing changed -- read back through
// the raw Portainer API, because a surface that previewed a call and
// executed it anyway would satisfy the first two identically.
//
// The victim group is a fixture this subtest owns (named uniquely per
// subtest, empty at the start). Four of the five rows target the
// environment the estate provisions (firstDockerEnvironmentID, reused
// rather than reimplemented, exactly as the task brief for this suite
// requires): create/update/delete never reference an environment at all,
// and add_endpoint's real effect -- moving that environment into the
// fixture group -- would be a genuine, observable change, since the group
// starts empty and that environment is not a member of it.
//
// delete_endpoint's row cannot reuse that setup. Nothing in provisioning
// ever assigns the estate's own environment to any group (no EndpointGroup
// reference anywhere in cmd/ provisioning or test/e2e/harness), so it
// starts outside the fixture group -- and a real removal from a group the
// environment was never in just sets GroupId to 1, the value it already
// has: unobservable, and satisfied identically by a broken interception
// that let the call through for real. That row is therefore given its own,
// dedicated environment, moved into the fixture group directly through the
// raw API (rawAddEndpointToGroup, not endpoint_groups.add_endpoint) before
// the safe-mode call, so a leaked delete_endpoint has something real to
// evict.
//
// Community Edition only, like TestSafeMode_Endpoints_...: safe-mode
// interception has nothing to do with edition, so it is proven against the
// leg every estate carries. Named resources only, never a total: other
// tests in this package create and delete environment groups in parallel
// with these subtests, so a count read before and after would measure them,
// not safe mode -- the exact defect already found and fixed in
// endpoints_test.go.
func TestSafeMode_EndpointGroups_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range endpointGroupsSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				groupName := uniqueName("endpoint-group-safe-existing")
				groupID := createEndpointGroupFixture(t, "CE", groupName)

				// envID is the environment this row's input names, and
				// beforeGroupID is the GroupId it must carry BEFORE the
				// safe-mode call -- read again afterward and compared, this
				// is what tells a genuine interception apart from a call
				// that reached Portainer but produced no observable change.
				// See this test's own doc comment for why delete_endpoint's
				// row needs a dedicated environment rather than the shared
				// one every other row uses.
				var envID int
				var beforeGroupID float64
				if mutation.action == "endpoint_groups.delete_endpoint" {
					envID = createEnvironmentThroughSurface(t, surface, "CE", uniqueName("env-groups-safe-victim"))
					rawAddEndpointToGroup(t, "CE", groupID, envID)
					beforeGroupID = float64(groupID)
				} else {
					envID = firstDockerEnvironmentID(t, "CE")
					got, ok := rawEnvironment(t, "CE", envID)["GroupId"].(float64)
					if !ok {
						t.Fatalf("environment %d carries no GroupId before the safe-mode call", envID)
					}
					beforeGroupID = got
				}

				input := mutation.input(groupID, envID)
				toolName, args := actionCallParams(t, surface, mutation.action, input)
				text := toolResultText(callToolExpectingSuccess(t, session, toolName, args))

				var preview map[string]any
				if err := json.Unmarshal([]byte(text), &preview); err != nil {
					t.Fatalf("decoding the safe-mode preview %q: %v", text, err)
				}
				if safeMode, _ := preview["safe_mode"].(bool); !safeMode {
					t.Fatalf("%s under safe mode = %v, want a safe_mode preview, not a real call", mutation.action, preview)
				}
				if got, _ := preview["action"].(string); got != mutation.action {
					t.Errorf("safe-mode preview action = %q, want %q", got, mutation.action)
				}
				if got, _ := preview["kind"].(string); got != mutation.kind {
					t.Errorf("safe-mode preview kind = %q, want %q (from this action's own ActionSpec flags)", got, mutation.kind)
				}
				wouldCall, ok := preview["would_call"].(map[string]any)
				if !ok {
					t.Fatalf("safe-mode preview carries no would_call: %v", preview)
				}
				for name := range input {
					if !previewNamesField(wouldCall["input_fields"], name) {
						t.Errorf("safe-mode preview's input_fields %v does not name %q, which this call sent",
							wouldCall["input_fields"], name)
					}
				}

				// Nothing changed. The fixture group this subtest owns is
				// still there, under its original name: create's input name
				// never created a second one, and update/delete never
				// touched this one.
				if got := endpointGroupIDByName(t, "CE", groupName); got != groupID {
					t.Errorf("group %q (%d) is gone or renamed: safe mode let %s through", groupName, groupID, mutation.action)
				}
				if name, ok := input["name"].(string); ok {
					if got := endpointGroupIDByName(t, "CE", name); got != -1 {
						t.Errorf("safe mode let %s through: group %q now exists as %d", mutation.action, name, got)
					}
				}
				// The target environment's GroupId is exactly what it was
				// before the call: unmoved for add_endpoint, un-evicted for
				// delete_endpoint.
				afterGroupID, ok := rawEnvironment(t, "CE", envID)["GroupId"].(float64)
				if !ok {
					t.Fatalf("environment %d carries no GroupId after the safe-mode call", envID)
				}
				if afterGroupID != beforeGroupID {
					t.Errorf("environment %d GroupId = %v, want %v: safe mode let %s through", envID, afterGroupID, beforeGroupID, mutation.action)
				}
			})
		}
	}
}
