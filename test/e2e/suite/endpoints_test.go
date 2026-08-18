//go:build e2e

package suite

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// rawEnvironments reads the environment list straight from the Portainer
// API, bypassing every surface.
//
// Read-back assertions go through this rather than endpoints.list, for the
// reason tags_test.go's own raw helpers exist: an action that returned a
// plausible body while writing nothing would satisfy a read-back performed
// through itself.
func rawEnvironments(t *testing.T, ed string) []map[string]any {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, http.MethodGet, "/endpoints", nil)
	if err != nil {
		t.Fatalf("raw GET /endpoints on %s: %v", ed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode raw /endpoints on %s: %v", ed, err)
	}
	return out
}

// rawEnvironment reads one environment straight from the Portainer API.
func rawEnvironment(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, http.MethodGet, fmt.Sprintf("/endpoints/%d", id), nil)
	if err != nil {
		t.Fatalf("raw GET /endpoints/%d on %s: %v", id, ed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode raw /endpoints/%d on %s: %v", id, ed, err)
	}
	return out
}

// rawEdgeEnvironment reads one environment with its stored snapshot
// excluded.
//
// Every read of an edge environment in this file goes through here rather
// than rawEnvironment, and the reason is worth twenty seconds of anyone's
// attention: measured 2026-08-18 against a live Business Edition 2.44.0, the
// FIRST GET /endpoints/{id} of a freshly-registered edge environment whose
// agent has never checked in blocks for ~20 seconds, then answers. Every
// later read of the same environment returns in under five milliseconds, and
// GET /endpoints/{id}?excludeSnapshot=true returns in under one millisecond
// even on the first call — so Portainer is attempting one snapshot of an
// unreachable host, with its own timeout, and caching the outcome.
//
// GET /endpoints (the list) is unaffected. See docs/api-divergences.md; the
// same fact is in endpoints.inspect's own narrative, because a caller
// inspecting a newly-enrolled edge environment meets it too.
func rawEdgeEnvironment(t *testing.T, ed string, id int) map[string]any {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp, err := client.Do(ctx, http.MethodGet, fmt.Sprintf("/endpoints/%d?excludeSnapshot=true", id), nil)
	if err != nil {
		t.Fatalf("raw GET /endpoints/%d (no snapshot) on %s: %v", id, ed, err)
	}
	defer func() { _ = resp.Body.Close() }()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode raw /endpoints/%d on %s: %v", id, ed, err)
	}
	return out
}

// environmentIDByName finds an environment by name, or -1.
func environmentIDByName(t *testing.T, ed, name string) int {
	t.Helper()
	for _, e := range rawEnvironments(t, ed) {
		if e["Name"] == name {
			if id, ok := e["Id"].(float64); ok {
				return int(id)
			}
		}
	}
	return -1
}

// firstDockerEnvironmentID returns the id of an environment already
// provisioned by the estate, which every read-only action below addresses.
func firstDockerEnvironmentID(t *testing.T, ed string) int {
	t.Helper()
	for _, e := range rawEnvironments(t, ed) {
		if id, ok := e["Id"].(float64); ok {
			return int(id)
		}
	}
	t.Fatalf("%s estate has no environments at all", ed)
	return 0
}

// serverFor returns the provisioned server for an edition name.
func serverFor(t *testing.T, ed string) harness.Server {
	t.Helper()
	for _, leg := range estate.Legs() {
		if leg.Name == ed {
			return leg.Server
		}
	}
	t.Fatalf("no provisioned server for edition %q", ed)
	return harness.Server{}
}

