package registries

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/edition"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
	apigen "github.com/jmrplens/portainer-mcp/internal/portainer/gen"
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

// registries is the pilot for gating inside a domain: some actions exist in
// both editions and some only in Business Edition.
func TestSpecs_EditionsSplitWithinTheDomain(t *testing.T) {
	t.Parallel()
	var ce, ee int
	for _, s := range Specs() {
		switch s.Edition {
		case edition.CE:
			ce++
		case edition.EE:
			ee++
		}
	}
	if ce == 0 || ee == 0 {
		t.Errorf("editions = %d CE / %d EE, want both present — this domain is the gating pilot", ce, ee)
	}
}

// Pin the exact split the brief's Step 1 greps recorded: 7 CE actions,
// 3 EE-only actions (ecr_delete_repository, ecr_delete_tags,
// repository_tags_delete).
func TestSpecs_EditionCounts_MatchTheVendoredSpecs(t *testing.T) {
	t.Parallel()
	var ce, ee int
	for _, s := range Specs() {
		switch s.Edition {
		case edition.CE:
			ce++
		case edition.EE:
			ee++
		}
	}
	if ce != 7 {
		t.Errorf("CE actions = %d, want 7", ce)
	}
	if ee != 3 {
		t.Errorf("EE actions = %d, want 3", ee)
	}
}

// Every DELETE must be both Mutating and Destructive; every POST/PUT must be
// Mutating. GET actions (list, inspect) must be neither.
func TestSpecs_MutatingAndDestructiveFlags_MatchHTTPSemantics(t *testing.T) {
	t.Parallel()
	destructive := map[string]bool{
		"registries.delete":                 true,
		"registries.ecr_delete_repository":  true,
		"registries.ecr_delete_tags":        true,
		"registries.repository_tags_delete": true,
	}
	mutating := map[string]bool{
		"registries.create":                 true,
		"registries.ping":                   true,
		"registries.update":                 true,
		"registries.configure":              true,
		"registries.delete":                 true,
		"registries.ecr_delete_repository":  true,
		"registries.ecr_delete_tags":        true,
		"registries.repository_tags_delete": true,
	}

	for _, s := range Specs() {
		if s.Mutating != mutating[s.Name] {
			t.Errorf("%s Mutating = %v, want %v", s.Name, s.Mutating, mutating[s.Name])
		}
		if s.Destructive != destructive[s.Name] {
			t.Errorf("%s Destructive = %v, want %v", s.Name, s.Destructive, destructive[s.Name])
		}
	}
}

func TestRegistryList_Success_ReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/registries" {
			t.Errorf("path = %q, want /api/registries", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":1,"Name":"docker-hub"}]`))
	})

	out, err := find(t, "registries.list")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, _ := json.Marshal(out)
	if string(encoded) == "null" {
		t.Error("handler returned nothing for a successful response")
	}
}

