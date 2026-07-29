// Package transform derives deterministic brand variants from promoted masters.
package transform

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/sprite"
	"github.com/araihu/assets/internal/svgasset"
)

const (
	brandLicense = "Arai Hu Brand Terms"
	opticalRatio = 0.76
	safeRatio    = 66.0 / 108.0
)

var (
	rootStart      = regexp.MustCompile(`(?s)^\s*<svg\b[^>]*>`)
	rootDimensions = regexp.MustCompile(`\s(?:width="(?:1024|2048)"|height="(?:1024|508)")`)
	rootViewBox    = regexp.MustCompile(`viewBox="[^"]+"`)
	paint          = regexp.MustCompile(`(?i)(fill|stroke)="(#[0-9a-f]{6})"`)

	opticalViewBoxes = map[string]string{
		"araihu/icon":   "262.510 263.185 498.904 498.904",
		"goshtoso/icon": "237.162 237.080 550.558 550.558",
		"manja/icon":    "167.325 123.377 776.662 776.662",
		"paje/icon":     "154.532 155.566 715.652 715.652",
		"x9/icon":       "69.210 69.362 885.279 885.279",
		"araihu/logo":   "156.655 88.812 1781.202 330.139",
		"goshtoso/logo": "133.328 88.739 1801.022 330.718",
		"manja/logo":    "304.217 91.472 1440.066 325.893",
		"paje/logo":     "525.286 99.476 985.121 332.227",
		"x9/logo":       "209.911 48.626 1620.856 410.636",
	}

	palettes = map[string]palette{
		"adaptive":  {surface: "#f3f2e9", ink: "#07111f", signal: "#c7ff4a"},
		"light":     {surface: "#f3f2e9", ink: "#07111f", signal: "#c7ff4a"},
		"dark":      {surface: "#07111f", ink: "#f3f2e9", signal: "#c7ff4a"},
		"grayscale": {surface: "#e6e6e6", ink: "#202020", signal: "#707070"},
		"tinted":    {surface: "#e6e6e6", ink: "#202020", signal: "#707070"},
	}
)

type palette struct{ surface, ink, signal string }

type recipe struct {
	artwork, appearance, surface, framing, colorBehavior string
}

// Result contains all generated SVG files and catalog/sprite records for a brand build.
type Result struct {
	Files         map[string][]byte
	Assets        []catalog.Asset
	SpriteEntries []sprite.Entry
}

// BuildBrand creates every declared optical and launcher SVG identity variant.
func BuildBrand(fsys fs.FS, brand manifest.Brand) (Result, error) {
	if err := brand.Validate(); err != nil {
		return Result{}, fmt.Errorf("brand manifest: %w", err)
	}
	result := Result{Files: make(map[string][]byte)}
	for _, product := range brand.Products {
		for _, recipe := range productRecipes() {
			variant, err := buildVariant(fsys, product, recipe)
			if err != nil {
				return Result{}, err
			}
			if _, duplicate := result.Files[variant.path]; duplicate {
				return Result{}, fmt.Errorf("duplicate output path %q", variant.path)
			}
			result.Files[variant.path] = variant.svg
			result.Assets = append(result.Assets, variant.asset)
			result.SpriteEntries = append(result.SpriteEntries, sprite.Entry{Symbol: variant.asset.SpriteSymbol, SVG: variant.svg})
		}
	}
	slices.SortFunc(result.Assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) })
	slices.SortFunc(result.SpriteEntries, func(a, b sprite.Entry) int { return strings.Compare(a.Symbol, b.Symbol) })
	brandSprite, err := sprite.Build(result.SpriteEntries)
	if err != nil {
		return Result{}, fmt.Errorf("brand sprite: %w", err)
	}
	result.Files["dist/icons/brand/sprite.svg"] = brandSprite
	return result, nil
}

func productRecipes() []recipe {
	return []recipe{
		{artwork: "icon", appearance: "adaptive", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "adaptive", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "light", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "light", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "dark", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "dark", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "monochrome", surface: "transparent", framing: "optical", colorBehavior: "monochrome"},
		{artwork: "icon", appearance: "grayscale", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "grayscale", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "tinted", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "tinted", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "icon", appearance: "adaptive", surface: "plate", framing: "launcher", colorBehavior: "protected"},
		{artwork: "icon", appearance: "tinted", surface: "plate", framing: "launcher", colorBehavior: "protected"},
		{artwork: "logo", appearance: "adaptive", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "adaptive", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "light", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "light", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "dark", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "dark", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "monochrome", surface: "transparent", framing: "optical", colorBehavior: "monochrome"},
		{artwork: "logo", appearance: "grayscale", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "grayscale", surface: "plate", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "tinted", surface: "transparent", framing: "optical", colorBehavior: "protected"},
		{artwork: "logo", appearance: "tinted", surface: "plate", framing: "optical", colorBehavior: "protected"},
	}
}

type builtVariant struct {
	path  string
	svg   []byte
	asset catalog.Asset
}

