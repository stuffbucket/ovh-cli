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

	"github.com/google/renameio/v2"

	"github.com/stuffbucket/ovh-cli/internal/xdgpaths"
)

// Refresh re-fetches the API description for region and atomically rewrites
// apispace.json + apispace.meta.json.
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

	as, err := fetchAPISpace(ctx, region, endpoint, allowedHosts, transport)
	if err != nil {
		return err
	}
	hash, err := schemaHash(as.Services)
	if err != nil {
		return err
	}
	as.SchemaHash = hash

	body, err := json.MarshalIndent(as, "", "  ")
	if err != nil {
		return fmt.Errorf("schema: marshal apispace: %w", err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("schema: mkdir cache: %w", err)
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
