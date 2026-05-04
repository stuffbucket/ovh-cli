# Third-party licenses

The `ovh` binary statically links the Go modules listed below. Each module's
license is allowlisted by the policy in PRD-08 §License policy
(MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, ISC, MPL-2.0).

This file is regenerated on every release by the CI pipeline:

```
go-licenses report ./... --template=hack/licenses.tpl > THIRD_PARTY_LICENSES.md
```

## Direct and transitive Go dependencies

- **github.com/adrg/xdg** — MIT — <https://github.com/adrg/xdg/blob/HEAD/LICENSE>
- **github.com/spf13/cobra** — Apache-2.0 — <https://github.com/spf13/cobra/blob/HEAD/LICENSE.txt>
- **github.com/spf13/pflag** — BSD-3-Clause — <https://github.com/spf13/pflag/blob/HEAD/LICENSE>

Transitive Windows-only modules (e.g. `github.com/inconshreveable/mousetrap`,
`golang.org/x/sys`) are excluded by `go-licenses report` when running on
non-Windows hosts and are listed in the SBOM attached to GitHub releases.

## License of this project

`github.com/stuffbucket/ovh-cli` is MIT-licensed; see [LICENSE](LICENSE).
