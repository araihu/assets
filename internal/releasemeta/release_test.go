package releasemeta

import (
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
