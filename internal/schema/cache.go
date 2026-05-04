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
// Pre:  region != ""; an empty region is a programming error and panics.
//
// Post: nil error => returned Querier is non-nil with services parsed and
//
//	ready to query. Querier.Stale() may be true. On any non-nil error
//	(ErrNoCache | ErrCacheCorrupt | ErrCachePerms) the returned Querier
//	is nil; calling any method on it panics.
//
// PRD-05 §OpenCache contract.
func OpenCache(region string) (Querier, error) {
	return openCacheAt(xdgpaths.SchemaDir(region), region, time.Now)
}

// openCacheAt is the test seam: explicit dir + clock injected for tests.
func openCacheAt(dir, region string, now func() time.Time) (Querier, error) {
	if region == "" {
		panic("schema.OpenCache: region must not be empty")
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
	services, err := readServices(filepath.Join(dir, "apispace.json"))
	if err != nil {
		return nil, err
	}
	return &cachedQuerier{meta: meta, services: services, now: now}, nil
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
	b, err := os.ReadFile(path) // #nosec G304 -- path is xdgpaths.SchemaDir; project-controlled
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

func readServices(path string) (map[string]Service, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- xdgpaths-derived
	if errors.Is(err, fs.ErrNotExist) {
		return nil, ErrNoCache
	}
	if err != nil {
		return nil, fmt.Errorf("schema: read apispace.json: %w", err)
	}
	var as APISpace
	if err := json.Unmarshal(b, &as); err != nil {
		return nil, fmt.Errorf("%w: spec: %v", ErrCacheCorrupt, err)
	}
	if as.Services == nil {
		return map[string]Service{}, nil
	}
	return as.Services, nil
}
