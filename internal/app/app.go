// Package app wires the reproducible asset commands to their bounded inputs.
package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing/fstest"

	"github.com/araihu/assets/internal/build"
	"github.com/araihu/assets/internal/campaigns"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/channels"
	assetexport "github.com/araihu/assets/internal/export"
	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/platform"
	"github.com/araihu/assets/internal/provenance"
	"github.com/araihu/assets/internal/releasemeta"
	"github.com/araihu/assets/internal/themes"
	"github.com/araihu/assets/internal/transform"
)

// Dependencies contains the narrow injectable process boundaries. A nil Doer
// uses the locked network client only for vendor; a nil Rasterizer uses pinned
// rsvg-convert for build and verify.
type Dependencies struct {
	Repo       string
	Doer       provenance.Doer
	Rasterizer platform.Rasterizer
}

// exportAfterEnumerationHook is a test-only cancellation seam.
var exportAfterEnumerationHook func()

// exportAfterOutputRootHook is a test-only cleanup-ownership seam.
var exportAfterOutputRootHook func()

// campaignAfterOutputRootHook is a test-only cleanup-ownership seam.
var campaignAfterOutputRootHook func()

// campaignAfterSnapshotHook is a test-only source-snapshot seam.
var campaignAfterSnapshotHook func()

// campaignAfterCapturedFileOpenHook is a test-only descriptor-race seam.
var campaignAfterCapturedFileOpenHook func(string)

// managedRootAfterChildOpenHook is a test-only managed-root race seam.
var managedRootAfterChildOpenHook func(string)

const (
	assetsPublicRoot      = "https://araihu.com"
	validationCampaignDay = "1970-01-01"
)

// UsageError describes invalid command or flag syntax. Callers map it to exit
// status 2; all other errors are command execution failures.
type UsageError struct{ message string }

func (err *UsageError) Error() string { return err.message }

// IsUsage reports whether err is command usage failure.
func IsUsage(err error) bool {
	var usage *UsageError
	return errors.As(err, &usage)
}

// Run dispatches one asset command. Only vendor can use a network client.
func Run(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if len(args) == 0 {
		return usagef("usage: araihu-assets <vendor|build|verify|proof|export|catalog|themes|campaigns>")
	}

	switch args[0] {
	case "vendor":
		return runVendor(ctx, deps, args[1:], stdout, stderr)
	case "build":
		return runBuild(ctx, deps, args[1:], stdout, stderr)
	case "verify":
		return runVerify(ctx, deps, args[1:], stdout, stderr)
	case "proof":
		return runProof(ctx, deps, args[1:], stdout, stderr)
	case "export":
		return runExport(ctx, deps, args[1:], stdout, stderr)
	case "catalog":
		return runCatalog(ctx, deps, args[1:], stdout, stderr)
	case "themes":
		return runThemes(ctx, deps, args[1:], stdout, stderr)
	case "campaigns":
		return runCampaigns(ctx, deps, args[1:], stdout, stderr)
	default:
		return usagef("unknown command %q; expected vendor, build, verify, proof, export, catalog, themes, or campaigns", args[0])
	}
}

func runThemes(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	if len(args) != 1 || args[0] != "validate" {
		return usagef("usage: araihu-assets themes validate")
	}
	if err := contextError("themes", ctx); err != nil {
		return err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("themes", err)
	}
	defer repoRoot.Close()
	if _, _, err := themeInputs(repoRoot.FS()); err != nil {
		return commandError("themes", err)
	}
	if err := contextError("themes", ctx); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "themes: manifests/themes.yaml and referenced stylesheets are valid")
	return nil
}

func runCampaigns(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return usagef("usage: araihu-assets campaigns <validate|resolve|publish>")
	}
	switch args[0] {
	case "validate":
		if len(args) != 1 {
			return usagef("usage: araihu-assets campaigns validate")
		}
		date, err := campaigns.ParseDate(validationCampaignDay)
		if err != nil {
			return commandError("campaigns", err)
		}
		if _, err := resolveCampaign(ctx, deps, date); err != nil {
			return err
		}
		fmt.Fprintln(stdout, "campaigns: manifests/campaigns.yaml and release references are valid")
		return nil
	case "resolve":
		return runCampaignResolve(ctx, deps, args[1:], stdout, stderr)
	case "publish":
		return runCampaignPublish(ctx, deps, args[1:], stdout, stderr)
	default:
		return usagef("campaigns: unknown command %q; expected validate, resolve, or publish", args[0])
	}
}

func runCampaignResolve(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("campaigns resolve", stderr, "usage: araihu-assets campaigns resolve --date YYYY-MM-DD")
	rawDate := flags.String("date", "", "UTC campaign date in YYYY-MM-DD form")
	if err := parse(flags, args); err != nil {
		return usagef("campaigns resolve: %v", err)
	}
	date, err := requiredCampaignDate("campaigns resolve", *rawDate)
	if err != nil {
		return err
	}
	document, err := resolveCampaign(ctx, deps, date)
	if err != nil {
		return err
	}
	encoded, err := channels.Encode(document)
	if err != nil {
		return fmt.Errorf("campaigns resolve: encode channel document: %w", err)
	}
	if _, err := stdout.Write(encoded); err != nil {
		return fmt.Errorf("campaigns resolve: write stdout: %w", err)
	}
	return nil
}

