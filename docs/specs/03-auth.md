---
title: Authentication
id: PRD-03
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
`ovh` supports two authentication methods in parallel: OAuth2 (preferred for interactive users on regions that support it) and classic Application Key / Application Secret / Consumer Key (required for automation, Ansible, service accounts, and regions where OAuth2 isn't yet rolled out). Per-region credentials, gh-style storage, OS keyring by default with file fallback.

## Goals
- One `ovh auth login` that picks the right flow per region without surprising the user.
- One `ovh auth setup-token` for non-interactive credential install (CI, Ansible bootstrapping).
- Per-region isolation: auth to `ovh-eu` never affects `ovh-us`.
- Refresh and rotation handled transparently when OAuth2 is in use.

## Non-goals
- Owning OVH application creation: users still create the application on the OVH portal; we open the right URL and parse the result.
- Multi-tenant credential brokering.

## Region registry
Source of truth: `internal/auth/regions.go`. Each region declares:
- `id` (e.g. `ovh-eu`)
- `display_name` (e.g. "OVHcloud Europe")
- `endpoint_const` (the `go-ovh` SDK constant)
- `portal_create_app_url`
- `oauth2_issuer` (nullable; nil = OAuth2 not available)
- `oauth2_default_scopes`
- `validation_host_pattern` (see below)

**Adding a region requires two synchronized edits** (no other code changes elsewhere):
1. Append a row to `internal/auth/regions.go`'s registry literal (`Regions()` source).
2. Append a row to the `validation_host_pattern` table below.

A unit test in `internal/auth/regions_test.go` (PRD-10 §Property / contract tests) asserts the two are 1:1 by region id and that no entry has an empty `ValidationHostPattern` slice.

### `validation_host_pattern`
Type: `[]string`. Semantics: a URL is acceptable iff its scheme is `https` and its host **exactly equals** at least one entry. No globs, no regex, no suffix matching — explicit equality only, to keep the security review trivial. Per-region values (concrete values are confirmed against the OVH SDK endpoint constants and the OVH portal URLs at adoption time; phase-2b acceptance gate):

| Region | Allowed hosts |
|---|---|
| `ovh-eu` | `eu.api.ovh.com`, `www.ovh.com` |
| `ovh-us` | `api.us.ovhcloud.com`, `us.ovhcloud.com` |
| `ovh-ca` | `ca.api.ovh.com`, `ca.ovh.com` |
| `kimsufi-eu` | `eu.api.kimsufi.com`, `www.kimsufi.com` |
| `kimsufi-ca` | `ca.api.kimsufi.com`, `ca.kimsufi.com` |
| `soyoustart-eu` | `eu.api.soyoustart.com`, `www.soyoustart.com` |
| `soyoustart-ca` | `ca.api.soyoustart.com`, `ca.soyoustart.com` |

Tests guarantee:
1. Every registry entry has a non-empty `validation_host_pattern`.
2. The classic-flow `validationUrl` and the OAuth2 `authorize_url` returned by OVH parse to a host that matches the active region's list.
3. Mismatch raises an error and refuses to open the browser.

### `Region` Go type
The Go type backing the registry described above. Cited as `Region`, `Regions()`, `RegionByID(id)` in the [package registry](06-runner-and-client.md#canonical-package-registry).

```go
package auth

type Region struct {
    ID                    string   // e.g. "ovh-eu"
    DisplayName           string   // e.g. "OVHcloud Europe"
    EndpointConst         string   // go-ovh constant: "ovh-eu", "ovh-us", "ovh-ca", ...
    PortalCreateAppURL    string
    OAuth2Issuer          string   // empty if OAuth2 not available for this region
    OAuth2DefaultScopes   []string
    ValidationHostPattern []string // exact-host equality list, see subsection above
}

// Regions returns the static registry, ordered by adoption date.
func Regions() []Region

// RegionByID looks up a region; returns (Region{}, false) if unknown.
func RegionByID(id string) (Region, bool)
```

## `ovh auth login`
Interactive picker:
1. If `--region` not given, prompt with the region list.
2. Detect available methods for that region:
   - OAuth2 issuer present -> "OAuth2 (recommended)"
   - Always: "Classic application key / consumer key"
3. User picks method.
4. Run the chosen flow.
5. Persist credentials per `default.storage` (keyring or file).
6. Validate by calling `/me` and store `account` + `last_validated_at` as RO config keys.

Flags:
- `--region <id>`
- `--method {oauth2|consumer-key}`
- `--profile <name>` (default: `default`)
- `--insecure-storage` (force file)
- `--scopes <csv>` (OAuth2)
- `--web` / `--no-web` (force/skip browser)

### OAuth2 flow
- Client-credentials when `--client-id`/`--client-secret` provided (service accounts).
- Authorization-code + PKCE for interactive (when issuer supports it):
  1. Generate PKCE verifier + state.
  2. Open browser via `cli/browser` to issuer's authorize URL.
  3. Local loopback HTTP listener on `127.0.0.1:<random-high-port>` receives the callback.
  4. Exchange code -> access + refresh tokens.
  5. Store tokens; client uses `ovh.NewOAuth2Client` from go-ovh.

Hardening:
- Validate URL host matches `validation_host_pattern` before opening.
- Loopback listener bound to `127.0.0.1` only, single-shot, 5-minute timeout.
- `state` parameter validated on callback.

**Implementation note**: `internal/auth/oauth.go` is the only package in the codebase that source-imports `golang.org/x/oauth2` (PKCE verifier, state, code-exchange primitives). The resulting `*oauth2.TokenSource` is handed to `go-ovh.NewOAuth2Client`. `internal/schema` and `internal/client` do **not** source-import `x/oauth2` — they receive a configured client/credentials value from the composition root.

### Classic CK flow
1. If AK/AS not provided, open `portal_create_app_url` in browser; user pastes AK + AS into prompts (`huh` password fields).
2. POST `/auth/credential` with the rule set derived from `--scopes` (default: read-only on `/*`).
3. Receive `validationUrl` + pending consumer key.
4. Open `validationUrl` in browser; poll `/auth/credential/{id}` until `validationState=validated` or timeout (10 min).
5. Persist consumer key.

For automation, `ovh auth setup-token` accepts AK/AS/CK directly via flags or `OVH_*` env vars and skips the browser.

## Public symbols of `internal/auth` (canonical signatures)
The functions below are the names cited in the [package registry](06-runner-and-client.md#canonical-package-registry) for `internal/auth`. Other PRDs cite by symbol name and link here for the signature.

### `internal/auth/store.go` — credential lifecycle
```go
// Sentinel errors. Callers MUST distinguish via errors.Is — never by string match.
var (
    ErrNotConfigured      = errors.New("auth: no credentials configured for region/profile")
    ErrCredentialsExpired = errors.New("auth: stored credentials expired (OAuth2 refresh required)")
    ErrKeyringUnavailable = errors.New("auth: OS keyring not available; default.storage=file forces file fallback")
    ErrFileModeUnsafe     = errors.New("auth: credential file mode looser than registry value; refusing to read")
)

// LoadCredentials reads creds following PRD-04 §Read precedence
// (env -> flags -> ./ovh.conf -> ~/.ovh.conf -> $XDG_CONFIG_HOME/ovh/ovh.conf -> /etc/ovh.conf;
// the OS keyring is consulted in lieu of the $XDG file when default.storage=keyring).
//
// Pre:  ctx != nil; RegionByID(region) returns (_, true); profile != "".
// Post: nil error => returned Credentials is non-zero with .Region == region.
//       Otherwise returned Credentials is the zero value AND error is one of the
//       sentinels above. Nil-error + zero Credentials is FORBIDDEN.
func LoadCredentials(ctx context.Context, region, profile string) (Credentials, error)

// StoreCredentials persists creds per default.storage. Atomic write at the file mode
// declared in PRD-04 §Canonical file-mode registry.
//
// Pre:  ctx != nil; creds != Credentials{}; creds.Region == region.
// Post: nil error => after fsync, LoadCredentials(ctx, region, profile) returns the
//       same Credentials. Non-nil error => no partial write is observable
//       (renameio.WriteFile semantics; parent dir fsync'd).
func StoreCredentials(ctx context.Context, region, profile string, creds Credentials) error

// DeleteCredentials removes creds for region/profile from both keyring and file fallback.
//
// Pre:  ctx != nil.
// Post: nil error => LoadCredentials(ctx, region, profile) returns ErrNotConfigured.
//       Idempotent: calling on already-empty storage returns nil.
func DeleteCredentials(ctx context.Context, region, profile string) error
```

### `internal/auth/oauth.go` — OAuth2 PKCE
```go
// NewPKCE returns a fresh verifier and its derived S256 challenge.
func NewPKCE() (verifier, challenge string)

// RandomState returns a URL-safe 32-byte random state token.
func RandomState() string

// LoopbackListen binds a single-shot 127.0.0.1 listener, validates authorizeURL's
// host against allowedHosts, opens the URL in the browser, and returns (code, state)
// from the callback. Times out after the given duration.
func LoopbackListen(ctx context.Context, authorizeURL string, allowedHosts []string, timeout time.Duration) (code, state string, err error)

// ExchangePKCE swaps an authorization code for OAuth2 tokens using the PKCE verifier.
func ExchangePKCE(ctx context.Context, region Region, code, verifier string) (Credentials, error)

// RefreshOAuth2 exchanges the stored refresh token for a fresh access token.
//
// Pre:  ctx != nil; LoadCredentials has previously returned successfully with a
//       Credentials carrying a non-empty refresh token.
// Post: nil error => returned Credentials supersedes the previously stored value
//       AND StoreCredentials has been called with the new value before return.
// Concurrency invariant: concurrent calls with identical (region, profile)
//       coalesce through `golang.org/x/sync/singleflight` — at most ONE
//       token-exchange RPC is in flight per (region, profile); all concurrent
//       callers receive the same result. This is required by the TUI mid-session
//       rotation path (PRD-06) where multiple in-flight tea.Cmds may each
//       detect a 401 and dispatch refreshTokenCmd.
func RefreshOAuth2(ctx context.Context, region, profile string) (Credentials, error)
```

### `internal/auth/regions.go` — host validation
```go
// ValidateHost returns nil iff u's scheme is "https" and its host is exactly
// equal to one of allowedHosts. Cited by the OAuth2 flow (before opening the
// browser, see Hardening above) and by the composition root when injecting
// allowedHosts into Layer A (PRD-05 §Threat model).
func ValidateHost(u string, allowedHosts []string) error
```

## `ovh auth status`
Per region (or `--region`):
```
ovh-eu
  account: ab12345-ovh
  method: oauth2
  storage: keyring
  scopes: account:get, /cloud/project/*
  expires: 2026-05-09T12:00:00Z (in 6d)
  last validated: 2026-05-02T11:14:03Z
ovh-us
  not authenticated
ovh-ca
  account: ab12345-ovh
  method: consumer-key
  storage: file ($XDG_CONFIG_HOME/ovh/hosts.yml)
  consumer key: ****abcd
  validated: 2024-09-12T08:00:00Z (no expiry)
```

## `ovh auth switch`
Sets `default.region` (and optionally `default.profile`) atomically; mirrors `gh auth switch`.

Flags:
- `--region <id>` — region to switch to (required unless `--profile` is given and only one region has that profile).
- `--profile <name>` — credential profile within the region (writes `default.profile`).

## `ovh auth refresh`
- **OAuth2**: forces a refresh-token exchange via the cached refresh token; transparent under `--no-prompt`. Failure (refresh token expired/revoked) → exit code 3 with `next: rerun "ovh auth login --region <r>"`.
- **Classic CK**: the consumer-key validation flow inherently requires interactive browser confirmation, so refresh of an *invalid* CK cannot be silent.
  - With prompts allowed: re-runs `/auth/credential` and re-validates via the browser flow.
  - Under `--no-prompt`: refuses with exit code 3 and message `error: classic consumer-key refresh requires interactive browser flow; rerun "ovh auth login --region <r>" without --no-prompt`.
  - When the existing CK is still valid (verified by a `/me` call): no-op, exit 0.

## `ovh auth logout`
- Removes credentials for `--region` (and `--profile` if specified).
- Confirms before deleting unless `--no-prompt`.

## Storage
- **Read precedence** for credentials: per [PRD-04 §Read precedence](04-config-and-storage.md#read-precedence-for-credentialsconfig). Summary: env vars (1) → CLI flags (2) → `./ovh.conf` (3) → `$HOME/.ovh.conf` (4) → `$XDG_CONFIG_HOME/ovh/ovh.conf` (5) → `/etc/ovh.conf` (6). The OS keyring is consulted in lieu of file 5 when `default.storage=keyring`.
- Default: OS keyring via `zalando/go-keyring`.
- Fallback: `$XDG_CONFIG_HOME/ovh/hosts.yml`, mode per [PRD-04 §Canonical file-mode registry](04-config-and-storage.md#canonical-file-mode-registry) (0600, refuse-if-looser).
- `ovh.conf` may also carry credentials in its `[<region>]` section for python-ovh interop; when both present, hosts.yml wins (it tracks expiry/refresh state).

## Threat model (auth)
- **Phishing of validation URL**: mitigated by validating host against `validation_host_pattern` and refusing to open URLs that don't match.
- **Token leak via shell history**: prefer prompts over flags for secrets; secrets-via-flags accepted but documented as CI-only.
- **Token leak via logs**: structured logger has a redaction list; `--debug` masks token-like strings.
- **Compromised file fallback**: chmod 0600, parent dir 0700, refuse on looser perms.
- **Loopback hijack**: bind 127.0.0.1 only, single-shot listener, validate `state`.

## Open questions
- Do we support hardware-backed keyring (Yubikey via PKCS#11) in v0.1? (Lean: no; v0.2.)
- Default OAuth2 scope set per region — confirm with OVH docs before v0.1 RC.
