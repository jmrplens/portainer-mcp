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
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
	"github.com/jmrplens/portainer-mcp/internal/tools/resource_controls"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
	"github.com/jmrplens/portainer-mcp/test/e2e/harness"
)

// resource_controls has no read route — not in this catalog and not on the
// server: GET /resource_controls and GET /resource_controls/{id} both answer
// 405 Method Not Allowed (asserted below, on both editions). That shapes
// every assertion here.
//
// The naive reading is that a create or update can only be checked against
// its own response, which would break this package's standing rule that a
// read-back never goes through the action under test: a create answering
// with a plausible body while writing nothing would satisfy it perfectly.
// There is a way out, and it is what this file uses. A resource control is
// stored ON the resource it guards, and Portainer's own Docker proxy
// publishes it: GET /endpoints/{id}/docker/volumes/{name} answers with a
// "Portainer" object carrying the whole ResourceControl. That is a raw read,
// through a route no action in this catalog serves, so it is a genuine
// third-party witness to what each action wrote.
//
// So both are asserted at every step: the action's own response (which is
// what a model actually sees, and the only place a new control's Id is ever
// published) and the raw witness on the volume.

// createVolumeFixture creates a Docker volume named name through Portainer's
// own Docker proxy on edition ed's server, registers its removal, and
// returns Portainer's identifier for it.
//
// The returned identifier is NOT the volume name, and that distinction is
// the reason this helper returns anything at all rather than letting callers
// reuse the name they passed in. Measured on Community Edition: a volume
// named V comes back with ResourceID "V_dcm81sn24aeeuf36q9cbktjkw" — the
// name with a node/cluster suffix — and a resource control created against
// the bare name is accepted with 200 and governs nothing, because Portainer
// never checks that the named resource exists. A test built on the bare name
// would pass while proving that resource_controls.create can write a control
// over a resource that does not exist, which is the opposite of what it
// claims to prove. resource_controls.create's narrative says the same thing
// to a caller.
func createVolumeFixture(t *testing.T, ed, name string) string {
	t.Helper()
	srv := serverFor(t, ed)
	envID := volumeEnvID(t, ed)

	var resourceID string
	retryFixture(t, fmt.Sprintf("create volume %q on %s", name, ed), func() error {
		ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
		defer cancel()

		resp := dockerProxy(ctx, t, srv, envID, http.MethodPost, "/volumes/create", map[string]any{"Name": name})
		body := readBody(t, resp)
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("create volume %q: HTTP %d: %s", name, resp.StatusCode, body)
		}
		var created struct {
			ResourceID string
		}
		if err := json.Unmarshal([]byte(body), &created); err != nil {
			return fmt.Errorf("create volume %q: decode %s: %w", name, body, err)
		}
		if created.ResourceID == "" {
			return fmt.Errorf("create volume %q: response carried no ResourceID: %s", name, body)
		}
		resourceID = created.ResourceID
		return nil
	})

	newLedger(t).Register("volume", name, func(ctx context.Context) error {
		return deleteVolumeIfPresent(ctx, ed, name)
	})
	return resourceID
}

// deleteVolumeIfPresent removes a volume through Portainer's Docker proxy,
// tolerating one that is already gone. Nothing in this file deletes a volume
// through an action, so "already gone" here means a previous cleanup, never
// a passing test — but the tolerance costs nothing and keeps a partial
// teardown from reporting a failure for a test that succeeded.
func deleteVolumeIfPresent(ctx context.Context, ed, name string) error {
	srv, err := rawServerFor(ed)
	if err != nil {
		return err
	}
	envID, ok := srv.Environment("docker")
	if !ok {
		return fmt.Errorf("delete volume %q: %s estate has no docker environment", name, ed)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete,
		fmt.Sprintf("%s/api/endpoints/%d/docker/volumes/%s", strings.TrimRight(srv.BaseURL, "/"), envID, name), nil)
	if err != nil {
		return fmt.Errorf("delete volume %q: %w", name, err)
	}
	req.Header.Set("X-API-Key", srv.Creds.APIKey)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("delete volume %q: %w", name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("delete volume %q: HTTP %d", name, resp.StatusCode)
	}
	return nil
}