func runCampaignPublish(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("campaigns publish", stderr, "usage: araihu-assets campaigns publish --date YYYY-MM-DD --output <directory>")
	rawDate := flags.String("date", "", "UTC campaign date in YYYY-MM-DD form")
	output := flags.String("output", "", "consumer-controlled channel output directory")
	if err := parse(flags, args); err != nil {
		return usagef("campaigns publish: %v", err)
	}
	date, err := requiredCampaignDate("campaigns publish", *rawDate)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*output) == "" {
		return usagef("campaigns publish: --output is required")
	}
	current, baseline, latest, runtime, err := campaignDocuments(ctx, deps, date)
	if err != nil {
		return err
	}
	currentJSON, err := channels.Encode(current)
	if err != nil {
		return fmt.Errorf("campaigns publish: encode current channel: %w", err)
	}
	baselineJSON, err := channels.Encode(baseline)
	if err != nil {
		return fmt.Errorf("campaigns publish: encode default channel: %w", err)
	}
	if err := contextError("campaigns publish", ctx); err != nil {
		return err
	}
	published, err := openCampaignOutput(ctx, *output)
	if err != nil {
		return fmt.Errorf("campaigns publish: create output root %q: %w", *output, err)
	}
	cleanupOutput := true
	defer func() {
		if cleanupOutput {
			published.cleanup()
		} else {
			published.close()
		}
	}()
	if campaignAfterOutputRootHook != nil {
		campaignAfterOutputRootHook()
	}
	if err := contextError("campaigns publish", ctx); err != nil {
		return err
	}
	if err := published.verifyPath(); err != nil {
		return fmt.Errorf("campaigns publish: output root %q changed: %w", *output, err)
	}
	files := fstest.MapFS{
		"campaign/v1.js":        {Data: runtime},
		"releases/current.json": {Data: currentJSON},
		"releases/default.json": {Data: baselineJSON},
		"releases/latest.json":  {Data: latest},
	}
	if err := assetexport.CopyContext(ctx, files, []string{"campaign/v1.js", "releases/latest.json", "releases/default.json", "releases/current.json"}, published.root); err != nil {
		return fmt.Errorf("campaigns publish: write channel output %q: %w", *output, err)
	}
	cleanupOutput = false
	fmt.Fprintln(stdout, "campaigns: published latest, default, current, and runtime channels")
	return nil
}

func requiredCampaignDate(command, raw string) (campaigns.Date, error) {
	if strings.TrimSpace(raw) == "" {
		return campaigns.Date{}, usagef("%s: --date is required", command)
	}
	date, err := campaigns.ParseDate(raw)
	if err != nil {
		return campaigns.Date{}, usagef("%s: %v", command, err)
	}
	return date, nil
}

func resolveCampaign(ctx context.Context, deps Dependencies, date campaigns.Date) (channels.Document, error) {
	current, _, _, _, err := campaignDocuments(ctx, deps, date)
	return current, err
}

type campaignReleaseSnapshot struct {
	Catalog catalog.Catalog
	Themes  themes.Catalog
	// Campaigns belongs to immutable release provenance. Resolution uses the
	// strict live source manifest captured in campaignSnapshot instead.
	Campaigns campaigns.Manifest
	Runtime   []byte
}

type campaignSnapshot struct {
	Default         channels.Default
	Campaigns       campaigns.Manifest
	Latest          campaignReleaseSnapshot
	Promoted        campaignReleaseSnapshot
	PublishedLatest []byte
}

func campaignDocuments(ctx context.Context, deps Dependencies, date campaigns.Date) (channels.Document, channels.Document, []byte, []byte, error) {
	snapshot, err := loadCampaignSnapshot(ctx, deps)
	if err != nil {
		return channels.Document{}, channels.Document{}, nil, nil, err
	}
	if campaignAfterSnapshotHook != nil {
		campaignAfterSnapshotHook()
	}
	currentInput := campaignInputFor(snapshot.Default, snapshot.Promoted, snapshot.Campaigns, date)
	current, err := channels.Resolve(currentInput)
	if err != nil {
		return channels.Document{}, channels.Document{}, nil, nil, fmt.Errorf("campaigns: resolve current: %w", err)
	}
	defaultInput := campaignInputFor(snapshot.Default, snapshot.Promoted, campaigns.Manifest{SchemaVersion: 1}, date)
	baseline, err := channels.Resolve(defaultInput)
	if err != nil {
		return channels.Document{}, channels.Document{}, nil, nil, fmt.Errorf("campaigns: resolve default: %w", err)
	}
	latestPromotion := snapshot.Default
	latestPromotion.Release = snapshot.Latest.Catalog.Release
	latestPromotion.Theme = latestTheme(snapshot.Latest.Themes, snapshot.Default.Theme)
	latestJSON := append([]byte(nil), snapshot.PublishedLatest...)
	if len(latestJSON) == 0 {
		latestInput := campaignInputFor(latestPromotion, snapshot.Latest, campaigns.Manifest{SchemaVersion: 1}, date)
		latest, err := channels.Resolve(latestInput)
		if err != nil {
			return channels.Document{}, channels.Document{}, nil, nil, fmt.Errorf("campaigns: resolve latest: %w", err)
		}
		latestJSON, err = channels.Encode(latest)
		if err != nil {
			return channels.Document{}, channels.Document{}, nil, nil, fmt.Errorf("campaigns: encode latest channel: %w", err)
		}
	}
	if err := contextError("campaigns", ctx); err != nil {
		return channels.Document{}, channels.Document{}, nil, nil, err
	}
	return current, baseline, latestJSON, append([]byte(nil), snapshot.Promoted.Runtime...), nil
}

