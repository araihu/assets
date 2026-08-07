package proof

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/araihu/assets/internal/catalog"
)

var lowerKebab = regexp.MustCompile(`^[a-z][a-z0-9]*(?:-[a-z0-9]+)*$`)

var allowedMasks = map[string]struct{}{
	"none":     {},
	"circle":   {},
	"squircle": {},
}

// Load reads one strict scenario declaration and constructs a catalog-backed
// view model. Asset files are checked by Build against the caller's verified
// distribution filesystem.
func Load(c catalog.Catalog, r io.Reader) (Model, error) {
	if err := catalog.Validate(c); err != nil {
		return Model{}, fmt.Errorf("validate catalog: %w", err)
	}
	if r == nil {
		return Model{}, errors.New("read scenarios: nil reader")
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return Model{}, fmt.Errorf("read scenarios: %w", err)
	}
	if err := validateScenarioDocumentShape(data); err != nil {
		return Model{}, err
	}

	var document struct {
		Scenarios []Scenario `json:"scenarios"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&document); err != nil {
		return Model{}, fmt.Errorf("decode scenarios: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Model{}, errors.New("decode scenarios: multiple JSON values")
		}
		return Model{}, fmt.Errorf("decode scenarios: %w", err)
	}
	return newModel(c, document.Scenarios)
}

// Build verifies every rendered reference and buffers template execution before
// one writer call. It does not make publication to an arbitrary io.Writer
// filesystem-atomic; a later command layer owns atomic file replacement.
func Build(m Model, fsys fs.FS, output io.Writer) error {
	return build(m, fsys, output, parseDocumentTemplate)
}

// BuildTemplate renders a proof document with the caller-selected template.
// Build keeps repository discovery for direct command use; build assembly uses
// this form so its staged repository never depends on process working directory.
func BuildTemplate(m Model, fsys fs.FS, templateFile string, output io.Writer) error {
	return build(m, fsys, output, func() (*template.Template, error) {
		return parseProofTemplate(templateFile)
	})
}

func build(m Model, fsys fs.FS, output io.Writer, parseTemplate func() (*template.Template, error)) error {
	if output == nil {
		return errors.New("build proof: nil writer")
	}
	canonical, err := validatedCanonicalModel(m)
	if err != nil {
		return fmt.Errorf("build proof model: %w", err)
	}
	m = canonical
	if fsys == nil {
		return errors.New("build proof: nil distribution filesystem")
	}

	assets := assetsByName(m.Catalog)
	checked := make(map[string]struct{}, len(m.Scenarios)+len(m.Platform)*5)
	for _, scenario := range m.Scenarios {
		if _, already := checked[scenario.Asset]; already {
			continue
		}
		checked[scenario.Asset] = struct{}{}
		asset := assets[scenario.Asset]
		info, err := fs.Stat(fsys, asset.Path)
		if err != nil {
			return fmt.Errorf("missing referenced distribution file %q for scenario %q: %w", asset.Path, scenario.ID, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("referenced distribution file %q for scenario %q is not regular", asset.Path, scenario.ID)
		}
	}
	for _, product := range m.Platform {
		if err := validateProofFile(fsys, checked, product.Master.Path, "catalog launcher master for product "+product.Product); err != nil {
			return err
		}
		for _, reference := range product.Packages {
			if err := validateProofFile(fsys, checked, reference.Path, reference.Kind+" for product "+reference.Product); err != nil {
				return err
			}
			if err := validateProofFile(fsys, checked, reference.ProvenancePath, "provenance for "+reference.Kind+" for product "+reference.Product); err != nil {
				return err
			}
		}
	}
	if hasProduct(m.Catalog, "heroicons") {
		for _, reference := range []string{"icons/ui/sprite.svg", "licenses/heroicons-MIT.txt", "icons/ui/heroicons/provenance.json"} {
			if err := validateProofFile(fsys, checked, reference, "UI license or provenance evidence"); err != nil {
				return err
			}
		}
	}
	if hasProduct(m.Catalog, "developer-icons") {
		for _, reference := range []string{"icons/brand/developer-icons/sprite.svg", "licenses/developer-icons-MIT.txt", "icons/brand/developer-icons/provenance.json"} {
			if err := validateProofFile(fsys, checked, reference, "Developer Icons license or provenance evidence"); err != nil {
				return err
			}
		}
	}
	for _, reference := range []string{"NOTICE", "licenses/Apache-2.0.txt"} {
		if err := validateProofFile(fsys, checked, reference, "release license or provenance evidence"); err != nil {
			return err
		}
	}

	document, err := newDocumentModel(m)
	if err != nil {
		return err
	}
	page, err := parseTemplate()
	if err != nil {
		return err
	}
	var rendered bytes.Buffer
	if err := page.Execute(&rendered, document); err != nil {
		return fmt.Errorf("render proof document: %w", err)
	}
	n, err := output.Write(rendered.Bytes())
	if err != nil {
		return fmt.Errorf("write proof document: %w", err)
	}
	if n != rendered.Len() {
		return fmt.Errorf("write proof document: %w", io.ErrShortWrite)
	}
	return nil
}

func validateProofFile(fsys fs.FS, checked map[string]struct{}, proofPath, purpose string) error {
	if _, already := checked[proofPath]; already {
		return nil
	}
	checked[proofPath] = struct{}{}
	info, err := fs.Stat(fsys, proofPath)
	if err != nil {
		return fmt.Errorf("missing referenced distribution file %q for %s: %w", proofPath, purpose, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("referenced distribution file %q for %s is not regular", proofPath, purpose)
	}
	return nil
}

func hasProduct(c catalog.Catalog, product string) bool {
	for _, asset := range c.Assets {
		if asset.Product == product {
			return true
		}
	}
	return false
}

type documentModel struct {
	Release          string
	NoticeURL        string
	ApacheLicenseURL string
	Products         []documentProduct
	ExactSizes       []int
	BrandScenarios   []documentSpecimen
	UIScenarios      []documentSpecimen
	Metrics          []documentMetric
	Licenses         []documentLicense
}

type documentProduct struct {
	ID       string
	Name     string
	Master   documentMaster
	Packages []documentPackage
}

type documentMaster struct {
	CanonicalName string
	Artwork       string
	Variant       string
	Appearance    string
	Surface       string
	URL           string
}

type documentPackage struct {
	Kind          string
	URL           string
	ProvenanceURL string
}

type documentSpecimen struct {
	ID             string
	ProductID      string
	Product        string
	Artwork        string
	Variant        string
	Appearance     string
	Surface        string
	Mask           string
	Sizes          []int
	URL            string
	SpriteURL      string
	IsTransparent  bool
	StressSurfaces []string
}

type documentMetric struct {
	Product    string
	ProductID  string
	Asset      string
	ViewBox    string
	Format     string
	Appearance string
	Surface    string
}

type documentLicense struct {
	Product       string
	Asset         string
	License       string
	LicenseURL    string
	Source        string
	ProvenanceURL string
	URL           string
}

func newDocumentModel(m Model) (documentModel, error) {
	assets := assetsByName(m.Catalog)
	document := documentModel{
		Release:          m.Catalog.Release,
		NoticeURL:        relativeProofURL("NOTICE"),
		ApacheLicenseURL: relativeProofURL("licenses/Apache-2.0.txt"),
		Products:         documentProducts(m.Platform),
		ExactSizes:       slices.Clone(m.ExactSizes),
		BrandScenarios:   make([]documentSpecimen, 0),
		UIScenarios:      make([]documentSpecimen, 0),
		Metrics:          make([]documentMetric, 0, len(m.Catalog.Assets)),
		Licenses:         make([]documentLicense, 0, len(m.Catalog.Assets)),
	}
	for _, scenario := range m.Scenarios {
		asset := assets[scenario.Asset]
		specimen := documentSpecimen{
			ID:             scenario.ID,
			ProductID:      asset.Product,
			Product:        productName(asset.Product),
			Artwork:        asset.Artwork,
			Variant:        strings.Join([]string{asset.Surface, asset.Appearance, asset.Framing}, " "),
			Appearance:     asset.Appearance,
			Surface:        asset.Surface,
			Mask:           scenario.Mask,
			Sizes:          slices.Clone(scenario.Sizes),
			URL:            relativeProofURL(asset.Path),
			SpriteURL:      relativeProofURL("icons/ui/sprite.svg") + "#" + asset.SpriteSymbol,
			IsTransparent:  asset.Surface == "transparent",
			StressSurfaces: []string{"checker", "paper", "midnight"},
		}
		if asset.Namespace == "brand" {
			document.BrandScenarios = append(document.BrandScenarios, specimen)
		} else {
			document.UIScenarios = append(document.UIScenarios, specimen)
		}
	}
	for _, asset := range m.Catalog.Assets {
		document.Metrics = append(document.Metrics, documentMetric{
			Product: productName(asset.Product), ProductID: asset.Product, Asset: asset.CanonicalName,
			ViewBox: asset.Dimensions.ViewBox, Format: asset.Format, Appearance: asset.Appearance, Surface: asset.Surface,
		})
		licenseURL, provenanceURL := relativeProofURL("NOTICE"), relativeProofURL("NOTICE")
		if asset.Product == "heroicons" {
			licenseURL = relativeProofURL("licenses/heroicons-MIT.txt")
			provenanceURL = relativeProofURL("icons/ui/heroicons/provenance.json")
		} else if asset.Product == "developer-icons" {
			licenseURL = relativeProofURL("licenses/developer-icons-MIT.txt")
			provenanceURL = relativeProofURL("icons/brand/developer-icons/provenance.json")
		}
		document.Licenses = append(document.Licenses, documentLicense{
			Product: productName(asset.Product), Asset: asset.CanonicalName,
			License: asset.License, LicenseURL: licenseURL, Source: asset.Source,
			ProvenanceURL: provenanceURL, URL: relativeProofURL(asset.Path),
		})
	}
	return document, nil
}

func documentProducts(products []PlatformProof) []documentProduct {
	result := make([]documentProduct, 0, len(products))
	for _, product := range products {
		packages := make([]documentPackage, 0, len(product.Packages))
		for _, reference := range product.Packages {
			packages = append(packages, documentPackage{
				Kind: reference.Kind, URL: relativeProofURL(reference.Path), ProvenanceURL: relativeProofURL(reference.ProvenancePath),
			})
		}
		result = append(result, documentProduct{
			ID: product.Product, Name: productName(product.Product),
			Master: documentMaster{
				CanonicalName: product.Master.CanonicalName,
				Artwork:       product.Master.Artwork,
				Variant:       strings.Join([]string{product.Master.Appearance, product.Master.Surface, product.Master.Framing}, " "),
				Appearance:    product.Master.Appearance,
				Surface:       product.Master.Surface,
				URL:           relativeProofURL(product.Master.Path),
			},
			Packages: packages,
		})
	}
	return result
}

func productName(id string) string {
	switch id {
	case "araihu":
		return "Arai Hû"
	case "paje":
		return "Pajé"
	case "x9":
		return "X9"
	case "goshtoso":
		return "Goshtoso"
	case "manja":
		return "Manja"
	case "heroicons":
		return "Heroicons"
	case "developer-icons":
		return "Developer Icons"
	default:
		return id
	}
}

func relativeProofURL(distributionPath string) string {
	return AssetURL("assets/" + path.Clean(distributionPath))
}

func parseDocumentTemplate() (*template.Template, error) {
	file, err := proofTemplatePath()
	if err != nil {
		return nil, err
	}
	return parseProofTemplate(file)
}

func parseProofTemplate(file string) (*template.Template, error) {
	page, err := template.New(filepath.Base(file)).Funcs(template.FuncMap{"AssetURL": AssetURL}).ParseFiles(file)
	if err != nil {
		return nil, fmt.Errorf("parse proof template: %w", err)
	}
	return page, nil
}

func proofTemplatePath() (string, error) {
	directory, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locate proof template: %w", err)
	}
	for {
		candidate := filepath.Join(directory, "site", "proof", "index.tmpl")
		if info, statErr := os.Stat(candidate); statErr == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			return "", errors.New("locate proof template: site/proof/index.tmpl not found")
		}
		directory = parent
	}
}

func validatedCanonicalModel(m Model) (Model, error) {
	if m.provenance == (modelProvenance{}) {
		return Model{}, errors.New("missing model provenance: construct model with Load")
	}
	if err := catalog.Validate(m.Catalog); err != nil {
		return Model{}, fmt.Errorf("validate catalog: %w", err)
	}
	canonical, err := newModel(m.Catalog, m.Scenarios)
	if err != nil {
		return Model{}, err
	}
	if m.provenance != canonical.provenance {
		return Model{}, errors.New("semantic provenance mismatch: Catalog or Scenarios changed after Load")
	}
	for _, field := range []struct {
		name string
		got  any
		want any
	}{
		{"Catalog", m.Catalog, canonical.Catalog},
		{"Products", m.Products, canonical.Products},
		{"Platform", m.Platform, canonical.Platform},
		{"Scenarios", m.Scenarios, canonical.Scenarios},
		{"ExactSizes", m.ExactSizes, canonical.ExactSizes},
	} {
		if !reflect.DeepEqual(field.got, field.want) {
			return Model{}, fmt.Errorf("noncanonical %s: does not match catalog/scenario-derived model", field.name)
		}
	}
	return canonical, nil
}

func newModel(c catalog.Catalog, scenarios []Scenario) (Model, error) {
	if len(scenarios) == 0 {
		return Model{}, errors.New("scenarios are empty")
	}
	catalogCopy := c
	catalogCopy.Assets = slices.Clone(c.Assets)
	sort.Slice(catalogCopy.Assets, func(i, j int) bool {
		return catalogCopy.Assets[i].CanonicalName < catalogCopy.Assets[j].CanonicalName
	})
	assets := assetsByName(catalogCopy)
	products := make(map[string][]catalog.Asset)
	for _, asset := range catalogCopy.Assets {
		products[asset.Product] = append(products[asset.Product], asset)
	}

	seenIDs := make(map[string]struct{}, len(scenarios))
	coveredProducts := make(map[string]struct{}, len(products))
	exactSizes := make(map[int]struct{})
	modelScenarios := slices.Clone(scenarios)
	for index := range modelScenarios {
		scenario := modelScenarios[index]
		if err := validateScenario(scenario, assets); err != nil {
			return Model{}, fmt.Errorf("scenario[%d] %q: %w", index, scenario.ID, err)
		}
		if _, ok := seenIDs[scenario.ID]; ok {
			return Model{}, fmt.Errorf("duplicate scenario id %q", scenario.ID)
		}
		seenIDs[scenario.ID] = struct{}{}
		coveredProducts[assets[scenario.Asset].Product] = struct{}{}
		modelScenarios[index].Sizes = slices.Clone(scenario.Sizes)
		for _, size := range scenario.Sizes {
			exactSizes[size] = struct{}{}
		}
	}
	for product := range products {
		if _, ok := coveredProducts[product]; !ok {
			return Model{}, fmt.Errorf("missing product coverage for %q", product)
		}
	}

	sort.Slice(modelScenarios, func(i, j int) bool { return modelScenarios[i].ID < modelScenarios[j].ID })
	productIDs := make([]string, 0, len(products))
	for id := range products {
		productIDs = append(productIDs, id)
	}
	sort.Strings(productIDs)
	productProofs := make([]ProductProof, 0, len(productIDs))
	for _, id := range productIDs {
		productAssets := slices.Clone(products[id])
		sort.Slice(productAssets, func(i, j int) bool { return productAssets[i].CanonicalName < productAssets[j].CanonicalName })
		productProofs = append(productProofs, ProductProof{ID: id, Assets: productAssets})
	}
	sizes := make([]int, 0, len(exactSizes))
	for size := range exactSizes {
		sizes = append(sizes, size)
	}
	sort.Ints(sizes)
	provenance, err := semanticProvenance(catalogCopy, modelScenarios)
	if err != nil {
		return Model{}, err
	}
	platform, err := newPlatformProofs(productProofs)
	if err != nil {
		return Model{}, err
	}
	return Model{Catalog: catalogCopy, Products: productProofs, Platform: platform, Scenarios: modelScenarios, ExactSizes: sizes, provenance: provenance}, nil
}

func newPlatformProofs(products []ProductProof) ([]PlatformProof, error) {
	result := make([]PlatformProof, 0, len(products))
	for _, product := range products {
		// Developer Icons are third-party brand glyphs, not Arai Hû application
		// identities, so they intentionally have no launcher-package contract.
		if product.ID == "developer-icons" {
			continue
		}
		if !hasBrandAssets(product) {
			continue
		}
		expectedPath := path.Join("platform", "web", product.ID, "icon-maskable-512.png")
		var master *catalog.Asset
		for i := range product.Assets {
			asset := &product.Assets[i]
			if asset.Path != expectedPath || asset.Artwork != "icon" || asset.Appearance != "light" || asset.Surface != "plate" || asset.Framing != "launcher" || asset.Format != "png" || asset.Dimensions.Width != 512 || asset.Dimensions.Height != 512 {
				continue
			}
			if master != nil {
				return nil, fmt.Errorf("duplicate catalog-backed maskable 512 master for product %q", product.ID)
			}
			master = asset
		}
		if master == nil {
			return nil, fmt.Errorf("missing catalog-backed maskable 512 master for product %q", product.ID)
		}
		result = append(result, PlatformProof{
			Product: product.ID,
			Master:  *master,
			Packages: []PackageProof{
				{Product: product.ID, Kind: "web-manifest", Path: path.Join("platform", "web", product.ID, "manifest-icons.json"), ProvenancePath: "NOTICE"},
				{Product: product.ID, Kind: "android-adaptive-icon", Path: path.Join("platform", "android", product.ID, "res", "mipmap-anydpi-v26", "ic_launcher.xml"), ProvenancePath: "NOTICE"},
				{Product: product.ID, Kind: "apple-app-icon", Path: path.Join("platform", "apple", product.ID, "Assets.xcassets", "AppIcon.appiconset", "Contents.json"), ProvenancePath: "NOTICE"},
			},
		})
	}
	return result, nil
}

func hasBrandAssets(product ProductProof) bool {
	for _, asset := range product.Assets {
		if asset.Namespace == "brand" {
			return true
		}
	}
	return false
}

func semanticProvenance(c catalog.Catalog, scenarios []Scenario) (modelProvenance, error) {
	hash := sha256.New()
	_, _ = io.WriteString(hash, "araihu-proof-model-v1\x00")
	if err := catalog.Encode(hash, c); err != nil {
		return modelProvenance{}, fmt.Errorf("encode provenance catalog: %w", err)
	}
	_, _ = hash.Write([]byte{0})
	encoder := json.NewEncoder(hash)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(struct {
		Scenarios []Scenario `json:"scenarios"`
	}{Scenarios: scenarios}); err != nil {
		return modelProvenance{}, fmt.Errorf("encode provenance scenarios: %w", err)
	}
	var provenance modelProvenance
	copy(provenance[:], hash.Sum(nil))
	return provenance, nil
}

func validateScenario(scenario Scenario, assets map[string]catalog.Asset) error {
	if !lowerKebab.MatchString(scenario.ID) {
		return fmt.Errorf("invalid id %q", scenario.ID)
	}
	asset, ok := assets[scenario.Asset]
	if !ok {
		return fmt.Errorf("unknown canonicalName %q", scenario.Asset)
	}
	for _, field := range []struct {
		name string
		got  string
		want string
	}{
		{"group", scenario.Group, asset.Namespace},
		{"artwork", scenario.Artwork, asset.Artwork},
		{"appearance", scenario.Appearance, asset.Appearance},
		{"surface", scenario.Surface, asset.Surface},
		{"framing", scenario.Framing, asset.Framing},
	} {
		if field.got != field.want {
			return fmt.Errorf("%s %q does not match catalog %q", field.name, field.got, field.want)
		}
	}
	if !lowerKebab.MatchString(scenario.Context) {
		return fmt.Errorf("invalid context %q", scenario.Context)
	}
	if _, ok := allowedMasks[scenario.Mask]; !ok {
		return fmt.Errorf("invalid mask %q", scenario.Mask)
	}
	if scenario.Mask != "none" && (asset.Namespace != "brand" || asset.Artwork != "icon" || asset.Surface != "plate" || asset.Framing != "launcher") {
		return fmt.Errorf("mask requires launcher-framed plated brand icon")
	}
	if len(scenario.Sizes) == 0 {
		return errors.New("sizes are empty")
	}
	for i, size := range scenario.Sizes {
		if size <= 0 {
			return fmt.Errorf("size %d is not positive", size)
		}
		if i > 0 && scenario.Sizes[i-1] >= size {
			return fmt.Errorf("sizes must be strictly increasing")
		}
	}
	return nil
}

func assetsByName(c catalog.Catalog) map[string]catalog.Asset {
	assets := make(map[string]catalog.Asset, len(c.Assets))
	for _, asset := range c.Assets {
		assets[asset.CanonicalName] = asset
	}
	return assets
}

type scenarioValueValidator func(*json.Decoder) error

func validateScenarioDocumentShape(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := validateJSONObject(decoder, "scenarios", map[string]scenarioValueValidator{
		"scenarios": func(decoder *json.Decoder) error {
			return validateJSONArray(decoder, "scenarios", validateScenarioJSONObject)
		},
	}); err != nil {
		return fmt.Errorf("decode scenarios: %w", err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("decode scenarios: multiple JSON values")
		}
		return fmt.Errorf("decode scenarios: %w", err)
	}
	return nil
}

func validateScenarioJSONObject(decoder *json.Decoder) error {
	return validateJSONObject(decoder, "scenario", map[string]scenarioValueValidator{
		"id":         consumeJSONValue,
		"group":      consumeJSONValue,
		"asset":      consumeJSONValue,
		"artwork":    consumeJSONValue,
		"appearance": consumeJSONValue,
		"surface":    consumeJSONValue,
		"framing":    consumeJSONValue,
		"mask":       consumeJSONValue,
		"context":    consumeJSONValue,
		"sizes":      consumeJSONValue,
	})
}

func validateJSONObject(decoder *json.Decoder, name string, fields map[string]scenarioValueValidator) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("%s must be a JSON object", name)
	}
	seen := make(map[string]struct{}, len(fields))
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("%s key is not a string", name)
		}
		if _, ok := seen[key]; ok {
			return fmt.Errorf("duplicate %s key %q", name, key)
		}
		seen[key] = struct{}{}
		validator, ok := fields[key]
		if !ok {
			return fmt.Errorf("unknown %s key %q", name, key)
		}
		if err := validator(decoder); err != nil {
			return err
		}
	}
	token, err = decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '}' {
		return fmt.Errorf("%s must end with a JSON object", name)
	}
	return nil
}

func validateJSONArray(decoder *json.Decoder, name string, validator scenarioValueValidator) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '[' {
		return fmt.Errorf("%s must be a JSON array", name)
	}
	for decoder.More() {
		if err := validator(decoder); err != nil {
			return err
		}
	}
	token, err = decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != ']' {
		return fmt.Errorf("%s must end with a JSON array", name)
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	var value json.RawMessage
	return decoder.Decode(&value)
}
