package config

import (
	"log/slog"
	"testing"
)

func TestLoad_NoEnvironment_AppliesDefaults(t *testing.T) {
	// Hermetic: clear every variable Load reads so the ambient shell (or a
	// later phase's new env var) cannot change the outcome. t.Setenv restores
	// the prior value automatically, and empty is treated as unset by both
	// envOr and envBool.
	for _, key := range []string{
		"PORTAINER_URL",
		"PORTAINER_TOKEN",
		"PORTAINER_SKIP_TLS_VERIFY",
		"TOOL_SURFACE",
		"PORTAINER_READ_ONLY",
		"PORTAINER_SAFE_MODE",
		"LOG_LEVEL",
		"PORTAINER_EDITION",
	} {
		t.Setenv(key, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ToolSurface != SurfaceDynamic {
		t.Errorf("ToolSurface = %q, want %q", cfg.ToolSurface, SurfaceDynamic)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelInfo)
	}
	if cfg.SkipTLSVerify || cfg.ReadOnly || cfg.SafeMode {
		t.Errorf("boolean defaults must all be false, got %+v", cfg)
	}
}

func TestLoad_EnvironmentSet_ReadsValues(t *testing.T) {
	t.Setenv("PORTAINER_URL", "https://portainer.example.com/")
	t.Setenv("PORTAINER_TOKEN", "ptr_abc")
	t.Setenv("PORTAINER_SKIP_TLS_VERIFY", "true")
	t.Setenv("TOOL_SURFACE", "meta")
	t.Setenv("PORTAINER_READ_ONLY", "true")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.URL != "https://portainer.example.com" {
		t.Errorf("URL = %q, want trailing slash trimmed", cfg.URL)
	}
	if cfg.Token != "ptr_abc" {
		t.Errorf("Token = %q, want %q", cfg.Token, "ptr_abc")
	}
	if !cfg.SkipTLSVerify || !cfg.ReadOnly {
		t.Errorf("booleans not parsed: %+v", cfg)
	}
	if cfg.ToolSurface != SurfaceMeta {
		t.Errorf("ToolSurface = %q, want %q", cfg.ToolSurface, SurfaceMeta)
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoad_InvalidSurface_ReturnsError(t *testing.T) {
	t.Setenv("TOOL_SURFACE", "nonsense")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for an invalid TOOL_SURFACE")
	}
}

func TestLoad_EditionOverride_IsRead(t *testing.T) {
	t.Setenv("PORTAINER_EDITION", "ee")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Edition != "EE" {
		t.Errorf("Edition = %q, want %q", cfg.Edition, "EE")
	}
}

func TestLoad_InvalidEdition_ReturnsError(t *testing.T) {
	t.Setenv("PORTAINER_EDITION", "business")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for an invalid PORTAINER_EDITION")
	}
}

func TestValidate_MissingURL_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &Config{Token: "ptr_abc", ToolSurface: SurfaceDynamic}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want an error for a missing URL")
	}
}

func TestValidate_MissingToken_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &Config{URL: "https://portainer.example.com", ToolSurface: SurfaceDynamic}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want an error for a missing token")
	}
}

func TestValidate_NonHTTPURL_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &Config{URL: "ftp://portainer.example.com", Token: "ptr_abc", ToolSurface: SurfaceDynamic}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want an error for a non-HTTP scheme")
	}
}

func TestValidate_Complete_ReturnsNil(t *testing.T) {
	t.Parallel()
	cfg := &Config{URL: "https://portainer.example.com", Token: "ptr_abc", ToolSurface: SurfaceDynamic}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() error = %v, want nil", err)
	}
}

func TestLoad_MalformedReadOnly_ReturnsError(t *testing.T) {
	t.Setenv("PORTAINER_READ_ONLY", "yes")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for a non-boolean PORTAINER_READ_ONLY")
	}
}

func TestLoad_MalformedSafeMode_ReturnsError(t *testing.T) {
	t.Setenv("PORTAINER_SAFE_MODE", "on")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for a non-boolean PORTAINER_SAFE_MODE")
	}
}

func TestLoad_MalformedSkipTLSVerify_ReturnsError(t *testing.T) {
	t.Setenv("PORTAINER_SKIP_TLS_VERIFY", "sure")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want an error for a non-boolean PORTAINER_SKIP_TLS_VERIFY")
	}
}

func TestValidate_InvalidToolSurface_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := &Config{URL: "https://portainer.example.com", Token: "ptr_abc", ToolSurface: "nonsense"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() error = nil, want an error for an invalid ToolSurface")
	}
}

func TestValidate_TrailingSlashURL_IsNormalised(t *testing.T) {
	t.Parallel()
	cfg := &Config{URL: "https://portainer.example.com/", Token: "ptr_abc", ToolSurface: SurfaceDynamic}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	if cfg.URL != "https://portainer.example.com" {
		t.Errorf("URL = %q, want the trailing slash trimmed", cfg.URL)
	}
}
