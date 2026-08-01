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
	switch surface {
	case SurfaceDynamic, SurfaceMeta, SurfaceIndividual:
	default:
		return nil, fmt.Errorf("invalid TOOL_SURFACE %q: want dynamic, meta or individual", surface)
	}

	level, err := parseLevel(envOr("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	return &Config{
		URL:           strings.TrimRight(os.Getenv("PORTAINER_URL"), "/"),
		Token:         os.Getenv("PORTAINER_TOKEN"),
		SkipTLSVerify: envBool("PORTAINER_SKIP_TLS_VERIFY"),
		ToolSurface:   surface,
		ReadOnly:      envBool("PORTAINER_READ_ONLY"),
		SafeMode:      envBool("PORTAINER_SAFE_MODE"),
		LogLevel:      level,
	}, nil
}

// Validate reports whether the configuration is usable for stdio mode.
func (c *Config) Validate() error {
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

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string) bool {
	v, err := strconv.ParseBool(os.Getenv(key))
	return err == nil && v
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
