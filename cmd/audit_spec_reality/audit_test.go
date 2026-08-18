package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// registeredMux builds an http.ServeMux carrying handlers only for the given
// paths (each answering 401, Portainer's real answer to this command's
// probe credential), leaving every other path to Go's real default
// not-found fallback — the same fallback this whole command's mechanism
// depends on, exercised here for real rather than faked.
func registeredMux(paths ...string) *http.ServeMux {
	mux := http.NewServeMux()
	for _, p := range paths {
		mux.HandleFunc(p, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Invalid JWT token","details":"Unauthorized"}`))
		})
	}
	return mux
}

// TestUnit_AuditLeg_KnownAbsentOperation_ReportedAsDivergent is this
// package's central positive proof: an operation the fixture "spec" (ops)
// declares, but whose path the fake server never registers a handler for,
// must be classified as absent and appear in Divergent — not merely pass
// some other assertion incidentally.
func TestUnit_AuditLeg_KnownAbsentOperation_ReportedAsDivergent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(registeredMux("/api/tags"))
	defer srv.Close()

	ops := map[string]specOperation{
		"TagsList":         {OperationID: "TagsList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
		"NeverImplemented": {OperationID: "NeverImplemented", Method: http.MethodGet, Path: "/nowhere/at/all", Domain: "ghost"},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.Divergent) != 1 || result.Divergent[0].OperationID != "NeverImplemented" {
		t.Fatalf("auditLeg() Divergent = %+v, want exactly [NeverImplemented]", result.Divergent)
	}
}

// TestUnit_AuditLeg_PresentOperation_NotReportedAsDivergent is the negative
// control for the same fixture: the operation that IS registered must not
// appear in Divergent.
func TestUnit_AuditLeg_PresentOperation_NotReportedAsDivergent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(registeredMux("/api/tags"))
	defer srv.Close()

	ops := map[string]specOperation{
		"TagsList": {OperationID: "TagsList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.Divergent) != 0 {
		t.Errorf("auditLeg() Divergent = %+v, want empty: TagsList is registered", result.Divergent)
	}
	if result.Total != 1 {
		t.Errorf("auditLeg() Total = %d, want 1", result.Total)
	}
}

// stubCatchAllServer answers every request — including the self-test's
// manufactured canary path — as though it were a real, present route. It
// models the exact failure this project's own history warns is
// indistinguishable from a clean run unless specifically guarded against: a
// broken or misconfigured probe (a proxy in front, a wrong base URL that
// happens to resolve somewhere that answers everything, a server that 200s
// unconditionally) which would otherwise make every single operation look
// served, "no divergence", when in truth nothing was ever really checked.
func stubCatchAllServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
}