func campaignInputFor(defaultPromotion channels.Default, release campaignReleaseSnapshot, calendar campaigns.Manifest, date campaigns.Date) channels.Input {
	return channels.Input{Date: date, Default: defaultPromotion, Catalog: release.Catalog, Themes: release.Themes, Campaigns: calendar, PublicRoot: assetsPublicRoot}
}

func latestTheme(catalog themes.Catalog, preferred string) string {
	for _, theme := range catalog.Themes {
		if theme.ID == preferred {
			return preferred
		}
	}
	values := slices.Clone(catalog.Themes)
	slices.SortFunc(values, func(a, b themes.CatalogTheme) int { return strings.Compare(a.ID, b.ID) })
	if len(values) == 0 {
		return ""
	}
	return values[0].ID
}

func loadCampaignSnapshot(ctx context.Context, deps Dependencies) (campaignSnapshot, error) {
	if err := contextError("campaigns", ctx); err != nil {
		return campaignSnapshot{}, err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return campaignSnapshot{}, commandError("campaigns", err)
	}
	defer repoRoot.Close()
	defaultPromotion, err := channels.LoadDefault(repoRoot.FS(), "manifests/default.yaml")
	if err != nil {
		return campaignSnapshot{}, fmt.Errorf("campaigns: manifest manifests/default.yaml: %w", err)
	}
	liveCampaigns, err := campaigns.Load(repoRoot.FS(), "manifests/campaigns.yaml")
	if err != nil {
		return campaignSnapshot{}, fmt.Errorf("campaigns: manifest manifests/campaigns.yaml: %w", err)
	}
	latestPath := "dist"
	if _, err := repoRoot.Lstat("releases/latest"); err == nil {
		latestPath = "releases/latest"
	} else if !errors.Is(err, fs.ErrNotExist) {
		return campaignSnapshot{}, fmt.Errorf("campaigns: inspect published latest release snapshot: %w", err)
	}
	latest, err := loadCampaignRelease(ctx, repoRoot, latestPath)
	if err != nil {
		return campaignSnapshot{}, err
	}
	var publishedLatest []byte
	if latestPath != "dist" {
		publishedLatest, err = loadPublishedLatest(ctx, repoRoot, latestPath, latest.Catalog.Release)
		if err != nil {
			return campaignSnapshot{}, err
		}
	}
	promoted := latest
	if defaultPromotion.Release != latest.Catalog.Release {
		promoted, err = loadCampaignRelease(ctx, repoRoot, "releases/"+defaultPromotion.Release)
		if err != nil {
			return campaignSnapshot{}, fmt.Errorf("campaigns: load promoted release snapshot %q: %w", defaultPromotion.Release, err)
		}
	}
	if err := contextError("campaigns", ctx); err != nil {
		return campaignSnapshot{}, err
	}
	return campaignSnapshot{Default: defaultPromotion, Campaigns: liveCampaigns, Latest: latest, Promoted: promoted, PublishedLatest: publishedLatest}, nil
}

func loadCampaignRelease(ctx context.Context, repoRoot *os.Root, path string) (campaignReleaseSnapshot, error) {
	releaseRoot, err := managedRoot(repoRoot, path, false)
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: open release snapshot %q: %w", path, err)
	}
	defer releaseRoot.Close()
	releaseJSON, err := readCapturedRegularFile(ctx, releaseRoot, "release.json")
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: read release snapshot %q release.json: %w", path, err)
	}
	var releaseDocument releasemeta.Document
	if err := decodeOneJSON(releaseJSON, &releaseDocument); err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: decode release snapshot %q release.json: %w", path, err)
	}
	if err := releaseDocument.Validate(); err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: validate release snapshot %q release.json: %w", path, err)
	}
	catalogJSON, err := readCapturedRegularFile(ctx, releaseRoot, "catalog.json")
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: read release snapshot %q catalog.json: %w", path, err)
	}
	assetCatalog, err := catalog.Decode(bytes.NewReader(catalogJSON))
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: validate release snapshot %q catalog.json: %w", path, err)
	}
	themesJSON, err := readCapturedRegularFile(ctx, releaseRoot, "themes.json")
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: read release snapshot %q themes.json: %w", path, err)
	}
	themeCatalog, err := decodeThemeCatalog(themesJSON)
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: decode release snapshot %q themes.json: %w", path, err)
	}
	campaignsJSON, err := readCapturedRegularFile(ctx, releaseRoot, "campaigns.json")
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: read release snapshot %q campaigns.json: %w", path, err)
	}
	campaignManifest, err := decodeCampaignManifest(campaignsJSON)
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: decode release snapshot %q campaigns.json: %w", path, err)
	}
	snapshot := campaignReleaseSnapshot{Catalog: assetCatalog, Themes: themeCatalog, Campaigns: campaignManifest}
	runtime, err := readCapturedRegularFile(ctx, releaseRoot, "campaign/v1.js")
	if err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: read release snapshot %q campaign/v1.js: %w", path, err)
	}
	snapshot.Runtime = append([]byte(nil), runtime...)
	if err := validateCapturedRelease(releaseDocument, snapshot, catalogJSON, themesJSON, campaignsJSON); err != nil {
		return campaignReleaseSnapshot{}, fmt.Errorf("campaigns: validate captured release snapshot %q: %w", path, err)
	}
	return snapshot, nil
}

