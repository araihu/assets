// Package proof defines the catalog-backed data model for identity review.
package proof

import "github.com/araihu/assets/internal/catalog"

// Scenario declares one review context for a published catalog asset.
// Semantic fields repeat the referenced asset's metadata so scenario files are
// independently reviewable without becoming a second asset catalog.
type Scenario struct {
	ID         string `json:"id"`
	Group      string `json:"group"`
	Asset      string `json:"asset"`
	Artwork    string `json:"artwork"`
	Appearance string `json:"appearance"`
	Surface    string `json:"surface"`
	Framing    string `json:"framing"`
	Mask       string `json:"mask"`
	Context    string `json:"context"`
	Sizes      []int  `json:"sizes"`
}

// ProductProof collects catalog assets by product for rendering layers.
type ProductProof struct {
	ID     string
	Assets []catalog.Asset
}

// Model is a validated, deterministic proof view model. It deliberately has
// no HTML or rendering concerns.
type Model struct {
	Catalog    catalog.Catalog
	Products   []ProductProof
	Scenarios  []Scenario
	ExactSizes []int
}

// HasProduct reports whether id has a product proof in the model.
func (m Model) HasProduct(id string) bool {
	for _, product := range m.Products {
		if product.ID == id {
			return true
		}
	}
	return false
}
