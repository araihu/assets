package proof

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/araihu/assets/internal/catalog"
)

func TestLoadRejectsMissingCatalogReference(t *testing.T) {
	_, err := Load(fixtureCatalog(t), strings.NewReader(`{"scenarios":[{"id":"missing","group":"brand","asset":"missing","surface":"transparent","mask":"none","context":"web-navigation","sizes":[16]}]}`))
	if err == nil || !strings.Contains(err.Error(), "unknown canonicalName") {
		t.Fatalf("Load() error = %v, want unknown canonicalName", err)
	}
}

func TestLoadRejectsDuplicateScenarioID(t *testing.T) {
	input := `{"scenarios":[` + scenarioJSON("brand-araihu-icon", "first", []int{16}) + `,{"id":"first","group":"ui","asset":"ui-hi-16-solid-check","artwork":"icon","appearance":"default","surface":"transparent","framing":"optical","mask":"none","context":"ui-sprite","sizes":[20]}]}`
	_, err := Load(fixtureCatalog(t), strings.NewReader(input))
	if err == nil || !strings.Contains(err.Error(), "duplicate scenario id") {
		t.Fatalf("Load() error = %v, want duplicate scenario id", err)
	}
}

func TestLoadRejectsInvalidMaskAndMaskUnsafeAsset(t *testing.T) {
	for _, tc := range []struct {
		name  string
		mask  string
		asset string
		want  string
	}{
		{"unknown mask", "hexagon", "brand-araihu-icon", "invalid mask"},
		{"mask on non-launcher", "circle", "brand-araihu-icon", "mask requires launcher"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Load(fixtureCatalog(t), strings.NewReader(`{"scenarios":[`+scenarioJSONWithMask(tc.asset, "one", tc.mask, []int{16})+`,`+scenarioJSON("ui-hi-16-solid-check", "two", []int{20})+`]}`))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Load() error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestLoadRejectsNonpositiveAndDuplicateSizes(t *testing.T) {
	for _, sizes := range [][]int{{0}, {-1}, {16, 16}} {
		input := `{"scenarios":[` + scenarioJSON("brand-araihu-icon", "one", sizes) + `,` + scenarioJSON("ui-hi-16-solid-check", "two", []int{20}) + `]}`
		_, err := Load(fixtureCatalog(t), strings.NewReader(input))
		if err == nil {
			t.Fatalf("Load(%v) error = nil", sizes)
		}
	}
}

func TestLoadRejectsMalformedAndUnknownScenarioJSON(t *testing.T) {
	for _, input := range []string{
		`{"scenarios":[],"extra":true}`,
		`{"scenarios":[{"id":"one","id":"two"}]}`,
		`{"scenarios":[]} {}`,
	} {
		if _, err := Load(fixtureCatalog(t), strings.NewReader(input)); err == nil {
			t.Fatalf("Load(%q) error = nil", input)
		}
	}
}

func TestLoadRejectsMissingProductCoverage(t *testing.T) {
	_, err := Load(fixtureCatalog(t), strings.NewReader(`{"scenarios":[`+scenarioJSON("brand-araihu-icon", "only", []int{16})+`]}`))
	if err == nil || !strings.Contains(err.Error(), "missing product coverage") {
		t.Fatalf("Load() error = %v, want missing product coverage", err)
	}
}

func TestBuildRejectsMissingReferencedDistributionFile(t *testing.T) {
	m := fixtureModel(t)
	var output bytes.Buffer
	err := Build(m, fstest.MapFS{
		"icons/brand/araihu-icon.svg": &fstest.MapFile{Data: []byte("svg")},
	}, &output)
	if err == nil || !strings.Contains(err.Error(), "missing referenced distribution file") {
		t.Fatalf("Build() error = %v, want missing referenced distribution file", err)
	}
}

func TestProductionScenariosCoverLiteral512AndEveryProduct(t *testing.T) {
	m := loadProduction(t)
	if !slices.Contains(m.ExactSizes, 512) {
		t.Fatalf("ExactSizes = %v, want 512", m.ExactSizes)
	}
	for _, id := range []string{"araihu", "goshtoso", "manja", "paje", "x9"} {
		if !m.HasProduct(id) {
			t.Fatalf("HasProduct(%q) = false", id)
		}
	}
}

func TestProductionScenariosCoverBrandCatalogAndUISpriteGlyphs(t *testing.T) {
	m := loadProduction(t)
	covered := make(map[string]Scenario, len(m.Scenarios))
	for _, scenario := range m.Scenarios {
		if _, exists := covered[scenario.Asset]; !exists {
			covered[scenario.Asset] = scenario
		}
	}

	uiGlyphs := 0
	for _, asset := range m.Catalog.Assets {
		scenario, ok := covered[asset.CanonicalName]
		if !ok {
			t.Fatalf("asset %q has no scenario", asset.CanonicalName)
		}
		if asset.Namespace == "brand" && (scenario.Surface != asset.Surface || scenario.Artwork != asset.Artwork || scenario.Appearance != asset.Appearance || scenario.Framing != asset.Framing) {
			t.Fatalf("scenario %q does not preserve brand semantics for %q", scenario.ID, asset.CanonicalName)
		}
		if asset.Namespace == "ui" && asset.SpriteSymbol != "" {
			uiGlyphs++
		}
	}
	if uiGlyphs != 67 {
		t.Fatalf("UI sprite glyph coverage = %d, want 67", uiGlyphs)
	}
}

func TestBuildAcceptsProductionDistribution(t *testing.T) {
	m := loadProduction(t)
	root := filepath.Join("..", "..", "dist")
	var output bytes.Buffer
	if err := Build(m, os.DirFS(root), &output); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
}

func fixtureCatalog(t *testing.T) catalog.Catalog {
	t.Helper()
	b, err := os.ReadFile("testdata/catalog.json")
	if err != nil {
		t.Fatalf("ReadFile catalog fixture: %v", err)
	}
	c, err := catalog.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("catalog.Decode() error = %v", err)
	}
	return c
}

func fixtureModel(t *testing.T) Model {
	t.Helper()
	b, err := os.ReadFile("testdata/scenarios.json")
	if err != nil {
		t.Fatalf("ReadFile scenario fixture: %v", err)
	}
	m, err := Load(fixtureCatalog(t), bytes.NewReader(b))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return m
}

func loadProduction(t *testing.T) Model {
	t.Helper()
	catalogFile, err := os.Open(filepath.Join("..", "..", "dist", "catalog.json"))
	if err != nil {
		t.Fatalf("Open production catalog: %v", err)
	}
	t.Cleanup(func() { _ = catalogFile.Close() })
	c, err := catalog.Decode(catalogFile)
	if err != nil {
		t.Fatalf("catalog.Decode() error = %v", err)
	}
	scenarios, err := os.Open(filepath.Join("..", "..", "site", "proof", "scenarios.json"))
	if err != nil {
		t.Fatalf("Open production scenarios: %v", err)
	}
	t.Cleanup(func() { _ = scenarios.Close() })
	m, err := Load(c, scenarios)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return m
}

func scenarioJSON(asset, id string, sizes []int) string {
	return scenarioJSONWithMask(asset, id, "none", sizes)
}

func scenarioJSONWithMask(asset, id, mask string, sizes []int) string {
	values := make([]string, len(sizes))
	for i, size := range sizes {
		values[i] = strconv.Itoa(size)
	}
	return `{"id":"` + id + `","group":"brand","asset":"` + asset + `","artwork":"icon","appearance":"adaptive","surface":"transparent","framing":"optical","mask":"` + mask + `","context":"web-navigation","sizes":[` + strings.Join(values, ",") + `]}`
}
