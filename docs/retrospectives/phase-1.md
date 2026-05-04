---
title: Phase 1 Retrospective
phase: 1
status: closed
opened: 2026-05-02
closed: 2026-05-04
last-commit: 1d34b86
---

## Outcome
Phase 1 delivered the cobra skeleton with all PRD-11 acceptance criteria green. CI is green on `main` at `1d34b86`; the repo is live at <https://github.com/stuffbucket/ovh-cli>.

| Acceptance criterion | Status |
|---|---|
| `ovh --help` / `version` / `completion <shell>` | ✅ |
| `go build ./...` clean | ✅ |
| CI: build, test, vet, gofmt, golangci-lint, govulncheck, license check, vendor drift, gitleaks | ✅ all six jobs green |
| Dependabot configured | ✅ |
| `docs/security/SECURITY.md` published | ✅ |
| `THIRD_PARTY_LICENSES.md` generated | ✅ + `hack/licenses.tpl` for CI regeneration |
| Repo-local git identity per stuffbucket convention | ✅ verified against 9 sibling repos |
| First commit pushed | ✅ 7 commits, all pushed |
| Vendoring committed | ✅ `vendor/` (≈120 files) |

## What went well

- **The PRD set behaved like a compiler.** Six boundary-review rounds + one Sonnet pseudo-code stress test + one Dijkstra-style critique left phase-1 implementation with no naming or structural surprises. Every file location, package name, and cobra command placement was already pinned in a registry; coding it was mechanical.
- **Single-owner registries paid off immediately.** Implementing `internal/cli/exitcode` was a straight copy from PRD-01 §Canonical exit-code registry — no interpretation, no judgment calls.
- **Identity discovery was 30 seconds.** Scraping sibling stuffbucket repos for the noreply email pattern returned a unanimous result (`231133237+stuffbucket@users.noreply.github.com`) across 9 repos.
- **The `run() int` pattern caught early.** gocritic's `exitAfterDefer` lint flagged `os.Exit` in `main()` skipping `defer stop()` from `signal.NotifyContext`. The fix is the standard idiom and is now established before any complex defer chains exist.
- **Pinning beats latest.** Pinning `golangci-lint` to `v2.11.3` (matched to local) beat `version: v2.1` (which the action interpreted as "any v2.1.x" and got a build with go1.24, incompatible with our 1.26 module).

## What didn't go well

- **`golangci-lint` `depguard` file negation didn't behave as documented.** The pattern `!**/internal/xdgpaths/**` in the `files:` field was supposed to mean "apply this rule to files NOT in xdgpaths". Local testing showed the rule still fired on `internal/xdgpaths/paths.go`. I deferred the full PRD-08 depguard ruleset to phase 2a/2b (when target packages exist), keeping only `forbidigo` for now. **Risk**: if not addressed before phase-2 packages merge, the layer rules won't be mechanically enforced. Action item below.
- **`gofmt -l .` walked `vendor/`.** Flagged upstream pflag formatting. Easy fix (`gofmt -l $(go list -f '{{.Dir}}' ./...)`); a beginner mistake.
- **Required two CI fixup commits to reach green.** `gofmt vendor`, then `golangci-lint-action@v6 -> v7`, then `lint version v2.1 -> v2.11.3`. None of these were code defects; all were CI config friction. Acceptable for first-time setup but worth the time-to-green entry.
- **`THIRD_PARTY_LICENSES.md` from `go-licenses` is minimal (4 entries).** `go-licenses` traces from `cmd/ovh`, which doesn't yet pull in `internal/xdgpaths` transitively. Will fill out as phase 2 wires xdgpaths into cobra deps.

## What we learned

- **PRD review rounds find different defect classes, not diminishing returns.** Round 1 caught 10 issues; round 6 caught 10. The marginal cost is roughly constant, but each round drove out a different *kind* of defect (cross-PRD names → structural anchors → implementation invariants → claims-without-checks). We probably stopped at the right number.
- **A pseudo-code stress test catches scenario-execution defects, not structural ones.** The Sonnet round added 7 findings the architect agent had not surfaced — symbols-required-but-never-named (`auth.StoreCredentials`, `auth.ValidateHost`, `RefreshOAuth2`), TUI mid-session lifetime questions, and the `LoadCredentials` env-vs-keyring precedence delegation. These were invisible to the boundary review because they only emerge when you actually try to write the code.
- **The Dijkstra pass is a "wishes vs guarantees" filter.** It flagged every claim ("never call X", "atomic", "redact") and asked whether it was enforceable. About half needed concrete verification mechanisms attached — mostly via PRD-10 §Property / contract tests.

## Process changes adopted mid-phase

- **Canonical registries pattern.** Three (then five, then nine) single-owner sections own cross-cutting symbols. Other PRDs cite, never redefine. Eliminated bridge-rot drift class entirely after round 4.
- **Five PRD reviews are the floor, not the ceiling.** Shape of findings only stabilizes around rounds 3–4.
- **Stress-test before implementation.** Going forward: every phase gets a pseudo-code stress test before coding starts.

## Action items for phase 2

1. **Re-attempt the full `depguard` ruleset** with explicit include-lists (instead of glob negation) when phase 2a brings `internal/schema` online and phase 2b brings `internal/{auth,client,config}`. The Layer A allowlist for `internal/schema` is the single most important rule.
2. **Translate PRD-10 §Property / contract tests into code** as soon as the corresponding packages exist — atomic write, refuse-if-looser, redaction, retry bound, singleflight refresh, secret zeroing.
3. **Run a Sonnet pseudo-code stress test for phase 2** before coding starts, focused on: schema refresh end-to-end including ETag round-trip; `OpenCache` `(Querier, ErrNoCache)` semantics; `auth.LoadCredentials` env-vs-keyring branching; `RefreshOAuth2` singleflight in a contended TUI session.
4. **Resolve `errors.AsType[Coded]`** (Go 1.26 generic version) once `internal/cli/exitcode` gets test coverage.
5. **Update `actions/checkout@v4`/`setup-go@v5`/`gitleaks-action@v2` to Node-24 versions** before Sept 16 2026 deprecation.

## Numbers

- **PRDs**: 13 (PRD-00 + 12 specs)
- **Review rounds**: 6 boundary + 1 Sonnet stress test + 1 Dijkstra
- **Findings closed across all rounds**: ~70
- **Canonical sections**: 9 (5 primary registries + 3 secondary + depguard ruleset)
- **Phase-1 source**: 7 Go files + 5 config files + 6 docs/foundation files
- **Vendor**: 4 direct deps, ≈120 files
- **Commits on `main`**: 7
- **Time to CI green**: 3 fixup commits after the initial CI wiring
