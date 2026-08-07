// Package provenance turns locked third-party source directories into deterministic,
// language-neutral icon artifacts, sprites, licenses, and provenance documents.
package provenance

import (
	"bytes"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/acquisition"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/sprite"
)

// Record and Source are the narrow generated-acquisition boundary.
type Record = acquisition.Record
type Source = acquisition.Source

// Result contains deterministic dist-relative artifacts and catalog entries.
type Result struct {
	Files  map[string][]byte
	Assets []catalog.Asset
}

type builtPack struct {
	Files   map[string][]byte
	Assets  []catalog.Asset
	Entries []sprite.Entry
}

// BuildUI reads only SHA-384-locked files and performs no network I/O.
func BuildUI(source Source, ui UI) (Result, error) {
	if source == nil {
		return Result{}, fmt.Errorf("build UI: source is nil")
	}
	files := make(map[string][]byte)
	assets := make([]catalog.Asset, 0, heroiconsExpectedAssetCount+developerExpectedAssetCount)
	packs := slices.Clone(ui.Packs)
	slices.SortFunc(packs, func(a, b Pack) int { return strings.Compare(a.Source, b.Source) })
	for _, pack := range packs {
		built, err := buildPack(source, pack)
		if err != nil {
			return Result{}, err
		}
		for path, data := range built.Files {
			if _, duplicate := files[path]; duplicate {
				return Result{}, fmt.Errorf("build UI: duplicate artifact %q", path)
			}
			files[path] = data
		}
		assets = append(assets, built.Assets...)
		spriteBytes, err := sprite.Build(built.Entries)
		if err != nil {
			return Result{}, fmt.Errorf("build UI: %s sprite: %w", pack.Source, err)
		}
		files[spritePath(pack.Source)] = spriteBytes
	}
	slices.SortFunc(assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) })
	return Result{Files: files, Assets: assets}, nil
}

func buildPack(source Source, pack Pack) (builtPack, error) {
	directory, entries, err := readLockedDirectory(source, pack.SourceRef.Resource, pack.SourceRef.Download)
	if err != nil {
		return builtPack{}, fmt.Errorf("build UI: %s source directory: %w", pack.Source, err)
	}
	built, provenanceAssets, err := normalizePack(pack, entries)
	if err != nil {
		return builtPack{}, fmt.Errorf("build UI: %s: %w", pack.Source, err)
	}
	_, license, err := readLocked(source, pack.LicenseRef.Resource, pack.LicenseRef.Download)
	if err != nil {
		return builtPack{}, fmt.Errorf("build UI: %s license: %w", pack.Source, err)
	}
	built.Files[licensePath(pack.Source)] = license
	provenanceBytes, err := json.MarshalIndent(provenanceDocument(pack, directory, provenanceAssets), "", "  ")
	if err != nil {
		return builtPack{}, fmt.Errorf("build UI: %s provenance: %w", pack.Source, err)
	}
	built.Files[provenancePath(pack.Source)] = append(provenanceBytes, '\n')
	return built, nil
}

func normalizePack(pack Pack, entries []archiveEntry) (builtPack, []provenanceAsset, error) {
	result := builtPack{Files: make(map[string][]byte)}
	var provenanceAssets []provenanceAsset
	seenCanonical := make(map[string]struct{})
	seenSymbols := make(map[string]struct{})
	for _, entry := range entries {
		asset, sourcePath, symbol, colorBehavior, ok, err := packAsset(pack, entry)
		if err != nil {
			return builtPack{}, nil, err
		}
		if !ok {
			continue
		}
		if _, duplicate := seenCanonical[asset.CanonicalName]; duplicate {
			return builtPack{}, nil, fmt.Errorf("duplicate canonical name %q", asset.CanonicalName)
		}
		if _, duplicate := seenSymbols[symbol]; duplicate {
			return builtPack{}, nil, fmt.Errorf("duplicate sprite symbol %q", symbol)
		}
		seenCanonical[asset.CanonicalName] = struct{}{}
		seenSymbols[symbol] = struct{}{}
		normalized, document, err := normalizeSourceSVG(entry.data, symbol, colorBehavior)
		if err != nil {
			return builtPack{}, nil, fmt.Errorf("normalize %s: %w", sourcePath, err)
		}
		asset.Dimensions = catalog.Dimensions{ViewBox: document.ViewBox()}
		asset.SHA256 = checksum(normalized)
		result.Files[asset.Path] = normalized
		result.Assets = append(result.Assets, asset)
		result.Entries = append(result.Entries, sprite.Entry{Symbol: symbol, SVG: normalized})
		provenanceAssets = append(provenanceAssets, provenanceAsset{
			CanonicalName: asset.CanonicalName, SpriteSymbol: symbol, SourcePath: sourcePath,
			ArtifactPath: asset.Path, SHA256: asset.SHA256,
		})
	}
	want := heroiconsExpectedAssetCount
	if pack.Source == "developer-icons" {
		want = developerExpectedAssetCount
	}
	if len(result.Assets) != want {
		return builtPack{}, nil, fmt.Errorf("source directory assets = %d, want %d", len(result.Assets), want)
	}
	slices.SortFunc(result.Assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) })
	slices.SortFunc(result.Entries, func(a, b sprite.Entry) int { return strings.Compare(a.Symbol, b.Symbol) })
	slices.SortFunc(provenanceAssets, func(a, b provenanceAsset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) })
	return result, provenanceAssets, nil
}

