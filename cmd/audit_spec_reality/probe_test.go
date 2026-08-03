package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			body := bodyFor(method)
			if body == nil {
				t.Fatalf("bodyFor(%s) = nil, want a body", method)
			}
			buf := make([]byte, 8)
			n, _ := body.Read(buf)
			if string(buf[:n]) != "{}" {
				t.Errorf("bodyFor(%s) = %q, want \"{}\"", method, buf[:n])
			}
		})
	}
}

// TestUnit_BodyFor_ReadOnlyVerbs_CarryNoBody proves GET/HEAD/DELETE/OPTIONS
// carry nothing: see bodyFor's own doc comment for why there is nothing for
// a body to protect against on those verbs.
func TestUnit_BodyFor_ReadOnlyVerbs_CarryNoBody(t *testing.T) {
	t.Parallel()
	for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodDelete, http.MethodOptions} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()
			if body := bodyFor(method); body != nil {
				t.Errorf("bodyFor(%s) = non-nil, want nil", method)
			}
		})
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
		// t.Fatal/t.Fatalf here would call runtime.Goexit in this handler's
		// own goroutine (the httptest.Server's), not the test goroutine —
		// that stops the handler but leaves the test running and could mask
		// a real failure. Use t.Error/t.Errorf with an explicit return.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Error("test server does not support hijacking")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		_ = conn.Close()
	}))
	defer srv.Close()

	_, err := probe(context.Background(), &http.Client{}, time.Second, http.MethodGet, srv.URL, "/anything")
	if err == nil {
		t.Fatal("probe() error = nil, want an error: the server closed the connection with no response")
	}
}

// countingReadCloser counts every byte actually read from the underlying
// reader, so a test can observe how much of a response body probe() pulled
// off the wire, regardless of how much the server offered.
type countingReadCloser struct {
	io.ReadCloser
	n *int64
}

func (c countingReadCloser) Read(p []byte) (int, error) {
	n, err := c.ReadCloser.Read(p)
	atomic.AddInt64(c.n, int64(n))
	return n, err
}

// countingRoundTripper wraps every response body in a countingReadCloser
// sharing the same counter, without altering anything else about the
// response.
type countingRoundTripper struct {
	base http.RoundTripper
	n    *int64
}

func (rt countingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := rt.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = countingReadCloser{ReadCloser: resp.Body, n: rt.n}
	return resp, nil
}

// TestUnit_Probe_LargeResponseBody_ReadIsBoundedByMaxProbeBodyBytes is the
// mutation-evidence test for the bounded read: it proves probe() never pulls
// more than maxProbeBodyBytes off the wire, however much a server sends. The
// defect this guards against is real — the audit probes all 441 documented
// operations with probeConcurrency in flight at once, and some routes stream
// archives, backups or log tails, so several large bodies could be resident
// in memory simultaneously with an unbounded read.
func TestUnit_Probe_LargeResponseBody_ReadIsBoundedByMaxProbeBodyBytes(t *testing.T) {
	t.Parallel()
	const streamed = 64 * maxProbeBodyBytes // 256KiB: far larger than the 4KiB bound

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, _ := w.(http.Flusher)
		chunk := bytes.Repeat([]byte{'x'}, 4096)
		written := 0
		for written < streamed {
			n, err := w.Write(chunk)
			written += n
			if flusher != nil {
				flusher.Flush()
			}
			if err != nil {
				// The client (probe's bounded reader) stopped reading once
				// it had enough and closed the connection. Expected once
				// the fix is in place; nothing left to do.
				return
			}
		}
	}))
	defer srv.Close()

	var bytesRead int64
	client := &http.Client{Transport: countingRoundTripper{base: http.DefaultTransport, n: &bytesRead}}

	if _, err := probe(context.Background(), client, 5*time.Second, http.MethodGet, srv.URL, "/anything"); err != nil {
		t.Fatalf("probe() error = %v", err)
	}
	if bytesRead > maxProbeBodyBytes {
		t.Errorf("probe() read %d bytes off the wire, want at most maxProbeBodyBytes (%d): the server offered %d bytes total",
			bytesRead, maxProbeBodyBytes, streamed)
	}
}
