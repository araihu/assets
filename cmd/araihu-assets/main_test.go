package main

import (
	"bytes"
	"context"
	"os"
	"syscall"
	"testing"

	"github.com/araihu/assets/internal/app"
)

func TestRunMainMapsUsageToExitTwo(t *testing.T) {
	if got := runMain(context.Background(), app.Dependencies{}, []string{"codegen", "go"}, &bytes.Buffer{}, &bytes.Buffer{}); got != 2 {
		t.Fatalf("runMain() = %d, want 2", got)
	}
}

func TestSignalContextWiresInterruptAndSIGTERM(t *testing.T) {
	original := notifyContext
	t.Cleanup(func() { notifyContext = original })
	var got []os.Signal
	notifyContext = func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc) {
		got = append([]os.Signal(nil), signals...)
		return context.WithCancel(parent)
	}
	_, stop := signalContext(context.Background())
	defer stop()
	if len(got) != 2 || got[0] != os.Interrupt || got[1] != syscall.SIGTERM {
		t.Fatalf("signalContext signals = %v, want [%v %v]", got, os.Interrupt, syscall.SIGTERM)
	}
}

func TestRunMainMapsExecutionFailureToExitOne(t *testing.T) {
	if got := runMain(context.Background(), app.Dependencies{}, []string{"catalog"}, &bytes.Buffer{}, &bytes.Buffer{}); got != 1 {
		t.Fatalf("runMain() = %d, want 1", got)
	}
}
