//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/tools/team_memberships"
	"github.com/jmrplens/portainer-mcp/internal/tools/teams"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

// The two domains this file covers ship together, and so do their tests: a
// membership names a team and a user, so nothing about team_memberships can
// be exercised without a team fixture, and splitting the two files would
// mean building that fixture twice. internal/tools/team_memberships' own
// package comment says the same thing from the other side.
//
// Every read-back here goes through the raw Portainer API rather than
// through an action in either domain, following
// endpoint_groups_test.go's own rule and for the identical reason: a create
// or update that answered with a plausible body while writing nothing would
// satisfy a read-back performed through itself, or through its own sibling.

// rawTeams reads the team list straight from the Portainer API, bypassing
// every surface, through rawEndpointsJSON (endpoints_test.go) -- generic
// over the path rather than specific to /endpoints, so it serves this
// domain's own reads too rather than being reimplemented here.
func rawTeams(t *testing.T, ed string) []map[string]any {
	t.Helper()
	var out []map[string]any
	rawEndpointsJSON(t, ed, "/teams", &out)
	return out
}

// teamIDByName finds a team by name in a raw GET /teams, or -1.
//
// By name, not by id, because the name is what a caller controls and what
// uniqueName makes unique per subtest: the estate is shared, so an
// assertion phrased as "how many teams exist" would measure every other
// subtest running beside this one. A team's name is effectively its unique
// key on the server as well -- a duplicate is refused with 409 (measured on
// both editions; see teams.create's narrative).
func teamIDByName(t *testing.T, ed, name string) int {
	t.Helper()
	for _, team := range rawTeams(t, ed) {
		if team["Name"] == name {
			if id, ok := team["Id"].(float64); ok {
				return int(id)
			}
		}
	}
	return -1
}

// rawTeam reads ONE team straight from the Portainer API (GET /teams/{id}),
// bypassing every surface.
//
// This is the raw twin of teams.inspect, and it exists because the list read
// above cannot answer every question: GET /teams is where a team's name is
// checked, but DenyPortainerAccess is a per-team flag whose read-back below
// wants the team itself, unambiguously, rather than a name match over a list
// every other subtest is concurrently adding to and removing from.
func rawTeam(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	var out map[string]any
	rawEndpointsJSON(t, ed, fmt.Sprintf("/teams/%d", id), &out)
	return out
}

// rawMembershipsOfTeam reads ONE team's memberships straight from the
// Portainer API (GET /teams/{id}/memberships), bypassing every surface.
//
// This is the raw twin of team_memberships.list_for_team, and it is what
// every membership read-back in this file asserts against. Note what an
// empty answer here does NOT prove: an id no team has answers 200 with an
// empty array rather than 404 (measured on both editions), so "empty" means
// "no memberships", never "no such team" -- teamIDByName above is what
// answers the existence question.
func rawMembershipsOfTeam(t *testing.T, ed string, teamID int) []map[string]any {
	t.Helper()
	var out []map[string]any
	rawEndpointsJSON(t, ed, fmt.Sprintf("/teams/%d/memberships", teamID), &out)
	return out
}

// membershipInt reads one integer field out of a raw membership object,
// which arrives as float64 through a generic JSON decode. The bool reports
// whether the field was present and numeric at all, so a caller can tell
// "Role 0" apart from "no Role in the answer".
func membershipInt(m map[string]any, field string) (int, bool) {
	got, ok := m[field].(float64)
	return int(got), ok
}

// membershipByID finds a membership by its own id in a raw list, or nil.
func membershipByID(memberships []map[string]any, id int) map[string]any {
	for _, m := range memberships {
		if got, ok := membershipInt(m, "Id"); ok && got == id {
			return m
		}
	}
	return nil
}

// createTeamFixture creates a team named name on edition ed's server
// directly through the Portainer API, not through teams.create -- an action
// under test elsewhere in this file. Building a fixture through the very
// action under test would let a broken create silently pass the test meant
// to catch it, the same reasoning createEndpointGroupFixture's and
// createTag's own doc comments give.
//
// TeamLeaders is deliberately never sent. It is the one and only way a team
// action itself creates a membership: measured on both editions, a create
// carrying TeamLeaders [1] left a Role 1 membership behind (see
// teams.create's narrative). Every caller here needs a team that starts
// membership-free -- the safe-mode table's team_memberships.create row
// asserts exactly that emptiness afterwards -- so this helper never offers
// the field.
//
// Retried the same way createTag and createEndpointGroupFixture are
// (fixtures_test.go): every session in the matrix creates fixtures
// concurrently against the same server, and Portainer's own name-uniqueness
// check races under that concurrency.
func createTeamFixture(t *testing.T, ed, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create team %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp, err := client.API.TeamCreateWithResponse(ctx, apigen.TeamCreateJSONRequestBody{Name: name})
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create team %q: response carried no id", name)
		}
		id = *resp.JSON200.Id
		return nil
	})

	registerTeamCleanup(t, ed, id)
	return id
}