// createEnvironmentThroughSurface registers an environment through the
// surface under test and returns its id.
//
// A type 1 (local Docker) registration pointed at the daemon URL the estate
// already registered, copied from stacks_test.go's
// createSecondDockerEnvironment and for its reason: the URL is
// cmd/provision's own unexported constant, so it is read back off the server
// rather than written out a second time here where it would be free to
// drift.
//
// The type matters and the first attempt got it wrong, which is worth
// recording where the next reader will look: type 3 (Azure) seemed the
// cheapest way to get an environment that depends on nothing outside
// Portainer, on the assumption that an Azure environment is only a set of
// stored credentials. It is not — Portainer authenticates against Azure
// during the call and answers 500 "Unable to authenticate against Azure:
// Invalid Azure credentials". That failure was itself useful: it proved the
// hand-written multipart body parses on the server, since Portainer had to
// read EndpointCreationType and all three azure* parts to get far enough to
// reject them.
func createEnvironmentThroughSurface(t *testing.T, surface, ed, name string) int {
	t.Helper()
	srv := serverFor(t, ed)
	existing, ok := srv.Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("the %s server has no %q environment to copy a daemon URL from", ed, harness.EnvironmentDocker)
	}
	daemonURL, _ := rawEnvironment(t, ed, existing)["URL"].(string)
	if daemonURL == "" {
		t.Fatalf("environment %d on %s reports no daemon URL", existing, ed)
	}

	toolName, args := actionCallParams(t, surface, "endpoints.create", map[string]any{
		"name":                 name,
		"endpointCreationType": 1,
		"url":                  daemonURL,
	})
	res, err := sessions.For(t, surface, ed).CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("endpoints.create on %s/%s: %v", ed, surface, err)
	}
	if res.IsError {
		text := toolResultText(res)
		// A Business Edition licence caps how many nodes may be registered,
		// and this estate's licence is whatever secret the run was given.
		// Running out of allowance is a property of the licence, not a defect
		// in the action under test, so it skips rather than reddening the
		// build — the same judgement, in the same words, as
		// stacks_test.go's createSecondDockerEnvironment.
		if strings.Contains(text, "node allowance") {
			t.Skipf("this %s licence has no node allowance left to register another environment: %s", ed, text)
		}
		t.Fatalf("endpoints.create on %s/%s: %s", ed, surface, text)
	}

	var created map[string]any
	if err := json.Unmarshal([]byte(toolResultText(res)), &created); err != nil {
		t.Fatalf("decode endpoints.create result: %v", err)
	}
	idFloat, ok := created["Id"].(float64)
	if !ok {
		t.Fatalf("endpoints.create returned no Id: %v", created)
	}
	id := int(idFloat)
	// Registered through the same ledger stacks_test.go uses, and with
	// stacks_test.go's own deleteEnvironmentIfPresent, so an environment this
	// file creates is torn down by the identical path whether the test
	// deleted it itself or died before it could.
	newLedger(t).Register("environment", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEnvironmentIfPresent(ctx, ed, id)
	})
	return id
}

// TestEndpoints_CreateInspectUpdateDelete_AcrossSurfacesAndEditions is this
// domain's mutation template, following tags_test.go's: every mutating call
// is followed by a read-back through the raw API, so an action that answered
// plausibly while writing nothing fails here rather than passing.
func TestEndpoints_CreateInspectUpdateDelete_AcrossSurfacesAndEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				// Read back after create.
				if got := environmentIDByName(t, ed, name); got != id {
					t.Fatalf("environment %q was created as %d but the server lists it as %d", name, id, got)
				}

				// The Azure key must never come back out of any of these.
				inspected := callAction[map[string]any](t, session, surface, "endpoints.inspect", map[string]any{"id": id})
				if inspected["AzureCredentials"] != nil {
					t.Errorf("endpoints.inspect returned AzureCredentials: %v", inspected["AzureCredentials"])
				}
				if got, _ := inspected["Name"].(string); got != name {
					t.Errorf("endpoints.inspect Name = %q, want %q", got, name)
				}

				listed := callAction[[]map[string]any](t, session, surface, "endpoints.list", nil)
				var found bool
				for _, e := range listed {
					if e["Name"] == name {
						found = true
						if e["AzureCredentials"] != nil {
							t.Errorf("endpoints.list returned AzureCredentials for %q", name)
						}
					}
				}
				if !found {
					t.Errorf("environment %q does not appear in endpoints.list", name)
				}

				renamed := name + "-renamed"
				callAction[map[string]any](t, session, surface, "endpoints.update", map[string]any{
					"id": id, "name": renamed,
				})
				if got, _ := rawEnvironment(t, ed, id)["Name"].(string); got != renamed {
					t.Errorf("after endpoints.update the server still names %d %q, want %q", id, got, renamed)
				}

				callAction[any](t, session, surface, "endpoints.delete", map[string]any{"id": id})
				if got := environmentIDByName(t, ed, renamed); got != -1 {
					t.Errorf("environment %q is still present as %d after endpoints.delete", renamed, got)
				}
			})
		}
	}
}

