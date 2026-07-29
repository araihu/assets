package main

import (
	"bytes"
	"context"
	"testing"

	"github.com/araihu/assets/internal/app"
)

func TestRunMainMapsUsageToExitTwo(t *testing.T) {
	if got := runMain(context.Background(), app.Dependencies{}, []string{"codegen", "go"}, &bytes.Buffer{}, &bytes.Buffer{}); got != 2 {
		t.Fatalf("runMain() = %d, want 2", got)
	}
}

func TestRunMainMapsExecutionFailureToExitOne(t *testing.T) {
	if got := runMain(context.Background(), app.Dependencies{}, []string{"catalog"}, &bytes.Buffer{}, &bytes.Buffer{}); got != 1 {
		t.Fatalf("runMain() = %d, want 1", got)
	}
}
