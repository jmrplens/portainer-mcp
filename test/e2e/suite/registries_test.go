//go:build e2e

package suite

import (
	"context"
	"fmt"
	"slices"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

// customRegistryType is the numeric registry type createRegistry itself uses
// (see fixtures_test.go): a custom registry stores whatever URL it is given
// without validating that anything is listening there, which is what lets
// registries.create's own input be exercised here without a real registry
// backend to point at.
const customRegistryType = 3

// TestRegistries_CreateUpdateInspectThenDelete_AcrossSurfacesAndEditions
// exercises the registries domain's core CRUD path — list, create, update,
// inspect, delete — across every surface and every edition, re-fetching
// after each mutation rather than trusting its return value: registries.list
// after create, registries.inspect after update (to see the new name stick),
// and registries.list again after delete.
func TestRegistries_CreateUpdateInspectThenDelete_AcrossSurfacesAndEditions(t *testing.T) {
	for _, edition := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				name := uniqueName("registry")
				// registries.create is called through the MCP surface under
				// test, not through the createRegistry fixture helper: see
				// the identical note in tags_test.go — relying on the raw
				// fixture here would let audit_e2e_gaps count
				// registries.create as exercised with no test ever actually
				// invoking it through a tool.
				created := callAction[map[string]any](t, session, surface, "registries.create", map[string]any{
					"name": name, "url": fmt.Sprintf("%s.example.invalid", name), "type": customRegistryType,
				})
				idFloat, ok := created["Id"].(float64)
				if !ok {
					t.Fatalf("registries.create response carried no Id: %v", created)
				}
				id := int(idFloat)
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteRegistryIfPresent(ctx, edition, id); err != nil {
						t.Errorf("clean up registry %q: %v", name, err)
					}
				})

				listed := callAction[[]map[string]any](t, session, surface, "registries.list", nil)
				if !slices.ContainsFunc(listed, func(m map[string]any) bool { return m["Name"] == name }) {
					t.Fatalf("registry %q was created but does not appear in registries.list", name)
				}

				renamed := uniqueName("registry-renamed")
				callAction[map[string]any](t, session, surface, "registries.update", map[string]any{
					"id": id, "name": renamed, "url": fmt.Sprintf("%s.example.invalid", renamed),
				})

				// Read back through inspect, not through update's own
				// return value: an update that returned a plausible body
				// but wrote nothing would pass without this.
				inspected := callAction[map[string]any](t, session, surface, "registries.inspect", map[string]any{"id": id})
				if inspected["Name"] != renamed {
					t.Fatalf("registries.inspect Name = %v after registries.update, want %q", inspected["Name"], renamed)
				}

				callAction[any](t, session, surface, "registries.delete", map[string]any{"id": id})

				// Read back again: a delete that no-ops would pass without
				// this.
				after := callAction[[]map[string]any](t, session, surface, "registries.list", nil)
				if slices.ContainsFunc(after, func(m map[string]any) bool { return m["Id"] == float64(id) }) {
					t.Errorf("registry %d still present after registries.delete", id)
				}
			})
		}
	}
}