// rawVolumeControl reads the resource control Portainer stores on a volume,
// straight through the Docker proxy, and returns nil when the volume carries
// none.
//
// This is the raw witness every assertion in this file reads: no action in
// this catalog serves GET /endpoints/{id}/docker/volumes/{name}, so nothing
// here is checking one of the three actions under test against itself or
// against its own sibling.
func rawVolumeControl(t *testing.T, ed, name string) map[string]any {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), portainer.DefaultCallTimeout)
	defer cancel()

	resp := dockerProxy(ctx, t, serverFor(t, ed), volumeEnvID(t, ed), http.MethodGet, "/volumes/"+name, nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw volume inspect %q on %s: HTTP %d: %s", name, ed, resp.StatusCode, body)
	}
	var inspected struct {
		Portainer *struct {
			ResourceControl map[string]any
		}
	}
	if err := json.Unmarshal([]byte(body), &inspected); err != nil {
		t.Fatalf("raw volume inspect %q on %s: decode %s: %v", name, ed, body, err)
	}
	if inspected.Portainer == nil {
		return nil
	}
	return inspected.Portainer.ResourceControl
}

// deleteResourceControlRaw removes a resource control through the generated
// Portainer client rather than through resource_controls.delete.
//
// Two callers, both needing the action under test kept out of it. The
// lifecycle test uses it to clear the control Portainer auto-creates with
// every volume, so that resource_controls.create has real work to do — a
// create against a resource that already has one is refused 409 (measured),
// so without this the create under test could never run. And every ledger
// cleanup uses it, because a test that failed midway may leave a control
// behind that resource_controls.delete never reached.
func deleteResourceControlRaw(ctx context.Context, ed string, id int) error {
	client, err := rawClientFor(ed)
	if err != nil {
		return err
	}
	resp, err := client.API.ResourceControlDeleteWithResponse(ctx, id)
	if err != nil {
		return fmt.Errorf("delete resource control %d: %w", id, err)
	}
	// 404 is the ordinary outcome for a control the test already removed
	// through the action under test: there is no list route to check
	// presence against first, the way deleteTeamIfPresent can, so this
	// tolerates the status rather than reading the state.
	if resp.StatusCode() == http.StatusNotFound {
		return nil
	}
	if err := toolutil.Check(resp); err != nil {
		return fmt.Errorf("delete resource control %d: %w", id, err)
	}
	return nil
}

// rawControlID reads a raw resource control's own Id, which arrives as a
// float64 through a generic JSON decode. The bool reports whether the field
// was present and numeric at all, so a caller can tell "Id 0" apart from "no
// Id in the answer" — and this domain's whole point is that the Id is the
// only handle a caller ever gets, so an absent one is worth distinguishing.
func rawControlID(c map[string]any) (int, bool) {
	got, ok := c["Id"].(float64)
	return int(got), ok
}

// controlBool reads one boolean field out of a raw resource control. The
// second bool reports whether the field was present and of the right type at
// all, so a caller can tell "false" apart from "absent".

func controlBool(c map[string]any, field string) (bool, bool) {
	got, ok := c[field].(bool)
	return got, ok
}

// controlUserIDs returns the user ids a raw resource control grants access
// to, so a test can assert the grant set rather than its length.
func controlUserIDs(c map[string]any) []int {
	accesses, ok := c["UserAccesses"].([]any)
	if !ok {
		return nil
	}
	var ids []int
	for _, entry := range accesses {
		access, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		if id, ok := access["UserId"].(float64); ok {
			ids = append(ids, int(id))
		}
	}
	return ids
}

