---
title: Config & XDG Storage
id: PRD-04
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
The CLI manages a single canonical configuration file in INI format (`ovh.conf`), wire-compatible with `python-ovh`, plus auxiliary XDG locations for cache, state, and credential fallback storage. All keys are namespaced and documented; secret-bearing keys are clearly tagged.

## Goals
- One config format that interoperates with python-ovh (and therefore Ansible) out of the box.
- A documented schema with RW/RO classification, types, and defaults.
- Atomic, durable writes for any file containing secrets.
- XDG compliance (`XDG_CONFIG_HOME`, `XDG_CACHE_HOME`, `XDG_DATA_HOME`, `XDG_STATE_HOME`).

## Non-goals
- Writing to system-wide `/etc/ovh.conf` from the CLI (we read it; we don't write it).
- Fully replacing `~/.ovh.conf` for users who insist on it (we read it; we offer migration).

## File layout (XDG)
```
$XDG_CONFIG_HOME/ovh/ovh.conf            # canonical, INI, python-ovh compatible
$XDG_CONFIG_HOME/ovh/hosts.yml           # YAML, per-region credential state (file fallback only)
$XDG_DATA_HOME/ovh/                      # persistent non-secret state (history, exports)
$XDG_CACHE_HOME/ovh/schema/<region>/     # apispace.json + apispace.meta.json
$XDG_CACHE_HOME/ovh/catalog/<region>/    # flavors, images, regions (24h TTL)
$XDG_STATE_HOME/ovh/logs/ovh.log         # rotating slog output (max 10MB x 5)
```
Path resolution: `internal/xdgpaths` is the only package that source-imports `github.com/adrg/xdg`. It exposes typed accessors (`Config()`, `Cache()`, `Data()`, `State()` plus subpath helpers) and is on the **Layer A allowlist** (PRD-05) so `internal/schema/cache.go` can resolve its own cache path without depending on `internal/config`. `internal/config` is a higher-level package that loads/writes structured config files using paths from `internal/xdgpaths`.

## Read precedence for credentials/config
Matches go-ovh & python-ovh order so users can drop into existing setups:
1. `OVH_*` env vars
2. CLI flags (`--region`, `--config`)
3. `./ovh.conf` (current dir)
4. `$HOME/.ovh.conf`
5. `$XDG_CONFIG_HOME/ovh/ovh.conf`  <- canonical write target
6. `/etc/ovh.conf`

`ovh config status` prints which source won and why. The active source is exposed programmatically as `internal/config.Source` — an enum with values `Env`, `Flag`, `CWDFile`, `HomeFile`, `XDGFile`, `EtcFile` matching the precedence list above. This is the symbol cited as `Source` in the [package registry](06-runner-and-client.md#canonical-package-registry).

## `ovh.conf` schema
INI; one `[default]` section + one section per region id.

```ini
[default]
endpoint = ovh-eu
output = table
storage = keyring

[ovh-eu]
application_key = abc...
application_secret = keyring:ovh-eu:application_secret    ; if storage=keyring
consumer_key = keyring:ovh-eu:consumer_key
client_id = oauth2-client-id
client_secret = keyring:ovh-eu:client_secret
oauth_scopes = account:get,/cloud/project/*

[ovh-us]
; not configured
```

## Canonical key registry
**Single source of truth** for every config key in the project. Other PRDs cite keys by name and link to this section; they do **not** redefine type, default, RW/RO, or enum values. New keys are added here first; only then may another PRD reference them.

Defined in `internal/config/schema.go`; drives `ovh config list --describe`, validation, and shell completion. The `internal/config.Schema` exported symbol (see [PRD-06 package registry](06-runner-and-client.md#canonical-package-registry)) is generated from this table.

| Key | RW | Type | Default | Description |
|---|---|---|---|---|
| `default.region` | RW | enum | `ovh-eu` | Active OVH endpoint id |
| `default.output` | RW | enum | (TTY-aware) | `table\|json\|yaml\|tsv` (template mode is flag-only via `--output template=<expr>`, not persistable) |
| `default.pager` | RW | str | `$PAGER` then `less -R` | Pager for long output |
| `default.editor` | RW | str | `$EDITOR` then `vi` | Editor used by `ovh config edit` |
| `default.no_color` | RW | bool | `false` | Disable ANSI color |
| `default.confirm` | RW | bool | `true` | Require confirm on destructive ops |
| `default.timeout` | RW | duration | `30s` | Client request timeout (e.g. `30s`, `1m`, `2m30s`) |
| `default.cache_ttl_hours` | RW | int | `24` | Catalog & schema cache TTL |
| `default.log_level` | RW | enum | `info` | `error\|warn\|info\|debug` |
| `default.storage` | RW | enum | `keyring` | `keyring\|file` |
| `default.profile` | RW | str | `default` | Active credential profile |
| `default.tui_theme` | RW | enum | `default` | `default\|highcontrast`; TUI theme |
| `<region>.output` | RW | enum | (inherits `default.output`) | Per-region output override; resolved per the precedence chain in PRD-01 |
| `<region>.application_key` | RW | str |  | Classic flow AK |
| `<region>.application_secret` | RW | secret |  | Classic flow AS |
| `<region>.consumer_key` | RW | secret |  | Classic flow CK |
| `<region>.client_id` | RW | str |  | OAuth2 client id |
| `<region>.client_secret` | RW | secret |  | OAuth2 client secret |
| `<region>.oauth_scopes` | RW | csv |  | Requested OAuth2 scopes |
| `<region>.endpoint_url` | RO | url | (from SDK) | Resolved API base URL |
| `<region>.auth_method` | RO | enum |  | `oauth2\|consumer_key\|none` |
| `<region>.token_expires_at` | RO | rfc3339 |  | OAuth2 expiry |
| `<region>.account` | RO | str |  | Logged-in nichandle |
| `<region>.last_validated_at` | RO | rfc3339 |  | Last successful `auth status` |
| `cli.version` | RO | str |  | Binary version |
| `cli.config_path` | RO | path |  | Active `ovh.conf` location |
| `cli.cache_dir` | RO | path |  |  |
| `cli.state_dir` | RO | path |  |  |
| `cli.data_dir` | RO | path |  |  |

## Commands
- `ovh config list [--describe] [--show-secrets]`
- `ovh config get <key>`
- `ovh config set <key> <value>` (rejects RO keys; validates type)
- `ovh config unset <key>`
- `ovh config edit` (opens `default.editor`; re-validates on save)
- `ovh config path` (prints the active config file path)
- `ovh config status` (which source won, perms, secret backend)
- `ovh config migrate` (folds `~/.ovh.conf` into `$XDG_CONFIG_HOME/ovh/ovh.conf`, backing up the original)

## Atomic writes
- All writes via `google/renameio/v2` with parent-dir fsync.
- File modes follow the [Canonical file-mode registry](#canonical-file-mode-registry) below.
- **Backup naming**: `ovh config migrate` (and any other in-place rewrite of a user-owned file) writes the original aside as `<original-path>.bak.<RFC3339-second>` — e.g. `~/.ovh.conf.bak.2026-05-02T15-30-45Z`. Second granularity prevents collisions on repeated migrations.

## Canonical file-mode registry
**Single source of truth** for file/directory permission constants. PRDs 03 and 05 cite by path key; they do **not** introduce new modes inline. Read paths refuse to parse a file whose mode is looser than the registry value when `Refuse-if-looser=yes`.

| Path pattern | Mode | Refuse-if-looser | Rationale |
|---|---|---|---|
| `$XDG_CONFIG_HOME/ovh/` | 0700 | yes | parent of credential fallback files |
| `$XDG_CONFIG_HOME/ovh/ovh.conf` | 0600 | yes | may contain AS/CK when `default.storage=file` |
| `$XDG_CONFIG_HOME/ovh/hosts.yml` | 0600 | yes | credential fallback storage |
| `$XDG_CACHE_HOME/ovh/schema/` | 0700 | yes | per PRD-05 threat model |
| `$XDG_CACHE_HOME/ovh/schema/<region>/apispace.json` | 0644 | no | non-secret schema cache |
| `$XDG_CACHE_HOME/ovh/catalog/` | 0755 | no | non-secret catalog cache |
| `$XDG_DATA_HOME/ovh/` | 0755 | no | non-secret reports / exports / skills artifacts |
| `$XDG_STATE_HOME/ovh/` | 0700 | yes | logs may carry redacted-but-still-sensitive context |
| `$XDG_STATE_HOME/ovh/logs/ovh.log` | 0600 | no | log file |

## Canonical env-var registry
**Single source of truth** for every environment variable the binary reads. Three kinds:
- **`shadow`** — sets a config key; type/default/enum live in the [key registry](#canonical-key-registry) above.
- **`runtime`** — per-invocation behavior, not persisted to any config key.
- **`external`** — third-party convention not under our control (e.g. `NO_COLOR`).

PRDs 01, 03, 07, 11 cite this table by name; they do **not** introduce new env vars without first adding a row here.

| Env | Kind | Equivalent key / behavior |
|---|---|---|
| `OVH_REGION` | shadow | `default.region` |
| `OVH_OUTPUT` | shadow | `default.output` |
| `OVH_NO_COLOR` | shadow | `default.no_color=true` |
| `NO_COLOR` (bare) | external | [no-color.org](https://no-color.org) de-facto standard: any non-empty value disables ANSI color. Read directly by the output formatter; not mapped to a config key. |
| `OVH_NO_PROMPT` | runtime | disables every interactive prompt; CLI-only, not persisted |
| `OVH_CONFIG_FILE` | runtime | overrides config path entirely; CLI-only, not persisted |
| `OVH_TIMEOUT` | shadow | `default.timeout` (parsed as `time.Duration`, e.g. `30s`, `1m`) |
| `OVH_DEBUG` | shadow | sets `default.log_level=debug` when truthy (`1`, `true`, `yes`) |
| `OVH_APPLICATION_KEY` / `OVH_APPLICATION_SECRET` / `OVH_CONSUMER_KEY` | shadow | per-region creds for current `OVH_REGION` |
| `OVH_CLIENT_ID` / `OVH_CLIENT_SECRET` | shadow | OAuth2 creds for current `OVH_REGION` |

## Validation
- Type-coerce on `set`; reject with usage error.
- Enum values come from `internal/config/schema.go` and feed shell completion.
- Region ids validated against `internal/auth/regions.go`.

## Open questions
- Do we support per-host (per-region) `default.*` overrides, e.g. `[ovh-us]\noutput = json`? (Lean: yes, with `<section>.<key>` resolution falling back to `[default]`.)
