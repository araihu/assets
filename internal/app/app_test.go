package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
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

func TestVendorRejectsSymlinkedManagedVersionDirectory(t *testing.T) {
	repo, outside := t.TempDir(), t.TempDir()
	copyManifest(t, repo, "icons-ui.yaml")
	managed := filepath.Join(repo, "vendor", "icons", "ui", "heroicons")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(managed, "v2.2.0")); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"vendor"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "symbolic-link component") {
		t.Fatalf("Run(vendor) error = %v, want managed symlink rejection", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("vendor wrote outside root: %v, %v", entries, err)
	}
}

func TestExportAndCatalogRejectSymlinkedDist(t *testing.T) {
	repo, outside, output := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "catalog.json"), []byte(`{"not":"catalog"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "dist")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"export", "--output", output}, {"catalog"}} {
		err := Run(context.Background(), Dependencies{Repo: repo}, args, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "symbolic-link component") {
			t.Fatalf("Run(%q) error = %v, want managed symlink rejection", args, err)
		}
	}
	if entries, err := os.ReadDir(output); err != nil || len(entries) != 0 {
		t.Fatalf("export used outside dist: %v, %v", entries, err)
	}
}

func TestExportCancellationAfterEnumerationLeavesNewOutputAbsent(t *testing.T) {
	repo, outputParent := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dist", "release.txt"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outputParent, "new-output")
	ctx, cancel := context.WithCancel(context.Background())
	exportAfterEnumerationHook = cancel
	t.Cleanup(func() { exportAfterEnumerationHook = nil })

	err := Run(ctx, Dependencies{Repo: repo}, []string{"export", "--output", output}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(export) error = %v, want context canceled", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled export created output %q: %v", output, err)
	}
}

func copyManifest(t *testing.T, repo, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "manifests", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "manifests", name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}
