---
title: Testing & Quality
id: PRD-10
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
Tests are layered: pure unit tests for libraries, golden-file tests for output and config rendering, fake-server integration tests for the client, schema-replay tests for Layer A, end-to-end smoke tests for release builds. CI gates every PR on the full suite plus the security scanners (PRD-08).

## Goals
- High confidence in the auth and config layers — they touch security and break user trust if wrong.
- Output tests catch JSON-shape regressions before they hit Ansible users.
- Schema-replay tests run offline using committed fixtures so CI is hermetic.

## Non-goals
- Hitting real OVH endpoints in CI (manual smoke tests only, on release).
- 100% coverage targets — coverage chased blindly produces low-value tests.

## Test layers

### Unit
- Pure-function tests under each package's `*_test.go`.
- `internal/config/schema_test.go` — every key validates correctly, RO keys reject `set`.
- `internal/auth/regions_test.go` — every region declared has every required field; the registry literal in `regions.go` and the `validation_host_pattern` table in PRD-03 are 1:1 by region id; no entry has an empty `ValidationHostPattern` slice.
- `internal/cli/output/*` — table/JSON/template rendering.

### Property / contract tests (assert PRD claims mechanically)
Every "claim" in the PRDs that asserts a system property must have a corresponding test below. Adding a new asserted property to any PRD requires adding a row here.

| Claim | Source | Test |
|---|---|---|
| **Atomic write** | PRD-04 §Atomic writes | `internal/config/atomic_test.go` writes via the renameio path while a parallel goroutine `os.ReadDir`s the parent; asserts no intermediate filename ever appears (only the final target name or nothing). |
| **Refuse-if-looser** | PRD-04 §Canonical file-mode registry | Table-driven test in `internal/auth/store_test.go` and `internal/config/config_test.go`: for each `Refuse-if-looser=yes` row, chmod the target one bit looser than the registry value; assert the loader returns `auth.ErrFileModeUnsafe` (or its config equivalent). |
| **File-mode-on-creation** | PRD-04 §Canonical file-mode registry | `auth.StoreCredentials` against a nonexistent `$XDG_CONFIG_HOME/ovh/` creates the directory at exactly 0700 and the file at 0600. |
| **Redaction applied** | PRD-08 §Canonical redaction allowlist | `internal/cli/output/redact_test.go` feeds every field/header/pattern through the slog handler and the formatter; asserts each emitted line matches the stated mask. Boundary cases for the token-shape regex: 31-char hex (no match), 32-char hex (match), 32-char non-hex (no match). |
| **Pagination terminates** | PRD-06 §Pagination | Fake-server test with a cyclic ID list; assert `client.PaginateIDs` channel closes within `default.timeout` and surfaces `ctx.Err()` rather than silent termination. |
| **Retry bounded** | PRD-06 §Retry policy | Mock transport with an atomic counter returns permanent 5xx; assert `client.Do` invokes the transport exactly 4 times (not "≤4"). |
| **Singleflight refresh** | PRD-03 §RefreshOAuth2 | Spawn 10 concurrent `RefreshOAuth2` calls against a fake issuer; assert the issuer received exactly 1 token-exchange RPC and all 10 returned the same `Credentials`. |
| **Secret zeroing** | PRD-08 §Secret prompts | After `WithSecret(b, fn)` returns, `bytes.Equal(b, make([]byte, len(b)))`. |
| **Layer A no Layer B imports** | PRD-05 §Module layout | `golangci-lint` `depguard` rule denies any `<module>/internal/{cli,auth,client,config}` import from `internal/schema`; CI fails if introduced. |
| **No back-edge auth → client** | PRD-06 §Composition root | `depguard` rule denies any `<module>/internal/client` import from `internal/auth`. |
| **Non-Credentials reference from client → auth** | PRD-06 §Canonical package registry | `forbidigo` rule denies references like `auth.LoadCredentials`, `auth.StoreCredentials`, `auth.RefreshOAuth2` from `internal/client/*`. |

### Golden files
- Path convention: `testdata/golden/<command-path>/<scenario>.{json,txt}` where `<command-path>` mirrors the cobra command tree with `_` replacing spaces (e.g. `testdata/golden/auth_status/default.json`, `testdata/golden/cloud_instance_list/empty.json`). Diagnostic commands follow the same pattern.
- `go test -update` regenerates; PRs review the diff like code.
- Used for: every `--output json` shape, every help text, error message templates, config-list output.

### Fake-server integration
- `internal/client/clienttest/server.go` — `httptest.Server` that loads request/response pairs from YAML fixtures.
- Asserts request signing headers (`X-Ovh-Application`, `X-Ovh-Signature`, `X-Ovh-Timestamp`) are present and well-formed without trying to verify signatures cryptographically (that's go-ovh's job).
- Each typed command has at least one fake-server test covering happy-path + one error code.

### Schema replay
- `internal/schema/testdata/<region>/1.0.json` and per-service fixtures committed.
- `internal/schema/fetch_test.go` runs the full fetch against a `httptest.Server` that serves the fixtures.
- `apispace.json` golden output checked in.
- Refresh of fixtures: `make schema-fixtures-refresh` (manual, anonymized).

### TUI
- `bubbletea` `teatest.NewTestModel` to drive the model with synthetic key events.
- Snapshot the rendered string against goldens.

### End-to-end smoke
- Run on release tag against a real `ovh-eu` test account secret-injected from GitHub secrets.
- Smoke matrix: `auth login --method consumer-key` (with pre-provisioned AK/AS), `me`, `cloud project list`, `domain list`, `api GET /me`, `version`.
- Failure blocks the release.

## Quality gates (CI)
| Gate | Tool |
|---|---|
| Build | `go build ./...` |
| Test | `go test -race -coverprofile=coverage.out ./...` |
| Vet | `go vet ./...` |
| Lint | `golangci-lint run` |
| Format | `gofmt -l .` (must be empty) |
| Vulns | `govulncheck ./...` |
| Static security | `gosec` (via golangci-lint) |
| Licenses | `go-licenses report` against allowlist |
| Vendor drift | `go mod tidy && go mod vendor && git diff --exit-code -- vendor/ go.mod go.sum` |
| Schemagen drift (active **from phase 2a**) | `go generate ./... && git diff --exit-code -- internal/client/types/`; in phase 1 the gate is satisfied by a stub `internal/client/types/doc.go` containing only `// Code generated; replaced in phase 2a.` and no `//go:generate` directives anywhere. |
| Secrets | `gitleaks detect --redact` |
| Schema | `go test ./internal/schema/...` (golden) |
| TUI | `go test ./internal/cli/tui/...` (teatest goldens) |
| Docs | `cobra-cli generate docs` clean diff |

## Coverage
- Track coverage in CI; surface trend via Codecov or stub.
- Targets (soft): `internal/auth` >=85%, `internal/config` >=85%, `internal/client` >=75%, others >=60%.

## Test data hygiene
- All committed fixtures redact tokens to `****`.
- `gitleaks` config allowlists known fixture paths.
- `make audit-fixtures` greps for known-bad patterns (real-looking tokens) before commit.

## Open questions
- Mutation testing (`gremlins`) on auth + config? (Lean: v0.2.)
- Property-based tests for path/method validation against `apispace.json`? (Lean: yes, lightweight, gopter.)
