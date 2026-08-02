package acquisition

import (
	"io/fs"

	"github.com/araihu/assets/assetmeta"
)

// Record is one generated acquisition record used by offline builders.
type Record struct {
	URL       string
	Path      string
	Integrity string
	Hash      string
}

// Source opens and describes locked embedded downloads.
type Source interface {
	Open(resource, download string) (fs.File, error)
	Lookup(resource, download string) (Record, bool)
}

type embeddedSource struct{}

// Embedded returns the immutable generated Muamba source.
func Embedded() Source { return embeddedSource{} }

func (embeddedSource) Open(resource, download string) (fs.File, error) {
	return MuambaOpen(resource, download)
}

func (embeddedSource) Lookup(resource, download string) (Record, bool) {
	group, ok := MuambaResourceByName(resource)
	if !ok {
		return Record{}, false
	}
	for _, item := range group.Downloads {
		if item.Name == download {
			return Record{URL: item.URL, Path: item.Path, Integrity: item.Integrity, Hash: item.Hash}, true
		}
	}
	return Record{}, false
}

// Inventory adapts the generated Muamba registry for typed metadata overlays.
func Inventory() (*assetmeta.Inventory, error) {
	resources := make([]assetmeta.Resource, 0, len(MuambaResources()))
	for _, resource := range MuambaResources() {
		downloads := make([]assetmeta.Download, 0, len(resource.Downloads))
		for _, download := range resource.Downloads {
			downloads = append(downloads, assetmeta.Download{
				Name: download.Name, URL: download.URL, Path: download.Path,
				Integrity: download.Integrity, Hash: download.Hash,
			})
		}
		resources = append(resources, assetmeta.Resource{Name: resource.Name, Version: resource.Version, Downloads: downloads})
	}
	return assetmeta.NewInventory(resources)
}
