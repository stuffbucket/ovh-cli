package schema

import (
	"github.com/spf13/cobra"

	pkgschema "github.com/stuffbucket/ovh-cli/internal/schema"
)

func newRefreshCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Re-fetch the OVH API schema for a region",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			region := resolveRegionFlag(cmd)
			rc, err := resolveRegion(region)
			if err != nil {
				return err
			}
			return pkgschema.Refresh(cmd.Context(), region, rc.Endpoint, rc.AllowedHosts)
		},
	}
}
