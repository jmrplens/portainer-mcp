package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jmrplens/portainer-mcp/internal/edition"
)

// specHarnessResult is what the generated harness program (see
// specHarnessTemplate) reports about the one action it found and called.
type specHarnessResult struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Edition     string   `json:"edition"`
	Mutating    bool     `json:"mutating"`
	Destructive bool     `json:"destructive"`
	Usage       string   `json:"usage"`
	Tags        []string `json:"tags"`
	Method      string   `json:"method"`
	Path        string   `json:"path"`
}

// specHarnessTemplate is a throwaway `package main` that calls
// generatedSpecs() (the real output renderGeneratedSpecs produces, compiled
// alongside the handler it references), finds the one entry named
// %[1]q, calls its Handler against a recording HTTP stub, and prints both
// the composed ActionSpec's own fields and the request the handler actually
// made — proving the emitter wired Name/Title/Description/Edition/Mutating/
// Destructive/Usage/Tags AND a real, callable Handler to the same
// toolutil.ActionSpec value, not two independently-plausible halves.
const specHarnessTemplate = `package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"

	"github.com/jmrplens/portainer-mcp/internal/config"
	"github.com/jmrplens/portainer-mcp/internal/portainer"
)

func main() {
	var method, path string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		method, path = r.Method, r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(%[3]q))
	}))
	defer server.Close()

	c, err := portainer.New(&config.Config{URL: server.URL, Token: "test-token"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "build client:", err)
		os.Exit(1)
	}

	var found *toolutilActionSpecAlias
	for _, s := range generatedSpecs() {
		if s.Name == %[1]q {
			cp := toolutilActionSpecAlias(s)
			found = &cp
		}
	}
	if found == nil {
		fmt.Fprintln(os.Stderr, "action not found:", %[1]q)
		os.Exit(1)
	}

	if _, err := found.Handler(context.Background(), c, json.RawMessage(%[2]q)); err != nil {
		fmt.Fprintln(os.Stderr, "handler error:", err)
		os.Exit(1)
	}

	out, _ := json.Marshal(map[string]any{
		"name": found.Name, "title": found.Title, "description": found.Description,
		"edition": string(found.Edition), "mutating": found.Mutating, "destructive": found.Destructive,
		"usage": found.Usage, "tags": found.Tags,
		"method": method, "path": path,
	})
	fmt.Println(string(out))
}
`

