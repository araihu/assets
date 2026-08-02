package provenance

import (
	"os"
	"strings"
	"testing"

	"github.com/araihu/assets/assetmeta"
	"github.com/araihu/assets/internal/acquisition"
)

func TestLoadUIPreservesCompleteInventoryOrder(t *testing.T) {
	inventory := acquisitionInventory(t)
	file, err := os.Open("../../manifests/icons-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	ui, err := LoadUI(file, inventory)
	if err != nil {
		t.Fatalf("LoadUI() error = %v", err)
	}
	if len(ui.Icons) != 67 {
		t.Fatalf("icons = %d, want 67", len(ui.Icons))
	}
	if got := ui.Icons[0].Path; got != "16/solid/arrow-down-tray.svg" {
		t.Fatalf("first icon = %q", got)
	}
	if got := ui.Icons[len(ui.Icons)-1].Path; got != "16/solid/x-mark.svg" {
		t.Fatalf("last icon = %q", got)
	}
}

func TestLoadUIRejectsInvalidOverlayContracts(t *testing.T) {
	inventory := acquisitionInventory(t)
	raw, err := os.ReadFile("../../manifests/icons-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	valid := string(raw)
	tests := []struct {
		name string
		raw  string
	}{
		{name: "schema", raw: strings.Replace(valid, "schema: 1", "schema: 2", 1)},
		{name: "unknown download", raw: strings.Replace(valid, "heroicons/icon-16-solid-arrow-down-tray", "heroicons/missing", 1)},
		{name: "duplicate icon", raw: strings.Replace(valid, "heroicons/icon-16-solid-arrow-down\n", "heroicons/icon-16-solid-arrow-down-tray\n", 1)},
		{name: "missing license", raw: strings.Replace(valid, "license_ref: heroicons/license", "license_ref: heroicons/missing", 1)},
		{name: "wrong version", raw: strings.Replace(valid, "version: v2.2.0", "version: v2.1.0", 1)},
		{name: "invalid alias", raw: strings.Replace(valid, "alias: hi", "alias: HI!", 1)},
		{name: "missing icon", raw: valid[:strings.LastIndex(valid, "    - ref:")]},
		{name: "wrong path", raw: strings.Replace(valid, "path: 16/solid/arrow-down-tray.svg", "path: 16/solid/not-the-download.svg", 1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := LoadUI(strings.NewReader(test.raw), inventory); err == nil {
				t.Fatal("LoadUI() error = nil")
			}
		})
	}
}

func TestLoadUIRejectsNonHeroiconsInventory(t *testing.T) {
	inventory, err := assetmeta.NewInventory([]assetmeta.Resource{{
		Name: "other", Version: "v2.2.0", Downloads: []assetmeta.Download{{
			Name: "license", URL: "https://example.invalid/LICENSE", Path: "LICENSE", Integrity: "sha384-test", Hash: "sha384:test",
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUI(strings.NewReader("schema: 1\nmetadata: {}\n"), inventory); err == nil {
		t.Fatal("LoadUI() error = nil")
	}
}

func acquisitionInventory(t *testing.T) *assetmeta.Inventory {
	t.Helper()
	resources := make([]assetmeta.Resource, 0, len(acquisition.MuambaResources()))
	for _, resource := range acquisition.MuambaResources() {
		downloads := make([]assetmeta.Download, 0, len(resource.Downloads))
		for _, download := range resource.Downloads {
			downloads = append(downloads, assetmeta.Download{
				Name: download.Name, URL: download.URL, Path: download.Path,
				Integrity: download.Integrity, Hash: download.Hash,
			})
		}
		resources = append(resources, assetmeta.Resource{Name: resource.Name, Version: resource.Version, Downloads: downloads})
	}
	inventory, err := assetmeta.NewInventory(resources)
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
