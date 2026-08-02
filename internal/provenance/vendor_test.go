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
			got := "sha384:" + hex.EncodeToString(sum[:])
			hash, ok := acquisition.MuambaHash(resource.Name, download.Name)
			if !ok || hash != got {
				t.Fatalf("MuambaHash(%s/%s) = %q, %v; want %q", resource.Name, download.Name, hash, ok, got)
			}
		}
	}
}

func TestBuildUIFromMuambaIsDeterministicAndCompatible(t *testing.T) {
	ui := lockedUI(t)
	first, err := BuildUI(acquisition.Embedded(), ui)
	if err != nil {
		t.Fatalf("BuildUI(first) error = %v", err)
	}
	second, err := BuildUI(acquisition.Embedded(), ui)
	if err != nil {
		t.Fatalf("BuildUI(second) error = %v", err)
	}
	if len(first.Assets) != 67 || len(first.Files) != 70 {
		t.Fatalf("BuildUI() assets/files = %d/%d, want 67/70", len(first.Assets), len(first.Files))
	}
	for name, want := range first.Files {
		if got := second.Files[name]; !bytes.Equal(got, want) {
			t.Fatalf("BuildUI() changed %s between runs", name)
		}
		if name == "icons/ui/heroicons/provenance.json" {
			continue
		}
		tracked, err := os.ReadFile("../../dist/" + name)
		if err != nil {
			t.Fatalf("read tracked %s: %v", name, err)
		}
		if !bytes.Equal(want, tracked) {
			t.Fatalf("generated %s differs from pre-migration dist", name)
		}
	}
	icon := first.Files["icons/ui/heroicons/16-solid-check.svg"]
	if !bytes.Contains(icon, []byte(`fill="currentColor"`)) || bytes.Contains(icon, []byte(`width="16"`)) {
		t.Fatalf("normalized icon = %s", icon)
	}
	for name, svg := range first.Files {
		if !strings.HasSuffix(name, ".svg") || name == "icons/ui/sprite.svg" {
			continue
		}
		if _, err := svgasset.Parse(svg); err != nil {
			t.Errorf("generated %s did not reparse: %v", name, err)
		}
	}
	if err := sprite.Validate(first.Files["icons/ui/sprite.svg"]); err != nil {
		t.Fatalf("validate generated UI sprite: %v", err)
	}
	provenance := first.Files["icons/ui/heroicons/provenance.json"]
	for _, want := range []string{`"ref": "heroicons/icon-16-solid-check"`, `"integrity": "sha384-`, `"hash": "sha384:`} {
		if !bytes.Contains(provenance, []byte(want)) {
			t.Fatalf("provenance omits %s", want)
		}
	}
}

func TestBuildUIRejectsMissingOrCorruptAcquisition(t *testing.T) {
	ui := lockedUI(t)
	for _, source := range []Source{
		mutatedSource{Source: acquisition.Embedded(), missing: "icon-16-solid-check"},
		mutatedSource{Source: acquisition.Embedded(), corrupt: "icon-16-solid-check"},
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

func lockedUI(t *testing.T) UI {
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
	ui, err := LoadUI(file, inventory)
	if err != nil {
		t.Fatal(err)
	}
	return ui
}
