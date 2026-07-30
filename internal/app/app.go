// Package app wires the reproducible asset commands to their bounded inputs.
package app

import (
	"bytes"
	"context"
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
	current, err := resolveCampaign(ctx, deps, date)
	if err != nil {
		return err
	}
	baseline, err := resolveDefaultCampaign(ctx, deps, date)
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
	runtime, err := campaignRuntime(ctx, deps)
	if err != nil {
		return err
	}
	if err := contextError("campaigns publish", ctx); err != nil {
		return err
	}
	owned, err := createOutputRoot(ctx, *output)
	if err != nil {
		removeEmptyOwnedDirectories(owned)
		return fmt.Errorf("campaigns publish: create output root %q: %w", *output, err)
	}
	if campaignAfterOutputRootHook != nil {
		campaignAfterOutputRootHook()
	}
	cleanupOwned := true
	defer func() {
		if cleanupOwned {
			removeEmptyOwnedDirectories(owned)
		} else {
			closeOwnedDirectories(owned)
		}
	}()
	if err := contextError("campaigns publish", ctx); err != nil {
		return err
	}
	destination, err := os.OpenRoot(*output)
	if err != nil {
		return fmt.Errorf("campaigns publish: open output root %q: %w", *output, err)
	}
	defer destination.Close()
	files := fstest.MapFS{
		"campaign/v1.js":        {Data: runtime},
		"releases/current.json": {Data: currentJSON},
		"releases/default.json": {Data: baselineJSON},
		"releases/latest.json":  {Data: baselineJSON},
	}
	if err := assetexport.CopyContext(ctx, files, []string{"campaign/v1.js", "releases/latest.json", "releases/default.json", "releases/current.json"}, destination); err != nil {
		return fmt.Errorf("campaigns publish: write channel output %q: %w", *output, err)
	}
	cleanupOwned = false
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
	input, err := campaignInput(ctx, deps, date)
	if err != nil {
		return channels.Document{}, err
	}
	document, err := channels.Resolve(input)
	if err != nil {
		return channels.Document{}, fmt.Errorf("campaigns: resolve: %w", err)
	}
	if err := contextError("campaigns", ctx); err != nil {
		return channels.Document{}, err
	}
	return document, nil
}

func resolveDefaultCampaign(ctx context.Context, deps Dependencies, date campaigns.Date) (channels.Document, error) {
	input, err := campaignInput(ctx, deps, date)
	if err != nil {
		return channels.Document{}, err
	}
	input.Campaigns.Campaigns = nil
	document, err := channels.Resolve(input)
	if err != nil {
		return channels.Document{}, fmt.Errorf("campaigns: resolve default: %w", err)
	}
	if err := contextError("campaigns", ctx); err != nil {
		return channels.Document{}, err
	}
	return document, nil
}

func campaignInput(ctx context.Context, deps Dependencies, date campaigns.Date) (channels.Input, error) {
	if err := contextError("campaigns", ctx); err != nil {
		return channels.Input{}, err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return channels.Input{}, commandError("campaigns", err)
	}
	defer repoRoot.Close()
	defaultPromotion, err := channels.LoadDefault(repoRoot.FS(), "manifests/default.yaml")
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: manifest manifests/default.yaml: %w", err)
	}
	campaignManifest, err := campaigns.Load(repoRoot.FS(), "manifests/campaigns.yaml")
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: manifest manifests/campaigns.yaml: %w", err)
	}
	dist, err := managedRoot(repoRoot, "dist", false)
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: open live dist root: %w", err)
	}
	defer dist.Close()
	catalogJSON, err := dist.ReadFile("catalog.json")
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: read live dist catalog.json: %w", err)
	}
	assetCatalog, err := catalog.Decode(bytes.NewReader(catalogJSON))
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: validate live dist catalog.json: %w", err)
	}
	themesJSON, err := dist.ReadFile("themes.json")
	if err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: read live dist themes.json: %w", err)
	}
	var themeCatalog themes.Catalog
	decoder := json.NewDecoder(bytes.NewReader(themesJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&themeCatalog); err != nil {
		return channels.Input{}, fmt.Errorf("campaigns: decode live dist themes.json: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return channels.Input{}, errors.New("campaigns: decode live dist themes.json: multiple JSON values")
		}
		return channels.Input{}, fmt.Errorf("campaigns: decode live dist themes.json: %w", err)
	}
	if err := contextError("campaigns", ctx); err != nil {
		return channels.Input{}, err
	}
	return channels.Input{Date: date, Default: defaultPromotion, Catalog: assetCatalog, Themes: themeCatalog, Campaigns: campaignManifest, PublicRoot: assetsPublicRoot}, nil
}

func campaignRuntime(ctx context.Context, deps Dependencies) ([]byte, error) {
	if err := contextError("campaigns", ctx); err != nil {
		return nil, err
	}
	_, repoRoot, err := openRepository(deps)
	if err != nil {
		return nil, commandError("campaigns", err)
	}
	defer repoRoot.Close()
	dist, err := managedRoot(repoRoot, "dist", false)
	if err != nil {
		return nil, fmt.Errorf("campaigns: open live dist root: %w", err)
	}
	defer dist.Close()
	info, err := dist.Lstat("campaign/v1.js")
	if err != nil {
		return nil, fmt.Errorf("campaigns: inspect live dist campaign/v1.js: %w", err)
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, errors.New("campaigns: live dist campaign/v1.js is not a regular file")
	}
	runtime, err := dist.ReadFile("campaign/v1.js")
	if err != nil {
		return nil, fmt.Errorf("campaigns: read live dist campaign/v1.js: %w", err)
	}
	if err := contextError("campaigns", ctx); err != nil {
		return nil, err
	}
	return runtime, nil
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
	root, err := repoRoot.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("open managed path %q: %w", name, err)
	}
	return root, nil
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
