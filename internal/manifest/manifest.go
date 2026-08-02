// Package manifest loads and validates the versioned asset input manifests.
package manifest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	schemaVersion = 1
)

var (
	lowerKebab       = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	lowerSHA256      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	lowerHexColor    = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	expectedProducts = map[string]string{
		"araihu": "Arai Hu", "goshtoso": "Goshtoso", "manja": "Manja", "paje": "Paje", "x9": "X9",
	}
)

// Brand describes the versioned Arai Hu product identity inputs.
type Brand struct {
	SchemaVersion    int                `yaml:"schema_version"`
	IdentityRevision int                `yaml:"identity_revision"`
	Products         []Product          `yaml:"products"`
	Palettes         map[string]Palette `yaml:"palettes"`
	Recipes          []BrandRecipe      `yaml:"recipes"`
}

// Product identifies one published product identity.
type Product struct {
	ID           string                       `yaml:"id"`
	DisplayName  string                       `yaml:"display_name"`
	Aliases      []string                     `yaml:"aliases"`
	Sources      map[string]map[string]string `yaml:"sources"`
	SourceHashes map[string]string            `yaml:"source_hashes"`
}

// Palette holds a named color vocabulary.
type Palette struct {
	Name   string            `yaml:"name"`
	Colors map[string]string `yaml:"colors"`
}

// BrandRecipe declares how a brand asset is composed.
type BrandRecipe struct {
	Name          string `yaml:"name"`
	Artwork       string `yaml:"artwork"`
	Appearance    string `yaml:"appearance"`
	Surface       string `yaml:"surface"`
	Framing       string `yaml:"framing"`
	ColorBehavior string `yaml:"color_behavior"`
}

// LoadBrand decodes a single strict YAML brand manifest and validates it.
func LoadBrand(fsys fs.FS, name string) (Brand, error) {
	var brand Brand
	if err := decode(fsys, name, &brand); err != nil {
		return Brand{}, err
	}
	if err := brand.Validate(); err != nil {
		return Brand{}, err
	}
	return brand, nil
}

func decode(fsys fs.FS, name string, dst any) error {
	if !fs.ValidPath(name) {
		return fmt.Errorf("manifest path %q is invalid", name)
	}
	f, err := fsys.Open(name)
	if err != nil {
		return fmt.Errorf("open manifest %q: %w", name, err)
	}
	defer f.Close()
	decoder := yaml.NewDecoder(f)
	decoder.KnownFields(true)
	if err := decoder.Decode(dst); err != nil {
		return fmt.Errorf("decode manifest %q: %w", name, err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("decode manifest %q: multiple documents", name)
		}
		return fmt.Errorf("decode manifest %q: %w", name, err)
	}
	return nil
}

// Validate checks that a brand manifest is closed, versioned, and unambiguous.
func (brand Brand) Validate() error {
	if brand.SchemaVersion != schemaVersion {
		return fmt.Errorf("brand schema_version = %d, want %d", brand.SchemaVersion, schemaVersion)
	}
	if brand.IdentityRevision != 11 {
		return fmt.Errorf("brand identity_revision = %d, want 11", brand.IdentityRevision)
	}
	seenProducts := make(map[string]struct{}, len(brand.Products))
	identifiers := make(map[string]string, len(brand.Products)*2)
	for _, product := range brand.Products {
		if !lowerKebab.MatchString(product.ID) {
			return fmt.Errorf("invalid product id %q", product.ID)
		}
		if _, ok := seenProducts[product.ID]; ok {
			return fmt.Errorf("duplicate product %q", product.ID)
		}
		seenProducts[product.ID] = struct{}{}
		if _, ok := expectedProducts[product.ID]; !ok {
			return fmt.Errorf("unknown product %q", product.ID)
		}
		if owner, ok := identifiers[product.ID]; ok {
			return fmt.Errorf("duplicate identifier %q: product %q conflicts with %s", product.ID, product.ID, owner)
		}
		identifiers[product.ID] = fmt.Sprintf("product %q", product.ID)
		if strings.TrimSpace(product.DisplayName) == "" {
			return fmt.Errorf("product %q display_name is empty", product.ID)
		}
		if err := validateAliases(product); err != nil {
			return err
		}
		for _, alias := range product.Aliases {
			if owner, ok := identifiers[alias]; ok {
				return fmt.Errorf("duplicate identifier %q: alias for product %q conflicts with %s", alias, product.ID, owner)
			}
			identifiers[alias] = fmt.Sprintf("alias for product %q", product.ID)
		}
		if err := validateProductSources(product); err != nil {
			return err
		}
	}
	if len(brand.Products) != len(expectedProducts) {
		return fmt.Errorf("brand products = %d, want %d", len(brand.Products), len(expectedProducts))
	}
	for id := range expectedProducts {
		if _, ok := seenProducts[id]; !ok {
			return fmt.Errorf("missing product %q", id)
		}
	}
	if err := validatePalettes(brand.Palettes); err != nil {
		return err
	}
	return validateRecipes(brand.Recipes)
}

