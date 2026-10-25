package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	v024TagURL       = "https://github.com/araihu/assets/tree/v0.2.4"
	v024ReleaseURL   = "https://github.com/araihu/assets/releases/tag/v0.2.4"
	v024ArchiveURL   = "https://github.com/araihu/assets/releases/download/v0.2.4/araihu-assets-v0.2.4.tar.gz"
	v024ChecksumsURL = "https://github.com/araihu/assets/releases/download/v0.2.4/SHA256SUMS"
)

func TestPublicationTruthDocumentation(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	for _, path := range []string{
		"README.md",
		"docs/integration/goshtoso.md",
	} {
		contents := mustRead(t, filepath.Join(root, path))
		for _, want := range []string{
			v024TagURL,
			v024ReleaseURL,
			v024ArchiveURL,
			v024ChecksumsURL,
		} {
			if !strings.Contains(contents, want) {
				t.Errorf("%s does not link %q", path, want)
			}
		}
	}

	goshtoso := mustRead(t, filepath.Join(root, "docs", "integration", "goshtoso.md"))
	if strings.Contains(goshtoso, "unpublished evidence") {
		t.Error("Goshtoso integration guide still calls public v0.2.4 unpublished")
	}

	readme := mustRead(t, filepath.Join(root, "README.md"))
	if strings.Contains(readme, "Before that public identity exists") {
		t.Error("README still treats public v0.2.4 identity as unavailable")
	}

	consumers := mustRead(t, filepath.Join(root, "docs", "integration", "consumers.md"))
	if !strings.Contains(consumers, "`manifests/default.yaml` promotes `v0.2.5`.") {
		t.Error("consumer guide does not identify the current promoted default")
	}

	defaultManifest := mustRead(t, filepath.Join(root, "manifests", "default.yaml"))
	if !strings.Contains(defaultManifest, "release: v0.2.5\n") {
		t.Error("default channel does not select the current release")
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
