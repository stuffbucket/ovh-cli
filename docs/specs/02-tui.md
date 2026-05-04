---
title: TUI
id: PRD-02
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
`ovh tui` launches a bubbletea-based interactive shell over the same client and config layers as the CLI. Read and write across all v0.1 services, with credential prompts when required.

## Goals
- Same auth/config/output guarantees as the CLI.
- Discoverable: keymap footer always visible, `?` opens full help, `/` filters, `:` opens a command palette.
- Feels like a terminal app, not a website (no mouse-required interactions).
- Full CRUD parity with CLI for v0.1 service set.

## Non-goals
- Not a replacement for the CLI in pipelines.
- No background polling beyond explicit refresh.
- No mouse-only flows (mouse may augment but never replace keys).

## Layout
```
+- ovh . ovh-eu . acme-account ---------- keyring -+
| Services        | Cloud / Projects / abc-123      |
|  Cloud        > |                                 |
|   Projects   >  |  id: abc-123                    |
|   Instances  >  |  name: prod                     |
|   Volumes       |  region: GRA9                   |
|   Networks      |  status: ok                     |
|  Domain         |                                 |
|  Dedicated      |  [r] reboot  [d] delete  [e]edit|
|  VPS            |                                 |
|  IP             |                                 |
| --------------- | ------------------------------- |
| ? help / filter : palette  q quit  [ctrl-r]reload |
+---------------------------------------------------+
```

## Components
- `bubbles/list` for service nav.
- `bubbles/table` for resource lists.
- `bubbles/viewport` for detail pane.
- `huh` for forms (create/edit) and credential prompts.
- `lipgloss` for theming.

## Keymap (defaults)
| Key | Action |
|---|---|
| `up/down` `j/k` | move |
| `right/l` `enter` | drill in |
| `left/h` `esc` | back |
| `r` | resource-specific reboot/refresh (context) |
| `e` | edit |
| `d` | delete (confirm) |
| `n` | new |
| `/` | filter current list |
| `:` | command palette (fuzzy over taxonomy) |
| `?` | full help |
| `q` `ctrl-c` | quit |

## Credential prompts
- Username/password/2FA collected via `huh` `EchoModePassword` for secret fields.
- Never persisted; only the resulting OAuth2 token or consumer key is stored (per auth PRD).
- When `stdin` is not a TTY (impossible from `tui` flow but defensive), refuse and exit code 4.

## State management
- One `Model` struct per pane; root model dispatches messages by active pane.
- All API calls are `tea.Cmd` returning typed result messages.
- Cache: lists pulled once on entry, refreshed on `ctrl-r` or after a mutation.

## Error display
- Inline at the bottom of detail pane, dismissible with `esc`.
- Mirrors CLI error shape (summary + "next step").

## Accessibility
- Respect `NO_COLOR`.
- High-contrast theme variant available via the `default.tui_theme` config key (enum values per the [key registry](04-config-and-storage.md#canonical-key-registry)).
- All actions reachable from keyboard; no required mouse.

## Open questions
- Should the palette accept full CLI commands and shell out, or only TUI-internal verbs? (Lean: TUI-internal in v0.1.)
- Multi-region split-pane in v0.1 or v0.2? (Lean: v0.2.)
