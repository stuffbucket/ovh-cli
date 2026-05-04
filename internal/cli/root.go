// Package cli is the composition root: it wires Layer B packages together and
// hands a *Deps bundle to cobra commands. See PRD-06 §Composition root.
//
// Phase 1 contains only the cobra skeleton: root command, global flag
// declarations, and the version/completion subcommands. The Deps wiring
// (config, auth, client, schema, output) lands in phase 2.
package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/stuffbucket/ovh-cli/internal/cli/completion"
	"github.com/stuffbucket/ovh-cli/internal/cli/schema"
	"github.com/stuffbucket/ovh-cli/internal/cli/version"
)

// Execute runs the ovh CLI with the given args.
func Execute(ctx context.Context, args []string) error {
	cmd := newRootCmd()
	cmd.SetArgs(args)
	return cmd.ExecuteContext(ctx)
}

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ovh",
		Short:         "OVHcloud CLI and TUI",
		Long:          "ovh — operate OVHcloud services from the terminal.\nSee https://github.com/stuffbucket/ovh-cli for documentation.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	addGlobalFlags(cmd)
	cmd.AddCommand(version.NewCmd())
	cmd.AddCommand(completion.NewCmd())
	cmd.AddCommand(schema.NewCmd())
	return cmd
}

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
