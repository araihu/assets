package platform

import (
	"bytes"
	"compress/zlib"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/svgasset"
)

func TestBuildIncludesAllPlatformContracts(t *testing.T) {
	result, err := Build(context.Background(), fakeRasterizer{}, testIcons())
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	if got, want := len(result.Files), 155; got != want {
		t.Fatalf("generated file count = %d, want %d", got, want)
	}
	for path, data := range result.Files {
		if !strings.HasSuffix(path, ".svg") {
			continue
		}
		if _, err := svgasset.ParseGenerated(data); err != nil {
			t.Errorf("generated platform SVG %s: %v", path, err)
		}
	}
	requirePNG(t, result, "dist/platform/web/araihu/favicon-16.png", 16, false, false)
	requirePNG(t, result, "dist/platform/web/araihu/icon-maskable-512.png", 512, false, true)
	requireFile(t, result, "dist/platform/android/araihu/res/mipmap-anydpi-v26/ic_launcher.xml")
	requireFile(t, result, "dist/platform/android/araihu/res/mipmap-anydpi-v33/ic_launcher.xml")
	requirePNG(t, result, "dist/platform/android/araihu/res/drawable-xxxhdpi/ic_launcher_foreground.png", 432, true, true)
	requirePNG(t, result, "dist/platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png", 1024, false, true)
	requireFile(t, result, "dist/platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/AppIcon-1024-dark.png")
	requireFile(t, result, "dist/platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/AppIcon-1024-tinted.png")

	manifest := string(result.Files["dist/platform/web/araihu/manifest-icons.json"])
	for _, want := range []string{`"purpose":"any"`, `"purpose":"maskable"`, `"icon-192.png"`, `"icon-maskable-512.png"`} {
		if !strings.Contains(manifest, want) {
			t.Errorf("manifest missing %s: %s", want, manifest)
		}
	}
	api26 := string(result.Files["dist/platform/android/araihu/res/mipmap-anydpi-v26/ic_launcher.xml"])
	api33 := string(result.Files["dist/platform/android/araihu/res/mipmap-anydpi-v33/ic_launcher.xml"])
	if strings.Contains(api26, "<monochrome") || !strings.Contains(api33, "<monochrome") {
		t.Fatalf("incorrect Android monochrome API contract")
	}
}

func TestBuildCatalogCoversEveryPublishedVisualArtifact(t *testing.T) {
	result, err := Build(context.Background(), fakeRasterizer{}, testIcons())
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.Validate(catalog.Catalog{SchemaVersion: catalog.SchemaVersion, Release: "v0.1.0", IdentityRevision: 11, Assets: result.Assets}); err != nil {
		t.Fatalf("platform catalog: %v", err)
	}
	assets := make(map[string]catalog.Asset, len(result.Assets))
	for _, asset := range result.Assets {
		if _, duplicate := assets[asset.Path]; duplicate {
			t.Fatalf("duplicate catalog path %q", asset.Path)
		}
		assets[asset.Path] = asset
	}
	for name, data := range result.Files {
		path := strings.TrimPrefix(name, "dist/")
		visual := strings.HasSuffix(path, ".svg") || strings.HasSuffix(path, ".png")
		asset, found := assets[path]
		if visual != found {
			t.Fatalf("catalog presence for %s = %t, want %t", path, found, visual)
		}
		if !visual {
			continue
		}
		sum := sha256.Sum256(data)
		if asset.Product == "" || asset.Artwork != "icon" || asset.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("catalog metadata for %s = %#v", path, asset)
		}
	}
}

func TestBuildPublishesAdaptiveFaviconAndUsesGrayscaleAppleSource(t *testing.T) {
	icons := testIcons()
	rasterizer := &recordingRasterizer{}
	result, err := Build(context.Background(), rasterizer, icons)
	if err != nil {
		t.Fatalf("Build(): %v", err)
	}
	favicon := result.Files["dist/platform/web/araihu/favicon.svg"]
	if !bytes.Equal(favicon, icons[0].AdaptiveSVG) {
		t.Fatalf("favicon.svg = %q, want genuine adaptive Task 4 source", favicon)
	}
	if bytes.Equal(favicon, icons[0].LightSVG) || !bytes.Contains(favicon, []byte("adaptive-only-marker")) {
		t.Fatal("favicon.svg lost adaptive semantics")
	}
	grayscaleRasters := 0
	for _, request := range rasterizer.requests {
		if request.Width == 1024 && request.Background == tintBackground {
			grayscaleRasters++
			if !bytes.Contains(request.SVG, []byte("grayscale-source")) || bytes.Contains(request.SVG, []byte("tint-source")) {
				t.Fatalf("Apple tinted/grayscale raster used wrong source: %s", request.SVG)
			}
		}
	}
	if grayscaleRasters != len(products) {
		t.Fatalf("Apple grayscale raster count = %d, want %d", grayscaleRasters, len(products))
	}
	contents := string(result.Files["dist/platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/Contents.json"])
	for _, want := range []string{`"filename":"AppIcon-1024-tinted.png"`, `"appearance":"luminosity"`, `"value":"tinted"`} {
		if !strings.Contains(contents, want) {
			t.Errorf("Apple Contents.json missing approved tinted catalog naming %s: %s", want, contents)
		}
	}
}

