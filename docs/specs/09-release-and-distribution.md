---
title: Release & Distribution
id: PRD-09
status: draft
owner: stuffbucket
version: 0.1
last-updated: 2026-05-02
---

## Summary
`ovh` ships as a single static binary via Homebrew tap, GitHub Releases, .deb and .rpm packages. Releases are reproducible, signed, and SBOM-attested. Tap publish follows stuffbucket's standard remote workflow.

## Goals
- One `brew install stuffbucket/tap/ovh` command on macOS and Linux.
- Cross-platform binaries: darwin/{arm64,amd64}, linux/{arm64,amd64}, windows/amd64.
- Reproducible builds; signed artifacts; SBOM + provenance attached.
- Same git identity convention as other stuffbucket repos.

## Non-goals
- Self-update mechanism in v0.1.
- Hosting our own apt/yum repo (we attach .deb/.rpm to GitHub Releases; users add via direct URL).

## Versioning
- Semver. v0.1.x while behavior may shift; v1.0 once command JSON shape is frozen.
- `ovh version` includes commit hash, build date, Go version (`-buildvcs=true`).

## goreleaser configuration (highlights)
```yaml
project_name: ovh
builds:
  - main: ./cmd/ovh
    flags: [-trimpath]
    ldflags: [-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}]
    goos: [darwin, linux, windows]
    goarch: [amd64, arm64]
archives:
  - format_overrides:
      - goos: windows
        format: zip
sboms:
  - artifacts: archive
    documents: ["{{ .ArtifactName }}.cdx.json"]
signs:
  - cmd: cosign
    artifacts: all
    args: ["sign-blob", "--yes", "--output-signature=${signature}", "${artifact}"]
nfpms:
  - vendor: stuffbucket
    homepage: https://github.com/stuffbucket/ovh-cli
    license: MIT
    formats: [deb, rpm]
brews:
  - repository:
      owner: stuffbucket
      name: homebrew-tap
    homepage: https://github.com/stuffbucket/ovh-cli
    description: "OVHcloud CLI and TUI"
    license: MIT
    install: |
      bin.install "ovh"
      generate_completions_from_executable(bin/"ovh", "completion")
      man1.install Dir["man/man1/*.1"]
    test: |
      assert_match "ovh", shell_output("#{bin}/ovh version")
release:
  github:
    owner: stuffbucket
    name: ovh-cli
  prerelease: auto
```

## Homebrew formula (recommended shape)
- `class Ovh < Formula`, `desc`, `homepage`, `url` (versioned tarball), `sha256`, `license "MIT"`.
- For source builds: `depends_on "go" => :build` + `system "go", "build", ...` + `bin.install "ovh"`.
- For prebuilt bottles (preferred when goreleaser produces them): no Go dep at runtime.
- Completions: `generate_completions_from_executable(bin/"ovh", "completion")` covers bash/zsh/fish.
- Man page: `man1.install` against goreleaser-generated `man/man1/ovh.1`.
- `test do` block runs `ovh version` and exercises a no-network command.
- Lints clean against `brew audit --strict --new` before tap PR.

## Tap publish workflow
- On tag `vX.Y.Z`, GitHub Actions workflow `release.yml` runs goreleaser.
- goreleaser pushes the formula PR (or commits directly, mirroring other stuffbucket projects) to `github.com/stuffbucket/homebrew-tap` using a deploy token in repo secret `HOMEBREW_TAP_TOKEN`.
- Commit identity for tap commits: same as other stuffbucket repos — `user.name=stuffbucket`, `user.email=<the noreply address used in sibling repos>` (resolved at first release by inspecting an existing stuffbucket repo's `git log`; recorded here once confirmed). Repo-local config only.
- No global git config changes ever.

## Release checklist
1. Tag prep PR updates `CHANGELOG.md`, bumps any version constant.
2. CI green: build · test · vet · lint · govulncheck · gosec · gitleaks · license check · vendor drift.
3. Tag push (`git tag vX.Y.Z && git push --tags`) triggers `release.yml`.
4. Workflow:
   - goreleaser build (reproducible).
   - cosign sign all artifacts.
   - syft SBOM attached.
   - SLSA provenance attestation attached.
   - Homebrew tap PR/commit pushed.
   - GitHub Release published.
5. Post-release: `brew upgrade stuffbucket/tap/ovh` on a clean macOS arm64 box and run smoke tests.

## Distribution surfaces
| Surface | How users install | Verification |
|---|---|---|
| Homebrew | `brew install stuffbucket/tap/ovh` | sha256 in formula |
| GitHub Release tarball | curl tarball + checksums | cosign verify-blob |
| Debian | `dpkg -i ovh_X.Y.Z_amd64.deb` | sha256 in checksums.txt |
| RPM | `rpm -i ovh-X.Y.Z.amd64.rpm` | sha256 in checksums.txt |
| `go install` | `go install github.com/stuffbucket/ovh-cli/cmd/ovh@latest` | go module proxy + sumdb |

## Open questions
- Do we publish a Docker image? (Lean: yes, scratch-based, signed; v0.1 nice-to-have.)
- Apt/yum repo via GitHub Pages? (Lean: v0.2.)
