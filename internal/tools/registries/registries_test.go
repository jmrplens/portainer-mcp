package registries

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
