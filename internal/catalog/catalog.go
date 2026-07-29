// Package catalog defines the language-neutral catalog.json release contract.
package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// SchemaVersion is the only catalog schema supported by this package.
const SchemaVersion = 1

var (
	lowerKebab = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	sha256Hex  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	releaseTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$`)

	allowedFormats = map[string]struct{}{
		"png": {},
		"svg": {},
	}
	allowedColorBehaviors = map[string]struct{}{
		"monochrome": {},
		"protected":  {},
		"tintable":   {},
	}
	allowedNamespaces = map[string]struct{}{
		"brand": {},
		"ui":    {},
	}
	artifactRoots = map[string]struct{}{
		"brand":    {},
		"icons":    {},
		"platform": {},
	}
)

// Catalog is the versioned catalog.json release contract.
type Catalog struct {
	SchemaVersion    int     `json:"schemaVersion"`
	Release          string  `json:"release"`
	IdentityRevision int     `json:"identityRevision"`
	Assets           []Asset `json:"assets"`
}

// Dimensions describes a published visual artifact's dimensions.
type Dimensions struct {
	Width   int    `json:"width,omitempty"`
	Height  int    `json:"height,omitempty"`
	ViewBox string `json:"viewBox,omitempty"`
}

// Asset is one generated, distributable visual artifact.
type Asset struct {
	CanonicalName string     `json:"canonicalName"`
	Namespace     string     `json:"namespace"`
	Path          string     `json:"path"`
	Product       string     `json:"product"`
	Artwork       string     `json:"artwork"`
	Appearance    string     `json:"appearance"`
	Surface       string     `json:"surface"`
	Framing       string     `json:"framing"`
	Format        string     `json:"format"`
	Dimensions    Dimensions `json:"dimensions"`
	SpriteSymbol  string     `json:"spriteSymbol"`
	ColorBehavior string     `json:"colorBehavior"`
	License       string     `json:"license"`
	Source        string     `json:"source"`
	SHA256        string     `json:"sha256"`
}

// Validate checks that c is a closed schema-v1 catalog of distributable assets.
func Validate(c Catalog) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", c.SchemaVersion)
	}
	if !releaseTag.MatchString(c.Release) {
		return fmt.Errorf("invalid release %q", c.Release)
	}
	if c.IdentityRevision <= 0 {
		return fmt.Errorf("identityRevision = %d, want positive", c.IdentityRevision)
	}
	if len(c.Assets) == 0 {
		return errors.New("assets are empty")
	}

	names := make(map[string]struct{}, len(c.Assets))
	symbols := make(map[string]struct{}, len(c.Assets))
	for i, asset := range c.Assets {
		if err := validateAsset(asset); err != nil {
			return fmt.Errorf("asset[%d] %q: %w", i, asset.CanonicalName, err)
		}
		if _, ok := names[asset.CanonicalName]; ok {
			return fmt.Errorf("duplicate canonicalName %q", asset.CanonicalName)
		}
		names[asset.CanonicalName] = struct{}{}
		if asset.SpriteSymbol == "" {
			continue
		}
		if _, ok := symbols[asset.SpriteSymbol]; ok {
			return fmt.Errorf("duplicate spriteSymbol %q", asset.SpriteSymbol)
		}
		symbols[asset.SpriteSymbol] = struct{}{}
	}
	return nil
}

func validateAsset(asset Asset) error {
	if !lowerKebab.MatchString(asset.CanonicalName) {
		return fmt.Errorf("invalid canonicalName %q", asset.CanonicalName)
	}
	if _, ok := allowedNamespaces[asset.Namespace]; !ok {
		return fmt.Errorf("invalid namespace %q", asset.Namespace)
	}
	if err := validateArtifactPath(asset.Path, asset.Format); err != nil {
		return err
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{"product", asset.Product},
		{"artwork", asset.Artwork},
		{"appearance", asset.Appearance},
		{"surface", asset.Surface},
		{"framing", asset.Framing},
	} {
		if !lowerKebab.MatchString(field.value) {
			return fmt.Errorf("invalid %s %q", field.name, field.value)
		}
	}
	if _, ok := allowedFormats[asset.Format]; !ok {
		return fmt.Errorf("invalid format %q", asset.Format)
	}
	if _, ok := allowedColorBehaviors[asset.ColorBehavior]; !ok {
		return fmt.Errorf("invalid colorBehavior %q", asset.ColorBehavior)
	}
	if !validText(asset.License) {
		return errors.New("license is empty or invalid")
	}
	if !validText(asset.Source) {
		return errors.New("source is empty or invalid")
	}
	if !sha256Hex.MatchString(asset.SHA256) {
		return fmt.Errorf("invalid sha256 %q", asset.SHA256)
	}
	if err := validateDimensions(asset.Format, asset.Dimensions); err != nil {
		return err
	}
	if asset.Format == "svg" {
		if !lowerKebab.MatchString(asset.SpriteSymbol) {
			return fmt.Errorf("invalid spriteSymbol %q", asset.SpriteSymbol)
		}
	} else if asset.SpriteSymbol != "" {
		return fmt.Errorf("non-SVG asset has spriteSymbol %q", asset.SpriteSymbol)
	}
	return nil
}

func validateArtifactPath(path, format string) error {
	if !fs.ValidPath(path) || strings.HasPrefix(path, "dist/") {
		return fmt.Errorf("invalid dist-relative path %q", path)
	}
	parts := strings.Split(path, "/")
	if _, ok := artifactRoots[parts[0]]; !ok {
		return fmt.Errorf("path %q is not a distributed artifact", path)
	}
	if !strings.HasSuffix(path, "."+format) {
		return fmt.Errorf("path %q does not match format %q", path, format)
	}
	return nil
}

func validateDimensions(format string, d Dimensions) error {
	if d.Width < 0 || d.Height < 0 || (d.Width == 0) != (d.Height == 0) {
		return fmt.Errorf("invalid dimensions %dx%d", d.Width, d.Height)
	}
	switch format {
	case "png":
		if d.Width == 0 || d.ViewBox != "" {
			return errors.New("png dimensions require width and height without viewBox")
		}
	case "svg":
		if !validViewBox(d.ViewBox) {
			return fmt.Errorf("invalid SVG viewBox %q", d.ViewBox)
		}
	}
	return nil
}

func validViewBox(viewBox string) bool {
	fields := strings.Fields(viewBox)
	if len(fields) != 4 {
		return false
	}
	values := make([]float64, 4)
	for i, field := range fields {
		value, err := strconv.ParseFloat(field, 64)
		if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		values[i] = value
	}
	return values[2] > 0 && values[3] > 0
}

func validText(value string) bool {
	return strings.TrimSpace(value) == value && value != "" && !strings.ContainsAny(value, "\r\n\t")
}

// Encode validates c and writes canonical, deterministic JSON without mutating c.
func Encode(w io.Writer, c Catalog) error {
	if err := Validate(c); err != nil {
		return err
	}
	canonical := c
	canonical.Assets = slices.Clone(c.Assets)
	slices.SortFunc(canonical.Assets, func(a, b Asset) int {
		if byName := strings.Compare(a.CanonicalName, b.CanonicalName); byName != 0 {
			return byName
		}
		return strings.Compare(a.Path, b.Path)
	})
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(canonical); err != nil {
		return fmt.Errorf("encode catalog: %w", err)
	}
	return nil
}

// Decode reads one strict catalog.json document and validates it.
func Decode(r io.Reader) (Catalog, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var c Catalog
	if err := decoder.Decode(&c); err != nil {
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Catalog{}, errors.New("decode catalog: multiple JSON values")
		}
		return Catalog{}, fmt.Errorf("decode catalog: %w", err)
	}
	if err := Validate(c); err != nil {
		return Catalog{}, err
	}
	return c, nil
}

// ValidatePatchCompatibility permits additive patch catalogs only. Existing
// canonical names must retain their rendering and provenance semantics.
func ValidatePatchCompatibility(previous, next Catalog) error {
	if err := Validate(previous); err != nil {
		return fmt.Errorf("previous catalog: %w", err)
	}
	if err := Validate(next); err != nil {
		return fmt.Errorf("next catalog: %w", err)
	}
	if previous.SchemaVersion != next.SchemaVersion {
		return fmt.Errorf("schemaVersion changed from %d to %d", previous.SchemaVersion, next.SchemaVersion)
	}
	if previous.IdentityRevision != next.IdentityRevision {
		return fmt.Errorf("identityRevision changed from %d to %d", previous.IdentityRevision, next.IdentityRevision)
	}
	nextByName := make(map[string]Asset, len(next.Assets))
	for _, asset := range next.Assets {
		nextByName[asset.CanonicalName] = asset
	}
	for _, old := range previous.Assets {
		candidate, ok := nextByName[old.CanonicalName]
		if !ok {
			return fmt.Errorf("patch removes canonicalName %q", old.CanonicalName)
		}
		if !sameSemantics(old, candidate) {
			return fmt.Errorf("patch changes semantics for canonicalName %q", old.CanonicalName)
		}
	}
	return nil
}

func sameSemantics(a, b Asset) bool {
	return a.Namespace == b.Namespace &&
		a.Product == b.Product &&
		a.Artwork == b.Artwork &&
		a.Appearance == b.Appearance &&
		a.Surface == b.Surface &&
		a.Framing == b.Framing &&
		a.Format == b.Format &&
		a.Dimensions == b.Dimensions &&
		a.SpriteSymbol == b.SpriteSymbol &&
		a.ColorBehavior == b.ColorBehavior &&
		a.License == b.License &&
		a.Source == b.Source
}
