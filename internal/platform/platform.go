package platform

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/svgasset"
)

const (
	lightBackground = "#f3f2e9"
	darkBackground  = "#07111f"
	tintBackground  = "#e6e6e6"
	safeRatio       = 66.0 / 108.0
)

var products = []string{"araihu", "goshtoso", "manja", "paje", "x9"}

// BrandIcon is Task 4's static icon matrix for one product. AdaptiveSVG remains
// an individual SVG, preserving its media-query behavior in web favicon output.
type BrandIcon struct {
	Product                                    string
	LightSVG, DarkSVG, TintedSVG, GrayscaleSVG []byte
	MonochromeSVG, AdaptiveSVG, LauncherSVG    []byte
}

// Result holds generated files and metadata for every published visual asset.
// Caller owns publication.
type Result struct {
	Files  map[string][]byte
	Assets []catalog.Asset
}

// Build generates every web, Android, and Apple package in stable path order.
func Build(ctx context.Context, rasterizer Rasterizer, icons []BrandIcon) (Result, error) {
	if rasterizer == nil {
		return Result{}, errors.New("platform: rasterizer is required")
	}
	ordered, err := validateIcons(icons)
	if err != nil {
		return Result{}, err
	}
	result := Result{Files: make(map[string][]byte, len(ordered)*31)}
	for _, icon := range ordered {
		if err := ctx.Err(); err != nil {
			return Result{}, fmt.Errorf("platform: %w", err)
		}
		if err := buildProduct(ctx, rasterizer, &result, icon); err != nil {
			return Result{}, fmt.Errorf("platform %s: %w", icon.Product, err)
		}
	}
	assets, err := catalogAssets(result.Files)
	if err != nil {
		return Result{}, err
	}
	result.Assets = assets
	return result, nil
}

