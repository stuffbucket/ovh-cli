// Package schema is Layer A: it fetches OVH's API description, normalizes it,
// and serves a queryable cached JSON file (apispace.json) to the rest of the
// binary. See PRD-05 for the full spec, including the import allowlist that
// constrains this package to: stdlib, golang.org/x/sync/errgroup,
// github.com/tidwall/gjson, github.com/google/renameio/v2, anonymous
// github.com/ovh/go-ovh/ovh, and <module>/internal/xdgpaths.
package schema

import (
	"encoding/json"
	"time"
)

// SpecVersion is the current apispace.json schema version. A bump invalidates
// every cache produced by older binaries.
const SpecVersion = 1

// APISpace is the merged OVH API description for a single region.
// PRD-05 §apispace.json format.
type APISpace struct {
	Version    int                `json:"version"`
	Region     string             `json:"region"`
	FetchedAt  time.Time          `json:"fetched_at"`
	ETag       string             `json:"etag,omitempty"`
	SchemaHash string             `json:"schema_hash"`
	Services   map[string]Service `json:"services"`
}

// Service is one /1.0/<service> entry. Models is left as raw JSON so callers
// that don't need to walk it pay no decode cost.
type Service struct {
	Description string              `json:"description,omitempty"`
	Paths       map[string]PathSpec `json:"paths,omitempty"`
	Error       string              `json:"error,omitempty"`
	Models      json.RawMessage     `json:"models,omitempty"`
}

// PathSpec is the methods-by-name map for one path. Keys are "GET", "POST", etc.
type PathSpec map[string]MethodSpec

// MethodSpec describes one HTTP method on a path. The full OVH method object
// is broader; v0.1 keeps only what the runner and TUI explorer need.
type MethodSpec struct {
	Description string          `json:"description,omitempty"`
	Auth        []string        `json:"auth,omitempty"`
	Params      json.RawMessage `json:"params,omitempty"`
	Response    json.RawMessage `json:"response,omitempty"`
}

// Meta is apispace.meta.json — the small companion file consulted for ETag
// and TTL decisions before reading the (potentially large) apispace.json.
type Meta struct {
	Version    int       `json:"version"`
	Region     string    `json:"region"`
	FetchedAt  time.Time `json:"fetched_at"`
	ETag       string    `json:"etag,omitempty"`
	SchemaHash string    `json:"schema_hash"`
}