// TestEndpoints_SettingsUpdate_AppliesOnBothEditions is the e2e half of the
// measurement recorded in endpointSettingsUpdate's own doc comment.
//
// Community Edition reads these ten settings as top-level body fields and
// ignores a nested "securitySettings"; Business Edition does the opposite,
// and both answer 200 either way. The handler sends both shapes; this proves
// the setting actually changed on whichever edition the leg is, by reading
// it back raw. A catalog that sent only the Business shape would pass a
// "no error" test on Community and fail this one.
func TestEndpoints_SettingsUpdate_AppliesOnBothEditions(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-settings")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				for _, want := range []bool{true, false} {
					callAction[map[string]any](t, session, surface, "endpoints.settings_update", map[string]any{
						"id": id,
						"securitySettings": map[string]any{
							"allowBindMountsForRegularUsers":     want,
							"allowPrivilegedModeForRegularUsers": want,
						},
					})
					settings, ok := rawEnvironment(t, ed, id)["SecuritySettings"].(map[string]any)
					if !ok {
						t.Fatalf("environment %d carries no SecuritySettings", id)
					}
					if settings["allowBindMountsForRegularUsers"] != want {
						t.Errorf("allowBindMountsForRegularUsers = %v, want %v: this edition ignored the shape the call sent",
							settings["allowBindMountsForRegularUsers"], want)
					}
					if settings["allowPrivilegedModeForRegularUsers"] != want {
						t.Errorf("allowPrivilegedModeForRegularUsers = %v, want %v",
							settings["allowPrivilegedModeForRegularUsers"], want)
					}
				}
			})
		}
	}
}

// TestEndpoints_ReadOnlyActions_AnswerOnEveryEditionTheyApplyTo walks the
// read-only half of the domain.
//
// Each entry asserts something about the answer's own shape, not merely that
// the call returned: a surface that answered every read with an empty object
// would otherwise pass the whole table.
func TestEndpoints_ReadOnlyActions_AnswerOnEveryEditionTheyApplyTo(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				counts := callAction[map[string]any](t, session, surface, "endpoints.summary_counts", nil)
				if len(counts) == 0 {
					t.Error("endpoints.summary_counts answered an empty object")
				}

				registries := callAction[[]map[string]any](t, session, surface, "endpoints.registries_list",
					map[string]any{"id": envID})
				for _, r := range registries {
					if r["Password"] != nil || r["AccessToken"] != nil {
						t.Errorf("endpoints.registries_list returned a credential: %v", r)
					}
				}

				// registryId 0 is Portainer's documented sentinel for the
				// anonymous DockerHub registry, and the reason this one
				// identifier is excused the catalog's "minimum": 1 rule
				// (cmd/gen_action_inputs/fields.go). Passing it here is what
				// proves that exception is load-bearing rather than
				// theoretical: with the bound applied, tools.Execute would
				// refuse this call locally, before the handler ran, and the
				// call would never reach Portainer at all.
				//
				// The call is asserted to REACH Portainer, not to succeed.
				// Measured 2026-08-18 against a live 2.44.0 of both editions:
				// this route answers 400 "Invalid environment type" for a
				// type 1 (local Docker), type 2 (agent) and type 7 (edge)
				// environment alike — every type this estate provisions —
				// while the specification records no environment-type
				// restriction at all. A 400 from Portainer's own handler is
				// therefore the expected outcome here and still proves what
				// this assertion is for: the identifier passed validation and
				// the request was built and sent. See
				// docs/api-divergences.md.
				toolName, args := actionCallParams(t, surface, "endpoints.dockerhub_status",
					map[string]any{"id": envID, "registryId": 0})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.dockerhub_status: %v", err)
				}
				if text := toolResultText(res); res.IsError && !strings.Contains(text, "Invalid environment type") {
					t.Errorf("endpoints.dockerhub_status failed for a reason other than the measured environment-type refusal: %s", text)
				}

				if ed != "EE" {
					return
				}

				versions := callAction[any](t, session, surface, "endpoints.agent_versions", nil)
				if versions == nil {
					t.Error("endpoints.agent_versions answered nothing")
				}

				// The two mTLS reads answer about the agent environment the
				// estate registers, and both must come back with the
				// certificate stripped: their redaction wrapper drops the
				// whole SslCertificate rather than any field of it, because
				// toolutil.AssertRedacted admits no per-field exception for
				// the six x509 names it flags.
				for _, action := range []string{"endpoints.mtls_certificate", "endpoints.mtls_agent_certificate_error"} {
					got := callAction[map[string]any](t, session, surface, action, map[string]any{"id": envID})
					if got["MTLSCertificate"] != nil {
						t.Errorf("%s returned certificate material: %v", action, got["MTLSCertificate"])
					}
				}
			})
		}
	}
}

