// Package themes defines strict, versioned theme metadata for asset releases.
package themes

import (
	"bytes"
	"encoding/json"
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
	tokenContract = "goshtoso-theme-v1"
)

var (
	lowerKebab = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	sha256Hex  = regexp.MustCompile(`^[0-9a-f]{64}$`)
	releaseTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
)

// Manifest is the source themes.yaml contract.
type Manifest struct {
	SchemaVersion int     `yaml:"schema_version"`
	TokenContract string  `yaml:"token_contract"`
	Themes        []Theme `yaml:"themes"`
}

// Theme identifies one source stylesheet. TokenContract and SHA256 are filled
// from captured source bytes before the theme is published in a catalog.
type Theme struct {
	ID            string `yaml:"id" json:"id"`
	CSSPath       string `yaml:"css_path" json:"cssPath"`
	TokenContract string `yaml:"-" json:"tokenContract"`
	SHA256        string `yaml:"-" json:"sha256"`
}

// Catalog is the versioned themes.json release contract.
type Catalog struct {
	SchemaVersion int            `json:"schemaVersion"`
	Release       string         `json:"release"`
	TokenContract string         `json:"tokenContract"`
	Themes        []CatalogTheme `json:"themes"`
}

// CatalogTheme is one hash-addressed published stylesheet.
type CatalogTheme struct {
	ID            string `json:"id"`
	CSSPath       string `json:"cssPath"`
	TokenContract string `json:"tokenContract"`
	SHA256        string `json:"sha256"`
}

// Load reads one strict YAML manifest and validates its source metadata.
func Load(fsys fs.FS, name string) (Manifest, error) {
	file, err := fsys.Open(name)
	if err != nil {
		return Manifest{}, fmt.Errorf("open themes manifest: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var manifest Manifest
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode themes manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Manifest{}, errors.New("decode themes manifest: multiple YAML documents")
		}
		return Manifest{}, fmt.Errorf("decode themes manifest: %w", err)
	}
	if err := manifest.Validate(); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// Validate checks the closed source schema and safe stylesheet references.
func (m Manifest) Validate() error {
	if m.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schema_version %d", m.SchemaVersion)
	}
	if m.TokenContract != tokenContract {
		return fmt.Errorf("unsupported token_contract %q", m.TokenContract)
	}
	if len(m.Themes) == 0 {
		return errors.New("themes are empty")
	}
	seen := make(map[string]struct{}, len(m.Themes))
	for i, theme := range m.Themes {
		if !lowerKebab.MatchString(theme.ID) {
			return fmt.Errorf("theme[%d]: invalid id %q", i, theme.ID)
		}
		if _, exists := seen[theme.ID]; exists {
			return fmt.Errorf("theme[%d]: duplicate id %q", i, theme.ID)
		}
		seen[theme.ID] = struct{}{}
		if !validCSSPath(theme.CSSPath) {
			return fmt.Errorf("theme[%d] %q: invalid css_path %q", i, theme.ID, theme.CSSPath)
		}
	}
	return nil
}

// Catalog returns the deterministic release catalog. Each Theme must already
// have the hash of its captured CSS bytes.
func (m Manifest) Catalog(release string) (Catalog, error) {
	if err := m.Validate(); err != nil {
		return Catalog{}, err
	}
	catalog := Catalog{SchemaVersion: schemaVersion, Release: release, TokenContract: m.TokenContract, Themes: make([]CatalogTheme, 0, len(m.Themes))}
	for _, theme := range m.Themes {
		contract := theme.TokenContract
		if contract == "" {
			contract = m.TokenContract
		}
		catalog.Themes = append(catalog.Themes, CatalogTheme{ID: theme.ID, CSSPath: theme.CSSPath, TokenContract: contract, SHA256: theme.SHA256})
	}
	slices.SortFunc(catalog.Themes, func(a, b CatalogTheme) int { return strings.Compare(a.ID, b.ID) })
	if err := catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

// Validate checks the complete published themes.json contract.
func (c Catalog) Validate() error {
	if c.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", c.SchemaVersion)
	}
	if !releaseTag.MatchString(c.Release) {
		return fmt.Errorf("invalid release %q", c.Release)
	}
	if c.TokenContract != tokenContract {
		return fmt.Errorf("unsupported tokenContract %q", c.TokenContract)
	}
	if len(c.Themes) == 0 {
		return errors.New("themes are empty")
	}
	seen := make(map[string]struct{}, len(c.Themes))
	for i, theme := range c.Themes {
		if !lowerKebab.MatchString(theme.ID) {
			return fmt.Errorf("theme[%d]: invalid id %q", i, theme.ID)
		}
		if _, exists := seen[theme.ID]; exists {
			return fmt.Errorf("theme[%d]: duplicate id %q", i, theme.ID)
		}
		seen[theme.ID] = struct{}{}
		if !validCSSPath(theme.CSSPath) {
			return fmt.Errorf("theme[%d] %q: invalid cssPath %q", i, theme.ID, theme.CSSPath)
		}
		if theme.TokenContract != c.TokenContract {
			return fmt.Errorf("theme[%d] %q: tokenContract %q does not match catalog", i, theme.ID, theme.TokenContract)
		}
		if !sha256Hex.MatchString(theme.SHA256) {
			return fmt.Errorf("theme[%d] %q: invalid sha256 %q", i, theme.ID, theme.SHA256)
		}
	}
	return nil
}

// Encode validates and serializes a canonical, deterministic themes catalog.
func Encode(catalog Catalog) ([]byte, error) {
	if err := catalog.Validate(); err != nil {
		return nil, err
	}
	canonical := catalog
	canonical.Themes = slices.Clone(catalog.Themes)
	slices.SortFunc(canonical.Themes, func(a, b CatalogTheme) int { return strings.Compare(a.ID, b.ID) })
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(canonical); err != nil {
		return nil, fmt.Errorf("encode themes catalog: %w", err)
	}
	return output.Bytes(), nil
}

func validCSSPath(path string) bool {
	return fs.ValidPath(path) && !strings.Contains(path, `\`) && strings.HasSuffix(path, ".css")
}
