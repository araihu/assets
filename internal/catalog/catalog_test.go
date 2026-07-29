package catalog

import (
	"bytes"
	"os"
	"slices"
	"strings"
	"testing"
)

func TestValidateRejectsDuplicateCanonicalName(t *testing.T) {
	c := validCatalog(t)
	c.Assets = append(c.Assets, c.Assets[0])
	if err := Validate(c); err == nil || !strings.Contains(err.Error(), "duplicate canonicalName") {
		t.Fatalf("Validate() error = %v, want duplicate canonicalName", err)
	}
}

func TestValidateRejectsInvalidCatalogEntries(t *testing.T) {
	for _, mutate := range []struct {
		name string
		fn   func(*Catalog)
	}{
		{"duplicate sprite symbol", func(c *Catalog) { c.Assets[1].SpriteSymbol = c.Assets[0].SpriteSymbol }},
		{"source path", func(c *Catalog) { c.Assets[0].Path = "source/brand/original/araihu.svg" }},
		{"non artifact path", func(c *Catalog) { c.Assets[0].Path = "dist/icons/brand/araihu.svg" }},
		{"unknown format", func(c *Catalog) { c.Assets[0].Format = "webp" }},
		{"unknown color behavior", func(c *Catalog) { c.Assets[0].ColorBehavior = "rainbow" }},
		{"non-finite viewBox", func(c *Catalog) { c.Assets[0].Dimensions.ViewBox = "0 0 +Inf 16" }},
		{"missing provenance", func(c *Catalog) { c.Assets[0].SHA256 = "" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			c := validCatalog(t)
			mutate.fn(&c)
			if err := Validate(c); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func TestEncodeIsDeterministic(t *testing.T) {
	c := validCatalog(t)
	slices.Reverse(c.Assets)
	var a, b bytes.Buffer
	if err := Encode(&a, c); err != nil {
		t.Fatalf("Encode(first) error = %v", err)
	}
	if err := Encode(&b, c); err != nil {
		t.Fatalf("Encode(second) error = %v", err)
	}
	if !bytes.Equal(a.Bytes(), b.Bytes()) {
		t.Fatalf("Encode() differs:\n%s\n---\n%s", a.String(), b.String())
	}
	if got, want := a.String(), "\n"; !strings.HasSuffix(got, want) {
		t.Fatalf("Encode() must end in newline: %q", got)
	}
	if first, second := strings.Index(a.String(), "brand-araihu-icon"), strings.Index(a.String(), "ui-hi-16-solid-check"); first < 0 || second < 0 || first > second {
		t.Fatalf("Encode() asset order wrong:\n%s", a.String())
	}
}

func TestDecodeRejectsUnknownAndInvalidJSON(t *testing.T) {
	for _, input := range []string{
		`{"schemaVersion":1,"release":"v0.1.0","identityRevision":11,"assets":[],"extra":true}`,
		`{"schemaVersion":2,"release":"v0.1.0","identityRevision":11,"assets":[]}`,
		fixture(t, "catalog.json") + ` {}`,
	} {
		if _, err := Decode(strings.NewReader(input)); err == nil {
			t.Fatalf("Decode(%q) error = nil", input)
		}
	}
}

func TestValidatePatchCompatibilityAllowsOnlyAdditions(t *testing.T) {
	previous := validCatalog(t)
	next := validCatalog(t)
	next.Assets = append(next.Assets, Asset{
		CanonicalName: "ui-hi-16-solid-plus",
		Namespace:     "ui",
		Path:          "icons/ui/heroicons/16-solid-plus.svg",
		Product:       "heroicons",
		Artwork:       "icon",
		Appearance:    "default",
		Surface:       "transparent",
		Framing:       "optical",
		Format:        "svg",
		Dimensions:    Dimensions{ViewBox: "0 0 16 16"},
		SpriteSymbol:  "hi-16-solid-plus",
		ColorBehavior: "monochrome",
		License:       "MIT",
		Source:        "heroicons@v2.2.0",
		SHA256:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	})
	if err := ValidatePatchCompatibility(previous, next); err != nil {
		t.Fatalf("ValidatePatchCompatibility(addition) error = %v", err)
	}

	for _, mutate := range []struct {
		name string
		fn   func(*Catalog)
	}{
		{"removal", func(c *Catalog) { c.Assets = c.Assets[1:] }},
		{"symbol change", func(c *Catalog) { c.Assets[0].SpriteSymbol = "changed" }},
		{"semantic change", func(c *Catalog) { c.Assets[1].ColorBehavior = "monochrome" }},
	} {
		t.Run(mutate.name, func(t *testing.T) {
			candidate := validCatalog(t)
			mutate.fn(&candidate)
			if err := ValidatePatchCompatibility(previous, candidate); err == nil {
				t.Fatal("ValidatePatchCompatibility() error = nil")
			}
		})
	}
}

func validCatalog(t *testing.T) Catalog {
	t.Helper()
	c, err := Decode(strings.NewReader(fixture(t, "catalog.json")))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	return c
}

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("ReadFile(%q) error = %v", name, err)
	}
	return string(b)
}