// registerTeamCleanup arranges for team id to be removed when t ends,
// through the same ledger every other fixture in this package registers
// against (newLedger, ledger_test.go) so the order of teardown across a
// test's several fixtures stays observable and reversed.
//
// It is a function of its own rather than inlined into createTeamFixture
// because the lifecycle test below creates its team through teams.create --
// the action under test -- and still needs the identical safety net for the
// case where that test fails before reaching its own teams.delete.
func registerTeamCleanup(t *testing.T, ed string, id int) {
	t.Helper()
	newLedger(t).Register("team", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteTeamIfPresent(ctx, ed, id)
	})
}

// deleteTeamIfPresent deletes team id on edition ed's server only if a
// TeamList still reports it, so a caller that already deleted it (through
// the action under test, not through this helper) does not turn a passing
// test into a spurious cleanup failure -- the same reasoning as
// deleteTagIfPresent (fixtures_test.go), and here it is not a hypothetical:
// deleting an already-deleted team answers 404, not 204 (measured on both
// editions; see teams.delete's narrative).
func deleteTeamIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.TeamListWithResponse(ctx, &apigen.TeamListParams{})
	if err != nil {
		return fmt.Errorf("list teams before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list teams before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, team := range *resp.JSON200 {
			if team.Id != nil && *team.Id == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.TeamDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	if err := toolutil.Check(delResp); err != nil {
		return fmt.Errorf("delete team %d: %w", id, err)
	}
	return nil
}

// createUserFixture creates a standard (Role 2) user named name on edition
// ed's server directly through the Portainer API and registers its removal.
//
// A membership is the join between a user and a team, so a membership test
// needs a real user -- and it must not be the estate's own administrator
// (Id 1). Two reasons: the estate is shared, so leaving the administrator
// in a team, however briefly, changes what every other suite's caller can
// see; and this file's list_for_team test needs two memberships that differ
// in more than their team id, which one shared user cannot provide.
//
// The password is generated, never written down: generateUserPassword
// (endpoints_test.go) draws it from crypto/rand for the reasons its own doc
// comment records, chief among them that the one hard-coded literal this
// suite ever carried turned the repository's secret scan red on every run.
func createUserFixture(t *testing.T, ed, name string) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create user %q", name), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp, err := client.API.UserCreateWithResponse(ctx, apigen.UserCreateJSONRequestBody{
			Username: name,
			Password: generateUserPassword(t),
			Role:     2,
		})
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil {
			return fmt.Errorf("create user %q: response carried no body", name)
		}
		id = resp.JSON200.Id
		return nil
	})

	newLedger(t).Register("user", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteUserIfPresent(ctx, ed, id)
	})
	return id
}

// deleteUserIfPresent deletes user id on edition ed's server only if a
// UserList still reports it, for the reason deleteTeamIfPresent gives.
func deleteUserIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.UserListWithResponse(ctx, &apigen.UserListParams{})
	if err != nil {
		return fmt.Errorf("list users before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list users before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, user := range *resp.JSON200 {
			if user.Id != nil && *user.Id == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.UserDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	if err := toolutil.Check(delResp); err != nil {
		return fmt.Errorf("delete user %d: %w", id, err)
	}
	return nil
}

// createMembershipFixture joins userID to teamID with the given role,
// directly through the Portainer API rather than through
// team_memberships.create -- an action under test elsewhere in this file,
// for the reason createTeamFixture's own doc comment gives.
//
// It registers its cleanup like every other fixture here, even though
// deleting the team removes its memberships with it (measured on both
// editions; see team_memberships.delete's narrative). The ledger tears
// down in reverse registration order, so a membership registered after its
// team is removed first and the cascade has nothing left to do; and
// deleteMembershipIfPresent tolerates the row already being gone, which is
// what makes registering it harmless rather than a second failure mode.
func createMembershipFixture(t *testing.T, ed string, teamID, userID, role int) int {
	t.Helper()
	client := fixtureClient(t, ed)

	var id int
	retryFixture(t, fmt.Sprintf("create membership of team %d for user %d", teamID, userID), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp, err := client.API.TeamMembershipCreateWithResponse(ctx, apigen.TeamMembershipCreateJSONRequestBody{
			TeamID: teamID,
			UserID: userID,
			Role:   apigen.TeammembershipsTeamMembershipCreatePayloadRole(role),
		})
		if err != nil {
			return err
		}
		if err := toolutil.Check(resp); err != nil {
			return err
		}
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			return fmt.Errorf("create membership of team %d for user %d: response carried no id", teamID, userID)
		}
		id = *resp.JSON200.Id
		return nil
	})

	newLedger(t).Register("team_membership", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteMembershipIfPresent(ctx, ed, id)
	})
	return id
}