// TestResourceControls_Lifecycle_AcrossSurfacesAndEditions runs the domain's
// whole chain against a real Docker volume: create a control over it, update
// who may reach it, delete the control — every mutation through the surface
// under test, every read-back raw off the volume itself.
//
// The volume is created directly through Portainer's Docker proxy, and the
// control Portainer auto-creates with it is removed the same way, before the
// action under test runs. That deletion is not tidying: a create against a
// resource that already holds a control is refused 409 (measured on
// Community, "A resource control is already associated to this resource"),
// so leaving the auto-created one in place would make this test assert that
// resource_controls.create fails, which is not what it is for.
func TestResourceControls_Lifecycle_AcrossSurfacesAndEditions(t *testing.T) {
	editions := sessions.Editions()
	if len(editions) == 0 {
		t.Fatal("sessions.Editions() is empty: this test would pass by running nothing")
	}

	for _, ed := range editions {
		for _, surface := range surfaceNames {
			t.Run(ed+"/"+surface, func(t *testing.T) {
				t.Parallel()
				session := sessions.For(t, surface, ed)

				volumeName := uniqueName("rc-volume")
				resourceID := createVolumeFixture(t, ed, volumeName)

				// Clear the control Portainer created with the volume, so
				// the create under test has a resource with none.
				auto := rawVolumeControl(t, ed, volumeName)
				if auto == nil {
					t.Fatalf("volume %q carries no resource control after creation: this test's premise (Portainer auto-creates one) no longer holds", volumeName)
				}
				autoID, ok := rawControlID(auto)
				if !ok {
					t.Fatalf("the auto-created control on volume %q carries no Id: %v", volumeName, auto)
				}
				if err := deleteResourceControlRaw(t.Context(), ed, autoID); err != nil {
					t.Fatalf("clearing the auto-created control %d: %v", autoID, err)
				}
				if got := rawVolumeControl(t, ed, volumeName); got != nil {
					t.Fatalf("volume %q still carries control %v after the auto-created one was removed", volumeName, got)
				}

				// resource_controls.create, through the surface under test.
				// administratorsOnly true rather than an empty grant set: at
				// least one of users, teams, public or administratorsOnly
				// must be on, or Portainer refuses with 400 (measured on
				// both editions; see the action's narrative).
				created := callAction[map[string]any](t, session, surface, "resource_controls.create", map[string]any{
					"resourceId":         resourceID,
					"type":               3,
					"administratorsOnly": true,
				})
				idFloat, ok := created["Id"].(float64)
				if !ok {
					t.Fatalf("resource_controls.create response carried no Id: %v", created)
				}
				controlID := int(idFloat)
				newLedger(t).Register("resource_control", strconv.Itoa(controlID), func(ctx context.Context) error {
					return deleteResourceControlRaw(ctx, ed, controlID)
				})

				// The response is the only place this Id is ever published,
				// so it is asserted on its own terms as well as through the
				// witness below.
				if got, _ := created["ResourceId"].(string); got != resourceID {
					t.Errorf("resource_controls.create returned ResourceId %q, want %q", got, resourceID)
				}
				if got, _ := created["Type"].(float64); int(got) != 3 {
					t.Errorf("resource_controls.create returned Type %v, want 3 (volume)", created["Type"])
				}
				if got, _ := created["AdministratorsOnly"].(bool); !got {
					t.Errorf("resource_controls.create returned AdministratorsOnly %v, want true", created["AdministratorsOnly"])
				}

				// The witness: the volume itself, read raw. A create that
				// answered with a plausible body and wrote nothing passes
				// every assertion above and fails this one.
				afterCreate := rawVolumeControl(t, ed, volumeName)
				if afterCreate == nil {
					t.Fatalf("volume %q carries no resource control after resource_controls.create", volumeName)
				}
				if got, ok := rawControlID(afterCreate); !ok || got != controlID {
					t.Fatalf("volume %q carries control %v (present=%v), want the created %d", volumeName, afterCreate["Id"], ok, controlID)
				}
				if got, ok := controlBool(afterCreate, "AdministratorsOnly"); !ok || !got {
					t.Errorf("control %d on volume %q reads AdministratorsOnly %v (present=%v), want true", controlID, volumeName, afterCreate["AdministratorsOnly"], ok)
				}

				// resource_controls.update. public true AND users [1]
				// together, moving both fields away from what the create
				// left: a row whose victim is already in the state the call
				// asks for cannot fail, and this stage has caught one such
				// assertion already. administratorsOnly is sent false
				// explicitly, so the update is asked to change all three.
				updated := callAction[map[string]any](t, session, surface, "resource_controls.update", map[string]any{
					"id":                 controlID,
					"public":             true,
					"administratorsOnly": false,
					"users":              []int{1},
				})
				if got, _ := updated["Id"].(float64); int(got) != controlID {
					t.Errorf("resource_controls.update answered about control %v, want %d", updated["Id"], controlID)
				}
				if got, _ := updated["Public"].(bool); !got {
					t.Errorf("resource_controls.update returned Public %v, want true", updated["Public"])
				}

				afterUpdate := rawVolumeControl(t, ed, volumeName)
				if afterUpdate == nil {
					t.Fatalf("volume %q carries no resource control after resource_controls.update", volumeName)
				}
				if got, ok := rawControlID(afterUpdate); !ok || got != controlID {
					t.Fatalf("volume %q carries control %v (present=%v) after the update, want %d unchanged", volumeName, afterUpdate["Id"], ok, controlID)
				}
				if got, ok := controlBool(afterUpdate, "Public"); !ok || !got {
					t.Errorf("control %d reads Public %v (present=%v) after resource_controls.update, want true", controlID, afterUpdate["Public"], ok)
				}
				if got, ok := controlBool(afterUpdate, "AdministratorsOnly"); !ok || got {
					t.Errorf("control %d reads AdministratorsOnly %v (present=%v) after resource_controls.update, want false", controlID, afterUpdate["AdministratorsOnly"], ok)
				}
				// The grant set itself, not its length: an update that
				// ignored its users field would leave this empty, and an
				// update that rebuilt the control from a partly-empty
				// payload would put someone else in it.
				if got := controlUserIDs(afterUpdate); len(got) != 1 || got[0] != 1 {
					t.Errorf("control %d grants users %v after resource_controls.update, want exactly [1]", controlID, got)
				}

				// resource_controls.delete, and the witness again: the
				// volume must come back carrying no control at all, which
				// is what a delete that no-opped fails.
				callAction[any](t, session, surface, "resource_controls.delete", map[string]any{"id": controlID})
				if got := rawVolumeControl(t, ed, volumeName); got != nil {
					t.Errorf("volume %q still carries control %v after resource_controls.delete", volumeName, got)
				}
			})
		}
	}
}

