package releasemeta

import (
	"io/fs"
	"strings"
	"testing"
	"testing/fstest"
)

func TestBuildInventoriesFilesAndDocumentHashes(t *testing.T) {
	document, err := Build(Input{
		Release:          "v0.1.1",
		IdentityRevision: 11,
		RuntimeVersion:   1,
		Files:            fixtureFiles(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.Files[0].Path != "catalog.json" || len(document.Files[0].SHA256) != 64 {
		t.Fatalf("files = %#v", document.Files)
	}
	if document.CatalogSHA256 != document.Files[0].SHA256 || document.ThemesSHA256 == "" || document.CampaignsSHA256 == "" {
		t.Fatalf("document hashes = %#v", document)
	}
}

func TestBuildRejectsReleaseSelfHashAndUnsafeFiles(t *testing.T) {
	for _, name := range []string{"release self hash", "invalid release", "non-SemVer prerelease"} {
		t.Run(name, func(t *testing.T) {
			files := fixtureFiles()
			if name == "release self hash" {
				files["release.json"] = &fstest.MapFile{Data: []byte("release")}
			}
			release := "v0.1.1"
			if name == "invalid release" {
				release = "latest"
			}
			if name == "non-SemVer prerelease" {
				release = "v0.1.1-01"
			}
			if _, err := Build(Input{Release: release, IdentityRevision: 11, RuntimeVersion: 1, Files: files}); err == nil {
				t.Fatal("Build() error = nil")
			}
		})
	}
}

func TestBuildRejectsSingleBackslashAndMaskedSymlink(t *testing.T) {
	files := fixtureFiles()
	files[`icons\logo.svg`] = &fstest.MapFile{Data: []byte("backslash")}
	if _, err := Build(Input{Release: "v0.1.1", IdentityRevision: 11, RuntimeVersion: 1, Files: files}); err == nil || !strings.Contains(err.Error(), "invalid file path") {
		t.Fatalf("Build() error = %v", err)
	}

	files = fixtureFiles()
	files["hidden"] = &fstest.MapFile{Data: []byte("link"), Mode: fs.ModeSymlink}
	if _, err := Build(Input{Release: "v0.1.1", IdentityRevision: 11, RuntimeVersion: 1, Files: maskedTypeFS{files}}); err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Build() error = %v", err)
	}

	files = fixtureFiles()
	files["special"] = &fstest.MapFile{Data: []byte("pipe"), Mode: fs.ModeNamedPipe}
	if _, err := Build(Input{Release: "v0.1.1", IdentityRevision: 11, RuntimeVersion: 1, Files: maskedTypeFS{files}}); err == nil || !strings.Contains(err.Error(), "non-regular") {
		t.Fatalf("Build() error = %v", err)
	}
}

func TestEncodeCanonicalizesSortedInventory(t *testing.T) {
	document, err := Build(Input{Release: "v0.1.1", IdentityRevision: 11, RuntimeVersion: 1, Files: fixtureFiles()})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(encoded), "\n") || !strings.Contains(string(encoded), "\n  \"catalogSha256\": ") {
		t.Fatalf("Encode() = %s", encoded)
	}
}

func fixtureFiles() fstest.MapFS {
	return fstest.MapFS{
		"catalog.json":   {Data: []byte("catalog")},
		"themes.json":    {Data: []byte("themes")},
		"campaigns.json": {Data: []byte("campaigns")},
		"icons/logo.svg": {Data: []byte("logo")},
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
