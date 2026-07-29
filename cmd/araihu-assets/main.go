package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/araihu/assets/internal/app"
)

func main() {
	ctx, stop := signalContext(context.Background())
	code := runMain(ctx, app.Dependencies{}, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	mainExit(code)
}

var notifyContext = signal.NotifyContext
var mainExit = os.Exit

func signalContext(parent context.Context) (context.Context, context.CancelFunc) {
	return notifyContext(parent, os.Interrupt, syscall.SIGTERM)
}

// runMain is process-free so exit behavior remains directly testable.
func runMain(ctx context.Context, deps app.Dependencies, args []string, stdout, stderr io.Writer) int {
	err := app.Run(ctx, deps, args, stdout, stderr)
	if err == nil {
		return 0
	}
	fmt.Fprintln(stderr, err)
	if app.IsUsage(err) {
		return 2
	}
	return 1
}
