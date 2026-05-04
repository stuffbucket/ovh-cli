package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// CacheTTL is the freshness window beyond which Querier.Stale() reports true.
// PRD-04 key registry exposes this as default.cache_ttl_hours; phase 2b wires
// the config value through. Phase 2a hard-codes the default.
const CacheTTL = 24 * time.Hour

// OpenCache opens the cached apispace.json for the given region read-only.
//
// Pre:  region != "".
// Post: nil error => returned Querier is non-nil; Querier.Stale() may be true.
//
//	On any non-nil error (ErrNoCache | ErrCacheCorrupt | ErrCachePerms) the
//	returned Querier is nil; calling any method on it panics.
//
// PRD-05 §OpenCache contract.
func OpenCache(region string) (Querier, error) {
	return openCacheAt(xdgpaths.SchemaDir(region), region, time.Now)
}

// openCacheAt is the test seam: it accepts an explicit directory and clock so
// tests don't need to touch $XDG_CACHE_HOME or wall-clock time.
func openCacheAt(dir, region string, now func() time.Time) (Querier, error) {
	if region == "" {
		return nil, errors.New("schema: region must not be empty")
	}

	if err := assertSecureDir(dir); err != nil {
		return nil, err
	}

	meta, err := readMeta(filepath.Join(dir, "apispace.meta.json"))
	if err != nil {
		return nil, err
	}
	if meta.Version != SpecVersion || meta.Region != region {
		return nil, ErrCacheCorrupt
	}

	specBytes, err := os.ReadFile(filepath.Join(dir, "apispace.json")) // #nosec G304 -- dir is xdgpaths.SchemaDir(region); region validated above
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoCache
	}
	if err != nil {
		return nil, fmt.Errorf("schema: read apispace.json: %w", err)
	}

	return &cachedQuerier{meta: meta, spec: specBytes, now: now}, nil
}

func assertSecureDir(dir string) error {
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return ErrNoCache
	}
	if err != nil {
		return fmt.Errorf("schema: stat cache dir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("schema: cache path %q is not a directory", dir)
	}
	// PRD-04 §Canonical file-mode registry: schema/ is 0700, refuse-if-looser.
	if info.Mode().Perm()&0o077 != 0 {
		return ErrCachePerms
	}
	return nil
}

func readMeta(path string) (Meta, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path is constructed from xdgpaths.SchemaDir; project-controlled
	if errors.Is(err, fs.ErrNotExist) {
		return Meta{}, ErrNoCache
	}
	if err != nil {
		return Meta{}, fmt.Errorf("schema: read apispace.meta.json: %w", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return Meta{}, fmt.Errorf("%w: meta: %v", ErrCacheCorrupt, err)
	}
	return m, nil
}
