package registries

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
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
			var called atomic.Bool
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.create")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called.Load() {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

// TestRegistryCreate_ExplicitTLSFalse_ReachesRequestBody guards against the
// bool-vs-pointer defect: when TLS was a plain bool, the handler only
// forwarded it when true, so an explicit {"tls":false} was indistinguishable
// from omitting the field entirely. With *bool, an explicit false must be
// forwarded rather than silently dropped.
func TestRegistryCreate_ExplicitTLSFalse_ReachesRequestBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":9,"Name":"my-registry"}`))
	})

	input, _ := json.Marshal(map[string]any{
		"name": "my-registry", "url": "registry.example.com", "type": 3, "tls": false,
	})
	_, err := find(t, "registries.create")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	tls, present := gotBody["TLS"]
	if !present {
		t.Fatal("request body has no TLS field; an explicit false was dropped as if the field had been omitted")
	}
	if tls != false {
		t.Errorf("request body TLS = %v, want false", tls)
	}
}

// TestRegistryCreate_ForwardsNestedProviderObjects is the task-4 regression
// guard: Ecr, Github, Gitlab and Quay are declared on the generated input
// schema but were silently dropped by the handler. The body is decoded into
// the actual generated wire type (not a map, and not string-matched) so the
// assertion discriminates a converter that wires a field to the wrong slot,
// or one that never runs at all, from one that genuinely forwards it —
// each nested object carries a value that appears nowhere else in the
// fixture or the request.
func TestRegistryCreate_ForwardsNestedProviderObjects(t *testing.T) {
	t.Parallel()
	var body apigen.RegistriesRegistryCreatePayload
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":9,"Name":"my-registry"}`))
	})

	input, _ := json.Marshal(map[string]any{
		"name": "my-registry", "url": "registry.example.com", "type": 7,
		"ecr":    map[string]any{"region": "eu-west-1"},
		"github": map[string]any{"organisationName": "acme-gh", "useOrganisation": true},
		"gitlab": map[string]any{"instanceUrl": "https://gitlab.example.com", "projectId": 42, "projectPath": "group/project"},
		"quay":   map[string]any{"organisationName": "acme-quay", "useOrganisation": false},
	})
	_, err := find(t, "registries.create")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if body.Ecr == nil || body.Ecr.Region == nil || *body.Ecr.Region != "eu-west-1" {
		t.Errorf("Ecr = %+v, want Region=eu-west-1", body.Ecr)
	}
	if body.Github == nil || body.Github.OrganisationName == nil || *body.Github.OrganisationName != "acme-gh" ||
		body.Github.UseOrganisation == nil || !*body.Github.UseOrganisation {
		t.Errorf("Github = %+v, want OrganisationName=acme-gh UseOrganisation=true", body.Github)
	}
	if body.Gitlab == nil || body.Gitlab.InstanceURL == nil || *body.Gitlab.InstanceURL != "https://gitlab.example.com" ||
		body.Gitlab.ProjectId == nil || *body.Gitlab.ProjectId != 42 ||
		body.Gitlab.ProjectPath == nil || *body.Gitlab.ProjectPath != "group/project" {
		t.Errorf("Gitlab = %+v, want InstanceURL/ProjectId/ProjectPath forwarded", body.Gitlab)
	}
	if body.Quay == nil || body.Quay.OrganisationName == nil || *body.Quay.OrganisationName != "acme-quay" ||
		body.Quay.UseOrganisation == nil || *body.Quay.UseOrganisation {
		t.Errorf("Quay = %+v, want OrganisationName=acme-quay UseOrganisation=false", body.Quay)
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
	var called atomic.Bool
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

	_, err := find(t, "registries.ping")(context.Background(), c, json.RawMessage(`{"type":3}`))
	if err == nil {
		t.Fatal("handler error = nil, want an error for a missing url")
	}
	if called.Load() {
		t.Error("the API was called despite missing required input")
	}
}

