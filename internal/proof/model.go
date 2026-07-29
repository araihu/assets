// Package proof defines the catalog-backed data model for identity review.
package proof

import (
	"crypto/sha256"

	"github.com/araihu/assets/internal/catalog"
)

// modelProvenance binds a Model to canonical semantic inputs accepted by Load.
type modelProvenance [sha256.Size]byte

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

// PackageProof is a release-package metadata reference not represented by a
// catalog asset. Its product, kind, and path are fixed by the proof contract.
type PackageProof struct {
	Product        string
	Kind           string
	Path           string
	ProvenancePath string
}

// PlatformProof binds a product's literal launcher master to catalog metadata
// and retains strict references to its non-catalog package metadata.
type PlatformProof struct {
	Product  string
	Master   catalog.Asset
	Packages []PackageProof
}

// Model is a validated, deterministic proof view model. It deliberately has
// no HTML or rendering concerns.
type Model struct {
	Catalog    catalog.Catalog
	Products   []ProductProof
	Platform   []PlatformProof
	Scenarios  []Scenario
	ExactSizes []int
	provenance modelProvenance
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
