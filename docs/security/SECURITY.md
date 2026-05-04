# Security policy

## Reporting a vulnerability

Please do **not** file a public GitHub issue for security-sensitive reports.
Use GitHub's private security advisory channel instead:

<https://github.com/stuffbucket/ovh-cli/security/advisories/new>

## Embargo and disclosure

We aim to acknowledge reports within 5 business days and ship a fix within
90 days, whichever comes first. Coordinated disclosure is encouraged; the
project requests and assigns a CVE for confirmed vulnerabilities.

## Supported versions

Only the latest minor release receives security fixes. Older releases are
documented as superseded.

## Supply-chain controls

PRD-08 (`docs/specs/08-security-and-supply-chain.md`) carries the full
dependency classification, hardening rules, and CI gating policy.
Highlights:

- Releases are signed with **cosign** (keyless via Sigstore).
- **SBOMs** (CycloneDX via syft) and **SLSA-3 provenance** attestations are
  attached to every GitHub release.
- The `depguard` ruleset confining sensitive third-party imports to specific
  packages lives in PRD-08 §Canonical depguard ruleset.
- The redaction allowlist for the structured logger lives in PRD-08
  §Canonical redaction allowlist; a CI gate asserts no committed golden
  fixture violates the rules.
- File modes for credential-bearing files are pinned in PRD-04 §Canonical
  file-mode registry; loaders refuse to read on looser modes.
