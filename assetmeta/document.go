package assetmeta

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Document contains consumer-owned metadata associated with an acquisition
// inventory.
type Document[Root, ResourceMeta, DownloadMeta any] struct {
	Metadata  Root                                                    `yaml:"metadata"`
	Resources map[string]ResourceMetadata[ResourceMeta, DownloadMeta] `yaml:"resources"`
	inventory *Inventory
}

// ResourceMetadata contains consumer metadata for a resource and any
// annotated downloads.
type ResourceMetadata[ResourceMeta, DownloadMeta any] struct {
	Metadata  ResourceMeta                              `yaml:"metadata"`
	Downloads map[string]DownloadMetadata[DownloadMeta] `yaml:"downloads"`
}

// DownloadMetadata contains consumer metadata for one download.
type DownloadMetadata[DownloadMeta any] struct {
	Metadata DownloadMeta `yaml:"metadata"`
}

type yamlDocument[Root, ResourceMeta, DownloadMeta any] struct {
	Schema    int                                                     `yaml:"schema"`
	Metadata  Root                                                    `yaml:"metadata"`
	Resources map[string]ResourceMetadata[ResourceMeta, DownloadMeta] `yaml:"resources"`
}

// Load strictly decodes one typed metadata overlay and associates it with an
// independent copy of inventory.
func Load[Root, ResourceMeta, DownloadMeta any](reader io.Reader, inventory *Inventory) (*Document[Root, ResourceMeta, DownloadMeta], error) {
	if reader == nil {
		return nil, fmt.Errorf("reader is nil")
	}
	if inventory == nil {
		return nil, fmt.Errorf("inventory is nil")
	}

	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)
	var decoded yamlDocument[Root, ResourceMeta, DownloadMeta]
	if err := decoder.Decode(&decoded); err != nil {
		return nil, fmt.Errorf("decode overlay: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return nil, fmt.Errorf("multiple YAML documents")
	} else if err != io.EOF {
		return nil, fmt.Errorf("decode trailing overlay: %w", err)
	}
	if decoded.Schema != 1 {
		return nil, fmt.Errorf("unsupported schema %d", decoded.Schema)
	}
	var unknown []string
	for resourceName, resourceMetadata := range decoded.Resources {
		resource, ok := inventory.Resource(resourceName)
		if !ok {
			unknown = append(unknown, fmt.Sprintf("resource %q", resourceName))
			continue
		}
		downloads := make(map[string]struct{}, len(resource.Downloads))
		for _, download := range resource.Downloads {
			downloads[download.Name] = struct{}{}
		}
		for downloadName := range resourceMetadata.Downloads {
			if _, ok := downloads[downloadName]; !ok {
				unknown = append(unknown, fmt.Sprintf("download %q", resourceName+"/"+downloadName))
			}
		}
	}
	if len(unknown) != 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown overlay keys: %s", strings.Join(unknown, ", "))
	}

	owned, err := NewInventory(inventory.Resources())
	if err != nil {
		return nil, fmt.Errorf("copy inventory: %w", err)
	}
	return &Document[Root, ResourceMeta, DownloadMeta]{
		Metadata:  decoded.Metadata,
		Resources: decoded.Resources,
		inventory: owned,
	}, nil
}

// Inventory returns an independent copy of the document inventory.
func (d *Document[Root, ResourceMeta, DownloadMeta]) Inventory() *Inventory {
	if d == nil || d.inventory == nil {
		return nil
	}
	inventory, err := NewInventory(d.inventory.Resources())
	if err != nil {
		return nil
	}
	return inventory
}

// Resolve delegates reference resolution to the document inventory.
func (d *Document[Root, ResourceMeta, DownloadMeta]) Resolve(ref Ref) (Resolved, bool) {
	if d == nil {
		return Resolved{}, false
	}
	return d.inventory.Resolve(ref)
}
