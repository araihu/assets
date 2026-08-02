package provenance

import (
	"fmt"
	"io"
	"io/fs"
	"strings"

	"github.com/araihu/assets/assetmeta"
)

const heroiconsVersion = "v2.2.0"

type uiMetadata struct {
	Source     string        `yaml:"source"`
	Alias      string        `yaml:"alias"`
	Version    string        `yaml:"version"`
	License    string        `yaml:"license"`
	LicenseRef assetmeta.Ref `yaml:"license_ref"`
	Icons      []Icon        `yaml:"icons"`
}

// UI contains resolved consumer semantics for the locked Heroicons inventory.
type UI struct {
	Source     string
	Alias      string
	Version    string
	License    string
	LicenseRef assetmeta.Ref
	Icons      []Icon
}

// Icon associates a stable semantic path with one acquired download.
type Icon struct {
	Ref  assetmeta.Ref `yaml:"ref"`
	Path string        `yaml:"path"`
}

// LoadUI strictly resolves the UI metadata overlay against an acquisition inventory.
func LoadUI(reader io.Reader, inventory *assetmeta.Inventory) (UI, error) {
	if inventory == nil {
		return UI{}, fmt.Errorf("load UI: inventory is nil")
	}
	resources := inventory.Resources()
	if len(resources) != 1 || resources[0].Name != "heroicons" {
		return UI{}, fmt.Errorf("load UI: inventory must contain only heroicons")
	}
	resource := resources[0]
	if resource.Version != heroiconsVersion {
		return UI{}, fmt.Errorf("load UI: heroicons version = %q, want %q", resource.Version, heroiconsVersion)
	}

	document, err := assetmeta.Load[uiMetadata, struct{}, struct{}](reader, inventory)
	if err != nil {
		return UI{}, fmt.Errorf("load UI: %w", err)
	}
	metadata := document.Metadata
	if metadata.Source != "heroicons" || metadata.Alias != "hi" || metadata.Version != heroiconsVersion || metadata.License != "MIT" {
		return UI{}, fmt.Errorf("load UI: metadata must describe Heroicons %s as hi under MIT", heroiconsVersion)
	}
	if metadata.LicenseRef.String() != "heroicons/license" {
		return UI{}, fmt.Errorf("load UI: license_ref = %q, want heroicons/license", metadata.LicenseRef.String())
	}
	if err := assetmeta.ValidateRefs(inventory, metadata.LicenseRef); err != nil {
		return UI{}, fmt.Errorf("load UI: license: %w", err)
	}

	expected := make(map[string]string, len(resource.Downloads)-1)
	const pathPrefix = "internal/acquisition/vendor/heroicons/v2.2.0/"
	for _, download := range resource.Downloads {
		if download.Name == "license" {
			continue
		}
		if !strings.HasPrefix(download.Path, pathPrefix) {
			return UI{}, fmt.Errorf("load UI: download %q has unexpected path %q", download.Name, download.Path)
		}
		expected[download.Name] = strings.TrimPrefix(download.Path, pathPrefix)
	}
	if len(metadata.Icons) != len(expected) {
		return UI{}, fmt.Errorf("load UI: icons = %d, want %d", len(metadata.Icons), len(expected))
	}

	seenRefs := make(map[string]struct{}, len(metadata.Icons))
	seenPaths := make(map[string]struct{}, len(metadata.Icons))
	icons := make([]Icon, 0, len(metadata.Icons))
	for _, icon := range metadata.Icons {
		ref := icon.Ref.String()
		if _, ok := seenRefs[ref]; ok {
			return UI{}, fmt.Errorf("load UI: duplicate icon ref %q", ref)
		}
		seenRefs[ref] = struct{}{}
		if _, ok := seenPaths[icon.Path]; ok {
			return UI{}, fmt.Errorf("load UI: duplicate icon path %q", icon.Path)
		}
		seenPaths[icon.Path] = struct{}{}
		if icon.Ref.Resource != "heroicons" || icon.Ref.Download == "license" {
			return UI{}, fmt.Errorf("load UI: invalid icon ref %q", ref)
		}
		if !fs.ValidPath(icon.Path) || !strings.HasPrefix(icon.Path, "16/solid/") || !strings.HasSuffix(icon.Path, ".svg") {
			return UI{}, fmt.Errorf("load UI: invalid icon path %q", icon.Path)
		}
		resolved, ok := document.Resolve(icon.Ref)
		if !ok {
			return UI{}, fmt.Errorf("load UI: unknown icon ref %q", ref)
		}
		wantPath, ok := expected[icon.Ref.Download]
		if !ok || wantPath != icon.Path || !strings.HasSuffix(resolved.Download.Path, "/"+icon.Path) {
			return UI{}, fmt.Errorf("load UI: ref %q does not resolve to %q", ref, icon.Path)
		}
		delete(expected, icon.Ref.Download)
		icons = append(icons, icon)
	}
	if len(expected) != 0 {
		return UI{}, fmt.Errorf("load UI: overlay is missing %d acquired icons", len(expected))
	}

	return UI{
		Source: metadata.Source, Alias: metadata.Alias, Version: metadata.Version,
		License: metadata.License, LicenseRef: metadata.LicenseRef, Icons: icons,
	}, nil
}
