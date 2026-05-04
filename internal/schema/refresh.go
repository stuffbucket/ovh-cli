package schema

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/renameio/v2"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// Refresh re-fetches the API description for region and atomically rewrites
// apispace.json + apispace.meta.json. When the upstream returns 304 Not
// Modified (the on-disk ETag matches), apispace.json is left unchanged and
// only apispace.meta.json's FetchedAt is updated.
//
// Pre:  ctx != nil; region != ""; endpoint != ""; len(allowedHosts) > 0 —
//
//	empty allowlist or empty region/endpoint is a programming error and
//	triggers a runtime panic, NOT an error return. The caller (composition
//	root) resolves region+allowedHosts+endpoint from internal/auth in
//	phase 2b; phase 2a's cobra wrapper does the lookup itself.
//
// Post: nil error => OpenCache(region) returns a non-nil, non-stale Querier
//
//	whose FetchedAt is within this call's wall-clock window. Non-nil error
//	=> previously cached files are unchanged (renameio atomic-write contract).
//
// PRD-05 §Refresh contract.
func Refresh(ctx context.Context, region, endpoint string, allowedHosts []string) error {
	return refreshFrom(ctx, region, endpoint, allowedHosts, xdgpaths.SchemaDir(region), nil)
}

// refreshFrom is the test seam: callers pass an explicit target directory and
// optional http.RoundTripper. transport=nil uses http.DefaultTransport;
// tests pass an httptest.NewTLSServer's transport to trust the self-signed
// cert. Both Refresh and refreshFrom enforce the PRD-05 pre-conditions via
// panic.
func refreshFrom(ctx context.Context, region, endpoint string, allowedHosts []string, dir string, transport http.RoundTripper) error {
	if ctx == nil {
		panic("schema.Refresh: ctx must not be nil")
	}
	if region == "" {
		panic("schema.Refresh: region must not be empty")
	}
	if endpoint == "" {
		panic("schema.Refresh: endpoint must not be empty")
	}
	if len(allowedHosts) == 0 {
		panic("schema.Refresh: allowedHosts must be non-empty (PRD-05 threat model)")
	}

	prevETag := readPreviousETag(dir)
	result, err := fetchAPISpace(ctx, region, endpoint, allowedHosts, transport, prevETag)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("schema: mkdir cache: %w", err)
	}
	if !result.Modified {
		// 304 path: leave apispace.json untouched; only bump FetchedAt
		// (and ETag, if upstream sent one) in apispace.meta.json.
		return rewriteMetaFetchedAt(dir, result.ETag)
	}
	return writeFresh(dir, result.Space)
}

// writeFresh marshals as into apispace.json and writes the matching
// apispace.meta.json. Atomic per renameio.
func writeFresh(dir string, as *APISpace) error {
	hash, err := schemaHash(as.Services)
	if err != nil {
		return err
	}
	as.SchemaHash = hash

	body, err := json.MarshalIndent(as, "", "  ")
	if err != nil {
		return fmt.Errorf("schema: marshal apispace: %w", err)
	}
	// PRD-04 §Canonical file-mode registry: apispace.json mode 0644.
	if err := renameio.WriteFile(filepath.Join(dir, "apispace.json"), body, 0o644); err != nil {
		return fmt.Errorf("schema: write apispace.json: %w", err)
	}

	metaBytes, err := json.MarshalIndent(Meta{
		Version:    as.Version,
		Region:     as.Region,
		FetchedAt:  as.FetchedAt,
		ETag:       as.ETag,
		SchemaHash: as.SchemaHash,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("schema: marshal meta: %w", err)
	}
	if err := renameio.WriteFile(filepath.Join(dir, "apispace.meta.json"), metaBytes, 0o644); err != nil {
		return fmt.Errorf("schema: write apispace.meta.json: %w", err)
	}
	return nil
}

// readPreviousETag returns the ETag from apispace.meta.json or "" when no
// prior meta exists or it's unreadable. Errors are treated as "no etag" so
// the next call refetches from scratch.
func readPreviousETag(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "apispace.meta.json")) // #nosec G304 -- xdgpaths-derived
	if err != nil {
		return ""
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return ""
	}
	return m.ETag
}

// rewriteMetaFetchedAt updates apispace.meta.json's FetchedAt (and ETag, if
// non-empty) without touching apispace.json. Used on the 304 path.
func rewriteMetaFetchedAt(dir, etag string) error {
	metaPath := filepath.Join(dir, "apispace.meta.json")
	b, err := os.ReadFile(metaPath) // #nosec G304 -- xdgpaths-derived
	if err != nil {
		return fmt.Errorf("schema: read meta for refresh: %w", err)
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("schema: parse meta: %w", err)
	}
	m.FetchedAt = time.Now().UTC()
	if etag != "" {
		m.ETag = etag
	}
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("schema: marshal meta: %w", err)
	}
	return renameio.WriteFile(metaPath, out, 0o644)
}

// schemaHash hashes a stable JSON serialization of services. encoding/json
// sorts map keys alphabetically (Go 1.12+), so the output is deterministic
// across runs and across binaries.
func schemaHash(services map[string]Service) (string, error) {
	body, err := json.Marshal(services)
	if err != nil {
		return "", fmt.Errorf("schema: hash services: %w", err)
	}
	sum := sha256.Sum256(body)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}
