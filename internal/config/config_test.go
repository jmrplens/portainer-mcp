package config

import (
	"log/slog"
	"testing"
)

// clearPortainerEnv makes a test hermetic by clearing every variable Load
// reads, so the ambient shell — or a later phase's new variable — cannot
// change the outcome. t.Setenv restores prior values automatically, and empty
// is treated as unset by both envOr and envBool.
func clearPortainerEnv(t *testing.T) {
	t.Helper()
	for _, suffix := range []string{
		"URL",
		"TOKEN",
		"SKIP_TLS_VERIFY",
		"TOOL_SURFACE",
		"READ_ONLY",
		"SAFE_MODE",
		"LOG_LEVEL",
		"EDITION",
	} {
		t.Setenv(envPrefix+suffix, "")
	}
}

func TestLoad_NoEnvironment_AppliesDefaults(t *testing.T) {
	clearPortainerEnv(t)

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
	t.Setenv("PORTAINER_TOOL_SURFACE", "meta")
	t.Setenv("PORTAINER_READ_ONLY", "true")
	t.Setenv("PORTAINER_LOG_LEVEL", "debug")

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
	t.Setenv("PORTAINER_TOOL_SURFACE", "nonsense")
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

// TestLoad_UnprefixedNamesAreIgnored is the guard for the defect this prefix
// exists to prevent. TOOL_SURFACE and LOG_LEVEL were read without the
// PORTAINER_ prefix every other variable carries, so an unrelated process in
// the environment — LOG_LEVEL is a common name — could change our behaviour.
// Reading through env() makes that structurally impossible; this pins it.
func TestLoad_UnprefixedNamesAreIgnored(t *testing.T) {
	clearPortainerEnv(t)
	for _, key := range []string{
		"URL", "TOKEN", "SKIP_TLS_VERIFY", "TOOL_SURFACE",
		"READ_ONLY", "SAFE_MODE", "LOG_LEVEL", "EDITION",
	} {
		t.Setenv(key, "")
	}

	// Values that would be visibly wrong if any of them were honoured: a
	// non-default surface, debug logging, and both security controls off-by-
	// default flipped on.
	t.Setenv("TOOL_SURFACE", "individual")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("READ_ONLY", "true")
	t.Setenv("SAFE_MODE", "true")
	t.Setenv("URL", "https://unprefixed.example.com")
	t.Setenv("TOKEN", "unprefixed-token")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.ToolSurface != SurfaceDynamic {
		t.Errorf("ToolSurface = %q, want the default %q: an unprefixed TOOL_SURFACE was honoured",
			cfg.ToolSurface, SurfaceDynamic)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want the default info: an unprefixed LOG_LEVEL was honoured", cfg.LogLevel)
	}
	if cfg.ReadOnly || cfg.SafeMode {
		t.Errorf("ReadOnly = %v, SafeMode = %v, want both false: an unprefixed variable was honoured",
			cfg.ReadOnly, cfg.SafeMode)
	}
	if cfg.URL != "" || cfg.Token != "" {
		t.Errorf("URL = %q, Token = %q, want both empty: an unprefixed variable was honoured", cfg.URL, cfg.Token)
	}
}
