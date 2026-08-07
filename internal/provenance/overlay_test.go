package provenance

import (
	"os"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/acquisition"
)

func TestLoadUIResolvesExactPinnedPackSurface(t *testing.T) {
	source := lockedSource(t)
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
		t.Fatalf("LoadUI() error = %v", err)
	}
	if len(ui.Packs) != 2 || ui.Packs[0].Source != "developer-icons" || ui.Packs[1].Source != "heroicons" {
		t.Fatalf("packs = %#v", ui.Packs)
	}
	if got := ui.Packs[0].Revision; got != developerIconsRevision {
		t.Fatalf("developer-icons revision = %q", got)
	}
	if got := ui.Packs[1].Variants; len(got) != 4 || got[2] != "24-outline" {
		t.Fatalf("heroicons variants = %#v", got)
	}
}

func TestLoadUIRejectsMetadataDrift(t *testing.T) {
	source := lockedSource(t)
	inventory, err := acquisition.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../../manifests/icons-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{
		strings.Replace(string(raw), developerIconsRevision, "0000000000000000000000000000000000000000", 1),
		strings.Replace(string(raw), "- 24-outline", "- 24-mini", 1),
		strings.Replace(string(raw), "source_ref: heroicons/optimized", "source_ref: heroicons/missing", 1),
	} {
		if _, err := LoadUI(strings.NewReader(changed), inventory, source); err == nil {
			t.Fatal("LoadUI() accepted metadata drift")
		}
	}
}
