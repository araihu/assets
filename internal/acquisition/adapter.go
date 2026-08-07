package acquisition

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/araihu/assets/assetmeta"
	"gopkg.in/yaml.v3"
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
	Directory(resource, directory string) (LockedDirectory, bool)
	OpenPath(name string) (fs.File, error)
}

// LockedDirectory is one exact archive and its deterministic resolved file set.
type LockedDirectory struct {
	ID        string
	URL       string
	Path      string
	Size      int64
	Integrity string
	Files     []LockedDirectoryFile
}

// LockedDirectoryFile is one exact source-to-destination member of an archive lock.
type LockedDirectoryFile struct {
	Source    string
	Path      string
	Size      int64
	Integrity string
}

type lockDocument struct {
	Schema      int               `yaml:"schema"`
	Files       []lockedFile      `yaml:"files"`
	Directories []lockedDirectory `yaml:"directories"`
}

type lockedFile struct {
	ID        string `yaml:"id"`
	URL       string `yaml:"url"`
	Path      string `yaml:"path"`
	Size      int64  `yaml:"size"`
	Integrity string `yaml:"integrity"`
}

type lockedDirectory struct {
	ID        string                `yaml:"id"`
	URL       string                `yaml:"url"`
	Path      string                `yaml:"path"`
	Size      int64                 `yaml:"size"`
	Integrity string                `yaml:"integrity"`
	Files     []lockedDirectoryFile `yaml:"files"`
}

type lockedDirectoryFile struct {
	Source    string `yaml:"source"`
	Path      string `yaml:"path"`
	Size      int64  `yaml:"size"`
	Integrity string `yaml:"integrity"`
}

type repositorySource struct {
	root        fs.FS
	directories map[string]LockedDirectory
}

// Repository loads the exact Muamba directory locks used by the offline builder.
func Repository(root fs.FS, lockPath string) (Source, error) {
	if root == nil {
		return nil, fmt.Errorf("acquisition repository: root is nil")
	}
	file, err := root.Open(lockPath)
	if err != nil {
		return nil, fmt.Errorf("acquisition repository: open %s: %w", lockPath, err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var document lockDocument
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("acquisition repository: decode %s: %w", lockPath, err)
	}
	if document.Schema != 1 {
		return nil, fmt.Errorf("acquisition repository: lock schema = %d, want 1", document.Schema)
	}
	directories := make(map[string]LockedDirectory, len(document.Directories))
	for _, raw := range document.Directories {
		directory, err := validateLockedDirectory(raw)
		if err != nil {
			return nil, fmt.Errorf("acquisition repository: %w", err)
		}
		if _, duplicate := directories[directory.ID]; duplicate {
			return nil, fmt.Errorf("acquisition repository: duplicate directory %q", directory.ID)
		}
		directories[directory.ID] = directory
	}
	return repositorySource{root: root, directories: directories}, nil
}

func (repositorySource) Open(resource, download string) (fs.File, error) {
	return MuambaOpen(resource, download)
}

func (repositorySource) Lookup(resource, download string) (Record, bool) {
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

func (source repositorySource) Directory(resource, directory string) (LockedDirectory, bool) {
	locked, ok := source.directories[resource+"/"+directory]
	locked.Files = slices.Clone(locked.Files)
	return locked, ok
}

func (source repositorySource) OpenPath(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fmt.Errorf("invalid acquisition path %q", name)
	}
	return source.root.Open(name)
}

func validateLockedDirectory(raw lockedDirectory) (LockedDirectory, error) {
	if raw.ID == "" || raw.URL == "" || raw.Path == "" || raw.Size <= 0 || !validSRI(raw.Integrity) || len(raw.Files) == 0 {
		return LockedDirectory{}, fmt.Errorf("invalid directory lock %q", raw.ID)
	}
	if !strings.HasPrefix(raw.URL, "https://") || !fs.ValidPath(raw.Path) {
		return LockedDirectory{}, fmt.Errorf("invalid directory lock %q source", raw.ID)
	}
	files := make([]LockedDirectoryFile, 0, len(raw.Files))
	seenSource := make(map[string]struct{}, len(raw.Files))
	seenPath := make(map[string]struct{}, len(raw.Files))
	for _, item := range raw.Files {
		if !fs.ValidPath(item.Source) || !fs.ValidPath(item.Path) || item.Size < 0 || !validSRI(item.Integrity) {
			return LockedDirectory{}, fmt.Errorf("invalid directory lock %q member %q", raw.ID, item.Source)
		}
		if item.Path != raw.Path && !strings.HasPrefix(item.Path, raw.Path+"/") {
			return LockedDirectory{}, fmt.Errorf("directory lock %q member escapes destination: %q", raw.ID, item.Path)
		}
		if _, duplicate := seenSource[item.Source]; duplicate {
			return LockedDirectory{}, fmt.Errorf("directory lock %q duplicates source %q", raw.ID, item.Source)
		}
		if _, duplicate := seenPath[item.Path]; duplicate {
			return LockedDirectory{}, fmt.Errorf("directory lock %q duplicates path %q", raw.ID, item.Path)
		}
		seenSource[item.Source] = struct{}{}
		seenPath[item.Path] = struct{}{}
		files = append(files, LockedDirectoryFile{Source: item.Source, Path: item.Path, Size: item.Size, Integrity: item.Integrity})
	}
	if !slices.IsSortedFunc(files, func(a, b LockedDirectoryFile) int { return strings.Compare(a.Path, b.Path) }) {
		return LockedDirectory{}, fmt.Errorf("directory lock %q members are not sorted", raw.ID)
	}
	return LockedDirectory{ID: raw.ID, URL: raw.URL, Path: path.Clean(raw.Path), Size: raw.Size, Integrity: raw.Integrity, Files: files}, nil
}

func validSRI(value string) bool {
	const prefix = "sha384-"
	return strings.HasPrefix(value, prefix) && len(value) == len(prefix)+64
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
