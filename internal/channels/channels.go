// Package channels resolves promoted default and seasonal campaign documents.
package channels

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"regexp"
	"strings"

	"github.com/araihu/assets/internal/campaigns"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/themes"
	"gopkg.in/yaml.v3"
)

const (
	schemaVersion  = 1
	runtimeVersion = 1
)

var (
	lowerKebab = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)
	releaseTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	sha256Hex  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Default is an explicit baseline promotion for one immutable release.
type Default struct {
	SchemaVersion int    `yaml:"schema_version"`
	Release       string `yaml:"release"`
	Theme         string `yaml:"theme"`
}

// Input contains the fully decoded release contracts and the UTC date to resolve.
type Input struct {
	Date       campaigns.Date
	Default    Default
	Catalog    catalog.Catalog
	Themes     themes.Catalog
	Campaigns  campaigns.Manifest
	PublicRoot string
}

// Document is the deterministic current channel contract consumed by the runtime.
type Document struct {
	SchemaVersion  int               `json:"schemaVersion"`
	RuntimeVersion int               `json:"runtimeVersion"`
	Release        string            `json:"release"`
	Source         string            `json:"source"`
	Theme          ResolvedTheme     `json:"theme"`
	Campaign       *ResolvedCampaign `json:"campaign,omitempty"`
	Digest         string            `json:"digest"`
}

// ResolvedTheme contains the immutable stylesheet selected for the document.
type ResolvedTheme struct {
	ID     string `json:"id"`
	CSSURL string `json:"cssUrl"`
}

// ResolvedCampaign contains resolved campaign substitutions.
type ResolvedCampaign struct {
	ID     string         `json:"id"`
	Toggle ResolvedToggle `json:"toggle"`
	Brand  ResolvedBrand  `json:"brand"`
}

// ResolvedToggle contains the two runtime icon states.
type ResolvedToggle struct {
	EnabledIcon  ResolvedIcon `json:"enabledIcon"`
	DisabledIcon ResolvedIcon `json:"disabledIcon"`
}

// ResolvedBrand contains campaign replacement marks.
type ResolvedBrand struct {
	Logo ResolvedAsset `json:"logo"`
	Icon ResolvedAsset `json:"icon"`
}

// ResolvedAsset identifies an immutable direct asset URL.
type ResolvedAsset struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// ResolvedIcon identifies either an immutable direct icon or a sprite symbol.
type ResolvedIcon struct {
	ID           string `json:"id"`
	Mode         string `json:"mode"`
	URL          string `json:"url"`
	SpriteSymbol string `json:"spriteSymbol,omitempty"`
}

// LoadDefault reads one strict default-promotion YAML document.
func LoadDefault(fsys fs.FS, name string) (Default, error) {
	if !fs.ValidPath(name) {
		return Default{}, fmt.Errorf("default manifest path %q is invalid", name)
	}
	file, err := fsys.Open(name)
	if err != nil {
		return Default{}, fmt.Errorf("open default manifest: %w", err)
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var promotion Default
	if err := decoder.Decode(&promotion); err != nil {
		return Default{}, fmt.Errorf("decode default manifest: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Default{}, errors.New("decode default manifest: multiple YAML documents")
		}
		return Default{}, fmt.Errorf("decode default manifest: %w", err)
	}
	if err := promotion.Validate(); err != nil {
		return Default{}, err
	}
	return promotion, nil
}

// Validate checks the closed default-promotion source schema.
func (d Default) Validate() error {
	if d.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schema_version %d", d.SchemaVersion)
	}
	if !releaseTag.MatchString(d.Release) {
		return fmt.Errorf("invalid release %q", d.Release)
	}
	if !lowerKebab.MatchString(d.Theme) {
		return fmt.Errorf("invalid theme %q", d.Theme)
	}
	return nil
}

