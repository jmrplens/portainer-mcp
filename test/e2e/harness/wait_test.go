package harness

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestWaitReady_ServerBecomesReady_ReturnsVersion(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// The first two attempts fail the way a starting Portainer does:
		// the port is open but the API is not serving yet.
		if calls.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"abc"}`))
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	ready, err := WaitReady(ctx, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("WaitReady() error = %v", err)
	}
	if ready.Version != "2.44.0" {
		t.Errorf("version = %q, want %q", ready.Version, "2.44.0")
	}
	if ready.InstanceID != "abc" {
		t.Errorf("instance id = %q, want %q", ready.InstanceID, "abc")
	}
	if got := calls.Load(); got < 3 {
		t.Errorf("attempts = %d, want at least 3: WaitReady gave up too early or did not retry", got)
	}
}

func TestWaitReady_EmptyVersion_IsNotTreatedAsReady(t *testing.T) {
	t.Parallel()
	// A 200 with no version is a server that is answering but not yet
	// serving. Accepting it hands the caller an address that will fail on
	// the very next call, with a confusing error far from the cause.
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) <= 2 {
			_, _ = w.Write([]byte(`{"Version":"","InstanceID":""}`))
			return
		}
		_, _ = w.Write([]byte(`{"Version":"2.44.0","InstanceID":"abc"}`))
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	ready, err := WaitReady(ctx, server.Client(), server.URL)
	if err != nil {
		t.Fatalf("WaitReady() error = %v", err)
	}
	if ready.Version != "2.44.0" {
		t.Errorf("version = %q: an empty version was accepted as ready", ready.Version)
	}
	if got := calls.Load(); got < 3 {
		t.Errorf("attempts = %d, want at least 3: the empty-version responses were treated as ready", got)
	}
}

func TestWaitReady_NeverReady_ReturnsErrorAtDeadline(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	t.Cleanup(cancel)

	start := time.Now()
	if _, err := WaitReady(ctx, server.Client(), server.URL); err == nil {
		t.Fatal("WaitReady() error = nil, want an error when the server never becomes ready")
	}
	// The bug this guards: a fixed retry count that ignores the context and
	// runs for minutes after the caller has given up.
	if elapsed := time.Since(start); elapsed > 3*time.Second {
		t.Errorf("WaitReady took %v after a 300ms deadline: it is not honouring the context", elapsed)
	}
}

func TestWaitReady_ConnectionRefused_IsRetriedNotFatal(t *testing.T) {
	t.Parallel()
	// A port with nothing behind it: exactly what the first moments of
	// "docker run" look like. This must be a retry, not an immediate failure.
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	t.Cleanup(cancel)

	start := time.Now()
	_, err := WaitReady(ctx, http.DefaultClient, "http://127.0.0.1:1")
	if err == nil {
		t.Fatal("WaitReady() error = nil, want an error")
	}
	if elapsed := time.Since(start); elapsed < 200*time.Millisecond {
		t.Errorf("WaitReady returned after %v: a refused connection was treated as fatal rather than retried", elapsed)
	}
}
