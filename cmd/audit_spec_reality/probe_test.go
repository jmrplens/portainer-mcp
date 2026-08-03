package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestUnit_ResolvePath_ReplacesEveryPathParam proves every {param} segment is
// substituted, however many a template carries, regardless of name.
func TestUnit_ResolvePath_ReplacesEveryPathParam(t *testing.T) {
	t.Parallel()
	got := resolvePath("/kubernetes/{id}/volumes/{namespace}/{volume}")
	want := "/kubernetes/1/volumes/1/1"
	if got != want {
		t.Errorf("resolvePath() = %q, want %q", got, want)
	}
}

// TestUnit_ResolvePath_NoParams_ReturnsPathUnchanged is the boundary: a
// literal path with no template segment must round-trip untouched.
func TestUnit_ResolvePath_NoParams_ReturnsPathUnchanged(t *testing.T) {
	t.Parallel()
	got := resolvePath("/tags")
	if got != "/tags" {
		t.Errorf("resolvePath(%q) = %q, want it unchanged", "/tags", got)
	}
}

// TestUnit_IsRouteAbsent_ExactGoMuxLiteral_ReturnsTrue is the mechanism this
// whole command rests on: Go's own http.Error(w, "404 page not found", 404)
// — what net/http's DefaultServeMux answers an unmatched path with — must
// classify as an absent route.
func TestUnit_IsRouteAbsent_ExactGoMuxLiteral_ReturnsTrue(t *testing.T) {
	t.Parallel()
	if !isRouteAbsent(http.StatusNotFound, []byte("404 page not found\n")) {
		t.Fatal("isRouteAbsent() = false, want true for Go's literal default-mux 404 body")
	}
}

// TestUnit_IsRouteAbsent_JSONResourceNotFound_ReturnsFalse is the case this
// audit must never confuse with the one above: a matched route whose
// handler reports its own missing resource as Portainer's normal
// {"message","details"} JSON shape is evidence the route exists.
func TestUnit_IsRouteAbsent_JSONResourceNotFound_ReturnsFalse(t *testing.T) {
	t.Parallel()
	body := []byte(`{"message":"Registry not found","details":"the registry does not exist"}`)
	if isRouteAbsent(http.StatusNotFound, body) {
		t.Fatal("isRouteAbsent() = true, want false: this is a handled resource-missing response, not an absent route")
	}
}

// TestUnit_IsRouteAbsent_AuthFailure_ReturnsFalse covers the response every
// real, present route returns to this command's probe credential: a 401,
// which must never be mistaken for a 404 of either kind.
func TestUnit_IsRouteAbsent_AuthFailure_ReturnsFalse(t *testing.T) {
	t.Parallel()
	body := []byte(`{"message":"Invalid JWT token","details":"Unauthorized"}`)
	if isRouteAbsent(http.StatusUnauthorized, body) {
		t.Fatal("isRouteAbsent() = true, want false for a 401")
	}
}

// TestUnit_IsRouteAbsent_NotFoundButDifferentText_ReturnsFalse guards
// against a substring match: a 404 whose plain-text body merely contains
// "404" or "not found" as part of unrelated prose must not classify as the
// literal Go mux fallback.
func TestUnit_IsRouteAbsent_NotFoundButDifferentText_ReturnsFalse(t *testing.T) {
	t.Parallel()
	if isRouteAbsent(http.StatusNotFound, []byte("the requested widget could not be found")) {
		t.Fatal("isRouteAbsent() = true, want false: this text is not Go's exact literal")
	}
}

// TestUnit_BodyFor_MutatingVerbs_CarryEmptyJSONObject proves POST/PUT/PATCH
// get a syntactically valid, empty body.
func TestUnit_BodyFor_MutatingVerbs_CarryEmptyJSONObject(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodPatch} {
		body := bodyFor(method)
		if body == nil {
			t.Errorf("bodyFor(%s) = nil, want a body", method)
			continue
		}
		buf := make([]byte, 8)
		n, _ := body.Read(buf)
		if string(buf[:n]) != "{}" {
			t.Errorf("bodyFor(%s) = %q, want \"{}\"", method, buf[:n])
		}
	}
}

// TestUnit_BodyFor_ReadOnlyVerbs_CarryNoBody proves GET/HEAD/DELETE/OPTIONS
// carry nothing: see bodyFor's own doc comment for why there is nothing for
// a body to protect against on those verbs.
func TestUnit_BodyFor_ReadOnlyVerbs_CarryNoBody(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodOptions} {
		if body := bodyFor(method); body != nil {
			t.Errorf("bodyFor(%s) = non-nil, want nil", method)
		}
	}
}

// TestUnit_Probe_NeverSendsTheRealCredential proves the one property that
// makes every probe on every verb safe: the request always carries
// probeSentinelValue, never anything a caller passed in, no matter how the
// function is invoked. There is no parameter to pass a real credential
// through in the first place — this test exists so that fact stays true if
// probe's signature ever grows one.
func TestUnit_Probe_NeverSendsTheRealCredential(t *testing.T) {
	t.Parallel()
	var gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-API-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &http.Client{}
	if _, err := probe(context.Background(), client, time.Second, http.MethodGet, srv.URL, "/anything"); err != nil {
		t.Fatalf("probe() error = %v", err)
	}
	if gotKey != probeSentinelValue {
		t.Errorf("X-API-Key sent = %q, want the fixed probe credential %q", gotKey, probeSentinelValue)
	}
}

// TestUnit_Probe_AgainstRealDefaultServeMux_AbsentRoute_ClassifiesAsAbsent
// is the mechanism proven against the real thing it depends on: an actual
// http.ServeMux (the same fallback Portainer's own router falls through to)
// with no route registered for the probed path.
func TestUnit_Probe_AgainstRealDefaultServeMux_AbsentRoute_ClassifiesAsAbsent(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/registered", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := probe(context.Background(), &http.Client{}, time.Second, http.MethodGet, srv.URL, "/api/not-registered-at-all")
	if err != nil {
		t.Fatalf("probe() error = %v", err)
	}
	if !res.RouteAbsent {
		t.Errorf("probe() RouteAbsent = false, want true: status=%d", res.StatusCode)
	}
}

// TestUnit_Probe_AgainstRealDefaultServeMux_RegisteredRoute_ClassifiesAsPresent
// is that same real mux's positive control.
func TestUnit_Probe_AgainstRealDefaultServeMux_RegisteredRoute_ClassifiesAsPresent(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/registered", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Unauthorized", "details": "Unauthorized"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	res, err := probe(context.Background(), &http.Client{}, time.Second, http.MethodGet, srv.URL, "/api/registered")
	if err != nil {
		t.Fatalf("probe() error = %v", err)
	}
	if res.RouteAbsent {
		t.Fatal("probe() RouteAbsent = true, want false: this route is registered and answered with its own JSON body")
	}
	if res.StatusCode != http.StatusUnauthorized {
		t.Errorf("probe() StatusCode = %d, want 401", res.StatusCode)
	}
}

// TestUnit_Probe_TransportFailure_ReturnsError covers a server that closes
// the connection outright: probe must surface this as an error, not silently
// classify it as anything.
func TestUnit_Probe_TransportFailure_ReturnsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Fatal("test server does not support hijacking")
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Fatalf("hijack: %v", err)
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	_, err := probe(context.Background(), &http.Client{}, time.Second, http.MethodGet, srv.URL, "/anything")
	if err == nil {
		t.Fatal("probe() error = nil, want an error: the server closed the connection with no response")
	}
}
