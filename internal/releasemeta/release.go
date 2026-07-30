// Package releasemeta builds the immutable inventory for one asset release.
package releasemeta

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
	"slices"
	"strings"
)

const schemaVersion = 1

var (
	releaseTag = regexp.MustCompile(`^v(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)\.(?:0|[1-9][0-9]*)(?:-(?:(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*)(?:\.(?:0|[1-9][0-9]*|[0-9A-Za-z-]*[A-Za-z-][0-9A-Za-z-]*))*))?(?:\+[0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*)?$`)
	sha256Hex  = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// Input is the captured, pre-release filesystem used to inventory one release.
type Input struct {
	Release          string
	IdentityRevision int
	RuntimeVersion   int
	Files            fs.FS
}

// File records one immutable release member.
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Document is the canonical release.json contract.
type Document struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Release          string `json:"release"`
	IdentityRevision int    `json:"identityRevision"`
	RuntimeVersion   int    `json:"runtimeVersion"`
	CatalogSHA256    string `json:"catalogSha256"`
	ThemesSHA256     string `json:"themesSha256"`
	CampaignsSHA256  string `json:"campaignsSha256"`
	Files            []File `json:"files"`
}

// Build inventories every regular file in input. release.json must not exist
// yet, so it cannot claim a hash for itself.
func Build(input Input) (Document, error) {
	if !releaseTag.MatchString(input.Release) {
		return Document{}, fmt.Errorf("release metadata: invalid release %q", input.Release)
	}
	if input.IdentityRevision < 1 {
		return Document{}, fmt.Errorf("release metadata: invalid identity revision %d", input.IdentityRevision)
	}
	if input.RuntimeVersion != 1 {
		return Document{}, fmt.Errorf("release metadata: unsupported runtime version %d", input.RuntimeVersion)
	}
	if input.Files == nil {
		return Document{}, errors.New("release metadata: files are nil")
	}

	files := make([]File, 0)
	err := fs.WalkDir(input.Files, ".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == "." {
			return nil
		}
		if !validPath(path) {
			return fmt.Errorf("invalid file path %q", path)
		}
		info, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect %s: %w", path, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %s", path)
		}
		if info.IsDir() {
			return nil
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("non-regular file %s", path)
		}
		if path == "release.json" {
			return errors.New("release.json must not hash itself")
		}
		data, err := fs.ReadFile(input.Files, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}
		sum := sha256.Sum256(data)
		files = append(files, File{Path: path, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))})
		return nil
	})
	if err != nil {
		return Document{}, fmt.Errorf("release metadata: inventory: %w", err)
	}
	slices.SortFunc(files, compareFiles)

	document := Document{SchemaVersion: schemaVersion, Release: input.Release, IdentityRevision: input.IdentityRevision, RuntimeVersion: input.RuntimeVersion, Files: files}
	for _, file := range files {
		switch file.Path {
		case "catalog.json":
			document.CatalogSHA256 = file.SHA256
		case "themes.json":
			document.ThemesSHA256 = file.SHA256
		case "campaigns.json":
			document.CampaignsSHA256 = file.SHA256
		}
	}
	if err := document.Validate(); err != nil {
		return Document{}, err
	}
	return document, nil
}

// Validate checks a complete release document, including its sorted inventory.
func (document Document) Validate() error {
	if document.SchemaVersion != schemaVersion {
		return fmt.Errorf("release metadata: unsupported schemaVersion %d", document.SchemaVersion)
	}
	if !releaseTag.MatchString(document.Release) {
		return fmt.Errorf("release metadata: invalid release %q", document.Release)
	}
	if document.IdentityRevision < 1 {
		return fmt.Errorf("release metadata: invalid identity revision %d", document.IdentityRevision)
	}
	if document.RuntimeVersion != 1 {
		return fmt.Errorf("release metadata: unsupported runtime version %d", document.RuntimeVersion)
	}
	for _, hash := range []struct {
		name  string
		value string
	}{{"catalogSha256", document.CatalogSHA256}, {"themesSha256", document.ThemesSHA256}, {"campaignsSha256", document.CampaignsSHA256}} {
		if !sha256Hex.MatchString(hash.value) {
			return fmt.Errorf("release metadata: invalid %s %q", hash.name, hash.value)
		}
	}
	if len(document.Files) == 0 {
		return errors.New("release metadata: files are empty")
	}
	previous := ""
	documentHashes := map[string]string{}
	for index, file := range document.Files {
		if !validPath(file.Path) || file.Path == "release.json" {
			return fmt.Errorf("release metadata: invalid inventory path %q", file.Path)
		}
		if index > 0 && compareFilePaths(previous, file.Path) >= 0 {
			return fmt.Errorf("release metadata: files are not sorted and unique at %q", file.Path)
		}
		if !sha256Hex.MatchString(file.SHA256) || file.Size < 0 {
			return fmt.Errorf("release metadata: invalid inventory file %q", file.Path)
		}
		documentHashes[file.Path] = file.SHA256
		previous = file.Path
	}
	for name, hash := range map[string]string{"catalog.json": document.CatalogSHA256, "themes.json": document.ThemesSHA256, "campaigns.json": document.CampaignsSHA256} {
		if documentHashes[name] != hash {
			return fmt.Errorf("release metadata: %s hash does not match inventory", name)
		}
	}
	return nil
}

// Encode validates and serializes canonical two-space release JSON.
func Encode(document Document) ([]byte, error) {
	if err := document.Validate(); err != nil {
		return nil, err
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(document); err != nil {
		return nil, fmt.Errorf("release metadata: encode: %w", err)
	}
	return output.Bytes(), nil
}

func validPath(path string) bool {
	return path != "." && fs.ValidPath(path) && !strings.Contains(path, `\`) && !strings.Contains(strings.Split(path, "/")[0], ":")
}

func compareFiles(a, b File) int { return compareFilePaths(a.Path, b.Path) }

func compareFilePaths(a, b string) int {
	order := map[string]int{"catalog.json": 0, "themes.json": 1, "campaigns.json": 2}
	ai, aDocument := order[a]
	bi, bDocument := order[b]
	if aDocument && bDocument {
		return ai - bi
	}
	if aDocument {
		return -1
	}
	if bDocument {
		return 1
	}
	return strings.Compare(a, b)
}
