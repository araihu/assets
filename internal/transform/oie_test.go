package transform

import (
	"bytes"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/svgasset"
)

func TestOieKeepsApprovedFlatFaceAcrossLightAndDark(t *testing.T) {
	result := mustBuildBrand(t)
	for _, asset := range result.Assets {
		if asset.Product == "oie" && !strings.Contains(asset.License, "Apache-2.0") {
			t.Fatal("Noto artwork license missing from catalog", asset.Path)
		}
	}
	for _, appearance := range []string{"light", "dark"} {
		icon := result.Files["dist/icons/brand/oie-icon-"+appearance+"-transparent-optical.svg"]
		for _, color := range []string{"#6092ab", "#212121", "#274c5e"} {
			if !bytes.Contains(icon, []byte(`fill="`+color+`"`)) {
				t.Fatalf("%s lost approved face paint %s", appearance, color)
			}
		}
		if bytes.Contains(icon, []byte("Gradient")) {
			t.Fatal("flat identity gained shading")
		}
	}
	dark := result.Files["dist/brand/oie/logo/dark-transparent-optical.svg"]
	if !bytes.Contains(dark, []byte("#fffaf4")) {
		t.Fatal("dark logo has no readable light lettering")
	}
	mono := result.Files["dist/icons/brand/oie-icon-monochrome-transparent-optical.svg"]
	if !bytes.Contains(mono, []byte(`fill="black"`)) || !bytes.Contains(mono, []byte(`fill="white"`)) || !bytes.Contains(mono, []byte(`fill="currentColor"`)) {
		t.Fatal("monochrome face lost structural mask or runtime ink")
	}
	adaptive := result.Files["dist/icons/brand/oie-icon-adaptive-transparent-optical.svg"]
	if _, err := svgasset.ParseGenerated(adaptive); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(adaptive, []byte("--araihu-logo-auto-paint-6092ab: #6092ab;")) {
		t.Fatal("adaptive mode changes approved blue")
	}
}