// TestRegistryPing_ExplicitTLSFalse_ReachesRequestBody is the ping-action
// counterpart of the registries.create guard above.
func TestRegistryPing_ExplicitTLSFalse_ReachesRequestBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true}`))
	})

	input, _ := json.Marshal(map[string]any{"url": "registry.example.com", "type": 3, "tls": false})
	_, err := find(t, "registries.ping")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	tls, present := gotBody["TLS"]
	if !present {
		t.Fatal("request body has no TLS field; an explicit false was dropped as if the field had been omitted")
	}
	if tls != false {
		t.Errorf("request body TLS = %v, want false", tls)
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
		var called atomic.Bool
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

		input, _ := json.Marshal(map[string]any{"id": id})
		_, err := find(t, "registries.inspect")(context.Background(), c, input)
		if err == nil {
			t.Errorf("id=%d: handler error = nil, want an error for a non-positive id", id)
		}
		if called.Load() {
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
			var called atomic.Bool
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.update")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called.Load() {
				t.Error("the API was called despite missing required input")
			}
		})
	}
}

// TestRegistryUpdate_ForwardsBaseURLNestedProviderObjectsAndRegistryAccesses
// is the task-4 regression guard for registries.update: BaseURL, Ecr,
// Github, Quay and RegistryAccesses are declared on the generated input
// schema but were silently dropped. RegistryAccesses is asserted three
// levels deep (namespace, team policy, user policy) because forwarding only
// the top level while dropping the nested policies would be the same defect
// one layer down.
func TestRegistryUpdate_ForwardsBaseURLNestedProviderObjectsAndRegistryAccesses(t *testing.T) {
	t.Parallel()
	var body apigen.RegistriesRegistryUpdatePayload
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":4,"Name":"quay"}`))
	})

	input, _ := json.Marshal(map[string]any{
		"id": 4, "name": "quay", "url": "quay.io",
		"baseUrl": "registry.mydomain.tld:2375",
		"ecr":     map[string]any{"region": "us-east-1"},
		"github":  map[string]any{"organisationName": "acme-gh", "useOrganisation": true},
		"quay":    map[string]any{"organisationName": "acme-quay", "useOrganisation": false},
		"registryAccesses": map[string]any{
			"3": map[string]any{
				"namespaces": []string{"prod"},
				"teamAccessPolicies": map[string]any{
					"7": map[string]any{"roleId": 2, "namespaces": []string{"prod"}},
				},
				"userAccessPolicies": map[string]any{
					"11": map[string]any{"roleId": 1},
				},
			},
		},
	})
	_, err := find(t, "registries.update")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if body.BaseURL == nil || *body.BaseURL != "registry.mydomain.tld:2375" {
		t.Errorf("BaseURL = %v, want registry.mydomain.tld:2375", body.BaseURL)
	}
	if body.Ecr == nil || body.Ecr.Region == nil || *body.Ecr.Region != "us-east-1" {
		t.Errorf("Ecr = %+v, want Region=us-east-1", body.Ecr)
	}
	if body.Github == nil || body.Github.OrganisationName == nil || *body.Github.OrganisationName != "acme-gh" {
		t.Errorf("Github = %+v, want OrganisationName=acme-gh", body.Github)
	}
	if body.Quay == nil || body.Quay.OrganisationName == nil || *body.Quay.OrganisationName != "acme-quay" {
		t.Errorf("Quay = %+v, want OrganisationName=acme-quay", body.Quay)
	}

	if body.RegistryAccesses == nil {
		t.Fatal("RegistryAccesses = nil, want forwarded")
	}
	access, ok := (*body.RegistryAccesses)["3"]
	if !ok {
		t.Fatal(`RegistryAccesses["3"] missing`)
	}
	if access.Namespaces == nil || len(*access.Namespaces) != 1 || (*access.Namespaces)[0] != "prod" {
		t.Errorf("RegistryAccesses[3].Namespaces = %v, want [prod]", access.Namespaces)
	}
	if access.TeamAccessPolicies == nil {
		t.Fatal("RegistryAccesses[3].TeamAccessPolicies = nil, want forwarded")
	}
	if team, ok := (*access.TeamAccessPolicies)["7"]; !ok || team.RoleId != 2 {
		t.Errorf("TeamAccessPolicies[7] = %+v (present=%v), want RoleId=2", team, ok)
	}
	if access.UserAccessPolicies == nil {
		t.Fatal("RegistryAccesses[3].UserAccessPolicies = nil, want forwarded")
	}
	if user, ok := (*access.UserAccessPolicies)["11"]; !ok || user.RoleId != 1 {
		t.Errorf("UserAccessPolicies[11] = %+v (present=%v), want RoleId=1", user, ok)
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

// TestRegistryConfigure_ExplicitTLSFalse_ReachesRequestBody is the configure-
// action counterpart of the create and ping guards above, and additionally
// covers TLSSkipVerify, which had the identical bool-vs-pointer defect.
func TestRegistryConfigure_ExplicitTLSFalse_ReachesRequestBody(t *testing.T) {
	t.Parallel()
	var gotBody map[string]any
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusNoContent)
	})

	input, _ := json.Marshal(map[string]any{"id": 4, "tls": false, "tlsSkipVerify": false})
	_, err := find(t, "registries.configure")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}
	tls, present := gotBody["TLS"]
	if !present || tls != false {
		t.Errorf("request body TLS = %v (present=%v), want false present", tls, present)
	}
	skip, present := gotBody["TLSSkipVerify"]
	if !present || skip != false {
		t.Errorf("request body TLSSkipVerify = %v (present=%v), want false present", skip, present)
	}
}

