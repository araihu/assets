package build

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/campaigns"
	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/platform"
	"github.com/araihu/assets/internal/provenance"
	"github.com/araihu/assets/internal/releaseinfo"
	"github.com/araihu/assets/internal/releasemeta"
	"github.com/araihu/assets/internal/themes"
	"github.com/araihu/assets/internal/transform"
)

func TestRunPublishesCapturedThemeCatalog(t *testing.T) {
	repo := testRepo(t)
	inputs := testInputs([]byte("asset"))
	css := []byte("[data-theme=\"araihu\"] { --color-surface: #f3f2e9; }\n")
	inputs.Themes = themes.Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []themes.Theme{{ID: "araihu", CSSPath: "themes/araihu.css"}}}
	inputs.ThemeCSS = map[string][]byte{"themes/araihu.css": css}

	if err := Run(repo, inputs); err != nil {
		t.Fatal(err)
	}
	gotCSS, err := os.ReadFile(filepath.Join(repo, "dist", "themes", "araihu.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotCSS, css) {
		t.Fatalf("published CSS = %q, want %q", gotCSS, css)
	}
	gotCatalog, err := os.ReadFile(filepath.Join(repo, "dist", "themes.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(gotCatalog, []byte(`"sha256": "`+hash(css)+`"`)) {
		t.Fatalf("themes catalog misses captured CSS hash: %s", gotCatalog)
	}
	mustWrite(t, filepath.Join(repo, "themes", "araihu.css"), []byte("live source changed"))
	if err := Check(repo, inputs); err != nil {
		t.Fatalf("Check() reread live CSS source: %v", err)
	}
}

func TestRunPublishesReleaseInventoryBeforeChecksumsAndArchives(t *testing.T) {
	repo := testRepo(t)
	inputs := testInputs([]byte("asset"))
	inputs.Campaigns = campaigns.Manifest{SchemaVersion: 1}
	if err := Run(repo, inputs); err != nil {
		t.Fatal(err)
	}
	releaseBytes, err := os.ReadFile(filepath.Join(repo, "dist", "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document releasemeta.Document
	if err := json.Unmarshal(releaseBytes, &document); err != nil {
		t.Fatal(err)
	}
	if err := document.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalog.json", "themes.json", "campaigns.json"} {
		data, err := os.ReadFile(filepath.Join(repo, "dist", name))
		if err != nil {
			t.Fatal(err)
		}
		if !inventoryHasHash(document.Files, name, hash(data)) {
			t.Fatalf("release inventory omits current %s", name)
		}
	}
	if inventoryHasPath(document.Files, "release.json") || inventoryHasPath(document.Files, "checksums.txt") {
		t.Fatalf("release inventory includes generated successor: %#v", document.Files)
	}
	checksums, err := os.ReadFile(filepath.Join(repo, "dist", "checksums.txt"))
	if err != nil || !bytes.Contains(checksums, []byte("  release.json\n")) {
		t.Fatalf("checksums = %q, %v", checksums, err)
	}
	members := tarArchiveMembers(t, filepath.Join(repo, "dist", "releases", releaseinfo.ArchiveName("tar.gz")))
	for _, name := range []string{"catalog.json", "themes.json", "campaigns.json", "release.json", "checksums.txt"} {
		if !slices.Contains(members, name) {
			t.Fatalf("archive omits %s: %q", name, members)
		}
	}
}

func TestRunEmitsConsistentV014ReleaseFields(t *testing.T) {
	repo := testRepo(t)
	if err := Run(repo, testInputs([]byte("asset"))); err != nil {
		t.Fatal(err)
	}

	catalogBytes, err := os.ReadFile(filepath.Join(repo, "dist", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	assetCatalog, err := catalog.Decode(bytes.NewReader(catalogBytes))
	if err != nil {
		t.Fatal(err)
	}
	themesBytes, err := os.ReadFile(filepath.Join(repo, "dist", "themes.json"))
	if err != nil {
		t.Fatal(err)
	}
	var themeCatalog themes.Catalog
	if err := json.Unmarshal(themesBytes, &themeCatalog); err != nil {
		t.Fatal(err)
	}
	releaseBytes, err := os.ReadFile(filepath.Join(repo, "dist", "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var releaseDocument releasemeta.Document
	if err := json.Unmarshal(releaseBytes, &releaseDocument); err != nil {
		t.Fatal(err)
	}

	const wantRelease = "v0.1.4"
	for name, got := range map[string]string{
		"catalog":  assetCatalog.Release,
		"themes":   themeCatalog.Release,
		"metadata": releaseDocument.Release,
	} {
		if got != wantRelease {
			t.Fatalf("%s release = %q, want %q", name, got, wantRelease)
		}
	}
	checksums, err := os.ReadFile(filepath.Join(repo, "dist", "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(checksums, []byte(hash(releaseBytes)+"  release.json\n")) {
		t.Fatal("checksums omit the same release metadata")
	}
	for _, extension := range []string{"tar.gz", "zip"} {
		archive := filepath.Join(repo, "dist", "releases", "araihu-assets-"+wantRelease+"."+extension)
		if _, err := os.Stat(archive); err != nil {
			t.Fatalf("release archive %q: %v", archive, err)
		}
	}
}

func TestRunPublishesCapturedCampaignRuntimeInInventoryChecksumsAndArchive(t *testing.T) {
	repo := testRepo(t)
	runtimePath := filepath.Join(repo, "runtime", "campaign", "v1.js")
	captured := []byte("(function(){window.campaignVersion=1;}());\n")
	mustWrite(t, runtimePath, captured)
	buildHook = func(phase buildPhase) {
		if phase == beforePublish {
			mustWrite(t, runtimePath, []byte("changed after staging\n"))
		}
	}
	t.Cleanup(func() { buildHook = nil })

	if err := Run(repo, testInputs([]byte("asset"))); err != nil {
		t.Fatal(err)
	}
	published, err := os.ReadFile(filepath.Join(repo, "dist", "campaign", "v1.js"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(published, captured) {
		t.Fatalf("published runtime = %q, want captured %q", published, captured)
	}
	releaseBytes, err := os.ReadFile(filepath.Join(repo, "dist", "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document releasemeta.Document
	if err := json.Unmarshal(releaseBytes, &document); err != nil {
		t.Fatal(err)
	}
	if !inventoryHasHash(document.Files, "campaign/v1.js", hash(captured)) {
		t.Fatal("release inventory omits captured campaign/v1.js")
	}
	checksums, err := os.ReadFile(filepath.Join(repo, "dist", "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(checksums, []byte(hash(captured)+"  campaign/v1.js\n")) {
		t.Fatalf("checksums omit captured campaign/v1.js: %s", checksums)
	}
	members := tarArchiveMembers(t, filepath.Join(repo, "dist", "releases", releaseinfo.ArchiveName("tar.gz")))
	if !slices.Contains(members, "campaign/v1.js") {
		t.Fatalf("release archive omits campaign/v1.js: %q", members)
	}
}

func TestRunFailurePreservesPublishedDist(t *testing.T) {
	repo := testRepo(t)
	mustWrite(t, filepath.Join(repo, "dist", "sentinel.txt"), []byte("keep"))
	inputs := testInputs([]byte("asset"))
	inputs.Brand.Assets[0].SHA256 = "not-a-hash"
	if err := Run(repo, inputs); err == nil {
		t.Fatal("Run accepted invalid catalog input")
	}
	if got, err := os.ReadFile(filepath.Join(repo, "dist", "sentinel.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("failed build changed published dist: %q, %v", got, err)
	}
}

func TestRunRejectsUnsafeGeneratedSVG(t *testing.T) {
	for _, generated := range [][]byte{
		[]byte(`<svg viewBox="0 0 1 1"><script/></svg>`),
		[]byte(`<svg viewBox="0 0 1 1"><use href="#missing"/></svg>`),
	} {
		repo := testRepo(t)
		inputs := testInputs([]byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`))
		inputs.Brand.Files["dist/icons/brand/asset.svg"] = generated
		inputs.Brand.Assets[0].SHA256 = hash(generated)
		if err := Run(repo, inputs); err == nil || !strings.Contains(err.Error(), "validate generated SVG") {
			t.Fatalf("Run() error = %v, want generated SVG validation failure", err)
		}
	}
}

func TestRunContextCancellationBeforePublishPreservesDist(t *testing.T) {
	repo := testRepo(t)
	mustWrite(t, filepath.Join(repo, "dist", "sentinel.txt"), []byte("keep"))
	ctx, cancel := context.WithCancel(context.Background())
	buildHook = func(phase buildPhase) {
		if phase == beforePublish {
			cancel()
		}
	}
	t.Cleanup(func() { buildHook = nil })

	err := RunContext(ctx, repo, testInputs([]byte("asset")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("RunContext() error = %v, want context canceled", err)
	}
	if got, err := os.ReadFile(filepath.Join(repo, "dist", "sentinel.txt")); err != nil || string(got) != "keep" {
		t.Fatalf("cancelled build changed published dist: %q, %v", got, err)
	}
}

func TestInputPathRejectsDriveAndVolumeAmbiguity(t *testing.T) {
	for _, name := range []string{"C:/asset.svg", "c:asset.svg", "drive:segment/file.svg", `dir\\asset.svg`} {
		if _, err := normalizeInputPath(name); err == nil {
			t.Fatalf("normalizeInputPath accepted %q", name)
		}
	}
}

func TestRunPublishesOnlyManagedDistAndCheckMatchesExactBytes(t *testing.T) {
	repo := testRepo(t)
	mustWrite(t, filepath.Join(repo, "dist", "obsolete.txt"), []byte("old"))
	inputs := testInputs([]byte("asset"))
	if err := Run(repo, inputs); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(repo, "dist", "obsolete.txt")); !os.IsNotExist(err) {
		t.Fatalf("unmanaged prior output remains: %v", err)
	}
	if err := Check(repo, inputs); err != nil {
		t.Fatalf("Check published output: %v", err)
	}
	catalogBytes, err := os.ReadFile(filepath.Join(repo, "dist", "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := catalog.Decode(bytes.NewReader(catalogBytes))
	if err != nil {
		t.Fatal(err)
	}
	foundPlatform := false
	for _, asset := range decoded.Assets {
		if asset.Path == "platform/web/araihu/favicon.svg" {
			foundPlatform = true
		}
	}
	if !foundPlatform {
		t.Fatal("catalog omits platform visual artifact")
	}
	mustWrite(t, filepath.Join(repo, "dist", "icons", "brand", "asset.svg"), []byte("changed"))
	if err := Check(repo, inputs); err == nil {
		t.Fatal("Check accepted changed artifact bytes")
	}
}

func TestRunCopiesProofStaticAssetsDeterministically(t *testing.T) {
	repo := testRepo(t)
	styles := []byte("body { color: cobalt; }\n")
	script := []byte("window.proofControls = true;\n")
	mustWrite(t, filepath.Join(repo, "site", "proof", "styles.css"), styles)
	mustWrite(t, filepath.Join(repo, "site", "proof", "app.js"), script)

	if err := Run(repo, testInputs([]byte("asset"))); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]byte{
		"styles.css": styles,
		"app.js":     script,
	} {
		got, err := os.ReadFile(filepath.Join(repo, "dist", "proof", name))
		if err != nil {
			t.Fatalf("ReadFile dist/proof/%s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("dist/proof/%s = %q, want %q", name, got, want)
		}
	}
	if err := Check(repo, testInputs([]byte("asset"))); err != nil {
		t.Fatalf("Check() error = %v", err)
	}
}

func TestRunPublishesCatalogBoundProofDocument(t *testing.T) {
	repo := testRepo(t)
	template, err := os.ReadFile(filepath.Join("..", "..", "site", "proof", "index.tmpl"))
	if err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(repo, "site", "proof", "index.tmpl"), template)
	mustWrite(t, filepath.Join(repo, "site", "proof", "scenarios.json"), []byte(`{"scenarios":[{"id":"araihu-icon-proof","group":"brand","asset":"araihu-icon-light-transparent-optical","artwork":"icon","appearance":"light","surface":"transparent","framing":"optical","mask":"none","context":"web-navigation","sizes":[16]}]}`))

	if err := Run(repo, proofInputs()); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(filepath.Join(repo, "dist", "proof", "index.html"))
	if err != nil {
		t.Fatalf("ReadFile generated proof: %v", err)
	}
	if !bytes.Contains(page, []byte(`data-scenario="araihu-icon-proof"`)) {
		t.Fatalf("generated proof misses catalog scenario: %s", page)
	}
	proofAsset, err := os.ReadFile(filepath.Join(repo, "dist", "proof", "assets", "icons", "brand", "asset.svg"))
	if err != nil {
		t.Fatalf("ReadFile proof-local asset: %v", err)
	}
	if !bytes.Equal(proofAsset, []byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`)) {
		t.Fatalf("proof-local asset = %q", proofAsset)
	}
	if err := Check(repo, proofInputs()); err != nil {
		t.Fatalf("Check() rejects generated proof: %v", err)
	}
}

func TestRunWritesSortedChecksumsAndDeterministicReleaseMembership(t *testing.T) {
	first, second := testRepo(t), testRepo(t)
	inputs := testInputs([]byte("asset"))
	for _, repo := range []string{first, second} {
		if err := Run(repo, inputs); err != nil {
			t.Fatal(err)
		}
	}
	checksums, err := os.ReadFile(filepath.Join(first, "dist", "checksums.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(checksums, []byte("checksums.txt")) || bytes.Contains(checksums, []byte("releases/")) {
		t.Fatalf("checksums contain self or archive reference: %s", checksums)
	}
	lines := strings.FieldsFunc(strings.TrimSuffix(string(checksums), "\n"), func(r rune) bool { return r == '\n' })
	if !slices.IsSortedFunc(lines, func(a, b string) int { return strings.Compare(a[66:], b[66:]) }) {
		t.Fatalf("checksums not sorted by path: %s", checksums)
	}
	line := regexp.MustCompile(`^[0-9a-f]{64}  [^\\]+$`)
	for _, checksum := range lines {
		if !line.MatchString(checksum) {
			t.Fatalf("invalid checksum line %q", checksum)
		}
	}
	firstArchive, err := os.ReadFile(filepath.Join(first, "dist", "releases", releaseinfo.ArchiveName("tar.gz")))
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err := os.ReadFile(filepath.Join(second, "dist", "releases", releaseinfo.ArchiveName("tar.gz")))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstArchive, secondArchive) {
		t.Fatal("independent tar.gz builds differ")
	}
	requireArchiveMembers(t, firstArchive, []string{"NOTICE", "campaign/v1.js", "campaigns.json", "catalog.json", "checksums.txt", "icons/brand/asset.svg", "licenses/Apache-2.0.txt", "licenses/heroicons-MIT.txt", "platform/web/araihu/favicon.svg", "proof/app.js", "proof/styles.css", "release.json", "themes.json", "themes/araihu.css"})
}

func TestProductionReleaseArchivesIncludeExactProofTree(t *testing.T) {
	dist := filepath.Join("..", "..", "dist")
	want := releaseIncludeList(t, dist)
	proof := releaseIncludeList(t, filepath.Join(dist, "proof"))
	for _, name := range []string{"index.html", "styles.css", "app.js"} {
		if !slices.Contains(proof, name) {
			t.Fatalf("tracked dist/proof omits required file %q", name)
		}
	}
	localAssets := 0
	for _, name := range proof {
		if strings.HasPrefix(name, "assets/") {
			localAssets++
		}
	}
	if localAssets == 0 {
		t.Fatal("tracked dist/proof omits local copied assets")
	}
	if len(proof) < 4 {
		t.Fatalf("tracked dist/proof has %d files, want local proof assets", len(proof))
	}

	for _, tc := range []struct {
		name    string
		archive string
		members func(t *testing.T, archive string) []string
	}{
		{"tar.gz", filepath.Join(dist, "releases", releaseinfo.ArchiveName("tar.gz")), tarArchiveMembers},
		{"zip", filepath.Join(dist, "releases", releaseinfo.ArchiveName("zip")), zipArchiveMembers},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.members(t, tc.archive)
			if !slices.Equal(got, want) {
				t.Fatalf("release include-list differs: got %d members, want %d", len(got), len(want))
			}
			for _, name := range proof {
				if !slices.Contains(got, "proof/"+name) {
					t.Fatalf("release archive omits dist/proof/%s", name)
				}
			}
			for _, forbidden := range []string{"review/screenshots/", ".impeccable/critique/"} {
				for _, name := range got {
					if strings.HasPrefix(name, forbidden) {
						t.Fatalf("release archive contains excluded review artifact %q", name)
					}
				}
			}
		})
	}
}

func testInputs(_ []byte) Inputs {
	data := []byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`)
	sum := sha256.Sum256(data)
	return Inputs{
		Brand: transform.Result{Files: map[string][]byte{"dist/icons/brand/asset.svg": data}, Assets: []catalog.Asset{{
			CanonicalName: "araihu-icon-light-transparent-optical", Namespace: "brand", Path: "icons/brand/asset.svg", Product: "araihu", Artwork: "icon", Appearance: "light", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: "0 0 1 1"}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "source/brand/original/asset.svg", SHA256: hex.EncodeToString(sum[:]),
		}}},
		UI:        provenance.Result{Files: map[string][]byte{"licenses/heroicons-MIT.txt": []byte("MIT\n")}},
		Campaigns: campaigns.Manifest{SchemaVersion: 1},
		Themes:    themes.Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []themes.Theme{{ID: "araihu", CSSPath: "themes/araihu.css"}}},
		ThemeCSS:  map[string][]byte{"themes/araihu.css": []byte("[data-theme=\"araihu\"] {}\n")},
		Platform: platform.Result{Files: map[string][]byte{"dist/platform/web/araihu/favicon.svg": []byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`)}, Assets: []catalog.Asset{{
			CanonicalName: "platform-web-araihu-favicon-svg", Namespace: "brand", Path: "platform/web/araihu/favicon.svg", Product: "araihu", Artwork: "icon", Appearance: "adaptive", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: "0 0 1 1"}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "platform generator v0.1.0", SHA256: hash([]byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`)),
		}}},
	}
}

func proofInputs() Inputs {
	icon := []byte(`<svg viewBox="0 0 1 1"><path d="M0 0h1v1z"/></svg>`)
	master := []byte("png")
	return Inputs{
		Brand: transform.Result{Files: map[string][]byte{"dist/icons/brand/asset.svg": icon}, Assets: []catalog.Asset{{
			CanonicalName: "araihu-icon-light-transparent-optical", Namespace: "brand", Path: "icons/brand/asset.svg", Product: "araihu", Artwork: "icon", Appearance: "light", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: "0 0 1 1"}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "source/brand/original/asset.svg", SHA256: hash(icon),
		}}},
		Platform: platform.Result{Files: map[string][]byte{
			"dist/platform/web/araihu/icon-maskable-512.png":                              master,
			"dist/platform/web/araihu/manifest-icons.json":                                []byte("{}"),
			"dist/platform/android/araihu/res/mipmap-anydpi-v26/ic_launcher.xml":          []byte("<resources/>"),
			"dist/platform/apple/araihu/Assets.xcassets/AppIcon.appiconset/Contents.json": []byte("{}"),
		}, Assets: []catalog.Asset{{
			CanonicalName: "platform-web-araihu-icon-maskable-512-png", Namespace: "brand", Path: "platform/web/araihu/icon-maskable-512.png", Product: "araihu", Artwork: "icon", Appearance: "light", Surface: "plate", Framing: "launcher", Format: "png",
			Dimensions: catalog.Dimensions{Width: 512, Height: 512}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "platform generator", SHA256: hash(master),
		}}},
		UI:        provenance.Result{Files: map[string][]byte{"licenses/heroicons-MIT.txt": []byte("MIT\n")}},
		Campaigns: campaigns.Manifest{SchemaVersion: 1},
		Themes:    themes.Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []themes.Theme{{ID: "araihu", CSSPath: "themes/araihu.css"}}},
		ThemeCSS:  map[string][]byte{"themes/araihu.css": []byte("[data-theme=\"araihu\"] {}\n")},
	}
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func inventoryHasHash(files []releasemeta.File, name, want string) bool {
	for _, file := range files {
		if file.Path == name && file.SHA256 == want {
			return true
		}
	}
	return false
}

func inventoryHasPath(files []releasemeta.File, name string) bool {
	for _, file := range files {
		if file.Path == name {
			return true
		}
	}
	return false
}

func testRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "LICENSE"), []byte("Apache License\n"))
	mustWrite(t, filepath.Join(repo, "site", "proof", "styles.css"), []byte("body {}\n"))
	mustWrite(t, filepath.Join(repo, "site", "proof", "app.js"), []byte("\"use strict\";\n"))
	mustWrite(t, filepath.Join(repo, "runtime", "campaign", "v1.js"), []byte("(function(){})();\n"))
	return repo
}

func mustWrite(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func requireArchiveMembers(t *testing.T, data []byte, want []string) {
	t.Helper()
	gzipReader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gzipReader.Close()
	tarReader := tar.NewReader(gzipReader)
	var got []string
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, header.Name)
	}
	if len(got) != len(want) {
		t.Fatalf("archive members = %q, want %q", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("archive members = %q, want %q", got, want)
		}
	}
}

func releaseIncludeList(t *testing.T, root string) []string {
	t.Helper()
	files := make([]string, 0)
	if err := filepath.WalkDir(root, func(name string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular release file %q", name)
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if strings.HasPrefix(relative, "releases/") {
			return nil
		}
		files = append(files, relative)
		return nil
	}); err != nil {
		t.Fatalf("list release files: %v", err)
	}
	slices.Sort(files)
	return files
}

func tarArchiveMembers(t *testing.T, archive string) []string {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	reader, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	members := make([]string, 0)
	entries := tar.NewReader(reader)
	for {
		header, err := entries.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		members = append(members, header.Name)
	}
	return members
}

func zipArchiveMembers(t *testing.T, archive string) []string {
	t.Helper()
	reader, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	members := make([]string, 0, len(reader.File))
	for _, file := range reader.File {
		members = append(members, file.Name)
	}
	return members
}
