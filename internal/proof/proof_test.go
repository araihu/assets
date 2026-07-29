package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
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

func TestLoadRejectsMissingCatalogBackedMaskable512Master(t *testing.T) {
	c := fixtureCatalog(t)
	c.Assets = slices.DeleteFunc(c.Assets, func(asset catalog.Asset) bool {
		return asset.CanonicalName == "platform-web-araihu-icon-maskable-512-png"
	})
	_, err := Load(c, strings.NewReader(readFixtureScenarios(t)))
	if err == nil || !strings.Contains(err.Error(), "maskable 512") {
		t.Fatalf("Load() error = %v, want missing catalog-backed maskable 512 master", err)
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

func TestBuildRejectsCatalogMutatedAfterLoad(t *testing.T) {
	m := fixtureModel(t)
	m.Catalog.Assets = append(m.Catalog.Assets, m.Catalog.Assets[0])

	err := Build(m, fixtureDistributionFS(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "duplicate canonicalName") {
		t.Fatalf("Build() error = %v, want duplicate canonicalName", err)
	}
}

func TestBuildRejectsSemanticMutationAfterLoad(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*Model)
	}{
		{"scenario context", func(m *Model) { m.Scenarios[0].Context = "mobile-app-bar" }},
		{"catalog release", func(m *Model) { m.Catalog.Release = "v0.1.1" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureModel(t)
			tc.mutate(&m)

			err := Build(m, fixtureDistributionFS(), &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "semantic provenance mismatch") {
				t.Fatalf("Build() error = %v, want semantic provenance mismatch", err)
			}
		})
	}
}

func TestBuildRejectsModelWithoutLoadProvenance(t *testing.T) {
	loaded := fixtureModel(t)
	manual := Model{
		Catalog:    loaded.Catalog,
		Products:   loaded.Products,
		Scenarios:  loaded.Scenarios,
		ExactSizes: loaded.ExactSizes,
	}

	err := Build(manual, fixtureDistributionFS(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "missing model provenance") {
		t.Fatalf("Build() error = %v, want missing model provenance", err)
	}
}

func TestBuildRejectsNoncanonicalDerivedModel(t *testing.T) {
	for _, tc := range []struct {
		name   string
		field  string
		mutate func(*Model)
	}{
		{"nil products", "Products", func(m *Model) { m.Products = nil }},
		{"nil exact sizes", "ExactSizes", func(m *Model) { m.ExactSizes = nil }},
		{"reversed catalog assets", "Catalog", func(m *Model) { slices.Reverse(m.Catalog.Assets) }},
		{"reversed scenarios", "Scenarios", func(m *Model) { slices.Reverse(m.Scenarios) }},
		{"altered product proof", "Products", func(m *Model) {
			m.Products[0].Assets = slices.Clone(m.Products[0].Assets)
			m.Products[0].Assets[0].Path = "icons/brand/altered.svg"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := fixtureModel(t)
			tc.mutate(&m)

			err := Build(m, fixtureDistributionFS(), &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), "noncanonical "+tc.field) {
				t.Fatalf("Build() error = %v, want noncanonical %s", err, tc.field)
			}
		})
	}
}

func TestBuildRejectsNonRegularReferencedDistributionFile(t *testing.T) {
	m := fixtureModel(t)
	distribution := fixtureDistributionFS()
	distribution["icons/brand/araihu-icon.svg"] = &fstest.MapFile{Mode: fs.ModeDir}

	err := Build(m, distribution, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "is not regular") {
		t.Fatalf("Build() error = %v, want non-regular file", err)
	}
}

func TestBuildRejectsMissingUISpriteDistributionFile(t *testing.T) {
	m := fixtureModel(t)
	distribution := fixtureDistributionFS()
	delete(distribution, "icons/ui/sprite.svg")

	err := Build(m, distribution, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "icons/ui/sprite.svg") {
		t.Fatalf("Build() error = %v, want missing UI sprite", err)
	}
}

func TestBuildReturnsShortWriteAndWriterError(t *testing.T) {
	outputFailure := errors.New("output failed")
	for _, tc := range []struct {
		name string
		out  io.Writer
		want error
	}{
		{"short write", shortWriter{}, io.ErrShortWrite},
		{"writer error", errorWriter{err: outputFailure}, outputFailure},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Build(fixtureModel(t), fixtureDistributionFS(), tc.out)
			if !errors.Is(err, tc.want) {
				t.Fatalf("Build() error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestLoadSortsModelDeterministicallyFromPermutedInputs(t *testing.T) {
	want := loadProduction(t)
	c := want.Catalog
	c.Assets = slices.Clone(c.Assets)
	slices.Reverse(c.Assets)
	scenarios := slices.Clone(want.Scenarios)
	slices.Reverse(scenarios)
	document, err := json.Marshal(struct {
		Scenarios []Scenario `json:"scenarios"`
	}{Scenarios: scenarios})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	got, err := Load(c, bytes.NewReader(document))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.provenance != want.provenance {
		t.Fatal("semantic provenance differs for permuted inputs")
	}
	if !slices.IsSortedFunc(got.Catalog.Assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) }) {
		t.Fatal("Catalog assets are not sorted by canonicalName")
	}
	if !slices.IsSortedFunc(got.Scenarios, func(a, b Scenario) int { return strings.Compare(a.ID, b.ID) }) {
		t.Fatal("Scenarios are not sorted by ID")
	}
	if !slices.IsSortedFunc(got.Products, func(a, b ProductProof) int { return strings.Compare(a.ID, b.ID) }) {
		t.Fatal("Products are not sorted by ID")
	}
	for _, product := range got.Products {
		if !slices.IsSortedFunc(product.Assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) }) {
			t.Fatalf("product %q assets are not sorted by canonicalName", product.ID)
		}
	}
	if !slices.IsSorted(got.ExactSizes) {
		t.Fatalf("ExactSizes are not sorted: %v", got.ExactSizes)
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

func TestProductionMasterProofUsesCatalogMaskable512Semantics(t *testing.T) {
	m := loadProduction(t)
	var master catalog.Asset
	for _, platform := range m.Platform {
		if platform.Product == "araihu" {
			master = platform.Master
			break
		}
	}
	if master.CanonicalName != "platform-web-araihu-icon-maskable-512-png" || master.Path != "platform/web/araihu/icon-maskable-512.png" || master.Artwork != "icon" || master.Appearance != "light" || master.Surface != "plate" || master.Framing != "launcher" || master.Dimensions.Width != 512 || master.Dimensions.Height != 512 {
		t.Fatalf("Arai Hû literal master = %#v, want catalog-backed light plated launcher 512", master)
	}
	var output bytes.Buffer
	if err := Build(m, os.DirFS(filepath.Join("..", "..", "dist")), &output); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	html := output.String()
	for _, want := range []string{
		`src="assets/platform/web/araihu/icon-maskable-512.png"`,
		`aria-label="Arai Hû icon, light plate launcher, 512 pixels"`,
		`data-canonical-name="platform-web-araihu-icon-maskable-512-png"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("production proof missing %q", want)
		}
	}
	if strings.Contains(html, `data-canonical-name="platform-web-araihu-icon-maskable-512-png"><img src="assets/platform/web/araihu/icon-512.png"`) {
		t.Fatal("literal master mislabels the transparent optical 512 artifact")
	}
}

func TestBuildEmitsSemanticLabelsAndMetricsTable(t *testing.T) {
	html := buildFixture(t)
	for _, want := range []string{
		`<link rel="stylesheet" href="styles.css">`,
		`<script defer src="app.js"></script>`,
		`<a class="skip-link" href="#proof-main">Skip to proof evidence</a>`,
		`<table aria-label="Normalized SVG art bounds">`,
		`aria-label="Arai Hû icon, transparent adaptive optical, 16 pixels"`,
		`aria-label="Arai Hû icon, light plate launcher, 512 pixels"`,
		`href="assets/NOTICE"`,
		`href="assets/licenses/Apache-2.0.txt"`,
		`href="assets/licenses/heroicons-MIT.txt"`,
		`href="assets/icons/ui/heroicons/provenance.json"`,
		`<section aria-labelledby="family-comparison-title">`,
		`<section aria-labelledby="web-contexts-title">`,
		`<section aria-labelledby="mobile-contexts-title">`,
		`<section aria-labelledby="platform-packages-title">`,
		`<section aria-labelledby="ui-sprite-rail-title">`,
		`<section aria-labelledby="findings-title">`,
		`<section aria-labelledby="license-provenance-title">`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("Build() output missing %q", want)
		}
	}
	if strings.Contains(html, `src="assets/concepts/v11/`) || strings.Contains(html, "v11/") {
		t.Fatalf("Build() output contains a legacy V11 URL: %s", html)
	}
}

func TestBuildEmitsReviewFilteringAndMaskEvidence(t *testing.T) {
	html := buildProduction(t)
	for _, want := range []string{
		`data-evidence`,
		`data-proof-surface="transparent"`,
		`data-proof-appearance="light"`,
		`data-mask="circle"`,
		`class="mask-frame mask-circle"`,
		`class="transparent-stress checker"`,
		`class="transparent-stress paper"`,
		`class="transparent-stress midnight"`,
		`<caption>Catalog-backed geometry for the selected product evidence.</caption>`,
		`id="evidence-summary"`,
		`data-reset`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("Build() output missing review contract %q", want)
		}
	}
}

func TestProofAssetsContainRequiredInteractionContracts(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "site", "proof", "styles.css"))
	if err != nil {
		t.Fatalf("ReadFile styles.css: %v", err)
	}
	js, err := os.ReadFile(filepath.Join("..", "..", "site", "proof", "app.js"))
	if err != nil {
		t.Fatalf("ReadFile app.js: %v", err)
	}
	for _, want := range []string{
		`@media (prefers-reduced-motion: reduce)`,
		`:focus-visible`,
		`.exact-size-rail`,
		`.metric-table`,
		`html[data-mode="plate"]`,
	} {
		if !strings.Contains(string(css), want) {
			t.Fatalf("styles.css missing %q", want)
		}
	}
	for _, want := range []string{
		`history.replaceState`,
		`URLSearchParams`,
		`data-product`,
		`data-mode`,
		`data-scheme`,
		`allowed[key].has`,
	} {
		if !strings.Contains(string(js), want) {
			t.Fatalf("app.js missing %q", want)
		}
	}
	if strings.Contains(string(js), "fetch(") {
		t.Fatal("app.js must not fetch")
	}
}

