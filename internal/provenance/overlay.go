package provenance

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/araihu/assets/assetmeta"
)

const (
	heroiconsRelease            = "v2.2.0"
	heroiconsRevision           = "0435d4ca364a608cc75e2f8683d374e55abbae26"
	developerIconsRelease       = "v7.0.1"
	developerIconsRevision      = "28b895aba6a4984b8c43714336fafa5fa832f08f"
	heroiconsExpectedAssetCount = 1288
	developerExpectedAssetCount = 318
)

type uiMetadata struct {
	Packs []Pack `yaml:"packs"`
}

// UI contains language-neutral semantics for locked icon-pack directories.
type UI struct {
	Packs []Pack
}

// Pack binds one acquired source directory to its catalog and license semantics.
type Pack struct {
	Source     string        `yaml:"source"`
	Alias      string        `yaml:"alias"`
	Release    string        `yaml:"release"`
	Revision   string        `yaml:"revision"`
	Repository string        `yaml:"repository"`
	SourceRef  assetmeta.Ref `yaml:"source_ref"`
	License    string        `yaml:"license"`
	LicenseRef assetmeta.Ref `yaml:"license_ref"`
	Namespace  string        `yaml:"namespace"`
	Variants   []string      `yaml:"variants"`
}

// LoadUI strictly resolves icon-pack metadata against the generated inventory.
func LoadUI(reader io.Reader, inventory *assetmeta.Inventory, source Source) (UI, error) {
	if inventory == nil {
		return UI{}, fmt.Errorf("load UI: inventory is nil")
	}
	if source == nil {
		return UI{}, fmt.Errorf("load UI: source is nil")
	}
	document, err := assetmeta.Load[uiMetadata, struct{}, struct{}](reader, inventory)
	if err != nil {
		return UI{}, fmt.Errorf("load UI: %w", err)
	}
	packs := slices.Clone(document.Metadata.Packs)
	slices.SortFunc(packs, func(a, b Pack) int { return strings.Compare(a.Source, b.Source) })
	if len(packs) != 2 {
		return UI{}, fmt.Errorf("load UI: packs = %d, want 2", len(packs))
	}
	want := map[string]Pack{
		"heroicons": {
			Source: "heroicons", Alias: "hi", Release: heroiconsRelease, Revision: heroiconsRevision,
			Repository: "https://github.com/tailwindlabs/heroicons", SourceRef: assetmeta.Ref{Resource: "heroicons", Download: "optimized"},
			License: "MIT", LicenseRef: assetmeta.Ref{Resource: "heroicons", Download: "license"}, Namespace: "ui",
			Variants: []string{"16-solid", "20-solid", "24-outline", "24-solid"},
		},
		"developer-icons": {
			Source: "developer-icons", Alias: "devicon", Release: developerIconsRelease, Revision: developerIconsRevision,
			Repository: "https://github.com/xandemon/developer-icons", SourceRef: assetmeta.Ref{Resource: "developer-icons", Download: "icons"},
			License: "MIT", LicenseRef: assetmeta.Ref{Resource: "developer-icons", Download: "license"}, Namespace: "brand",
			Variants: []string{"default", "dark", "light"},
		},
	}
	seen := make(map[string]struct{}, len(packs))
	for _, pack := range packs {
		expected, ok := want[pack.Source]
		if !ok || !equalPack(pack, expected) {
			return UI{}, fmt.Errorf("load UI: invalid pack metadata for %q", pack.Source)
		}
		if _, duplicate := seen[pack.Source]; duplicate {
			return UI{}, fmt.Errorf("load UI: duplicate pack %q", pack.Source)
		}
		seen[pack.Source] = struct{}{}
		if _, found := source.Directory(pack.SourceRef.Resource, pack.SourceRef.Download); !found {
			return UI{}, fmt.Errorf("load UI: missing source directory %s", pack.SourceRef.String())
		}
		if err := assetmeta.ValidateRefs(inventory, pack.LicenseRef); err != nil {
			return UI{}, fmt.Errorf("load UI: %s: %w", pack.LicenseRef.String(), err)
		}
		resource, ok := inventory.Resource(pack.Source)
		if !ok || resource.Version != pack.Release || len(resource.Downloads) != 1 {
			return UI{}, fmt.Errorf("load UI: acquisition resource %q does not match release %q", pack.Source, pack.Release)
		}
	}
	return UI{Packs: packs}, nil
}

func equalPack(a, b Pack) bool {
	return a.Source == b.Source && a.Alias == b.Alias && a.Release == b.Release && a.Revision == b.Revision &&
		a.Repository == b.Repository && a.SourceRef == b.SourceRef && a.License == b.License &&
		a.LicenseRef == b.LicenseRef && a.Namespace == b.Namespace && slices.Equal(a.Variants, b.Variants)
}
