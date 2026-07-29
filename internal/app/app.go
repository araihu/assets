// Package app wires the reproducible asset commands to their bounded inputs.
package app

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/araihu/assets/internal/build"
	"github.com/araihu/assets/internal/catalog"
	assetexport "github.com/araihu/assets/internal/export"
	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/platform"
	"github.com/araihu/assets/internal/provenance"
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
		return usagef("usage: araihu-assets <vendor|build|verify|export|catalog>")
	}

	switch args[0] {
	case "vendor":
		return runVendor(ctx, deps, args[1:], stdout, stderr)
	case "build":
		return runBuild(ctx, deps, args[1:], stdout, stderr)
	case "verify":
		return runVerify(ctx, deps, args[1:], stdout, stderr)
	case "export":
		return runExport(ctx, deps, args[1:], stdout, stderr)
	case "catalog":
		return runCatalog(ctx, deps, args[1:], stdout, stderr)
	default:
		return usagef("unknown command %q; expected vendor, build, verify, export, or catalog", args[0])
	}
}

func runVendor(ctx context.Context, deps Dependencies, args []string, stdout, stderr io.Writer) error {
	flags := newFlagSet("vendor", stderr, "usage: araihu-assets vendor")
	if err := parse(flags, args); err != nil {
		return usagef("vendor: %v", err)
	}
	if err := contextError("vendor", ctx); err != nil {
		return err
	}
	repo, err := repository(deps)
	if err != nil {
		return commandError("vendor", err)
	}
	ui, err := manifest.LoadUI(os.DirFS(repo), "manifests/icons-ui.yaml")
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
		path := filepath.Join(repo, "vendor", "icons", "ui", source.Name, source.Version)
		if err := os.MkdirAll(path, 0o755); err != nil {
			return fmt.Errorf("vendor: create rooted vendor path %q: %w", path, err)
		}
		root, err := os.OpenRoot(path)
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
	inputs, repo, err := inputs(ctx, deps)
	if err != nil {
		return commandError("build", err)
	}
	if *check {
		if err := build.Check(repo, inputs); err != nil {
			return fmt.Errorf("build: check: %w", err)
		}
		fmt.Fprintln(stdout, "build: dist matches deterministic offline output")
		return nil
	}
	if err := build.Run(repo, inputs); err != nil {
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
	inputs, repo, err := inputs(ctx, deps)
	if err != nil {
		return commandError("verify", err)
	}
	if err := build.Check(repo, inputs); err != nil {
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
	repo, err := repository(deps)
	if err != nil {
		return commandError("export", err)
	}
	source, err := os.OpenRoot(filepath.Join(repo, "dist"))
	if err != nil {
		return fmt.Errorf("export: open live dist root: %w", err)
	}
	defer source.Close()
	paths, err := rootedFiles(ctx, source)
	if err != nil {
		return fmt.Errorf("export: inspect live dist: %w", err)
	}
	if err := os.MkdirAll(*output, 0o755); err != nil {
		return fmt.Errorf("export: create output root %q: %w", *output, err)
	}
	destination, err := os.OpenRoot(*output)
	if err != nil {
		return fmt.Errorf("export: open output root %q: %w", *output, err)
	}
	defer destination.Close()
	if err := assetexport.CopyRoot(source, paths, destination); err != nil {
		return fmt.Errorf("export: copy live dist into output root %q: %w", *output, err)
	}
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
	repo, err := repository(deps)
	if err != nil {
		return commandError("catalog", err)
	}
	root, err := os.OpenRoot(filepath.Join(repo, "dist"))
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

func inputs(ctx context.Context, deps Dependencies) (build.Inputs, string, error) {
	if err := ctx.Err(); err != nil {
		return build.Inputs{}, "", err
	}
	repo, err := repository(deps)
	if err != nil {
		return build.Inputs{}, "", err
	}
	files := os.DirFS(repo)
	brandManifest, err := manifest.LoadBrand(files, "manifests/brand.yaml")
	if err != nil {
		return build.Inputs{}, "", fmt.Errorf("manifest manifests/brand.yaml: %w", err)
	}
	uiManifest, err := manifest.LoadUI(files, "manifests/icons-ui.yaml")
	if err != nil {
		return build.Inputs{}, "", fmt.Errorf("manifest manifests/icons-ui.yaml: %w", err)
	}
	brand, err := transform.BuildBrand(files, brandManifest)
	if err != nil {
		return build.Inputs{}, "", fmt.Errorf("brand generator: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return build.Inputs{}, "", err
	}
	ui, err := provenance.BuildUI(files, uiManifest)
	if err != nil {
		return build.Inputs{}, "", fmt.Errorf("ui generator: %w", err)
	}
	icons, err := platformIcons(brand)
	if err != nil {
		return build.Inputs{}, "", err
	}
	rasterizer := deps.Rasterizer
	if rasterizer == nil {
		rasterizer = platform.NewRSVG(nil)
	}
	platformFiles, err := platform.Build(ctx, rasterizer, icons)
	if err != nil {
		return build.Inputs{}, "", fmt.Errorf("platform generator: %w", err)
	}
	return build.Inputs{Brand: brand, UI: ui, Platform: platformFiles}, repo, nil
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