// deleteMembershipIfPresent deletes membership id on edition ed's server
// only if a TeamMembershipList still reports it.
//
// The "if present" is doing real work here rather than being defensive
// boilerplate: a membership can vanish either because the test under way
// deleted it through team_memberships.delete, or because its team was
// deleted and took it with it. Both are ordinary passing outcomes, and an
// unconditional delete would answer 404 and report a failure for a test
// that succeeded.
func deleteMembershipIfPresent(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.TeamMembershipListWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("list team memberships before cleanup: %w", err)
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("list team memberships before cleanup: %w", err)
	}
	if resp.JSON200 != nil {
		present := false
		for _, membership := range *resp.JSON200 {
			if membership.Id != nil && *membership.Id == id {
				present = true
				break
			}
		}
		if !present {
			return nil
		}
	}
	delResp, err := client.API.TeamMembershipDeleteWithResponse(ctx, id)
	if err != nil {
		return err
	}
	if err := toolutil.Check(delResp); err != nil {
		return fmt.Errorf("delete team membership %d: %w", id, err)
	}
	return nil
}

// TestTeams_TeamAndMembershipLifecycle_AcrossSurfacesAndEditions is the one
// chain the two domains share, and it is written as one chain rather than
// two because that is what the objects are: a membership cannot exist
// without a team and a user, so the interesting sequence necessarily runs
// through both packages.
//
// The sequence is the task brief's own: create a team, confirm it exists
// server-side, rename it and confirm the rename, inspect it, create a user
// to be its member, join that user to the team, confirm the membership
// server-side, raise the member's role and confirm the change, remove the
// membership, then delete the team and confirm both are gone.
//
// Every mutating call is made through the MCP surface under test; every
// read-back is raw. The one deliberate exception is teams.inspect, which is
// itself a read action under test here rather than a read-back: its
// assertion is that the answer is THIS team, by Id AND Name, which an
// inspect that ignored its id could not satisfy on a server where every
// other subtest's team carries a different unique name.
//
// The user is created directly through the Portainer API rather than
// through any action: no user domain exists in this catalog yet, so there is
// no action to create one through, and even if there were, building a
// membership's subject through a sibling domain's action would let one
// defect mask the other.
func TestTeams_TeamAndMembershipLifecycle_AcrossSurfacesAndEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				teamName := uniqueName("team")
				// teams.create goes through the surface under test, not
				// through createTeamFixture: see that helper's own doc
				// comment. Relying on the raw fixture here would let
				// cmd/audit_e2e_gaps count teams.create as referenced with
				// no test ever invoking it through a tool.
				created := callAction[map[string]any](t, session, surface, "teams.create", map[string]any{
					"name": teamName,
				})
				idFloat, ok := created["Id"].(float64)
				if !ok {
					t.Fatalf("teams.create response carried no Id: %v", created)
				}
				teamID := int(idFloat)
				registerTeamCleanup(t, ed, teamID)

				// Read back after create, raw: a create that returned a
				// plausible-looking body but wrote nothing would pass
				// without this.
				if got := teamIDByName(t, ed, teamName); got != teamID {
					t.Fatalf("team %q was created as %d but the server lists it as %d", teamName, teamID, got)
				}

				// teams.inspect answers about THIS team, on both fields.
				// An inspect that ignored its id and always answered about
				// the first team on the server would satisfy a bare success
				// check and fail this one.
				inspected := callAction[map[string]any](t, session, surface, "teams.inspect", map[string]any{
					"id": teamID,
				})
				if got, _ := inspected["Id"].(float64); int(got) != teamID {
					t.Errorf("teams.inspect(%d) returned Id %v, want %d: it answered about a different team", teamID, got, teamID)
				}
				if got, _ := inspected["Name"].(string); got != teamName {
					t.Errorf("teams.inspect(%d) returned Name %q, want %q: it answered about a different team", teamID, got, teamName)
				}

				// teams.update renames it. The DenyPortainerAccess half of
				// the same action follows the rename, below, conditioned on
				// the edition rather than skipped for depending on it.
				renamed := uniqueName("team-renamed")
				callAction[map[string]any](t, session, surface, "teams.update", map[string]any{
					"id": teamID, "name": renamed,
				})
				if got := teamIDByName(t, ed, renamed); got != teamID {
					t.Fatalf("team %d is not listed under its new name %q after teams.update (found %d)", teamID, renamed, got)
				}
				if got := teamIDByName(t, ed, teamName); got != -1 {
					t.Errorf("team %q is still listed under its old name after teams.update (as %d)", teamName, got)
				}

				assertDenyPortainerAccessIsBusinessOnly(t, session, surface, ed, teamID, renamed)

				userID := createUserFixture(t, ed, uniqueName("team-user"))

				// The membership starts at Role 2, an ordinary member, so
				// the update below has somewhere to move it to. A
				// membership created at Role 1 would already be in the
				// state the update asks for, and the role assertion could
				// then pass against an update that did nothing at all.
				membership := callAction[map[string]any](t, session, surface, "team_memberships.create", map[string]any{
					"teamId": teamID, "userId": userID, "role": 2,
				})
				membershipIDFloat, ok := membership["Id"].(float64)
				if !ok {
					t.Fatalf("team_memberships.create response carried no Id: %v", membership)
				}
				membershipID := int(membershipIDFloat)
				newLedger(t).Register("team_membership", strconv.Itoa(membershipID), func(ctx context.Context) error {
					return deleteMembershipIfPresent(ctx, ed, membershipID)
				})

				// Read back raw, through the team's own membership route.
				// This is the assertion a create that wrote nothing fails.
				afterCreate := membershipByID(rawMembershipsOfTeam(t, ed, teamID), membershipID)
				if afterCreate == nil {
					t.Fatalf("membership %d is absent from GET /teams/%d/memberships after team_memberships.create", membershipID, teamID)
				}
				if got, ok := membershipInt(afterCreate, "UserID"); !ok || got != userID {
					t.Errorf("membership %d carries UserID %v (present=%v), want %d", membershipID, afterCreate["UserID"], ok, userID)
				}
				if got, ok := membershipInt(afterCreate, "TeamID"); !ok || got != teamID {
					t.Errorf("membership %d carries TeamID %v (present=%v), want %d", membershipID, afterCreate["TeamID"], ok, teamID)
				}
				if got, ok := membershipInt(afterCreate, "Role"); !ok || got != 2 {
					t.Fatalf("membership %d carries Role %v (present=%v), want 2: the role update below would prove nothing from any other starting point",
						membershipID, afterCreate["Role"], ok)
				}

				// Raise the member to team leader. All three of UserID,
				// TeamID and Role are required by the payload, so the two
				// that are not changing are sent back unchanged --
				// team_memberships.update's own narrative says so.
				callAction[map[string]any](t, session, surface, "team_memberships.update", map[string]any{
					"id": membershipID, "teamId": teamID, "userId": userID, "role": 1,
				})

				// This is the line the task brief's mutation step exists to
				// prove bites: make team_memberships.update ignore its role
				// field, and this assertion must fail on all three surfaces
				// and both editions.
				afterUpdate := membershipByID(rawMembershipsOfTeam(t, ed, teamID), membershipID)
				if afterUpdate == nil {
					t.Fatalf("membership %d is absent from GET /teams/%d/memberships after team_memberships.update", membershipID, teamID)
				}
				if got, ok := membershipInt(afterUpdate, "Role"); !ok || got != 1 {
					t.Errorf("membership %d carries Role %v (present=%v) after team_memberships.update, want 1 (team leader)",
						membershipID, afterUpdate["Role"], ok)
				}
				// The update must not have moved the membership to some
				// other user or team on its way: it is a role change, and
				// a handler that rebuilt the row from a partly-empty
				// payload would show up here as a UserID or TeamID of 0.
				if got, ok := membershipInt(afterUpdate, "UserID"); !ok || got != userID {
					t.Errorf("membership %d carries UserID %v (present=%v) after team_memberships.update, want %d unchanged",
						membershipID, afterUpdate["UserID"], ok, userID)
				}
				if got, ok := membershipInt(afterUpdate, "TeamID"); !ok || got != teamID {
					t.Errorf("membership %d carries TeamID %v (present=%v) after team_memberships.update, want %d unchanged",
						membershipID, afterUpdate["TeamID"], ok, teamID)
				}

				callAction[any](t, session, surface, "team_memberships.delete", map[string]any{
					"id": membershipID,
				})
				if got := membershipByID(rawMembershipsOfTeam(t, ed, teamID), membershipID); got != nil {
					t.Errorf("membership %d is still present after team_memberships.delete: %v", membershipID, got)
				}

				callAction[any](t, session, surface, "teams.delete", map[string]any{"id": teamID})

				// Read back raw again: a delete that no-ops would pass
				// without this.
				if got := teamIDByName(t, ed, renamed); got != -1 {
					t.Errorf("team %q is still present as %d after teams.delete", renamed, got)
				}
			})
		}
	}
}

