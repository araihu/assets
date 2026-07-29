// Package provenance vendors locked third-party UI-icon sources and builds
// deterministic UI artifacts from tracked vendor bytes.
package provenance

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/sprite"
	"github.com/araihu/assets/internal/svgasset"
)

// Doer is the small HTTP boundary used by Sync.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// NewHTTPClient returns the only live-network client suitable for Sync.
// Redirects are retained as responses and rejected by Sync.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Sync downloads exactly one locked Heroicons source into root. Each downloaded
// file is checked before a root-relative temporary file is atomically renamed.
func Sync(ctx context.Context, doer Doer, source manifest.UISource, root *os.Root) error {
	if root == nil {
		return errors.New("vendor: root is nil")
	}
	if doer == nil {
		return errors.New("vendor: HTTP doer is nil")
	}
	if err := validateSource(source); err != nil {
		return err
	}

	icons := slices.Clone(source.Icons)
	slices.SortFunc(icons, func(a, b manifest.UIIcon) int { return strings.Compare(a.Path, b.Path) })
	temporary := ".sync-" + randomToken()
	if err := root.MkdirAll(temporary, 0o755); err != nil {
		return fmt.Errorf("vendor: create temporary root: %w", err)
	}
	defer func() { _ = root.RemoveAll(temporary) }()

	for _, icon := range icons {
		if existing, err := root.ReadFile(icon.Path); err == nil && checksum(existing) == icon.SHA256 {
			continue
		} else if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("vendor: read %s: %w", icon.Path, err)
		}
		body, err := download(ctx, doer, source, icon)
		if err != nil {
			return err
		}
		if checksum(body) != icon.SHA256 {
			return fmt.Errorf("vendor: checksum mismatch for %s", icon.Path)
		}
		temporaryPath := path.Join(temporary, icon.Path)
		if err := root.MkdirAll(path.Dir(temporaryPath), 0o755); err != nil {
			return fmt.Errorf("vendor: create temporary directory for %s: %w", icon.Path, err)
		}
		if err := root.WriteFile(temporaryPath, body, 0o644); err != nil {
			return fmt.Errorf("vendor: write temporary %s: %w", icon.Path, err)
		}
	}

	for _, icon := range icons {
		temporaryPath := path.Join(temporary, icon.Path)
		if _, err := root.Stat(temporaryPath); errors.Is(err, fs.ErrNotExist) {
			continue
		} else if err != nil {
			return fmt.Errorf("vendor: stat temporary %s: %w", icon.Path, err)
		}
		if err := root.MkdirAll(path.Dir(icon.Path), 0o755); err != nil {
			return fmt.Errorf("vendor: create vendor directory for %s: %w", icon.Path, err)
		}
		if err := root.Rename(temporaryPath, icon.Path); err != nil {
			return fmt.Errorf("vendor: publish %s: %w", icon.Path, err)
		}
	}
	return nil
}

func validateSource(source manifest.UISource) error {
	if source.Name != "heroicons" || source.Alias != "hi" || source.Version != "v2.2.0" || source.Commit != heroiconsCommit || source.BaseURL != heroiconsBaseURL || source.License != "MIT" {
		return errors.New("vendor: source is not immutable Heroicons v2.2.0")
	}
	base, err := url.Parse(source.BaseURL)
	if err != nil || base.Scheme != "https" || base.Host != "raw.githubusercontent.com" || base.Path != "/tailwindlabs/heroicons/"+heroiconsCommit+"/src/" || base.RawQuery != "" || base.Fragment != "" || base.User != nil {
		return errors.New("vendor: source is not immutable Heroicons v2.2.0")
	}
	if len(source.Icons) == 0 {
		return errors.New("vendor: source has no icons")
	}
	seen := make(map[string]struct{}, len(source.Icons))
	for _, icon := range source.Icons {
		if !fs.ValidPath(icon.Path) || !strings.HasPrefix(icon.Path, "16/solid/") || !strings.HasSuffix(icon.Path, ".svg") || !sha256Hex.MatchString(icon.SHA256) {
			return fmt.Errorf("vendor: invalid icon %q", icon.Path)
		}
		if _, ok := seen[icon.Path]; ok {
			return fmt.Errorf("vendor: duplicate icon %q", icon.Path)
		}
		seen[icon.Path] = struct{}{}
	}
	return nil
}

func download(ctx context.Context, doer Doer, source manifest.UISource, icon manifest.UIIcon) ([]byte, error) {
	requestURL := source.BaseURL + icon.Path
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("vendor: construct request for %s: %w", icon.Path, err)
	}
	response, err := doer.Do(request)
	if err != nil {
		return nil, fmt.Errorf("vendor: download %s: %w", icon.Path, err)
	}
	if response == nil || response.Body == nil {
		return nil, fmt.Errorf("vendor: download %s: empty response", icon.Path)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vendor: unexpected HTTP status %d for %s", response.StatusCode, icon.Path)
	}
	if response.Request != nil && response.Request.URL.String() != requestURL {
		return nil, fmt.Errorf("vendor: redirect rejected for %s", icon.Path)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return nil, fmt.Errorf("vendor: read %s: %w", icon.Path, err)
	}
	if len(body) > maxResponseBytes {
		return nil, fmt.Errorf("vendor: response exceeds %d bytes for %s", maxResponseBytes, icon.Path)
	}
	return body, nil
}

func randomToken() string {
	var data [12]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic("crypto/rand unavailable: " + err.Error())
	}
	return hex.EncodeToString(data[:])
}

