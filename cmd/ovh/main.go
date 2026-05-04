// Command ovh is the OVHcloud CLI/TUI entry point.
//
// See docs/specs/00-overview.md for the project's identity and architecture.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/stuffbucket/ovh-cli/internal/cli"
	"github.com/stuffbucket/ovh-cli/internal/cli/exitcode"
)

func main() {
	os.Exit(run())
}

// run is split out so deferred cleanup (signal handler, etc.) executes before
// process exit. Calling os.Exit inside main would skip those defers.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := cli.Execute(ctx, os.Args[1:]); err != nil {
		// cobra is configured with SilenceErrors=true so it doesn't double-
		// print; main owns the single user-facing error line. Phase 2b adds
		// the structured "next:" hint and JSON-mode error envelope per
		// PRD-01 §Errors.
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return exitcode.From(err)
	}
	return exitcode.OK
}
