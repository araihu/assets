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

func TestReleaseTreeHasNoHistoricalAssetTrees(t *testing.T) {
	for _, name := range []string{
		"concepts", "recraft", "logos", "brand", "archive",
		"output/pdf", "review/screenshots",
	} {
		_, err := os.Stat(filepath.Join(repoRoot(t), name))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("historical asset tree %q remains: %v", name, err)
		}
	}

	assertRootIgnorePolicy(t)
	assertTrackedReleaseLayout(t)
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

	assertTrackedOnly(t, paths, "review/", []string{
		"review/.gitignore", "review/logo-system-v11.html", "review/v11-assessment.md",
	})

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

func assertTrackedOnly(t *testing.T, paths []string, prefix string, allowed []string) {
	t.Helper()
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, path := range allowed {
		allowedSet[path] = struct{}{}
	}
	for _, path := range paths {
		if strings.HasPrefix(path, prefix) {
			if _, ok := allowedSet[path]; !ok {
				t.Errorf("legacy tracked path remains: %s", path)
			}
		}
	}
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
