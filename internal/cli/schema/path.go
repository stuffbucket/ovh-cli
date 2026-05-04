package schema

import (
	"fmt"

	"github.com/spf13/cobra"

	pkgschema "github.com/stuffbucket/ovh-cli/internal/schema"
)

func newPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path <prefix>",
		Short: "Search API paths by prefix",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			region := resolveRegionFlag(cmd)
			q, err := pkgschema.OpenCache(region)
			if err != nil {
				return fmt.Errorf("open cache for %s: %w", region, err)
			}
			for _, p := range q.Search(args[0]) {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
}