func checksum(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var sha256Hex = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Result contains deterministic dist-relative UI artifacts and catalog entries.
type Result struct {
	Files  map[string][]byte
	Assets []catalog.Asset
}

// BuildUI reads only tracked vendor inputs and returns normalized UI artifacts.
func BuildUI(fsys fs.FS, ui manifest.UI) (Result, error) {
	if err := ui.Validate(); err != nil {
		return Result{}, fmt.Errorf("build ui: manifest: %w", err)
	}
	source := ui.Sources[0]
	if err := validateSource(source); err != nil {
		return Result{}, err
	}
	if err := verifyVendorSet(fsys, source); err != nil {
		return Result{}, err
	}

	files := make(map[string][]byte, len(source.Icons)+3)
	assets := make([]catalog.Asset, 0, len(source.Icons))
	entries := make([]sprite.Entry, 0, len(source.Icons))
	icons := slices.Clone(source.Icons)
	slices.SortFunc(icons, func(a, b manifest.UIIcon) int { return strings.Compare(a.Path, b.Path) })
	for _, icon := range icons {
		inputPath := path.Join(heroiconsVendor, icon.Path)
		raw, err := fs.ReadFile(fsys, inputPath)
		if err != nil {
			return Result{}, fmt.Errorf("build ui: read %s: %w", inputPath, err)
		}
		if checksum(raw) != icon.SHA256 {
			return Result{}, fmt.Errorf("build ui: checksum mismatch for %s", icon.Path)
		}
		document, err := svgasset.Parse(stripRootDimensions(raw))
		if err != nil {
			return Result{}, fmt.Errorf("build ui: validate %s: %w", icon.Path, err)
		}
		normalized, err := document.Normalize(svgasset.Options{ColorBehavior: "monochrome"})
		if err != nil {
			return Result{}, fmt.Errorf("build ui: normalize %s: %w", icon.Path, err)
		}
		if _, err := svgasset.Parse(normalized); err != nil {
			return Result{}, fmt.Errorf("build ui: validate generated %s: %w", icon.Path, err)
		}
		name := canonicalName(icon.Path)
		outputPath := "icons/ui/heroicons/" + name + ".svg"
		symbol := "hi-" + name
		files[outputPath] = normalized
		entries = append(entries, sprite.Entry{Symbol: symbol, SVG: normalized})
		assets = append(assets, catalog.Asset{
			CanonicalName: "ui-" + symbol,
			Namespace:     "ui", Path: outputPath, Product: "heroicons", Artwork: "icon",
			Appearance: "default", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: document.ViewBox()}, SpriteSymbol: symbol,
			ColorBehavior: "monochrome", License: "MIT", Source: "heroicons@v2.2.0", SHA256: checksum(normalized),
		})
	}
	spriteBytes, err := sprite.Build(entries)
	if err != nil {
		return Result{}, fmt.Errorf("build ui: sprite: %w", err)
	}
	files["icons/ui/sprite.svg"] = spriteBytes
	files["licenses/heroicons-MIT.txt"] = []byte(heroiconsMIT)
	provenanceBytes, err := json.MarshalIndent(provenanceDocument(source), "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("build ui: provenance: %w", err)
	}
	files["icons/ui/heroicons/provenance.json"] = append(provenanceBytes, '\n')
	return Result{Files: files, Assets: assets}, nil
}

func verifyVendorSet(fsys fs.FS, source manifest.UISource) error {
	want := make(map[string]struct{}, len(source.Icons))
	for _, icon := range source.Icons {
		want[icon.Path] = struct{}{}
	}
	seen := make(map[string]struct{}, len(want))
	err := fs.WalkDir(fsys, heroiconsVendor, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(name, ".svg") {
			return nil
		}
		relative := strings.TrimPrefix(name, heroiconsVendor+"/")
		if _, ok := want[relative]; !ok {
			return fmt.Errorf("build ui: extra vendored SVG %s", relative)
		}
		seen[relative] = struct{}{}
		return nil
	})
	if err != nil {
		return fmt.Errorf("build ui: inspect vendor set: %w", err)
	}
	for _, icon := range source.Icons {
		if _, ok := seen[icon.Path]; !ok {
			return fmt.Errorf("build ui: missing vendored SVG %s", icon.Path)
		}
	}
	return nil
}

func canonicalName(sourcePath string) string {
	parts := strings.Split(strings.TrimSuffix(sourcePath, ".svg"), "/")
	return strings.Join(parts, "-")
}

var rootDimension = regexp.MustCompile(`\s(?:width|height)=(?:"[^"]*"|'[^']*')`)

func stripRootDimensions(raw []byte) []byte {
	end := bytes.IndexByte(raw, '>')
	if end < 0 {
		return raw
	}
	result := append([]byte(nil), raw...)
	result = append(rootDimension.ReplaceAll(result[:end], nil), result[end:]...)
	return result
}

type provenance struct {
	Source     string       `json:"source"`
	Alias      string       `json:"alias"`
	Version    string       `json:"version"`
	Commit     string       `json:"commit"`
	BaseURL    string       `json:"baseURL"`
	License    string       `json:"license"`
	LicenseURL string       `json:"licenseURL"`
	Icons      []lockedIcon `json:"icons"`
}

type lockedIcon struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

func provenanceDocument(source manifest.UISource) provenance {
	icons := slices.Clone(source.Icons)
	slices.SortFunc(icons, func(a, b manifest.UIIcon) int { return strings.Compare(a.Path, b.Path) })
	locked := make([]lockedIcon, 0, len(icons))
	for _, icon := range icons {
		locked = append(locked, lockedIcon{Path: icon.Path, SHA256: icon.SHA256})
	}
	return provenance{Source: source.Name, Alias: source.Alias, Version: source.Version, Commit: source.Commit, BaseURL: source.BaseURL, License: source.License, LicenseURL: source.LicenseURL, Icons: locked}
}