// assertDenyPortainerAccessIsBusinessOnly exercises the one behavioural
// claim this domain makes that differs by edition, on both editions.
//
// It is measured, not documented: docs/api-divergences.md records that the
// Business document declares DenyPortainerAccess on teams.teamCreatePayload
// and teams.teamUpdatePayload while the Community document declares neither,
// and that a live Community server answers 200, applies the name, and leaves
// the flag false. Both teams.create's and teams.update's narratives promise
// that behaviour to callers. Without this, the claim those two narratives
// make most loudly would be the one claim in the domain with no regression
// test behind it -- and if Community ever started honouring the flag, or
// Business stopped, nothing here would notice.
//
// Conditioned on the edition rather than skipped for depending on one: the
// idiom endpoints_test.go (`if ed != "EE"`) and stacks_test.go
// (`if ed == "EE"`) already use. What each edition must do differs in kind,
// not merely in value, and both halves were measured live before being
// written down:
//
//   - EE: the call succeeds and the flag really lands, so the raw read-back
//     is true. If Business ever stops applying it, this fails.
//
//   - CE: the call never reaches Portainer at all. teamUpdateInput tags the
//     field `edition:"EE"`; actioncatalog.Build prunes it out of the
//     Community input schema, and ValidateInput refuses it as an unexpected
//     additional property before any handler runs. Measured through the
//     dynamic surface against the live Community leg:
//
//     teams.update: validating root: unexpected additional properties
//     ["denyPortainerAccess"]
//
//     That is a better outcome than the silent no-op Portainer itself would
//     give -- a Community caller is told rather than left guessing -- and it
//     is what a Community caller actually experiences, so it is what this
//     asserts.
//
// The raw flag is read on BOTH editions regardless of which branch ran, and
// that is deliberate: it makes the two failure modes independent. Lose the
// edition pruning and the refusal assertion fails; have Community start
// honouring the flag and the "still false" assertion fails, even though the
// refusal it is paired with would still hold.
//
// It goes through actionCallParams and a direct CallTool rather than
// callAction because callAction's own callTool fails the test on any
// IsError result, which is exactly what the Community half needs to inspect
// rather than treat as a harness failure -- the reason actionCallParams
// exists at all (see its doc comment).
func assertDenyPortainerAccessIsBusinessOnly(t *testing.T, session *mcp.ClientSession, surface, ed string, teamID int, currentName string) {
	t.Helper()

	// No name in the payload, which pins the partial-update half at the same
	// time: a PUT carrying DenyPortainerAccess alone must leave the name
	// exactly as it was (measured on both editions, teams.update's
	// narrative).
	toolName, args := actionCallParams(t, surface, "teams.update", map[string]any{
		"id": teamID, "denyPortainerAccess": true,
	})
	res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("teams.update with denyPortainerAccess on %s: %v", ed, err)
	}
	text := toolResultText(res)

	if ed == "EE" {
		if res.IsError {
			t.Fatalf("teams.update with denyPortainerAccess was refused on Business Edition, where the field is declared: %s", text)
		}
		// Only asserted on the leg whose call actually ran: on Community the
		// call never reached Portainer, so "the name is unchanged" there
		// would be an assertion that could not fail.
		if got, _ := rawTeam(t, ed, teamID)["Name"].(string); got != currentName {
			t.Errorf("team %d is named %q after a teams.update carrying only denyPortainerAccess, want %q unchanged: "+
				"that update is partial, not a replace", teamID, got, currentName)
		}
	} else if !res.IsError {
		t.Errorf("teams.update accepted denyPortainerAccess on %s, where teamUpdateInput's edition:\"EE\" tag should have "+
			"pruned the field out of the schema and ValidateInput should have refused it: %s", ed, text)
	} else if !strings.Contains(text, "denyPortainerAccess") {
		t.Errorf("teams.update on %s was refused, but not for the denyPortainerAccess field this call sent: %s", ed, text)
	}

	wantDeny := ed == "EE"
	got, ok := rawTeam(t, ed, teamID)["DenyPortainerAccess"].(bool)
	if !ok {
		t.Fatalf("team %d carries no DenyPortainerAccess on %s: %v", teamID, ed, rawTeam(t, ed, teamID))
	}
	if got != wantDeny {
		t.Errorf("team %d reads DenyPortainerAccess %v on %s after teams.update asked for true, want %v: "+
			"Business applies this flag and Community does not (docs/api-divergences.md)", teamID, got, ed, wantDeny)
	}
}

