package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/releasefanout"
)

func TestOutputsUsesVerifiedPlanWithoutCredentials(t *testing.T) {
	directory := t.TempDir()
	manifest := filepath.Join(directory, "consumers.yaml")
	if err := os.WriteFile(manifest, []byte("schema_version: 1\nowner: araihu\nrepositories: [alpha, beta]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	p := releasefanout.Plan{Consumers: releasefanout.Manifest{SchemaVersion: 1, Owner: "araihu", Repositories: []string{"alpha", "beta"}}, Metadata: releasefanout.Metadata{
		AssetsRepository: "araihu/assets", AssetsRevision: strings.Repeat("a", 40), Release: "v0.2.4",
		ReleaseURL:    "https://github.com/araihu/assets/releases/download/v0.2.4/araihu-assets-v0.2.4.tar.gz",
		ReleaseSHA256: strings.Repeat("b", 64), ReleaseJSONSHA256: strings.Repeat("c", 64),
	}}
	if err := releasefanout.WritePlan(directory, p); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "github-output")
	var stdout, stderr bytes.Buffer
	args := []string{"outputs", "--manifest", manifest, "--plan", directory, "--github-output", output}
	if err := run(t.Context(), args, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(output)
	if string(data) != "repositories=alpha,beta\n" {
		t.Fatalf("unexpected output %q", data)
	}
	p.Consumers.Repositories = []string{"outside"}
	if err := releasefanout.WritePlan(directory, p); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), args, &stdout, &stderr); err == nil {
		t.Fatal("modified plan reached Actions output")
	}
	after, _ := os.ReadFile(output)
	if !bytes.Equal(after, data) {
		t.Fatal("failed verification wrote output")
	}
}
