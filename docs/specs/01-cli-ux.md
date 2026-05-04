---
title: CLI UX
id: PRD-01
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
The `ovh` command surfaces OVH services with a `gh`-shaped taxonomy: noun-then-verb subcommands, consistent flags, TTY-aware output, machine-friendly modes, and explicit automation knobs. No pseudo-concepts are invented to bridge OVH's URL tree onto the gh shape — where divergence is unavoidable, command names follow OVH's own service vocabulary.

## Goals
- One taxonomy that fits in one screen of help and reads like `gh`.
- Output good for humans by default, good for pipelines on demand, good for Ansible always.
- Errors that include the next step a user should take.

## Non-goals
- 1:1 mirror of OVH's REST URL tree as command paths (`ovh api` does that).
- Verb soup (`ovh cloud-instance-create-from-snapshot-boot-once`); composition lives in flags.

## Command taxonomy (v0.1)
```
ovh auth     login | logout | status | refresh | setup-token | switch
ovh config   list | get | set | unset | edit | path | status | migrate
ovh me       show | ssh-key (list|add|delete) | api-application list
ovh cloud    project (list|show|quota)
             instance (list|show|create|delete|reboot|rebuild|ssh|console)
             volume   (list|show|create|delete|attach|detach|snapshot)
             network  (list|show|create|delete)
             image    (list|show)
             flavor   (list)
             region   (list)
ovh domain   list | show
             record (list|add|edit|delete)
             zone   (refresh|export|import)
ovh dedicated server (list|show|reboot|boot|ipmi|reinstall)
ovh vps      list | show | start | stop | reboot | reinstall | console
ovh ip       list | block list | reverse (get|set|delete)
ovh storage  list | container (list|show|create|delete)
ovh schema   refresh | show | path | diff
ovh api      <METHOD> <path> [-f key=val] [-F key=@file] [--paginate] [--jq] [--describe]
ovh tui                                                    # launch bubbletea
ovh completion bash|zsh|fish|powershell
ovh version
```

## Global flags
- `--region <id>` — overrides default region for this invocation.
- `--profile <name>` — selects a named credential profile (per-region).
- `--output, -o {table|json|yaml|tsv|template=<expr>}` — output format. The `template=<expr>` form is flag-only; the persistable `default.output` config key accepts only the bare enums (`table|json|yaml|tsv`) per PRD-04.
- `--jq <expr>` — filter JSON output through gojq, mirrors `gh`.
- `--no-color` / `NO_COLOR`.
- `--no-prompt` / `OVH_NO_PROMPT=1` — disables every interactive prompt.
- `--debug` / `OVH_DEBUG=1` — slog at debug to stderr (secrets redacted).
- `--config <path>` / `OVH_CONFIG_FILE`.
- `--timeout <duration>` / `OVH_TIMEOUT`.

## Output rules
- Default: human-readable table when stdout is a TTY, structured JSON otherwise (matches gh).
- `--output table` works even when piped (overrides auto-detect).
- Precedence: `--output` flag > `OVH_OUTPUT` env > `[<region>].output` (when set, per PRD-04) > `[default].output` > TTY auto-detect.
- **All commands honor `--output`, including diagnostics** (`auth status`, `config status`, `config list`, `config get`, `config path`, `schema show`, `me show`, `version`). Diagnostic JSON shapes are committed as goldens like any other command's output and are subject to the same JSON-stability contract (PRD-01 Automation contract; breaking changes only on major version bumps).
- All list commands accept `--limit`, `--page`, `--all` (auto-paginate).
- Templates use Go `text/template` with a sprig-like helper subset.

## Help patterns
- `ovh <cmd> --help`: short one-liner, "Examples", flag groups (Filters / Output / Global).
- `ovh help <cmd>` mirrors `--help` (gh parity).
- Examples always use real-looking values; no `<placeholder>` syntax.
- All help text is generated from cobra metadata; no drift between `--help` and docs.

## Errors
- Single-line summary + a "Next step" hint:
  ```
  error: not authenticated for region ovh-us
    next: run "ovh auth login --region ovh-us"
  ```
- JSON mode emits `{"error":{"code":"AUTH_REQUIRED","message":"...","next":"..."}}` to stderr and exits non-zero.

