// Package schema provides the `ovh schema` cobra subcommand tree:
// refresh, show, path, diff. PRD-01 §Command taxonomy and PRD-05.
package schema

import "github.com/spf13/cobra"

// NewCmd returns the root `ovh schema` command with its subcommands attached.
func NewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schema",
		Short: "Inspect and refresh the cached OVH API schema",
		Long: `Inspect and refresh the cached OVH API schema (apispace.json).

The cache lives at $XDG_CACHE_HOME/ovh/schema/<region>/. Run 'refresh' to
populate or update; 'show', 'path', and 'diff' are read-only.`,
	}
	cmd.AddCommand(newRefreshCmd(), newShowCmd(), newPathCmd(), newDiffCmd())
	return cmd
}

// resolveRegionFlag reads the inherited --region flag, falling back to the
// PRD-04 default. Phase 2b will read default.region from the active config
// (per PRD-04 §Read precedence) instead of hard-coding the fallback.
func resolveRegionFlag(cmd *cobra.Command) string {
	if r, _ := cmd.InheritedFlags().GetString("region"); r != "" {
		return r
	}
	return "ovh-eu"
}
