---
title: Phase 2b Sonnet Pseudo-Code Stress Test
phase: 2b (pre-iter-2b)
status: open
date: 2026-05-04
---

## Summary
Per the phase-1 retrospective action item — "every phase gets a pseudo-code stress test before coding starts" — a Sonnet sub-agent walked five end-to-end scenarios spanning all of phase 2b's planned iterations (2b keyring → 2c hosts.yml → 3 OAuth2 → 4 client → 5 cobra). The test surfaced **14 findings**: 7 GAPs, 3 REGISTRY-MISSes, 1 CONFLICT, 2 FRICTIONs, 2 PHASE-2A-DEBTs.

Disposition split: **4 fixed in PRDs now** (iter-2b unblockers), **10 deferred** to the iteration that needs them. The deferral is intentional — fixing all 14 would balloon this commit, and the deferred items have clear "fix before iter X" gates.

## Scenarios walked

1. First-time interactive OAuth2 login on `ovh-eu` with `default.storage=keyring`.
2. Ansible-style `OVH_NO_PROMPT=1` + `OVH_*` env-var auth + `ovh api GET /me --output json`. Includes 401 failure path.
3. `ovh auth refresh --no-prompt` with classic CK creds (must exit 3 with documented next-step) and OAuth2 success path with singleflight.
4. TUI mid-session 401 with **two** in-flight `tea.Cmd`s; singleflight should coalesce; root model swaps client only via `Update`.
5. `ovh auth setup-token --ak X --as Y --ck Z --region ovh-eu --no-prompt` non-interactive bootstrap; missing-flag path.

## Findings

### Iter 2b blockers (FIX-NOW)

**GAP-1: keyring placeholder grammar.** PRD-04 §ovh.conf schema showed `keyring:ovh-eu:application_secret` as an example but didn't formalize the grammar (separator, fields, profile encoding). → **Fixed in PRD-04** §Keyring placeholder grammar.

**GAP-2: keyring service/user names.** `zalando/go-keyring` requires `(service, user, password)`; no PRD specified what to put there. → **Fixed in PRD-04** as `service="ovh-cli"`, user `<region>:<profile>:<key>`.

**PHASE-2A-DEBT-1: `LoadCredentials` has no storage param.** Iter 2a's `LoadCredentials(ctx, region, profile)` cannot read `default.storage` to know whether to resolve `keyring:` placeholders. Cycle rule prevents importing `internal/config`. → **Fixed in PRD-03** §store.go: `LoadCredentials` reads `[default].storage` directly from the same `ovh.conf` it parses for credentials. No signature change needed.

**PHASE-2A-DEBT-2: `DeleteCredentials` ignores profile.** Iter 2a deletes the entire `[region]` section regardless of profile; iter 2b's keyring path keys by profile and must not delete another profile's secret. → **Fixed in PRD-03** §DeleteCredentials: explicit per-profile semantics for iter 2b.

### Iter 2c blocker (DEFER until iter 2c starts)

**GAP-3: `hosts.yml` YAML schema.** PRD-03 / PRD-04 name `hosts.yml` as "OAuth2 token state" but never document the schema (field names, nesting, version key). Iter 2c cannot proceed without it.

**Action**: when iter 2c starts, write a schema spec into PRD-04 §File layout (or new §hosts.yml schema). Suggested shape:
```yaml
version: 1
hosts:
  <region>:
    profiles:
      <profile>:
        access_token: ...
        refresh_token: ...
        expiry: <RFC3339>
        scopes: [...]
        last_validated_at: <RFC3339>
```

### Iter 3 blockers (DEFER until iter 3 starts)

**GAP-5: `internal/auth/flow.go RunConsumerKeyFlow` symbol.** PRD-03 §Classic CK flow describes the steps but no exported entry point is in the package registry. Scenario 3's interactive recovery path needs something to call.

**GAP-7: `ExchangePKCE` token endpoint resolution.** `Region.OAuth2Issuer` is the issuer URL but the token endpoint is a separate URL. Either OIDC discovery (`<issuer>/.well-known/openid-configuration`) or hard-coded `<issuer>/token` — needs to be specified.