// TestRegistryConfigure_ForwardsTLSCertificateFileFields is the task-4
// regression guard for registries.configure: TLSCACertFile, TLSCertFile and
// TLSKeyFile are declared on the generated input schema (as []int, per the
// spec's int32-array items) but were silently dropped. The body is decoded
// into the wire type's []int32 fields so the assertion catches both a
// dropped field and a converter that mixes the three slices up.
func TestRegistryConfigure_ForwardsTLSCertificateFileFields(t *testing.T) {
	t.Parallel()
	var body apigen.RegistriesRegistryConfigurePayload
	c := clientFor(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	})

	input, _ := json.Marshal(map[string]any{
		"id": 4, "authentication": false,
		"tlsCACertFile": []int{1, 2, 3},
		"tlsCertFile":   []int{4, 5},
		"tlsKeyFile":    []int{6},
	})
	_, err := find(t, "registries.configure")(context.Background(), c, input)
	if err != nil {
		t.Fatalf("handler error = %v", err)
	}

	if body.TLSCACertFile == nil || !reflect.DeepEqual(*body.TLSCACertFile, []int32{1, 2, 3}) {
		t.Errorf("TLSCACertFile = %v, want [1 2 3]", body.TLSCACertFile)
	}
	if body.TLSCertFile == nil || !reflect.DeepEqual(*body.TLSCertFile, []int32{4, 5}) {
		t.Errorf("TLSCertFile = %v, want [4 5]", body.TLSCertFile)
	}
	if body.TLSKeyFile == nil || !reflect.DeepEqual(*body.TLSKeyFile, []int32{6}) {
		t.Errorf("TLSKeyFile = %v, want [6]", body.TLSKeyFile)
	}
}

// TestRegistryConfigure_TLSCertificateFileOverflowsInt32_ReturnsErrorWithoutCallingAPI
// guards the conversion's bounds check: a byte value that does not fit in
// int32 must fail before the request is sent, not be silently truncated.
func TestRegistryConfigure_TLSCertificateFileOverflowsInt32_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

	input, _ := json.Marshal(map[string]any{"id": 4, "tlsCACertFile": []int64{1<<31 + 1}})
	_, err := find(t, "registries.configure")(context.Background(), c, input)
	if err == nil {
		t.Fatal("handler error = nil, want an error for a TLS file byte that overflows int32")
	}
	if called.Load() {
		t.Error("the API was called despite an out-of-range TLS file byte")
	}
}

