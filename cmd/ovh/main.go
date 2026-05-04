// Command ovh is the OVHcloud CLI/TUI entry point.
//
// See docs/specs/00-overview.md for the project's identity and architecture.
package main

import (
	"context"
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
		return exitcode.From(err)
	}
	return exitcode.OK
}
