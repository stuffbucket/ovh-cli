---
title: Schema Layer (Layer A)
id: PRD-05
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
Layer A is a self-contained package that fetches OVH's API description from each region, normalizes it, and serves it as a queryable cached JSON file (`apispace.json`) to the rest of the binary. It has no dependency on cobra, bubbletea, the credential store, or the typed command surface.

## Goals
- One source of truth for "what OVH knows about" per region, refreshed on demand.
- Read path is cheap (memory-mapped JSON queries via gjson) so the runner can ask "is path X available in region Y" without HTTP.
- Deterministic, atomic cache writes; no torn reads.
- Codegen pipeline (build-time, optional) that converts the cache into typed Go DTOs.

## Non-goals
- Owning runtime auth (it uses go-ovh's anonymous client for `/1.0` discovery).
- Mutating OVH state.
- Becoming a generic OpenAPI tool.

## Module layout
```
internal/schema/
  fetch.go         # walk /1.0, fan out per service via errgroup
  merge.go         # build apispace.json
  cache.go         # XDG paths, atomic write, etag, ttl
  meta.go          # apispace.meta.json
  query.go         # Querier interface (read-only)
  diff.go          # for `ovh schema diff`
  walk.go          # path/method/parameter walkers
tools/schemagen/   # build-time DTO emitter (separate go.mod)
```
Import rule (enforced by `depguard` in golangci-lint):

- **Allowed source imports**: stdlib; `golang.org/x/sync/errgroup`; `github.com/tidwall/gjson`; `github.com/google/renameio/v2`; `github.com/ovh/go-ovh/ovh` (anonymous client only — Layer A never calls `NewOAuth2Client`, never holds AK/AS/CK, never touches the keyring or `internal/auth`); `<module>/internal/xdgpaths` (leaf package wrapping `adrg/xdg`; resolves the schema cache path without pulling in `internal/config`).
- **Transitive deps** brought in via `go-ovh` (notably `golang.org/x/oauth2`) are not source-imported by `internal/schema` and therefore not subject to the rule. Their inclusion in the binary is governed by PRD-08, not by this layer rule.
- **Disallowed source imports**: anything in `internal/cli`, `internal/auth`, `internal/client`, `internal/config`, plus any other third-party module not on the allowlist above.

The lint config enumerates the allowlist; CI fails on any new import that isn't explicitly added.

## `apispace.json` format (v1)
```jsonc
{
  "version": 1,
  "region": "ovh-eu",
  "fetched_at": "2026-05-02T11:14:03Z",
  "etag": "W/\"abc123\"",
  "schema_hash": "sha256:...",
  "services": {
    "/cloud": {
      "description": "Cloud Project APIs",
      "models": { "...": "..." },
      "paths": {
        "/cloud/project/{serviceName}/instance": {
          "GET":  { "description": "...", "params": [...], "response": "...", "auth": ["account:get"] },
          "POST": { "description": "...", "params": [...], "response": "...", "auth": ["..."] }
        }
      }
    }
  }
}
```
Native OVH shape preserved per service, merged under `services`. No OpenAPI conversion in v0.1.

## Fetch
- Use anonymous `go-ovh` client (no auth needed for `/1.0`).
- `GET /1.0/?format=json` -> list of service paths.
- For each service, `GET /1.0/<service>?format=json` concurrently with `errgroup` (max 8 concurrent).
- Honor `If-None-Match` from previous `etag`; on `304`, skip merge and bump `fetched_at`.
- Per-service failures are non-fatal: record as `{"error": "..."}` under `services["/path"].error`, continue.

## Cache
- Path: `$XDG_CACHE_HOME/ovh/schema/<region>/apispace.json` + `apispace.meta.json`.
- Writes via `renameio.WriteFile` with mode 0644.
- TTL: from `default.cache_ttl_hours` (default 24h). `Querier` returns `(data, stale bool)`; stale data is still served — refresh is explicit (`ovh schema refresh`) or background-on-first-use.

## Querier interface
```go
type Querier interface {
    HasPath(method, path string) bool
    Describe(path string) (PathSpec, bool)
    Search(prefix string) []string
    Paths() []string
    Region() string
    FetchedAt() time.Time
    Stale() bool
}
```
Implemented over an `mmap`'d `apispace.json` + gjson; zero-alloc reads.

Package-level constructors (cited by the [canonical package registry in PRD-06](06-runner-and-client.md#canonical-package-registry)):

```go
// Sentinel errors.
var (
    ErrNoCache      = errors.New("schema: no apispace.json for region; call Refresh first")
    ErrCacheCorrupt = errors.New("schema: apispace.json failed validation")
    ErrCachePerms   = errors.New("schema: cache directory mode looser than registry value; refusing to read")
)

// OpenCache opens the cached apispace.json for the given region read-only.
//
// Pre:  region != "".
// Post: nil error => returned Querier is non-nil. Querier.Stale() MAY be true
//       (stale-but-served per cache policy); callers wanting fresh data must
//       call Refresh explicitly. On any non-nil error
//       (ErrNoCache | ErrCacheCorrupt | ErrCachePerms), the returned Querier
//       is nil — calling any method on it panics.
func OpenCache(region string) (Querier, error)

// Refresh re-fetches the API description for the region and atomically
// rewrites apispace.json + apispace.meta.json.
//
// Pre:  ctx != nil; region != ""; len(allowedHosts) > 0 — empty allowlist is a
//       programming error (would silently accept any HTTPS host) and triggers a
//       runtime panic, NOT an error return. allowedHosts is injected from the
//       composition root (see Threat model below; PRD-06 §Composition root).
// Post: nil error => OpenCache(region) returns a non-nil Querier with
//       Stale()==false and FetchedAt() within this call's wall-clock window.
//       Non-nil error => previously cached files are unchanged
//       (atomic-write contract per PRD-04 §Atomic writes).
func Refresh(ctx context.Context, region string, allowedHosts []string) error
```

## Commands (Layer B surface for Layer A)
- `ovh schema refresh [--region] [--all]`
- `ovh schema show <path> [--region] [--method GET]`
- `ovh schema path <prefix>` (search)
- `ovh schema diff <region-a> <region-b>` (capability divergence between clouds)

## Codegen (`tools/schemagen`)
- Separate `go.mod` so build-time codegen deps (see [PRD-08 §Codegen tool](08-security-and-supply-chain.md#codegen-tool-build-time-only) for the current pick) never ship in the runtime binary.
- Reads `apispace.json` for a chosen region (default: `ovh-eu`, the most complete) and emits Go structs for response models into `internal/client/types/`.
- Hand-typed commands import `internal/client/types/`.
- Output is committed; CI re-runs schemagen and fails if the diff isn't empty (catches schema drift in PRs).

## Threat model (schema layer)
- **Malicious response payload from a non-OVH host**: discovery client refuses HTTPS endpoints whose host doesn't match a region's expected API host pattern. The allowed-host list is **passed in as a parameter** from the composition root (`internal/cli/deps.go`) — Layer A does not import `internal/auth/regions.go` to fetch it, preserving the import-allowlist invariant from PRD-05.
- **Cache poisoning by another local user**: cache directory is `$XDG_CACHE_HOME/ovh/schema/`, mode 0700; we refuse to read on looser perms.
- **JSON parser DoS**: response size capped via `http.MaxBytesReader` (default 32MB per service); fan-out concurrency capped.

## Open questions
- Do we ship a default-bundled `apispace.json` (build-baked) so first run is offline-capable, then refresh in background? (Lean: yes, baked at release time.)
- Do we expose `ovh schema export --format openapi3` in v0.1 or v0.2? (Lean: v0.2.)
