// Package manifest loads and validates the versioned asset input manifests.
package manifest

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	schemaVersion    = 1
	heroiconsCommit  = "0435d4ca364a608cc75e2f8683d374e55abbae26"
	heroiconsBaseURL = "https://raw.githubusercontent.com/tailwindlabs/heroicons/" + heroiconsCommit + "/src/"
)

var (
	lowerKebab       = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	lowerSHA256      = regexp.MustCompile(`^[0-9a-f]{64}$`)
	expectedProducts = map[string]string{
		"araihu": "Arai Hu", "goshtoso": "Goshtoso", "manja": "Manja", "paje": "Paje", "x9": "X9",
	}
	expectedUIPaths = []string{
		"16/solid/arrow-down-tray.svg", "16/solid/arrow-down.svg", "16/solid/arrow-path.svg", "16/solid/arrow-top-right-on-square.svg",
		"16/solid/arrow-up.svg", "16/solid/arrow-uturn-left.svg", "16/solid/arrows-up-down.svg", "16/solid/bars-3.svg", "16/solid/bell.svg",
		"16/solid/book-open.svg", "16/solid/chart-bar.svg", "16/solid/check-circle.svg", "16/solid/check.svg", "16/solid/chevron-down.svg",
		"16/solid/chevron-left.svg", "16/solid/chevron-right.svg", "16/solid/clipboard-document-list.svg", "16/solid/clipboard.svg",
		"16/solid/clock.svg", "16/solid/cloud-arrow-up.svg", "16/solid/code-bracket.svg", "16/solid/cog-6-tooth.svg", "16/solid/cube.svg",
		"16/solid/document-duplicate.svg", "16/solid/document-text.svg", "16/solid/ellipsis-horizontal.svg", "16/solid/ellipsis-vertical.svg",
		"16/solid/exclamation-circle.svg", "16/solid/eye-slash.svg", "16/solid/eye.svg", "16/solid/face-smile.svg", "16/solid/folder.svg",
		"16/solid/funnel.svg", "16/solid/heart.svg", "16/solid/home.svg", "16/solid/identification.svg", "16/solid/inbox.svg",
		"16/solid/information-circle.svg", "16/solid/language.svg", "16/solid/link.svg", "16/solid/lock-closed.svg", "16/solid/magnifying-glass.svg",
		"16/solid/microphone.svg", "16/solid/moon.svg", "16/solid/paint-brush.svg", "16/solid/paper-clip.svg", "16/solid/pause.svg",
		"16/solid/pencil-square.svg", "16/solid/play.svg", "16/solid/plus.svg", "16/solid/printer.svg", "16/solid/queue-list.svg",
		"16/solid/rectangle-group.svg", "16/solid/scissors.svg", "16/solid/shield-check.svg", "16/solid/sparkles.svg", "16/solid/squares-2x2.svg",
		"16/solid/star.svg", "16/solid/sun.svg", "16/solid/table-cells.svg", "16/solid/trash.svg", "16/solid/user-circle.svg",
		"16/solid/user.svg", "16/solid/users.svg", "16/solid/window.svg", "16/solid/x-circle.svg", "16/solid/x-mark.svg",
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
	Appearance    string `yaml:"appearance"`
	Surface       string `yaml:"surface"`
	Framing       string `yaml:"framing"`
	ColorBehavior string `yaml:"color_behavior"`
}

// UI describes immutable external UI-icon source files.
type UI struct {
	SchemaVersion int        `yaml:"schema_version"`
	Sources       []UISource `yaml:"sources"`
}

// UISource identifies one immutable icon release.
type UISource struct {
	Name       string   `yaml:"name"`
	Alias      string   `yaml:"alias"`
	Version    string   `yaml:"version"`
	Commit     string   `yaml:"commit"`
	BaseURL    string   `yaml:"base_url"`
	License    string   `yaml:"license"`
	LicenseURL string   `yaml:"license_url"`
	Icons      []UIIcon `yaml:"icons"`
}

// UIIcon is one pinned source path and its expected bytes hash.
type UIIcon struct {
	Path   string `yaml:"path"`
	SHA256 string `yaml:"sha256"`
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

// LoadUI decodes a single strict YAML UI manifest and validates it.
func LoadUI(fsys fs.FS, name string) (UI, error) {
	var ui UI
	if err := decode(fsys, name, &ui); err != nil {
		return UI{}, err
	}
	if err := ui.Validate(); err != nil {
		return UI{}, err
	}
	return ui, nil
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
	if len(product.Sources) == 0 {
		return fmt.Errorf("product %q has no sources", product.ID)
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
	if len(palettes) == 0 {
		return errors.New("brand palettes are empty")
	}
	for id, palette := range palettes {
		if !lowerKebab.MatchString(id) || !lowerKebab.MatchString(palette.Name) || len(palette.Colors) == 0 {
			return fmt.Errorf("invalid palette %q", id)
		}
		for color, value := range palette.Colors {
			if !lowerKebab.MatchString(color) || !strings.HasPrefix(value, "#") {
				return fmt.Errorf("invalid palette color %q in %q", color, id)
			}
		}
	}
	return nil
}

func validateRecipes(recipes []BrandRecipe) error {
	if len(recipes) == 0 {
		return errors.New("brand recipes are empty")
	}
	seen := make(map[string]struct{}, len(recipes))
	for _, recipe := range recipes {
		if !lowerKebab.MatchString(recipe.Name) || strings.TrimSpace(recipe.Appearance) == "" || strings.TrimSpace(recipe.Surface) == "" || strings.TrimSpace(recipe.Framing) == "" || strings.TrimSpace(recipe.ColorBehavior) == "" {
			return fmt.Errorf("invalid recipe %q", recipe.Name)
		}
		if _, ok := seen[recipe.Name]; ok {
			return fmt.Errorf("duplicate recipe %q", recipe.Name)
		}
		seen[recipe.Name] = struct{}{}
	}
	return nil
}

// Validate checks that a UI manifest is an exact, immutable Heroicons v2.2.0 lock.
func (ui UI) Validate() error {
	if ui.SchemaVersion != schemaVersion {
		return fmt.Errorf("ui schema_version = %d, want %d", ui.SchemaVersion, schemaVersion)
	}
	if len(ui.Sources) != 1 {
		return fmt.Errorf("ui sources = %d, want 1", len(ui.Sources))
	}
	source := ui.Sources[0]
	if source.Name != "heroicons" || source.Alias != "hi" || source.Version != "v2.2.0" || source.Commit != heroiconsCommit || source.BaseURL != heroiconsBaseURL || source.License != "MIT" {
		return errors.New("ui source must be immutable Heroicons v2.2.0")
	}
	if !lowerKebab.MatchString(source.Name) || !lowerKebab.MatchString(source.Alias) || !validHTTPS(source.LicenseURL) {
		return errors.New("ui source metadata is invalid")
	}
	if len(source.Icons) != len(expectedUIPaths) {
		return fmt.Errorf("ui icon count = %d, want %d", len(source.Icons), len(expectedUIPaths))
	}
	want := make(map[string]struct{}, len(expectedUIPaths))
	for _, path := range expectedUIPaths {
		want[path] = struct{}{}
	}
	for _, icon := range source.Icons {
		if !fs.ValidPath(icon.Path) || !strings.HasPrefix(icon.Path, "16/solid/") || !strings.HasSuffix(icon.Path, ".svg") {
			return fmt.Errorf("invalid ui icon path %q", icon.Path)
		}
		if !lowerSHA256.MatchString(icon.SHA256) {
			return fmt.Errorf("invalid sha256 for %q", icon.Path)
		}
		if _, ok := want[icon.Path]; !ok {
			return fmt.Errorf("unexpected or duplicate ui icon %q", icon.Path)
		}
		delete(want, icon.Path)
	}
	if len(want) != 0 {
		return errors.New("ui manifest is missing expected icon paths")
	}
	return nil
}

func validHTTPS(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "https" && u.Host != "" && u.User == nil && u.Fragment == ""
}