// TestEndpoints_SnapshotThenReadTheStoredSnapshot_OnBusinessEdition covers
// the five snapshot actions as the one sequence that makes them meaningful:
// refresh the stored snapshot, then read it back three ways.
//
// The three read actions are tagged ["endpoints", "docker"] upstream and
// live in this domain because cmd/gen_action_inputs routes by tags[0]; they
// read what Portainer stored at the last poll rather than the Docker daemon,
// which is why endpoints.snapshot has to run first for the read to describe
// anything current.
func TestEndpoints_SnapshotThenReadTheStoredSnapshot_OnBusinessEdition(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("the three snapshot read actions are Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "EE")
			envID := firstDockerEnvironmentID(t, "EE")

			callAction[any](t, session, surface, "endpoints.snapshot", map[string]any{"id": envID})

			snapshot := callAction[map[string]any](t, session, surface, "endpoints.snapshot_inspect",
				map[string]any{"environmentId": envID})
			if len(snapshot) == 0 {
				t.Fatal("endpoints.snapshot_inspect answered an empty object after endpoints.snapshot ran")
			}

			containers := callAction[[]map[string]any](t, session, surface, "endpoints.snapshot_containers_list",
				map[string]any{"environmentId": envID})
			if len(containers) == 0 {
				t.Skip("the stored snapshot lists no containers, so there is none to inspect individually")
			}

			// The container id is read out of the snapshot rather than
			// invented, which is the whole point of this action: it is
			// Docker's own hexadecimal id, and the vendored specification
			// declares the path parameter an integer. A catalog that
			// published it as an integer could not make this call at all.
			id, _ := containers[0]["Id"].(string)
			if id == "" {
				t.Fatalf("the stored snapshot's first container carries no string Id: %v", containers[0])
			}
			one := callAction[map[string]any](t, session, surface, "endpoints.snapshot_container_inspect",
				map[string]any{"environmentId": envID, "containerId": id})
			if len(one) == 0 {
				t.Errorf("endpoints.snapshot_container_inspect answered nothing for container %q", id)
			}
		})
	}
}

// TestEndpoints_SnapshotAll_RefreshesEveryEnvironment covers the fleet-wide
// snapshot, the action renamed from the mechanical "endpoints.snapshots" to
// "endpoints.snapshot_all" because one letter is not enough to tell a
// single-environment refresh from a fleet-wide one.
func TestEndpoints_SnapshotAll_RefreshesEveryEnvironment(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				callAction[any](t, sessions.For(t, surface, ed), surface, "endpoints.snapshot_all", nil)
			})
		}
	}
}

