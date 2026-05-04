// Package version exposes build metadata via `ovh version`.
//
// Build-time values are injected by goreleaser via -ldflags (PRD-09); falling
// back to runtime/debug.BuildInfo keeps `go run` and `go install` informative.
package version

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var (
	// Version is the semver string; overwritten via -ldflags at release time.
	Version = "0.1.0-dev"
	// Commit is the git commit hash; overwritten via -ldflags at release time.
	Commit = "unknown"
	// Date is the build date; overwritten via -ldflags at release time.
	Date = "unknown"
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
	v, c, d = Version, Commit, Date
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
