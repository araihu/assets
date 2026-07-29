package export

import (
	"context"
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

func TestCopyPreflightsAllCollisionsBeforePublishing(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "z.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = Copy(fstest.MapFS{"a.txt": &fstest.MapFile{Data: []byte("new-a")}, "z.txt": &fstest.MapFile{Data: []byte("new-z")}}, []string{"a.txt", "z.txt"}, root)
	if err == nil {
		t.Fatal("Copy accepted later collision")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "a.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("a.txt published before collision: %v", err)
	}
}

func TestCopyRollsBackCreatedFilesAfterLateNoReplaceFailure(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publishHook = func(phase publishPhase, name string, destination *os.Root) error {
		if phase == beforeLink && name == "z.txt" {
			return destination.WriteFile(name, []byte("racer"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { publishHook = nil })
	err = Copy(fstest.MapFS{"a.txt": &fstest.MapFile{Data: []byte("a")}, "z.txt": &fstest.MapFile{Data: []byte("z")}}, []string{"a.txt", "z.txt"}, root)
	if err == nil {
		t.Fatal("Copy accepted concurrent collision")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "a.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("rollback left a.txt: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(rootDir, "z.txt")); err != nil || string(got) != "racer" {
		t.Fatalf("race destination = %q, %v", got, err)
	}
}

func TestCopyRejectsTargetReplacementAfterLinkWithoutDeletingIt(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publishHook = func(phase publishPhase, name string, destination *os.Root) error {
		if phase == afterLink && name == "victim.txt" {
			if err := destination.Remove(name); err != nil {
				return err
			}
			return destination.WriteFile(name, []byte("attacker"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { publishHook = nil })
	if err := Copy(fstest.MapFS{"victim.txt": &fstest.MapFile{Data: []byte("release")}}, []string{"victim.txt"}, root); err == nil {
		t.Fatal("Copy returned success after target replacement")
	}
	if got, err := os.ReadFile(filepath.Join(rootDir, "victim.txt")); err != nil || string(got) != "attacker" {
		t.Fatalf("replacement target = %q, %v", got, err)
	}
}

func TestCopyRechecksStagedTargetBytesAfterLink(t *testing.T) {
	rootDir := t.TempDir()
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publishHook = func(phase publishPhase, name string, destination *os.Root) error {
		if phase == afterLink && name == "victim.txt" {
			return destination.WriteFile(name, []byte("mutated"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { publishHook = nil })
	if err := Copy(fstest.MapFS{"victim.txt": &fstest.MapFile{Data: []byte("release")}}, []string{"victim.txt"}, root); err == nil {
		t.Fatal("Copy returned success after staged target mutation")
	}
	if _, err := os.Stat(filepath.Join(rootDir, "victim.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("operation-owned mutated target remains: %v", err)
	}
}

func TestCopyRevalidatesPreexistingIdenticalTargetAtPublishBoundary(t *testing.T) {
	rootDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(rootDir, "same.txt"), []byte("same"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(rootDir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publishHook = func(phase publishPhase, name string, destination *os.Root) error {
		if phase == beforeExistingVerify && name == "same.txt" {
			return destination.WriteFile(name, []byte("mutated"), 0o644)
		}
		return nil
	}
	t.Cleanup(func() { publishHook = nil })
	if err := Copy(fstest.MapFS{"same.txt": &fstest.MapFile{Data: []byte("same")}}, []string{"same.txt"}, root); err == nil {
		t.Fatal("Copy returned success after preexisting target mutation")
	}
	if got, err := os.ReadFile(filepath.Join(rootDir, "same.txt")); err != nil || string(got) != "mutated" {
		t.Fatalf("mutated target = %q, %v", got, err)
	}
}

func TestCopyRejectsIntermediateSourceSymlinkAndVolumePath(t *testing.T) {
	sourceRoot, outside, destination := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sourceRoot, "link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := Copy(os.DirFS(sourceRoot), []string{"link/file.txt"}, root); err == nil {
		t.Fatal("Copy followed intermediate source symlink")
	}
	if err := Copy(fstest.MapFS{"C:/file.txt": &fstest.MapFile{Data: []byte("x")}}, []string{"C:/file.txt"}, root); err == nil {
		t.Fatal("Copy accepted drive path")
	}
}

func TestCopyRootRejectsEscapingSourceSymlink(t *testing.T) {
	sourceDir, outside, destination := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sourceDir, "link")); err != nil {
		t.Fatal(err)
	}
	source, err := os.OpenRoot(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	target, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	if err := CopyRoot(source, []string{"link/file.txt"}, target); err == nil {
		t.Fatal("CopyRoot accepted escaping source symlink")
	}
}

func TestCopyRootContextCancellationBeforePublishPreservesDestination(t *testing.T) {
	sourceDir, destinationDir := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "release.txt"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	source, err := os.OpenRoot(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	destination, err := os.OpenRoot(destinationDir)
	if err != nil {
		t.Fatal(err)
	}
	defer destination.Close()
	ctx, cancel := context.WithCancel(context.Background())
	publishHook = func(phase publishPhase, name string, destination *os.Root) error {
		if phase == beforeLink && name == "release.txt" {
			cancel()
		}
		return nil
	}
	t.Cleanup(func() { publishHook = nil })

	err = CopyRootContext(ctx, source, []string{"release.txt"}, destination)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("CopyRootContext() error = %v, want context canceled", err)
	}
	if _, err := os.Stat(filepath.Join(destinationDir, "release.txt")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("cancelled export published release.txt: %v", err)
	}
}