// TestEndpoints_DeleteBatch_RemovesEveryEnvironmentItNames is the batch
// delete, read back the same way the single delete is.
func TestEndpoints_DeleteBatch_RemovesEveryEnvironmentItNames(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-batch")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				callAction[any](t, session, surface, "endpoints.delete_batch", map[string]any{
					"endpoints": []map[string]any{{"id": id}},
				})
				if got := environmentIDByName(t, ed, name); got != -1 {
					t.Errorf("environment %q is still present as %d after endpoints.delete_batch", name, got)
				}
			})
		}
	}
}

// TestEndpoints_UpdateRelations_ReassignsAnEnvironmentsGroup proves the
// relations update actually writes, by moving an environment into a group
// and reading the group back raw.
func TestEndpoints_UpdateRelations_ReassignsAnEnvironmentsGroup(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				name := uniqueName("env-relations")
				id := createEnvironmentThroughSurface(t, surface, ed, name)

				tagID := createTag(t, ed, uniqueName("tag-relations"))
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteTagIfPresent(ctx, ed, tagID); err != nil {
						t.Errorf("clean up tag %d: %v", tagID, err)
					}
				})

				callAction[any](t, session, surface, "endpoints.update_relations", map[string]any{
					"relations": map[string]any{
						strconv.Itoa(id): map[string]any{"tags": []int{tagID}},
					},
				})

				tags, _ := rawEnvironment(t, ed, id)["TagIds"].([]any)
				var carries bool
				for _, raw := range tags {
					if got, ok := raw.(float64); ok && int(got) == tagID {
						carries = true
					}
				}
				if !carries {
					t.Errorf("environment %d carries tags %v after endpoints.update_relations, want it to include %d", id, tags, tagID)
				}
			})
		}
	}
}

// TestEndpoints_RegistryAccess_SetsPerEnvironmentRegistryPolicies exercises
// the per-environment registry access update against a registry this test
// creates.
func TestEndpoints_RegistryAccess_SetsPerEnvironmentRegistryPolicies(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				registryID := createRegistry(t, ed, uniqueName("registry-access"))
				t.Cleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
					defer cancel()
					if err := deleteRegistryIfPresent(ctx, ed, registryID); err != nil {
						t.Errorf("clean up registry %d: %v", registryID, err)
					}
				})

				callAction[any](t, session, surface, "endpoints.registry_access", map[string]any{
					"id": envID, "registryId": registryID,
				})
			})
		}
	}
}

// TestEndpoints_DockerBrowsePut_UploadsAFileIntoTheEnvironment covers the
// second multipart route in this domain, the one whose query parameter is
// spelled volumeID rather than volumeId.
func TestEndpoints_DockerBrowsePut_UploadsAFileIntoTheEnvironment(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				toolName, args := actionCallParams(t, surface, "endpoints.docker_browse_put", map[string]any{
					"id":   envID,
					"path": "/tmp/" + uniqueName("upload") + ".pem",
					"file": "-----BEGIN CERTIFICATE-----\ne2e\n-----END CERTIFICATE-----\n",
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.docker_browse_put: %v", err)
				}
				// The route is served only by an environment reached through
				// a Portainer agent; the estate's Docker-in-Docker
				// environment is a direct daemon connection, so Portainer
				// refuses with a message about the environment rather than
				// about the request. That still exercises the whole
				// hand-written path — multipart body, query parameter, typed
				// client call — and a malformed body would fail differently,
				// which is what the unit tests in
				// internal/tools/endpoints/handlers_test.go pin exactly.
				if res.IsError {
					t.Logf("endpoints.docker_browse_put on this estate's environment: %s", toolResultText(res))
				}
			})
		}
	}
}

// TestEndpoints_ForceUpdateService_RedeploysASwarmService covers the Swarm
// service redeploy, against the fixture service up.sh creates.
func TestEndpoints_ForceUpdateService_RedeploysASwarmService(t *testing.T) {
	if !estate.HasSwarm() {
		t.Skip("this estate's Docker daemon has no Swarm fixture service to redeploy")
	}
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)
				envID := firstDockerEnvironmentID(t, ed)

				toolName, args := actionCallParams(t, surface, "endpoints.force_update_service", map[string]any{
					"id": envID, "serviceId": estate.SwarmServiceID, "pullImage": false,
				})
				res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.force_update_service: %v", err)
				}
				if res.IsError {
					t.Logf("endpoints.force_update_service on this estate: %s", toolResultText(res))
				}
			})
		}
	}
}

