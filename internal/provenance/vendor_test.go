package provenance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/araihu/assets/internal/manifest"
)

func TestSyncDoesNotPublishHashMismatch(t *testing.T) {
	root := openTempRoot(t)
	err := Sync(context.Background(), fakeDoer{body: []byte("wrong")}, source("expected"), root)
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("Sync() error = %v, want checksum mismatch", err)
	}
	if _, err := root.Stat("16/solid/check.svg"); !os.IsNotExist(err) {
		t.Fatalf("published mismatched bytes: stat error = %v", err)
	}
}

func TestSyncRejectsRedirectAndOversize(t *testing.T) {
	root := openTempRoot(t)
	if err := Sync(context.Background(), fakeDoer{status: http.StatusFound}, source("expected"), root); err == nil || !strings.Contains(err.Error(), "unexpected HTTP status") {
		t.Fatalf("redirect Sync() error = %v, want unexpected HTTP status", err)
	}
	tooLarge := bytes.Repeat([]byte("a"), maxResponseBytes+1)
	if err := Sync(context.Background(), fakeDoer{body: tooLarge}, source(string(tooLarge)), root); err == nil || !strings.Contains(err.Error(), "response exceeds") {
		t.Fatalf("oversize Sync() error = %v, want response exceeds", err)
	}
}

func TestNewHTTPClientDisablesRedirectFollowing(t *testing.T) {
	client := NewHTTPClient()
	if client.Timeout != 15*time.Second {
		t.Fatalf("client timeout = %s, want 15s", client.Timeout)
	}
	if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect() error = %v, want ErrUseLastResponse", err)
	}
}