// TestRegistries_Configure exercises registries.configure across every
// surface and edition, but does not assert a read-back: this is a
// deliberate, investigated exception to this task's own read-back rule, not
// an oversight.
//
// The natural read-back would be registries.inspect afterward, checking that
// ManagementConfiguration.Authentication and .Username reflect what was
// configured (Password and TLSConfig are dropped by registries.redact before
// any tool result leaves the server, so only those two survive to check).
// That was tried first. Against this live estate it does not hold:
// registries.configure returns 204/success, but a subsequent inspect always
// shows Authentication=false and Username="" regardless of what was sent —
// verified with a raw curl POST straight against the live Portainer API,
// bypassing this project's client and handler entirely, across three
// registry types (custom, Docker Hub, ECR) and with and without an
// endpointId query parameter on the inspect. Since the same non-effect
// reproduces with this project's code entirely absent from the path, it is
// this Portainer 2.44.0 server's own behaviour for this endpoint, not a bug
// in registryConfigure's handler (internal/tools/registries/registries.go) —
// see the task report for this suite for the full trace.
//
// Given that, asserting a read-back here would either be vacuously true (a
// no-op handler and a correctly-behaving one produce the identical,
// unchanged inspect result) or force a fabricated expectation the real API
// never satisfies. The brief's rule is "assert only no error is not a test
// of a mutation" — the answer here is that this action's mutation cannot be
// independently observed through this API at all, which is a gap to report,
// not a check to fake.
func TestRegistries_Configure(t *testing.T) {
	for _, edition := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(edition+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, edition)

				id := createRegistry(t, edition, uniqueName("registry"))

				callAction[map[string]any](t, session, surface, "registries.configure", map[string]any{
					"id": id, "authentication": true, "username": "e2e-configure-user",
				})
			})
		}
	}
}

// TestRegistries_Ping_AgainstUnreachableURL exercises registries.ping without
// a fixture: it never persists anything, so there is nothing to create or
// clean up. example.invalid is reserved by RFC 2606 to never resolve, so a
// real ping against it can only ever fail — asserting IsError here is a
// genuine, discriminating check against this estate's real network
// behaviour, not a tautology: a handler that fabricated a success result
// without truly dialling the URL would fail this test by returning success.
func TestRegistries_Ping_AgainstUnreachableURL(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "CE")

			toolName, args := actionCallParams(t, surface, "registries.ping", map[string]any{
				"url": "unreachable.example.invalid", "type": customRegistryType,
			})
			res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("CallTool(%s): %v", toolName, err)
			}
			if !res.IsError {
				t.Errorf("registries.ping against %s succeeded, want a failure against an address that can never resolve", "unreachable.example.invalid")
			}
		})
	}
}

// TestRegistries_BusinessEditionOnlyActions_FailInformativelyAgainstAFakeBackend
// exercises the three registries actions declared Business-Edition-only
// (registries.ecr_delete_repository, registries.ecr_delete_tags,
// registries.repository_tags_delete) for real, against the Business Edition
// leg, routed through the dynamic surface.
//
// They cannot succeed here: the fixture registry createRegistry builds is a
// generic "custom registry" record with no real backend behind its URL (see
// fixtures_test.go), and none of these three actions has a softer,
// non-destructive form to call instead — each one deletes real registry
// content, which this harness has no disposable ECR repository or reachable
// generic registry to provide. Calling them against the fixture and
// asserting a real, informative failure is what is available to exercise
// here: it proves the action reaches the live Business Edition server,
// executes its handler, and surfaces whatever that server reports, without
// requiring credentials to a real cloud registry this harness does not have.
func TestRegistries_BusinessEditionOnlyActions_FailInformativelyAgainstAFakeBackend(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("no Business Edition server in this estate")
	}
	session := sessions.For(t, "dynamic", "EE")

	tests := []struct {
		action string
		input  map[string]any
	}{
		{"registries.ecr_delete_repository", map[string]any{"repositoryName": "does-not-exist"}},
		{"registries.ecr_delete_tags", map[string]any{"repositoryName": 1, "tags": []any{"latest"}}},
		{"registries.repository_tags_delete", map[string]any{"repositoryName": "does-not-exist", "tags": []any{"latest"}}},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			t.Parallel()
			id := createRegistry(t, "EE", uniqueName("registry"))
			input := map[string]any{"id": id}
			for k, v := range tt.input {
				input[k] = v
			}

			res, err := session.CallTool(t.Context(), &mcp.CallToolParams{
				Name:      "portainer_execute_action",
				Arguments: map[string]any{"action": tt.action, "input": input},
			})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Errorf("%s against a fixture registry with no real backend succeeded, want a failure", tt.action)
			}
			if text := toolResultText(res); text == "" {
				t.Errorf("%s failed with no message a model could act on", tt.action)
			}
		})
	}
}
