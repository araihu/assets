package channels

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/araihu/assets/internal/campaigns"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/themes"
)

func TestResolveUsesCampaignThenDefault(t *testing.T) {
	input := fixtureInput(t)
	active, err := Resolve(input.withDate(t, "2026-10-31"))
	if err != nil || active.Source != "campaign" || active.Campaign == nil || active.Campaign.ID != "halloween-2026" {
		t.Fatalf("active = %#v, %v", active, err)
	}
	if got, want := active.Theme.CSSURL, "https://araihu.example/assets/releases/v0.1.0/themes/araihu-halloween.css"; got != want {
		t.Fatalf("active theme CSS URL = %q, want %q", got, want)
	}
	if got, want := active.Campaign.Toggle.EnabledIcon.URL, "https://araihu.example/assets/releases/v0.1.0/icons/ui/sprite.svg"; got != want {
		t.Fatalf("active enabled sprite URL = %q, want %q", got, want)
	}

	baseline, err := Resolve(input.withDate(t, "2026-11-01"))
	if err != nil || baseline.Source != "default" || baseline.Theme.ID != "araihu" || baseline.Campaign != nil {
		t.Fatalf("baseline = %#v, %v", baseline, err)
	}
}

func TestResolveRejectsMissingCatalogReference(t *testing.T) {
	input := fixtureInput(t)
	input.Campaigns.Campaigns[0].Brand.Logo = "missing-logo"
	if _, err := Resolve(input); err == nil || !strings.Contains(err.Error(), "missing-logo") {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestResolveRejectsPublicRootWithEmptyQuery(t *testing.T) {
	input := fixtureInput(t)
	input.PublicRoot = "https://araihu.example?"
	if _, err := Resolve(input); err == nil || !strings.Contains(err.Error(), "HTTPS origin") {
		t.Fatalf("Resolve() error = %v", err)
	}
}

func TestLoadDefaultRejectsUnknownFieldsAndMultipleDocuments(t *testing.T) {
	for _, raw := range []string{
		"schema_version: 1\nrelease: v0.1.0\ntheme: araihu\nextra: true\n",
		"schema_version: 1\nrelease: v0.1.0\ntheme: araihu\n---\nschema_version: 1\nrelease: v0.1.0\ntheme: araihu\n",
	} {
		if _, err := LoadDefault(fstest.MapFS{"default.yaml": {Data: []byte(raw)}}, "default.yaml"); err == nil {
			t.Fatal("LoadDefault() error = nil")
		}
	}
}

func TestEncodeIsDeterministicAndDigestCoversEmptyDigest(t *testing.T) {
	document, err := Resolve(fixtureInput(t).withDate(t, "2026-10-31"))
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	first, err := Encode(document)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	for range 20 {
		next, err := Encode(document)
		if err != nil || string(next) != string(first) {
			t.Fatalf("Encode() = %q, %v; want byte-identical %q", next, err, first)
		}
	}
	withoutDigest := document
	withoutDigest.Digest = ""
	payload, err := encodeCanonical(withoutDigest)
	if err != nil {
		t.Fatalf("encode canonical document: %v", err)
	}
	if got, want := document.Digest, digest(payload); got != want {
		t.Fatalf("document digest = %q, want %q", got, want)
	}
}

func TestSourceDefaultRemainsV011WhileLatestResolvesBaseline(t *testing.T) {
	root := os.DirFS("../..")
	defaultPromotion, err := LoadDefault(root, "manifests/default.yaml")
	if err != nil {
		t.Fatalf("LoadDefault() error = %v", err)
	}
	if defaultPromotion.Release != "v0.1.1" {
		t.Fatalf("default release = %q, want retained v0.1.1", defaultPromotion.Release)
	}
	snapshot := "dist"
	catalogFile, err := root.Open(snapshot + "/catalog.json")
	if err != nil {
		t.Fatal(err)
	}
	defer catalogFile.Close()
	assetCatalog, err := catalog.Decode(catalogFile)
	if err != nil {
		t.Fatalf("catalog.Decode() error = %v", err)
	}
	themeFile, err := root.Open(snapshot + "/themes.json")
	if err != nil {
		t.Fatal(err)
	}
	defer themeFile.Close()
	var themeCatalog themes.Catalog
	if err := json.NewDecoder(themeFile).Decode(&themeCatalog); err != nil {
		t.Fatal(err)
	}
	if assetCatalog.Release != "v0.2.0" || themeCatalog.Release != "v0.2.0" {
		t.Fatalf("latest release mismatch: catalog=%q themes=%q", assetCatalog.Release, themeCatalog.Release)
	}
	latestPromotion := defaultPromotion
	latestPromotion.Release = assetCatalog.Release
	date := mustDate(t, "2026-08-01")
	document, err := Resolve(Input{Date: date, Default: latestPromotion, Catalog: assetCatalog, Themes: themeCatalog, Campaigns: campaigns.Manifest{SchemaVersion: 1}, PublicRoot: "https://araihu.example"})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if document.Source != "default" || document.Theme.ID != "araihu" || document.Campaign != nil {
		t.Fatalf("resolved latest baseline = %#v", document)
	}
}

func (input Input) withDate(t *testing.T, raw string) Input {
	t.Helper()
	date, err := campaigns.ParseDate(raw)
	if err != nil {
		t.Fatalf("ParseDate(%q) error = %v", raw, err)
	}
	input.Date = date
	return input
}

func fixtureInput(t *testing.T) Input {
	t.Helper()
	date, err := campaigns.ParseDate("2026-10-01")
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	return Input{
		Date:    date,
		Default: Default{SchemaVersion: 1, Release: "v0.1.0", Theme: "araihu"},
		Catalog: catalog.Catalog{SchemaVersion: 1, Release: "v0.1.0", IdentityRevision: 11, Assets: []catalog.Asset{
			asset("ui-hi-16-solid-sparkles", "ui", "icons/ui/heroicons/16-solid-sparkles.svg", "hi-16-solid-sparkles"),
			asset("ui-hi-16-solid-moon", "ui", "icons/ui/heroicons/16-solid-moon.svg", "hi-16-solid-moon"),
			brandAsset("araihu-logo-tinted-transparent-optical", "logo", "brand/araihu/logo/tinted-transparent-optical.svg", "araihu-logo-tinted-transparent-optical"),
			brandAsset("araihu-icon-tinted-transparent-optical", "icon", "icons/brand/araihu-icon-tinted-transparent-optical.svg", "araihu-icon-tinted-transparent-optical"),
		}},
		Themes: themes.Catalog{SchemaVersion: 1, Release: "v0.1.0", TokenContract: "goshtoso-theme-v1", Themes: []themes.CatalogTheme{
			{ID: "araihu-halloween", CSSPath: "themes/araihu-halloween.css", TokenContract: "goshtoso-theme-v1", SHA256: hash},
			{ID: "araihu", CSSPath: "themes/araihu.css", TokenContract: "goshtoso-theme-v1", SHA256: hash},
		}},
		Campaigns: campaigns.Manifest{SchemaVersion: 1, Campaigns: []campaigns.Campaign{{
			ID: "halloween-2026", Enabled: true,
			StartsOn: mustDate(t, "2026-10-01"), EndsOn: mustDate(t, "2026-10-31"), Theme: "araihu-halloween",
			Toggle: campaigns.Toggle{EnabledIcon: campaigns.IconRef{Asset: "ui-hi-16-solid-sparkles", Mode: "sprite"}, DisabledIcon: campaigns.IconRef{Asset: "ui-hi-16-solid-moon", Mode: "asset"}},
			Brand:  campaigns.Brand{Logo: "araihu-logo-tinted-transparent-optical", Icon: "araihu-icon-tinted-transparent-optical"},
		}}},
		PublicRoot: "https://araihu.example",
	}
}

func asset(name, namespace, path, symbol string) catalog.Asset {
	return catalog.Asset{CanonicalName: name, Namespace: namespace, Path: path, Product: "araihu", Artwork: "icon", Appearance: "default", Surface: "transparent", Framing: "optical", Format: "svg", Dimensions: catalog.Dimensions{ViewBox: "0 0 16 16"}, SpriteSymbol: symbol, ColorBehavior: "monochrome", License: "MIT", Source: "fixture", SHA256: strings.Repeat("a", 64)}
}

func brandAsset(name, artwork, path, symbol string) catalog.Asset {
	asset := asset(name, "brand", path, symbol)
	asset.Artwork = artwork
	return asset
}

func mustDate(t *testing.T, raw string) campaigns.Date {
	t.Helper()
	date, err := campaigns.ParseDate(raw)
	if err != nil {
		t.Fatal(err)
	}
	return date
}