func TestBuildIsDeterministicAndDoesNotRasterizeAdaptiveSVG(t *testing.T) {
	icons := testIcons()
	first, err := Build(context.Background(), fakeRasterizer{}, icons)
	if err != nil {
		t.Fatalf("first Build(): %v", err)
	}
	second, err := Build(context.Background(), fakeRasterizer{}, icons)
	if err != nil {
		t.Fatalf("second Build(): %v", err)
	}
	if len(first.Files) != len(second.Files) {
		t.Fatal("non-deterministic file count")
	}
	for path, want := range first.Files {
		if got := second.Files[path]; !bytes.Equal(got, want) {
			t.Errorf("non-deterministic output %s", path)
		}
	}
	for path, data := range first.Files {
		if strings.HasSuffix(path, ".png") && bytes.Contains(data, []byte("adaptive-only-marker")) {
			t.Fatalf("adaptive individual-only SVG was rasterized into %s", path)
		}
	}
}

func TestBuildRejectsInvalidInputsAndBadRasterOutput(t *testing.T) {
	icons := testIcons()
	icons[0].Product = "wrong"
	if _, err := Build(context.Background(), fakeRasterizer{}, icons); err == nil {
		t.Fatal("Build accepted unknown product")
	}
	icons = testIcons()
	icons[0].LightSVG = nil
	if _, err := Build(context.Background(), fakeRasterizer{}, icons); err == nil {
		t.Fatal("Build accepted missing static SVG")
	}
	icons = testIcons()
	icons[0].AdaptiveSVG = []byte(`<svg viewBox="0 0 108 108"><style>path { fill: red; }</style><path d="M21 21h66v66"/></svg>`)
	if _, err := Build(context.Background(), fakeRasterizer{}, icons); err == nil {
		t.Fatal("Build accepted arbitrary adaptive stylesheet")
	}
	if _, err := Build(context.Background(), badRasterizer{}, testIcons()); err == nil {
		t.Fatal("Build accepted invalid raster output")
	}
}

func TestValidatePNGRejectsTimestampMetadata(t *testing.T) {
	data, err := fakeRasterizer{}.Rasterize(context.Background(), Request{Width: 16, Height: 16})
	if err != nil {
		t.Fatal(err)
	}
	data = appendPNGChunk(data, "tIME", []byte{7, 234, 1, 1, 0, 0, 0})
	if err := validatePNG(data, 16, "", false); err == nil || !strings.Contains(err.Error(), "timestamp") {
		t.Fatalf("timestamp PNG error = %v", err)
	}
}

func TestValidatePNGRejectsOpaqueRGBAForTransparentOutput(t *testing.T) {
	if err := validatePNG(opaqueRGBAPNG(t, 16), 16, "", false); err == nil || !strings.Contains(err.Error(), "transparent pixels") {
		t.Fatalf("opaque RGBA PNG error = %v", err)
	}
}

type fakeRasterizer struct{}

func (fakeRasterizer) Rasterize(_ context.Context, request Request) ([]byte, error) {
	if request.Width != request.Height {
		return nil, fmt.Errorf("fake only supports square output")
	}
	if bytes.Contains(request.SVG, []byte("adaptive-only-marker")) {
		return nil, fmt.Errorf("adaptive individual SVG must not be rasterized")
	}
	img := image.NewRGBA(image.Rect(0, 0, request.Width, request.Height))
	if request.Background != "" {
		background := color.RGBA{R: 243, G: 242, B: 233, A: 255}
		if request.Background == "#07111f" {
			background = color.RGBA{R: 7, G: 17, B: 31, A: 255}
		}
		if request.Background == "#e6e6e6" {
			background = color.RGBA{R: 230, G: 230, B: 230, A: 255}
		}
		for y := 0; y < request.Height; y++ {
			for x := 0; x < request.Width; x++ {
				img.Set(x, y, background)
			}
		}
	}
	margin := request.Width * 21 / 100
	art := color.RGBA{R: 7, G: 17, B: 31, A: 255}
	if request.Background == "#07111f" {
		art = color.RGBA{R: 243, G: 242, B: 233, A: 255}
	}
	for y := margin; y < request.Height-margin; y++ {
		for x := margin; x < request.Width-margin; x++ {
			img.Set(x, y, art)
		}
	}
	var output bytes.Buffer
	if err := png.Encode(&output, img); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

type badRasterizer struct{}

func (badRasterizer) Rasterize(context.Context, Request) ([]byte, error) {
	return []byte("not png"), nil
}

type recordingRasterizer struct{ requests []Request }

func (r *recordingRasterizer) Rasterize(ctx context.Context, request Request) ([]byte, error) {
	r.requests = append(r.requests, Request{SVG: append([]byte(nil), request.SVG...), Width: request.Width, Height: request.Height, Background: request.Background})
	return fakeRasterizer{}.Rasterize(ctx, request)
}

func appendPNGChunk(data []byte, kind string, payload []byte) []byte {
	chunk := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(payload)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[8+len(payload):], crc32.ChecksumIEEE(chunk[4:8+len(payload)]))
	insert := len(data) - 12
	return append(append(append([]byte(nil), data[:insert]...), chunk...), data[insert:]...)
}

