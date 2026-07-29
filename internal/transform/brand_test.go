package transform

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/svgasset"
)

func TestBuildBrandProducesCompleteDeclaredMatrix(t *testing.T) {
	result := mustBuildBrand(t)

	const products = 5
	const variantsPerProduct = 24 // icon: 13; logo: 11
	if got, want := len(result.Files), products*variantsPerProduct+1; got != want {
		t.Fatalf("BuildBrand() files = %d, want %d", got, want)
	}
	if got, want := len(result.Assets), products*variantsPerProduct; got != want {
		t.Fatalf("BuildBrand() catalog assets = %d, want %d", got, want)
	}
	const adaptiveVariantsPerProduct = 5
	if got, want := len(result.SpriteEntries), products*(variantsPerProduct-adaptiveVariantsPerProduct); got != want {
		t.Fatalf("BuildBrand() sprite entries = %d, want %d", got, want)
	}

	for _, path := range []string{
		"dist/icons/brand/araihu-icon-adaptive-transparent-optical.svg",
		"dist/icons/brand/araihu-icon-tinted-plate-launcher.svg",
		"dist/icons/brand/araihu-icon-monochrome-transparent-optical.svg",
		"dist/brand/araihu/logo/dark-transparent-optical.svg",
		"dist/brand/araihu/logo/grayscale-plate-optical.svg",
		"dist/icons/brand/sprite.svg",
	} {
		if _, ok := result.Files[path]; !ok {
			t.Errorf("BuildBrand() missing %q", path)
		}
	}

	for _, asset := range result.Assets {
		if asset.ColorBehavior == "protected" && strings.Contains(string(result.Files["dist/"+asset.Path]), "currentColor") {
			t.Errorf("protected asset %q uses currentColor", asset.CanonicalName)
		}
	}
}

func TestBuildBrandRejectsRecipeManifestDrift(t *testing.T) {
	brand := testManifest(t)
	brand.Recipes = brand.Recipes[1:]
	if _, err := BuildBrand(os.DirFS(filepath.Join("..", "..")), brand); err == nil || !strings.Contains(err.Error(), "recipe") {
		t.Fatalf("BuildBrand() error = %v, want recipe matrix failure", err)
	}
}

func TestBuildBrandCatalogAssetsValidate(t *testing.T) {
	result := mustBuildBrand(t)
	c := catalog.Catalog{
		SchemaVersion: catalog.SchemaVersion, Release: "v0.1.0", IdentityRevision: 11, Assets: result.Assets,
	}
	if err := catalog.Validate(c); err != nil {
		t.Fatalf("catalog.Validate(BuildBrand assets): %v", err)
	}
}

func TestBuildBrandAdaptivePreservesApprovedSchemeBehavior(t *testing.T) {
	result := mustBuildBrand(t)
	adaptivePath := "dist/icons/brand/araihu-icon-adaptive-transparent-optical.svg"
	lightPath := "dist/icons/brand/araihu-icon-light-transparent-optical.svg"
	adaptive := result.Files[adaptivePath]
	light := result.Files[lightPath]
	if string(adaptive) == string(light) {
		t.Fatal("adaptive and light SVGs are byte-identical")
	}
	for _, want := range []string{
		`<style>`,
		`@media (prefers-color-scheme: dark)`,
		`--araihu-logo-auto-surface: #07111f`,
		`--araihu-logo-auto-ink: #f3f2e9`,
		`var(--araihu-logo-ink, var(--araihu-logo-auto-ink, #07111f))`,
	} {
		if !strings.Contains(string(adaptive), want) {
			t.Errorf("adaptive SVG missing %q", want)
		}
	}
	if _, err := svgasset.Parse(adaptive); err == nil {
		t.Fatal("generic SVG parser accepted generated adaptive stylesheet")
	}
	asset := findAsset(t, result, "araihu-icon-adaptive-transparent-optical")
	if asset.SpriteSymbol != "" {
		t.Fatalf("adaptive SpriteSymbol = %q, want individual-only", asset.SpriteSymbol)
	}
	if strings.Contains(string(result.Files["dist/icons/brand/sprite.svg"]), "araihu-icon-adaptive-transparent-optical") {
		t.Fatal("adaptive individual-only asset leaked into sprite")
	}
}

