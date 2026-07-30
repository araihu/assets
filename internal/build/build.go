// Package build assembles the complete managed dist tree without exposing
// partially generated output to consumers.
package build

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/araihu/assets/internal/campaigns"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/platform"
	"github.com/araihu/assets/internal/proof"
	"github.com/araihu/assets/internal/provenance"
	"github.com/araihu/assets/internal/release"
	"github.com/araihu/assets/internal/releasemeta"
	"github.com/araihu/assets/internal/sprite"
	"github.com/araihu/assets/internal/svgasset"
	"github.com/araihu/assets/internal/themes"
	"github.com/araihu/assets/internal/transform"
)

const (
	releaseVersion = "v0.1.0"
	// ManagedDistContract makes ownership explicit: a successful Run replaces
	// every path under dist, including unrelated files from a prior release.
	ManagedDistContract = "dist is wholly managed by build.Run"
)

var generatedPaths = map[string]struct{}{
	"catalog.json":      {},
	"campaign/v1.js":    {},
	"campaigns.json":    {},
	"checksums.txt":     {},
	"NOTICE":            {},
	"proof/index.html":  {},
	"proof/styles.css":  {},
	"proof/app.js":      {},
	"release.json":      {},
	"themes.json":       {},
	"themes/araihu.css": {},
}

// Inputs are fully generated, offline artifacts from approved generators.
// Brand and Platform paths include a dist/ prefix; UI paths are dist-relative.
type Inputs struct {
	Brand     transform.Result
	UI        provenance.Result
	Platform  platform.Result
	Themes    themes.Manifest
	Campaigns campaigns.Manifest
	// ThemeCSS holds stylesheet bytes captured by the caller before build.
	// Build never rereads source paths while staging or publishing a release.
	ThemeCSS map[string][]byte
}

type buildPhase uint8

const (
	beforeStage buildPhase = iota
	beforePublish
)

// buildHook is a test-only cancellation seam. Production leaves it nil.
var buildHook func(buildPhase)

// Run stages, validates, archives, and publishes the managed dist tree.
func Run(repo string, input Inputs) error {
	return RunContext(context.Background(), repo, input)
}

// RunContext stages and publishes only while ctx remains active. Once the
// final rename publishes dist, that atomic operation is indivisible.
func RunContext(ctx context.Context, repo string, input Inputs) error {
	stage, err := stage(ctx, repo, input)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	if err := checkpoint(ctx, beforePublish); err != nil {
		return err
	}
	if err := publishContext(ctx, repo, stage); err != nil {
		return err
	}
	return nil
}

// Check regenerates into a sibling temporary tree and requires exact path and
// byte equality with the already published managed dist tree.
func Check(repo string, input Inputs) error {
	return CheckContext(context.Background(), repo, input)
}

// CheckContext regenerates into a temporary sibling tree without publishing.
func CheckContext(ctx context.Context, repo string, input Inputs) error {
	stage, err := stage(ctx, repo, input)
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	actual := filepath.Join(repo, "dist")
	if err := compareTrees(ctx, stage, actual); err != nil {
		return fmt.Errorf("build: dist differs from generated output: %w", err)
	}
	return nil
}

func stage(ctx context.Context, repo string, input Inputs) (string, error) {
	if err := checkpoint(ctx, beforeStage); err != nil {
		return "", err
	}
	if strings.TrimSpace(repo) == "" {
		return "", errors.New("build: repository path is empty")
	}
	parent := filepath.Dir(filepath.Join(repo, "dist"))
	stage, err := os.MkdirTemp(parent, ".dist-stage-")
	if err != nil {
		return "", fmt.Errorf("build: create sibling stage: %w", err)
	}
	failed := true
	defer func() {
		if failed {
			_ = os.RemoveAll(stage)
		}
	}()
	files, err := assembledFiles(ctx, repo, input)
	if err != nil {
		return "", err
	}
	if err := writeFiles(ctx, stage, files); err != nil {
		return "", err
	}
	if err := validateGeneratedSVGs(ctx, stage); err != nil {
		return "", err
	}
	if err := validateCatalog(ctx, stage); err != nil {
		return "", err
	}
	if err := writeReleaseInventory(ctx, stage); err != nil {
		return "", err
	}
	if err := writeChecksums(ctx, stage); err != nil {
		return "", err
	}
	if err := writeArchives(ctx, stage); err != nil {
		return "", err
	}
	failed = false
	return stage, nil
}