// TestResourceControls_NoReadRouteExists is the regression test behind the
// one sentence all three narratives repeat: there is no way to read a
// resource control back.
//
// That claim is load-bearing — it is why each narrative tells a model to
// keep the Id the create answers with — and it is the kind of claim that
// rots silently. If Portainer ever adds GET /resource_controls, all three
// descriptions become wrong in the same breath, and nothing else in this
// repository would notice: no action exists to fail, no schema changes, and
// the audits compare the catalog against the vendored document, which would
// still declare no such route either.
//
// Read raw, on both editions, and asserted as 405 specifically rather than
// "not 200": a 401 or a 404 here would mean something quite different (a
// route that exists and is refusing this caller, or one that moved), and
// neither would justify the narratives' phrasing.
func TestResourceControls_NoReadRouteExists(t *testing.T) {
	editions := sessions.Editions()
	if len(editions) == 0 {
		t.Fatal("sessions.Editions() is empty: this test would pass by running nothing")
	}

	for _, ed := range editions {
		for _, path := range []string{"/resource_controls", "/resource_controls/1"} {
			t.Run(ed+path, func(t *testing.T) {
				t.Parallel()
				srv := serverFor(t, ed)
				ctx, cancel := context.WithTimeout(t.Context(), portainer.DefaultCallTimeout)
				defer cancel()

				req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(srv.BaseURL, "/")+"/api"+path, nil)
				if err != nil {
					t.Fatalf("building GET %s: %v", path, err)
				}
				req.Header.Set("X-API-Key", srv.Creds.APIKey)
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatalf("GET %s on %s: %v", path, ed, err)
				}
				body := readBody(t, resp)
				if resp.StatusCode != http.StatusMethodNotAllowed {
					t.Errorf("GET %s on %s answered HTTP %d (%s), want 405: all three resource_controls narratives tell a model no route reads a control back, "+
						"and each one tells it to keep the Id resource_controls.create answers with because of that", path, ed, resp.StatusCode, body)
				}
			})
		}
	}
}

