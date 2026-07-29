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