func TestSyncKeepsByteIdenticalExistingFile(t *testing.T) {
	root := openTempRoot(t)
	contents := []byte("already verified")
	if err := root.MkdirAll("16/solid", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := root.WriteFile("16/solid/check.svg", contents, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Sync(context.Background(), fakeDoer{body: []byte("different")}, source(string(contents)), root); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	got, err := root.ReadFile("16/solid/check.svg")
	if err != nil || !bytes.Equal(got, contents) {
		t.Fatalf("existing file changed: got %q, err %v", got, err)
	}
}

func TestSyncRejectsUnsafeSource(t *testing.T) {
	root := openTempRoot(t)
	s := source("expected")
	s.BaseURL = "https://example.invalid/"
	if err := Sync(context.Background(), fakeDoer{}, s, root); err == nil || !strings.Contains(err.Error(), "immutable Heroicons") {
		t.Fatalf("Sync() error = %v, want immutable Heroicons", err)
	}
}

func TestBuildUIRejectsMissingVendoredSVG(t *testing.T) {
	ui := lockedUI(t)
	root := copiedVendorRoot(t)
	if err := os.Remove(filepath.Join(root, heroiconsVendor, "16/solid/check.svg")); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildUI(os.DirFS(root), ui); err == nil || !strings.Contains(err.Error(), "missing vendored SVG") {
		t.Fatalf("BuildUI() error = %v, want missing vendored SVG", err)
	}
}

func TestBuildUIProducesDeterministicUIArtifactsOffline(t *testing.T) {
	ui := lockedUI(t)
	first, err := BuildUI(os.DirFS("../.."), ui)
	if err != nil {
		t.Fatalf("BuildUI(first) error = %v", err)
	}
	second, err := BuildUI(os.DirFS("../.."), ui)
	if err != nil {
		t.Fatalf("BuildUI(second) error = %v", err)
	}
	if len(first.Assets) != 67 || len(first.Files) != 70 {
		t.Fatalf("BuildUI() assets/files = %d/%d, want 67/70", len(first.Assets), len(first.Files))
	}
	if !bytes.Equal(first.Files["icons/ui/sprite.svg"], second.Files["icons/ui/sprite.svg"]) || !bytes.Equal(first.Files["icons/ui/heroicons/provenance.json"], second.Files["icons/ui/heroicons/provenance.json"]) {
		t.Fatal("BuildUI() output is not deterministic")
	}
	icon := first.Files["icons/ui/heroicons/16-solid-check.svg"]
	if !bytes.Contains(icon, []byte(`fill="currentColor"`)) || bytes.Contains(icon, []byte(`width="16"`)) {
		t.Fatalf("normalized icon = %s", icon)
	}
	if !bytes.Contains(first.Files["icons/ui/sprite.svg"], []byte(`id="hi-16-solid-check"`)) {
		t.Fatal("sprite omits hi-16-solid-check")
	}
	if got := string(first.Files["licenses/heroicons-MIT.txt"]); got != heroiconsMIT {
		t.Fatalf("MIT text differs:\n%s", got)
	}
	if !bytes.Contains(first.Files["icons/ui/heroicons/provenance.json"], []byte(`"path": "16/solid/check.svg"`)) {
		t.Fatal("provenance JSON does not use stable public field names")
	}
}

func TestBuildUIRejectsExtraVendoredSVG(t *testing.T) {
	ui := lockedUI(t)
	root := copiedVendorRoot(t)
	extra := filepath.Join(root, heroiconsVendor, "16/solid/unexpected.svg")
	if err := os.WriteFile(extra, []byte(`<svg viewBox="0 0 16 16"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := BuildUI(os.DirFS(root), ui); err == nil || !strings.Contains(err.Error(), "extra vendored SVG") {
		t.Fatalf("BuildUI() error = %v, want extra vendored SVG", err)
	}
}

func copiedVendorRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	destination := filepath.Join(root, heroiconsVendor)
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(destination, os.DirFS(filepath.Join("../..", heroiconsVendor))); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestLiveSyncLockedHeroicons(t *testing.T) {
	if os.Getenv("ARAIHU_LIVE_SYNC") != "1" {
		t.Skip("set ARAIHU_LIVE_SYNC=1 to vendor immutable Heroicons")
	}
	ui := lockedUI(t)
	if err := os.MkdirAll(filepath.Join("../..", heroiconsVendor), 0o755); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(filepath.Join("../..", heroiconsVendor))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := Sync(context.Background(), NewHTTPClient(), ui.Sources[0], root); err != nil {
		t.Fatal(err)
	}
}

func TestGenerateTrackedUIArtifacts(t *testing.T) {
	if os.Getenv("ARAIHU_WRITE_UI_ARTIFACTS") != "1" {
		t.Skip("set ARAIHU_WRITE_UI_ARTIFACTS=1 to write tracked UI artifacts")
	}
	ui := lockedUI(t)
	result, err := BuildUI(os.DirFS("../.."), ui)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range result.Files {
		output := filepath.Join("../..", "dist", filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	data, err := json.MarshalIndent(provenanceDocument(ui.Sources[0]), "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("../..", heroiconsVendor, "provenance.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func lockedUI(t *testing.T) manifest.UI {
	t.Helper()
	ui, err := manifest.LoadUI(os.DirFS("../.."), "manifests/icons-ui.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return ui
}

type fakeDoer struct {
	body   []byte
	status int
}

func (d fakeDoer) Do(*http.Request) (*http.Response, error) {
	status := d.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{StatusCode: status, Body: io.NopCloser(bytes.NewReader(d.body)), Header: make(http.Header)}, nil
}

func source(body string) manifest.UISource {
	hash := sha256.Sum256([]byte(body))
	return manifest.UISource{
		Name:    "heroicons",
		Alias:   "hi",
		Version: "v2.2.0",
		Commit:  heroiconsCommit,
		BaseURL: heroiconsBaseURL,
		License: "MIT",
		Icons: []manifest.UIIcon{{
			Path: "16/solid/check.svg", SHA256: fmt.Sprintf("%x", hash),
		}},
	}
}

func openTempRoot(t *testing.T) *os.Root {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = root.Close() })
	return root
}
