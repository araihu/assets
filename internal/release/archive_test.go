package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"io"
	"testing"
	"testing/fstest"
	"time"
)

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
	files := fstest.MapFS{"file.txt": &fstest.MapFile{Data: []byte("x")}, "dir": &fstest.MapFile{Mode: 0o755 | 1<<31}}
	for _, paths := range [][]string{{"../file.txt"}, {`file\\name.txt`}, {"file.txt", "file.txt"}, {"dir"}} {
		if err := Archive(io.Discard, files, paths); err == nil {
			t.Fatalf("Archive accepted %q", paths)
		}
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