// TestTeams_List_ReflectsATeamCreatedDirectly proves teams.list surfaces
// every team on the server, not only ones this same MCP session happens to
// have created: createTeamFixture creates directly against the Portainer
// API, the same relationship tags_test.go's own
// TestTags_List_ReflectsATagCreatedDirectly has with createTag, and
// teams.list here must still find it.
//
// It asserts the team's presence by name and nothing else -- never a count.
// The estate is shared and every other subtest in this package creates and
// deletes teams beside this one, so a length assertion would measure them
// rather than teams.list. DenyPortainerAccess is not asserted here because
// this fixture never sets it, not because it is edition-dependent: the
// lifecycle test above exercises that flag on both editions, conditionally
// (assertDenyPortainerAccessIsBusinessOnly).
func TestTeams_List_ReflectsATeamCreatedDirectly(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("team-list")
				createTeamFixture(t, ed, name)

				listed := callAction[[]map[string]any](t, session, surface, "teams.list", nil)
				if !slices.ContainsFunc(listed, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Errorf("team %q created directly does not appear in teams.list", name)
				}
			})
		}
	}
}

// TestTeamMemberships_ListForTeam_IsNotList_AcrossSurfacesAndEditions is
// the assertion this whole file exists for, and the reason
// team_memberships.list_for_team needed a name of its own.
//
// Two actions in one domain are both called "list something":
//
//   - team_memberships.list (GET /team_memberships) returns EVERY membership
//     on the server.
//   - team_memberships.list_for_team (GET /teams/{id}/memberships) returns
//     the memberships of ONE team.
//
// Their operationIds differ (TeamMembershipList and the bare
// TeamMemberships), and cmd/gen_action_inputs's actionNameOverrides table
// supplies "list_for_team" for the second because the mechanical rule would
// have minted "team_memberships.team_memberships". A single-team test
// cannot tell the two apart: with one team on the server holding one
// membership, an implementation of list_for_team that ignored its id
// entirely and returned the server-wide list would pass.
//
// So there are two teams here, each with its own membership and its own
// user, and three assertions that no id-ignoring implementation survives:
// list_for_team on the first team returns the first team's membership,
// does NOT return the second's, and returns nothing belonging to any other
// team at all -- while list, asked the same question at the same moment,
// returns both.
//
// list is server-wide, so its assertion is containment of two named
// memberships and never a count: other subtests in this package hold
// memberships of their own while this one runs, and a length assertion
// would measure them. list_for_team's assertions are stronger because they
// can be: this subtest owns both teams and nothing else ever adds a
// membership to them.
func TestTeamMemberships_ListForTeam_IsNotList_AcrossSurfacesAndEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				firstTeam := createTeamFixture(t, ed, uniqueName("team-scope-first"))
				secondTeam := createTeamFixture(t, ed, uniqueName("team-scope-second"))
				// Two users rather than one in both teams: it makes the two
				// memberships differ in UserID as well as TeamID, so a
				// list_for_team that filtered on the wrong field is caught
				// too, not only one that filtered on nothing.
				firstUser := createUserFixture(t, ed, uniqueName("team-scope-user-first"))
				secondUser := createUserFixture(t, ed, uniqueName("team-scope-user-second"))

				firstMembership := createMembershipFixture(t, ed, firstTeam, firstUser, 2)
				secondMembership := createMembershipFixture(t, ed, secondTeam, secondUser, 2)

				scoped := callAction[[]map[string]any](t, session, surface, "team_memberships.list_for_team", map[string]any{
					"id": firstTeam,
				})
				if membershipByID(scoped, firstMembership) == nil {
					t.Errorf("team_memberships.list_for_team(%d) omits membership %d, which belongs to that team: %v",
						firstTeam, firstMembership, scoped)
				}
				if got := membershipByID(scoped, secondMembership); got != nil {
					t.Errorf("team_memberships.list_for_team(%d) returned membership %d, which belongs to team %d: "+
						"it is answering with the server-wide list rather than this team's",
						firstTeam, secondMembership, secondTeam)
				}
				for _, m := range scoped {
					if got, ok := membershipInt(m, "TeamID"); !ok || got != firstTeam {
						t.Errorf("team_memberships.list_for_team(%d) returned an entry with TeamID %v (present=%v): %v",
							firstTeam, m["TeamID"], ok, m)
					}
				}

				// The same two memberships, asked of the server-wide list at
				// the same moment. This is the half that proves the two
				// actions really are different: if list_for_team's answer
				// above were the server-wide list, this would be identical
				// to it.
				all := callAction[[]map[string]any](t, session, surface, "team_memberships.list", nil)
				if membershipByID(all, firstMembership) == nil {
					t.Errorf("team_memberships.list omits membership %d (team %d)", firstMembership, firstTeam)
				}
				if membershipByID(all, secondMembership) == nil {
					t.Errorf("team_memberships.list omits membership %d (team %d), so it is not the server-wide list it claims to be",
						secondMembership, secondTeam)
				}
			})
		}
	}
}