func catalogAssets(files map[string][]byte) ([]catalog.Asset, error) {
	paths := make([]string, 0, len(files))
	for name := range files {
		if strings.HasSuffix(name, ".svg") || strings.HasSuffix(name, ".png") {
			paths = append(paths, name)
		}
	}
	slices.Sort(paths)
	assets := make([]catalog.Asset, 0, len(paths))
	for _, name := range paths {
		asset, err := catalogAsset(name, files[name])
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, nil
}

func catalogAsset(name string, data []byte) (catalog.Asset, error) {
	path := strings.TrimPrefix(name, "dist/")
	parts := strings.Split(path, "/")
	if len(parts) < 4 || parts[0] != "platform" {
		return catalog.Asset{}, fmt.Errorf("platform: invalid generated path %q", name)
	}
	format := strings.TrimPrefix(strings.ToLower(filepathExt(path)), ".")
	dimensions := catalog.Dimensions{}
	switch format {
	case "svg":
		dimensions.ViewBox = viewBoxOf(data)
	case "png":
		info, err := inspectPNG(data)
		if err != nil {
			return catalog.Asset{}, fmt.Errorf("platform: inspect %s: %w", name, err)
		}
		dimensions.Width, dimensions.Height = info.width, info.height
	default:
		return catalog.Asset{}, fmt.Errorf("platform: unsupported visual format %q", name)
	}
	sum := sha256.Sum256(data)
	return catalog.Asset{
		CanonicalName: platformCanonicalName(path, format), Namespace: "brand", Path: path, Product: parts[2], Artwork: "icon",
		Appearance: platformAppearance(path), Surface: platformSurface(path), Framing: platformFraming(path), Format: format, Dimensions: dimensions,
		ColorBehavior: platformColorBehavior(path), License: "Arai Hu Brand Terms", Source: "platform generator v0.1.0", SHA256: hex.EncodeToString(sum[:]),
	}, nil
}

func filepathExt(name string) string {
	if index := strings.LastIndexByte(name, '.'); index >= 0 {
		return name[index:]
	}
	return ""
}

func platformCanonicalName(path, format string) string {
	base := strings.TrimSuffix(strings.ToLower(path), "."+format)
	base = strings.NewReplacer("/", "-", ".", "-", "_", "-").Replace(base)
	return base + "-" + format
}

func platformAppearance(path string) string {
	switch {
	case strings.Contains(path, "monochrome"):
		return "monochrome"
	case strings.Contains(path, "dark"):
		return "dark"
	case strings.Contains(path, "tinted"):
		return "grayscale"
	case strings.HasSuffix(path, "favicon.svg"):
		return "adaptive"
	default:
		return "light"
	}
}

func platformSurface(path string) string {
	if strings.HasSuffix(path, "favicon.svg") || strings.Contains(path, "icon-192.png") || strings.Contains(path, "icon-512.png") || strings.Contains(path, "foreground.png") || strings.Contains(path, "monochrome.png") {
		return "transparent"
	}
	return "plate"
}

func platformFraming(path string) string {
	if strings.Contains(path, "/android/") || strings.Contains(path, "/apple/") || strings.Contains(path, "maskable") || strings.Contains(path, "apple-touch") {
		return "launcher"
	}
	return "optical"
}

func platformColorBehavior(path string) string {
	if strings.Contains(path, "monochrome") {
		return "monochrome"
	}
	return "protected"
}

func validateIcons(icons []BrandIcon) ([]BrandIcon, error) {
	if len(icons) != len(products) {
		return nil, fmt.Errorf("platform: expected %d products, got %d", len(products), len(icons))
	}
	byProduct := make(map[string]BrandIcon, len(icons))
	for _, icon := range icons {
		if !slices.Contains(products, icon.Product) {
			return nil, fmt.Errorf("platform: unknown product %q", icon.Product)
		}
		if _, exists := byProduct[icon.Product]; exists {
			return nil, fmt.Errorf("platform: duplicate product %q", icon.Product)
		}
		for name, svg := range map[string][]byte{
			"light": icon.LightSVG, "dark": icon.DarkSVG, "tinted": icon.TintedSVG, "grayscale": icon.GrayscaleSVG,
			"monochrome": icon.MonochromeSVG, "adaptive": icon.AdaptiveSVG, "launcher": icon.LauncherSVG,
		} {
			document, err := svgasset.ParseGenerated(svg)
			if err != nil || document.ViewBox() == "" {
				return nil, fmt.Errorf("platform: %s SVG for %s must be a safe generated SVG", name, icon.Product)
			}
		}
		byProduct[icon.Product] = icon
	}
	ordered := make([]BrandIcon, 0, len(products))
	for _, product := range products {
		icon, ok := byProduct[product]
		if !ok {
			return nil, fmt.Errorf("platform: missing product %q", product)
		}
		ordered = append(ordered, icon)
	}
	return ordered, nil
}

func buildProduct(ctx context.Context, rasterizer Rasterizer, result *Result, icon BrandIcon) error {
	web := "dist/platform/web/" + icon.Product + "/"
	result.Files[web+"favicon.svg"] = append([]byte(nil), icon.AdaptiveSVG...)
	if err := addRaster(ctx, rasterizer, result, web+"favicon-16.png", icon.LightSVG, 16, lightBackground, false); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"favicon-32.png", icon.LightSVG, 32, lightBackground, false); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"icon-192.png", icon.LightSVG, 192, "", false); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"icon-512.png", icon.LightSVG, 512, "", false); err != nil {
		return err
	}
	maskable, err := launcherViewBox(icon.LightSVG, icon.LauncherSVG)
	if err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"icon-maskable-192.png", maskable, 192, lightBackground, true); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"icon-maskable-512.png", maskable, 512, lightBackground, true); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, web+"apple-touch-icon-180.png", maskable, 180, lightBackground, true); err != nil {
		return err
	}
	result.Files[web+"manifest-icons.json"] = manifestJSON()

	android := "dist/platform/android/" + icon.Product + "/res/"
	foreground, err := launcherViewBox(icon.LightSVG, icon.LauncherSVG)
	if err != nil {
		return err
	}
	monochrome, err := launcherViewBox(icon.MonochromeSVG, icon.LauncherSVG)
	if err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, android+"drawable-xxxhdpi/ic_launcher_foreground.png", foreground, 432, "", true); err != nil {
		return err
	}
	if err := addRaster(ctx, rasterizer, result, android+"drawable-xxxhdpi/ic_launcher_monochrome.png", monochrome, 432, "", true); err != nil {
		return err
	}
	for _, density := range []struct {
		name string
		size int
	}{{"mdpi", 48}, {"hdpi", 72}, {"xhdpi", 96}, {"xxhdpi", 144}, {"xxxhdpi", 192}} {
		for _, name := range []string{"ic_launcher.png", "ic_launcher_round.png"} {
			if err := addRaster(ctx, rasterizer, result, android+"mipmap-"+density.name+"/"+name, foreground, density.size, lightBackground, true); err != nil {
				return err
			}
		}
	}
	for _, api := range []int{26, 33} {
		for _, name := range []string{"ic_launcher.xml", "ic_launcher_round.xml"} {
			result.Files[fmt.Sprintf("%smipmap-anydpi-v%d/%s", android, api, name)] = adaptiveXML(api == 33)
		}
	}
	result.Files[android+"values/colors.xml"] = []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<resources>\n  <color name=\"ic_launcher_background\">#F3F2E9</color>\n</resources>\n")

	apple := "dist/platform/apple/" + icon.Product + "/Assets.xcassets/"
	result.Files[apple+"Contents.json"] = []byte("{\"info\":{\"author\":\"araihu\",\"version\":1}}\n")
	appicon := apple + "AppIcon.appiconset/"
	appleVariants := []struct {
		name       string
		svg        []byte
		background string
	}{
		{"AppIcon-1024.png", icon.LightSVG, lightBackground},
		{"AppIcon-1024-dark.png", icon.DarkSVG, darkBackground},
		// Apple's `tinted` appearance uses the approved grayscale/tinted master,
		// not Task 4's separately designed blue tint artwork.
		{"AppIcon-1024-tinted.png", icon.GrayscaleSVG, tintBackground},
	}
	for _, variant := range appleVariants {
		safe, err := launcherViewBox(variant.svg, icon.LauncherSVG)
		if err != nil {
			return err
		}
		if err := addRaster(ctx, rasterizer, result, appicon+variant.name, safe, 1024, variant.background, true); err != nil {
			return err
		}
	}
	result.Files[appicon+"Contents.json"] = appleContentsJSON()
	return nil
}

