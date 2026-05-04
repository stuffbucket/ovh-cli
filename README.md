# ovh

OVHcloud CLI and TUI — a single static binary that wraps the OVHcloud API for
day-to-day operational jobs (cloud, domain, dedicated, VPS, IP, storage).

UX patterned on `gh` and `az`. Distributed via the
[stuffbucket/homebrew-tap](https://github.com/stuffbucket/homebrew-tap).

## Status

**Phase 1 — skeleton.** Only `ovh --help`, `ovh version`, and
`ovh completion <shell>` are wired. Auth, schema, and resource commands land
in subsequent phases.

## Documentation

The full specification lives under [`docs/specs/`](docs/specs/) — start at
[`00-overview.md`](docs/specs/00-overview.md). Cross-cutting symbols
(config keys, env vars, internal packages, exit codes, error codes, file
modes, redaction rules, depguard rules) live in canonical single-owner
sections enumerated by the citation pattern in PRD-00.

## License

MIT — see [LICENSE](LICENSE).