func validateAliases(product Product) error {
	seen := make(map[string]struct{}, len(product.Aliases))
	for _, alias := range product.Aliases {
		if !lowerKebab.MatchString(alias) {
			return fmt.Errorf("invalid alias %q for product %q", alias, product.ID)
		}
		if _, ok := seen[alias]; ok {
			return fmt.Errorf("duplicate alias %q for product %q", alias, product.ID)
		}
		seen[alias] = struct{}{}
	}
	if product.ID == "x9" && !slices.Contains(product.Aliases, "xisnove") {
		return errors.New("product \"x9\" must include alias \"xisnove\"")
	}
	return nil
}

func validateProductSources(product Product) error {
	if len(product.Sources) != 1 {
		return fmt.Errorf("product %q must declare only original source provenance", product.ID)
	}
	for source, files := range product.Sources {
		if !lowerKebab.MatchString(source) {
			return fmt.Errorf("invalid source %q for product %q", source, product.ID)
		}
		if len(files) == 0 {
			return fmt.Errorf("source %q for product %q is empty", source, product.ID)
		}
		for kind, path := range files {
			if !lowerKebab.MatchString(kind) || !fs.ValidPath(path) {
				return fmt.Errorf("invalid source entry %q=%q for product %q", kind, path, product.ID)
			}
		}
	}
	original, ok := product.Sources["original"]
	if !ok || len(original) != 4 {
		return fmt.Errorf("product %q must declare four original sources", product.ID)
	}
	if len(product.SourceHashes) != len(original) {
		return fmt.Errorf("product %q source_hashes = %d, want %d", product.ID, len(product.SourceHashes), len(original))
	}
	for kind := range original {
		hash, ok := product.SourceHashes[kind]
		if !ok || !lowerSHA256.MatchString(hash) {
			return fmt.Errorf("product %q has invalid source hash for %q", product.ID, kind)
		}
	}
	for kind := range product.SourceHashes {
		if _, ok := original[kind]; !ok {
			return fmt.Errorf("product %q source hash %q has no original source", product.ID, kind)
		}
	}
	return nil
}

func validatePalettes(palettes map[string]Palette) error {
	required := []string{"light", "dark", "grayscale", "tinted"}
	if len(palettes) != len(required) {
		return fmt.Errorf("brand palettes = %d, want %d", len(palettes), len(required))
	}
	for _, id := range required {
		palette, ok := palettes[id]
		if !ok || palette.Name != id || len(palette.Colors) != 3 {
			return fmt.Errorf("invalid palette %q", id)
		}
		for _, color := range []string{"surface", "ink", "signal"} {
			value, ok := palette.Colors[color]
			if !ok || !lowerHexColor.MatchString(value) {
				return fmt.Errorf("invalid palette color %q in %q", color, id)
			}
		}
	}
	return nil
}

