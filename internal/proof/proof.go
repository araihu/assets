package proof

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"regexp"
	"slices"
	"sort"

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

// Build verifies that every scenario asset exists in fsys. Rendering belongs
// to the next proof layer; this model layer intentionally writes no document.
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
	checked := make(map[string]struct{}, len(m.Scenarios))
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
	return nil
}

func validatedCanonicalModel(m Model) (Model, error) {
	if err := catalog.Validate(m.Catalog); err != nil {
		return Model{}, fmt.Errorf("validate catalog: %w", err)
	}
	canonical, err := newModel(m.Catalog, m.Scenarios)
	if err != nil {
		return Model{}, err
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
	return Model{Catalog: catalogCopy, Products: productProofs, Scenarios: modelScenarios, ExactSizes: sizes}, nil
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