func validateGeneratedSVGs(ctx context.Context, root string) error {
	paths, err := filesUnder(ctx, root)
	if err != nil {
		return err
	}
	for _, name := range paths {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if !strings.HasSuffix(name, ".svg") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("build: read generated SVG %s: %w", name, err)
		}
		if strings.HasSuffix(name, "/sprite.svg") {
			if err := sprite.Validate(data); err != nil {
				return fmt.Errorf("build: validate generated sprite %s: %w", name, err)
			}
			continue
		}
		if _, err := svgasset.ParseGenerated(data); err != nil {
			return fmt.Errorf("build: validate generated SVG %s: %w", name, err)
		}
	}
	return nil
}

func assembledFiles(ctx context.Context, repo string, input Inputs) (map[string][]byte, error) {
	themesPresent := input.Themes.SchemaVersion != 0 || input.Themes.TokenContract != "" || len(input.Themes.Themes) != 0 || len(input.ThemeCSS) != 0
	themePaths := map[string]struct{}{}
	if themesPresent {
		if err := input.Themes.Validate(); err != nil {
			return nil, fmt.Errorf("build: validate themes manifest: %w", err)
		}
		for _, theme := range input.Themes.Themes {
			themePaths[theme.CSSPath] = struct{}{}
		}
	}
	if err := input.Campaigns.Validate(); err != nil {
		return nil, fmt.Errorf("build: validate campaigns manifest: %w", err)
	}
	runtimeBytes, err := os.ReadFile(filepath.Join(repo, "runtime", "campaign", "v1.js"))
	if err != nil {
		return nil, fmt.Errorf("build: capture campaign runtime: %w", err)
	}
	files := make(map[string][]byte, len(input.Brand.Files)+len(input.UI.Files)+len(input.Platform.Files)+len(themePaths)+7)
	files["campaign/v1.js"] = append([]byte(nil), runtimeBytes...)
	for _, group := range []map[string][]byte{input.Brand.Files, input.UI.Files, input.Platform.Files} {
		for sourceName, data := range group {
			if err := checkContext(ctx); err != nil {
				return nil, err
			}
			name, err := normalizeInputPath(sourceName)
			if err != nil {
				return nil, err
			}
			_, themePath := themePaths[name]
			if _, reserved := generatedPaths[name]; reserved || themePath || strings.HasPrefix(name, "releases/") {
				return nil, fmt.Errorf("build: input attempts to own generated path %q", name)
			}
			if _, exists := files[name]; exists {
				return nil, fmt.Errorf("build: duplicate generated path %q", name)
			}
			files[name] = append([]byte(nil), data...)
		}
	}
	license, err := os.ReadFile(filepath.Join(repo, "LICENSE"))
	if err != nil {
		return nil, fmt.Errorf("build: read LICENSE: %w", err)
	}
	files["licenses/Apache-2.0.txt"] = license
	files["NOTICE"] = []byte("Arai Hu Assets " + releaseVersion + "\n\nArai Hu brand assets are subject to Arai Hu Brand Terms.\nHeroicons material is included under its MIT license in licenses/heroicons-MIT.txt.\n")
	proofFiles, err := proofStaticFiles(repo)
	if err != nil {
		return nil, err
	}
	for name, data := range proofFiles {
		files[name] = data
	}
	assets := make([]catalog.Asset, 0, len(input.Brand.Assets)+len(input.UI.Assets)+len(input.Platform.Assets))
	assets = append(assets, input.Brand.Assets...)
	assets = append(assets, input.UI.Assets...)
	assets = append(assets, input.Platform.Assets...)
	catalogBytes, err := catalogBytes(assets, files)
	if err != nil {
		return nil, err
	}
	files["catalog.json"] = catalogBytes
	if themesPresent {
		themeFiles, themeCatalog, err := assembleThemes(input.Themes, input.ThemeCSS)
		if err != nil {
			return nil, err
		}
		for name, data := range themeFiles {
			files[name] = data
		}
		files["themes.json"] = themeCatalog
	} else {
		return nil, errors.New("build: themes input is required")
	}
	campaignBytes, err := encodeCampaigns(input.Campaigns)
	if err != nil {
		return nil, err
	}
	files["campaigns.json"] = campaignBytes
	proofPage, err := proofDocument(repo, files)
	if err != nil {
		return nil, err
	}
	if proofPage != nil {
		proofSources := make([]string, 0, len(files))
		for name := range files {
			if strings.HasPrefix(name, "proof/") || strings.HasPrefix(name, "releases/") || name == "catalog.json" || name == "checksums.txt" {
				continue
			}
			proofSources = append(proofSources, name)
		}
		slices.Sort(proofSources)
		for _, name := range proofSources {
			files["proof/assets/"+name] = append([]byte(nil), files[name]...)
		}
		files["proof/index.html"] = proofPage
	}
	return files, nil
}

