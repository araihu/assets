package consumerupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/araihu/assets/internal/catalog"
	"github.com/araihu/assets/internal/releasefanout"
	"github.com/araihu/assets/internal/releasemeta"
)

func fixture(t *testing.T) (Manifest, releasefanout.Metadata, map[string][]byte) {
	t.Helper()
	logo := []byte(`<svg viewBox="0 0 10 10"/>`)
	var cat bytes.Buffer
	if err := catalog.Encode(&cat, catalog.Catalog{SchemaVersion: 2, Release: "v0.2.5", IdentityRevision: 11, Assets: []catalog.Asset{{CanonicalName: "x9-logo-adaptive-transparent-optical", Namespace: "brand", Path: "brand/x9/logo/adaptive-transparent-optical.svg", Product: "x9", Artwork: "logo", Appearance: "adaptive", Surface: "transparent", Framing: "optical", Format: "svg", Dimensions: catalog.Dimensions{ViewBox: "0 0 10 10"}, ColorBehavior: "protected", License: "Brand Terms", Source: "source/logo.svg", SHA256: digest(logo)}}}); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{"catalog.json": cat.Bytes(), "themes.json": []byte(`{}`), "campaigns.json": []byte(`{}`), "brand/x9/logo/adaptive-transparent-optical.svg": logo, "themes/araihu.css": []byte(`body { color: black; }`)}
	fsys := fstest.MapFS{}
	for name, data := range files {
		fsys[name] = &fstest.MapFile{Data: data}
	}
	doc, err := releasemeta.Build(releasemeta.Input{Release: "v0.2.5", IdentityRevision: 11, RuntimeVersion: 1, Files: fsys})
	if err != nil {
		t.Fatal(err)
	}
	files["release.json"], err = releasemeta.Encode(doc)
	if err != nil {
		t.Fatal(err)
	}
	metadata := releasefanout.Metadata{AssetsRepository: "araihu/assets", AssetsRevision: strings.Repeat("a", 40), Release: "v0.2.5", ReleaseURL: "https://github.com/araihu/assets/releases/download/v0.2.5/araihu-assets-v0.2.5.tar.gz", ReleaseJSONSHA256: digest(files["release.json"])}
	metadata.ReleaseSHA256 = digest(tarball(t, files, nil))
	m := Manifest{SchemaVersion: 1, Metadata: metadata, Mappings: []Mapping{{Source: "brand/x9/logo/adaptive-transparent-optical.svg", Destination: "site/assets/logo.svg", CanonicalName: "x9-logo-adaptive-transparent-optical"}, {Source: "themes/araihu.css", Destination: "ui/theme.css"}}}
	return m, metadata, files
}

