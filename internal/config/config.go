// Package config loads, normalises and validates runtime configuration.
//
// Values are resolved from three sources, each overriding the previous: a .env
// file in the working directory, process environment variables, and CLI flags
// applied by the caller in cmd/portainer-mcp. Load covers the first two; flags
// are the caller's responsibility so that flag defaults never mask environment
// values.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// ToolSurface selects which projection of the action catalog is registered.
type ToolSurface string

// Supported tool surfaces. Dynamic is the default: it registers two tools and
// keeps the model-facing token cost lowest.
const (
	SurfaceDynamic    ToolSurface = "dynamic"
	SurfaceMeta       ToolSurface = "meta"
	SurfaceIndividual ToolSurface = "individual"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	URL           string
	Token         string
	SkipTLSVerify bool
	ToolSurface   ToolSurface
	ReadOnly      bool
	SafeMode      bool
	LogLevel      slog.Level
}

// Load reads a .env file when present, then the process environment, and
// returns the resolved configuration. Absent values fall back to defaults.
// Load does not call Validate, because HTTP mode resolves the URL and token
// per request rather than at startup.
func Load() (*Config, error) {
	// godotenv.Load never overwrites variables already set in the environment,
	// which is what gives the environment precedence over the .env file.
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load .env: %w", err)
	}

	surface := ToolSurface(strings.ToLower(envOr("TOOL_SURFACE", string(SurfaceDynamic))))
	if err := validateToolSurface(surface); err != nil {
		return nil, fmt.Errorf("invalid TOOL_SURFACE: %w", err)
	}

	level, err := parseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	skipTLSVerify, err := envBool("PORTAINER_SKIP_TLS_VERIFY")
	if err != nil {
		return nil, err
	}
	readOnly, err := envBool("PORTAINER_READ_ONLY")
	if err != nil {
		return nil, err
	}
	safeMode, err := envBool("PORTAINER_SAFE_MODE")
	if err != nil {
		return nil, err
	}

	return &Config{
		URL:           strings.TrimRight(os.Getenv("PORTAINER_URL"), "/"),
		Token:         os.Getenv("PORTAINER_TOKEN"),
		SkipTLSVerify: skipTLSVerify,
		ToolSurface:   surface,
		ReadOnly:      readOnly,
		SafeMode:      safeMode,
		LogLevel:      level,
	}, nil
}

// Validate reports whether the configuration is usable for stdio mode.
//
// It re-validates fields that Load already checks, and normalises c.URL by
// trimming a trailing slash. This is what makes Validate a single gate for
// both the environment path (Load) and the CLI flag path in cmd/portainer-mcp,
// where a flag can override a field Load already validated with a value that
// was never checked or normalised.
func (c *Config) Validate() error {
	c.URL = strings.TrimRight(c.URL, "/")

	if err := validateToolSurface(c.ToolSurface); err != nil {
		return fmt.Errorf("invalid tool surface: %w", err)
	}

	if c.URL == "" {
		return errors.New("PORTAINER_URL is required")
	}
	parsed, err := url.Parse(c.URL)
	if err != nil {
		return fmt.Errorf("parse PORTAINER_URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("PORTAINER_URL scheme %q: want http or https", parsed.Scheme)
	}
	if parsed.Host == "" {
		return errors.New("PORTAINER_URL has no host")
	}
	if c.Token == "" {
		return errors.New("PORTAINER_TOKEN is required")
	}
	return nil
}

// validateToolSurface reports whether s is one of the supported tool
// surfaces. It is the single source of truth for that check, used by both
// Load (environment path) and Validate (CLI flag path).
func validateToolSurface(s ToolSurface) error {
	switch s {
	case SurfaceDynamic, SurfaceMeta, SurfaceIndividual:
		return nil
	default:
		return fmt.Errorf("%q: want dynamic, meta or individual", s)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool parses a boolean environment variable. An unset variable is false;
// a variable set to something ParseBool rejects is an error, never a silent
// false — read-only and safe mode are security controls, and a typo such as
// PORTAINER_READ_ONLY=yes must not disable one without saying so.
func envBool(key string) (bool, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q: want a boolean such as true or false", key, raw)
	}
	return v, nil
}

func parseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid LOG_LEVEL %q: want debug, info, warn or error", s)
	}
}