// TestResourceControls_SubResourceIdsFieldName_IsRefusedUnlessSpelledAsPublished
// pins the one refusal in this domain that comes from the CATALOG rather
// than from Portainer, and that a model will meet by accident.
//
// The create payload's SubResourceIDs property becomes the published wire
// name "subResourceIdS", capital S. The natural spelling "subResourceIds" is
// refused by this action's own schema, before any call is made, so
// resource_controls.create's narrative gives the exact spelling. This is what
// makes that sentence a tested promise rather than a guess: if the naming
// rule ever changes, the published name changes with it and the narrative
// silently starts lying.
//
// The capital S comes from bodyJSONTag in cmd/gen_action_inputs/naming.go,
// mirrored in internal/specdiff/naming.go — NOT from internal/specnaming,
// which holds only the parameter/body collision rule and the
// synthetic-operationId rule and contains neither function. splitWords
// correctly emits ["Sub" "Resource" "ID" "s"]; goFieldName special-cases the
// lone trailing "s" and bodyJSONTag does not, so title("s") renders "S".
// internal/tools/resource_controls' package doc carries the full derivation
// and the five affected fields across three domains.
//
// Both halves are asserted, and the pair is the point. The refusal alone
// would still hold if the field vanished from the schema entirely; the
// acceptance alone would still hold if BOTH spellings were accepted, which
// would make the narrative's warning pointless noise.
//
// Community Edition only: the field is not edition-gated in any way, and
// schema validation happens before any server is contacted, so the Business
// leg would be running the identical code path against the identical schema.
func TestResourceControls_SubResourceIdsFieldName_IsRefusedUnlessSpelledAsPublished(t *testing.T) {
	for _, surface := range surfaceNames {
		t.Run(surface, func(t *testing.T) {
			t.Parallel()
			session := sessions.For(t, surface, "CE")

			volumeName := uniqueName("rc-subres")
			resourceID := createVolumeFixture(t, "CE", volumeName)

			// The natural spelling, which this action does not publish.
			toolName, args := actionCallParams(t, surface, "resource_controls.create", map[string]any{
				"resourceId":         resourceID + "-natural",
				"type":               3,
				"administratorsOnly": true,
				"subResourceIds":     []string{"nothing-is-created-by-this-call"},
			})
			res, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: toolName, Arguments: args})
			if err != nil {
				t.Fatalf("resource_controls.create with subResourceIds: %v", err)
			}
			text := toolResultText(res)
			if !res.IsError {
				t.Errorf("resource_controls.create accepted the field spelled subResourceIds; this action publishes subResourceIdS, "+
					"and its narrative tells a model so: %s", text)
			} else {
				// Both fragments, not just the field name: the name alone
				// would also be satisfied by an unrelated validation failure
				// that happened to mention it, which would leave the
				// schema-shape mechanism — the thing the narrative's warning
				// is about — unpinned. The two validation paths word their
				// prefixes differently (the MCP SDK's typed-tool validation
				// on the individual surface, Execute's ValidateInput on meta
				// and dynamic) and agree on exactly these two fragments.
				for _, want := range []string{"unexpected additional properties", "subResourceIds"} {
					if !strings.Contains(text, want) {
						t.Errorf("resource_controls.create was refused, but not as an unexpected-additional-property refusal naming the field: "+
							"the message does not contain %q: %s", want, text)
					}
				}
			}

			// The spelling this action does publish, on the same volume,
			// through the same surface. Portainer auto-created a control
			// with the volume, so this create names a sub-resource list on a
			// fresh control of its own rather than colliding with it.
			created := callAction[map[string]any](t, session, surface, "resource_controls.create", map[string]any{
				"resourceId":         resourceID + "-published",
				"type":               3,
				"administratorsOnly": true,
				"subResourceIdS":     []string{volumeName + "-sub"},
			})
			idFloat, ok := created["Id"].(float64)
			if !ok {
				t.Fatalf("resource_controls.create response carried no Id: %v", created)
			}
			newLedger(t).Register("resource_control", strconv.Itoa(int(idFloat)), func(ctx context.Context) error {
				return deleteResourceControlRaw(ctx, "CE", int(idFloat))
			})
			subs, ok := created["SubResourceIds"].([]any)
			if !ok || len(subs) != 1 || subs[0] != volumeName+"-sub" {
				t.Errorf("resource_controls.create stored SubResourceIds %v, want exactly [%q]: "+
					"the published spelling subResourceIdS must actually reach Portainer, not merely be accepted by the schema",
					created["SubResourceIds"], volumeName+"-sub")
			}
		})
	}
}