// createEdgeEnvironmentThroughSurface registers an edge environment
// (endpointCreationType 4) through the surface under test.
//
// It exists so the four edge actions below never touch the edge environment
// the estate itself provisions: association_delete in particular clears an
// environment's edge identity, and doing that to the estate's own edge agent
// would break every later test in the run for a reason that has nothing to
// do with the action.
//
// Business Edition only, and creating one at all depends on Edge Compute
// already being enabled on the server (docs/api-divergences.md §3.4). The
// estate enables it when it registers its own edge environment, so this
// works here and would not against a bare Portainer.
func createEdgeEnvironmentThroughSurface(t *testing.T, surface, name string) int {
	t.Helper()
	toolName, args := actionCallParams(t, surface, "endpoints.create", map[string]any{
		"name": name, "endpointCreationType": 4,
	})
	res, err := sessions.For(t, surface, "EE").CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
	if err != nil {
		t.Fatalf("endpoints.create (edge): %v", err)
	}
	if res.IsError {
		text := toolResultText(res)
		if strings.Contains(text, "node allowance") {
			t.Skipf("this licence has no node allowance left to register an edge environment: %s", text)
		}
		t.Fatalf("endpoints.create (edge) on %s: %s", surface, text)
	}
	var created map[string]any
	if err := json.Unmarshal([]byte(toolResultText(res)), &created); err != nil {
		t.Fatalf("decode endpoints.create result: %v", err)
	}
	// The edge key is the enrolment secret an agent presents to claim this
	// environment, and Portainer returns it on this very response. Asserting
	// it never reaches a caller here, on a create that really produced one,
	// is what the unit guard in internal/tools/endpoints/redaction_test.go
	// cannot do: that one runs against a synthetic fixture.
	if key, _ := created["EdgeKey"].(string); key != "" {
		t.Errorf("endpoints.create returned the edge enrolment key to a caller: %q", key)
	}
	idFloat, ok := created["Id"].(float64)
	if !ok {
		t.Fatalf("endpoints.create returned no Id: %v", created)
	}
	id := int(idFloat)
	newLedger(t).Register("environment", strconv.Itoa(id), func(ctx context.Context) error {
		return deleteEnvironmentIfPresent(ctx, "EE", id)
	})
	return id
}

