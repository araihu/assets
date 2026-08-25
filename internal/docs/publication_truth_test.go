package docs_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	v023TagURL       = "https://github.com/araihu/assets/tree/v0.2.3"
	v023ReleaseURL   = "https://github.com/araihu/assets/releases/tag/v0.2.3"
	v023ArchiveURL   = "https://github.com/araihu/assets/releases/download/v0.2.3/araihu-assets-v0.2.3.tar.gz"
	v023ChecksumsURL = "https://github.com/araihu/assets/releases/download/v0.2.3/SHA256SUMS"
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
			v023TagURL,
			v023ReleaseURL,
			v023ArchiveURL,
			v023ChecksumsURL,
		} {
			if !strings.Contains(contents, want) {
				t.Errorf("%s does not link %q", path, want)
			}
		}
	}

	goshtoso := mustRead(t, filepath.Join(root, "docs", "integration", "goshtoso.md"))
	if strings.Contains(goshtoso, "unpublished evidence") {
		t.Error("Goshtoso integration guide still calls public v0.2.3 unpublished")
	}

	readme := mustRead(t, filepath.Join(root, "README.md"))
	if strings.Contains(readme, "Before that public identity exists") {
		t.Error("README still treats public v0.2.3 identity as unavailable")
	}

	consumers := mustRead(t, filepath.Join(root, "docs", "integration", "consumers.md"))
	if !strings.Contains(consumers, "`v0.1.1` remains the independently published promoted default.") {
		t.Error("consumer guide no longer distinguishes the independently promoted v0.1.1 default")
	}

	defaultManifest := mustRead(t, filepath.Join(root, "manifests", "default.yaml"))
	if !strings.Contains(defaultManifest, "release: v0.1.1\n") {
		t.Error("default channel changed; T-AS-007 documents publication truth only")
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