func addRaster(ctx context.Context, rasterizer Rasterizer, result *Result, path string, svg []byte, size int, background string, safe bool) error {
	png, err := rasterizer.Rasterize(ctx, Request{SVG: svg, Width: size, Height: size, Background: background})
	if err != nil {
		return fmt.Errorf("rasterize %s: %w", path, err)
	}
	if err := validatePNG(png, size, background, safe); err != nil {
		return fmt.Errorf("validate %s: %w", path, err)
	}
	result.Files[path] = png
	return nil
}

func manifestJSON() []byte {
	// Explicit bytes avoid map ordering and retain consumer-facing field names.
	return []byte("{\"icons\":[{\"src\":\"icon-192.png\",\"sizes\":\"192x192\",\"type\":\"image/png\",\"purpose\":\"any\"},{\"src\":\"icon-512.png\",\"sizes\":\"512x512\",\"type\":\"image/png\",\"purpose\":\"any\"},{\"src\":\"icon-maskable-192.png\",\"sizes\":\"192x192\",\"type\":\"image/png\",\"purpose\":\"maskable\"},{\"src\":\"icon-maskable-512.png\",\"sizes\":\"512x512\",\"type\":\"image/png\",\"purpose\":\"maskable\"}]}\n")
}

func adaptiveXML(monochrome bool) []byte {
	mono := ""
	if monochrome {
		mono = "\n  <monochrome android:drawable=\"@drawable/ic_launcher_monochrome\" />"
	}
	return []byte("<?xml version=\"1.0\" encoding=\"utf-8\"?>\n<adaptive-icon xmlns:android=\"http://schemas.android.com/apk/res/android\">\n  <background android:drawable=\"@color/ic_launcher_background\" />\n  <foreground android:drawable=\"@drawable/ic_launcher_foreground\" />" + mono + "\n</adaptive-icon>\n")
}

func appleContentsJSON() []byte {
	return []byte("{\"images\":[{\"filename\":\"AppIcon-1024.png\",\"idiom\":\"universal\",\"platform\":\"ios\",\"size\":\"1024x1024\"},{\"appearances\":[{\"appearance\":\"luminosity\",\"value\":\"dark\"}],\"filename\":\"AppIcon-1024-dark.png\",\"idiom\":\"universal\",\"platform\":\"ios\",\"size\":\"1024x1024\"},{\"appearances\":[{\"appearance\":\"luminosity\",\"value\":\"tinted\"}],\"filename\":\"AppIcon-1024-tinted.png\",\"idiom\":\"universal\",\"platform\":\"ios\",\"size\":\"1024x1024\"}],\"info\":{\"author\":\"araihu\",\"version\":1}}\n")
}

func launcherViewBox(svg, launcher []byte) ([]byte, error) {
	viewBox := viewBoxOf(launcher)
	if viewBox == "" {
		return nil, errors.New("launcher SVG has no root viewBox")
	}
	start, end := rootSVGRange(svg)
	if start < 0 || end < 0 {
		return nil, errors.New("SVG has no root element")
	}
	root := string(svg[start:end])
	if !strings.Contains(root, "viewBox=") {
		return nil, errors.New("SVG has no root viewBox")
	}
	old := viewBoxOf(svg)
	return []byte(strings.Replace(string(svg), `viewBox="`+old+`"`, `viewBox="`+viewBox+`"`, 1)), nil
}

func rootSVGRange(svg []byte) (int, int) {
	start := bytes.Index(svg, []byte("<svg"))
	if start < 0 {
		return -1, -1
	}
	endRel := bytes.IndexByte(svg[start:], '>')
	if endRel < 0 {
		return -1, -1
	}
	return start, start + endRel + 1
}

func viewBoxOf(svg []byte) string {
	start, end := rootSVGRange(svg)
	if start < 0 {
		return ""
	}
	root := string(svg[start:end])
	marker := `viewBox="`
	index := strings.Index(root, marker)
	if index < 0 {
		return ""
	}
	rest := root[index+len(marker):]
	close := strings.IndexByte(rest, '"')
	if close < 0 || strings.TrimSpace(rest[:close]) == "" {
		return ""
	}
	return rest[:close]
}

