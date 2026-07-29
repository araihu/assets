package build

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestReleaseTreeHasNoHistoricalAssetTrees(t *testing.T) {
	for _, name := range []string{"concepts", "recraft", "logos", "archive"} {
		_, err := os.Stat(filepath.Join(repoRoot(t), name))
		if !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("historical asset tree %q remains: %v", name, err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate layout test")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
