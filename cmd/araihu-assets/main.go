package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/araihu/assets/internal/app"
)

func main() {
	os.Exit(runMain(context.Background(), app.Dependencies{}, os.Args[1:], os.Stdout, os.Stderr))
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