// teamsSafeModeMutations is one entry per mutating action across both
// domains, following endpointGroupsSafeModeMutations' shape: inputs
// realistic enough that safe mode's interception is the only reason none of
// them execute.
//
// Six rows, not the eight the task brief guessed at: teams declares five
// actions of which create, update and delete mutate, and team_memberships
// declares five of which create, update and delete mutate. list, inspect and
// list_for_team are reads and safe mode never intercepts them. The count is
// derived from the two domains' own ActionSpec flags, and
// TestSafeMode_Teams_TableCoversEveryMutatingAction below fails if a seventh
// mutating action is ever added without a row here.
//
// needsMembership marks the two rows whose input names a membership id. They
// get a real membership at Role 2 created for them; every other row gets a
// team that is deliberately membership-free, which is what makes the
// team_memberships.create row's "still empty afterwards" assertion mean
// something.
var teamsSafeModeMutations = []struct {
	action          string
	kind            string
	needsMembership bool
	input           func(teamID, userID, membershipID int) map[string]any
}{
	{"teams.create", "mutating", false, func(int, int, int) map[string]any {
		return map[string]any{"name": uniqueName("team-safe")}
	}},
	{"teams.update", "mutating", false, func(teamID, _, _ int) map[string]any {
		return map[string]any{"id": teamID, "name": uniqueName("team-safe-renamed")}
	}},
	{"teams.delete", "destructive", false, func(teamID, _, _ int) map[string]any {
		return map[string]any{"id": teamID}
	}},
	{"team_memberships.create", "mutating", false, func(teamID, userID, _ int) map[string]any {
		return map[string]any{"teamId": teamID, "userId": userID, "role": 2}
	}},
	// Role 1 against a membership that really is at Role 2: a preview row
	// whose victim is already in the state the call asks for cannot fail,
	// and this stage has already caught one such assertion.
	{"team_memberships.update", "mutating", true, func(teamID, userID, membershipID int) map[string]any {
		return map[string]any{"id": membershipID, "teamId": teamID, "userId": userID, "role": 1}
	}},
	{"team_memberships.delete", "destructive", true, func(_, _, membershipID int) map[string]any {
		return map[string]any{"id": membershipID}
	}},
}