func tarball(t *testing.T, files map[string][]byte, extra *tar.Header) []byte {
	t.Helper()
	var out bytes.Buffer
	gz := gzip.NewWriter(&out)
	tw := tar.NewWriter(gz)
	// Map iteration must not change the archive digest across fixture calls.
	for _, name := range []string{"catalog.json", "themes.json", "campaigns.json", "brand/x9/logo/adaptive-transparent-optical.svg", "themes/araihu.css", "release.json"} {
		data, ok := files[name]
		if !ok {
			continue
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if extra != nil {
		if err := tw.WriteHeader(extra); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func TestPrepareV2AndIdempotentPatch(t *testing.T) {
	m, metadata, files := fixture(t)
	archive := tarball(t, files, nil)
	patch, err := Prepare(m, metadata, archive)
	if err != nil {
		t.Fatal(err)
	}
	if len(patch) != 3 || !bytes.Equal(patch["site/assets/logo.svg"], files[m.Mappings[0].Source]) {
		t.Fatalf("unexpected patch: %v", patch)
	}
	receipt, err := Load(patch[ManifestPath])
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Metadata != metadata {
		t.Fatal("receipt changed provenance")
	}
	second, err := Prepare(receipt, metadata, archive)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range patch {
		if !bytes.Equal(data, second[name]) {
			t.Errorf("non-deterministic %s", name)
		}
	}
	output := filepath.Join(t.TempDir(), "patch")
	if err := WritePatch(output, patch); err != nil {
		t.Fatal(err)
	}
	if err := WritePatch(output, patch); err == nil {
		t.Fatal("accepted existing output")
	}
	if got, err := os.ReadFile(filepath.Join(output, "site/assets/logo.svg")); err != nil || !bytes.Equal(got, files[m.Mappings[0].Source]) {
		t.Fatalf("written logo: %v", err)
	}
}

func TestRejectTamperingAndUnsafeArchives(t *testing.T) {
	for _, name := range []string{"archive", "release metadata", "mapped bytes", "selection", "missing", "traversal", "symlink", "duplicate", "case collision"} {
		t.Run(name, func(t *testing.T) {
			m, metadata, files := fixture(t)
			var extra *tar.Header
			switch name {
			case "archive":
				metadata.ReleaseSHA256 = strings.Repeat("b", 64)
			case "release metadata":
				metadata.ReleaseJSONSHA256 = strings.Repeat("b", 64)
			case "mapped bytes":
				files[m.Mappings[0].Source] = []byte("tampered")
			case "selection":
				m.Mappings[0].CanonicalName = "x9-icon-adaptive-transparent-optical"
			case "missing":
				delete(files, m.Mappings[0].Source)
			case "traversal":
				extra = &tar.Header{Name: "../escape", Mode: 0o644}
			case "symlink":
				extra = &tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"}
			case "duplicate":
				extra = &tar.Header{Name: "catalog.json", Mode: 0o644}
			case "case collision":
				extra = &tar.Header{Name: "CATALOG.JSON", Mode: 0o644}
			}
			archive := tarball(t, files, extra)
			if name != "archive" {
				metadata.ReleaseSHA256 = digest(archive)
			}
			if _, err := Prepare(m, metadata, archive); err == nil {
				t.Fatal("accepted invalid release")
			}
		})
	}
}

func TestStrictConsumerManifest(t *testing.T) {
	m, _, _ := fixture(t)
	for _, name := range []string{"unknown", "duplicate key", "traversal", "collision", "unselected visual", "git path"} {
		t.Run(name, func(t *testing.T) {
			copy := m
			copy.Mappings = append([]Mapping(nil), m.Mappings...)
			switch name {
			case "traversal":
				copy.Mappings[0].Destination = "../escape"
			case "collision":
				copy.Mappings[1].Destination = strings.ToUpper(copy.Mappings[0].Destination)
			case "unselected visual":
				copy.Mappings[0].CanonicalName = ""
			case "git path":
				copy.Mappings[0].Destination = ".git/config"
			}
			data, _ := json.Marshal(copy)
			if name == "unknown" {
				data = append([]byte(`{"unexpected":true,`), data[1:]...)
			}
			if name == "duplicate key" {
				data = append([]byte(`{"schemaVersion":1,`), data[1:]...)
			}
			if _, err := Load(data); err == nil {
				t.Fatal("accepted invalid manifest")
			}
		})
	}
}

func TestEventMetadata(t *testing.T) {
	_, metadata, _ := fixture(t)
	raw, _ := json.Marshal(metadata)
	for _, data := range []string{`{"action":"araihu-assets-released","client_payload":` + string(raw) + `}`, `{"inputs":` + string(raw) + `}`} {
		got, err := EventMetadata([]byte(data))
		if err != nil || got != metadata {
			t.Fatalf("event: %v", err)
		}
	}
	for _, data := range []string{`{}`, `{"action":"other","client_payload":` + string(raw) + `}`, `{"action":"araihu-assets-released","client_payload":` + string(raw) + `,"inputs":` + string(raw) + `}`, `{"inputs":{"release":"v0.2.5","release":"v0.2.4"}}`} {
		if _, err := EventMetadata([]byte(data)); err == nil {
			t.Fatal("accepted invalid event")
		}
	}
}
