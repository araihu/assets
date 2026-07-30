package themes

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoadRejectsUnknownField(t *testing.T) {
	_, err := Load(fstest.MapFS{"themes.yaml": {Data: []byte("schema_version: 1\nthemes: []\nextra: true\n")}}, "themes.yaml")
	if err == nil || !strings.Contains(err.Error(), "field extra not found") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsMultipleDocuments(t *testing.T) {
	_, err := Load(fstest.MapFS{"themes.yaml": {Data: []byte("schema_version: 1\ntoken_contract: goshtoso-theme-v1\nthemes: []\n---\nschema_version: 1\n")}}, "themes.yaml")
	if err == nil || !strings.Contains(err.Error(), "multiple YAML documents") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestValidateRejectsDuplicateThemeAndTraversal(t *testing.T) {
	manifest := Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []Theme{
		{ID: "araihu", CSSPath: "themes/araihu.css"},
		{ID: "araihu", CSSPath: "../secret.css"},
	}}
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() succeeded")
	}
}

func TestManifestCatalogSortsThemesAndRetainsHashes(t *testing.T) {
	manifest := Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []Theme{
		{ID: "zebra", CSSPath: "themes/zebra.css", SHA256: strings.Repeat("a", 64)},
		{ID: "araihu", CSSPath: "themes/araihu.css", SHA256: strings.Repeat("b", 64)},
	}}
	catalog, err := manifest.Catalog("v0.1.0")
	if err != nil {
		t.Fatalf("Catalog() error = %v", err)
	}
	if got, want := catalog.Themes[0], (CatalogTheme{ID: "araihu", CSSPath: "themes/araihu.css", TokenContract: "goshtoso-theme-v1", SHA256: strings.Repeat("b", 64)}); got != want {
		t.Fatalf("catalog.Themes[0] = %#v, want %#v", got, want)
	}
	if got := catalog.Themes[1].ID; got != "zebra" {
		t.Fatalf("catalog themes are not sorted: %q", got)
	}
}

func TestEncodeProducesCanonicalCatalog(t *testing.T) {
	catalog := Catalog{SchemaVersion: 1, Release: "v0.1.0", TokenContract: "goshtoso-theme-v1", Themes: []CatalogTheme{{
		ID: "araihu", CSSPath: "themes/araihu.css", TokenContract: "goshtoso-theme-v1", SHA256: strings.Repeat("a", 64),
	}}}
	got, err := Encode(catalog)
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	want := "{\n  \"schemaVersion\": 1,\n  \"release\": \"v0.1.0\",\n  \"tokenContract\": \"goshtoso-theme-v1\",\n  \"themes\": [\n    {\n      \"id\": \"araihu\",\n      \"cssPath\": \"themes/araihu.css\",\n      \"tokenContract\": \"goshtoso-theme-v1\",\n      \"sha256\": \"" + strings.Repeat("a", 64) + "\"\n    }\n  ]\n}\n"
	if string(got) != want {
		t.Fatalf("Encode() = %s, want %s", got, want)
	}
}
