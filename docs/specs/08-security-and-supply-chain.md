---
title: Security & Supply Chain
id: PRD-08
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
Authoritative record of which dependencies the project uses, how sensitive each is, why each was chosen over alternatives, and what supply-chain controls gate every release.

## Sensitivity classification
- **High**: handles credentials, tokens, signing.
- **Medium**: handles secret-bearing data in transit (config parse, atomic write of secret files, secret prompts, browser URLs containing tokens).
- **Low**: neither.

All license fields are **verified at adoption time** by reading each repo's `LICENSE` file; the upstream commit hash of the reviewed `LICENSE` is recorded next to the dep when added.

## High-sensitivity picks

### Auth + signing transport
- **Pick**: `github.com/ovh/go-ovh/ovh` (BSD-3-Clause)
- **Alternatives evaluated**:
  - hand-roll on `net/http` (rejected: re-implementing crypto-adjacent signing)
  - `lvauvillier/ovh-go` (MIT, unmaintained)
  - generic `oapi-codegen` client (Apache-2.0, OVH spec isn't OpenAPI)

### OAuth2 protocol primitives
- **Pick**: `golang.org/x/oauth2` (BSD-3-Clause)
- **Alternatives evaluated**:
  - `coreos/go-oidc/v3` (Apache-2.0, OVH isn't OIDC)
  - `int128/oauth2cli` (Apache-2.0, niche helper)
  - hand-roll (rejected: refresh + PKCE + state + expiry math)

### Credential storage (OS keyring)
- **Pick**: `github.com/zalando/go-keyring` (MIT)
- **Alternatives evaluated**:
  - `99designs/keyring` (MIT, larger surface, kept as fallback option)
  - `keybase/go-keychain` (MIT, macOS-only)
  - per-platform assembly (`danieljoos/wincred` + dbus + go-keychain) — we'd become the integrator.

## Medium-sensitivity picks

### Browser open
- **Pick**: `github.com/cli/browser` (BSD-2-Clause), gh's hardened fork.
- **Alternatives evaluated**:
  - `pkg/browser` (BSD-2, unmaintained)
  - `skratchdot/open-golang` (MIT, less rigor)
  - raw `exec.Command` (we'd own the platform matrix)
- **Hardening**: validate URL scheme=https, host matches the region's `validation_host_pattern`, reject userinfo, log redacted URL only.

### Secret prompts
- **Pick**: `github.com/charmbracelet/huh` (MIT) for interactive CLI prompts; `golang.org/x/term.ReadPassword` (BSD-3) as the **CLI-only** fallback for password-style reads when `huh` cannot run (e.g., constrained TTY in some terminal multiplexers). The TUI never falls back to `x/term`: if `ovh tui` is invoked without a TTY it refuses and exits with code 4 (per PRD-02).
- **Alternatives evaluated**:
  - x/term alone (rejected: charm UX divergence)
  - `AlecAivazis/survey/v2` (MIT, double prompts library)
  - `manifoldco/promptui` (BSD-3, less active)
- **Hardening**: secrets in `[]byte`, zeroed after use; never formatted via slog/fmt verbs; redaction list applied to the structured logger. **Test asserts**: after `WithSecret(b []byte, fn func([]byte))` returns, `bytes.Equal(b, make([]byte, len(b)))` holds (PRD-10 §Property / contract tests).

### INI config parser
- **Pick**: `gopkg.in/ini.v1` (Apache-2.0), wire-compatible with python-ovh.
- **Alternatives evaluated**:
  - `go-ini/ini` (same upstream)
  - `BurntSushi/toml` (MIT, breaks Ansible compat)
  - hand-roll (comment preservation is more code than expected)

### Config orchestration
- **Pick**: `github.com/knadh/koanf/v2` (MIT)
- **Alternatives evaluated**:
  - `spf13/viper` (MIT, heavy global state, surprising precedence)
  - `cristalhq/aconfig` (MIT, smaller ecosystem)
  - hand-roll
- **Hardening**: secret-bearing keys are read only via `internal/config.Get*()` typed accessors that route through the redaction filter; never marshal a koanf instance directly to a logger or error message; never call koanf's raw `*Bytes()` / `Sprint()` dump helpers in non-test code. A test asserts no log line carries an un-redacted secret-bearing key value.

### Atomic file writes
- **Pick**: `github.com/google/renameio/v2` (Apache-2.0)
- **Alternatives evaluated**:
  - `natefinch/atomic` (MIT, less active)
  - `rogpeppe/go-internal/lockedfile` (BSD-3, adds locking we don't need)
  - stdlib (forgets parent-dir fsync)

### JSON parsing
- **Pick**: stdlib `encoding/json` for typed paths; `github.com/tidwall/gjson` (MIT) for read-only queries against cached `apispace.json`.
- **Alternatives evaluated**:
  - `goccy/go-json` (MIT, recent CVEs)
  - `buger/jsonparser` (MIT, less ergonomic)
  - `valyala/fastjson` (MIT, mutable API foot-guns)
- **Hardening**: `http.MaxBytesReader` cap on every API response; explicit struct types for typed commands; gjson never reads raw network bytes.

### Codegen tool (build-time only)
- **Pick**: `github.com/dave/jennifer` (BSD-2-Clause)
- **Alternatives evaluated**:
  - stdlib `text/template`
  - `golang.org/x/tools/go/ast`
  - `oapi-codegen` (Apache-2.0, OpenAPI-only)
- **Note**: `tools/schemagen` lives behind a separate `go.mod`, so jennifer never ships in the runtime binary.

## Low-sensitivity picks
- `spf13/cobra` (Apache-2.0) — alternatives: `urfave/cli` (MIT), `alecthomas/kingpin` (MIT), `mitchellh/cli` (MPL-2.0).
- `adrg/xdg` (MIT) — alternatives: `kirsle/configdir` (MIT), `OpenPeeDeeP/xdg` (MIT).
- `golang.org/x/sync/errgroup` (BSD-3) — alternatives: `sourcegraph/conc` (MIT), `alitto/pond` (MIT).
- `charmbracelet/{bubbletea,bubbles,lipgloss}` (MIT) — alternatives: `rivo/tview` (MIT), `gdamore/tcell` (Apache-2.0).
- `sahilm/fuzzy` (MIT) — alternatives: `lithammer/fuzzysearch` (MIT), `renstrom/fuzzysearch` (MIT).
- `google/go-cmp` (BSD-3) — alternatives: `stretchr/testify` (MIT), stdlib `reflect.DeepEqual`.
- `itchyny/gojq` (MIT) — pure-Go jq, alternatives: shelling out to `jq` (rejected: not always installed).

## Canonical redaction allowlist
**Single source of truth** for what the structured logger and output formatter must mask. PRDs 03 and 06 cite this table; PRD-10 has a CI gate asserting no committed golden fixture violates these rules.

| Field / header / pattern | Where it appears | Redaction rule |
|---|---|---|
| `application_key` | config keys, slog attrs | replace with `****<last4>` |
| `application_secret` | config keys, slog attrs | full mask `****` |
| `consumer_key` | config keys, slog attrs | replace with `****<last4>` |
| `client_id` | config keys, slog attrs | replace with `****<last4>` |
| `client_secret` | config keys, slog attrs | full mask `****` |
| `oauth.access_token` | token storage, slog | full mask `****` |
| `oauth.refresh_token` | token storage, slog | full mask `****` |
| `X-Ovh-Application` | outbound HTTP request headers in slog | replace value with `****<last4>` |
| `X-Ovh-Consumer` | outbound HTTP request headers in slog | replace value with `****<last4>` |
| `X-Ovh-Signature` | outbound HTTP request headers in slog | full mask `****` |
| `Authorization` (Bearer) | outbound HTTP request headers in slog | replace value with `Bearer ****<last4>` |
| Token-shaped bare strings (`/^[a-f0-9]{32,}$/`) | error messages, slog values | replace with `****<last4>` |

`--show-secrets` (on `config get` / `config list`) bypasses redaction for the user's own *config* values; it never bypasses redaction for outbound HTTP headers, OAuth tokens, or log lines.

## Canonical depguard ruleset
**Single source of truth** for sensitive third-party imports that must be confined to specific packages. The per-package allowlists implicit in [PRD-06 §Canonical package registry](06-runner-and-client.md#canonical-package-registry) cover most of the surface; this table carries the **explicit deny rules** that span packages and must be present in `.golangci.yml` for the layer rules to be mechanically enforceable.

| Denied import | Allowed only in | Rationale |
|---|---|---|
| `golang.org/x/oauth2` | `internal/auth` | OAuth2 primitives confined to one package; PRD-03 §Implementation note |
| `github.com/ovh/go-ovh/ovh` | `internal/auth`, `internal/client`, `internal/schema` | OVH HTTP boundary; PRD-06 §Composition root |
| `github.com/zalando/go-keyring` | `internal/auth` | Credential storage isolation; PRD-08 §High-sensitivity picks |
| `github.com/cli/browser` | `internal/auth` | Browser-open isolation (URL hardening per PRD-03) |
| `github.com/adrg/xdg` | `internal/xdgpaths` | XDG path resolution single owner; PRD-04 |
| `gopkg.in/ini.v1` | `internal/config` | INI parsing isolated to config layer |
| `golang.org/x/term` | `internal/auth` | Terminal-secret-read confined to auth (CLI fallback only; PRD-08 §Secret prompts) |

`golangci-lint run` fails on any violation; CI gates on a clean run (PRD-10 §Quality gates). **Adding a new third-party dep that handles secrets, credentials, or external I/O requires a new row here before the import is added.**

## License policy
Allowlist: `MIT`, `BSD-2-Clause`, `BSD-3-Clause`, `Apache-2.0`, `ISC`, `MPL-2.0`.
Deny: `GPL-*`, `AGPL-*`, `LGPL-*` (we ship statically linked binaries), unlicensed.
Enforced via `google/go-licenses report` in CI; `THIRD_PARTY_LICENSES.md` regenerated on every release.

## Supply-chain controls
| Control | How |
|---|---|
| Pinning | `go.sum` committed; `GOFLAGS=-mod=readonly` in CI |
| Verification | `go mod verify`; default `GOSUMDB=on` |
| Vendoring | `go mod vendor` committed; CI fails on `git diff -- vendor/` |
| Vulnerability scan | `govulncheck` on PRs + weekly cron; `osv-scanner` augment |
| Static analysis | `golangci-lint` (revive, staticcheck, errcheck, govet, gosec, bodyclose, noctx, gocritic) |
| Dependency updates | Dependabot weekly for `gomod` and `github-actions` |
| Posture | OpenSSF Scorecard action, gates min score |
| Provenance | goreleaser SLSA-3 attestation |
| SBOM | goreleaser CycloneDX via syft, attached to releases |
| Signing | cosign keyless (Sigstore) on tags; signatures on tap formula too |
| Reproducible builds | `-trimpath`, fixed `-buildid`, goreleaser reproducible mode |
| Secret scanning | GitHub native + `gitleaks` action; `.gitleaks.toml` allowlists fixtures only |
| Branch protection | required reviews, required CI checks, no force-push to main |

## Disclosure
- `SECURITY.md` references private security advisory channel via GitHub.
- Embargo policy: 90 days or until fix released, whichever sooner.
- Coordinated disclosure encouraged; CVE filed by maintainers.

## Open questions
- SLSA level target for v0.1: 3 (build provenance, hermetic) feasible; 4 (two-person review) post-v0.1.
- Do we additionally publish signed `cosign.bundle` artifacts for offline verification? (Lean: yes; small.)