func TestBuildBrandDesignedTintDiffersFromGrayscaleAndMonochrome(t *testing.T) {
	result := mustBuildBrand(t)
	grayscale := result.Files["dist/icons/brand/araihu-icon-grayscale-plate-optical.svg"]
	tinted := result.Files["dist/icons/brand/araihu-icon-tinted-plate-optical.svg"]
	monochrome := result.Files["dist/icons/brand/araihu-icon-monochrome-transparent-optical.svg"]
	if string(grayscale) == string(tinted) {
		t.Fatal("grayscale and designed tint SVGs are byte-identical")
	}
	for _, want := range []string{`#e6e6e6`, `#202020`, `#707070`} {
		if !strings.Contains(string(grayscale), want) {
			t.Errorf("grayscale SVG missing %s", want)
		}
	}
	for _, want := range []string{`#d5ddeb`, `#31588f`, `#07111f`} {
		if !strings.Contains(string(tinted), want) {
			t.Errorf("designed tint SVG missing %s", want)
		}
	}
	if strings.Contains(string(tinted), "currentColor") {
		t.Fatal("designed tint became runtime-tintable")
	}
	if !strings.Contains(string(monochrome), "currentColor") {
		t.Fatal("monochrome SVG is not runtime-tintable")
	}
}

func TestBuildBrandPreservesEveryPathGeometry(t *testing.T) {
	result := mustBuildBrand(t)
	for _, product := range testManifest(t).Products {
		for _, artwork := range []string{"icon", "logo"} {
			for sourceSurface, outputSurface := range map[string]string{"background": "plate", "transparent": "transparent"} {
				source := product.Sources["original"][artwork+"-"+sourceSurface]
				sourceBytes, err := os.ReadFile(filepath.Join("../..", source))
				if err != nil {
					t.Fatalf("ReadFile(%q): %v", source, err)
				}
				sourceDoc, err := svgasset.Parse(normalizedSource(sourceBytes))
				if err != nil {
					t.Fatalf("Parse(%q): %v", source, err)
				}
				for _, asset := range result.Assets {
					if asset.Product != product.ID || asset.Artwork != artwork || asset.Surface != outputSurface {
						continue
					}
					path := "dist/" + asset.Path
					generated := result.Files[path]
					generatedDoc, err := parseGenerated(generated)
					if err != nil {
						t.Fatalf("Parse(%q): %v", path, err)
					}
					if got, want := string(generatedDoc.GeometrySignature()), string(sourceDoc.GeometrySignature()); got != want {
						t.Errorf("geometry changed: %s", path)
					}
				}
			}
		}
	}
}

func findAsset(t *testing.T, result Result, canonical string) catalog.Asset {
	t.Helper()
	for _, asset := range result.Assets {
		if asset.CanonicalName == canonical {
			return asset
		}
	}
	t.Fatalf("missing catalog asset %q", canonical)
	return catalog.Asset{}
}

func parseGenerated(svg []byte) (svgasset.Document, error) {
	return svgasset.Parse(generatedStyle.ReplaceAll(svg, nil))
}

func TestBuildBrandPromotedSourcesMatchManifestHashes(t *testing.T) {
	brand := testManifest(t)
	for _, product := range brand.Products {
		for kind, want := range product.SourceHashes {
			path := product.Sources["original"][kind]
			data, err := os.ReadFile(filepath.Join("../..", path))
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", path, err)
			}
			got := sha256.Sum256(data)
			if actual := fmtHash(got); actual != want {
				t.Errorf("%s %s hash = %s, want %s", product.ID, kind, actual, want)
			}
		}
	}
}

func mustBuildBrand(t *testing.T) Result {
	t.Helper()
	result, err := BuildBrand(os.DirFS(filepath.Join("..", "..")), testManifest(t))
	if err != nil {
		t.Fatalf("BuildBrand(): %v", err)
	}
	return result
}

func testManifest(t *testing.T) manifest.Brand {
	t.Helper()
	brand, err := manifest.LoadBrand(os.DirFS(filepath.Join("..", "..")), "manifests/brand.yaml")
	if err != nil {
		t.Fatalf("LoadBrand(): %v", err)
	}
	return brand
}

func normalizedSource(svg []byte) []byte {
	return testRootDimensions.ReplaceAllFunc(svg, func(root []byte) []byte {
		root = []byte(strings.ReplaceAll(string(root), ` width="1024"`, ""))
		root = []byte(strings.ReplaceAll(string(root), ` width="2048"`, ""))
		root = []byte(strings.ReplaceAll(string(root), ` height="1024"`, ""))
		return []byte(strings.ReplaceAll(string(root), ` height="508"`, ""))
	})
}

var testRootDimensions = regexp.MustCompile(`<svg((?:\s+(?:xmlns|width|height|viewBox)="[^"]+")*)>`)
var generatedStyle = regexp.MustCompile(`(?s)<style>.*?</style>`)
