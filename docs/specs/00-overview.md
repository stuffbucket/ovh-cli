---
title: OVH CLI/TUI — Project Overview
id: PRD-00
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
`ovh` is a Go CLI and bubbletea TUI that wraps the OVHcloud API for the most common **operational** day-to-day jobs (cloud projects/instances/volumes/networks, domains/records, dedicated servers, VPS, IP, storage). UX is patterned on `gh` and `az`. A single static binary, distributed via Homebrew tap (`stuffbucket/homebrew-tap`), goreleaser-built, MIT-licensed. **Billing/payment and service lifecycle (renewals, cancellations, new orders) are explicitly deferred to a separate concern — see PRD-12.**

## Goals
- One binary, one consistent taxonomy across all OVH service families.
- Authentication parity with both OVH auth modes: OAuth2 (preferred where supported) and classic Application Key / Application Secret / Consumer Key (required for automation).
- Multi-region first-class: `ovh-eu`, `ovh-us`, `ovh-ca`, plus `kimsufi-*`, `soyoustart-*`. No region hard-coding.
- gh-style muscle memory: `auth login`, `auth status`, `auth switch`, `config get/set`, `api`, `completion`, `version`.
- Ansible/automation friendly: stable JSON output, deterministic exit codes, full env-var configuration, no required interaction.
- All persistent state in XDG Base Directory locations.
- Curated `.claude/skills/` shipping in-repo for common multi-step jobs.
- Supply-chain hardened from day one (vendored deps, pinned, scanned).

## Non-goals
- Not a drop-in replacement for `python-ovh` clients — but stays compatible with `ovh.conf` so they can co-exist.
- Not a hosted service, daemon, or background agent.
- No telemetry, no analytics, no auto-update.
- Not exhaustive coverage of every OVH API path in v0.1 — the long tail goes through `ovh api` (raw passthrough).
- Not an IaC tool — provisioning workflows live in skills/playbooks, not in the CLI core.
- **No billing, payment, invoice, or contract surface** in v0.1 (separate concern; see PRD-12).
- **No service lifecycle verbs** — no renewals, cancellations, or new orders in v0.1 (separate concern; see PRD-12).

## Stakeholders
- Primary user: operators running OVH workloads from a terminal or playbook.
- Secondary user: Claude Code agents invoking `.claude/skills/`.
- Maintainers: stuffbucket org.

## Project identity
- **Go module**: `github.com/stuffbucket/ovh-cli`
- **Binary**: `ovh`
- **Repo**: `github.com/stuffbucket/ovh-cli`
- **Homebrew tap**: `github.com/stuffbucket/homebrew-tap`
- **License**: MIT
- **Minimum Go version**: 1.26 (declared in `go.mod`; CI matrix tests current and current-1)

## Citation pattern (single-owner registries)
Cross-cutting symbols are defined in **canonical single-owner sections**. Other PRDs cite by name and link back; they do not redefine type, default, allowlist, signature, or value.

**Primary registries** (any new symbol of these classes lands here first):

1. **Config key registry** — every `default.*`, `<region>.*`, `cli.*` config key.
   Owner: [PRD-04 → Canonical key registry](04-config-and-storage.md#canonical-key-registry).
2. **Env-var registry** — every environment variable the binary reads (shadow/runtime/external).
   Owner: [PRD-04 → Canonical env-var registry](04-config-and-storage.md#canonical-env-var-registry).
3. **Package registry** — every `internal/<pkg>` package: layer, allowed imports, exported symbols.
   Owner: [PRD-06 → Canonical package registry](06-runner-and-client.md#canonical-package-registry).
4. **Exit-code registry** — every process exit code with its Go constant in `internal/cli/exitcode`.
   Owner: [PRD-01 → Canonical exit-code registry](01-cli-ux.md#canonical-exit-code-registry).
5. **Error-code registry** — every JSON `error.code` string, mapped to its exit code.
   Owner: [PRD-01 → Canonical error-code registry](01-cli-ux.md#canonical-error-code-registry).

**Secondary canonical sections** (finer-grained constants and conventions):

6. **File-mode registry** — XDG path → permission mode + refuse-if-looser flag.
   Owner: [PRD-04 → Canonical file-mode registry](04-config-and-storage.md#canonical-file-mode-registry).
7. **Redaction allowlist** — fields/headers/patterns the logger and formatter must mask.
   Owner: [PRD-08 → Canonical redaction allowlist](08-security-and-supply-chain.md#canonical-redaction-allowlist).
8. **Cross-cutting conventions** — request-id format, struct tag grammar, retry policy.
   Owner: [PRD-06 → Cross-cutting conventions](06-runner-and-client.md#cross-cutting-conventions).
9. **Depguard ruleset** — third-party imports denied except in named packages.
   Owner: [PRD-08 → Canonical depguard ruleset](08-security-and-supply-chain.md#canonical-depguard-ruleset).

When a PRD needs to introduce a new symbol of any of these classes: **add the row to the appropriate canonical section first**; only then may any other PRD reference it. This prevents the cross-PRD name drift ("bridge-rot") surfaced by the boundary-review rounds — a name introduced in passing in one PRD, treated as authoritative by another, defined nowhere. The pattern collapses recurring drift defects into "is the registry row up to date?"

## Architecture (one-liner)
Two-layer monolith. **Layer A** (schema) is a self-contained package that fetches/normalizes/caches the OVH API surface to `apispace.json`. **Layer B** (runner) is the cobra/bubbletea shell that consumes Layer A's cache, talks to OVH via `github.com/ovh/go-ovh/ovh`, and reads/writes XDG config. Layer B may import Layer A; never the reverse.

## Success criteria for v0.1
- `ovh auth login` succeeds end-to-end via OAuth2 *and* classic CK on `ovh-eu`, `ovh-us`, `ovh-ca`.
- `ovh me`, `ovh cloud project list`, `ovh cloud instance list/show/create/delete`, `ovh domain record list/add/delete`, `ovh dedicated server list/show`, `ovh vps list`, `ovh ip list` all work in human and JSON modes.
- `ovh api` reaches any OVH endpoint with full schema-driven completion.
- TUI launches, lists services, drills in to instances/records, supports CRUD with credential prompts.
- Homebrew install: `brew install stuffbucket/tap/ovh` works on macOS arm64/amd64 and Linux.
- An Ansible role using `command:` to drive `ovh` round-trips a DNS record without interactive prompts.
- All CI gates pass (build, test, vet, lint, govulncheck, gosec, gitleaks, license check, vendor drift).

## PRD index
- [01 — CLI UX](01-cli-ux.md)
- [02 — TUI](02-tui.md)
- [03 — Authentication](03-auth.md)
- [04 — Config & Storage](04-config-and-storage.md)
- [05 — Schema Layer](05-schema-layer.md)
- [06 — Runner & Client](06-runner-and-client.md)
- [07 — Claude Skills](07-claude-skills.md)
- [08 — Security & Supply Chain](08-security-and-supply-chain.md)
- [09 — Release & Distribution](09-release-and-distribution.md)
- [10 — Testing & Quality](10-testing-and-quality.md)
- [11 — Roadmap](11-roadmap.md)
- [12 — Deferred Scope: Billing & Lifecycle](12-deferred-billing-and-lifecycle.md)