func loadPublishedLatest(ctx context.Context, repoRoot *os.Root, path, release string) ([]byte, error) {
	root, err := managedRoot(repoRoot, path, false)
	if err != nil {
		return nil, fmt.Errorf("campaigns: open published latest snapshot %q: %w", path, err)
	}
	defer root.Close()
	data, err := readCapturedRegularFile(ctx, root, "latest.json")
	if err != nil {
		return nil, fmt.Errorf("campaigns: read published latest snapshot %q latest.json: %w", path, err)
	}
	var document channels.Document
	if err := decodeOneJSON(data, &document); err != nil {
		return nil, fmt.Errorf("campaigns: decode published latest snapshot %q latest.json: %w", path, err)
	}
	canonical, err := channels.Encode(document)
	if err != nil {
		return nil, fmt.Errorf("campaigns: validate published latest snapshot %q latest.json: %w", path, err)
	}
	if document.Release != release || document.Source != "default" || document.Campaign != nil {
		return nil, fmt.Errorf("campaigns: published latest snapshot %q does not identify release %q baseline", path, release)
	}
	if !bytes.Equal(data, canonical) {
		return nil, fmt.Errorf("campaigns: published latest snapshot %q latest.json is not canonical", path)
	}
	return append([]byte(nil), data...), nil
}

func decodeThemeCatalog(data []byte) (themes.Catalog, error) {
	var catalog themes.Catalog
	if err := decodeOneJSON(data, &catalog); err != nil {
		return themes.Catalog{}, err
	}
	return catalog, nil
}

func decodeCampaignManifest(data []byte) (campaigns.Manifest, error) {
	var manifest campaigns.Manifest
	if err := decodeOneJSON(data, &manifest); err != nil {
		return campaigns.Manifest{}, err
	}
	if err := manifest.Validate(); err != nil {
		return campaigns.Manifest{}, err
	}
	return manifest, nil
}

func decodeOneJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func validateCapturedRelease(document releasemeta.Document, snapshot campaignReleaseSnapshot, catalogJSON, themesJSON, campaignsJSON []byte) error {
	if document.Release != snapshot.Catalog.Release || document.Release != snapshot.Themes.Release {
		return fmt.Errorf("release %q does not match captured contracts", document.Release)
	}
	files := map[string][]byte{
		"catalog.json":   catalogJSON,
		"themes.json":    themesJSON,
		"campaigns.json": campaignsJSON,
	}
	if snapshot.Runtime != nil {
		files["campaign/v1.js"] = snapshot.Runtime
	}
	indexed := make(map[string]releasemeta.File, len(document.Files))
	for _, file := range document.Files {
		indexed[file.Path] = file
	}
	for name, data := range files {
		file, found := indexed[name]
		if !found {
			return fmt.Errorf("release inventory does not contain %q", name)
		}
		sum := sha256.Sum256(data)
		if file.Size != int64(len(data)) || file.SHA256 != fmt.Sprintf("%x", sum) {
			return fmt.Errorf("release inventory mismatch for %q", name)
		}
	}
	return nil
}

// readCapturedRegularFile snapshots one release member through a descriptor.
// Both path observations and the opened descriptor must retain one regular-file
// identity, so an in-root symlink or replacement cannot be mistaken for an
// immutable release input.
func readCapturedRegularFile(ctx context.Context, root *os.Root, name string) ([]byte, error) {
	if root == nil {
		return nil, errors.New("release root is nil")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	pre, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if pre.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("release member %q is a symbolic link", name)
	}
	if !pre.Mode().IsRegular() {
		return nil, fmt.Errorf("release member %q is not a regular file", name)
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if campaignAfterCapturedFileOpenHook != nil {
		campaignAfterCapturedFileOpenHook(name)
	}
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	post, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if post.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("release member %q changed to a symbolic link while opening", name)
	}
	if opened.Mode()&fs.ModeSymlink != 0 || !opened.Mode().IsRegular() || !post.Mode().IsRegular() || !os.SameFile(pre, opened) || !os.SameFile(pre, post) {
		return nil, fmt.Errorf("release member %q changed while opening", name)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	finalFile, err := file.Stat()
	if err != nil {
		return nil, err
	}
	finalPath, err := root.Lstat(name)
	if err != nil {
		return nil, err
	}
	if finalPath.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("release member %q changed to a symbolic link while reading", name)
	}
	if finalFile.Mode()&fs.ModeSymlink != 0 || !finalFile.Mode().IsRegular() || !finalPath.Mode().IsRegular() || !os.SameFile(pre, finalFile) || !os.SameFile(pre, finalPath) {
		return nil, fmt.Errorf("release member %q changed while reading", name)
	}
	return data, nil
}

func runProof(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("proof", stderr, "usage: araihu-assets proof [--check]")
	check := flags.Bool("check", false, "check generated proof drift without writing")
	if err := parse(flags, args); err != nil {
		return usagef("proof: %v", err)
	}
	if err := contextError("proof", ctx); err != nil {
		return err
	}
	repo, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("proof", err)
	}
	defer repoRoot.Close()
	inputs, err := inputs(ctx, repoRoot, deps)
	if err != nil {
		return commandError("proof", err)
	}
	if *check {
		if err := build.CheckContext(ctx, repo, inputs); err != nil {
			return fmt.Errorf("proof: check: %w", err)
		}
		fmt.Fprintln(stdout, "proof: dist/proof matches deterministic offline output")
		return nil
	}
	if err := build.RunContext(ctx, repo, inputs); err != nil {
		return fmt.Errorf("proof: publish: %w", err)
	}
	fmt.Fprintln(stdout, "proof: published deterministic offline output")
	return nil
}

