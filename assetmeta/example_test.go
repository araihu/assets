package assetmeta_test

import (
	"fmt"
	"strings"

	"github.com/araihu/assets/assetmeta"
)

func ExampleLoad() {
	type exampleRootMetadata struct {
		Entry assetmeta.Ref `yaml:"entry"`
	}
	type exampleResourceMetadata struct {
		Label string `yaml:"label"`
	}
	type exampleDownloadMetadata struct {
		Role string `yaml:"role"`
	}

	inventory, err := assetmeta.NewInventory([]assetmeta.Resource{{
		Name:    "alpinejs",
		Version: "3.14.9",
		Downloads: []assetmeta.Download{{
			Name:      "core-js",
			URL:       "https://cdn.example/alpine.js",
			Path:      "assets/js/alpine.js",
			Integrity: "sha384-base64-value",
			Hash:      "sha384:0123456789abcdef",
		}},
	}})
	if err != nil {
		panic(err)
	}
	const overlay = `schema: 1
metadata:
  entry: alpinejs/core-js
resources:
  alpinejs:
    metadata:
      label: Alpine.js
    downloads:
      core-js:
        metadata:
          role: runtime
`
	document, err := assetmeta.Load[exampleRootMetadata, exampleResourceMetadata, exampleDownloadMetadata](
		strings.NewReader(overlay), inventory,
	)
	if err != nil {
		panic(err)
	}
	resolved, ok := document.Resolve(document.Metadata.Entry)
	if !ok {
		panic("entry did not resolve")
	}
	if err := assetmeta.ValidateRefs(document.Inventory(), document.Metadata.Entry); err != nil {
		panic(err)
	}
	fmt.Println(document.Metadata.Entry, resolved.Download.Hash)
	// Output:
	// alpinejs/core-js sha384:0123456789abcdef
}