// resourceControlsSafeModeMutations is one entry per mutating action in the
// domain, following teamsSafeModeMutations' shape: inputs realistic enough
// that safe mode's interception is the only reason none of them execute.
//
// All three actions mutate — there is no read in this domain to leave out —
// and TestSafeMode_ResourceControls_TableCoversEveryMutatingAction below
// fails if a fourth is added without a row here.
var resourceControlsSafeModeMutations = []struct {
	action string
	kind   string
	input  func(resourceID string, controlID int) map[string]any
}{
	{"resource_controls.create", "mutating", func(resourceID string, _ int) map[string]any {
		// A resource id nothing holds a control over, so a leak creates one.
		return map[string]any{"resourceId": resourceID + "-safe", "type": 3, "administratorsOnly": true}
	}},
	// public true against a control that really is administratorsOnly: a
	// preview row whose victim is already in the state the call asks for
	// cannot fail.
	{"resource_controls.update", "mutating", func(_ string, controlID int) map[string]any {
		return map[string]any{"id": controlID, "public": true}
	}},
	{"resource_controls.delete", "destructive", func(_ string, controlID int) map[string]any {
		return map[string]any{"id": controlID}
	}},
}

// assertNoControlGoverns fails when any resource control already governs
// resourceID, and it has to establish that indirectly: there is no route that
// reads a control back, so "does one exist for this resource?" cannot be
// asked directly on any layer.
//
// What it uses instead is the 409 the create route answers when a resource
// already holds a control ("A resource control is already associated to this
// resource", measured on Community). A raw create against the id therefore
// answers 409 if something is there and 200 if nothing is, and the 200 case
// leaves a control of its own that is deleted again immediately.
//
// This exists for exactly one caller: the safe-mode create row, whose input
// names resourceID+"-safe" rather than the volume's own id. A leak from that
// row writes a control the volume witness cannot see, because a volume holds
// at most one control and the leaked one hangs off a different resource id.
// Without this the row was pinned only by total non-interception.
//
// Raw throughout, deliberately: proving that resource_controls.create did not
// run must not itself go through resource_controls.create.
func assertNoControlGoverns(t *testing.T, ed, resourceID, why string) {
	t.Helper()
	client := fixtureClient(t, ed)
	ctx, cancel := context.WithTimeout(t.Context(), portainer.DefaultCallTimeout)
	defer cancel()

	administratorsOnly := true
	resp, err := client.API.ResourceControlCreateWithResponse(ctx, apigen.ResourceControlCreateJSONRequestBody{
		ResourceID:         resourceID,
		Type:               apigen.PortainerResourceControlTypeVolumeResourceControl,
		AdministratorsOnly: &administratorsOnly,
	})
	if err != nil {
		t.Fatalf("probing whether a control governs %q: %v", resourceID, err)
	}

	switch resp.StatusCode() {
	case http.StatusConflict:
		t.Errorf("a resource control already governs %q: %s", resourceID, why)
	case http.StatusOK:
		// Nothing was there. Remove the probe's own control again, so this
		// assertion leaves the estate exactly as it found it.
		if resp.JSON200 == nil || resp.JSON200.Id == nil {
			t.Fatalf("the probe create for %q answered 200 with no Id, so it cannot be cleaned up: %s", resourceID, resp.Body)
		}
		if err := deleteResourceControlRaw(ctx, ed, *resp.JSON200.Id); err != nil {
			t.Fatalf("removing the probe control %d for %q: %v", *resp.JSON200.Id, resourceID, err)
		}
	default:
		t.Fatalf("probing whether a control governs %q: HTTP %d: %s", resourceID, resp.StatusCode(), resp.Body)
	}
}

