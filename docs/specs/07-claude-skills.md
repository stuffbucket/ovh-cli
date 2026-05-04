---
title: Claude Skills (.claude/skills)
id: PRD-07
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
The repo ships a curated set of Claude Code skills under `.claude/skills/` that drive `ovh` for common multi-step jobs. Each skill is a `SKILL.md` plus thin scripts; none invents new APIs — they shell out to `ovh`.

## Goals
- Useful from day one for repetitive **operational** jobs (provisioning, DNS bulk edits, fleet audits).
- All side effects go through `ovh` (and therefore through audited auth + structured output).
- Skills work under `--no-prompt` so they're safe in playbooks.
- No skill in v0.1 touches billing, payment, or service-lifecycle (renew/cancel/order) endpoints — that scope is deferred to PRD-12.

## Non-goals
- Skills don't carry their own credentials or auth flows.
- Skills don't bypass `ovh` to call OVH directly.

## Skill set (v0.1)

### `ovh-bootstrap`
- **Trigger**: user wants to install/configure `ovh` on a new machine.
- **Inputs**: region (prompt), preferred storage (keyring/file).
- **Steps**: detect or install binary (brew/curl/go install), run `ovh auth login --region <r>`, run `ovh auth status`, write `default.region`.
- **Outputs**: confirmation + first `ovh me` call.

### `ovh-instance-recipe`
- **Trigger**: user wants to provision a curated stack (`dev-box`, `k3s-seed`, `lamp`).
- **Inputs**: project id, recipe name, region, size hint.
- **Steps**: `ovh cloud flavor list` + `ovh cloud image list` to pick the closest match; `ovh cloud instance create` with curated user-data; poll `ovh cloud instance show` until ACTIVE; print SSH command.
- **Outputs**: instance id + SSH hint.

### `ovh-domain-bulk`
- **Trigger**: user has CSV/YAML of DNS records to apply.
- **Inputs**: zone, file, mode (`apply`, `dry-run`).
- **Steps**: parse file; `ovh domain record list <zone> --output json` to compute diff; `ovh domain record add/edit/delete --ensure ...` per row with `--no-prompt`; `ovh domain zone refresh`.
- **Outputs**: per-record status table.

### `ovh-fleet-audit`
- **Trigger**: user wants a current operational inventory across projects.
- **Inputs**: regions (default all configured), output path (default `$XDG_DATA_HOME/ovh/skills/ovh-fleet-audit/`).
- **Steps**: `ovh cloud project list --output json`; per project, `ovh cloud instance list --output json` + `ovh cloud volume list --output json`; aggregate uptime, image, flavor, public IP, status; render markdown table.
- **Outputs**: `fleet-YYYY-MM-DD.md` saved under `$XDG_DATA_HOME/ovh/skills/ovh-fleet-audit/`; symlinked into `$XDG_DATA_HOME/ovh/reports/` per the output convention above.

### `ovh-region-pin`
- **Trigger**: user wants to switch the active region.
- **Inputs**: region.
- **Steps**: `ovh auth status --region <r>`; if not authenticated, run `ovh auth login --region <r>`; then `ovh auth switch --region <r>`.
- **Outputs**: confirmation.

## Skill structure
```
.claude/skills/<name>/
  SKILL.md
  scripts/<task>.sh        # POSIX sh, shebang, shellcheck-clean
```
- All scripts use `--output json` and parse with `jq` (which the user has) or `gojq` if available.
- Scripts always run with `OVH_NO_PROMPT=1` set; if a credential is missing the script returns a clear "run `ovh auth login`" message.
- **Output convention**: skill artifacts are written under `$XDG_DATA_HOME/ovh/skills/<skill-name>/` (where `<skill-name>` is the skill directory name with the `ovh-` prefix retained — e.g. `ovh-fleet-audit`). Reports specifically may also be linked from `$XDG_DATA_HOME/ovh/reports/` for cross-skill discoverability. New skills follow this convention; deviations require an update to this PRD.
- `SKILL.md` follows the project's existing skill template (see `stuffbucket:skill-creator`).

## Open questions
- Do we ship a `ovh-pageant`-style skill that brokers credentials between ssh-agent and OVH? (Lean: out of scope.)
- Should skills declare which `ovh` minimum version they require, and gate on `ovh version`? (Lean: yes.)