func packAsset(pack Pack, entry archiveEntry) (catalog.Asset, string, string, string, bool, error) {
	switch pack.Source {
	case "heroicons":
		parts := strings.Split(entry.path, "/")
		if len(parts) != 4 || parts[0] != "optimized" || !strings.HasSuffix(parts[3], ".svg") {
			return catalog.Asset{}, "", "", "", false, nil
		}
		variant := parts[1] + "-" + parts[2]
		if !slices.Contains(pack.Variants, variant) {
			return catalog.Asset{}, "", "", "", false, nil
		}
		name := strings.TrimSuffix(parts[3], ".svg")
		symbol := "hi-" + variant + "-" + name
		path := "icons/ui/heroicons/" + variant + "-" + name + ".svg"
		return catalog.Asset{
			CanonicalName: "ui-" + symbol, Namespace: "ui", Path: path, Product: "heroicons",
			Artwork: "icon", Appearance: variant, Surface: "transparent", Framing: "optical", Format: "svg",
			SpriteSymbol: symbol, ColorBehavior: "monochrome", License: pack.License,
			Source: "tailwindlabs/heroicons@" + pack.Revision + ":" + entry.path,
		}, entry.path, symbol, "monochrome", true, nil
	case "developer-icons":
		if !strings.HasPrefix(entry.path, "icons/") || !strings.HasSuffix(entry.path, ".svg") {
			return catalog.Asset{}, "", "", "", false, nil
		}
		name := strings.TrimSuffix(strings.TrimPrefix(entry.path, "icons/"), ".svg")
		if name == "" || strings.Contains(name, "/") || name == "developer-icons" {
			return catalog.Asset{}, "", "", "", false, nil
		}
		variant := developerVariant(name)
		if !slices.Contains(pack.Variants, variant) {
			return catalog.Asset{}, "", "", "", false, fmt.Errorf("unsupported variant %q for %s", variant, entry.path)
		}
		// Canonical name preserves upstream spelling. Sprite symbol is a separate,
		// downstream-safe SVG identifier; tRPC therefore maps to devicon-trpc.
		canonicalName := "brand-developer-icons-" + name
		symbol := "devicon-" + strings.ToLower(name)
		path := "icons/brand/developer-icons/" + name + ".svg"
		return catalog.Asset{
			CanonicalName: canonicalName, Namespace: "brand", Path: path, Product: "developer-icons",
			Artwork: "icon", Appearance: variant, Surface: "transparent", Framing: "optical", Format: "svg",
			SpriteSymbol: symbol, ColorBehavior: "protected", License: pack.License,
			Source: "xandemon/developer-icons@" + pack.Revision + ":" + entry.path,
		}, entry.path, symbol, "protected", true, nil
	default:
		return catalog.Asset{}, "", "", "", false, fmt.Errorf("unsupported pack %q", pack.Source)
	}
}