// Resolve validates all release contracts and produces the active campaign or
// promoted default for input.Date. Every output URL targets the selected
// immutable release below the configured public origin.
func Resolve(input Input) (Document, error) {
	if err := input.Default.Validate(); err != nil {
		return Document{}, err
	}
	if err := catalog.Validate(input.Catalog); err != nil {
		return Document{}, fmt.Errorf("catalog: %w", err)
	}
	if err := input.Themes.Validate(); err != nil {
		return Document{}, fmt.Errorf("themes: %w", err)
	}
	if err := input.Campaigns.Validate(); err != nil {
		return Document{}, fmt.Errorf("campaigns: %w", err)
	}
	if input.Default.Release != input.Catalog.Release {
		return Document{}, fmt.Errorf("default release %q does not match catalog release %q", input.Default.Release, input.Catalog.Release)
	}
	if input.Default.Release != input.Themes.Release {
		return Document{}, fmt.Errorf("default release %q does not match themes release %q", input.Default.Release, input.Themes.Release)
	}
	root, err := parsePublicRoot(input.PublicRoot)
	if err != nil {
		return Document{}, err
	}

	assets := make(map[string]catalog.Asset, len(input.Catalog.Assets))
	for _, asset := range input.Catalog.Assets {
		assets[asset.CanonicalName] = asset
	}
	themeCatalog := make(map[string]themes.CatalogTheme, len(input.Themes.Themes))
	for _, theme := range input.Themes.Themes {
		themeCatalog[theme.ID] = theme
	}
	defaultTheme, err := resolveTheme(root, input.Default.Release, themeCatalog, input.Default.Theme)
	if err != nil {
		return Document{}, fmt.Errorf("default theme: %w", err)
	}

	// Validate every record, including disabled campaigns, before selection.
	for _, campaign := range input.Campaigns.Campaigns {
		if _, err := resolveTheme(root, input.Default.Release, themeCatalog, campaign.Theme); err != nil {
			return Document{}, fmt.Errorf("campaign %q theme: %w", campaign.ID, err)
		}
		if _, err := resolveIcon(root, input.Default.Release, assets, campaign.Toggle.EnabledIcon); err != nil {
			return Document{}, fmt.Errorf("campaign %q enabled icon: %w", campaign.ID, err)
		}
		if _, err := resolveIcon(root, input.Default.Release, assets, campaign.Toggle.DisabledIcon); err != nil {
			return Document{}, fmt.Errorf("campaign %q disabled icon: %w", campaign.ID, err)
		}
		if _, err := resolveBrand(root, input.Default.Release, assets, campaign.Brand); err != nil {
			return Document{}, fmt.Errorf("campaign %q brand: %w", campaign.ID, err)
		}
	}

	active, err := input.Campaigns.Active(input.Date)
	if err != nil {
		return Document{}, fmt.Errorf("active campaign: %w", err)
	}
	document := Document{SchemaVersion: schemaVersion, RuntimeVersion: runtimeVersion, Release: input.Default.Release, Source: "default", Theme: defaultTheme}
	if active != nil {
		theme, err := resolveTheme(root, input.Default.Release, themeCatalog, active.Theme)
		if err != nil {
			return Document{}, err
		}
		toggle, err := resolveToggle(root, input.Default.Release, assets, active.Toggle)
		if err != nil {
			return Document{}, err
		}
		brand, err := resolveBrand(root, input.Default.Release, assets, active.Brand)
		if err != nil {
			return Document{}, err
		}
		document.Source = "campaign"
		document.Theme = theme
		document.Campaign = &ResolvedCampaign{ID: active.ID, Toggle: toggle, Brand: brand}
	}
	encoded, err := Encode(document)
	if err != nil {
		return Document{}, err
	}
	var completed Document
	if err := json.Unmarshal(encoded, &completed); err != nil {
		return Document{}, fmt.Errorf("decode resolved document: %w", err)
	}
	return completed, nil
}

// Encode writes canonical two-space JSON. It always recomputes Digest from the
// corresponding canonical document with Digest empty.
func Encode(document Document) ([]byte, error) {
	if err := validateDocument(document); err != nil {
		return nil, err
	}
	canonical := document
	canonical.Digest = ""
	payload, err := encodeCanonical(canonical)
	if err != nil {
		return nil, err
	}
	canonical.Digest = digest(payload)
	return encodeCanonical(canonical)
}

func resolveTheme(root *url.URL, release string, values map[string]themes.CatalogTheme, id string) (ResolvedTheme, error) {
	theme, found := values[id]
	if !found {
		return ResolvedTheme{}, fmt.Errorf("unknown theme %q", id)
	}
	return ResolvedTheme{ID: theme.ID, CSSURL: releaseURL(root, release, theme.CSSPath)}, nil
}

func resolveToggle(root *url.URL, release string, assets map[string]catalog.Asset, toggle campaigns.Toggle) (ResolvedToggle, error) {
	enabled, err := resolveIcon(root, release, assets, toggle.EnabledIcon)
	if err != nil {
		return ResolvedToggle{}, err
	}
	disabled, err := resolveIcon(root, release, assets, toggle.DisabledIcon)
	if err != nil {
		return ResolvedToggle{}, err
	}
	return ResolvedToggle{EnabledIcon: enabled, DisabledIcon: disabled}, nil
}

func resolveIcon(root *url.URL, release string, assets map[string]catalog.Asset, ref campaigns.IconRef) (ResolvedIcon, error) {
	asset, found := assets[ref.Asset]
	if !found {
		return ResolvedIcon{}, fmt.Errorf("unknown catalog asset %q", ref.Asset)
	}
	resolved := ResolvedIcon{ID: asset.CanonicalName, Mode: ref.Mode, URL: releaseURL(root, release, asset.Path)}
	if ref.Mode != "sprite" {
		return resolved, nil
	}
	if asset.SpriteSymbol == "" {
		return ResolvedIcon{}, fmt.Errorf("catalog asset %q has no sprite symbol", ref.Asset)
	}
	resolved.URL = releaseURL(root, release, "icons/"+asset.Namespace+"/sprite.svg")
	resolved.SpriteSymbol = asset.SpriteSymbol
	return resolved, nil
}

