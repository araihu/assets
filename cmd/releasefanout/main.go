// Command releasefanout is the Go adapter for the Assets release fan-out.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/araihu/assets/internal/releasefanout"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	cancel()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: releasefanout <plan|outputs|dispatch>")
	}
	flags := flag.NewFlagSet("releasefanout "+args[0], flag.ContinueOnError)
	flags.SetOutput(stderr)
	manifestPath := flags.String("manifest", "manifests/release-consumers.yaml", "Consumer enrollment manifest")
	directory := flags.String("plan", "", "Verified plan directory")
	tokenFile := flags.String("token-file", "", "Mounted GitHub token file; never a command-line token")
	release := flags.String("release", "", "Immutable Assets release tag")
	githubOutput := flags.String("github-output", "", "GitHub Actions output file")
	summaryFile := flags.String("summary", "", "Markdown dispatch receipt file")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *directory == "" {
		return errors.New("plan directory is required and positional arguments are not supported")
	}
	if args[0] != "plan" && args[0] != "outputs" && args[0] != "dispatch" {
		return errors.New("unknown fan-out command")
	}
	manifestData, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	manifest, err := releasefanout.LoadManifest(manifestData)
	if err != nil {
		return err
	}
	if args[0] == "outputs" {
		plan, err := releasefanout.ReadPlan(*directory, manifest)
		if err != nil {
			return err
		}
		if *githubOutput == "" {
			return errors.New("GitHub output path is required")
		}
		output, err := os.OpenFile(*githubOutput, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
		if err != nil {
			return err
		}
		_, writeErr := fmt.Fprintf(output, "repositories=%s\n", strings.Join(plan.Consumers.Repositories, ","))
		return errors.Join(writeErr, output.Close())
	}
	if *tokenFile == "" {
		return errors.New("mounted GitHub token file is required")
	}
	token, err := os.ReadFile(*tokenFile)
	if err != nil {
		return errors.New("could not read GitHub token file")
	}
	github, err := releasefanout.NewGitHub(strings.TrimSuffix(string(token), "\n"))
	if err != nil {
		return err
	}
	if args[0] == "plan" {
		plan, err := github.Prepare(ctx, manifest, *release)
		if err != nil {
			return err
		}
		if err := releasefanout.WritePlan(*directory, plan); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "fan-out plan: release %s verified for %d consumers\n", *release, len(manifest.Repositories))
		return nil
	}
	plan, err := releasefanout.ReadPlan(*directory, manifest)
	if err != nil {
		return err
	}
	results, dispatchErr := github.Dispatch(ctx, manifest, plan)
	summary := releasefanout.Summary(results, plan.Metadata.Release)
	_, printErr := io.WriteString(stdout, summary)
	if *summaryFile != "" {
		if err := os.MkdirAll(filepath.Dir(*summaryFile), 0o755); err != nil {
			return errors.Join(dispatchErr, printErr, err)
		}
		if err := os.WriteFile(*summaryFile, []byte(summary), 0o644); err != nil {
			return errors.Join(dispatchErr, printErr, err)
		}
	}
	return errors.Join(dispatchErr, printErr)
}