## Canonical exit-code registry
**Single source of truth** for every process exit code. Other PRDs cite by Go constant name (e.g. `exitcode.AuthFailure`) and link here; they do **not** introduce new codes inline. Constants live in `internal/cli/exitcode` (see [package registry](06-runner-and-client.md#canonical-package-registry)).

| Code | Constant | Meaning |
|---|---|---|
| 0 | `exitcode.OK` | Success |
| 1 | `exitcode.Generic` | Generic error |
| 2 | `exitcode.Usage` | Usage error (bad flags/args) |
| 3 | `exitcode.AuthFailure` | Authentication / authorization failure |
| 4 | `exitcode.MissingInput` | Missing required input under `--no-prompt` |
| 5 | `exitcode.NotFound` | Resource not found (404) |
| 6 | `exitcode.Conflict` | Conflict / precondition failed (409 / 412) |
| 7 | `exitcode.RateLimited` | Rate limited (429) |
| 8 | `exitcode.Network` | Network / transport error |

Adding a new code: add a row, declare the constant in `internal/cli/exitcode`, then any PRD may cite it by constant name.

## Canonical error-code registry
**Single source of truth** for the `code` strings that appear in JSON error output: `{"error":{"code":"...","message":"...","next":"..."}}`. Constants live alongside exit codes in `internal/cli/exitcode`.

| Code string | Emitted when | Maps to exit code |
|---|---|---|
| `AUTH_REQUIRED` | no creds for active region; `/me` returns 401 with no token | `exitcode.AuthFailure` |
| `AUTH_INVALID` | `/me` returns 401/403 with stored token | `exitcode.AuthFailure` |
| `MISSING_INPUT` | required flag/arg absent under `--no-prompt` | `exitcode.MissingInput` |
| `NOT_FOUND` | OVH API returned 404 | `exitcode.NotFound` |
| `CONFLICT` | OVH API returned 409 | `exitcode.Conflict` |
| `PRECONDITION_FAILED` | OVH API returned 412 | `exitcode.Conflict` |
| `RATE_LIMITED` | OVH API returned 429 | `exitcode.RateLimited` |
| `NETWORK` | transport-level failure (DNS, TLS, timeout) | `exitcode.Network` |
| `USAGE` | bad flag/arg combination caught locally | `exitcode.Usage` |
| `INTERNAL` | unexpected internal error | `exitcode.Generic` |

New error codes follow the same registry-first rule.

## Idempotency
Two `--ensure` shapes, one per resource semantic:

- **Existence** — `--ensure {present|absent}` on resources with a natural identity that can be created or deleted: DNS records, SSH keys, reverse-DNS entries, named instances/volumes/networks, storage containers.
- **Power state** — `--ensure {running|stopped|rebooted}` on resources whose lifecycle commands manipulate state rather than existence: `cloud instance`, `vps`, `dedicated server` start/stop/reboot verbs.

`reinstall` is **not** an `--ensure` value — it is a destructive verb retained at top level (`vps reinstall`, `dedicated server reinstall`, `cloud instance rebuild`). Use `reinstall --if-changed --image <id>` to skip when the image already matches.

`zone import` accepts `--strategy {merge|replace|dry-run}` (default `merge`). `--if-changed` is honored: with it, the command computes a diff against the current zone and exits 0 with no mutations if the imported state would be identical. `--strategy replace` is destructive and requires confirmation (or `--no-prompt` plus an explicit `--yes`).

Plus:
- `--if-not-exists` on create (no-op when the target identity already matches).
- `--if-changed` on update (no-op when the request body would not change observable state).

`record edit` semantics: full-record replacement keyed by `(zone, name, type, target)`. For idempotent upserts, prefer `record add --ensure present` which creates-or-updates by the same key.

## Out of scope for v0.1 (operational focus)
- **No billing surface.** No `ovh billing *`, no `ovh me bill *`. Reach billing data via `ovh api GET /me/bill` raw passthrough.
- **No service lifecycle.** No renew/cancel/new-order verbs. Reach via `ovh api POST /<service>/{id}/confirmTermination` and similar raw passthroughs.
- Tracked separately in PRD-12; revisited as a distinct concern with its own UX, threat model, and audit-log requirements.

## Automation contract (Ansible / scripts)
- All `OVH_*` env vars override config; the **authoritative list** is the [canonical env-var registry in PRD-04](04-config-and-storage.md#canonical-env-var-registry). Commonly used in playbooks: `OVH_REGION`, `OVH_OUTPUT`, `OVH_NO_PROMPT`, `OVH_NO_COLOR`, `OVH_CONFIG_FILE`, `OVH_TIMEOUT`, `OVH_DEBUG`, `OVH_APPLICATION_KEY`, `OVH_APPLICATION_SECRET`, `OVH_CONSUMER_KEY`, `OVH_CLIENT_ID`, `OVH_CLIENT_SECRET`. New env vars must be added to that registry first.
- JSON shape is documented per command and version-stable; breaking changes only on major version bumps.
- `ovh.conf` written by `ovh auth login` is byte-compatible with `python-ovh`, so Ansible modules from `community.general.ovh_*` and `synthesio.ovh` keep working.
- `ovh api` raw passthrough for endpoints not yet typed.

## Open questions
- Does `--profile` semantically equal "second account in same region", "alternate auth method", or both?
- `--jq` requires CGO-free pure-Go jq; pick `itchyny/gojq`.