func resolveBrand(root *url.URL, release string, assets map[string]catalog.Asset, brand campaigns.Brand) (ResolvedBrand, error) {
	logo, err := resolveBrandAsset(root, release, assets, brand.Logo, "logo")
	if err != nil {
		return ResolvedBrand{}, err
	}
	icon, err := resolveBrandAsset(root, release, assets, brand.Icon, "icon")
	if err != nil {
		return ResolvedBrand{}, err
	}
	return ResolvedBrand{Logo: logo, Icon: icon}, nil
}

func resolveBrandAsset(root *url.URL, release string, assets map[string]catalog.Asset, name, artwork string) (ResolvedAsset, error) {
	asset, found := assets[name]
	if !found {
		return ResolvedAsset{}, fmt.Errorf("unknown catalog asset %q", name)
	}
	if asset.Namespace != "brand" || asset.Artwork != artwork {
		return ResolvedAsset{}, fmt.Errorf("catalog asset %q is not a brand %s", name, artwork)
	}
	return ResolvedAsset{ID: asset.CanonicalName, URL: releaseURL(root, release, asset.Path)}, nil
}

func parsePublicRoot(raw string) (*url.URL, error) {
	root, err := url.Parse(raw)
	if err != nil || root.Scheme != "https" || root.Host == "" || root.User != nil || root.RawQuery != "" || root.Fragment != "" || (root.Path != "" && root.Path != "/") {
		return nil, fmt.Errorf("public root %q must be an HTTPS origin", raw)
	}
	root.Path = ""
	return root, nil
}

func releaseURL(root *url.URL, release, path string) string {
	resolved := *root
	resolved.Path = "/assets/releases/" + release + "/" + path
	resolved.RawPath = ""
	return resolved.String()
}

func validateDocument(document Document) error {
	if document.SchemaVersion != schemaVersion {
		return fmt.Errorf("unsupported schemaVersion %d", document.SchemaVersion)
	}
	if document.RuntimeVersion != runtimeVersion {
		return fmt.Errorf("unsupported runtimeVersion %d", document.RuntimeVersion)
	}
	if !releaseTag.MatchString(document.Release) {
		return fmt.Errorf("invalid release %q", document.Release)
	}
	if document.Source != "default" && document.Source != "campaign" {
		return fmt.Errorf("invalid source %q", document.Source)
	}
	if document.Source == "campaign" && document.Campaign == nil {
		return errors.New("campaign source requires campaign")
	}
	if document.Source == "default" && document.Campaign != nil {
		return errors.New("default source cannot contain campaign")
	}
	if !lowerKebab.MatchString(document.Theme.ID) || !validReleaseURL(document.Theme.CSSURL, document.Release) {
		return errors.New("invalid resolved theme")
	}
	if document.Campaign != nil {
		if err := validateCampaign(document.Release, *document.Campaign); err != nil {
			return err
		}
	}
	if document.Digest != "" && !sha256Hex.MatchString(document.Digest) {
		return fmt.Errorf("invalid digest %q", document.Digest)
	}
	return nil
}

func validateCampaign(release string, campaign ResolvedCampaign) error {
	if !lowerKebab.MatchString(campaign.ID) {
		return fmt.Errorf("invalid campaign id %q", campaign.ID)
	}
	for _, icon := range []ResolvedIcon{campaign.Toggle.EnabledIcon, campaign.Toggle.DisabledIcon} {
		if !lowerKebab.MatchString(icon.ID) || (icon.Mode != "asset" && icon.Mode != "sprite") || !validReleaseURL(icon.URL, release) {
			return errors.New("invalid resolved icon")
		}
		if icon.Mode == "sprite" && !lowerKebab.MatchString(icon.SpriteSymbol) {
			return errors.New("sprite icon requires sprite symbol")
		}
		if icon.Mode == "asset" && icon.SpriteSymbol != "" {
			return errors.New("asset icon cannot have sprite symbol")
		}
	}
	for _, asset := range []ResolvedAsset{campaign.Brand.Logo, campaign.Brand.Icon} {
		if !lowerKebab.MatchString(asset.ID) || !validReleaseURL(asset.URL, release) {
			return errors.New("invalid resolved brand asset")
		}
	}
	return nil
}

func validReleaseURL(raw, release string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return false
	}
	return strings.HasPrefix(parsed.EscapedPath(), "/assets/releases/"+release+"/")
}

func encodeCanonical(document Document) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("encode channel document: %w", err)
	}
	return output.Bytes(), nil
}

func digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