func TestRegistryConfigure_InvalidID_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	var called atomic.Bool
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

	_, err := find(t, "registries.configure")(context.Background(), c, json.RawMessage(`{"id":0}`))
	if err == nil {
		t.Fatal("handler error = nil, want an error for a non-positive id")
	}
	if called.Load() {
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
		var called atomic.Bool
		c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

		input, _ := json.Marshal(map[string]any{"id": id})
		_, err := find(t, "registries.delete")(context.Background(), c, input)
		if err == nil {
			t.Errorf("id=%d: handler error = nil, want an error for a non-positive id", id)
		}
		if called.Load() {
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
			var called atomic.Bool
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.ecr_delete_repository")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called.Load() {
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
	var called atomic.Bool
	c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })

	input, _ := json.Marshal(map[string]any{"id": 0, "repositoryName": 12})
	_, err := find(t, "registries.ecr_delete_tags")(context.Background(), c, input)
	if err == nil {
		t.Fatal("handler error = nil, want an error for a non-positive id")
	}
	if called.Load() {
		t.Error("the API was called despite an invalid id")
	}
}

// TestEcrDeleteTags_EmptyTags_ReturnsErrorWithoutCallingAPI is the regression
// guard: the action's description promises to delete "the given image tags",
// so an empty or omitted tags list has no defined meaning and must be
// refused before the registry ever sees the request, rather than sending a
// body with no Tags field and letting the registry decide what a tagless
// delete means.
func TestEcrDeleteTags_EmptyTags_ReturnsErrorWithoutCallingAPI(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		input map[string]any
	}{
		{"omitted tags", map[string]any{"id": 4, "repositoryName": 12}},
		{"empty tags", map[string]any{"id": 4, "repositoryName": 12, "tags": []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var called atomic.Bool
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.ecr_delete_tags")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for an empty tags list")
			}
			if called.Load() {
				t.Error("the API was called despite an empty tags list")
			}
		})
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
		{"omitted tags", map[string]any{"id": 4, "repositoryName": "my-repo"}},
		{"empty tags", map[string]any{"id": 4, "repositoryName": "my-repo", "tags": []string{}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var called atomic.Bool
			c := clientFor(t, func(http.ResponseWriter, *http.Request) { called.Store(true) })
			input, _ := json.Marshal(tt.input)
			_, err := find(t, "registries.repository_tags_delete")(context.Background(), c, input)
			if err == nil {
				t.Fatal("handler error = nil, want an error for missing required input")
			}
			if called.Load() {
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

// suspiciousFieldMarkers are substrings that make an exported field name look
// like it might carry a credential. Deliberately narrow: "auth" would also
// match AuthorizedTeams/AuthorizedUsers/Authentication and produce noise, so
// it and similarly broad candidates (bearer, signature, pat) are left out
// pending an explicit ruling rather than added unilaterally.
var suspiciousFieldMarkers = []string{"password", "token", "secret", "key", "credential", "cert"}

// walkForCredentialShapedFields walks v and calls report with the path of
// every populated field whose name matches a marker in
// suspiciousFieldMarkers, skipping any path present in skip.
//
// It descends through pointers, interfaces, maps, slices and arrays as well
// as structs — not structs alone. A first version of this walk stopped at
// pointer/struct, which silently missed a credential-named field reached
// through any of the other four kinds. PortainereeRegistry.RegistryAccesses
// is exactly that shape (map[string]PortainerRegistryAccessPolicies), so the
// gap was not hypothetical: it was already present in the type this walk
// exists to guard.
func walkForCredentialShapedFields(v reflect.Value, path string, skip map[string]string, report func(path string)) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return
		}
		walkForCredentialShapedFields(v.Elem(), path, skip, report)
		return
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		walkForCredentialShapedFields(v.Elem(), path, skip, report)
		return
	case reflect.Map:
		// RegistryAccesses is map-shaped, and a future credential field is as
		// likely to appear inside a map value as anywhere else. A walk that
		// stops at the map boundary would report coverage it does not have.
		for _, key := range v.MapKeys() {
			walkForCredentialShapedFields(v.MapIndex(key), fmt.Sprintf("%s[%v]", path, key.Interface()), skip, report)
		}
		return
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			walkForCredentialShapedFields(v.Index(i), fmt.Sprintf("%s[%d]", path, i), skip, report)
		}
		return
	case reflect.Struct:
	default:
		return
	}

	for i := range v.NumField() {
		field := v.Type().Field(i)
		if !field.IsExported() {
			continue
		}
		child := v.Field(i)
		name := strings.ToLower(field.Name)
		fieldPath := path + "." + field.Name
		if _, allowed := skip[fieldPath]; !allowed {
			for _, marker := range suspiciousFieldMarkers {
				if strings.Contains(name, marker) && !child.IsZero() {
					report(fieldPath)
					break
				}
			}
		}
		walkForCredentialShapedFields(child, fieldPath, skip, report)
	}
}

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
		// RegistryAccesses is map-shaped (map[string]PortainerRegistryAccessPolicies).
		// Populated here so the map branch of the walk is actually exercised
		// against the real generated type, not merely present in the walk's
		// code. Namespaces carries no credential-shaped name, so this
		// populates the map path without expecting it to be flagged — the
		// synthetic test below proves the walk detects a credential reached
		// this way, since no real field here can.
		RegistryAccesses: &apigen.PortainerRegistryAccesses{
			"1": apigen.PortainerRegistryAccessPolicies{Namespaces: &[]string{"production"}},
		},
		// Ecr/Github/Gitlab/Quay are populated too: task 4 widened what
		// registries.create/update forward into these provider-specific
		// objects, and PortainereeRegistry carries the identically-shaped
		// objects back on read. None of their fields are credential-shaped
		// today (checked by hand against PortainerEcrData,
		// PortainerGithubRegistryData, PortainerGitlabRegistryData and
		// PortainerQuayRegistryData), but leaving them nil here would mean
		// the walk below never descends into them at all — a nil pointer is
		// skipped, not "checked and found clean". Populating them proves the
		// walk actually reaches these paths, so a credential-shaped field
		// added to any of them later is caught here rather than by a
		// fixture no one thought to update.
		Ecr:    &apigen.PortainerEcrData{Region: ptr("eu-west-1")},
		Github: &apigen.PortainerGithubRegistryData{OrganisationName: ptr("acme-gh")},
		Gitlab: &apigen.PortainerGitlabRegistryData{InstanceURL: ptr("https://gitlab.example.com")},
		Quay:   &apigen.PortainerQuayRegistryData{OrganisationName: ptr("acme-quay")},
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
	walkForCredentialShapedFields(reflect.ValueOf(redact(registry)), "Registry", nil, func(path string) {
		if reason, allowed := redactAllowList[path]; allowed {
			t.Logf("%s is populated after redaction but allow-listed: %s", path, reason)
			return
		}
		t.Errorf("%s is populated after redaction; if it is not a credential, add it to the allow list with a reason", path)
	})
}

