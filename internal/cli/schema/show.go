package schema

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	pkgschema "github.com/stuffbucket/ovh-cli/internal/schema"
)

func newShowCmd() *cobra.Command {
	var method string
	cmd := &cobra.Command{
		Use:   "show <path>",
		Short: "Show the methods declared on an API path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			region := resolveRegionFlag(cmd)
			q, err := pkgschema.OpenCache(region)
			if err != nil {
				return fmt.Errorf("open cache for %s: %w", region, err)
			}
			ps, ok := q.Describe(args[0])
			if !ok {
				return fmt.Errorf("path %q not found in region %s (try `ovh schema path` to search)", args[0], region)
			}
			out := cmd.OutOrStdout()
			for _, m := range sortedMethods(ps) {
				if method != "" && !strings.EqualFold(m, method) {
					continue
				}
				spec := ps[m]
				fmt.Fprintf(out, "%s %s\n  %s\n", m, args[0], spec.Description)
				if len(spec.Auth) > 0 {
					fmt.Fprintf(out, "  auth: %s\n", strings.Join(spec.Auth, ", "))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&method, "method", "", "filter to one HTTP method (GET, POST, ...)")
	return cmd
}

func sortedMethods(ps pkgschema.PathSpec) []string {
	out := make([]string, 0, len(ps))
	for m := range ps {
		out = append(out, m)
	}
	sort.Strings(out)
	return out
}
