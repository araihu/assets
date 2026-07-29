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

// Build verifies every rendered asset before atomically writing one semantic
// proof document. It retains the canonical model accepted by Load.
func Build(m Model, fsys fs.FS, output io.Writer) error {
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
	checked := make(map[string]struct{}, len(m.Scenarios)+len(m.Products)*5)
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
	for _, product := range proofProducts(m.Products) {
		for _, proofPath := range product.RequiredPaths {
			if _, already := checked[proofPath]; already {
				continue
			}
			checked[proofPath] = struct{}{}
			info, err := fs.Stat(fsys, proofPath)
			if err != nil {
				return fmt.Errorf("missing referenced distribution file %q for product %q: %w", proofPath, product.ID, err)
			}
			if !info.Mode().IsRegular() {
				return fmt.Errorf("referenced distribution file %q for product %q is not regular", proofPath, product.ID)
			}
		}
	}
	if hasUIScenarios(m, assets) {
		const spritePath = "icons/ui/sprite.svg"
		info, err := fs.Stat(fsys, spritePath)
		if err != nil {
			return fmt.Errorf("missing referenced distribution file %q for UI sprite rail: %w", spritePath, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("referenced distribution file %q for UI sprite rail is not regular", spritePath)
		}
	}

	document, err := newDocumentModel(m)
	if err != nil {
		return err
	}
	page, err := parseDocumentTemplate()
	if err != nil {
		return err
	}
	var rendered bytes.Buffer
	if err := page.Execute(&rendered, document); err != nil {
		return fmt.Errorf("render proof document: %w", err)
	}
	if _, err := output.Write(rendered.Bytes()); err != nil {
		return fmt.Errorf("write proof document: %w", err)
	}
	return nil
}

func hasUIScenarios(m Model, assets map[string]catalog.Asset) bool {
	for _, scenario := range m.Scenarios {
		if assets[scenario.Asset].Namespace == "ui" {
			return true
		}
	}
	return false
}

type documentModel struct {
	Release        string
	Products       []documentProduct
	ExactSizes     []int
	BrandScenarios []documentSpecimen
	UIScenarios    []documentSpecimen
	Metrics        []documentMetric
	Licenses       []documentLicense
}

type documentProduct struct {
	ID            string
	Name          string
	MasterURL     string
	WebPackageURL string
	AndroidURL    string
	AppleURL      string
	RequiredPaths []string
}

type documentSpecimen struct {
	ID        string
	Product   string
	Artwork   string
	Variant   string
	Mask      string
	Sizes     []int
	URL       string
	SpriteURL string
}

type documentMetric struct {
	Product string
	Asset   string
	ViewBox string
	Format  string
}

type documentLicense struct {
	Product string
	Asset   string
	License string
	Source  string
	URL     string
}

func newDocumentModel(m Model) (documentModel, error) {
	assets := assetsByName(m.Catalog)
	document := documentModel{
		Release:        m.Catalog.Release,
		Products:       proofProducts(m.Products),
		ExactSizes:     slices.Clone(m.ExactSizes),
		BrandScenarios: make([]documentSpecimen, 0),
		UIScenarios:    make([]documentSpecimen, 0),
		Metrics:        make([]documentMetric, 0, len(m.Catalog.Assets)),
		Licenses:       make([]documentLicense, 0, len(m.Catalog.Assets)),
	}
	for _, scenario := range m.Scenarios {
		asset := assets[scenario.Asset]
		specimen := documentSpecimen{
			ID:        scenario.ID,
			Product:   productName(asset.Product),
			Artwork:   asset.Artwork,
			Variant:   strings.Join([]string{asset.Surface, asset.Appearance, asset.Framing}, " "),
			Mask:      scenario.Mask,
			Sizes:     slices.Clone(scenario.Sizes),
			URL:       relativeProofURL(asset.Path),
			SpriteURL: relativeProofURL("icons/ui/sprite.svg") + "#" + asset.SpriteSymbol,
		}
		if asset.Namespace == "brand" {
			document.BrandScenarios = append(document.BrandScenarios, specimen)
		} else {
			document.UIScenarios = append(document.UIScenarios, specimen)
		}
	}
	for _, asset := range m.Catalog.Assets {
		document.Metrics = append(document.Metrics, documentMetric{
			Product: productName(asset.Product), Asset: asset.CanonicalName,
			ViewBox: asset.Dimensions.ViewBox, Format: asset.Format,
		})
		document.Licenses = append(document.Licenses, documentLicense{
			Product: productName(asset.Product), Asset: asset.CanonicalName,
			License: asset.License, Source: asset.Source, URL: relativeProofURL(asset.Path),
		})
	}
	return document, nil
}

func proofProducts(products []ProductProof) []documentProduct {
	result := make([]documentProduct, 0, len(products))
	for _, product := range products {
		if product.ID == "heroicons" {
			continue
		}
		base := path.Join("platform", "web", product.ID)
		result = append(result, documentProduct{
			ID: product.ID, Name: productName(product.ID),
			MasterURL:     relativeProofURL(path.Join(base, "icon-512.png")),
			WebPackageURL: relativeProofURL(path.Join(base, "manifest-icons.json")),
			AndroidURL:    relativeProofURL(path.Join("platform", "android", product.ID, "res", "mipmap-anydpi-v26", "ic_launcher.xml")),
			AppleURL:      relativeProofURL(path.Join("platform", "apple", product.ID, "Assets.xcassets", "AppIcon.appiconset", "Contents.json")),
			RequiredPaths: []string{
				path.Join(base, "icon-512.png"), path.Join(base, "manifest-icons.json"),
				path.Join("platform", "android", product.ID, "res", "mipmap-anydpi-v26", "ic_launcher.xml"),
				path.Join("platform", "apple", product.ID, "Assets.xcassets", "AppIcon.appiconset", "Contents.json"),
			},
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
	default:
		return id
	}
}

func relativeProofURL(distributionPath string) string { return "../" + path.Clean(distributionPath) }

func parseDocumentTemplate() (*template.Template, error) {
	file, err := proofTemplatePath()
	if err != nil {
		return nil, err
	}
	page, err := template.ParseFiles(file)
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
	return Model{Catalog: catalogCopy, Products: productProofs, Scenarios: modelScenarios, ExactSizes: sizes, provenance: provenance}, nil
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