func encodeCampaigns(manifest campaigns.Manifest) ([]byte, error) {
	if err := manifest.Validate(); err != nil {
		return nil, fmt.Errorf("build: validate campaigns manifest: %w", err)
	}
	canonical := manifest
	canonical.Campaigns = slices.Clone(manifest.Campaigns)
	slices.SortFunc(canonical.Campaigns, func(a, b campaigns.Campaign) int { return strings.Compare(a.ID, b.ID) })
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(canonical); err != nil {
		return nil, fmt.Errorf("build: encode campaigns: %w", err)
	}
	return output.Bytes(), nil
}

func assembleThemes(manifest themes.Manifest, source map[string][]byte) (map[string][]byte, []byte, error) {
	if len(source) != len(manifest.Themes) {
		return nil, nil, errors.New("build: theme CSS inputs do not match manifest")
	}
	published := manifest
	published.Themes = slices.Clone(manifest.Themes)
	files := make(map[string][]byte, len(manifest.Themes))
	for i, theme := range published.Themes {
		data, ok := source[theme.CSSPath]
		if !ok {
			return nil, nil, fmt.Errorf("build: missing captured CSS for theme %q", theme.ID)
		}
		copy := append([]byte(nil), data...)
		sum := sha256.Sum256(copy)
		theme.TokenContract = published.TokenContract
		theme.SHA256 = hex.EncodeToString(sum[:])
		published.Themes[i] = theme
		files[theme.CSSPath] = copy
	}
	catalog, err := published.Catalog(releaseVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("build: create themes catalog: %w", err)
	}
	encoded, err := themes.Encode(catalog)
	if err != nil {
		return nil, nil, fmt.Errorf("build: encode themes catalog: %w", err)
	}
	return files, encoded, nil
}