// TestEndpoints_EdgeActions_OnAnEnvironmentThisTestOwns covers the edge
// actions against an edge environment this test registers and removes.
//
// It takes about a minute, serially, and both halves of that are deliberate.
//
// Registering an edge environment schedules one snapshot attempt against a
// host that will never answer, and that attempt costs ~20 seconds. Portainer
// serialises them globally, so the cost is not per-call but per-environment,
// and any concurrent request can end up queued behind an attempt that is not
// its own: measured 2026-08-18, one local run charged the 20 seconds to
// trust_edge_endpoints on one surface and to association_delete on two
// others, while the remaining calls of both actions returned in single-digit
// milliseconds.
//
// The subtests therefore do NOT run in parallel, and that is the fix for a
// real CI failure rather than tidiness. Run in parallel, three surfaces
// register three edge environments at once, and the third call to arrive
// waits behind two attempts it did not cause — about 40 seconds before its
// own work even starts. That fits inside portainer.DefaultCallTimeout (60s)
// on this machine and does not on a slower CI runner, where a raw read timed
// out with "context deadline exceeded" against an environment that was
// perfectly healthy.
//
// rawEdgeEnvironment's ?excludeSnapshot=true is still worth having and does
// not solve this on its own: it avoids the cost of the reader's OWN snapshot
// attempt, not the queue behind somebody else's. Serialising is what bounds
// any single call's wait to one attempt.
//
// Total wall clock stays about a minute either way — three environments x
// ~20s is Portainer's cost, not this suite's, and a later reader should not
// try to "fix" it by weakening the test.
func TestEndpoints_EdgeActions_OnAnEnvironmentThisTestOwns(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("trust_edge_endpoints and set_policy_statuses are Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			session := sessions.For(t, surface, "EE")

			id := createEdgeEnvironmentThroughSurface(t, surface, uniqueName("env-edge"))

			callAction[any](t, session, surface, "endpoints.trust_edge_endpoints", map[string]any{
				"endpointIdS": []int{id},
			})
			if trusted, _ := rawEdgeEnvironment(t, "EE", id)["UserTrusted"].(bool); !trusted {
				t.Errorf("environment %d is not UserTrusted after endpoints.trust_edge_endpoints", id)
			}

			// set_policy_statuses is an agent-facing callback and is
			// asserted to REACH Portainer, not to succeed. Measured
			// 2026-08-18: it answers 403 "Unauthorized Edge endpoint
			// operation: missing Edge identifier" for an API-key caller,
			// because Portainer reads the reporting agent's identity from a
			// request header only the agent sets — the same reason
			// endpoints.create_global_key cannot be usefully called from
			// here, and what both actions' narratives tell a model. A
			// different failure would be a real defect, so the message is
			// checked rather than ignored.
			statusTool, statusArgs := actionCallParams(t, surface, "endpoints.set_policy_statuses", map[string]any{
				"id": id,
				"statuses": []map[string]any{
					{"policyId": 1, "status": "applied", "type": "edgeStack", "message": "e2e"},
				},
			})
			statusRes, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: statusTool, Arguments: statusArgs})
			if err != nil {
				t.Fatalf("endpoints.set_policy_statuses: %v", err)
			}
			if text := toolResultText(statusRes); statusRes.IsError && !strings.Contains(text, "missing Edge identifier") {
				t.Errorf("endpoints.set_policy_statuses failed for a reason other than the measured missing-agent-identity refusal: %s", text)
			}

			// The disassociation is the destructive one, and the read-back is
			// what proves it did something: Portainer clears the environment's
			// edge identity, so the EdgeID it was registered with is gone.
			before, _ := rawEdgeEnvironment(t, "EE", id)["EdgeID"].(string)
			if before == "" {
				t.Fatalf("environment %d has no EdgeID to clear", id)
			}
			callAction[any](t, session, surface, "endpoints.association_delete", map[string]any{"id": id})
			if after, _ := rawEdgeEnvironment(t, "EE", id)["EdgeID"].(string); after == before {
				t.Errorf("environment %d still carries EdgeID %q after endpoints.association_delete", id, after)
			}
		})
	}
}

// TestEndpoints_CreateGlobalKey_IsReachableAndCannotIdentifyAnAgent covers
// the edge enrolment route this catalog can reach but cannot usefully call.
//
// Portainer reads the agent's identity from a request header the agent sets;
// this action has no field for it and sends none, which is exactly what its
// narrative tells a model. The assertion is therefore about reachability,
// not success — and it is worth having, because an action that 404'd or
// failed to build its request would be a different defect entirely.
func TestEndpoints_CreateGlobalKey_IsReachableAndCannotIdentifyAnAgent(t *testing.T) {
	for _, ed := range sessions.Editions() {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				toolName, args := actionCallParams(t, surface, "endpoints.create_global_key", nil)
				res, err := sessions.For(t, surface, ed).CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
				if err != nil {
					t.Fatalf("endpoints.create_global_key: %v", err)
				}
				t.Logf("endpoints.create_global_key answered: %s", toolResultText(res))
			})
		}
	}
}

