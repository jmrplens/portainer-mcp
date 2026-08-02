package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteJSON_Succeeds_WritesParseableFile(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "out.json")
	if err := writeJSON(path, map[string]any{"a": 1}); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
}

// The rename must leave no temporary files behind.
func TestWriteJSON_Succeeds_LeavesNoTempFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := writeJSON(filepath.Join(dir, "out.json"), map[string]any{"a": 1}); err != nil {
		t.Fatalf("writeJSON() error = %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "out.json" {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only out.json", names)
	}
}

func TestWriteJSON_UnwritableDirectory_ReturnsError(t *testing.T) {
	t.Parallel()
	if err := writeJSON(filepath.Join(t.TempDir(), "no-such-dir", "out.json"), map[string]any{}); err == nil {
		t.Error("writeJSON() error = nil, want an error for a missing directory")
	}
}

// TestWriteJSON_RenameFails_LeavesExistingFileIntact is the test that actually
// discriminates the atomic implementation from a direct os.WriteFile: it can
// only be written because the write is staged before it is published.
func TestWriteJSON_RenameFails_LeavesExistingFileIntact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")

	const original = `{"original":true}`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("seed the target: %v", err)
	}

	restore := renameFile
	t.Cleanup(func() { renameFile = restore })
	renameFile = func(_, _ string) error { return errors.New("forced rename failure") }

	if err := writeJSON(path, map[string]any{"replacement": true}); err == nil {
		t.Fatal("writeJSON() error = nil, want the forced rename failure to surface")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back the target: %v", err)
	}
	if string(got) != original {
		t.Errorf("target = %q, want the original content untouched by a failed write", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("directory contains %v, want only out.json — a temporary file leaked", names)
	}
}
