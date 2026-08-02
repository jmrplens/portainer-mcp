package system

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
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
	for _, s := range Specs() {
		if s.Name == name {
			return s.Handler
		}
	}
	t.Fatalf("action %q not declared", name)
	return nil
}

func TestSpecs_AreAllValid(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		if err := s.Validate(); err != nil {
			t.Errorf("%s: %v", s.Name, err)
		}
	}
}

// The interesting property of this domain: the mutating action differs by
// edition. CE has POST /system/upgrade, EE has POST /system/update.
func TestSpecs_MutatingActionsDifferByEdition(t *testing.T) {
	t.Parallel()
	byName := map[string]string{}
	for _, s := range Specs() {
		if s.Mutating {
			byName[s.Name] = string(s.Edition)
		}
	}
	if byName["system.upgrade"] != string(edition.CE) {
		t.Errorf("system.upgrade edition = %q, want CE", byName["system.upgrade"])
	}
	if byName["system.update"] != string(edition.EE) {
		t.Errorf("system.update edition = %q, want EE", byName["system.update"])
	}
}

// system.upgrade converts a Community Edition server to Business Edition and
// restarts it — a change this API offers no way to undo, the same property
// that makes tags.delete Destructive. system.update, by contrast, is a
// same-edition self-update rather than a one-way conversion, so it must stay
// merely Mutating.
func TestSpecs_Upgrade_IsDestructive(t *testing.T) {
	t.Parallel()
	for _, s := range Specs() {
		switch s.Name {
		case "system.upgrade":
			if !s.Mutating || !s.Destructive {
				t.Errorf("system.upgrade Mutating=%v Destructive=%v, want both true", s.Mutating, s.Destructive)
			}
		case "system.update":
			if s.Destructive {
				t.Error("system.update Destructive = true, want false: it is a same-edition self-update, not a one-way conversion")
			}
		}
	}
}

func TestSystemInfo_Success_ReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/system/info" {
			t.Errorf("path = %q, want /api/system/info", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"agents":2,"edgeAgents":1}`))
	})

	out, err := find(t, "system.info")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, _ := json.Marshal(out)
	if string(encoded) == "null" {
		t.Error("handler returned nothing for a successful response")
	}
}

func TestSystemInfo_Unauthorized_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized","details":"invalid API key"}`))
	})

	_, err := find(t, "system.info")(context.Background(), c, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 401 classified")
	}
	if !errors.Is(err, portainer.ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false; err = %v", err)
	}
}

func TestSystemUpgrade_Success_ReportsStarted(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "system.upgrade")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/system/upgrade" || gotMethod != http.MethodPost {
		t.Errorf("called %s %s, want POST /api/system/upgrade", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestSystemUpgrade_Forbidden_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Permission denied"}`))
	})

	_, err := find(t, "system.upgrade")(context.Background(), c, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 403 classified")
	}
	if !errors.Is(err, portainer.ErrForbidden) {
		t.Errorf("errors.Is(err, ErrForbidden) = false; err = %v", err)
	}
}
