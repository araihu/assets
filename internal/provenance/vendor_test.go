package provenance

import (
	"bytes"
	"crypto/sha512"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/acquisition"
	"github.com/araihu/assets/internal/sprite"
	"github.com/araihu/assets/internal/svgasset"
)

func TestMuambaHashMatchesEmbeddedBytes(t *testing.T) {
	for _, resource := range acquisition.MuambaResources() {
		for _, download := range resource.Downloads {
			file, err := acquisition.MuambaOpen(resource.Name, download.Name)
			if err != nil {
				t.Fatalf("MuambaOpen(%s/%s): %v", resource.Name, download.Name, err)
			}
			data, err := io.ReadAll(file)
			_ = file.Close()
			if err != nil {
				t.Fatal(err)
			}
			sum := sha512.Sum384(data)
			want := "sha384:" + hex.EncodeToString(sum[:])
			if got, ok := acquisition.MuambaHash(resource.Name, download.Name); !ok || got != want {
				t.Fatalf("MuambaHash(%s/%s) = %q, %v; want %q", resource.Name, download.Name, got, ok, want)
			}
		}
	}
}

func TestBuildUIFromPinnedArchivesIsCompleteSafeAndDeterministic(t *testing.T) {
	source := lockedSource(t)
	ui := lockedUI(t, source)
	first, err := BuildUI(source, ui)
	if err != nil {
		t.Fatalf("BuildUI(first) error = %v", err)
	}
	second, err := BuildUI(source, ui)
	if err != nil {
		t.Fatalf("BuildUI(second) error = %v", err)
	}
	if len(first.Assets) != 1606 || len(first.Files) != 1612 {
		t.Fatalf("BuildUI() assets/files = %d/%d, want 1606/1612", len(first.Assets), len(first.Files))
	}
	for name, want := range first.Files {
		if got := second.Files[name]; !bytes.Equal(got, want) {
			t.Fatalf("BuildUI() changed %s between runs", name)
		}
	}
	if _, ok := first.Files["icons/brand/developer-icons/tRPC.svg"]; !ok {
		t.Fatal("missing literal tRPC artifact path")
	}
	foundTRPC := false
	for _, asset := range first.Assets {
		if asset.CanonicalName == "brand-developer-icons-tRPC" {
			foundTRPC = true
			if asset.SpriteSymbol != "devicon-trpc" || !strings.Contains(asset.Source, developerIconsRevision) {
				t.Fatalf("tRPC catalog asset = %#v", asset)
			}
		}
	}
	if !foundTRPC {
		t.Fatal("catalog omits literal tRPC canonical name")
	}
	for name, data := range first.Files {
		if strings.HasSuffix(name, ".svg") && !strings.HasSuffix(name, "/sprite.svg") && name != "icons/ui/sprite.svg" {
			if _, err := svgasset.Parse(data); err != nil {
				t.Fatalf("generated %s is unsafe: %v", name, err)
			}
		}
		if bytes.Contains(bytes.ToLower(data), []byte("<script")) {
			t.Fatalf("generated %s contains script", name)
		}
	}
	for _, name := range []string{"icons/ui/sprite.svg", "icons/brand/developer-icons/sprite.svg"} {
		if err := sprite.Validate(first.Files[name]); err != nil {
			t.Fatalf("validate %s: %v", name, err)
		}
	}
}

func TestBuildUIRejectsMissingOrCorruptArchive(t *testing.T) {
	source := lockedSource(t)
	ui := lockedUI(t, source)
	for _, source := range []Source{
		mutatedSource{Source: source, missing: "optimized"},
		mutatedSource{Source: source, corrupt: "optimized"},
	} {
		if _, err := BuildUI(source, ui); err == nil {
			t.Fatal("BuildUI() error = nil")
		}
	}
}

type mutatedSource struct {
	Source
	missing string
	corrupt string
}

func (source mutatedSource) Lookup(resource, download string) (Record, bool) {
	if download == source.missing {
		return Record{}, false
	}
	record, ok := source.Source.Lookup(resource, download)
	if download == source.corrupt {
		record.Hash = "sha384:corrupt"
	}
	return record, ok
}

func (source mutatedSource) Directory(resource, directory string) (acquisition.LockedDirectory, bool) {
	if directory == source.missing {
		return acquisition.LockedDirectory{}, false
	}
	locked, ok := source.Source.Directory(resource, directory)
	if directory == source.corrupt && len(locked.Files) != 0 {
		locked.Files[0].Integrity = "sha384-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
	}
	return locked, ok
}

func lockedSource(t *testing.T) Source {
	t.Helper()
	source, err := acquisition.Repository(os.DirFS("../.."), ".muamba.lock.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func lockedUI(t *testing.T, source Source) UI {
	t.Helper()
	inventory, err := acquisition.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.Open("../../manifests/icons-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	ui, err := LoadUI(file, inventory, source)
	if err != nil {
		t.Fatal(err)
	}
	return ui
}