type pngInfo struct {
	width, height                 int
	hasAlpha, opaque, transparent bool
}

func inspectPNG(data []byte) (pngInfo, error) {
	if len(data) < 26 || !bytes.Equal(data[:8], []byte{137, 80, 78, 71, 13, 10, 26, 10}) || string(data[12:16]) != "IHDR" {
		return pngInfo{}, errors.New("invalid PNG signature or IHDR")
	}
	if err := rejectTimestampChunks(data); err != nil {
		return pngInfo{}, err
	}
	width, height := int(binary.BigEndian.Uint32(data[16:20])), int(binary.BigEndian.Uint32(data[20:24]))
	if width < 1 || height < 1 {
		return pngInfo{}, errors.New("nonpositive PNG dimensions")
	}
	colorType := data[25]
	if colorType != 0 && colorType != 2 && colorType != 3 && colorType != 4 && colorType != 6 {
		return pngInfo{}, fmt.Errorf("unsupported PNG color type %d", colorType)
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return pngInfo{}, fmt.Errorf("decode PNG: %w", err)
	}
	opaque, transparent := true, false
	for y := decoded.Bounds().Min.Y; y < decoded.Bounds().Max.Y && opaque; y++ {
		for x := decoded.Bounds().Min.X; x < decoded.Bounds().Max.X; x++ {
			_, _, _, alpha := decoded.At(x, y).RGBA()
			if alpha != 0xffff {
				opaque = false
				transparent = true
				break
			}
		}
	}
	info := pngInfo{width: width, height: height, hasAlpha: colorType == 4 || colorType == 6, opaque: opaque, transparent: transparent}
	return info, nil
}

func rejectTimestampChunks(data []byte) error {
	for offset := 8; offset < len(data); {
		if len(data)-offset < 12 {
			return errors.New("truncated PNG chunk")
		}
		length := int(binary.BigEndian.Uint32(data[offset : offset+4]))
		if length > len(data)-offset-12 {
			return errors.New("invalid PNG chunk length")
		}
		kind := string(data[offset+4 : offset+8])
		if kind == "tIME" {
			return errors.New("PNG contains forbidden tIME timestamp metadata")
		}
		offset += 12 + length
		if kind == "IEND" {
			if offset != len(data) {
				return errors.New("trailing bytes after PNG IEND")
			}
			return nil
		}
	}
	return errors.New("PNG is missing IEND")
}

func validatePNG(data []byte, size int, background string, requireSafe bool) error {
	info, err := inspectPNG(data)
	if err != nil {
		return err
	}
	if info.width != size || info.height != size {
		return fmt.Errorf("dimensions %dx%d, expected %dx%d", info.width, info.height, size, size)
	}
	if background == "" {
		if !info.hasAlpha {
			return errors.New("transparent raster lacks alpha channel")
		}
		if !info.transparent {
			return errors.New("transparent raster lacks transparent pixels")
		}
	} else if info.hasAlpha || !info.opaque {
		return errors.New("opaque raster has alpha channel or transparent pixels")
	}
	if requireSafe {
		if err := validateSafeBounds(data, background); err != nil {
			return err
		}
	}
	return nil
}

func validateSafeBounds(data []byte, background string) error {
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return err
	}
	bound := decoded.Bounds()
	var bg color.Color
	if background != "" {
		value, err := strconv.ParseUint(strings.TrimPrefix(background, "#"), 16, 32)
		if err != nil {
			return fmt.Errorf("parse background %q: %w", background, err)
		}
		bg = color.RGBA{R: uint8(value >> 16), G: uint8(value >> 8), B: uint8(value), A: 255}
	}
	x1, y1, x2, y2 := bound.Max.X, bound.Max.Y, -1, -1
	for y := bound.Min.Y; y < bound.Max.Y; y++ {
		for x := bound.Min.X; x < bound.Max.X; x++ {
			r, g, b, a := decoded.At(x, y).RGBA()
			visible := a != 0
			if bg != nil {
				br, bgc, bb, _ := bg.RGBA()
				visible = r != br || g != bgc || b != bb
			}
			if visible {
				x1, y1, x2, y2 = min(x1, x), min(y1, y), max(x2, x), max(y2, y)
			}
		}
	}
	if x2 < x1 {
		return errors.New("safe raster has no visible art")
	}
	margin := float64(bound.Dx()) * (1 - safeRatio) / 2
	if float64(min(x1, y1)) < math.Floor(margin)-2 || float64(max(x2, y2)) > math.Ceil(float64(bound.Dx())-margin)+2 {
		return fmt.Errorf("art bounds %d,%d-%d,%d exceed 66/108 safe square", x1, y1, x2, y2)
	}
	return nil
}
