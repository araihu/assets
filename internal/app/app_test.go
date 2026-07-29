package app

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

func TestRunRejectsClientCodegen(t *testing.T) {
	for _, args := range [][]string{{"codegen", "go"}, {"generate", "--language", "go"}} {
		err := Run(context.Background(), Dependencies{}, args, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("Run(%q) error = %v, want unknown command", args, err)
		}
		if !IsUsage(err) {
			t.Fatalf("Run(%q) error = %v, want usage error", args, err)
		}
	}
}

func TestExportRequiresOutput(t *testing.T) {
	err := Run(context.Background(), Dependencies{}, []string{"export"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--output is required") {
		t.Fatalf("Run(export) error = %v, want required output", err)
	}
	if !IsUsage(err) {
		t.Fatalf("Run(export) error = %v, want usage error", err)
	}
}

func TestRunRejectsCommandExtraArgumentsAndUnknownFlags(t *testing.T) {
	for _, args := range [][]string{{"catalog", "extra"}, {"catalog", "--unknown"}} {
		var stderr bytes.Buffer
		err := Run(context.Background(), Dependencies{}, args, io.Discard, &stderr)
		if err == nil || !IsUsage(err) {
			t.Fatalf("Run(%q) error = %v, want usage error", args, err)
		}
	}
}

func TestRunHonorsCancelledContextBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, Dependencies{}, []string{"build", "--offline"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "build: context canceled") {
		t.Fatalf("Run(cancelled build) error = %v", err)
	}
}