// TestSafeMode_Teams_MutatingActionsArePreviewedAndNothingChanges proves
// tools.Execute intercepts every mutating action in both domains before its
// handler runs, following
// TestSafeMode_EndpointGroups_MutatingActionsArePreviewedAndNothingChanges'
// three properties: the answer is a preview naming the action and the kind
// its own ActionSpec flags imply, and nothing changed -- read back through
// the raw Portainer API, because a surface that previewed a call and
// executed it anyway would satisfy the first two identically.
//
// Every row's would-be effect is observable, deliberately:
//
//   - teams.create names a team that does not exist, so a leak creates it.
//   - teams.update renames the victim team, so a leak changes its name.
//   - teams.delete targets the victim team, so a leak removes it.
//   - team_memberships.create names the victim team, which starts
//     membership-free (createTeamFixture never sends TeamLeaders), so a leak
//     adds the one membership it has.
//   - team_memberships.update asks for Role 1 against a membership at Role
//     2, so a leak changes the role. Asking for the role it already has
//     would be a row that could not fail.
//   - team_memberships.delete targets that same real membership, so a leak
//     removes it.
//
// Community Edition only, like the endpoint_groups safe-mode table: safe
// mode's interception has nothing to do with edition, so it is proven
// against the leg every estate carries. Named resources only, never a total:
// other subtests in this package create and delete teams, users and
// memberships in parallel with these, so a count read before and after would
// measure them, not safe mode.
func TestSafeMode_Teams_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range teamsSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				teamName := uniqueName("team-safe-existing")
				teamID := createTeamFixture(t, "CE", teamName)
				userID := createUserFixture(t, "CE", uniqueName("team-safe-user"))

				membershipID := -1
				if mutation.needsMembership {
					membershipID = createMembershipFixture(t, "CE", teamID, userID, 2)
				}

				// The victim team's membership list BEFORE the safe-mode
				// call. Compared against the same read afterwards, this is
				// what tells a genuine interception apart from a call that
				// reached Portainer and produced no observable change: it
				// covers the create row (empty must stay empty), the update
				// row (Role 2 must stay Role 2) and the delete row (the
				// membership must still be there) with one comparison.
				before := membershipRoles(t, "CE", teamID)
				if mutation.needsMembership {
					if got, ok := before[membershipID]; !ok || got != 2 {
						t.Fatalf("membership %d reads Role %v (present=%v) before the safe-mode call, want 2: "+
							"this row's input asks for Role 1, and a victim already in that state could not fail", membershipID, got, ok)
					}
				} else if len(before) != 0 {
					t.Fatalf("team %d holds %d membership(s) before the safe-mode call, want none: "+
						"the team_memberships.create row proves nothing against a team that already has one", teamID, len(before))
				}

				input := mutation.input(teamID, userID, membershipID)
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

				// Nothing changed. The victim team is still there under its
				// original name: create's input name never created a second
				// one, and update never renamed nor delete removed this one.
				if got := teamIDByName(t, "CE", teamName); got != teamID {
					t.Errorf("team %q (%d) is gone or renamed: safe mode let %s through", teamName, teamID, mutation.action)
				}
				if name, ok := input["name"].(string); ok {
					if got := teamIDByName(t, "CE", name); got != -1 {
						t.Errorf("safe mode let %s through: team %q now exists as %d", mutation.action, name, got)
					}
				}

				// The victim team's memberships are exactly what they were:
				// same ids, same roles.
				after := membershipRoles(t, "CE", teamID)
				if len(after) != len(before) {
					t.Errorf("team %d holds %d membership(s) after the safe-mode call, want %d: safe mode let %s through",
						teamID, len(after), len(before), mutation.action)
				}
				for id, role := range before {
					got, ok := after[id]
					if !ok {
						t.Errorf("membership %d of team %d is gone: safe mode let %s through", id, teamID, mutation.action)
						continue
					}
					if got != role {
						t.Errorf("membership %d of team %d reads Role %d, want %d: safe mode let %s through",
							id, teamID, got, role, mutation.action)
					}
				}
			})
		}
	}
}

