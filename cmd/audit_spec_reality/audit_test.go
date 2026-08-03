package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
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

	result, err := auditLeg(context.Background(), "TEST", srv.URL+"/api", ops, time.Second)
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

	result, err := auditLeg(context.Background(), "TEST", srv.URL+"/api", ops, time.Second)
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

	result, err := auditLeg(context.Background(), "TEST", srv.URL+"/api", ops, time.Second)
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
	_, err := auditLeg(context.Background(), "TEST", "http://127.0.0.1:1", ops, 500*time.Millisecond)
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

	result, err := auditLeg(context.Background(), "TEST", srv.URL+"/api", ops, time.Second)
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
