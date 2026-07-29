package platform

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/araihu/assets/internal/manifest"
	"github.com/araihu/assets/internal/transform"
)

func TestRSVGRequiresPinnedVersionAndPassesSafeArguments(t *testing.T) {
	runner := &recordingRunner{version: "rsvg-convert version 2.62.1\n", output: validPNGFallback(t)}
	rasterizer := NewRSVG(runner)
	_, err := rasterizer.Rasterize(context.Background(), Request{SVG: []byte("<svg/>"), Width: 16, Height: 16, Background: "#f3f2e9"})
	if err != nil {
		t.Fatalf("Rasterize(): %v", err)
	}
	if len(runner.calls) != 2 || runner.calls[0].name != "rsvg-convert" || len(runner.calls[0].args) != 1 || runner.calls[0].args[0] != "--version" {
		t.Fatalf("unexpected version command: %#v", runner.calls)
	}
	got := strings.Join(runner.calls[1].args, " ")
	for _, want := range []string{"--width 16", "--height 16", "--format png", "--background-color #f3f2e9", "-"} {
		if !strings.Contains(got, want) {
			t.Errorf("raster command missing %q: %s", want, got)
		}
	}
}

func TestRSVGRejectsMissingAndMismatchedBinary(t *testing.T) {
	missing := NewRSVG(&recordingRunner{err: exec.ErrNotFound})
	if _, err := missing.Rasterize(context.Background(), Request{SVG: []byte("<svg/>"), Width: 1, Height: 1}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing binary error = %v", err)
	}
	mismatch := NewRSVG(&recordingRunner{version: "rsvg-convert version 2.61.0\n"})
	if _, err := mismatch.Rasterize(context.Background(), Request{SVG: []byte("<svg/>"), Width: 1, Height: 1}); err == nil || !strings.Contains(err.Error(), "2.62.1") {
		t.Fatalf("version mismatch error = %v", err)
	}
}

func TestRSVGReportsBoundedStderrAndContextFailures(t *testing.T) {
	stderr := []byte(strings.Repeat("x", 6000))
	nonzero := NewRSVG(&recordingRunner{version: "rsvg-convert version 2.62.1\n", err: errors.New("exit status 1"), stderr: stderr})
	_, err := nonzero.Rasterize(context.Background(), Request{SVG: []byte("<svg/>"), Width: 1, Height: 1})
	if err == nil || !strings.Contains(err.Error(), "exit status 1") || len(err.Error()) > 4300 {
		t.Fatalf("nonzero error not useful/bounded: %v", err)
	}
	timeout := NewRSVG(&blockingRunner{})
	timeout.Timeout = time.Millisecond
	_, err = timeout.Rasterize(context.Background(), Request{SVG: []byte("<svg/>"), Width: 1, Height: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cancelled := NewRSVG(&blockingRunner{})
	_, err = cancelled.Rasterize(ctx, Request{SVG: []byte("<svg/>"), Width: 1, Height: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
}

func TestRSVGValidatesRequest(t *testing.T) {
	rasterizer := NewRSVG(&recordingRunner{})
	for _, request := range []Request{
		{Width: 1, Height: 1},
		{SVG: []byte("<svg/>"), Width: 0, Height: 1},
		{SVG: []byte("<svg/>"), Width: 1, Height: 1, Background: "red"},
		{SVG: []byte("<svg/>"), Width: 8193, Height: 1},
	} {
		if _, err := rasterizer.Rasterize(context.Background(), request); err == nil {
			t.Fatalf("Rasterize accepted %#v", request)
		}
	}
}

func TestRSVGIntegrationPinnedBinary(t *testing.T) {
	if os.Getenv("ARAIHU_RSVG_INTEGRATION") != "1" {
		t.Skip("set ARAIHU_RSVG_INTEGRATION=1 to run pinned rsvg-convert proof")
	}
	if _, err := exec.LookPath("rsvg-convert"); err != nil {
		t.Fatalf("rsvg-convert unavailable: %v", err)
	}
	result, err := Build(context.Background(), NewRSVG(nil), integrationIcons(t))
	if err != nil {
		t.Fatalf("Build with pinned rsvg-convert: %v", err)
	}
	if got, want := len(result.Files), 155; got != want {
		t.Fatalf("generated file count = %d, want %d", got, want)
	}
	requirePNG(t, result, "dist/platform/web/araihu/icon-maskable-512.png", 512, false, true)
	requirePNG(t, result, "dist/platform/android/x9/res/drawable-xxxhdpi/ic_launcher_foreground.png", 432, true, true)
	darkPath := "dist/platform/apple/manja/Assets.xcassets/AppIcon.appiconset/AppIcon-1024-dark.png"
	requirePNG(t, result, darkPath, 1024, false, false)
	if err := validateSafeBounds(result.Files[darkPath], darkBackground); err != nil {
		t.Fatalf("dark Apple safe bounds: %v", err)
	}
}

func integrationIcons(t *testing.T) []BrandIcon {
	t.Helper()
	root := filepath.Join("..", "..")
	brand, err := manifest.LoadBrand(os.DirFS(root), "manifests/brand.yaml")
	if err != nil {
		t.Fatalf("LoadBrand(): %v", err)
	}
	generated, err := transform.BuildBrand(os.DirFS(root), brand)
	if err != nil {
		t.Fatalf("BuildBrand(): %v", err)
	}
	icons := make([]BrandIcon, 0, len(products))
	for _, product := range products {
		get := func(name string) []byte {
			path := "dist/icons/brand/" + product + "-icon-" + name + ".svg"
			data, ok := generated.Files[path]
			if !ok {
				t.Fatalf("missing Task 4 output %s", path)
			}
			return data
		}
		icons = append(icons, BrandIcon{
			Product:       product,
			LightSVG:      get("light-transparent-optical"),
			DarkSVG:       get("dark-transparent-optical"),
			TintedSVG:     get("tinted-transparent-optical"),
			MonochromeSVG: get("monochrome-transparent-optical"),
			AdaptiveSVG:   get("adaptive-plate-launcher"),
			LauncherSVG:   get("tinted-plate-launcher"),
		})
	}
	return icons
}

type call struct {
	name string
	args []string
}
type recordingRunner struct {
	version        string
	output, stderr []byte
	err            error
	calls          []call
}

func (r *recordingRunner) Run(_ context.Context, name string, args []string, _ []byte) ([]byte, []byte, error) {
	r.calls = append(r.calls, call{name: name, args: append([]string(nil), args...)})
	if len(args) == 1 && args[0] == "--version" {
		return []byte(r.version), nil, r.err
	}
	return r.output, r.stderr, r.err
}

type blockingRunner struct{}

func (*blockingRunner) Run(ctx context.Context, _ string, _ []string, _ []byte) ([]byte, []byte, error) {
	<-ctx.Done()
	return nil, nil, ctx.Err()
}

func validPNGFallback(t *testing.T) []byte {
	t.Helper()
	data, err := fakeRasterizer{}.Rasterize(context.Background(), Request{Width: 16, Height: 16})
	if err != nil {
		t.Fatal(err)
	}
	return data
}
