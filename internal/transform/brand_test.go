package transform

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

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
	if got, want := len(result.SpriteEntries), products*variantsPerProduct; got != want {
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
					generatedDoc, err := svgasset.Parse(generated)
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