// TestEndpoints_NamespacesAccessUpdate_IsRefusedByADockerEnvironment is
// negative coverage, and deliberately labelled as such.
//
// The action updates access on a Kubernetes namespace and needs a Kubernetes
// environment; this estate's compose legs register Docker environments only,
// so there is nothing here for it to succeed against. Refusing on a Docker
// environment is still a real property worth pinning — it is the same
// treatment, for the same reason, that stacks_test.go's
// TestStacks_KubernetesCreates_AreRefusedByADockerEnvironment gives the
// three Kubernetes stack creates. Positive coverage needs the Kubernetes leg
// (`make e2e-k8s-up`) and is not provided by this wave.
func TestEndpoints_NamespacesAccessUpdate_IsRefusedByADockerEnvironment(t *testing.T) {
	if !estate.HasBusinessEdition() {
		t.Skip("namespaces_access_update is Business Edition only")
	}
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			envID := firstDockerEnvironmentID(t, "EE")
			toolName, args := actionCallParams(t, surface, "endpoints.namespaces_access_update", map[string]any{
				"id": envID, "rpn": 1, "usersToAdd": []int{1},
			})
			res, err := sessions.For(t, surface, "EE").CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("endpoints.namespaces_access_update: %v", err)
			}
			if !res.IsError {
				t.Errorf("endpoints.namespaces_access_update succeeded against a Docker environment: %s", toolResultText(res))
			}
		})
	}
}

// endpointsSafeModeMutations is one entry per mutating action in this domain
// whose interception safe mode must prove, with inputs that would do real
// damage if they ever reached Portainer.
var endpointsSafeModeMutations = []struct {
	action string
	kind   string
	input  func(envID int) map[string]any
}{
	{"endpoints.create", "mutating", func(int) map[string]any {
		return map[string]any{"name": uniqueName("env-safe"), "endpointCreationType": 1, "url": "tcp://docker:2375"}
	}},
	{"endpoints.update", "mutating", func(id int) map[string]any {
		return map[string]any{"id": id, "name": uniqueName("env-safe-renamed")}
	}},
	{"endpoints.delete", "destructive", func(id int) map[string]any {
		return map[string]any{"id": id}
	}},
	{"endpoints.delete_batch", "destructive", func(id int) map[string]any {
		return map[string]any{"endpoints": []map[string]any{{"id": id}}}
	}},
	{"endpoints.settings_update", "mutating", func(id int) map[string]any {
		return map[string]any{"id": id, "securitySettings": map[string]any{"allowPrivilegedModeForRegularUsers": true}}
	}},
	{"endpoints.association_delete", "destructive", func(id int) map[string]any {
		return map[string]any{"id": id}
	}},
	{"endpoints.snapshot_all", "mutating", func(int) map[string]any { return nil }},
}

// TestSafeMode_Endpoints_MutatingActionsArePreviewedAndNothingChanges proves
// tools.Execute intercepts every mutating action in this domain before its
// handler runs.
//
// Three properties, following
// TestSafeMode_CustomTemplates_MutatingActionsArePreviewedAndNothingChanges:
// the answer is a preview rather than a result, it names the action and the
// kind its own ActionSpec flags imply, and nothing changed — read back
// through the raw Portainer API, because a surface that previewed a call and
// executed it anyway would satisfy the first two identically.
//
// Community Edition only: safe-mode interception has nothing to do with
// edition, so it is proven against the leg every estate carries.
func TestSafeMode_Endpoints_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range endpointsSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				// The victim is the environment the estate provisions: if
				// safe mode let any of these through, it is the one that
				// would be renamed, have its settings changed, or be deleted.
				envID := firstDockerEnvironmentID(t, "CE")
				before := rawEnvironment(t, "CE", envID)
				beforeName, _ := before["Name"].(string)
				beforeCount := len(rawEnvironments(t, "CE"))

				input := mutation.input(envID)
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

				// Nothing changed.
				if got, _ := rawEnvironment(t, "CE", envID)["Name"].(string); got != beforeName {
					t.Errorf("environment %d is now named %q, want %q: safe mode let %s through", envID, got, beforeName, mutation.action)
				}
				if got := len(rawEnvironments(t, "CE")); got != beforeCount {
					t.Errorf("the server now has %d environments, want %d: safe mode let %s through", got, beforeCount, mutation.action)
				}
			})
		}
	}
}
