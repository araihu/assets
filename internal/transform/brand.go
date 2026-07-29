// Package transform derives deterministic brand variants from promoted masters.
package transform

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
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
	hexColor       = regexp.MustCompile(`(?i)^#[0-9a-f]{6}$`)

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
		"light":     {surface: "#f3f2e9", ink: "#07111f", signal: "#c7ff4a"},
		"dark":      {surface: "#07111f", ink: "#f3f2e9", signal: "#c7ff4a"},
		"grayscale": {surface: "#e6e6e6", ink: "#202020", signal: "#707070"},
		"tinted":    {surface: "#d5ddeb", ink: "#31588f", signal: "#07111f"},
	}
)

const adaptiveStyle = `<style>@media (prefers-color-scheme: dark) {:root {--araihu-logo-auto-surface: #07111f;--araihu-logo-auto-ink: #f3f2e9;--araihu-logo-auto-signal: #c7ff4a;}}</style>`

type palette struct{ surface, ink, signal string }

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
		for _, recipe := range brand.Recipes {
			variant, err := buildVariant(fsys, product, recipe)
			if err != nil {
				return Result{}, err
			}
			if _, duplicate := result.Files[variant.path]; duplicate {
				return Result{}, fmt.Errorf("duplicate output path %q", variant.path)
			}
			result.Files[variant.path] = variant.svg
			result.Assets = append(result.Assets, variant.asset)
			if variant.asset.SpriteSymbol != "" {
				result.SpriteEntries = append(result.SpriteEntries, sprite.Entry{Symbol: variant.asset.SpriteSymbol, SVG: variant.svg})
			}
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

type builtVariant struct {
	path  string
	svg   []byte
	asset catalog.Asset
}

func buildVariant(fsys fs.FS, product manifest.Product, recipe manifest.BrandRecipe) (builtVariant, error) {
	sourceKind := recipe.Artwork + "-transparent"
	if recipe.Surface == "plate" {
		sourceKind = recipe.Artwork + "-background"
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
	prepared, err = setViewBox(prepared, product.ID, recipe.Artwork, recipe.Surface, recipe.Framing)
	if err != nil {
		return builtVariant{}, err
	}
	if recipe.Appearance == "adaptive" {
		prepared, err = applyAdaptivePalette(prepared)
		if err != nil {
			return builtVariant{}, fmt.Errorf("semanticize adaptive %s: %w", sourcePath, err)
		}
	} else if recipe.Appearance != "monochrome" {
		prepared, err = applyPalette(prepared, recipe.Appearance)
		if err != nil {
			return builtVariant{}, err
		}
	}
	doc, err := svgasset.Parse(prepared)
	if err != nil {
		return builtVariant{}, fmt.Errorf("parse %s: %w", sourcePath, err)
	}
	normalized, err := doc.Normalize(svgasset.Options{ColorBehavior: recipe.ColorBehavior})
	if err != nil {
		return builtVariant{}, fmt.Errorf("normalize %s: %w", sourcePath, err)
	}

	if recipe.Appearance == "adaptive" {
		normalized = injectAdaptiveStyle(normalized)
	}

	canonical := strings.Join([]string{product.ID, recipe.Artwork, recipe.Appearance, recipe.Surface, recipe.Framing}, "-")
	path := variantPath(product.ID, recipe)
	viewBox := svgassetViewBox(normalized)
	spriteSymbol := canonical
	if recipe.Appearance == "adaptive" {
		spriteSymbol = ""
	}
	asset := catalog.Asset{
		CanonicalName: canonical, Namespace: "brand", Path: strings.TrimPrefix(path, "dist/"), Product: product.ID,
		Artwork: recipe.Artwork, Appearance: recipe.Appearance, Surface: recipe.Surface, Framing: recipe.Framing,
		Format: "svg", Dimensions: catalog.Dimensions{ViewBox: viewBox}, SpriteSymbol: spriteSymbol,
		ColorBehavior: recipe.ColorBehavior, License: brandLicense, Source: sourcePath + "#sha256=" + product.SourceHashes[sourceKind], SHA256: fmtHash(sha256.Sum256(normalized)),
	}
	return builtVariant{path: path, svg: normalized, asset: asset}, nil
}

func variantPath(product string, recipe manifest.BrandRecipe) string {
	name := recipe.Appearance + "-" + recipe.Surface + "-" + recipe.Framing + ".svg"
	if recipe.Artwork == "icon" {
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

func applyAdaptivePalette(svg []byte) ([]byte, error) {
	decoder := xml.NewDecoder(bytes.NewReader(svg))
	var output bytes.Buffer
	encoder := xml.NewEncoder(&output)
	defaultFill := adaptivePaint("ink")
	type state struct {
		excluded bool
		fill     string
		stroke   string
	}
	stack := make([]state, 0, 8)
	for {
		token, err := decoder.RawToken()
		if errors.Is(err, io.EOF) {
			if err := encoder.Flush(); err != nil {
				return nil, fmt.Errorf("flush adaptive SVG: %w", err)
			}
			return output.Bytes(), nil
		}
		if err != nil {
			return nil, fmt.Errorf("decode adaptive SVG: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			parent := state{fill: defaultFill, stroke: "none"}
			if len(stack) != 0 {
				parent = stack[len(stack)-1]
			}
			local := token.Name.Local
			current := state{excluded: parent.excluded || nonPaintingContext(local), fill: parent.fill, stroke: parent.stroke}
			hasFill := false
			hasStroke := false
			for index := range token.Attr {
				attr := &token.Attr[index]
				switch attr.Name.Local {
				case "fill":
					hasFill = true
					if !current.excluded && hexColor.MatchString(attr.Value) {
						attr.Value = adaptivePaint(colorRole(attr.Value))
					}
					current.fill = attr.Value
				case "stroke":
					hasStroke = true
					if !current.excluded && hexColor.MatchString(attr.Value) {
						attr.Value = adaptivePaint(colorRole(attr.Value))
					}
					current.stroke = attr.Value
				}
			}
			if !current.excluded && visualGeometry(local) {
				if local != "line" && !hasFill {
					token.Attr = append(token.Attr, xml.Attr{Name: xml.Name{Local: "fill"}, Value: current.fill})
				}
				if !hasStroke && !paintIsNone(current.stroke) {
					token.Attr = append(token.Attr, xml.Attr{Name: xml.Name{Local: "stroke"}, Value: current.stroke})
				}
			}
			stack = append(stack, current)
			if err := encoder.EncodeToken(token); err != nil {
				return nil, fmt.Errorf("encode adaptive SVG start: %w", err)
			}
		case xml.EndElement:
			if err := encoder.EncodeToken(token); err != nil {
				return nil, fmt.Errorf("encode adaptive SVG end: %w", err)
			}
			stack = stack[:len(stack)-1]
		default:
			if err := encoder.EncodeToken(token); err != nil {
				return nil, fmt.Errorf("encode adaptive SVG: %w", err)
			}
		}
	}
}

func adaptivePaint(role string) string {
	fallback := palettes["light"].color(role)
	return fmt.Sprintf("var(--araihu-logo-%s, var(--araihu-logo-auto-%s, %s))", role, role, fallback)
}

func nonPaintingContext(local string) bool {
	return local == "defs" || local == "clipPath" || local == "mask"
}

func visualGeometry(local string) bool {
	switch local {
	case "circle", "ellipse", "line", "path", "polygon", "polyline", "rect", "use":
		return true
	default:
		return false
	}
}

func paintIsNone(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "none")
}

func injectAdaptiveStyle(svg []byte) []byte {
	rootEnd := strings.IndexByte(string(svg), '>')
	if rootEnd < 0 {
		return svg
	}
	result := make([]byte, 0, len(svg)+len(adaptiveStyle))
	result = append(result, svg[:rootEnd+1]...)
	result = append(result, adaptiveStyle...)
	result = append(result, svg[rootEnd+1:]...)
	return result
}

func (p palette) colorFor(value string) string {
	return p.color(colorRole(value))
}

func (p palette) color(role string) string {
	switch role {
	case "signal":
		return p.signal
	case "surface":
		return p.surface
	default:
		return p.ink
	}
}

func colorRole(value string) string {
	value = strings.TrimPrefix(strings.ToLower(value), "#")
	var r, g, b uint8
	_, _ = fmt.Sscanf(value, "%02x%02x%02x", &r, &g, &b)
	if r > 150 && g > 180 && b < 140 {
		return "signal"
	}
	if r > 150 && g > 150 && b > 150 {
		return "surface"
	}
	return "ink"
}

func svgassetViewBox(svg []byte) string {
	match := rootViewBox.Find(svg)
	return strings.TrimSuffix(strings.TrimPrefix(string(match), `viewBox="`), `"`)
}

func fmtHash(sum [sha256.Size]byte) string {
	return fmt.Sprintf("%x", sum)
}