// TestSafeMode_ResourceControls_MutatingActionsArePreviewedAndNothingChanges
// proves tools.Execute intercepts every mutating action in this domain
// before its handler runs: the answer is a preview naming the action and the
// kind its own ActionSpec flags imply, and nothing changed — read back
// through the volume itself, raw, because a surface that previewed a call
// and executed it anyway would satisfy the first two identically.
//
// Every row's would-be effect is observable, deliberately:
//
//   - create names a resource id nothing controls, and that id is NOT the
//     volume's own — it is resourceID+"-safe". So a leak here writes a
//     control the volume's witness structurally cannot see: a volume carries
//     at most one control, and the leaked one would hang off a different
//     resource id entirely. An earlier revision of this comment claimed the
//     witness caught it anyway, via an assertion that the victim control is
//     the only one on the volume; no such assertion existed and none could,
//     so the row was pinned only by total non-interception. That hole is now
//     closed by assertNoControlGoverns below, which probes the leaked id
//     directly.
//   - update asks for public true against a control that is public false and
//     administratorsOnly false — the flags Portainer gives an auto-created
//     one (measured: {Public:false, AdministratorsOnly:false,
//     UserAccesses:[{UserId:1,AccessLevel:1}]}) — so a leak flips Public.
//   - delete targets that same real control, so a leak removes it and the
//     volume comes back carrying none.
//
// Community Edition only, like the teams safe-mode table: safe mode's
// interception has nothing to do with edition, so it is proven against the
// leg every estate carries. Named resources only, never a total: this
// subtest owns its own volume and asserts on the control that volume holds.
func TestSafeMode_ResourceControls_MutatingActionsArePreviewedAndNothingChanges(t *testing.T) {
	for _, surface := range surfaceNames {
		for _, mutation := range resourceControlsSafeModeMutations {
			t.Run(surface+"/"+mutation.action, func(t *testing.T) {
				t.Parallel()
				session := sessions.SafeMode(t, surface)

				volumeName := uniqueName("rc-safe")
				resourceID := createVolumeFixture(t, "CE", volumeName)

				// The victim is the control Portainer auto-created with
				// the volume, which arrives public-false and
				// administratorsOnly-false — which is what the update row's
				// "public true" has somewhere to move to.
				before := rawVolumeControl(t, "CE", volumeName)
				if before == nil {
					t.Fatalf("volume %q carries no resource control: every row here needs a victim to leave unchanged", volumeName)
				}
				controlID, ok := rawControlID(before)
				if !ok {
					t.Fatalf("the control on volume %q carries no Id: %v", volumeName, before)
				}
				if got, ok := controlBool(before, "Public"); !ok || got {
					t.Fatalf("control %d reads Public %v (present=%v) before the safe-mode call, want false: "+
						"the update row asks for true, and a victim already in that state could not fail", controlID, before["Public"], ok)
				}

				input := mutation.input(resourceID, controlID)
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

				// Nothing changed: the same control, still there, still with
				// the flags it had.
				after := rawVolumeControl(t, "CE", volumeName)
				if after == nil {
					t.Fatalf("volume %q carries no resource control after the safe-mode call: safe mode let %s through", volumeName, mutation.action)
				}
				if got, ok := rawControlID(after); !ok || got != controlID {
					t.Errorf("volume %q carries control %v (present=%v), want %d unchanged: safe mode let %s through",
						volumeName, after["Id"], ok, controlID, mutation.action)
				}
				for _, field := range []string{"Public", "AdministratorsOnly"} {
					want, wantOK := controlBool(before, field)
					got, gotOK := controlBool(after, field)
					if wantOK != gotOK || want != got {
						t.Errorf("control %d reads %s %v (present=%v), want %v (present=%v): safe mode let %s through",
							controlID, field, got, gotOK, want, wantOK, mutation.action)
					}
				}

				// And the leak the volume's own witness cannot see: a row
				// whose input names a resource id of its own would, if it
				// executed, leave a control hanging off that id rather than
				// off this volume. Only the create row does, and this is the
				// assertion that pins it.
				if leaked, ok := input["resourceId"].(string); ok {
					assertNoControlGoverns(t, "CE", leaked,
						fmt.Sprintf("safe mode let %s through: it created one over the id its input named", mutation.action))
				}
			})
		}
	}
}