func TestRegistryList_Unauthorized_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"message":"Unauthorized"}`))
	})

	_, err := find(t, "registries.list")(context.Background(), c, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 401 classified")
	}
	if !errors.Is(err, portainer.ErrUnauthorized) {
		t.Errorf("errors.Is(err, ErrUnauthorized) = false; err = %v", err)
	}
}

func TestRegistryCreate_Success_SendsFieldsAndReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/registries" || r.Method != http.MethodPost {
			t.Errorf("called %s %s, want POST /api/registries", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":9,"Name":"my-registry"}`))
	})

	input, _ := json.Marshal(map[string]any{"name": "my-registry", "url": "registry.example.com", "type": 3})
	out, err := find(t, "registries.create")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotBody["Name"] != "my-registry" || gotBody["URL"] != "registry.example.com" {
		t.Errorf("request body = %+v, want Name/URL forwarded", gotBody)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryCreate_MissingFields_ReturnErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"missing name", map[string]any{"url": "registry.example.com", "type": 3}},
		{"missing url", map[string]any{"name": "my-registry", "type": 3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.create")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

func TestRegistryPing_Success_ReturnsDecodedBody(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/registries/ping" || r.Method != http.MethodPost {
			t.Errorf("called %s %s, want POST /api/registries/ping", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"ok"}`))
	})

	input, _ := json.Marshal(map[string]any{"url": "registry.example.com", "type": 3})
	out, err := find(t, "registries.ping")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryPing_MissingURL_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	called := false
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

	_, err := find(t, "registries.ping")(context.Background(), c, json.RawMessage(`{"type":3}`))
	if err == nil {
		t.Fatal("handler error = nil, want an error for a missing url")
	}
	if called {
		t.Error("the API was called despite missing required input")
	}
}

func TestRegistryInspect_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":4,"Name":"quay"}`))
	})

	out, err := find(t, "registries.inspect")(context.Background(), c, json.RawMessage(`{"id":4}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4" {
		t.Errorf("path = %q, want /api/registries/4", gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryInspect_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	for _, id := range []int{0, -1} {
		called := false
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

		input, _ := json.Marshal(map[string]any{"id": id})
		_, err := find(t, "registries.inspect")(context.Background(), c, input)
		if err == nil {
			t.Errorf("id=%d: handler error = nil, want an error for a non-positive id", id)
		}
		if called {
			t.Errorf("id=%d: the API was called despite an invalid id", id)
		}
	}
}

func TestRegistryUpdate_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":4,"Name":"quay"}`))
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "name": "quay", "url": "quay.io"})
	out, err := find(t, "registries.update")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4" || gotMethod != http.MethodPut {
		t.Errorf("called %s %s, want PUT /api/registries/4", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryUpdate_MissingFields_ReturnErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"invalid id", map[string]any{"id": 0, "name": "quay", "url": "quay.io"}},
		{"missing name", map[string]any{"id": 4, "url": "quay.io"}},
		{"missing url", map[string]any{"id": 4, "name": "quay"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.update")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

func TestRegistryConfigure_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "registries.configure")(context.Background(), c, json.RawMessage(`{"id":4,"authentication":true,"username":"u","password":"p"}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4/configure" || gotMethod != http.MethodPost {
		t.Errorf("called %s %s, want POST /api/registries/4/configure", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryConfigure_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	called := false
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

	_, err := find(t, "registries.configure")(context.Background(), c, json.RawMessage(`{"id":0}`))
	if err == nil {
		t.Fatal("handler error = nil, want an error for a non-positive id")
	}
	if called {
		t.Error("the API was called despite an invalid id")
	}
}

func TestRegistryDelete_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	out, err := find(t, "registries.delete")(context.Background(), c, json.RawMessage(`{"id":4}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4" || gotMethod != http.MethodDelete {
		t.Errorf("called %s %s, want DELETE /api/registries/4", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRegistryDelete_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	for _, id := range []int{0, -1} {
		called := false
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

		input, _ := json.Marshal(map[string]any{"id": id})
		_, err := find(t, "registries.delete")(context.Background(), c, input)
		if err == nil {
			t.Errorf("id=%d: handler error = nil, want an error for a non-positive id", id)
		}
		if called {
			t.Errorf("id=%d: the API was called despite an invalid id", id)
		}
	}
}

func TestRegistryDelete_Forbidden_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Permission denied"}`))
	})

	_, err := find(t, "registries.delete")(context.Background(), c, json.RawMessage(`{"id":4}`))
	if err == nil {
		t.Fatal("handler error = nil, want the 403 classified")
	}
	if !errors.Is(err, portainer.ErrForbidden) {
		t.Errorf("errors.Is(err, ErrForbidden) = false; err = %v", err)
	}
}

