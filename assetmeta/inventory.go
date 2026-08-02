package assetmeta

import "fmt"

// Resource is one versioned acquisition group.
type Resource struct {
	Name      string
	Version   string
	Downloads []Download
}

// Download is one acquired file and its integrity data.
type Download struct {
	Name      string
	URL       string
	Path      string
	Integrity string
	Hash      string
}

// Resolved contains the resource and download selected by a reference.
type Resolved struct {
	Resource Resource
	Download Download
}

// Inventory is an immutable, indexed acquisition inventory.
type Inventory struct {
	resources       []Resource
	resourcesByName map[string]Resource
}

// NewInventory validates and copies resources into an immutable inventory.
func NewInventory(resources []Resource) (*Inventory, error) {
	inventory := &Inventory{
		resources:       make([]Resource, 0, len(resources)),
		resourcesByName: make(map[string]Resource, len(resources)),
	}

	for _, resource := range resources {
		if resource.Name == "" {
			return nil, fmt.Errorf("resource name is empty")
		}
		if _, exists := inventory.resourcesByName[resource.Name]; exists {
			return nil, fmt.Errorf("resource %q: duplicate name", resource.Name)
		}
		if resource.Version == "" {
			return nil, fmt.Errorf("resource %q: version is empty", resource.Name)
		}
		if len(resource.Downloads) == 0 {
			return nil, fmt.Errorf("resource %q: downloads are empty", resource.Name)
		}

		downloadNames := make(map[string]struct{}, len(resource.Downloads))
		for _, download := range resource.Downloads {
			if download.Name == "" {
				return nil, fmt.Errorf("resource %q: download name is empty", resource.Name)
			}
			if _, exists := downloadNames[download.Name]; exists {
				return nil, fmt.Errorf("resource %q download %q: duplicate name", resource.Name, download.Name)
			}
			downloadNames[download.Name] = struct{}{}
			if download.URL == "" {
				return nil, fmt.Errorf("resource %q download %q: URL is empty", resource.Name, download.Name)
			}
			if download.Path == "" {
				return nil, fmt.Errorf("resource %q download %q: path is empty", resource.Name, download.Name)
			}
			if download.Integrity == "" {
				return nil, fmt.Errorf("resource %q download %q: integrity is empty", resource.Name, download.Name)
			}
			if download.Hash == "" {
				return nil, fmt.Errorf("resource %q download %q: hash is empty", resource.Name, download.Name)
			}
		}

		owned := cloneResource(resource)
		inventory.resources = append(inventory.resources, owned)
		inventory.resourcesByName[owned.Name] = owned
	}

	return inventory, nil
}

// Resources returns resources in declaration order as caller-owned copies.
func (i *Inventory) Resources() []Resource {
	if i == nil {
		return nil
	}
	resources := make([]Resource, len(i.resources))
	for index, resource := range i.resources {
		resources[index] = cloneResource(resource)
	}
	return resources
}

// Resource returns a named resource as a caller-owned copy.
func (i *Inventory) Resource(name string) (Resource, bool) {
	if i == nil {
		return Resource{}, false
	}
	resource, ok := i.resourcesByName[name]
	if !ok {
		return Resource{}, false
	}
	return cloneResource(resource), true
}

func cloneResource(resource Resource) Resource {
	resource.Downloads = append([]Download(nil), resource.Downloads...)
	return resource
}
