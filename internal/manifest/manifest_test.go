package manifest

import (
	"os"
	"testing"
)

func TestUIRejectsMutableAndUnsafeSources(t *testing.T) {
	for _, mutate := range []func(*UI){
		func(m *UI) {
			m.Sources[0].BaseURL = "https://raw.githubusercontent.com/tailwindlabs/heroicons/master/src/"
		},
		func(m *UI) { m.Sources[0].Icons[0].Path = "../LICENSE" },
		func(m *UI) { m.Sources[0].Alias = "Hero Icons" },
	} {
		m := validUI(t)
		mutate(&m)
		if err := m.Validate(); err == nil {
			t.Fatal("Validate() error = nil")
		}
	}
}

func TestBrandRejectsDuplicateProducts(t *testing.T) {
	m := validBrand(t)
	m.Products = append(m.Products, m.Products[0])
	if err := m.Validate(); err == nil || !contains(err.Error(), "duplicate product") {
		t.Fatalf("Validate() error = %v, want duplicate product", err)
	}
}

func TestBrandRejectsCrossProductAliasCollisions(t *testing.T) {
	for _, tc := range []struct {
		name  string
		alias string
	}{
		{name: "alias to alias", alias: "xisnove"},
		{name: "alias to product id", alias: "x9"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := validBrand(t)
			m.Products[3].Aliases = append(m.Products[3].Aliases, tc.alias)
			if err := m.Validate(); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestLoadManifests(t *testing.T) {
	root := os.DirFS("../..")
	brand, err := LoadBrand(root, "manifests/brand.yaml")
	if err != nil {
		t.Fatalf("LoadBrand() error = %v", err)
	}
	if got, want := brand.IdentityRevision, 11; got != want {
		t.Fatalf("IdentityRevision = %d, want %d", got, want)
	}
	ui, err := LoadUI(root, "manifests/icons-ui.yaml")
	if err != nil {
		t.Fatalf("LoadUI() error = %v", err)
	}
	if got, want := len(ui.Sources[0].Icons), 67; got != want {
		t.Fatalf("icon count = %d, want %d", got, want)
	}
}

func TestBrandRequiresPinnedOriginalSourceHashes(t *testing.T) {
	m := validBrand(t)
	if got, want := len(m.Products), 5; got != want {
		t.Fatalf("products = %d, want %d", got, want)
	}
	for _, product := range m.Products {
		if got, want := len(product.Sources["original"]), 4; got != want {
			t.Errorf("%s original sources = %d, want %d", product.ID, got, want)
		}
		if got, want := len(product.SourceHashes), 4; got != want {
			t.Errorf("%s source hashes = %d, want %d", product.ID, got, want)
		}
		for kind, source := range product.Sources["original"] {
			if source == "" || product.SourceHashes[kind] == "" {
				t.Errorf("%s %s missing pinned source or hash", product.ID, kind)
			}
		}
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	root := os.DirFS("testdata")
	if _, err := LoadBrand(root, "brand-unknown.yaml"); err == nil {
		t.Fatal("LoadBrand() error = nil")
	}
	if _, err := LoadUI(root, "ui-unknown.yaml"); err == nil {
		t.Fatal("LoadUI() error = nil")
	}
}

func validBrand(t *testing.T) Brand {
	t.Helper()
	brand, err := LoadBrand(os.DirFS("../.."), "manifests/brand.yaml")
	if err != nil {
		t.Fatalf("LoadBrand() error = %v", err)
	}
	return brand
}

func validUI(t *testing.T) UI {
	t.Helper()
	ui, err := LoadUI(os.DirFS("../.."), "manifests/icons-ui.yaml")
	if err != nil {
		t.Fatalf("LoadUI() error = %v", err)
	}
	return ui
}

func contains(s, want string) bool {
	for i := 0; i+len(want) <= len(s); i++ {
		if s[i:i+len(want)] == want {
			return true
		}
	}
	return false
}