func validateRecipes(recipes []BrandRecipe) error {
	expected := approvedBrandRecipes()
	if len(recipes) != len(expected) {
		return fmt.Errorf("brand recipe matrix = %d, want %d", len(recipes), len(expected))
	}
	seenNames := make(map[string]struct{}, len(recipes))
	seenTuples := make(map[string]struct{}, len(recipes))
	for _, recipe := range recipes {
		if !lowerKebab.MatchString(recipe.Name) || !allowedValue(recipe.Artwork, "icon", "logo") || !allowedValue(recipe.Appearance, "adaptive", "light", "dark", "monochrome", "grayscale", "tinted") || !allowedValue(recipe.Surface, "transparent", "plate") || !allowedValue(recipe.Framing, "optical", "launcher") || !allowedValue(recipe.ColorBehavior, "protected", "monochrome") {
			return fmt.Errorf("invalid recipe %q", recipe.Name)
		}
		if _, ok := seenNames[recipe.Name]; ok {
			return fmt.Errorf("duplicate recipe %q", recipe.Name)
		}
		seenNames[recipe.Name] = struct{}{}
		tuple := recipeTuple(recipe)
		if _, ok := seenTuples[tuple]; ok {
			return fmt.Errorf("duplicate recipe tuple %q", tuple)
		}
		seenTuples[tuple] = struct{}{}
		want, ok := expected[recipe.Name]
		if !ok || recipe != want {
			return fmt.Errorf("recipe %q is outside approved matrix", recipe.Name)
		}
	}
	for name := range expected {
		if _, ok := seenNames[name]; !ok {
			return fmt.Errorf("missing required recipe %q", name)
		}
	}
	return nil
}

func allowedValue(value string, allowed ...string) bool {
	return slices.Contains(allowed, value)
}

func recipeTuple(recipe BrandRecipe) string {
	return strings.Join([]string{recipe.Artwork, recipe.Appearance, recipe.Surface, recipe.Framing, recipe.ColorBehavior}, "/")
}

func approvedBrandRecipes() map[string]BrandRecipe {
	recipes := []BrandRecipe{
		brandRecipe("icon", "adaptive", "transparent", "optical", "protected"),
		brandRecipe("icon", "adaptive", "plate", "optical", "protected"),
		brandRecipe("icon", "light", "transparent", "optical", "protected"),
		brandRecipe("icon", "light", "plate", "optical", "protected"),
		brandRecipe("icon", "dark", "transparent", "optical", "protected"),
		brandRecipe("icon", "dark", "plate", "optical", "protected"),
		brandRecipe("icon", "monochrome", "transparent", "optical", "monochrome"),
		brandRecipe("icon", "grayscale", "transparent", "optical", "protected"),
		brandRecipe("icon", "grayscale", "plate", "optical", "protected"),
		brandRecipe("icon", "tinted", "transparent", "optical", "protected"),
		brandRecipe("icon", "tinted", "plate", "optical", "protected"),
		brandRecipe("icon", "adaptive", "plate", "launcher", "protected"),
		brandRecipe("icon", "tinted", "plate", "launcher", "protected"),
		brandRecipe("logo", "adaptive", "transparent", "optical", "protected"),
		brandRecipe("logo", "adaptive", "plate", "optical", "protected"),
		brandRecipe("logo", "light", "transparent", "optical", "protected"),
		brandRecipe("logo", "light", "plate", "optical", "protected"),
		brandRecipe("logo", "dark", "transparent", "optical", "protected"),
		brandRecipe("logo", "dark", "plate", "optical", "protected"),
		brandRecipe("logo", "monochrome", "transparent", "optical", "monochrome"),
		brandRecipe("logo", "grayscale", "transparent", "optical", "protected"),
		brandRecipe("logo", "grayscale", "plate", "optical", "protected"),
		brandRecipe("logo", "tinted", "transparent", "optical", "protected"),
		brandRecipe("logo", "tinted", "plate", "optical", "protected"),
	}
	result := make(map[string]BrandRecipe, len(recipes))
	for _, recipe := range recipes {
		result[recipe.Name] = recipe
	}
	return result
}

func brandRecipe(artwork, appearance, surface, framing, colorBehavior string) BrandRecipe {
	return BrandRecipe{
		Name: strings.Join([]string{artwork, appearance, surface, framing}, "-"), Artwork: artwork,
		Appearance: appearance, Surface: surface, Framing: framing, ColorBehavior: colorBehavior,
	}
}