func TestEcrDeleteRepository_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		w.WriteHeader(http.StatusNoContent)
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "repositoryName": "my-repo"})
	out, err := find(t, "registries.ecr_delete_repository")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4/ecr/repositories/my-repo" || gotMethod != http.MethodDelete {
		t.Errorf("called %s %s, want DELETE /api/registries/4/ecr/repositories/my-repo", gotMethod, gotPath)
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestEcrDeleteRepository_MissingFields_ReturnErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"invalid id", map[string]any{"id": 0, "repositoryName": "my-repo"}},
		{"missing repositoryName", map[string]any{"id": 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.ecr_delete_repository")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

func TestEcrDeleteRepository_Forbidden_ReturnsClassifiedError(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"Permission denied"}`))
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "repositoryName": "my-repo"})
	_, err := find(t, "registries.ecr_delete_repository")(context.Background(), c, input)
	if err == nil {
		t.Fatal("handler error = nil, want the 403 classified")
	}
	if !errors.Is(err, portainer.ErrForbidden) {
		t.Errorf("errors.Is(err, ErrForbidden) = false; err = %v", err)
	}
}

func TestEcrDeleteTags_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "repositoryName": 12, "tags": []string{"latest"}})
	out, err := find(t, "registries.ecr_delete_tags")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4/ecr/repositories/12/tags" || gotMethod != http.MethodDelete {
		t.Errorf("called %s %s, want DELETE /api/registries/4/ecr/repositories/12/tags", gotMethod, gotPath)
	}
	tags, _ := gotBody["Tags"].([]any)
	if len(tags) != 1 || tags[0] != "latest" {
		t.Errorf("request body Tags = %v, want [latest]", gotBody["Tags"])
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestEcrDeleteTags_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	called := false
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })

	input, _ := json.Marshal(map[string]any{"id": 0, "repositoryName": 12})
	_, err := find(t, "registries.ecr_delete_tags")(context.Background(), c, input)
	if err == nil {
		t.Fatal("handler error = nil, want an error for a non-positive id")
	}
	if called {
		t.Error("the API was called despite an invalid id")
	}
}

func TestRepositoryTagsDelete_Success_CallsCorrectPath(t *testing.T) {
	t.Parallel()
	var gotPath, gotMethod string
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "repositoryName": "my-repo", "tags": []string{"v1"}})
	out, err := find(t, "registries.repository_tags_delete")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	if gotPath != "/api/registries/4/repositories/my-repo/tags" || gotMethod != http.MethodDelete {
		t.Errorf("called %s %s, want DELETE /api/registries/4/repositories/my-repo/tags", gotMethod, gotPath)
	}
	tags, _ := gotBody["tags"].([]any)
	if len(tags) != 1 || tags[0] != "v1" {
		t.Errorf("request body tags = %v, want [v1]", gotBody["tags"])
	}
	if out == nil {
		t.Error("handler returned nothing on success")
	}
}

func TestRepositoryTagsDelete_MissingFields_ReturnErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"invalid id", map[string]any{"id": 0, "repositoryName": "my-repo"}},
		{"missing repositoryName", map[string]any{"id": 4}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			called := false
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called = true })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.repository_tags_delete")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

// A registry response carrying a password must never reach the model. No
// fixture carried one before, which is why this went unnoticed.
func TestRegistryInspect_ResponseWithPassword_IsRedacted(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":1,"Name":"private","URL":"registry.example.com","Password":"hunter2"}`))
	})

	out, err := find(t, "registries.inspect")(context.Background(), c, json.RawMessage(`{"id":1}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "hunter2") {
		t.Errorf("the handler returned a password to the caller: %s", encoded)
	}
	if !strings.Contains(string(encoded), "private") {
		t.Error("redaction removed more than the credential")
	}
}

func TestRegistryCreate_ResponseWithPassword_IsRedacted(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":9,"Name":"my-registry","Password":"hunter2"}`))
	})

	input, _ := json.Marshal(map[string]any{"name": "my-registry", "url": "registry.example.com", "type": 3})
	out, err := find(t, "registries.create")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "hunter2") {
		t.Errorf("the handler returned a password to the caller: %s", encoded)
	}
	if !strings.Contains(string(encoded), "my-registry") {
		t.Error("redaction removed more than the credential")
	}
}

func TestRegistryUpdate_ResponseWithPassword_IsRedacted(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":4,"Name":"quay","Password":"hunter2"}`))
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "name": "quay", "url": "quay.io"})
	out, err := find(t, "registries.update")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "hunter2") {
		t.Errorf("the handler returned a password to the caller: %s", encoded)
	}
	if !strings.Contains(string(encoded), "quay") {
		t.Error("redaction removed more than the credential")
	}
}

// list returns an array, so this is the mutation-row-2 discriminator: if
// redactList's per-element loop were skipped or the wrong call site dropped,
// this is the one test among the four that catches it.
func TestRegistryList_ResponseWithPassword_IsRedacted(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"Id":1,"Name":"private","Password":"hunter2"},{"Id":2,"Name":"public"}]`))
	})

	out, err := find(t, "registries.list")(context.Background(), c, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(encoded), "hunter2") {
		t.Errorf("the handler returned a password to the caller: %s", encoded)
	}
	if !strings.Contains(string(encoded), "private") || !strings.Contains(string(encoded), "public") {
		t.Error("redaction removed more than the credential")
	}
}

// The nested ManagementConfiguration carries its own Password and AccessToken
// fields, independent of the top-level ones. A fixture exercising only the
// top-level field would not catch a redact that forgot the nested struct.
func TestRegistryInspect_ResponseWithNestedManagementCredentials_IsRedacted(t *testing.T) {
	t.Parallel()
	c := clientFor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"Id": 1,
			"Name": "private",
			"ManagementConfiguration": {
				"Password": "nested-secret",
				"AccessToken": "nested-token",
				"Username": "keep-me",
				"TLSConfig": {
					"TLS": true,
					"TLSCACert": "/data/tls/ca.pem",
					"TLSCert": "/data/tls/cert.pem",
					"TLSKey": "/data/tls/key.pem"
				}
			}
		}`))
	})

	out, err := find(t, "registries.inspect")(context.Background(), c, json.RawMessage(`{"id":1}`))
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	encoded, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, secret := range []string{"nested-secret", "nested-token", "/data/tls/ca.pem", "/data/tls/cert.pem", "/data/tls/key.pem"} {
		if strings.Contains(string(encoded), secret) {
			t.Errorf("the handler returned a nested management credential/path to the caller (%q): %s", secret, encoded)
		}
	}
	if !strings.Contains(string(encoded), "keep-me") {
		t.Error("redaction removed more than the nested credentials")
	}
}

