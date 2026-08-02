package main

import (
	"encoding/json"
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
