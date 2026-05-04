---
title: Deferred Scope — Billing & Service Lifecycle
id: PRD-12
status: deferred
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
Concerns that are intentionally **out of the v0.1 operational scope** and tracked here as a separate, future-dated concern. Two distinct families:

1. **Billing & payment** — invoices, bills, debits, payment methods, credit balance, contracts, currency, tax.
2. **Service lifecycle** — renewals, cancellations, new orders, expirations, auto-renew toggling.

These touch financial flows, contractual commitments, and irreversible state changes; they deserve their own UX, threat model, and review process distinct from the day-to-day operational CLI/TUI.

## Why deferred (not just out-of-scope)
- **Different threat profile.** Financial side effects warrant stronger confirmation, audit trails, and likely 2FA-on-action gates that v0.1 does not impose.
- **Different persona.** Finance ops vs platform ops; different defaults, different output expectations (CSV exports, currency formatting, tax-id handling).
- **Different review cadence.** Contractual changes benefit from maker/checker patterns; operational changes don't.
- **OVH-side maturity.** OVH's billing/order endpoints evolve on a different cadence than the cloud APIs; aiming at them is more productive once v0.1 has settled.

## Scope (for the future PRD)

### Billing & payment
- `ovh billing` (proposed): `bill list/show/download`, `debit list`, `credit show`, `payment-method list`, `contract list/show`.
- Cost snapshots / usage exports for finance reporting.
- Multi-currency display, tax-id handling, invoice PDF retrieval.

### Service lifecycle
- `ovh lifecycle` (proposed name; **not reserved** until the successor PRD lands — phase 1/2 work must not reuse the slot to mean something else, but the eventual chosen name may still change): `renew`, `cancel`, `list expirations`, `list contracts`, `auto-renew (on|off)`.
- `ovh billing` likewise (proposed name, not reserved).
- Per-service-family verbs once `lifecycle` settles (`ovh domain renew`, `ovh dedicated cancel`, `ovh vps renew`).
- Order placement: largely stays in the dashboard (captcha, contract acceptance); some renewal-style flows may become feasible.

## What stays in v0.1 (operational scope, for clarity)
- **Cloud**: project, instance, volume, network, image, flavor, region.
- **Domain**: zone, record, refresh, export, import (operational DNS state — not registrar lifecycle).
- **Dedicated server**: list, show, reboot, boot config, IPMI, reinstall (operational state — not contract).
- **VPS**: list, show, start, stop, reboot, reinstall, console.
- **IP**: list, block list, reverse get/set/delete.
- **Storage**: containers (operational only).
- **Auth, config, schema, api passthrough, TUI, completion, version.**

Anything that touches money or service existence/expiration is filed here as deferred.

## Interim affordances in v0.1
Users who need billing or lifecycle today still reach them through:
- `ovh api GET /me/bill`, `GET /me/bill/{id}`, `GET /me/bill/{id}/download` — raw passthrough.
- `ovh api POST /<service>/{id}/confirmTermination` — raw passthrough.
- The OVH dashboard.

This keeps v0.1 honest: "operational" doesn't mean we hide billing — it means we don't pretend we've designed for it.

## Migration path (when this scope opens)
1. Author a successor PRD (likely 13/14) inheriting structure from PRD-01 and PRD-03.
2. Add top-level commands `ovh billing` and `ovh lifecycle` under the same cobra root.
3. Extend `internal/config/schema.go` with finance keys (default currency, tax id, finance contact email).
4. Stronger confirm gates: `--yes-i-really-mean-it` style + interactive 2FA prompt for cancellations and renewals.
5. Append-only audit log capturing every lifecycle/billing mutation with timestamp, user, command, result. **Concrete path, format, rotation, and file mode** to be defined in the successor PRD (added to PRD-04's [file-mode registry](04-config-and-storage.md#canonical-file-mode-registry) and key registry at that time) — not pre-specified here so the choice can be informed by the threat model in force then.
6. Separate Ansible-friendly JSON contract review before any of these are marked GA.

## Open questions
- Will billing live alongside `ovh` as a sibling subcommand or be a separate binary `ovh-billing` for clearer separation? (Lean: subcommand, gated by a confirm-flag default.)
- Do we expose a read-only "billing dashboard" view (no mutations) earlier than full lifecycle? (Lean: yes — read-only billing is a smaller threat profile and ships before lifecycle.)