// membershipRoles reads one team's memberships raw and returns them as
// membership id to role, which is the shape the safe-mode table compares
// before against after. It is scoped to a single team the caller owns, so it
// is a named-resource read rather than a server-wide count.
func membershipRoles(t *testing.T, ed string, teamID int) map[int]int {
	t.Helper()
	roles := map[int]int{}
	for _, m := range rawMembershipsOfTeam(t, ed, teamID) {
		id, idOK := membershipInt(m, "Id")
		role, roleOK := membershipInt(m, "Role")
		if !idOK || !roleOK {
			t.Fatalf("membership of team %d carries no Id or Role: %v", teamID, m)
		}
		roles[id] = role
	}
	return roles
}

// TestSafeMode_Teams_TableCoversEveryMutatingAction fails when a mutating
// action is added to either domain without a row in
// teamsSafeModeMutations.
//
// The table above is a hand-written list, and a hand-written list of what a
// domain declares goes stale silently: a seventh mutating action would
// simply never be previewed by anything, and every test in this file would
// stay green. This derives the expected set from the domains' own
// ActionSpecs -- the same source safeModePreview reads Destructive from --
// so the table cannot fall behind the catalog without something failing.
//
// It also checks the other direction: a row naming an action that is not
// mutating at all would be asserting that safe mode intercepts something
// safe mode never sees.
func TestSafeMode_Teams_TableCoversEveryMutatingAction(t *testing.T) {
	t.Parallel()

	want := map[string]string{}
	for _, spec := range append(teams.Specs(), team_memberships.Specs()...) {
		if !spec.Mutating {
			continue
		}
		kind := "mutating"
		if spec.Destructive {
			kind = "destructive"
		}
		want[spec.Name] = kind
	}

	got := map[string]string{}
	for _, mutation := range teamsSafeModeMutations {
		got[mutation.action] = mutation.kind
	}

	for name, kind := range want {
		switch have, ok := got[name]; {
		case !ok:
			t.Errorf("%s is a mutating action with no row in teamsSafeModeMutations: safe mode's interception of it is unproven", name)
		case have != kind:
			t.Errorf("teamsSafeModeMutations declares %s as %q, but its ActionSpec flags imply %q", name, have, kind)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("teamsSafeModeMutations has a row for %s, which is not a mutating action in either domain", name)
		}
	}
}