**REGISTRY-MISS-1: `internal/auth/flow.go` exported symbols.** Same as GAP-5; PRD-06 package registry row for `internal/auth` doesn't list `RunConsumerKeyFlow`.

**Action when iter 3 starts**: add `RunConsumerKeyFlow(ctx, Region, profile, askAKAS func() (ak, as string, err error)) (Credentials, error)` signature to PRD-03; add the symbol to PRD-06 package registry; resolve PKCE token endpoint via verification against OVH's actual OAuth2 deployment.

### Iter 4 blocker (DEFER until iter 4 starts)

**REGISTRY-MISS-3: `meResponse` type.** Scenarios 1, 2, 5 all post-validate via `GET /me` and read `me.Nichandle`. No `Me` type is defined in `internal/client/types/` or `internal/client/paths/`.

**Action when iter 4 starts**: hand-write a minimal `Me` struct in `internal/client/types/me.go` (`Nichandle`, `Email`, `FirstName`, `Name`); add to PRD-06 package registry. `tools/schemagen` will regenerate from `apispace.json` later.

**CONFLICT-1: `RefreshOAuth2` and RO config keys.** PRD-03 §RefreshOAuth2 says "StoreCredentials has been called before return" but doesn't say whether RO keys (`last_validated_at`, `token_expires_at`) are also written. PRD-06 cycle rule says auth must not import config — so the answer is "caller writes RO keys, auth doesn't."

**Action when iter 4 starts**: amend PRD-03 §RefreshOAuth2 contract to state explicitly "RO config-key writes are the caller's responsibility; auth must not import internal/config."

### Iter 5 blockers (DEFER until iter 5 starts)

**GAP-4: TUI `authErrorMsg` / retry closure shape.** PRD-06 §Mid-session credential rotation describes the message-passing flow but doesn't specify the closure pattern. Scenario 4 invented a `retry func(*client.Client) tea.Cmd` field on the message.

**GAP-6: `OVH_NO_PROMPT` threading.** Read from env in every RunE, or read once into `*cli.Deps`? Scenarios 3 and 5 assumed env re-read — inconsistent with the Deps pattern.

**REGISTRY-MISS-2: `*cli.Deps` field enumeration.** PRD-06 §Composition root references Deps but the table only says "Deps, root cobra command tree." Scenarios invented `deps.Config`, `deps.Client`, `deps.Renderer`, `deps.Region`, etc.

**FRICTION-1: Two `tokenRefreshMsg`s for one refresh.** Without a `refreshInFlight` guard in the root model, two simultaneous 401s in the TUI dispatch two `refreshTokenCmd`s; singleflight collapses the RPC but `Update` still issues two retries.

**FRICTION-2: RO config keys written from internal callers.** `ovh config set` rejects RO keys (PRD-04). Login/setup-token need to write `<region>.account`, `<region>.last_validated_at`, etc. There's no documented escape hatch.

**Action when iter 5 starts**:
- PRD-06 §Mid-session: full message struct definitions (`authErrorMsg`, `tokenRefreshMsg`, `refreshInFlight bool`, `pendingRetries []func(*client.Client) tea.Cmd`).
- PRD-06 §Composition root: full `Deps` field table (`Config`, `Client`, `Renderer`, `Schema`, `Region`, `Profile`, `NoPrompt`, `Debug`).
- PRD-06 / PRD-04: clarify that global flag values are read **once** at composition-root build, never re-read in RunE.
- PRD-04 §Canonical key registry: add note that RO keys are written by internal callers via `config.SetInternal(key, value)` (separate from `Set`); test asserts `Set` rejects RO and `SetInternal` accepts.

## Tracking

When each iteration starts, the implementer **must** address its deferred findings as the first commit of that iteration (PRD update + retro entry update). The retro entry is closed when the last finding for the iteration is implemented.

| Iteration | Deferred findings |
|---|---|
| 2b (keyring) | (all blockers fixed in this commit) |
| 2c (hosts.yml) | GAP-3 |
| 3 (OAuth2) | GAP-5, GAP-7, REGISTRY-MISS-1 |
| 4 (client) | REGISTRY-MISS-3, CONFLICT-1 |
| 5 (cobra wiring) | GAP-4, GAP-6, REGISTRY-MISS-2, FRICTION-1, FRICTION-2 |
