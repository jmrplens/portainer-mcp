//go:build e2e

package suite

import (
	"context"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// TestTags_CreateThenListThenDelete_AcrossSurfacesAndEditions is the mutation
// template every P3 domain suite copies. It runs the full tags domain — the
// smallest one that includes a genuinely destructive action — through every
// surface and every edition this estate provisions.
//
// The rule that matters most here: after every mutating call, re-fetch and
// assert the new value. A test that asserted only "no error" from
// tags.create and tags.delete would pass even if delete silently no-op'd.
// The task report for this suite records the proof: tags.delete's handler
// was temporarily made to return success without calling the API, and the
// read-back assertion below failed on all three surfaces, exactly as it
// must.
func TestTags_CreateThenListThenDelete_AcrossSurfacesAndEditions(t *testing.T) {
	for _, edition := range []string{"CE", "EE"} {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				name := uniqueName("tag")
				// tags.create is called through the MCP surface under test,
				// not through the createTag fixture helper: that helper
				// creates directly against the Portainer API, and if this
				// test relied on it, the audit_e2e_gaps scan would count
				// tags.create as exercised while no test ever actually
				// invoked it through a tool. Cleanup is still registered
				// (deleteTagIfPresent, defined in fixtures_test.go): its
				// presence check is exactly what keeps a redundant cleanup
				// after this test's own successful tags.delete below from
				// reporting a spurious failure.
				created := callAction[map[string]any](t, session, surface, "tags.create", map[string]any{"name": name})
				idFloat, ok := created["ID"].(float64)
				if !ok {
					t.Fatalf("tags.create response carried no ID: %v", created)
				}
				id := int(idFloat)
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteTagIfPresent(ctx, edition, id); err != nil {
						t.Errorf("clean up tag %q: %v", name, err)
					}
				})

				// Read back after create: a create that returned a
				// plausible-looking body but wrote nothing would pass
				// without this.
				listed := callAction[[]map[string]any](t, session, surface, "tags.list", nil)
				if !slices.ContainsFunc(listed, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Fatalf("tag %q was created but does not appear in tags.list", name)
				}

				callAction[any](t, session, surface, "tags.delete", map[string]any{"id": id})

				// Read back again: a delete that no-ops would pass without
				// this. This is the one line Step 6 of the task brief exists
				// to prove bites: stub tags.delete's handler to return
				// success without calling the API, and this assertion must
				// fail on all three surfaces.
				after := callAction[[]map[string]any](t, session, surface, "tags.list", nil)
				if slices.ContainsFunc(after, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Errorf("tag %q still present after tags.delete", name)
				}
			})
		}
	}
}

// TestTags_List_ReflectsATagCreatedDirectly proves tags.list surfaces every
// tag on the server, not only ones this same MCP session happens to have
// created: createTag (fixtures_test.go) creates the fixture directly against
// the Portainer API — the helper task 7 built for Community Edition and this
// task extends to take an edition, closing the gap noted in the task brief
// — and tags.list here must still find it.
func TestTags_List_ReflectsATagCreatedDirectly(t *testing.T) {
	for _, edition := range []string{"CE", "EE"} {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				name := uniqueName("tag")
				createTag(t, edition, name)

				listed := callAction[[]map[string]any](t, session, surface, "tags.list", nil)
				if !slices.ContainsFunc(listed, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Errorf("tag %q created directly does not appear in tags.list", name)
				}
			})
		}
	}
}

// TestTags_Create_IsRejectedWithoutAName proves the handler's own input
// validation is reachable through every surface, not just through a direct
// unit test of the handler function. An empty name is refused by
// tagCreate itself (internal/tools/tags/tags.go), before any HTTP call: this
// asserts that refusal surfaces as a tool error on each surface, rather than
// a panic or a silently-created tag.
func TestTags_Create_IsRejectedWithoutAName(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "CE")

			toolName, args := actionCallParams(t, surface, "tags.create", map[string]any{"name": ""})
			res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool(%s): %v", toolName, err)
			}
			if !res.IsError {
				t.Fatal("tags.create with an empty name succeeded, want a rejection")
			}
		})
	}
}