func developerVariant(name string) string {
	switch {
	case strings.HasSuffix(name, "-dark"):
		return "dark"
	case strings.HasSuffix(name, "-light"):
		return "light"
	default:
		return "default"
	}
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func readLocked(source Source, resource, download string) (Record, []byte, error) {
	record, ok := source.Lookup(resource, download)
	if !ok {
		return Record{}, nil, fmt.Errorf("missing acquisition record %s/%s", resource, download)
	}
	file, err := source.Open(resource, download)
	if err != nil {
		return Record{}, nil, fmt.Errorf("open %s/%s: %w", resource, download, err)
	}
	data, readErr := io.ReadAll(file)
	closeErr := file.Close()
	if readErr != nil {
		return Record{}, nil, fmt.Errorf("read %s/%s: %w", resource, download, readErr)
	}
	if closeErr != nil {
		return Record{}, nil, fmt.Errorf("close %s/%s: %w", resource, download, closeErr)
	}
	if got := sha384Hash(data); got != record.Hash {
		return Record{}, nil, fmt.Errorf("hash mismatch for %s/%s: got %s, want %s", resource, download, got, record.Hash)
	}
	return record, data, nil
}

func readLockedDirectory(source Source, resource, directory string) (acquisition.LockedDirectory, []archiveEntry, error) {
	locked, ok := source.Directory(resource, directory)
	if !ok {
		return acquisition.LockedDirectory{}, nil, fmt.Errorf("missing acquisition directory %s/%s", resource, directory)
	}
	entries := make([]archiveEntry, 0, len(locked.Files))
	for _, member := range locked.Files {
		file, err := source.OpenPath(member.Path)
		if err != nil {
			return acquisition.LockedDirectory{}, nil, fmt.Errorf("open %s: %w", member.Path, err)
		}
		data, readErr := io.ReadAll(io.LimitReader(file, member.Size+1))
		closeErr := file.Close()
		if readErr != nil {
			return acquisition.LockedDirectory{}, nil, fmt.Errorf("read %s: %w", member.Path, readErr)
		}
		if closeErr != nil {
			return acquisition.LockedDirectory{}, nil, fmt.Errorf("close %s: %w", member.Path, closeErr)
		}
		if int64(len(data)) != member.Size {
			return acquisition.LockedDirectory{}, nil, fmt.Errorf("size mismatch for %s: got %d, want %d", member.Path, len(data), member.Size)
		}
		if got := sha384SRI(data); got != member.Integrity {
			return acquisition.LockedDirectory{}, nil, fmt.Errorf("hash mismatch for %s: got %s, want %s", member.Path, got, member.Integrity)
		}
		entries = append(entries, archiveEntry{path: member.Source, data: data})
	}
	return locked, entries, nil
}

func sha384Hash(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384:" + hex.EncodeToString(sum[:])
}

func sha384SRI(data []byte) string {
	sum := sha512.Sum384(data)
	return "sha384-" + base64.StdEncoding.EncodeToString(sum[:])
}

func sriHash(integrity string) string {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(integrity, "sha384-"))
	if err != nil || len(raw) != sha512.Size384 {
		return ""
	}
	return "sha384:" + hex.EncodeToString(raw)
}

func spritePath(source string) string {
	if source == "heroicons" {
		return "icons/ui/sprite.svg"
	}
	return "icons/brand/developer-icons/sprite.svg"
}

func licensePath(source string) string { return "licenses/" + source + "-MIT.txt" }

func provenancePath(source string) string {
	if source == "heroicons" {
		return "icons/ui/heroicons/provenance.json"
	}
	return "icons/brand/developer-icons/provenance.json"
}

type provenance struct {
	Source     string            `json:"source"`
	Alias      string            `json:"alias"`
	Release    string            `json:"release"`
	Revision   string            `json:"revision"`
	Repository string            `json:"repository"`
	License    string            `json:"license"`
	LicenseRef string            `json:"licenseRef"`
	SourceRef  string            `json:"sourceRef"`
	SourceURL  string            `json:"sourceUrl"`
	SourcePath string            `json:"sourcePath"`
	Integrity  string            `json:"integrity"`
	Hash       string            `json:"hash"`
	Variants   []string          `json:"variants"`
	AssetCount int               `json:"assetCount"`
	Assets     []provenanceAsset `json:"assets"`
}

type provenanceAsset struct {
	CanonicalName string `json:"canonicalName"`
	SpriteSymbol  string `json:"spriteSymbol"`
	SourcePath    string `json:"sourcePath"`
	ArtifactPath  string `json:"artifactPath"`
	SHA256        string `json:"sha256"`
}

func provenanceDocument(pack Pack, directory acquisition.LockedDirectory, assets []provenanceAsset) provenance {
	return provenance{
		Source: pack.Source, Alias: pack.Alias, Release: pack.Release, Revision: pack.Revision,
		Repository: pack.Repository, License: pack.License, LicenseRef: pack.LicenseRef.String(),
		SourceRef: pack.SourceRef.String(), SourceURL: directory.URL, SourcePath: directory.Path,
		Integrity: directory.Integrity, Hash: sriHash(directory.Integrity), Variants: slices.Clone(pack.Variants),
		AssetCount: len(assets), Assets: assets,
	}
}

// ensure imported bytes remains tied to deterministic byte comparisons in tests.
var _ = bytes.Equal