// TestUnit_AuditLeg_SelfTestCannotDetectAbsence_RefusesToReport is the
// mutation-proof this package's standing warning demands: with the
// detection mechanism broken (a server that answers the manufactured
// canary path as though it were real), auditLeg must refuse outright
// rather than report a clean "no divergence" that would be indistinguishable
// from a real one.
func TestUnit_AuditLeg_SelfTestCannotDetectAbsence_RefusesToReport(t *testing.T) {
	t.Parallel()
	srv := stubCatchAllServer(t)
	defer srv.Close()

	ops := map[string]specOperation{
		"TagsList": {OperationID: "TagsList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err == nil {
		t.Fatalf("auditLeg() error = nil, want an error: the self-test canary was answered as a present route, so nothing this run reports can be trusted (got result=%+v)", result)
	}
	if !strings.Contains(err.Error(), "self-test failed") {
		t.Errorf("auditLeg() error = %v, want it to name the self-test failure", err)
	}
}

// TestUnit_AuditLeg_SelfTestProbeCannotRun_ReturnsError covers the sibling
// failure mode: the self-test request itself cannot complete (server
// unreachable), which must also refuse to report rather than silently
// treating an empty result as "nothing found".
func TestUnit_AuditLeg_SelfTestProbeCannotRun_ReturnsError(t *testing.T) {
	t.Parallel()
	ops := map[string]specOperation{
		"TagsList": {OperationID: "TagsList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
	}
	// Port 1 is reserved; nothing in this test environment listens there,
	// so the connection is refused immediately rather than timing out slowly.
	_, err := auditLeg(context.Background(), io.Discard, "TEST", "http://127.0.0.1:1", ops, 500*time.Millisecond)
	if err == nil {
		t.Fatal("auditLeg() error = nil, want an error: the server is unreachable")
	}
}

// TestUnit_AuditLeg_TransportFailure_RecordedSeparatelyFromDivergence proves
// an operation whose probe cannot complete is recorded as a probe error, not
// folded into Divergent: a probe with no answer says nothing about whether
// the route exists, and counting it as a divergence would overstate what was
// actually observed. The self-test path and TagsList both answer normally on
// this fixture; only HangForever's handler hijacks the connection and closes
// it without ever writing a response.
func TestUnit_AuditLeg_TransportFailure_RecordedSeparatelyFromDivergence(t *testing.T) {
	t.Parallel()
	mux := registeredMux("/api/tags")
	mux.HandleFunc("/api/hangforever", func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		_ = conn.Close()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ops := map[string]specOperation{
		"TagsList":    {OperationID: "TagsList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
		"HangForever": {OperationID: "HangForever", Method: http.MethodGet, Path: "/hangforever", Domain: "x"},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.ProbeErrors) != 1 || !strings.Contains(result.ProbeErrors[0], "HangForever") {
		t.Fatalf("auditLeg() ProbeErrors = %v, want exactly one entry naming HangForever", result.ProbeErrors)
	}
	if len(result.Divergent) != 0 {
		t.Errorf("auditLeg() Divergent = %+v, want empty: HangForever's failure is a transport error, not evidence of an absent route", result.Divergent)
	}
}

// recordingServer answers like a real Portainer for the two things auditLeg
// asks about — the admin-check route and any registered path — and records
// every request it receives so a test can assert on what was, and was not,
// sent. adminExists decides whether the estate reports itself initialized.
//
// Requests are recorded under a mutex: probes run probeConcurrency-way
// concurrent, so an unguarded append would race under -race.
func recordingServer(t *testing.T, adminExists bool, registered ...string) (*httptest.Server, func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string

	reg := make(map[string]bool, len(registered))
	for _, p := range registered {
		reg[p] = true
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Method+" "+r.URL.Path)
		mu.Unlock()

		switch {
		case r.URL.Path == "/api"+adminCheckPath:
			if adminExists {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.WriteHeader(http.StatusNotFound)
			return
		case reg[r.URL.Path]:
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Invalid JWT token","details":"Unauthorized"}`))
			return
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		out := make([]string, len(seen))
		copy(out, seen)
		return out
	}
}

// TestUnit_AuditLeg_UninitializedEstate_SkipsPublicRoutesButProbesTheRest is
// I8's gate. A PublicAccess route has no credential check to reject the
// sentinel, so the safety argument covering every other route does not cover
// it; on an uninitialized estate a probe would reach real handler code. The
// audit must decline to send it.
//
// The authenticated operation alongside it must still be probed, or "safe"
// would have been achieved by doing nothing.
func TestUnit_AuditLeg_UninitializedEstate_SkipsPublicRoutesButProbesTheRest(t *testing.T) {
	t.Parallel()
	srv, requests := recordingServer(t, false, "/api/tags")
	ops := map[string]specOperation{
		"Restore": {OperationID: "Restore", Method: http.MethodPost, Path: "/restore", Domain: "backup", Public: true},
		"TagList": {OperationID: "TagList", Method: http.MethodGet, Path: "/tags", Domain: "tags"},
	}

	var warnings strings.Builder
	result, err := auditLeg(context.Background(), &warnings, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}

	for _, req := range requests() {
		if strings.Contains(req, "/restore") {
			t.Errorf("the audit sent %q against an estate it could not confirm is initialized; /restore is a PublicAccess route with no credential check to stop it", req)
		}
	}
	var probedTags bool
	for _, req := range requests() {
		if req == "GET /api/tags" {
			probedTags = true
		}
	}
	if !probedTags {
		t.Error("the authenticated operation was not probed either; the gate must skip public routes, not stop the audit")
	}

	if len(result.SkippedPublic) != 1 || !strings.Contains(result.SkippedPublic[0], "Restore") {
		t.Errorf("SkippedPublic = %v, want exactly one entry naming Restore: a route that was never probed must be reported as unmeasured, not counted as served", result.SkippedPublic)
	}
	if len(result.Divergent) != 0 {
		t.Errorf("Divergent = %+v, want empty: a skipped route is not evidence of an absent one", result.Divergent)
	}
	if !strings.Contains(warnings.String(), "PublicAccess") {
		t.Errorf("warnings = %q, want a visible, named warning about the skipped public routes", warnings.String())
	}
}

// TestUnit_AuditLeg_InitializedEstate_ProbesPublicRoutes is the discriminating
// other half. Without it, a gate that skipped every public route
// unconditionally — or skipped everything — would pass the test above and
// silently stop measuring 24 of the EE specification's operations forever.
func TestUnit_AuditLeg_InitializedEstate_ProbesPublicRoutes(t *testing.T) {
	t.Parallel()
	srv, requests := recordingServer(t, true, "/api/restore")
	ops := map[string]specOperation{
		"Restore": {OperationID: "Restore", Method: http.MethodPost, Path: "/restore", Domain: "backup", Public: true},
	}

	var warnings strings.Builder
	result, err := auditLeg(context.Background(), &warnings, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}

	var probed bool
	for _, req := range requests() {
		if req == "POST /api/restore" {
			probed = true
		}
	}
	if !probed {
		t.Error("the audit skipped /restore against an estate that reports an administrator account; an initialized estate is exactly the condition that makes probing it safe")
	}
	if len(result.SkippedPublic) != 0 {
		t.Errorf("SkippedPublic = %v, want empty on an initialized estate", result.SkippedPublic)
	}
}

// TestUnit_ParseSpecOperations_DerivesPublicFromTheDocument pins the other
// half of the gate: it is worth nothing if Public is wrong. Checked against
// the real vendored specification rather than a fixture, because the whole
// point of deriving this from the document is that it tracks the document.
func TestUnit_ParseSpecOperations_DerivesPublicFromTheDocument(t *testing.T) {
	t.Parallel()
	specPath := filepath.Join("..", "..", specsDir, fmt.Sprintf("ee-%s.json", defaultSpecVer))
	data, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("read %s: %v", specPath, err)
	}
	ops, err := parseSpecOperations(data)
	if err != nil {
		t.Fatalf("parseSpecOperations() error = %v", err)
	}

	// Declared with no security requirement at all in the vendored document.
	for _, name := range []string{
		"Restore", "UserAdminInit", "AuthenticateUser", "ValidateOAuth",
		"EndpointCreateGlobalKey", "EdgeStackStatusUpdate", "WebhookExecute",
		"EdgeStackWebhookInvoke", "StacksWebhookInvoke", "SystemUpdate",
	} {
		op, ok := ops[name]
		if !ok {
			t.Errorf("operation %q not found in the vendored EE spec", name)
			continue
		}
		if !op.Public {
			t.Errorf("%s (%s %s).Public = false, want true: the document declares no security requirement for it", name, op.Method, op.Path)
		}
	}

	// Declared with an explicit security requirement, so the ordinary
	// auth-rejection argument does cover them.
	for _, name := range []string{"TagList", "RegistryInspect", "WebhookList", "Logout"} {
		op, ok := ops[name]
		if !ok {
			t.Errorf("operation %q not found in the vendored EE spec", name)
			continue
		}
		if op.Public {
			t.Errorf("%s (%s %s).Public = true, want false: the document declares a security requirement for it", name, op.Method, op.Path)
		}
	}
}

// verbRestrictedMux registers path so that only served answers normally and
// every other method answers 405, the way a real router does, while leaving
// every unregistered path to Go's own not-found fallback.
//
// The 401 for the accepted verb is deliberate and matters: it is what a real
// Portainer answers this command's sentinel credential, so a test that
// distinguishes "wrong verb" from "served" here is distinguishing the same
// two things a live run has to.
func verbRestrictedMux(path, served string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != served {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Invalid JWT token","details":"Unauthorized"}`))
	})
	return mux
}

// TestUnit_AuditLeg_OperationDocumentedUnderTheWrongVerb_IsReportedAndNotCalledAbsent
// is the finding this whole classification exists for, reproduced from the
// real case that motivated it: the vendored documents declare
// EndpointAssociationDelete as PUT /endpoints/{id}/association, and a live
// Portainer 2.44.0 serves that path for DELETE only.
//
// Two assertions, and the second is the one that would have caught the
// original miss: before this change the operation was classified as SERVED,
// because isRouteAbsent only recognises Go's literal "404 page not found"
// and a 405 is not it. Reporting it as divergent would have been just as
// wrong in the other direction — the route exists.
func TestUnit_AuditLeg_OperationDocumentedUnderTheWrongVerb_IsReportedAndNotCalledAbsent(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(verbRestrictedMux("/api/endpoints/1/association", http.MethodDelete))
	defer srv.Close()

	ops := map[string]specOperation{
		"EndpointAssociationDelete": {
			OperationID: "EndpointAssociationDelete",
			Method:      http.MethodPut,
			Path:        "/endpoints/{id}/association",
			Domain:      "endpoints",
		},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.Divergent) != 0 {
		t.Errorf("auditLeg() Divergent = %+v, want empty: the path IS registered, just not for PUT", result.Divergent)
	}
	if len(result.WrongVerb) != 1 {
		t.Fatalf("auditLeg() WrongVerb = %+v, want exactly one finding", result.WrongVerb)
	}
	got := result.WrongVerb[0]
	if got.OperationID != "EndpointAssociationDelete" || got.Method != http.MethodPut {
		t.Errorf("auditLeg() WrongVerb[0] = %+v, want the documented operation and its documented verb", got)
	}
	if len(got.ServedBy) != 1 || got.ServedBy[0] != http.MethodDelete {
		t.Errorf("auditLeg() WrongVerb[0].ServedBy = %v, want [DELETE]: naming the verb that does work is the point", got.ServedBy)
	}
}

// TestUnit_AuditLeg_OperationServedUnderItsOwnVerb_IsNotAWrongVerbFinding is
// the negative control: the same fixture, probed with the verb the router
// accepts, must produce no finding of either kind.
func TestUnit_AuditLeg_OperationServedUnderItsOwnVerb_IsNotAWrongVerbFinding(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(verbRestrictedMux("/api/endpoints/1/association", http.MethodDelete))
	defer srv.Close()

	ops := map[string]specOperation{
		"EndpointAssociationDelete": {
			OperationID: "EndpointAssociationDelete",
			Method:      http.MethodDelete,
			Path:        "/endpoints/{id}/association",
			Domain:      "endpoints",
		},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.WrongVerb) != 0 || len(result.Divergent) != 0 {
		t.Errorf("auditLeg() WrongVerb = %+v, Divergent = %+v, want both empty", result.WrongVerb, result.Divergent)
	}
}

// TestUnit_AuditLeg_PublicRouteUnderTheWrongVerb_IsReportedButNeverSwept
// holds the safety restriction in place.
//
// A PublicAccess route has no credential check, so the argument that makes
// probing an arbitrary verb harmless — Portainer rejects the sentinel before
// any handler runs — does not cover it (see verbsServing's doc comment). The
// finding is still reported, because observing the 405 costs nothing beyond
// the probe the audit already makes; what must not happen is the follow-up
// sweep. ServedBy staying empty is how that is visible.
func TestUnit_AuditLeg_PublicRouteUnderTheWrongVerb_IsReportedButNeverSwept(t *testing.T) {
	t.Parallel()
	var methods []string
	var mu sync.Mutex
	// A ServeMux, not a bare handler: every path this test does not register
	// must reach Go's own not-found fallback, which is what auditLeg's
	// self-test canary probes before it will report anything at all.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/users/admin/init", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		methods = append(methods, r.Method)
		mu.Unlock()
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		// What an already-initialized Portainer answers this route: its own
		// refusal, not an absent route. See this command's package doc.
		w.WriteHeader(http.StatusBadRequest)
	})
	mux.HandleFunc("/api"+adminCheckPath, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ops := map[string]specOperation{
		"UserAdminInit": {
			OperationID: "UserAdminInit",
			Method:      http.MethodPut,
			Path:        "/users/admin/init",
			Domain:      "users",
			Public:      true,
		},
	}

	result, err := auditLeg(context.Background(), io.Discard, "TEST", srv.URL+"/api", ops, time.Second)
	if err != nil {
		t.Fatalf("auditLeg() error = %v", err)
	}
	if len(result.WrongVerb) != 1 {
		t.Fatalf("auditLeg() WrongVerb = %+v, want the finding to be reported", result.WrongVerb)
	}
	if got := result.WrongVerb[0].ServedBy; len(got) != 0 {
		t.Errorf("auditLeg() swept verbs on a PublicAccess route (ServedBy = %v); it must never do that", got)
	}

	mu.Lock()
	defer mu.Unlock()
	// Only the route under test records anything, so this counts probes of
	// that route alone: the one PUT is the operation's own probe, and any
	// POST, PATCH or DELETE beside it would be the sweep that must not run.
	if len(methods) != 1 || methods[0] != http.MethodPut {
		t.Errorf("the route under test saw methods %v; want exactly one PUT (the probe itself) and no sweep", methods)
	}
}
