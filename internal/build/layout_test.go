package build

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"
)

var retainedProofScreenshotPath = regexp.MustCompile(`^review/screenshots/v0\.1-proof-[^/.]+\.png$`)

func TestReleaseTreeHasNoHistoricalAssetTrees(t *testing.T) {
	for _, name := range []string{
		"concepts", "recraft", "logos", "brand", "archive",
		"output/pdf",
	} {
		_, err := os.Stat(filepath.Join(repoRoot(t), name))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("historical asset tree %q remains: %v", name, err)
		}
	}

	assertRootIgnorePolicy(t)
	assertReviewScreenshotTree(t)
	assertTrackedReleaseLayout(t)
}

func TestAllowedRetainedReviewPath(t *testing.T) {
	for path, want := range map[string]bool{
		"review/logo-system-v11.html":                    true,
		"review/v11-assessment.md":                       true,
		"review/screenshots/v0.1-proof-desktop.png":      true,
		"review/screenshots/v0.1-proof-mobile-1280.png":  true,
		"review/screenshots/logo-system-v11-desktop.png": false,
		"review/screenshots/v0.1-proof-desktop.svg":      false,
		"review/screenshots/random.png":                  false,
		"review/screenshots/v0.1-proof-/nested.png":      false,
		"review/site-context-v10.html":                   false,
	} {
		if got := allowedRetainedReviewPath(path); got != want {
			t.Errorf("allowedRetainedReviewPath(%q) = %t, want %t", path, got, want)
		}
	}
}

func assertRootIgnorePolicy(t *testing.T) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".gitignore"))
	if err != nil {
		t.Fatalf("read root ignore policy: %v", err)
	}
	if !slices.Contains(strings.Fields(string(data)), ".superpowers/sdd/") {
		t.Fatal("root ignore policy must exclude .superpowers/sdd/")
	}
}

func assertTrackedReleaseLayout(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repoRoot(t), ".git")); errors.Is(err, fs.ErrNotExist) {
		return
	} else if err != nil {
		t.Fatalf("stat repository metadata: %v", err)
	}

	paths := trackedPaths(t)
	for _, path := range paths {
		if strings.HasPrefix(path, ".superpowers/sdd/") {
			t.Errorf("tracked controller artifact remains: %s", path)
		}
	}

	assertTrackedReviewPaths(t, paths)

	legacyVersion := regexp.MustCompile(`(?:^|[^[:alnum:]])v(?:[2-9]|10)(?:[^[:digit:]]|$)`)
	legacyScriptNames := map[string]struct{}{
		"scripts/build-logo-system-v3.py":       {},
		"scripts/build-outlined-lockups.py":     {},
		"scripts/outline-wordmarks.py":          {},
		"scripts/render-blind-review-pdf.sh":    {},
		"scripts/render-logo-history.py":        {},
		"scripts/render-logo-lockups.py":        {},
		"scripts/render-logo-review.sh":         {},
		"scripts/render-logo-system-v5.py":      {},
		"scripts/requirements-wordmarks.txt":    {},
		"scripts/score-blind-reviews.py":        {},
		"scripts/test-score-blind-reviews.py":   {},
		"scripts/validate-logo-explorations.sh": {},
		"scripts/validate-logo-system.sh":       {},
	}
	for _, path := range paths {
		_, namedLegacyScript := legacyScriptNames[path]
		if strings.HasPrefix(path, "scripts/") && (namedLegacyScript || legacyVersion.MatchString(path)) {
			t.Errorf("obsolete historical script remains: %s", path)
		}
	}
}

func assertTrackedReviewPaths(t *testing.T, paths []string) {
	t.Helper()
	for _, path := range paths {
		if strings.HasPrefix(path, "review/") && !allowedRetainedReviewPath(path) {
			t.Errorf("legacy tracked path remains: %s", path)
		}
	}
}

func assertReviewScreenshotTree(t *testing.T) {
	t.Helper()
	root := filepath.Join(repoRoot(t), "review", "screenshots")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		if entry.IsDir() || !entry.Type().IsRegular() {
			t.Errorf("unexpected review screenshot entry: %s", path)
			return nil
		}
		relative, err := filepath.Rel(repoRoot(t), path)
		if err != nil {
			return err
		}
		if !allowedRetainedReviewPath(filepath.ToSlash(relative)) {
			t.Errorf("legacy review screenshot remains: %s", relative)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		t.Fatalf("walk review screenshots: %v", err)
	}
}

func allowedRetainedReviewPath(path string) bool {
	if slices.Contains([]string{
		"review/.gitignore", "review/logo-system-v11.html", "review/v11-assessment.md",
	}, path) {
		return true
	}
	return retainedProofScreenshotPath.MatchString(path)
}

func trackedPaths(t *testing.T) []string {
	t.Helper()
	command := exec.Command("git", "ls-files")
	command.Dir = repoRoot(t)
	output, err := command.Output()
	if err != nil {
		t.Fatalf("list tracked paths: %v", err)
	}
	return strings.Fields(string(output))
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate layout test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
