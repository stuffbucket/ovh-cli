package schema

import (
	"fmt"

	"github.com/spf13/cobra"

	pkgschema "github.com/stuffbucket/ovh-cli/internal/schema"
)

func newDiffCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "diff <region-a> <region-b>",
		Short: "Show capability differences between two cached regions",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := pkgschema.OpenCache(args[0])
			if err != nil {
				return fmt.Errorf("open %s: %w", args[0], err)
			}
			b, err := pkgschema.OpenCache(args[1])
			if err != nil {
				return fmt.Errorf("open %s: %w", args[1], err)
			}
			d := pkgschema.Compare(a, b)
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Only in %s (%d):\n", args[0], len(d.OnlyInA))
			for _, p := range d.OnlyInA {
				fmt.Fprintln(out, "  "+p)
			}
			fmt.Fprintf(out, "Only in %s (%d):\n", args[1], len(d.OnlyInB))
			for _, p := range d.OnlyInB {
				fmt.Fprintln(out, "  "+p)
			}
			fmt.Fprintf(out, "Common: %d paths\n", len(d.Common))
			return nil
		},
	}
}
