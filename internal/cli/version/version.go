// Package version exposes build metadata via `ovh version`.
//
// Build-time values are injected by goreleaser via -ldflags (PRD-09); falling
// back to runtime/debug.BuildInfo keeps `go run` and `go install` informative.
//
// The package-level vars are unexported so other packages cannot mutate them
// in tests; the linker -X flag still resolves identifiers regardless of export.
package version

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0-dev"
	commit  = "unknown"
	date    = "unknown"
)

// NewCmd returns the cobra command for `ovh version`.
func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit, and build date",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			v, c, d := resolve()
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "ovh %s (commit %s, built %s)\n", v, c, d)
			return err
		},
	}
}

func resolve() (v, c, d string) {
	v, c, d = version, commit, date
	if c != "unknown" && d != "unknown" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if c == "unknown" {
				c = s.Value
			}
		case "vcs.time":
			if d == "unknown" {
				d = s.Value
			}
		}
	}
	return
}
