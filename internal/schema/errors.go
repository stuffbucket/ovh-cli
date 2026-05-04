package schema

import "errors"

// Sentinel errors. Callers MUST distinguish via errors.Is — never by string
// match. PRD-05 §OpenCache contract.
var (
	// ErrNoCache means no apispace.json exists for the requested region.
	// Callers should run Refresh.
	ErrNoCache = errors.New("schema: no apispace.json for region; call Refresh first")

	// ErrCacheCorrupt means the cache files exist but failed validation
	// (bad JSON, version mismatch, schema-hash mismatch, region mismatch).
	ErrCacheCorrupt = errors.New("schema: apispace.json failed validation")

	// ErrCachePerms means the cache directory is owned by the current user
	// but its mode bits are looser than the value declared in PRD-04
	// §Canonical file-mode registry (0700 for the schema cache directory).
	ErrCachePerms = errors.New("schema: cache directory mode looser than registry value; refusing to read")
)