func TestProofAssetsConstrainOuterOverflowWithoutRemovingReviewRails(t *testing.T) {
	css, err := os.ReadFile(filepath.Join("..", "..", "site", "proof", "styles.css"))
	if err != nil {
		t.Fatalf("ReadFile styles.css: %v", err)
	}
	for _, selector := range []string{
		"main", "section", ".family-comparison", ".exact-size-rail, .master-rail, .ui-sprite-rail", ".metric-table",
	} {
		pattern := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{[^}]*min-inline-size:\s*0;[^}]*max-inline-size:\s*100%;`)
		if !pattern.Match(css) {
			t.Fatalf("%s must constrain its outer inline contribution", selector)
		}
	}
	for _, selector := range []string{".family-comparison", ".exact-size-rail, .master-rail, .ui-sprite-rail", ".metric-table"} {
		pattern := regexp.MustCompile(regexp.QuoteMeta(selector) + `\s*\{[^}]*overflow-x:\s*auto;`)
		if !pattern.Match(css) {
			t.Fatalf("%s must retain internal horizontal review scrolling", selector)
		}
	}
}

func TestBuildMatchesFixtureGoldenDeterministically(t *testing.T) {
	first := buildFixture(t)
	second := buildFixture(t)
	if first != second {
		t.Fatal("Build() output is not deterministic")
	}
	want, err := os.ReadFile("testdata/golden/index.html")
	if err != nil {
		t.Fatalf("ReadFile golden: %v", err)
	}
	if first != string(want) {
		t.Fatal("Build() output differs from golden")
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
	m, err := Load(fixtureCatalog(t), strings.NewReader(readFixtureScenarios(t)))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	return m
}

func readFixtureScenarios(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("testdata/scenarios.json")
	if err != nil {
		t.Fatalf("ReadFile scenario fixture: %v", err)
	}
	return string(b)
}

func buildFixture(t *testing.T) string {
	t.Helper()
	var output bytes.Buffer
	if err := Build(fixtureModel(t), fixtureDistributionFS(), &output); err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	return output.String()
}

func buildProduction(t *testing.T) string {
	t.Helper()
	var output bytes.Buffer
	if err := Build(loadProduction(t), os.DirFS(filepath.Join("..", "..", "dist")), &output); err != nil {
		t.Fatalf("Build() production error = %v", err)
	}
	return output.String()
}

func fixtureDistributionFS() fstest.MapFS {
	return fstest.MapFS{
		"icons/brand/araihu-icon.svg":                                            &fstest.MapFile{Data: []byte("svg")},
		"icons/ui/heroicons/16-solid-check.svg":                                  &fstest.MapFile{Data: []byte("svg")},
		"icons/ui/sprite.svg":                                                    &fstest.MapFile{Data: []byte("svg")},
		"platform/web/araihu/icon-512.png":                                       &fstest.MapFile{Data: []byte("png")},
		"platform/web/araihu/icon-maskable-512.png":                              &fstest.MapFile{Data: []byte("png")},
		"platform/web/araihu/manifest-icons.json":                                &fstest.MapFile{Data: []byte("json")},
		"platform/android/araihu/res/mipmap-anydpi-v26/ic_launcher.xml":          &fstest.MapFile{Data: []byte("xml")},
		"platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/Contents.json": &fstest.MapFile{Data: []byte("json")},
		"NOTICE":                             &fstest.MapFile{Data: []byte("notice")},
		"licenses/Apache-2.0.txt":            &fstest.MapFile{Data: []byte("apache")},
		"licenses/heroicons-MIT.txt":         &fstest.MapFile{Data: []byte("mit")},
		"icons/ui/heroicons/provenance.json": &fstest.MapFile{Data: []byte("provenance")},
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }

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
