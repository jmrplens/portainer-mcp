//go:build e2e

package suite

import (
	"math"
	"testing"
)

// roles is one read-only action over one route, and the whole domain's e2e
// coverage is this file: there is nothing to create, nothing to clean up and
// no fixture to build. What there is instead is an assertion that must
// DIFFER BY EDITION, and getting that shape right is the point of the file.
//
// Business Edition answers six roles; Community answers 200 with an empty
// array, because role-based access control is a Business feature and a
// Community server holds no roles at all (docs/api-divergences.md §5.5).
// Neither of the two obvious single-shaped assertions is acceptable:
//
//   - "assert a non-empty list" fails on Community for a reason that is not
//     a defect, so it would have to be skipped there — and a skipped half is
//     how a domain ends up with a single edition's coverage.
//   - "assert no error" passes on both, and would go on passing against an
//     action that returned nothing anywhere, which is precisely the failure
//     this domain is most likely to have.
//
// So each edition gets the assertion its own answer deserves, and they are
// different in kind rather than in value.

// businessRoles is what Business Edition's GET /roles answers, measured
// against a live 2.44.0 through all three tool surfaces on 2026-08-19.
//
// Written out in full, by Id AND Name, rather than asserted as a count.
// Six roles is satisfied by six of anything; this is not. Roles are
// built-in and immutable — nothing in this catalog or in Portainer's UI
// creates or deletes one — so unlike every other resource this suite reads,
// they can be pinned exactly without measuring whatever else the shared
// estate happens to hold. If Portainer ever ships a seventh, or renumbers
// these, this is where that shows up, which is the point: roles.list's
// narrative promises a model these six identifiers, and endpoints' and
// endpoint_groups' access policies take them as RoleId.
var businessRoles = map[int]string{
	1: "Environment Administrator",
	2: "Helpdesk User",
	3: "Standard User",
	4: "Read-only User",
	5: "Operator",
	6: "Namespace Operator",
}

// TestRoles_List_AnswersSixOnBusinessAndEmptyOnCommunity is the domain's
// only test, run on every provisioned edition and every surface.
//
// The edition is derived from sessions.Editions(), never a hardcoded
// []string{"CE", "EE"}: a contributor without a Business licence must still
// be able to run the Community half. That derivation has a consequence worth
// stating, because it is deliberate and it looks like a hole — a missing
// Business leg produces no EE subtests at all rather than a visible skip. It
// is not a hole this test works around, but it is one this test refuses to
// let widen silently: an edition name it has no assertion for fails the run
// rather than passing by doing nothing, so a third leg could never be
// "covered" here by omission.
func TestRoles_List_AnswersSixOnBusinessAndEmptyOnCommunity(t *testing.T) {
	editions := sessions.Editions()
	if len(editions) == 0 {
		t.Fatal("sessions.Editions() is empty: this test would pass by running nothing")
	}

	for _, ed := range editions {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				listed := callAction[[]map[string]any](t, session, surface, "roles.list", nil)

				switch ed {
				case "EE":
					// Exactly these six, by Id and Name. Checked in both
					// directions: every expected role present with the right
					// name, and no unexpected role in the answer.
					// The count is asserted before the contents, because the
					// two loops below compare sets and a set cannot see a
					// duplicate or a seventh record that collided with one of
					// the six.
					if len(listed) != len(businessRoles) {
						t.Errorf("roles.list returned %d role(s), want exactly %d: %v", len(listed), len(businessRoles), listed)
					}
					got := map[int]string{}
					for _, role := range listed {
						id, ok := role["Id"].(float64)
						if !ok {
							t.Errorf("roles.list returned an entry with no numeric Id: %v", role)
							continue
						}
						// JSON has one number type, so an Id arrives as a
						// float64 and int(id) would silently accept 1.5 as
						// role 1 — an identifier this catalog would then use
						// as if it had been given a whole number.
						if id != math.Trunc(id) {
							t.Errorf("roles.list returned role Id %v, which is not a whole number: %v", id, role)
							continue
						}
						name, ok := role["Name"].(string)
						if !ok {
							t.Errorf("roles.list returned role %v with no Name: %v", id, role)
							continue
						}
						if previous, seen := got[int(id)]; seen {
							t.Errorf("roles.list returned Id %d twice, as %q and %q; a map would have hidden one of them", int(id), previous, name)
							continue
						}
						got[int(id)] = name
					}
					for id, name := range businessRoles {
						switch have, ok := got[id]; {
						case !ok:
							t.Errorf("roles.list on Business Edition omits role %d (%q); it answered %v", id, name, got)
						case have != name:
							t.Errorf("roles.list role %d is named %q, want %q", id, have, name)
						}
					}
					for id, name := range got {
						if _, ok := businessRoles[id]; !ok {
							t.Errorf("roles.list on Business Edition returned role %d (%q), which is not one of the six built-in roles", id, name)
						}
					}

				case "CE":
					// The call must SUCCEED and answer empty. callAction
					// already fails the test on a tool error, so reaching
					// this line is the "it succeeded" half; the length is
					// the other half, and it is the assertion that would
					// catch Community suddenly growing roles — at which
					// point roles.list's narrative, and
					// docs/api-divergences.md §5.5, would both be wrong.
					if len(listed) != 0 {
						t.Errorf("roles.list on Community Edition returned %d role(s), want an empty array: "+
							"role-based access control is a Business Edition feature and Community has no roles (docs/api-divergences.md §5.5): %v",
							len(listed), listed)
					}

				default:
					t.Fatalf("no roles.list assertion for edition %q: add one rather than letting this leg pass by running nothing", ed)
				}
			})
		}
	}
}
