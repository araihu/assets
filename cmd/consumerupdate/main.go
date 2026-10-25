// consumerupdate prepares an allowlisted consumer patch from an immutable release.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"github.com/araihu/assets/internal/consumerupdate"
)

func run(ctx context.Context, args []string, out io.Writer) error {
	flags := flag.NewFlagSet("consumerupdate", flag.ContinueOnError)
	manifestPath := flags.String("manifest", "araihu-assets.json", "consumer-owned mapping manifest")
	eventPath := flags.String("event-file", "", "GitHub repository_dispatch or workflow_dispatch event")
	tokenPath := flags.String("token-file", "", "GitHub read token file")
	output := flags.String("output-dir", "", "new patch output directory, must not exist")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *eventPath == "" || *tokenPath == "" || *output == "" {
		return fmt.Errorf("event-file, token-file, and output-dir are required")
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	manifest, err := consumerupdate.Load(manifestData)
	if err != nil {
		return err
	}
	event, err := os.ReadFile(*eventPath)
	if err != nil {
		return err
	}
	metadata, err := consumerupdate.EventMetadata(event)
	if err != nil {
		return err
	}
	token, err := os.ReadFile(*tokenPath)
	if err != nil {
		return fmt.Errorf("read GitHub token file: %w", err)
	}
	archive, err := consumerupdate.Fetch(ctx, metadata, strings.TrimSpace(string(token)))
	if err != nil {
		return err
	}
	patch, err := consumerupdate.Prepare(manifest, metadata, archive)
	if err != nil {
		return err
	}
	if err := consumerupdate.WritePatch(*output, patch); err != nil {
		return err
	}
	fmt.Fprintf(out, "Prepared %d allowlisted files for %s\n", len(patch), metadata.Release)
	return nil
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "consumerupdate:", err)
		os.Exit(1)
	}
}