func runVendor(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("vendor", stderr, "usage: araihu-assets vendor")
	if err := parse(flags, args); err != nil {
		return usagef("vendor: %v", err)
	}
	if err := contextError("vendor", ctx); err != nil {
		return err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("vendor", err)
	}
	defer repoRoot.Close()
	ui, err := manifest.LoadUI(repoRoot.FS(), "manifests/icons-ui.yaml")
	if err != nil {
		return fmt.Errorf("vendor: manifest manifests/icons-ui.yaml: %w", err)
	}
	doer := deps.Doer
	if doer == nil {
		doer = provenance.NewHTTPClient()
	}
	for _, source := range ui.Sources {
		if err := contextError("vendor", ctx); err != nil {
			return err
		}
		path := "vendor/icons/ui/" + source.Name + "/" + source.Version
		root, err := managedRoot(repoRoot, path, true)
		if err != nil {
			return fmt.Errorf("vendor: open rooted vendor path %q: %w", path, err)
		}
		err = provenance.Sync(ctx, doer, source, root)
		closeErr := root.Close()
		if err != nil {
			return fmt.Errorf("vendor: source %s@%s: %w", source.Name, source.Version, err)
		}
		if closeErr != nil {
			return fmt.Errorf("vendor: close rooted vendor path %q: %w", path, closeErr)
		}
		fmt.Fprintf(stdout, "vendor: %s@%s synchronized %d locked files\n", source.Name, source.Version, len(source.Icons))
	}
	return nil
}

func runBuild(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("build", stderr, "usage: araihu-assets build [--offline] [--check]")
	offline := flags.Bool("offline", false, "assert offline generation")
	check := flags.Bool("check", false, "check dist for generated drift without writing")
	if err := parse(flags, args); err != nil {
		return usagef("build: %v", err)
	}
	if err := contextError("build", ctx); err != nil {
		return err
	}
	repo, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("build", err)
	}
	defer repoRoot.Close()
	inputs, err := inputs(ctx, repoRoot, deps)
	if err != nil {
		return commandError("build", err)
	}
	if *check {
		if err := build.CheckContext(ctx, repo, inputs); err != nil {
			return fmt.Errorf("build: check: %w", err)
		}
		fmt.Fprintln(stdout, "build: dist matches deterministic offline output")
		return nil
	}
	if err := build.RunContext(ctx, repo, inputs); err != nil {
		return fmt.Errorf("build: publish: %w", err)
	}
	if *offline {
		fmt.Fprintln(stdout, "build: published deterministic offline output")
	} else {
		fmt.Fprintln(stdout, "build: published deterministic output")
	}
	return nil
}

func runVerify(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("verify", stderr, "usage: araihu-assets verify")
	if err := parse(flags, args); err != nil {
		return usagef("verify: %v", err)
	}
	if err := contextError("verify", ctx); err != nil {
		return err
	}
	repo, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("verify", err)
	}
	defer repoRoot.Close()
	inputs, err := inputs(ctx, repoRoot, deps)
	if err != nil {
		return commandError("verify", err)
	}
	if err := build.CheckContext(ctx, repo, inputs); err != nil {
		return fmt.Errorf("verify: reproducibility rule failed: %w", err)
	}
	fmt.Fprintln(stdout, "verify: manifests, pinned inputs, generated artifacts, catalog, and dist are valid")
	return nil
}

func runExport(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("export", stderr, "usage: araihu-assets export --output <directory>")
	output := flags.String("output", "", "consumer-controlled output directory")
	if err := parse(flags, args); err != nil {
		return usagef("export: %v", err)
	}
	if strings.TrimSpace(*output) == "" {
		return usagef("export: --output is required")
	}
	if err := contextError("export", ctx); err != nil {
		return err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("export", err)
	}
	defer repoRoot.Close()
	source, err := managedRoot(repoRoot, "dist", false)
	if err != nil {
		return fmt.Errorf("export: open live dist root: %w", err)
	}
	defer source.Close()
	paths, err := rootedFiles(ctx, source)
	if err != nil {
		return fmt.Errorf("export: inspect live dist: %w", err)
	}
	if exportAfterEnumerationHook != nil {
		exportAfterEnumerationHook()
	}
	if err := contextError("export", ctx); err != nil {
		return err
	}
	owned, err := createOutputRoot(ctx, *output)
	if err != nil {
		removeEmptyOwnedDirectories(owned)
		return fmt.Errorf("export: create output root %q: %w", *output, err)
	}
	if exportAfterOutputRootHook != nil {
		exportAfterOutputRootHook()
	}
	cleanupOwned := true
	defer func() {
		if cleanupOwned {
			removeEmptyOwnedDirectories(owned)
		} else {
			closeOwnedDirectories(owned)
		}
	}()
	if err := contextError("export", ctx); err != nil {
		return err
	}
	destination, err := os.OpenRoot(*output)
	if err != nil {
		return fmt.Errorf("export: open output root %q: %w", *output, err)
	}
	defer destination.Close()
	if err := contextError("export", ctx); err != nil {
		return err
	}
	if err := assetexport.CopyRootContext(ctx, source, paths, destination); err != nil {
		return fmt.Errorf("export: copy live dist into output root %q: %w", *output, err)
	}
	cleanupOwned = false
	fmt.Fprintf(stdout, "export: copied %d release files\n", len(paths))
	return nil
}