// TestSafeMode_ResourceControls_TableCoversEveryMutatingAction fails when a
// mutating action is added to the domain without a row in
// resourceControlsSafeModeMutations.
//
// The table above is a hand-written list, and a hand-written list of what a
// domain declares goes stale silently: a fourth mutating action would simply
// never be previewed by anything, and every test in this file would stay
// green. This derives the expected set from the domain's own ActionSpecs —
// the same source safeModePreview reads Destructive from — so the table
// cannot fall behind the catalog without something failing. It checks the
// other direction too: a row naming an action that is not mutating at all
// would be asserting that safe mode intercepts something safe mode never
// sees.
func TestSafeMode_ResourceControls_TableCoversEveryMutatingAction(t *testing.T) {
	t.Parallel()

	want := map[string]string{}
	for _, spec := range resource_controls.Specs() {
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
	for _, mutation := range resourceControlsSafeModeMutations {
		got[mutation.action] = mutation.kind
	}

	for name, kind := range want {
		switch have, ok := got[name]; {
		case !ok:
			t.Errorf("%s is a mutating action with no row in resourceControlsSafeModeMutations: safe mode's interception of it is unproven", name)
		case have != kind:
			t.Errorf("resourceControlsSafeModeMutations declares %s as %q, but its ActionSpec flags imply %q", name, have, kind)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("resourceControlsSafeModeMutations has a row for %s, which is not a mutating action in this domain", name)
		}
	}
}

// volumeEnvID returns edition ed's own "docker" environment id, the one
// every volume in this file is created on.
//
// It fails rather than skips when the environment is missing: every compose
// leg registers one at provisioning time (cmd/provision/main.go), so its
// absence is a broken estate rather than an optional capability — the same
// judgement stacks_test.go's dockerEnvID makes, which this wraps by edition
// name because everything in this file is keyed by edition rather than by
// harness.Leg.
func volumeEnvID(t *testing.T, ed string) int {
	t.Helper()
	envID, ok := serverFor(t, ed).Environment(harness.EnvironmentDocker)
	if !ok {
		t.Fatalf("%s: estate has no %q environment", ed, harness.EnvironmentDocker)
	}
	return envID
}

// rawServerFor returns the provisioned server for an edition name without a
// *testing.T, for the ledger cleanup closures that only have a context.
// serverFor above is the *testing.T-friendly wrapper every test calls, and
// this stands in the same relation to it that rawClientFor stands in to
// fixtureClient.
func rawServerFor(ed string) (harness.Server, error) {
	for _, leg := range estate.Legs() {
		if leg.Name == ed {
			return leg.Server, nil
		}
	}
	return harness.Server{}, fmt.Errorf("no provisioned server for edition %q", ed)
}
