package export

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

func TestCopyRejectsTraversalAndDifferentCollision(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "same.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	source := fstest.MapFS{"same.txt": &fstest.MapFile{Data: []byte("new")}}
	if err := Copy(source, []string{"same.txt"}, root); err == nil {
		t.Fatal("Copy accepted different destination bytes")
	}
	if got, err := os.ReadFile(filepath.Join(rootDir, "same.txt")); err != nil || string(got) != "old" {
		t.Fatalf("collision changed destination: %q, %v", got, err)
	}
	if err := Copy(source, []string{"../escape.txt"}, root); err == nil {
		t.Fatal("Copy accepted traversal")
	}
	if err := Copy(source, []string{`dir\\name.txt`}, root); err == nil {
		t.Fatal("Copy accepted backslash path")
	}
}

func TestCopyAcceptsIdenticalExistingBytes(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(rootDir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rootDir, "nested", "file.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	err = Copy(fstest.MapFS{"nested/file.txt": &fstest.MapFile{Data: []byte("same")}}, []string{"nested/file.txt"}, root)
	if err != nil {
		t.Fatalf("Copy identical bytes: %v", err)
	}
}

func TestCopyRejectsEscapingDestinationSymlink(t *testing.T) {
	rootDir, outside := t.TempDir(), t.TempDir()
	if err := os.Symlink(outside, filepath.Join(rootDir, "out")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = Copy(fstest.MapFS{"out/file.txt": &fstest.MapFile{Data: []byte("unsafe")}}, []string{"out/file.txt"}, root)
	if err == nil {
		t.Fatal("Copy followed escaping destination symlink")
	}
	if _, statErr := os.Stat(filepath.Join(outside, "file.txt")); !errors.Is(statErr, fs.ErrNotExist) {
		t.Fatalf("outside file = %v, want absent", statErr)
	}
}