func proofDocument(repo string, files map[string][]byte) ([]byte, error) {
	scenarios, err := os.ReadFile(filepath.Join(repo, "site", "proof", "scenarios.json"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("build: read proof scenarios: %w", err)
	}
	decoded, err := catalog.Decode(bytes.NewReader(files["catalog.json"]))
	if err != nil {
		return nil, fmt.Errorf("build: decode proof catalog: %w", err)
	}
	model, err := proof.Load(decoded, bytes.NewReader(scenarios))
	if err != nil {
		return nil, fmt.Errorf("build: load proof model: %w", err)
	}
	distribution := make(fstest.MapFS, len(files))
	for name, data := range files {
		distribution[name] = &fstest.MapFile{Data: data}
	}
	var rendered bytes.Buffer
	if err := proof.BuildTemplate(model, distribution, filepath.Join(repo, "site", "proof", "index.tmpl"), &rendered); err != nil {
		return nil, fmt.Errorf("build: render proof document: %w", err)
	}
	return rendered.Bytes(), nil
}

func proofStaticFiles(repo string) (map[string][]byte, error) {
	files := make(map[string][]byte, 2)
	for _, name := range []string{"styles.css", "app.js"} {
		data, err := os.ReadFile(filepath.Join(repo, "site", "proof", name))
		if err != nil {
			return nil, fmt.Errorf("build: read proof static asset %s: %w", name, err)
		}
		files["proof/"+name] = data
	}
	return files, nil
}

func normalizeInputPath(name string) (string, error) {
	name = strings.TrimPrefix(name, "dist/")
	if name == "." || !fs.ValidPath(name) || strings.Contains(name, `\`) || strings.Contains(strings.Split(name, "/")[0], ":") {
		return "", fmt.Errorf("build: invalid dist-relative path %q", name)
	}
	return name, nil
}

func catalogBytes(assets []catalog.Asset, files map[string][]byte) ([]byte, error) {
	c := catalog.Catalog{SchemaVersion: catalog.SchemaVersion, Release: releaseVersion, IdentityRevision: 11, Assets: slices.Clone(assets)}
	for _, asset := range c.Assets {
		data, ok := files[asset.Path]
		if !ok {
			return nil, fmt.Errorf("build: catalog path %q is not published", asset.Path)
		}
		sum := sha256.Sum256(data)
		if asset.SHA256 != hex.EncodeToString(sum[:]) {
			return nil, fmt.Errorf("build: catalog hash mismatch for %s", asset.Path)
		}
	}
	var output bytes.Buffer
	if err := catalog.Encode(&output, c); err != nil {
		return nil, fmt.Errorf("build: validate catalog: %w", err)
	}
	return output.Bytes(), nil
}

func writeFiles(ctx context.Context, root string, files map[string][]byte) error {
	paths := make([]string, 0, len(files))
	for name := range files {
		paths = append(paths, name)
	}
	slices.Sort(paths)
	for _, name := range paths {
		if err := checkContext(ctx); err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("build: create directory for %s: %w", name, err)
		}
		if err := os.WriteFile(target, files[name], 0o644); err != nil {
			return fmt.Errorf("build: write %s: %w", name, err)
		}
	}
	return nil
}

func validateCatalog(ctx context.Context, root string) error {
	data, err := os.ReadFile(filepath.Join(root, "catalog.json"))
	if err != nil {
		return fmt.Errorf("build: read catalog: %w", err)
	}
	c, err := catalog.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("build: decode catalog: %w", err)
	}
	for _, asset := range c.Assets {
		if err := checkContext(ctx); err != nil {
			return err
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(asset.Path)))
		if err != nil {
			return fmt.Errorf("build: read catalog artifact %s: %w", asset.Path, err)
		}
		sum := sha256.Sum256(data)
		if asset.SHA256 != hex.EncodeToString(sum[:]) {
			return fmt.Errorf("build: catalog hash mismatch for %s", asset.Path)
		}
	}
	return nil
}

func writeChecksums(ctx context.Context, root string) error {
	paths, err := filesUnder(ctx, root)
	if err != nil {
		return err
	}
	var output strings.Builder
	for _, name := range paths {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if name == "checksums.txt" || strings.HasPrefix(name, "releases/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return fmt.Errorf("build: checksum %s: %w", name, err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&output, "%x  %s\n", sum, name)
	}
	if err := os.WriteFile(filepath.Join(root, "checksums.txt"), []byte(output.String()), 0o644); err != nil {
		return fmt.Errorf("build: write checksums: %w", err)
	}
	return nil
}

func writeReleaseInventory(ctx context.Context, root string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	document, err := releasemeta.Build(releasemeta.Input{
		Release:          releaseVersion,
		IdentityRevision: 11,
		RuntimeVersion:   1,
		Files:            os.DirFS(root),
	})
	if err != nil {
		return fmt.Errorf("build: release inventory: %w", err)
	}
	encoded, err := releasemeta.Encode(document)
	if err != nil {
		return fmt.Errorf("build: encode release inventory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(root, "release.json"), encoded, 0o644); err != nil {
		return fmt.Errorf("build: write release inventory: %w", err)
	}
	return nil
}

func writeArchives(ctx context.Context, root string) error {
	paths, err := filesUnder(ctx, root)
	if err != nil {
		return err
	}
	paths = slices.DeleteFunc(paths, func(name string) bool { return strings.HasPrefix(name, "releases/") })
	if err := os.MkdirAll(filepath.Join(root, "releases"), 0o755); err != nil {
		return fmt.Errorf("build: create releases: %w", err)
	}
	stageRoot, err := os.OpenRoot(root)
	if err != nil {
		return fmt.Errorf("build: open staged root: %w", err)
	}
	defer stageRoot.Close()
	if err := checkContext(ctx); err != nil {
		return err
	}
	zipFile, err := os.Create(filepath.Join(root, "releases", "araihu-assets-v0.1.0.zip"))
	if err != nil {
		return fmt.Errorf("build: create ZIP: %w", err)
	}
	if err := release.ZIPRoot(zipFile, stageRoot, paths); err != nil {
		_ = zipFile.Close()
		return fmt.Errorf("build: create ZIP: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		return fmt.Errorf("build: close ZIP: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	tarFile, err := os.Create(filepath.Join(root, "releases", "araihu-assets-v0.1.0.tar.gz"))
	if err != nil {
		return fmt.Errorf("build: create tar.gz: %w", err)
	}
	if err := release.ArchiveRoot(tarFile, stageRoot, paths); err != nil {
		_ = tarFile.Close()
		return fmt.Errorf("build: create tar.gz: %w", err)
	}
	if err := tarFile.Close(); err != nil {
		return fmt.Errorf("build: close tar.gz: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		return err
	}
	return nil
}

func filesUnder(ctx context.Context, root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := checkContext(ctx); err != nil {
			return err
		}
		if name == root {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("build: symbolic link %s", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("build: non-regular file %s", name)
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func publishContext(ctx context.Context, repo, stage string) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	dist := filepath.Join(repo, "dist")
	info, err := os.Lstat(dist)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("build: inspect dist: %w", err)
	}
	if err == nil && (!info.IsDir() || info.Mode()&fs.ModeSymlink != 0) {
		return errors.New("build: dist must be a real directory")
	}
	if errors.Is(err, fs.ErrNotExist) {
		if err := checkContext(ctx); err != nil {
			return err
		}
		if err := os.Rename(stage, dist); err != nil {
			return fmt.Errorf("build: publish dist: %w", err)
		}
		return nil
	}
	backup := stage + ".previous"
	if err := checkContext(ctx); err != nil {
		return err
	}
	if err := os.Rename(dist, backup); err != nil {
		return fmt.Errorf("build: preserve prior dist: %w", err)
	}
	if err := checkContext(ctx); err != nil {
		if restoreErr := os.Rename(backup, dist); restoreErr != nil {
			return fmt.Errorf("build: context canceled before publication; restore prior dist: %v", restoreErr)
		}
		return err
	}
	if err := os.Rename(stage, dist); err != nil {
		if restoreErr := os.Rename(backup, dist); restoreErr != nil {
			return fmt.Errorf("build: publish dist: %w; restore prior dist: %v", err, restoreErr)
		}
		return fmt.Errorf("build: publish dist: %w", err)
	}
	// New dist is already published. A stale sibling backup never changes the
	// managed tree and must not turn a completed publication into a failure.
	_ = os.RemoveAll(backup)
	return nil
}

func compareTrees(ctx context.Context, wantRoot, gotRoot string) error {
	want, err := filesUnder(ctx, wantRoot)
	if err != nil {
		return fmt.Errorf("inspect generated tree: %w", err)
	}
	got, err := filesUnder(ctx, gotRoot)
	if err != nil {
		return fmt.Errorf("inspect published tree: %w", err)
	}
	if !slices.Equal(want, got) {
		return fmt.Errorf("paths = %q, want %q", got, want)
	}
	for _, name := range want {
		if err := checkContext(ctx); err != nil {
			return err
		}
		expected, err := os.ReadFile(filepath.Join(wantRoot, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		actual, err := os.ReadFile(filepath.Join(gotRoot, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		if !bytes.Equal(expected, actual) {
			return fmt.Errorf("bytes differ for %s", name)
		}
	}
	return nil
}

func checkpoint(ctx context.Context, phase buildPhase) error {
	if buildHook != nil {
		buildHook(phase)
	}
	return checkContext(ctx)
}

func checkContext(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("build: %w", err)
	}
	return nil
}
