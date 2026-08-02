// Package provenance vendors locked third-party UI-icon sources and builds
// deterministic UI artifacts from tracked vendor bytes.
package provenance

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/acquisition"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/sprite"
	"github.com/araihu/assets/internal/svgasset"
)

// Record and Source are the narrow generated-acquisition boundary.
type Record = acquisition.Record
type Source = acquisition.Source

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Result contains deterministic dist-relative UI artifacts and catalog entries.
type Result struct {
	Files  map[string][]byte
	Assets []catalog.Asset
}

// BuildUI reads only embedded, locked inputs and returns normalized UI artifacts.
func BuildUI(source Source, ui UI) (Result, error) {
	if source == nil {
		return Result{}, fmt.Errorf("build ui: source is nil")
	}
	files := make(map[string][]byte, len(ui.Icons)+3)
	assets := make([]catalog.Asset, 0, len(ui.Icons))
	entries := make([]sprite.Entry, 0, len(ui.Icons))
	icons := slices.Clone(ui.Icons)
	slices.SortFunc(icons, func(a, b Icon) int { return strings.Compare(a.Path, b.Path) })
	locked := make([]lockedIcon, 0, len(icons))
	for _, icon := range icons {
		record, ok := source.Lookup(icon.Ref.Resource, icon.Ref.Download)
		if !ok {
			return Result{}, fmt.Errorf("build ui: missing acquisition record %s", icon.Ref.String())
		}
		file, err := source.Open(icon.Ref.Resource, icon.Ref.Download)
		if err != nil {
			return Result{}, fmt.Errorf("build ui: open %s: %w", icon.Ref.String(), err)
		}
		raw, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return Result{}, fmt.Errorf("build ui: read %s: %w", icon.Ref.String(), readErr)
		}
		if closeErr != nil {
			return Result{}, fmt.Errorf("build ui: close %s: %w", icon.Ref.String(), closeErr)
		}
		if got := sha384Hash(raw); got != record.Hash {
			return Result{}, fmt.Errorf("build ui: hash mismatch for %s: got %s, want %s", icon.Ref.String(), got, record.Hash)
		}
		document, err := svgasset.Parse(stripRootDimensions(raw))
		if err != nil {
			return Result{}, fmt.Errorf("build ui: validate %s: %w", icon.Path, err)
		}
		normalized, err := document.Normalize(svgasset.Options{ColorBehavior: "monochrome"})
		if err != nil {
			return Result{}, fmt.Errorf("build ui: normalize %s: %w", icon.Path, err)
		}
		if _, err := svgasset.Parse(normalized); err != nil {
			return Result{}, fmt.Errorf("build ui: validate generated %s: %w", icon.Path, err)
		}
		name := canonicalName(icon.Path)
		outputPath := "icons/ui/heroicons/" + name + ".svg"
		symbol := "hi-" + name
		files[outputPath] = normalized
		entries = append(entries, sprite.Entry{Symbol: symbol, SVG: normalized})
		assets = append(assets, catalog.Asset{
			CanonicalName: "ui-" + symbol,
			Namespace:     "ui", Path: outputPath, Product: "heroicons", Artwork: "icon",
			Appearance: "default", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: document.ViewBox()}, SpriteSymbol: symbol,
			ColorBehavior: "monochrome", License: "MIT", Source: "heroicons@v2.2.0", SHA256: checksum(normalized),
		})
		locked = append(locked, lockedIcon{
			Ref: icon.Ref.String(), Path: icon.Path, URL: record.URL,
			AcquisitionPath: record.Path, Integrity: record.Integrity, Hash: record.Hash,
		})
	}
	spriteBytes, err := sprite.Build(entries)
	if err != nil {
		return Result{}, fmt.Errorf("build ui: sprite: %w", err)
	}
	files["icons/ui/sprite.svg"] = spriteBytes
	license, err := readLocked(source, ui.LicenseRef.Resource, ui.LicenseRef.Download)
	if err != nil {
		return Result{}, fmt.Errorf("build ui: license: %w", err)
	}
	files["licenses/heroicons-MIT.txt"] = license
	provenanceBytes, err := json.MarshalIndent(provenanceDocument(ui, locked), "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("build ui: provenance: %w", err)
	}
	files["icons/ui/heroicons/provenance.json"] = append(provenanceBytes, '\n')
	return Result{Files: files, Assets: assets}, nil
}

func readLocked(source Source, resource, download string) ([]byte, error) {
	record, ok := source.Lookup(resource, download)
	if !ok {
		return nil, fmt.Errorf("missing acquisition record %s/%s", resource, download)
	}
	file, err := source.Open(resource, download)
	if err != nil {
		return nil, fmt.Errorf("open %s/%s: %w", resource, download, err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return nil, fmt.Errorf("read %s/%s: %w", resource, download, readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close %s/%s: %w", resource, download, closeErr)
	}
	if got := sha384Hash(data); got != record.Hash {
		return nil, fmt.Errorf("hash mismatch for %s/%s: got %s, want %s", resource, download, got, record.Hash)
	}
	return data, nil
}

func sha384Hash(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384:" + hex.EncodeToString(sum[:])
}

func canonicalName(sourcePath string) string {
	parts := strings.Split(strings.TrimSuffix(sourcePath, ".svg"), "/")
	return strings.Join(parts, "-")
}

var rootDimension = regexp.MustCompile(`\s(?:width|height)=(?:"[^"]*"|'[^']*')`)

func stripRootDimensions(raw []byte) []byte {
	end := bytes.IndexByte(raw, '>')
	if end < 0 {
		return raw
	}
	result := append([]byte(nil), raw...)
	result = append(rootDimension.ReplaceAll(result[:end], nil), result[end:]...)
	return result
}

type provenance struct {
	Source     string       `json:"source"`
	Alias      string       `json:"alias"`
	Version    string       `json:"version"`
	License    string       `json:"license"`
	LicenseRef string       `json:"licenseRef"`
	Icons      []lockedIcon `json:"icons"`
}

type lockedIcon struct {
	Ref             string `json:"ref"`
	Path            string `json:"path"`
	URL             string `json:"url"`
	AcquisitionPath string `json:"acquisitionPath"`
	Integrity       string `json:"integrity"`
	Hash            string `json:"hash"`
}

func provenanceDocument(ui UI, icons []lockedIcon) provenance {
	return provenance{
		Source: ui.Source, Alias: ui.Alias, Version: ui.Version, License: ui.License,
		LicenseRef: ui.LicenseRef.String(), Icons: icons,
	}
}