func runCatalog(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("catalog", stderr, "usage: araihu-assets catalog")
	if err := parse(flags, args); err != nil {
		return usagef("catalog: %v", err)
	}
	if err := contextError("catalog", ctx); err != nil {
		return err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return commandError("catalog", err)
	}
	defer repoRoot.Close()
	root, err := managedRoot(repoRoot, "dist", false)
	if err != nil {
		return fmt.Errorf("catalog: open live dist root: %w", err)
	}
	defer root.Close()
	data, err := root.ReadFile("catalog.json")
	if err != nil {
		return fmt.Errorf("catalog: read live dist catalog.json: %w", err)
	}
	c, err := catalog.Decode(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("catalog: validate live dist catalog.json: %w", err)
	}
	assets := slices.Clone(c.Assets)
	slices.SortFunc(assets, func(a, b catalog.Asset) int { return strings.Compare(a.CanonicalName, b.CanonicalName) })
	counts := map[string]int{}
	for _, asset := range assets {
		counts[asset.Namespace]++
	}
	fmt.Fprintf(stdout, "catalog: release=%s identityRevision=%d assets=%d brand=%d ui=%d\n", c.Release, c.IdentityRevision, len(assets), counts["brand"], counts["ui"])
	for _, asset := range assets {
		fmt.Fprintln(stdout, asset.CanonicalName)
	}
	return nil
}