// executeGeneratedSpec renders inputs (nil when the operation needs none),
// actions.gen.go for spec, and — when narrativeSrc is non-empty — a hand
// file supplying a package-level narrative function (the same convention a
// real domain's own non-generated file follows). It then compiles and runs a
// harness that calls generatedSpecs(), finds entryName and calls its
// Handler, exactly the way tools.Execute does in the real binary.
func executeGeneratedSpec(t *testing.T, structs []structSpec, spec handlerSpec, entries []specEntry, hasNarrativeHook bool, narrativeSrc string, entryName, inputJSON, stubBody string) specHarnessResult {
	t.Helper()

	dir, err := os.MkdirTemp(".", ".spectest-")
	if err != nil {
		t.Fatalf("MkdirTemp() error = %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })

	if len(structs) > 0 {
		src, err := renderFile("main", "test-spec.json", structs)
		if err != nil {
			t.Fatalf("renderFile() error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "input.go"), src, 0o600); err != nil {
			t.Fatalf("write input.go: %v", err)
		}
	}

	actionsSrc, err := renderActionsFile("main", "test-spec.json", []handlerSpec{spec}, entries, hasNarrativeHook)
	if err != nil {
		t.Fatalf("renderActionsFile() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "actions.gen.go"), actionsSrc, 0o600); err != nil {
		t.Fatalf("write actions.gen.go: %v", err)
	}

	if narrativeSrc != "" {
		if err := os.WriteFile(filepath.Join(dir, "narrative.go"), []byte(narrativeSrc), 0o600); err != nil {
			t.Fatalf("write narrative.go: %v", err)
		}
	}

	// toolutilActionSpecAlias lets the harness copy a toolutil.ActionSpec by
	// value under a locally-declared name, sidestepping the need to import
	// toolutil under its own name a second time in the harness template
	// above (Go forbids redeclaring an already-dot-free import under a new
	// alias in the same file cleanly for this purpose) — a type alias is
	// simplest.
	aliasSrc := "package main\n\nimport \"github.com/jmrplens/portainer-mcp/internal/toolutil\"\n\ntype toolutilActionSpecAlias = toolutil.ActionSpec\n"
	if err := os.WriteFile(filepath.Join(dir, "alias.go"), []byte(aliasSrc), 0o600); err != nil {
		t.Fatalf("write alias.go: %v", err)
	}

	harness := fmt.Sprintf(specHarnessTemplate, entryName, inputJSON, stubBody)
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(harness), 0o600); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	cmd := exec.CommandContext(t.Context(), "go", "run", ".")
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go run generated spec failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}

	var result specHarnessResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &result); err != nil {
		t.Fatalf("harness did not print valid JSON: %v\nstdout:\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	return result
}

// TestUnit_RenderGeneratedSpecs_NoHook_ComposesNameHandlerAndInputFromTheSameOperation
// proves the bare (no narrative hook) rendering path: generatedSpecs()'s one
// entry carries the mechanical fields buildActionSpecFields computed, and
// its Handler is a real, callable function reaching the real path — not a
// nil or placeholder Handler that would only fail once actually invoked.
func TestUnit_RenderGeneratedSpecs_NoHook_ComposesNameHandlerAndInputFromTheSameOperation(t *testing.T) {
	t.Parallel()
	structs, spec := buildRealHandlerSpec(t, "tags", "TagDelete")
	fields, err := buildActionSpecFields("tags", operationFor(t, "TagDelete"), realCEOperationIDs(t))
	if err != nil {
		t.Fatalf("buildActionSpecFields() error = %v", err)
	}
	entries := []specEntry{{Fields: fields, HandlerFunc: spec.FuncName, InputStruct: spec.InputStruct}}

	got := executeGeneratedSpec(t, structs, spec, entries, false, "", "tags.delete", `{"id":9}`, "{}")
	if got.Name != "tags.delete" {
		t.Errorf("Name = %q, want tags.delete", got.Name)
	}
	if got.Method != http.MethodDelete || got.Path != "/api/tags/9" {
		t.Errorf("request = %s %s, want DELETE /api/tags/9: generatedSpecs' Handler field did not reach the real client call", got.Method, got.Path)
	}
	if got.Edition != string(edition.CE) {
		t.Errorf("Edition = %q, want CE", got.Edition)
	}
	if !got.Mutating || !got.Destructive {
		t.Errorf("Mutating=%v Destructive=%v, want both true for a DELETE", got.Mutating, got.Destructive)
	}
	if got.Usage != "" || len(got.Tags) != 0 {
		t.Errorf("Usage/Tags = %q/%v, want both empty: no narrative hook was declared", got.Usage, got.Tags)
	}
}

// TestUnit_RenderGeneratedSpecs_WithHook_NarrativeOverridesSurvive proves the
// wrapped (hasNarrativeHook) rendering path end to end: the generated
// literal is wrapped in toolutil.WithNarrative(literal, narrative(id)), and
// narrative's implementation — a plain hand-written function, standing in
// for a domain's own hand file — actually reaches the composed ActionSpec:
// its Title/Description override replaces the mechanical text, and its
// Usage/Tags (fields no specification could ever supply) come through only
// because this call happened, not because renderSpecLiteral hard-coded them.
func TestUnit_RenderGeneratedSpecs_WithHook_NarrativeOverridesSurvive(t *testing.T) {
	t.Parallel()
	structs, spec := buildRealHandlerSpec(t, "tags", "TagList")
	fields, err := buildActionSpecFields("tags", operationFor(t, "TagList"), realCEOperationIDs(t))
	if err != nil {
		t.Fatalf("buildActionSpecFields() error = %v", err)
	}
	mechanicalTitle := fields.Title
	entries := []specEntry{{Fields: fields, HandlerFunc: spec.FuncName, InputStruct: spec.InputStruct}}

	narrativeSrc := `package main

import "github.com/jmrplens/portainer-mcp/internal/toolutil"

func narrative(operationID string) toolutil.ActionNarrative {
	if operationID == "TagList" {
		return toolutil.ActionNarrative{
			Title:       "Human title",
			Description: "Human description.",
			Usage:       "Prefer this when you need every tag at once.",
			Tags:        []string{"lifecycle"},
		}
	}
	return toolutil.ActionNarrative{}
}
`

	got := executeGeneratedSpec(t, structs, spec, entries, true, narrativeSrc, "tags.list", `{}`, "[]")
	if got.Name != "tags.list" {
		t.Fatalf("Name = %q, want tags.list", got.Name)
	}
	if got.Method != http.MethodGet || got.Path != "/api/tags" {
		t.Errorf("request = %s %s, want GET /api/tags: the hook wrapper must not change which Handler is called", got.Method, got.Path)
	}
	if got.Title == mechanicalTitle {
		t.Errorf("Title = %q, want the hook's override %q rather than the mechanical spec text, proving the hook actually ran", got.Title, "Human title")
	}
	if got.Title != "Human title" || got.Description != "Human description." {
		t.Errorf("Title/Description = %q/%q, want the hook's exact override", got.Title, got.Description)
	}
	if got.Usage != "Prefer this when you need every tag at once." {
		t.Errorf("Usage = %q, want the hook's value: no mechanical derivation could ever produce this", got.Usage)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "lifecycle" {
		t.Errorf("Tags = %v, want [\"lifecycle\"] from the hook", got.Tags)
	}
}

// operationFor looks up operationID in the real vendored EE spec, the same
// way realOperation does, but returns only the operation (buildActionSpecFields
// does not need the document or resolver realOperation also returns).
func operationFor(t *testing.T, operationID string) operation {
	t.Helper()
	op, _, _ := realOperation(t, operationID)
	return op
}

// realCEOperationIDs loads the real vendored Community Edition specification
// and returns every operationId it declares, exactly what main.go's own run()
// passes to buildActionSpecFields — used instead of an empty map so these
// tests exercise editionOf against the real CE/EE split (tags is CE in both
// specs) rather than a fixture that would silently make everything look
// EE-only.
func realCEOperationIDs(t *testing.T) map[string]bool {
	t.Helper()
	_, cePaths, err := loadDocument("../../api/specs/ce-2.44.0.json")
	if err != nil {
		t.Fatalf("loadDocument(ce spec) error = %v", err)
	}
	ceByTag, err := operationsByDomain(cePaths)
	if err != nil {
		t.Fatalf("operationsByDomain(ce spec) error = %v", err)
	}
	return ceOperationIDSet(ceByTag)
}
