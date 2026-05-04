package cli

import "github.com/spf13/cobra"

// addGlobalFlags declares the persistent flags listed in PRD-01 §Global flags.
//
// In phase 1 the flag values are not yet read; phase 2 introduces deps.go,
// which reads them along with OVH_* env vars (per PRD-04 §Canonical env-var
// registry) and the active config file.
func addGlobalFlags(cmd *cobra.Command) {
	f := cmd.PersistentFlags()
	f.String("region", "", "OVH endpoint id (default from default.region; see PRD-04)")
	f.String("profile", "default", "credential profile within the region")
	f.StringP("output", "o", "", "output format: table|json|yaml|tsv|template=<expr>")
	f.String("jq", "", "filter JSON output through gojq (mirrors gh)")
	f.Bool("no-color", false, "disable ANSI color (also honors NO_COLOR per no-color.org)")
	f.Bool("no-prompt", false, "disable every interactive prompt (OVH_NO_PROMPT=1)")
	f.Bool("debug", false, "slog debug to stderr (secrets redacted per PRD-08)")
	f.String("config", "", "path to ovh.conf (overrides discovery; OVH_CONFIG_FILE)")
	f.Duration("timeout", 0, "client request timeout (e.g. 30s, 1m); default.timeout")
}