func opaqueRGBAPNG(t *testing.T, size int) []byte {
	t.Helper()
	var output bytes.Buffer
	output.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10})
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[:4], uint32(size))
	binary.BigEndian.PutUint32(ihdr[4:8], uint32(size))
	ihdr[8], ihdr[9] = 8, 6
	output.Write(pngChunk("IHDR", ihdr))
	var raw bytes.Buffer
	for row := 0; row < size; row++ {
		raw.WriteByte(0)
		for column := 0; column < size; column++ {
			raw.Write([]byte{7, 17, 31, 255})
		}
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output.Write(pngChunk("IDAT", compressed.Bytes()))
	output.Write(pngChunk("IEND", nil))
	return output.Bytes()
}

func pngChunk(kind string, payload []byte) []byte {
	chunk := make([]byte, 12+len(payload))
	binary.BigEndian.PutUint32(chunk[:4], uint32(len(payload)))
	copy(chunk[4:8], kind)
	copy(chunk[8:], payload)
	binary.BigEndian.PutUint32(chunk[8+len(payload):], crc32.ChecksumIEEE(chunk[4:8+len(payload)]))
	return chunk
}

func testIcons() []BrandIcon {
	icons := make([]BrandIcon, 0, 5)
	for _, product := range []string{"araihu", "goshtoso", "manja", "paje", "x9"} {
		icons = append(icons, BrandIcon{
			Product:       product,
			LightSVG:      []byte(`<svg viewBox="0 0 108 108"><path d="M21 21h66v66"/></svg>`),
			DarkSVG:       []byte(`<svg viewBox="0 0 108 108"><path d="M21 21h66v66"/></svg>`),
			TintedSVG:     []byte(`<svg viewBox="0 0 108 108"><path id="tint-source" d="M21 21h66v66"/></svg>`),
			GrayscaleSVG:  []byte(`<svg viewBox="0 0 108 108"><path id="grayscale-source" d="M21 21h66v66"/></svg>`),
			MonochromeSVG: []byte(`<svg viewBox="0 0 108 108"><path d="M21 21h66v66"/></svg>`),
			AdaptiveSVG:   []byte(`<svg viewBox="0 0 108 108"><path id="adaptive-only-marker" d="M21 21h66v66"/></svg>`),
			LauncherSVG:   []byte(`<svg viewBox="0 0 108 108"><path d="M21 21h66v66"/></svg>`),
		})
	}
	return icons
}

func requireFile(t *testing.T, result Result, path string) {
	t.Helper()
	if _, ok := result.Files[path]; !ok {
		t.Fatalf("missing %s", path)
	}
}

func requirePNG(t *testing.T, result Result, path string, size int, wantAlpha, requireSafe bool) {
	t.Helper()
	data, ok := result.Files[path]
	if !ok {
		t.Fatalf("missing %s", path)
	}
	info, err := inspectPNG(data)
	if err != nil {
		t.Fatalf("inspect %s: %v", path, err)
	}
	if info.width != size || info.height != size {
		t.Fatalf("%s dimensions = %dx%d, want %dx%d", path, info.width, info.height, size, size)
	}
	if info.hasAlpha != wantAlpha {
		t.Fatalf("%s alpha channel = %v, want %v", path, info.hasAlpha, wantAlpha)
	}
	if wantAlpha && !info.transparent {
		t.Fatalf("%s has no transparent pixels", path)
	}
	if !wantAlpha && !info.opaque {
		t.Fatalf("%s is not opaque", path)
	}
	if requireSafe {
		background := ""
		if !wantAlpha {
			background = lightBackground
		}
		if err := validateSafeBounds(data, background); err != nil {
			t.Fatalf("%s art exceeds 66/108 safe square: %v", path, err)
		}
	}
}
