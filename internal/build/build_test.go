package build

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/platform"
	"github.com/araihu/assets/internal/provenance"
	"github.com/araihu/assets/internal/transform"
)

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
	firstArchive, err := os.ReadFile(filepath.Join(first, "dist", "releases", "araihu-assets-v0.1.0.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err := os.ReadFile(filepath.Join(second, "dist", "releases", "araihu-assets-v0.1.0.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstArchive, secondArchive) {
		t.Fatal("independent tar.gz builds differ")
	}
	requireArchiveMembers(t, firstArchive, []string{"NOTICE", "catalog.json", "checksums.txt", "icons/brand/asset.svg", "licenses/Apache-2.0.txt", "licenses/heroicons-MIT.txt", "platform/web/araihu/favicon.svg"})
}

func testInputs(data []byte) Inputs {
	sum := sha256.Sum256(data)
	return Inputs{
		Brand: transform.Result{Files: map[string][]byte{"dist/icons/brand/asset.svg": data}, Assets: []catalog.Asset{{
			CanonicalName: "araihu-icon-light-transparent-optical", Namespace: "brand", Path: "icons/brand/asset.svg", Product: "araihu", Artwork: "icon", Appearance: "light", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: "0 0 1 1"}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "source/brand/original/asset.svg", SHA256: hex.EncodeToString(sum[:]),
		}}},
		UI: provenance.Result{Files: map[string][]byte{"licenses/heroicons-MIT.txt": []byte("MIT\n")}},
		Platform: platform.Result{Files: map[string][]byte{"dist/platform/web/araihu/favicon.svg": []byte("platform")}, Assets: []catalog.Asset{{
			CanonicalName: "platform-web-araihu-favicon-svg", Namespace: "brand", Path: "platform/web/araihu/favicon.svg", Product: "araihu", Artwork: "icon", Appearance: "adaptive", Surface: "transparent", Framing: "optical", Format: "svg",
			Dimensions: catalog.Dimensions{ViewBox: "0 0 1 1"}, ColorBehavior: "protected", License: "Arai Hu Brand Terms", Source: "platform generator v0.1.0", SHA256: hash([]byte("platform")),
		}}},
	}
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func testRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustWrite(t, filepath.Join(repo, "LICENSE"), []byte("Apache License\n"))
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
