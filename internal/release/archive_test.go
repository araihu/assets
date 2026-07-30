package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestPublicBundleRejectsDifferentByteCollision(t *testing.T) {
	destination := t.TempDir()
	if err := os.MkdirAll(filepath.Join(destination, "releases", "v0.1.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "releases", "v0.1.1", "catalog.json"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = PublicBundle(context.Background(), root, "v0.1.1", fstest.MapFS{"catalog.json": {Data: []byte("new")}}, []string{"catalog.json"})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("assemble error = %v", err)
	}
}

func TestPublicBundlePreflightsCollisionBeforePublishing(t *testing.T) {
	destination := t.TempDir()
	if err := os.MkdirAll(filepath.Join(destination, "releases", "v0.1.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "releases", "v0.1.1", "z-collision.txt"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = PublicBundle(context.Background(), root, "v0.1.1", fstest.MapFS{
		"a-new.txt":       {Data: []byte("new")},
		"z-collision.txt": {Data: []byte("different")},
	}, []string{"a-new.txt", "z-collision.txt"})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("PublicBundle() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, "releases", "v0.1.1", "a-new.txt")); !os.IsNotExist(err) {
		t.Fatalf("partial public member = %v", err)
	}
}

func TestPublicBundleRejectsConcurrentCollisionAndRollsBack(t *testing.T) {
	destination := t.TempDir()
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	publicBundleAfterPreflight = func() {
		mustWrite(t, filepath.Join(destination, "releases", "v0.1.1", "z-raced.txt"), []byte("raced"))
	}
	t.Cleanup(func() { publicBundleAfterPreflight = nil })
	err = PublicBundle(context.Background(), root, "v0.1.1", fstest.MapFS{
		"a-created.txt": {Data: []byte("created")},
		"z-raced.txt":   {Data: []byte("wanted")},
	}, []string{"a-created.txt", "z-raced.txt"})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("PublicBundle() error = %v", err)
	}
	if _, err := os.Lstat(filepath.Join(destination, "releases", "v0.1.1", "a-created.txt")); !os.IsNotExist(err) {
		t.Fatalf("rolled back member = %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destination, "releases", "v0.1.1", "z-raced.txt"))
	if err != nil || string(got) != "raced" {
		t.Fatalf("concurrent member = %q, %v", got, err)
	}
}

func TestPublicBundleWrapsFilesAndAllowsIdenticalCollision(t *testing.T) {
	destination := t.TempDir()
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	source := fstest.MapFS{"catalog.json": {Data: []byte("same")}}
	for range 2 {
		if err := PublicBundle(context.Background(), root, "v0.1.1", source, []string{"catalog.json"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := os.ReadFile(filepath.Join(destination, "releases", "v0.1.1", "catalog.json"))
	if err != nil || string(got) != "same" {
		t.Fatalf("public file = %q, %v", got, err)
	}
}

func TestPublicBundleRejectsDestinationSymlink(t *testing.T) {
	destination := t.TempDir()
	if err := os.MkdirAll(filepath.Join(destination, "releases", "v0.1.1"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("elsewhere", filepath.Join(destination, "releases", "v0.1.1", "catalog.json")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	err = PublicBundle(context.Background(), root, "v0.1.1", fstest.MapFS{"catalog.json": {Data: []byte("new")}}, []string{"catalog.json"})
	if err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("assemble error = %v", err)
	}
}

func TestPublicBundleRejectsMaskedSourceSymlink(t *testing.T) {
	destination := t.TempDir()
	root, err := os.OpenRoot(destination)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	source := fstest.MapFS{"hidden": {Data: []byte("link"), Mode: fs.ModeSymlink}}
	err = PublicBundle(context.Background(), root, "v0.1.1", maskedTypeFS{source}, []string{"hidden"})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("PublicBundle() error = %v", err)
	}
	source = fstest.MapFS{"special": {Data: []byte("pipe"), Mode: fs.ModeNamedPipe}}
	err = PublicBundle(context.Background(), root, "v0.1.1", maskedTypeFS{source}, []string{"special"})
	if err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("PublicBundle() error = %v", err)
	}
}

func TestArchiveIsByteDeterministic(t *testing.T) {
	files := fstest.MapFS{
		"z.txt":        &fstest.MapFile{Data: []byte("z")},
		"nested/a.txt": &fstest.MapFile{Data: []byte("a")},
		"nested/b.txt": &fstest.MapFile{Data: []byte("b")},
	}
	paths := []string{"z.txt", "nested/b.txt", "nested/a.txt"}
	var first, second bytes.Buffer
	if err := Archive(&first, files, paths); err != nil {
		t.Fatal(err)
	}
	if err := Archive(&second, files, paths); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("tar.gz archive bytes differ")
	}
	requireTarMetadata(t, first.Bytes(), []string{"nested/a.txt", "nested/b.txt", "z.txt"})
}

func TestZIPIsByteDeterministicAndNormalized(t *testing.T) {
	files := fstest.MapFS{"b.txt": &fstest.MapFile{Data: []byte("b")}, "a.txt": &fstest.MapFile{Data: []byte("a")}}
	var first, second bytes.Buffer
	if err := ZIP(&first, files, []string{"b.txt", "a.txt"}); err != nil {
		t.Fatal(err)
	}
	if err := ZIP(&second, files, []string{"a.txt", "b.txt"}); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first.Bytes(), second.Bytes()) {
		t.Fatal("ZIP archive bytes differ")
	}
	reader, err := zip.NewReader(bytes.NewReader(first.Bytes()), int64(first.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 || reader.File[0].Name != "a.txt" || reader.File[1].Name != "b.txt" {
		t.Fatalf("ZIP members = %#v", reader.File)
	}
	for _, file := range reader.File {
		if !file.Modified.Equal(time.Unix(0, 0).UTC()) || file.Mode() != 0o644 || file.NonUTF8 {
			t.Fatalf("ZIP metadata for %s not normalized: modified=%s mode=%#o utf8=%t", file.Name, file.Modified, file.Mode(), file.NonUTF8)
		}
	}
}

func TestArchiveRejectsUnsafeDuplicateAndNonRegularPaths(t *testing.T) {
	files := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("x")}, "dir": &fstest.MapFile{Mode: 0o755 | 1<<31}, "C:/file.txt": &fstest.MapFile{Data: []byte("x")}}
	for _, paths := range [][]string{{"../file.txt"}, {`file\\name.txt`}, {"C:/file.txt"}, {"file.txt", "file.txt"}, {"dir"}} {
		if err := Archive(io.Discard, files, paths); err == nil {
			t.Fatalf("Archive accepted %q", paths)
		}
	}
}

func TestArchiveRejectsIntermediateSourceSymlink(t *testing.T) {
	sourceRoot, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sourceRoot, "link")); err != nil {
		t.Fatal(err)
	}
	if err := Archive(io.Discard, os.DirFS(sourceRoot), []string{"link/file.txt"}); err == nil {
		t.Fatal("Archive followed intermediate source symlink")
	}
}

func TestArchiveRootRejectsEscapingSourceSymlink(t *testing.T) {
	sourceRoot, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "file.txt"), []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sourceRoot, "link")); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(sourceRoot)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := ArchiveRoot(io.Discard, root, []string{"link/file.txt"}); err == nil {
		t.Fatal("ArchiveRoot accepted escaping source symlink")
	}
	if err := ZIPRoot(io.Discard, root, []string{"link/file.txt"}); err == nil {
		t.Fatal("ZIPRoot accepted escaping source symlink")
	}
}

func requireTarMetadata(t *testing.T, data []byte, want []string) {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var got []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, header.Name)
		if !header.ModTime.Equal(time.Unix(0, 0).UTC()) || header.Mode != 0o644 || header.Uid != 0 || header.Gid != 0 || header.Uname != "" || header.Gname != "" {
			t.Fatalf("tar metadata for %s not normalized: %#v", header.Name, header)
		}
	}
	if !equalStrings(got, want) {
		t.Fatalf("tar members = %q, want %q", got, want)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func mustWrite(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

type maskedTypeFS struct{ fs.FS }

func (filesystem maskedTypeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := fs.ReadDir(filesystem.FS, name)
	if err != nil {
		return nil, err
	}
	for index, entry := range entries {
		entries[index] = maskedTypeEntry{entry}
	}
	return entries, nil
}

type maskedTypeEntry struct{ fs.DirEntry }

func (maskedTypeEntry) Type() fs.FileMode { return 0 }
