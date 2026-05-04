---
title: Roadmap
id: PRD-11
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
Phased delivery from skeleton to v0.1 release. Each phase has a single demonstrable deliverable and explicit acceptance criteria; we don't move on until the criteria pass and CI is green.

## Phase 1 — Skeleton + supply chain
**Deliverable**: `ovh --help`, `ovh version`, `ovh completion <shell>` work; CI runs every gate; vendoring committed.

Acceptance:
- `go build ./...` green.
- `.github/workflows/ci.yml` runs build, test, vet, lint, govulncheck, gosec, gitleaks, license check, vendor drift.
- `.github/dependabot.yml` configured.
- `docs/security/SECURITY.md` published.
- `THIRD_PARTY_LICENSES.md` generated.
- Repo-local git identity set per stuffbucket convention; first commit pushed.

## Phase 2a — Schema layer (Layer A)
**Deliverable**: `ovh schema refresh`, `show`, `path`, `diff` work offline against committed fixtures and online against real OVH.

Acceptance:
- `apispace.json` produced and queryable for `ovh-eu`, `ovh-us`, `ovh-ca`.
- `Querier` interface stable; >85% coverage in `internal/schema`.
- ETag honored; second refresh in <1s.
- Schema goldens committed.

## Phase 2b — Auth + raw API
**Deliverable**: `ovh auth login` end-to-end (OAuth2 + classic CK), `ovh auth status`, `ovh auth setup-token`, `ovh api`.

Acceptance:
- OAuth2 PKCE flow works on `ovh-eu`.
- Classic CK flow works on `ovh-eu`, `ovh-us`, `ovh-ca`.
- Credentials persisted to keyring on macOS/Linux; file fallback works on headless Linux.
- `--no-prompt` mode functional with `OVH_*` env vars.
- `ovh auth setup-token --ak X --as Y --ck Z --region ovh-eu --no-prompt` writes valid creds without opening a browser; `ovh auth status --region ovh-eu` confirms within the same shell.
- `ovh.conf` written by `auth login`/`setup-token` is byte-compatible with `python-ovh`: a roundtrip test (a) parses the file with `gopkg.in/ini.v1`, (b) writes it back, (c) asserts comments and section ordering preserved, (d) loads it via a `python-ovh` fixture and asserts same key/value extraction.
- `ovh auth refresh --no-prompt` succeeds for OAuth2 and exits 3 with the documented next-step message for classic CK (per PRD-03).
- `ovh api GET /me` round-trips signed.
- Fake-server integration tests pass.

## Phase 3 — Cloud + Domain
**Deliverable**: `ovh cloud project list/show/quota`, `ovh cloud instance list/show/create/delete/reboot/ssh`, `ovh cloud volume *`, `ovh cloud network *`, `ovh domain list/show`, `ovh domain record *`, `ovh domain zone refresh/export/import`.

Acceptance:
- All commands have JSON goldens.
- All mutating commands implement `--ensure`/`--if-not-exists`/`--if-changed` where applicable.
- Idempotency tests pass on fake server.

## Phase 4 — Dedicated, VPS, IP, Storage
**Deliverable**: remaining v0.1 typed commands.

## Phase 5 — TUI
**Deliverable**: `ovh tui` covers all v0.1 services with full CRUD.

Acceptance:
- teatest goldens green.
- Credential prompts use `huh`; non-TTY invocations of `ovh tui` refuse and exit 4 (no `x/term` fallback in TUI, per PRD-02 and PRD-08).
- `?` help, `/` filter, `:` palette functional.

## Phase 6 — Skills, docs, goreleaser
**Deliverable**: `.claude/skills/<5 skills>` shipped, cobra-generated docs published, goreleaser produces a clean release on a test tag, `stuffbucket/homebrew-tap` formula merged.

Acceptance:
- `brew install stuffbucket/tap/ovh` works on a fresh macOS arm64 box.
- All artifacts cosign-verified, SBOMs attached.

## Phase 7 — Polish
**Deliverable**: error message audit, structured log audit, retry/backoff tuning, performance pass on `apispace.json` queries, docs proofread.

Acceptance:
- All error messages include "next step".
- No `slog` call leaks credentials in `--debug` mode (audited).
- `ovh schema show <path>` median latency <50ms over committed fixtures.

## Out of scope for v0.1
- **Billing, payment, invoices, contracts.** Separate concern; see PRD-12.
- **Service lifecycle: renewals, cancellations, new orders.** Separate concern; see PRD-12.
- Self-update.
- Multi-tenant credential brokering.
- Hardware keyring (Yubikey/PKCS#11).
- OpenAPI export.
- Docker image (likely v0.2).
- apt/yum repo (v0.2).

## Open questions
- Cut-over from v0.1 -> v1.0: trigger on JSON shape stabilization, Ansible community feedback, and 30 days of zero `--output json` regressions.
