package assetmeta_test

import (
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/araihu/assets/assetmeta"
)

type rootMetadata struct {
	Order []assetmeta.Ref `yaml:"order"`
}

type resourceMetadata struct {
	Label string `yaml:"label"`
}

type downloadMetadata struct {
	Role       string        `yaml:"role"`
	License    assetmeta.Ref `yaml:"license"`
	Provenance assetmeta.Ref `yaml:"provenance"`
}

func TestLoadValidSparseOverlay(t *testing.T) {
	inventory := documentInventory(t)
	overlay := `schema: 1
metadata:
  order:
    - alpinejs/core-js
resources:
  alpinejs:
    metadata:
      label: Alpine.js
    downloads:
      core-js:
        metadata:
          role: alpine
          license: alpinejs/license
          provenance: alpinejs/core-package
`

	document, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader(overlay), inventory)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wantOrder := []assetmeta.Ref{{Resource: "alpinejs", Download: "core-js"}}
	if !reflect.DeepEqual(document.Metadata.Order, wantOrder) {
		t.Fatalf("Metadata.Order = %#v, want %#v", document.Metadata.Order, wantOrder)
	}
	resource, ok := document.Resources["alpinejs"]
	if !ok || resource.Metadata.Label != "Alpine.js" {
		t.Fatalf("resource metadata = %#v, %v", resource, ok)
	}
	download, ok := resource.Downloads["core-js"]
	if !ok || download.Metadata.Role != "alpine" || download.Metadata.License.String() != "alpinejs/license" || download.Metadata.Provenance.String() != "alpinejs/core-package" {
		t.Fatalf("download metadata = %#v, %v", download, ok)
	}
	if _, ok := resource.Downloads["license"]; ok {
		t.Fatal("sparse overlay synthesized license metadata")
	}

	resolved, ok := document.Resolve(wantOrder[0])
	if !ok || resolved.Download.Hash != "sha384:0123456789abcdef" {
		t.Fatalf("Resolve() = %#v, %v", resolved, ok)
	}
	firstInventory := document.Inventory()
	secondInventory := document.Inventory()
	if firstInventory == nil || firstInventory == inventory || firstInventory == secondInventory {
		t.Fatalf("Inventory() did not return independent copies: source=%p first=%p second=%p", inventory, firstInventory, secondInventory)
	}
	resources := firstInventory.Resources()
	resources[0].Downloads[0].Hash = "changed"
	again, _ := document.Resolve(wantOrder[0])
	if again.Download.Hash != "sha384:0123456789abcdef" {
		t.Fatalf("Inventory copy mutation reached document: %#v", again)
	}
}

func TestLoadRejectsNilInputs(t *testing.T) {
	inventory := documentInventory(t)
	var reader io.Reader
	if _, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](reader, inventory); err == nil || err.Error() != "reader is nil" {
		t.Fatalf("Load(nil reader) error = %v", err)
	}
	if _, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader("schema: 1\n"), nil); err == nil || err.Error() != "inventory is nil" {
		t.Fatalf("Load(nil inventory) error = %v", err)
	}
}

func TestLoadRejectsMalformedOrUnknownYAML(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "empty YAML", raw: "", want: "decode overlay: EOF"},
		{name: "missing schema", raw: "metadata: {}\n", want: "unsupported schema 0"},
		{name: "zero schema", raw: "schema: 0\n", want: "unsupported schema 0"},
		{name: "future schema", raw: "schema: 2\n", want: "unsupported schema 2"},
		{name: "unknown top-level field", raw: "schema: 1\nextra: true\n", want: "field extra not found"},
		{name: "unknown root metadata field", raw: "schema: 1\nmetadata:\n  unexpected: true\n", want: "field unexpected not found"},
		{name: "unknown resource metadata field", raw: "schema: 1\nresources:\n  alpinejs:\n    metadata:\n      unexpected: true\n", want: "field unexpected not found"},
		{name: "unknown download metadata field", raw: "schema: 1\nresources:\n  alpinejs:\n    downloads:\n      core-js:\n        metadata:\n          unexpected: true\n", want: "field unexpected not found"},
		{name: "second document", raw: "schema: 1\n---\nnull\n", want: "multiple YAML documents"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader(test.raw), documentInventory(t))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("Load() error = %v, want containing %q", err, test.want)
			}
		})
	}
}

func TestLoadRejectsUnknownOverlayResourcesDeterministically(t *testing.T) {
	raw := `schema: 1
resources:
  missing-z:
    metadata: {}
  missing-a:
    metadata: {}
`
	_, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader(raw), documentInventory(t))
	const want = `unknown overlay keys: resource "missing-a", resource "missing-z"`
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadRejectsUnknownOverlayDownloadsDeterministically(t *testing.T) {
	raw := `schema: 1
resources:
  alpinejs:
    downloads:
      missing-z:
        metadata: {}
      missing-a:
        metadata: {}
`
	_, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader(raw), documentInventory(t))
	const want = `unknown overlay keys: download "alpinejs/missing-a", download "alpinejs/missing-z"`
	if err == nil || err.Error() != want {
		t.Fatalf("Load() error = %v, want %q", err, want)
	}
}

func TestLoadDoesNotValidateConsumerReferences(t *testing.T) {
	raw := `schema: 1
resources:
  alpinejs:
    downloads:
      core-js:
        metadata:
          role: alpine
          license: missing/license
          provenance: alpinejs/core-package
`
	document, err := assetmeta.Load[rootMetadata, resourceMetadata, downloadMetadata](strings.NewReader(raw), documentInventory(t))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	metadata := document.Resources["alpinejs"].Downloads["core-js"].Metadata
	if err := assetmeta.ValidateRefs(document.Inventory(), metadata.License, metadata.Provenance); err == nil || err.Error() != "unknown references: missing/license" {
		t.Fatalf("ValidateRefs() error = %v", err)
	}
}

func documentInventory(t *testing.T) *assetmeta.Inventory {
	t.Helper()
	inventory, err := assetmeta.NewInventory([]assetmeta.Resource{{
		Name:    "alpinejs",
		Version: "3.14.9",
		Downloads: []assetmeta.Download{
			{Name: "core-js", URL: "https://cdn.example/alpine.js", Path: "assets/js/alpine.js", Integrity: "sha384-core", Hash: "sha384:0123456789abcdef"},
			{Name: "license", URL: "https://cdn.example/LICENSE", Path: "licenses/alpine.txt", Integrity: "sha384-license", Hash: "sha384:license"},
			{Name: "core-package", URL: "https://cdn.example/package.json", Path: "provenance/alpine.json", Integrity: "sha384-package", Hash: "sha384:package"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return inventory
}