// TestWalkForCredentialShapedFields_DescendsThroughMapsSlicesAndInterfaces
// proves the walk actually descends into maps, slices and interfaces, not
// merely that it does not crash on them.
//
// PortainereeRegistry has no real credential-shaped field reachable through
// any of the three today — checked by hand against every field on
// PortainereeRegistry, PortainerRegistryManagementConfiguration, and every
// type either of those points to (PortainerRegistryAccessPolicies,
// PortainerTeamAccessPolicies, PortainerUserAccessPolicies, PortainerAccessPolicy):
// none of their fields match a marker. So this uses synthetic local types,
// purpose-built to carry one credential-shaped field behind each shape, run
// through the exact same walkForCredentialShapedFields used above.
func TestWalkForCredentialShapedFields_DescendsThroughMapsSlicesAndInterfaces(t *testing.T) {
	t.Parallel()

	type viaMap struct{ Password *string }
	type viaSlice struct{ Password *string }
	type viaInterface struct{ Password *string }

	secret := "SENTINEL-SECRET"
	fixture := struct {
		ViaMap       map[string]viaMap
		ViaSlice     []viaSlice
		ViaInterface any
	}{
		ViaMap:       map[string]viaMap{"k": {Password: &secret}},
		ViaSlice:     []viaSlice{{Password: &secret}},
		ViaInterface: viaInterface{Password: &secret},
	}

	var flagged []string
	walkForCredentialShapedFields(reflect.ValueOf(fixture), "Fixture", nil, func(path string) {
		flagged = append(flagged, path)
	})

	for _, want := range []string{
		"Fixture.ViaMap[k].Password",
		"Fixture.ViaSlice[0].Password",
		"Fixture.ViaInterface.Password",
	} {
		found := false
		for _, got := range flagged {
			if got == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("walk did not flag %s; flagged = %v", want, flagged)
		}
	}
}
