package tags

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	"github.com/jmrplens/portainer-mcp/internal/tools"
	"github.com/jmrplens/portainer-mcp/internal/toolutil"
)

func clientFor(t *testing.T, handler http.HandlerFunc) *portainer.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c, err := portainer.New(&config.Config{URL: server.URL, Token: "t", ToolSurface: config.SurfaceDynamic})
	if err != nil {
		t.Fatalf("portainer.New: %v", err)
	}
	return c
}

func find(t *testing.T, name string) func(context.Context, *portainer.Client, json.RawMessage) (any, error) {
	t.Helper()
	return findSpec(t, name).Handler
}

// findSpec returns the full declared ActionSpec for name, rather than just
// its handler — needed by tests that must route through tools.Execute (which
// takes the whole spec, to run the same schema validation every real caller
// goes through) instead of calling the handler directly.
func findSpec(t *testing.T, name string) toolutil.ActionSpec {
	t.Helper()
	for _, s := range Specs() {
		if s.Name == name {
			return s
		}
	}
	t.Fatalf("action %q not declared", name)
	return toolutil.ActionSpec{}
}

func TestSpecs_AreAllValid(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}

// tags.delete is the first genuinely destructive action in the catalog, so it
// is where the annotation chain is worth checking end to end.
func TestSpecs_Delete_IsMutatingAndDestructive(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if s.Name != "tags.delete" {
			continue
		}
		if !s.Mutating || !s.Destructive {
			t.Errorf("tags.delete Mutating=%v Destructive=%v, want both true", s.Mutating, s.Destructive)
		}
		return
	}
	t.Fatal("tags.delete is not declared")
}

// The other two actions must NOT be marked destructive, or safe mode would
// intercept read-only and additive operations as if they were dangerous.
func TestSpecs_ListAndCreate_AreNotDestructive(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		switch s.Name {
		case "tags.list":
			if s.Mutating || s.Destructive {
				t.Errorf("tags.list Mutating=%v Destructive=%v, want both false", s.Mutating, s.Destructive)
			}
		case "tags.create":
			if !s.Mutating {
				t.Error("tags.create Mutating = false, want true")
			}
			if s.Destructive {
				t.Error("tags.create Destructive = true, want false: creating is additive, not destructive")
			}
		}
	}
}

func TestTagList_Success_ReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" {
			t.Errorf("path = %q, want /api/tags", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":1,"Name":"prod"}]`))
	})

	out, err := find(t, "tags.list")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, _ := json.Marshal(out)
	if string(encoded) == "null" {
		t.Error("handler returned nothing for a successful response")
	}
}

func TestTagList_Unauthorized_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized","details":"invalid API key"}`))
	})

	_, err := find(t, "tags.list")(context.Background(), c, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 401 classified")
	}
	if !errors.Is(err, portainer.ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false; err = %v", err)
	}
}

func TestTagCreate_Success_SendsNameAndReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tags" || r.Method != http.MethodPost {
			t.Errorf("called %s %s, want POST /api/tags", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":7,"Name":"prod"}`))
	})

	out, err := find(t, "tags.create")(context.Background(), c, json.RawMessage(`{"name":"prod"}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotBody["Name"] != "prod" {
		t.Errorf("request body Name = %v, want %q", gotBody["Name"], "prod")
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

// TestTagCreate_MissingName_ReturnsErrorWithoutCallingAPI pins a
// required-field check. tags.create now runs on generated code (see this
// file's package doc): its generated handler deliberately carries no such
// guard, because schema validation now runs once, centrally, in
// tools.Execute, for every surface. So this routes through tools.Execute, the
// path every real caller actually takes, rather than calling the handler
// directly and asserting a guard clause the handler no longer has; the
// property under test (missing required input is refused before the API is
// ever called) is unchanged.
func TestUnit_TagCreate_MissingName_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

	spec := findSpec(t, "tags.create")
	result, err := tools.Execute(context.Background(), spec, tools.Deps{Client: c}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute error = %v", err)
	}
	if !result.IsError {
		t.Fatal("result.IsError = false, want true for a missing name")
	}
	if called.Load() {
		t.Error("the API was called despite missing required input")
	}
}

func TestTagCreate_Conflict_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"A tag with this name already exists"}`))
	})

	_, err := find(t, "tags.create")(context.Background(), c, json.RawMessage(`{"name":"prod"}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 409 classified")
	}
}

func TestTagDelete_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "tags.delete")(context.Background(), c, json.RawMessage(`{"id":3}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/tags/3" || gotMethod != http.MethodDelete {
		t.Errorf("called %s %s, want DELETE /api/tags/3", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

// TestTagDelete_InvalidID_ReturnsErrorWithoutCallingAPI pins the
// non-positive-id guard. tags.delete now runs on generated code (see this
// file's package doc): the guard now lives in the generated input schema's
// "minimum": 1 constraint on "id" (toolutil.MinimumParams), enforced by
// tools.Execute before any handler runs, so this routes through it rather
// than calling the handler directly.
func TestUnit_TagDelete_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	spec := findSpec(t, "tags.delete")
	for _, id := range []int{0, -1} {
		called := false
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

		input, _ := json.Marshal(map[string]any{"id": id})
		result, err := tools.Execute(context.Background(), spec, tools.Deps{Client: c}, input)
		if err != nil {
			t.Fatalf("id=%d: Execute error = %v", id, err)
		}
		if !result.IsError {
			t.Errorf("id=%d: result.IsError = false, want true for a non-positive id", id)
		}
		if called {
			t.Errorf("id=%d: the API was called despite an invalid id", id)
		}
	}
}

func TestTagDelete_Forbidden_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Permission denied"}`))
	})

	_, err := find(t, "tags.delete")(context.Background(), c, json.RawMessage(`{"id":3}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 403 classified")
	}
	if !errors.Is(err, portainer.ErrForbidden) {
		t.Errorf("errors.Is(err, ErrForbidden) = false; err = %v", err)
	}
}