func buildVariant(fsys fs.FS, product manifest.Product, recipe recipe) (builtVariant, error) {
	sourceKind := recipe.artwork + "-transparent"
	if recipe.surface == "plate" {
		sourceKind = recipe.artwork + "-background"
	}
	sourcePath := product.Sources["original"][sourceKind]
	source, err := fs.ReadFile(fsys, sourcePath)
	if err != nil {
		return builtVariant{}, fmt.Errorf("read %s: %w", sourcePath, err)
	}
	if actual := fmtHash(sha256.Sum256(source)); actual != product.SourceHashes[sourceKind] {
		return builtVariant{}, fmt.Errorf("source hash mismatch for %s: got %s", sourcePath, actual)
	}

	prepared := stripRootDimensions(source)
	prepared, err = setViewBox(prepared, product.ID, recipe.artwork, recipe.surface, recipe.framing)
	if err != nil {
		return builtVariant{}, err
	}
	if recipe.appearance != "monochrome" {
		prepared, err = applyPalette(prepared, recipe.appearance)
		if err != nil {
			return builtVariant{}, err
		}
	}
	doc, err := svgasset.Parse(prepared)
	if err != nil {
		return builtVariant{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}
	normalized, err := doc.Normalize(svgasset.Options{ColorBehavior: recipe.colorBehavior})
	if err != nil {
		return builtVariant{}, fmt.Errorf("normalize %s: %w", sourcePath, err)
	}

	canonical := strings.Join([]string{product.ID, recipe.artwork, recipe.appearance, recipe.surface, recipe.framing}, "-")
	path := variantPath(product.ID, recipe)
	viewBox := svgassetViewBox(normalized)
	asset := catalog.Asset{
		CanonicalName: canonical, Namespace: "brand", Path: strings.TrimPrefix(path, "dist/"), Product: product.ID,
		Artwork: recipe.artwork, Appearance: recipe.appearance, Surface: recipe.surface, Framing: recipe.framing,
		Format: "svg", Dimensions: catalog.Dimensions{ViewBox: viewBox}, SpriteSymbol: canonical,
		ColorBehavior: recipe.colorBehavior, License: brandLicense, Source: sourcePath + "#sha256=" + product.SourceHashes[sourceKind], SHA256: fmtHash(sha256.Sum256(normalized)),
	}
	return builtVariant{path: path, svg: normalized, asset: asset}, nil
}

func variantPath(product string, recipe recipe) string {
	name := recipe.appearance + "-" + recipe.surface + "-" + recipe.framing + ".svg"
	if recipe.artwork == "icon" {
		return "dist/icons/brand/" + product + "-icon-" + name
	}
	return "dist/brand/" + product + "/logo/" + name
}

func setViewBox(svg []byte, product, artwork, surface, framing string) ([]byte, error) {
	key := product + "/" + artwork
	viewBox, ok := opticalViewBoxes[key]
	if !ok {
		return nil, fmt.Errorf("missing optical viewBox for %s", key)
	}
	if surface == "plate" && framing == "optical" {
		if artwork == "icon" {
			viewBox = "0 0 1024 1024"
		} else {
			viewBox = "0 0 2048 508"
		}
	}
	if framing == "launcher" {
		if artwork != "icon" || surface != "plate" {
			return nil, fmt.Errorf("launcher framing requires a plated icon")
		}
		viewBox = launcherViewBox(viewBox)
	}
	if !rootViewBox.Match(svg) {
		return nil, fmt.Errorf("source SVG is missing a root viewBox")
	}
	return rootViewBox.ReplaceAll(svg, []byte(`viewBox="`+viewBox+`"`)), nil
}

func stripRootDimensions(svg []byte) []byte {
	return rootStart.ReplaceAllFunc(svg, func(root []byte) []byte {
		return rootDimensions.ReplaceAll(root, nil)
	})
}

func launcherViewBox(optical string) string {
	var x, y, size float64
	_, _ = fmt.Sscanf(optical, "%f %f %f", &x, &y, &size)
	expanded := size * opticalRatio / safeRatio
	x -= (expanded - size) / 2
	y -= (expanded - size) / 2
	return fmt.Sprintf("%.6f %.6f %.6f %.6f", x, y, expanded, expanded)
}

func applyPalette(svg []byte, appearance string) ([]byte, error) {
	p, ok := palettes[appearance]
	if !ok {
		return nil, fmt.Errorf("missing semantic palette %q", appearance)
	}
	return paint.ReplaceAllFunc(svg, func(match []byte) []byte {
		parts := paint.FindSubmatch(match)
		return []byte(string(parts[1]) + `="` + p.colorFor(string(parts[2])) + `"`)
	}), nil
}

func (p palette) colorFor(value string) string {
	value = strings.TrimPrefix(strings.ToLower(value), "#")
	var r, g, b uint8
	_, _ = fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b)
	if r > 150 && g > 180 && b < 140 {
		return p.signal
	}
	if r > 150 && g > 150 && b > 150 {
		return p.surface
	}
	return p.ink
}

func svgassetViewBox(svg []byte) string {
	match := rootViewBox.Find(svg)
	return strings.TrimSuffix(strings.TrimPrefix(string(match), `viewBox="`), `"`)
}

func fmtHash(sum [sha256.Size]byte) string {
	return fmt.Sprintf("%x", sum)
}
