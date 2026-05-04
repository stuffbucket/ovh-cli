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

	return cmd
}