func inputs(ctx context.Context, repoRoot *os.Root, deps Dependencies) (build.Inputs, error) {
	if err := ctx.Err(); err != nil {
		return build.Inputs{}, err
	}
	files := repoRoot.FS()
	brandManifest, err := manifest.LoadBrand(files, "manifests/brand.yaml")
	if err != nil {
		return build.Inputs{}, fmt.Errorf("manifest manifests/brand.yaml: %w", err)
	}
	uiManifest, err := manifest.LoadUI(files, "manifests/icons-ui.yaml")
	if err != nil {
		return build.Inputs{}, fmt.Errorf("manifest manifests/icons-ui.yaml: %w", err)
	}
	brand, err := transform.BuildBrand(files, brandManifest)
	if err != nil {
		return build.Inputs{}, fmt.Errorf("brand generator: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return build.Inputs{}, err
	}
	ui, err := provenance.BuildUI(files, uiManifest)
	if err != nil {
		return build.Inputs{}, fmt.Errorf("ui generator: %w", err)
	}
	icons, err := platformIcons(brand)
	if err != nil {
		return build.Inputs{}, err
	}
	rasterizer := deps.Rasterizer
	if rasterizer == nil {
		rasterizer = platform.NewRSVG(nil)
	}
	platformFiles, err := platform.Build(ctx, rasterizer, icons)
	if err != nil {
		return build.Inputs{}, fmt.Errorf("platform generator: %w", err)
	}
	themeManifest, themeCSS, err := themeInputs(files)
	if err != nil {
		return build.Inputs{}, err
	}
	campaignManifest, err := campaigns.Load(files, "manifests/campaigns.yaml")
	if err != nil {
		return build.Inputs{}, fmt.Errorf("manifest manifests/campaigns.yaml: %w", err)
	}
	return build.Inputs{Brand: brand, UI: ui, Platform: platformFiles, Themes: themeManifest, ThemeCSS: themeCSS, Campaigns: campaignManifest}, nil
}

func themeInputs(files fs.FS) (themes.Manifest, map[string][]byte, error) {
	manifest, err := themes.Load(files, "manifests/themes.yaml")
	if err != nil {
		return themes.Manifest{}, nil, fmt.Errorf("manifest manifests/themes.yaml: %w", err)
	}
	css := make(map[string][]byte, len(manifest.Themes))
	for _, theme := range manifest.Themes {
		data, err := fs.ReadFile(files, theme.CSSPath)
		if err != nil {
			return themes.Manifest{}, nil, fmt.Errorf("theme %q stylesheet %s: %w", theme.ID, theme.CSSPath, err)
		}
		css[theme.CSSPath] = append([]byte(nil), data...)
	}
	return manifest, css, nil
}

func platformIcons(brand transform.Result) ([]platform.BrandIcon, error) {
	products := []string{"araihu", "goshtoso", "manja", "paje", "x9"}
	icons := make([]platform.BrandIcon, 0, len(products))
	for _, product := range products {
		get := func(name string) ([]byte, error) {
			path := "dist/icons/brand/" + product + "-icon-" + name + ".svg"
			data, ok := brand.Files[path]
			if !ok {
				return nil, fmt.Errorf("platform inputs: missing generated brand artifact %q", path)
			}
			return data, nil
		}
		light, err := get("light-transparent-optical")
		if err != nil {
			return nil, err
		}
		dark, err := get("dark-transparent-optical")
		if err != nil {
			return nil, err
		}
		tinted, err := get("tinted-transparent-optical")
		if err != nil {
			return nil, err
		}
		grayscale, err := get("grayscale-transparent-optical")
		if err != nil {
			return nil, err
		}
		monochrome, err := get("monochrome-transparent-optical")
		if err != nil {
			return nil, err
		}
		adaptive, err := get("adaptive-transparent-optical")
		if err != nil {
			return nil, err
		}
		launcher, err := get("tinted-plate-launcher")
		if err != nil {
			return nil, err
		}
		icons = append(icons, platform.BrandIcon{Product: product, LightSVG: light, DarkSVG: dark, TintedSVG: tinted, GrayscaleSVG: grayscale, MonochromeSVG: monochrome, AdaptiveSVG: adaptive, LauncherSVG: launcher})
	}
	return icons, nil
}

func rootedFiles(ctx context.Context, root *os.Root) ([]string, error) {
	var paths []string
	err := fs.WalkDir(root.FS(), ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == "." {
			return nil
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symbolic link %q is not a release file", name)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular release file %q", name)
		}
		if !fs.ValidPath(name) {
			return fmt.Errorf("invalid release path %q", name)
		}
		paths = append(paths, name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(paths)
	return paths, nil
}

func repository(deps Dependencies) (string, error) {
	repo := deps.Repo
	if repo == "" {
		var err error
		repo, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("determine repository: %w", err)
		}
	}
	repo, err := filepath.Abs(repo)
	if err != nil {
		return "", fmt.Errorf("resolve repository: %w", err)
	}
	info, err := os.Stat(repo)
	if err != nil {
		return "", fmt.Errorf("inspect repository %q: %w", repo, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository %q is not a directory", repo)
	}
	return repo, nil
}

// openRepository opens the caller-selected repository once. Every managed
// child directory is subsequently opened relative to this trusted root.
func openRepository(deps Dependencies) (string, *os.Root, error) {
	repo, err := repository(deps)
	if err != nil {
		return "", nil, err
	}
	root, err := os.OpenRoot(repo)
	if err != nil {
		return "", nil, fmt.Errorf("open repository root %q: %w", repo, err)
	}
	return repo, root, nil
}

func managedRoot(repoRoot *os.Root, name string, create bool) (*os.Root, error) {
	if repoRoot == nil {
		return nil, errors.New("repository root is nil")
	}
	if !fs.ValidPath(name) || strings.Contains(name, `\`) || strings.Contains(strings.Split(name, "/")[0], ":") {
		return nil, fmt.Errorf("invalid managed path %q", name)
	}
	if err := rejectSymlinkComponents(repoRoot, name); err != nil {
		return nil, err
	}
	if create {
		if err := repoRoot.MkdirAll(name, 0o755); err != nil {
			return nil, fmt.Errorf("create managed path %q: %w", name, err)
		}
		if err := rejectSymlinkComponents(repoRoot, name); err != nil {
			return nil, err
		}
	}
	return openManagedRoot(repoRoot, name)
}

// openManagedRoot opens each managed component from its anchored parent. It
// never reopens a composite path, so replacing an intermediate directory with
// an in-root symlink cannot make the final component attacker-controlled.
func openManagedRoot(repoRoot *os.Root, name string) (_ *os.Root, err error) {
	current := repoRoot
	closeCurrent := false
	defer func() {
		if err != nil && closeCurrent {
			_ = current.Close()
		}
	}()
	for _, component := range strings.Split(name, "/") {
		pre, statErr := current.Lstat(component)
		if statErr != nil {
			return nil, fmt.Errorf("inspect managed path %q: %w", name, statErr)
		}
		if pre.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("managed path %q has symbolic-link component %q", name, component)
		}
		if !pre.IsDir() {
			return nil, fmt.Errorf("managed path %q component %q is not a directory", name, component)
		}
		next, openErr := current.OpenRoot(component)
		if openErr != nil {
			return nil, fmt.Errorf("open managed path %q component %q: %w", name, component, openErr)
		}
		if managedRootAfterChildOpenHook != nil {
			managedRootAfterChildOpenHook(component)
		}
		opened, openedErr := next.Stat(".")
		post, postErr := current.Lstat(component)
		if openedErr != nil || postErr != nil || post.Mode()&fs.ModeSymlink != 0 || !post.IsDir() || !os.SameFile(pre, opened) || !os.SameFile(pre, post) {
			_ = next.Close()
			return nil, fmt.Errorf("managed path %q component %q changed while opening", name, component)
		}
		if closeCurrent {
			_ = current.Close()
		}
		current = next
		closeCurrent = true
	}
	return current, nil
}

func rejectSymlinkComponents(root *os.Root, name string) error {
	current := ""
	for _, component := range strings.Split(name, "/") {
		if current == "" {
			current = component
		} else {
			current += "/" + component
		}
		info, err := root.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("inspect managed path %q: %w", current, err)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return fmt.Errorf("managed path %q has symbolic-link component %q", name, current)
		}
		if !info.IsDir() {
			return fmt.Errorf("managed path %q component %q is not a directory", name, current)
		}
	}
	return nil
}

type ownedDirectory struct {
	path   string
	info   fs.FileInfo
	handle *os.File
}

// campaignOutput is an output root anchored by an os.Root descriptor. The
// lexical path is retained only to detect replacement before publishing; all
// writes use root and therefore cannot be redirected by a later rename.
type campaignOutput struct {
	path  string
	info  fs.FileInfo
	root  *os.Root
	owned []ownedDirectory
}

func openCampaignOutput(ctx context.Context, output string) (_ *campaignOutput, err error) {
	absolute, err := campaignOutputPath(output)
	if err != nil {
		return nil, fmt.Errorf("resolve output root: %w", err)
	}
	volume := filepath.VolumeName(absolute)
	anchor := volume + string(filepath.Separator)
	root, err := os.OpenRoot(anchor)
	if err != nil {
		return nil, fmt.Errorf("open output anchor %q: %w", anchor, err)
	}
	result := &campaignOutput{path: absolute, root: root}
	defer func() {
		if err != nil {
			result.cleanup()
		}
	}()

	tail := strings.TrimPrefix(absolute, volume)
	currentPath := anchor
	for _, component := range strings.Split(tail, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		currentPath = filepath.Join(currentPath, component)
		info, statErr := root.Lstat(component)
		if errors.Is(statErr, fs.ErrNotExist) {
			created := false
			if err := root.Mkdir(component, 0o755); err == nil {
				created = true
			} else if !errors.Is(err, fs.ErrExist) {
				return nil, fmt.Errorf("create output component %q: %w", currentPath, err)
			}
			info, statErr = root.Lstat(component)
			if created && statErr == nil {
				handle, openErr := root.Open(component)
				if openErr != nil {
					return nil, fmt.Errorf("open created output component %q: %w", currentPath, openErr)
				}
				ownedInfo, infoErr := handle.Stat()
				if infoErr != nil {
					_ = handle.Close()
					return nil, fmt.Errorf("inspect created output component %q: %w", currentPath, infoErr)
				}
				result.owned = append(result.owned, ownedDirectory{path: currentPath, info: ownedInfo, handle: handle})
			}
		}
		if statErr != nil {
			return nil, fmt.Errorf("inspect output component %q: %w", currentPath, statErr)
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("output path %q has symbolic-link component %q", absolute, currentPath)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("output path %q component %q is not a directory", absolute, currentPath)
		}
		next, openErr := root.OpenRoot(component)
		if openErr != nil {
			return nil, fmt.Errorf("open output component %q: %w", currentPath, openErr)
		}
		openedInfo, infoErr := next.Stat(".")
		currentInfo, currentErr := root.Lstat(component)
		if infoErr != nil || currentErr != nil || currentInfo.Mode()&fs.ModeSymlink != 0 || !os.SameFile(info, currentInfo) || !os.SameFile(info, openedInfo) {
			_ = next.Close()
			return nil, fmt.Errorf("output component %q changed while opening", currentPath)
		}
		_ = root.Close()
		root = next
		result.root = root
		result.info = info
	}
	if result.info == nil {
		return nil, errors.New("output root is empty")
	}
	return result, nil
}

// campaignOutputPath canonicalizes only the process temp-directory prefix.
// macOS exposes that trusted prefix through /var even though its real path is
// /private/var; custom output components remain lexical and are rejected if
// they are symlinks.
func campaignOutputPath(output string) (string, error) {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	temporary := os.TempDir()
	relative, err := filepath.Rel(temporary, absolute)
	if err != nil || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || relative == ".." {
		return absolute, nil
	}
	resolved, err := filepath.EvalSymlinks(temporary)
	if err != nil {
		return "", err
	}
	return filepath.Join(resolved, relative), nil
}

func (output *campaignOutput) verifyPath() error {
	current, err := os.Lstat(output.path)
	if err != nil {
		return err
	}
	if current.Mode()&fs.ModeSymlink != 0 || !current.IsDir() || !os.SameFile(current, output.info) {
		return errors.New("output root replacement detected")
	}
	return nil
}

func (output *campaignOutput) cleanup() {
	if output == nil {
		return
	}
	if output.root != nil {
		_ = output.root.Close()
	}
	removeEmptyOwnedDirectories(output.owned)
}

func (output *campaignOutput) close() {
	if output == nil {
		return
	}
	if output.root != nil {
		_ = output.root.Close()
	}
	closeOwnedDirectories(output.owned)
}

// createOutputRoot creates path one component at a time so cancellation can
// remove only directories created by this invocation. A successful Mkdir is an
// indivisible boundary; cleanup later requires its observed file identity.
func createOutputRoot(ctx context.Context, output string) ([]ownedDirectory, error) {
	absolute, err := filepath.Abs(output)
	if err != nil {
		return nil, fmt.Errorf("resolve output root: %w", err)
	}
	volume := filepath.VolumeName(absolute)
	current := volume + string(filepath.Separator)
	tail := strings.TrimPrefix(absolute, volume)
	var owned []ownedDirectory
	for _, component := range strings.Split(tail, string(filepath.Separator)) {
		if component == "" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return owned, err
		}
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0o755); err == nil {
			handle, err := os.Open(current)
			if err != nil {
				return owned, err
			}
			info, err := handle.Stat()
			if err != nil {
				_ = handle.Close()
				return owned, err
			}
			if !info.IsDir() {
				_ = handle.Close()
				return owned, fmt.Errorf("created output component %q is not a directory", current)
			}
			owned = append(owned, ownedDirectory{path: current, info: info, handle: handle})
		} else if !errors.Is(err, fs.ErrExist) {
			return owned, err
		}
		if err := ctx.Err(); err != nil {
			return owned, err
		}
	}
	return owned, nil
}

// removeEmptyOwnedDirectories removes only an empty directory whose identity
// still matches the post-Mkdir observation. A replacement before Lstat is left
// intact. Standard path APIs cannot atomically bind Lstat and Remove, so a
// replacement after the identity check is an external race beyond this cleanup
// boundary; nonempty replacements still make Remove fail safely.
func removeEmptyOwnedDirectories(owned []ownedDirectory) {
	for index := len(owned) - 1; index >= 0; index-- {
		current, err := os.Lstat(owned[index].path)
		matches := err == nil && current.IsDir() && os.SameFile(current, owned[index].info)
		if owned[index].handle != nil {
			_ = owned[index].handle.Close()
		}
		if !matches {
			continue
		}
		_ = os.Remove(owned[index].path)
	}
}

func closeOwnedDirectories(owned []ownedDirectory) {
	for _, directory := range owned {
		if directory.handle != nil {
			_ = directory.handle.Close()
		}
	}
}

func newFlagSet(name string, stderr io.Writer, usage string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.Usage = func() { fmt.Fprintln(stderr, usage) }
	return flags
}

func parse(flags *flag.FlagSet, args []string) error {
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return nil
}

func contextError(command string, ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	return nil
}

func commandError(command string, err error) error { return fmt.Errorf("%s: %w", command, err) }
func usagef(format string, args ...any) error {
	return &UsageError{message: fmt.Sprintf(format, args...)}
}
