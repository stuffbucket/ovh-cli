// Package xdgpaths is the only package that source-imports github.com/adrg/xdg.
//
// Both Layer A (internal/schema) and Layer B (internal/auth, internal/client,
// internal/config, ...) consume this package for path resolution. The strict
// Layer A allowlist in PRD-05 includes xdgpaths and excludes everything else
// in internal/, so this package must remain dependency-light: stdlib + xdg
// only. See PRD-04 §Path resolution and PRD-06 §Canonical package registry.
package xdgpaths

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

const appDir = "ovh"

// Config returns "$XDG_CONFIG_HOME/ovh".
func Config() string { return filepath.Join(xdg.ConfigHome, appDir) }

// Cache returns "$XDG_CACHE_HOME/ovh".
func Cache() string { return filepath.Join(xdg.CacheHome, appDir) }

// Data returns "$XDG_DATA_HOME/ovh".
func Data() string { return filepath.Join(xdg.DataHome, appDir) }

// State returns "$XDG_STATE_HOME/ovh".
func State() string { return filepath.Join(xdg.StateHome, appDir) }

// ConfigFile returns the canonical ovh.conf path inside Config().
func ConfigFile() string { return filepath.Join(Config(), "ovh.conf") }

// HostsFile returns the credential fallback path inside Config().
func HostsFile() string { return filepath.Join(Config(), "hosts.yml") }

// SchemaDir returns the per-region schema cache directory.
func SchemaDir(region string) string { return filepath.Join(Cache(), "schema", region) }

// CatalogDir returns the per-region catalog cache directory.
func CatalogDir(region string) string { return filepath.Join(Cache(), "catalog", region) }

// LogFile returns the rotating log file path.
func LogFile() string { return filepath.Join(State(), "logs", "ovh.log") }