// ptr returns a pointer to v, for constructing fixtures against generated
// types whose optional fields are all pointers.
func ptr[T any](v T) *T { return &v }

// TestRedact_LeavesNoCredentialShapedFieldPopulated walks the redacted struct
// and fails on any field whose name looks like a credential. It is deliberately
// name-based and reflective rather than a fixed list: the generated type comes
// from Portainer's spec and will gain fields we did not anticipate, and the
// failure mode of missing one is a credential in a model's transcript.
//
// Allow list, with reasons — fields the walk flags by name but which are not
// credentials. If the walk below ever flags a field that is not a secret
// (e.g. a KeyID, a PublicKey, or a CertificateAuthority boolean), add its
// "Struct.Field" path here with a one-line reason rather than weakening the
// marker list.
var redactAllowList = map[string]string{
	// AccessTokenExpiry is a unix timestamp for how long the access token
	// remains valid, not the token itself. It matches the "token" marker only
	// by name, redact deliberately leaves it populated (it discloses nothing),
	// and it is populated in the fixture below specifically to prove this
	// allow list is load-bearing rather than decorative.
	"Registry.AccessTokenExpiry": "an expiry timestamp, not a credential",
}

func TestRedact_LeavesNoCredentialShapedFieldPopulated(t *testing.T) {
	t.Parallel()
	secret := "SENTINEL-SECRET"
	registry := &apigen.PortainereeRegistry{
		Name:     ptr("private"),
		Password: &secret,
		// AccessTokenExpiry is deliberately populated too: it matches the
		// "token" marker by name but is not a credential (see the allow list
		// above), and populating it here is what proves the allow list
		// actually suppresses a flagged-but-legitimate field rather than the
		// test simply never encountering one.
		AccessTokenExpiry: ptr(1893456000),
		// Populate every credential-shaped field the type currently has.
		// The reflective walk below is what catches ones added later.
	}
	// Set AccessToken and the nested configuration through the same sentinel.
	registry.AccessToken = &secret
	registry.ManagementConfiguration = &apigen.PortainerRegistryManagementConfiguration{
		Password:    &secret,
		AccessToken: &secret,
		TLSConfig: &apigen.PortainerTLSConfiguration{
			TLSCACert: &secret, TLSCert: &secret, TLSKey: &secret,
		},
	}

	encoded, err := json.Marshal(redact(registry))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), secret) {
		t.Errorf("a credential survived redaction: %s", encoded)
	}

	// And catch a field we have not thought of: any populated field whose name
	// suggests a secret must be gone after redaction.
	suspicious := []string{"password", "token", "secret", "key", "credential", "cert"}
	var walk func(v reflect.Value, path string)
	walk = func(v reflect.Value, path string) {
		for v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return
			}
			v = v.Elem()
		}
		if v.Kind() != reflect.Struct {
			return
		}
		for i := range v.NumField() {
			field := v.Type().Field(i)
			if !field.IsExported() {
				continue
			}
			name := strings.ToLower(field.Name)
			child := v.Field(i)
			fieldPath := path + "." + field.Name
			for _, marker := range suspicious {
				if strings.Contains(name, marker) && !child.IsZero() {
					if reason, allowed := redactAllowList[fieldPath]; allowed {
						t.Logf("%s is populated after redaction but allow-listed: %s", fieldPath, reason)
						continue
					}
					t.Errorf("%s is populated after redaction; if it is not a credential, add it to the allow list with a reason", fieldPath)
				}
			}
			walk(child, fieldPath)
		}
	}
	walk(reflect.ValueOf(redact(registry)), "Registry")
}
