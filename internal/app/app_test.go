package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/araihu/assets/internal/releasemeta"
)

func TestThemeInputsCaptureStylesheets(t *testing.T) {
	manifest, css, err := themeInputs(fstest.MapFS{
		"manifests/themes.yaml": {Data: []byte("schema_version: 1\ntoken_contract: goshtoso-theme-v1\nthemes:\n  - id: araihu\n    css_path: themes/araihu.css\n")},
		"themes/araihu.css":     {Data: []byte("[data-theme=\"araihu\"] {}\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Themes[0].ID; got != "araihu" {
		t.Fatalf("theme ID = %q", got)
	}
	if got := string(css["themes/araihu.css"]); got != "[data-theme=\"araihu\"] {}\n" {
		t.Fatalf("captured CSS = %q", got)
	}
}

func TestRunRejectsClientCodegen(t *testing.T) {
	for _, args := range [][]string{{"codegen", "go"}, {"generate", "--language", "go"}} {
		err := Run(context.Background(), Dependencies{}, args, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "unknown command") {
			t.Fatalf("Run(%q) error = %v, want unknown command", args, err)
		}
		if !IsUsage(err) {
			t.Fatalf("Run(%q) error = %v, want usage error", args, err)
		}
	}
}

func TestExportRequiresOutput(t *testing.T) {
	err := Run(context.Background(), Dependencies{}, []string{"export"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "--output is required") {
		t.Fatalf("Run(export) error = %v, want required output", err)
	}
	if !IsUsage(err) {
		t.Fatalf("Run(export) error = %v, want usage error", err)
	}
}

func TestRunRejectsCommandExtraArgumentsAndUnknownFlags(t *testing.T) {
	for _, args := range [][]string{{"catalog", "extra"}, {"catalog", "--unknown"}} {
		var stderr bytes.Buffer
		err := Run(context.Background(), Dependencies{}, args, io.Discard, &stderr)
		if err == nil || !IsUsage(err) {
			t.Fatalf("Run(%q) error = %v, want usage error", args, err)
		}
	}
}

func TestRunHonorsCancelledContextBeforeWork(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := Run(ctx, Dependencies{}, []string{"build", "--offline"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "build: context canceled") {
		t.Fatalf("Run(cancelled build) error = %v", err)
	}
}

func TestThemesValidateAndCampaignResolveUseStrictOfflineInputs(t *testing.T) {
	repo := fixtureRepo(t)
	for _, args := range [][]string{{"themes", "validate"}, {"campaigns", "validate"}} {
		var stdout, stderr bytes.Buffer
		if err := Run(context.Background(), Dependencies{Repo: repo}, args, &stdout, &stderr); err != nil {
			t.Fatalf("Run(%q) error = %v", args, err)
		}
		if stdout.Len() == 0 || stderr.Len() != 0 {
			t.Fatalf("Run(%q) stdout=%q stderr=%q", args, stdout.String(), stderr.String())
		}
	}

	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "resolve", "--date", "2026-10-31"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"schemaVersion": 1`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestThemeAndCampaignCommandsRejectInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"themes"},
		{"themes", "validate", "extra"},
		{"campaigns"},
		{"campaigns", "unknown"},
		{"campaigns", "resolve"},
		{"campaigns", "resolve", "--date", "2026-2-03"},
		{"campaigns", "resolve", "--date", "2026-02-03", "extra"},
		{"campaigns", "publish", "--date", "2026-02-03"},
		{"campaigns", "publish", "--output", "out"},
		{"campaigns", "publish", "--date", "2026-02-03", "--output", "out", "extra"},
	} {
		var stderr bytes.Buffer
		err := Run(context.Background(), Dependencies{}, args, io.Discard, &stderr)
		if err == nil || !IsUsage(err) {
			t.Fatalf("Run(%q) error = %v, want usage error", args, err)
		}
	}
}

func TestCampaignPublishWritesOnlyChannelsAndAcceptsIdenticalOutput(t *testing.T) {
	repo := campaignFixtureRepo(t)
	output := filepath.Join(t.TempDir(), "channel-output")
	args := []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}
	for range 2 {
		if err := Run(context.Background(), Dependencies{Repo: repo}, args, io.Discard, io.Discard); err != nil {
			t.Fatalf("Run(%q) error = %v", args, err)
		}
	}
	var paths []string
	err := filepath.WalkDir(output, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(output, path)
		if err != nil {
			return err
		}
		paths = append(paths, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Join(paths, ",")
	want := "campaign/v1.js,releases/current.json,releases/default.json,releases/latest.json"
	if got != want {
		t.Fatalf("published paths = %q, want %q", got, want)
	}
}

func TestCampaignPublishPreflightsDifferentByteCollision(t *testing.T) {
	repo := campaignFixtureRepo(t)
	output := t.TempDir()
	name := filepath.Join(output, "releases", "current.json")
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("Run(publish) error = %v, want collision", err)
	}
	if got, err := os.ReadFile(name); err != nil || string(got) != "old" {
		t.Fatalf("collision target = %q, %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(output, "campaign", "v1.js")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("partial channel output = %v", err)
	}
}

func TestCampaignPublishCancellationPreservesReplacementOutput(t *testing.T) {
	repo := campaignFixtureRepo(t)
	parent := t.TempDir()
	output := filepath.Join(parent, "new-output")
	ctx, cancel := context.WithCancel(context.Background())
	campaignAfterOutputRootHook = func() {
		if err := os.Remove(output); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(output, 0o755); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
	t.Cleanup(func() { campaignAfterOutputRootHook = nil })
	err := Run(ctx, Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(publish) error = %v, want context canceled", err)
	}
	if info, err := os.Lstat(output); err != nil || !info.IsDir() {
		t.Fatalf("cancelled publish removed replacement output %q: %v, %v", output, info, err)
	}
}

func TestCampaignPublishPreservesUnownedOutputPath(t *testing.T) {
	repo := campaignFixtureRepo(t)
	output := filepath.Join(t.TempDir(), "unowned")
	if err := os.WriteFile(output, []byte("do not remove"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard)
	if err == nil {
		t.Fatal("Run(publish) accepted regular-file output root")
	}
	if got, err := os.ReadFile(output); err != nil || string(got) != "do not remove" {
		t.Fatalf("unowned output = %q, %v", got, err)
	}
}

func TestCampaignPublishRejectsOutputRootAndParentSymlinks(t *testing.T) {
	repo := campaignFixtureRepo(t)
	for _, fixture := range func() []struct{ output, outside string } {
		parent, outside := t.TempDir(), t.TempDir()
		rootLink := filepath.Join(parent, "root-link")
		if err := os.Symlink(outside, rootLink); err != nil {
			t.Fatal(err)
		}
		parentLink := filepath.Join(parent, "parent-link")
		if err := os.Symlink(outside, parentLink); err != nil {
			t.Fatal(err)
		}
		return []struct{ output, outside string }{{rootLink, outside}, {filepath.Join(parentLink, "child"), outside}}
	}() {
		err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", fixture.output}, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "symbolic-link") {
			t.Fatalf("Run(publish %q) error = %v, want symbolic-link rejection", fixture.output, err)
		}
		if entries, err := os.ReadDir(fixture.outside); err != nil || len(entries) != 0 {
			t.Fatalf("publish wrote through %q: %v, %v", fixture.output, entries, err)
		}
	}
}

func TestCampaignPublishPreservesReplacementSymlinkWithoutWritingOutside(t *testing.T) {
	repo := campaignFixtureRepo(t)
	parent, outside := t.TempDir(), t.TempDir()
	output := filepath.Join(parent, "new-output")
	campaignAfterOutputRootHook = func() {
		if err := os.Remove(output); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, output); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { campaignAfterOutputRootHook = nil })
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Run(publish) error = %v, want replacement rejection", err)
	}
	if info, err := os.Lstat(output); err != nil || info.Mode()&fs.ModeSymlink == 0 {
		t.Fatalf("replacement output = %v, %v", info, err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("publish wrote outside replacement root: %v, %v", entries, err)
	}
}

func TestCampaignPublishSeparatesLatestFromPromotedDefault(t *testing.T) {
	repo := promotedSnapshotFixture(t)
	latestRelease := fixtureRelease(t, repo)

	output := filepath.Join(t.TempDir(), "channels")
	if err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	for _, want := range []struct {
		name    string
		release string
	}{
		{name: "releases/latest.json", release: latestRelease},
		{name: "releases/default.json", release: "v0.0.9"},
		{name: "releases/current.json", release: "v0.0.9"},
	} {
		data, err := os.ReadFile(filepath.Join(output, filepath.FromSlash(want.name)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"release": "`+want.release+`"`) || !strings.Contains(string(data), `"source": "default"`) {
			t.Fatalf("%s = %s", want.name, data)
		}
	}
}

func TestCampaignPublishUsesLiveCampaignManifestForCurrent(t *testing.T) {
	repo := campaignFixtureRepo(t)
	manifestPath := filepath.Join(repo, "manifests", "campaigns.yaml")
	disabled, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}

	publishCurrent := func(name string) []byte {
		t.Helper()
		output := filepath.Join(t.TempDir(), name)
		if err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-08-01", "--output", output}, io.Discard, io.Discard); err != nil {
			t.Fatal(err)
		}
		current, err := os.ReadFile(filepath.Join(output, "releases", "current.json"))
		if err != nil {
			t.Fatal(err)
		}
		return current
	}

	disabledCurrent := publishCurrent("disabled")
	enabled := bytes.Replace(disabled, []byte("enabled: false"), []byte("enabled: true"), 1)
	if err := os.WriteFile(manifestPath, enabled, 0o644); err != nil {
		t.Fatal(err)
	}
	enabledCurrent := publishCurrent("enabled")
	if bytes.Equal(disabledCurrent, enabledCurrent) || !bytes.Contains(enabledCurrent, []byte(`"source": "campaign"`)) {
		t.Fatalf("live enable did not change current channel\ndisabled=%s\nenabled=%s", disabledCurrent, enabledCurrent)
	}

	edited := bytes.Replace(enabled, []byte("theme: araihu-signal-night"), []byte("theme: araihu"), 1)
	if err := os.WriteFile(manifestPath, edited, 0o644); err != nil {
		t.Fatal(err)
	}
	editedCurrent := publishCurrent("edited")
	if bytes.Equal(enabledCurrent, editedCurrent) || !bytes.Contains(editedCurrent, []byte(`"id": "araihu"`)) {
		t.Fatalf("live edit did not change current channel\nenabled=%s\nedited=%s", enabledCurrent, editedCurrent)
	}

	if err := os.WriteFile(manifestPath, disabled, 0o644); err != nil {
		t.Fatal(err)
	}
	disabledAgain := publishCurrent("disabled-again")
	if !bytes.Equal(disabledCurrent, disabledAgain) {
		t.Fatalf("live disable did not restore current channel\nfirst=%s\nagain=%s", disabledCurrent, disabledAgain)
	}
}

func TestCampaignPublishRejectsSymlinkedPromotedSnapshotMembers(t *testing.T) {
	for _, name := range []string{".", "release.json", "catalog.json", "themes.json", "campaigns.json"} {
		t.Run(name, func(t *testing.T) {
			repo := promotedSnapshotFixture(t)
			snapshot := filepath.Join(repo, "releases", "v0.0.9")
			if name == "." {
				backup := snapshot + "-backup"
				if err := os.Rename(snapshot, backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(backup, snapshot); err != nil {
					t.Fatal(err)
				}
			} else {
				member := filepath.Join(snapshot, name)
				backup := member + ".real"
				if err := os.Rename(member, backup); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Base(backup), member); err != nil {
					t.Fatal(err)
				}
			}
			err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", filepath.Join(t.TempDir(), "output")}, io.Discard, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "symbolic") {
				t.Fatalf("Run(publish) error = %v, want symbolic-link rejection", err)
			}
		})
	}
}

func TestCampaignPublishRejectsNonRegularPromotedSnapshotMember(t *testing.T) {
	repo := promotedSnapshotFixture(t)
	member := filepath.Join(repo, "releases", "v0.0.9", "catalog.json")
	if err := os.Remove(member); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(member, 0o755); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", filepath.Join(t.TempDir(), "output")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "regular") {
		t.Fatalf("Run(publish) error = %v, want non-regular rejection", err)
	}
}

func TestCampaignPublishRejectsSymlinkedLiveRuntime(t *testing.T) {
	repo := campaignFixtureRepo(t)
	runtime := filepath.Join(repo, "dist", "campaign", "v1.js")
	backup := runtime + ".real"
	if err := os.Rename(runtime, backup); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Base(backup), runtime); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", filepath.Join(t.TempDir(), "output")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "symbolic") {
		t.Fatalf("Run(publish) error = %v, want runtime symbolic-link rejection", err)
	}
}

func TestCampaignPublishRejectsPromotedSnapshotReplacementDuringOpen(t *testing.T) {
	repo := promotedSnapshotFixture(t)
	snapshot := filepath.Join(repo, "releases", "v0.0.9")
	seenCatalog := 0
	campaignAfterCapturedFileOpenHook = func(name string) {
		if name != "catalog.json" {
			return
		}
		seenCatalog++
		if seenCatalog != 2 {
			return
		}
		member := filepath.Join(snapshot, name)
		if err := os.Remove(member); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink("catalog.json.real", member); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { campaignAfterCapturedFileOpenHook = nil })
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", filepath.Join(t.TempDir(), "output")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Run(publish) error = %v, want replacement rejection", err)
	}
}

func TestCampaignPublishRejectsIntermediateReleaseDirectoryReplacementDuringOpen(t *testing.T) {
	repo := promotedSnapshotFixture(t)
	releases := filepath.Join(repo, "releases")
	outside := t.TempDir()
	managedRootAfterChildOpenHook = func(name string) {
		if name != "releases" {
			return
		}
		backup := releases + ".real"
		if err := os.Rename(releases, backup); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, releases); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { managedRootAfterChildOpenHook = nil })
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", filepath.Join(t.TempDir(), "output")}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "changed") {
		t.Fatalf("Run(publish) error = %v, want intermediate replacement rejection", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("publish read or wrote through replacement directory: %v, %v", entries, err)
	}
}

func TestCampaignPublishUsesOneCapturedSnapshot(t *testing.T) {
	repo := campaignFixtureRepo(t)
	release := fixtureRelease(t, repo)
	runtime, err := os.ReadFile(filepath.Join(repo, "dist", "campaign", "v1.js"))
	if err != nil {
		t.Fatal(err)
	}
	campaignAfterSnapshotHook = func() {
		if err := os.WriteFile(filepath.Join(repo, "dist", "campaign", "v1.js"), []byte("changed after capture"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "dist", "themes.json"), []byte(`{"schemaVersion":999}`), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { campaignAfterSnapshotHook = nil })
	output := filepath.Join(t.TempDir(), "channels")
	if err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-10-31", "--output", output}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(output, "campaign", "v1.js"))
	if err != nil || !bytes.Equal(got, runtime) {
		t.Fatalf("runtime = %q, %v", got, err)
	}
	for _, want := range []struct {
		name    string
		release string
	}{
		{name: "current.json", release: release},
		{name: "default.json", release: release},
		{name: "latest.json", release: release},
	} {
		data, err := os.ReadFile(filepath.Join(output, "releases", want.name))
		if err != nil || !strings.Contains(string(data), `"release": "`+want.release+`"`) {
			t.Fatalf("%s = %q, %v", want.name, data, err)
		}
	}
}

func TestCampaignPublishUsesPublishedLatestChannelAndPromotedRuntime(t *testing.T) {
	repo := campaignFixtureRepo(t)
	seed := filepath.Join(t.TempDir(), "seed")
	if err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-08-01", "--output", seed}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	publishedLatest, err := os.ReadFile(filepath.Join(seed, "releases", "latest.json"))
	if err != nil {
		t.Fatal(err)
	}
	publishedRuntime, err := os.ReadFile(filepath.Join(repo, "dist", "campaign", "v1.js"))
	if err != nil {
		t.Fatal(err)
	}

	publishedRoot := filepath.Join(repo, "releases", "latest")
	for _, name := range []string{"release.json", "catalog.json", "themes.json", "campaigns.json", "campaign/v1.js"} {
		data, err := os.ReadFile(filepath.Join(repo, "dist", filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(publishedRoot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(publishedRoot, "latest.json"), publishedLatest, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dist", "campaign", "v1.js"), []byte("untagged main runtime"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "published")
	if err := Run(context.Background(), Dependencies{Repo: repo}, []string{"campaigns", "publish", "--date", "2026-08-01", "--output", output}, io.Discard, io.Discard); err != nil {
		t.Fatal(err)
	}
	gotLatest, err := os.ReadFile(filepath.Join(output, "releases", "latest.json"))
	if err != nil || !bytes.Equal(gotLatest, publishedLatest) {
		t.Fatalf("latest = %q, %v", gotLatest, err)
	}
	gotRuntime, err := os.ReadFile(filepath.Join(output, "campaign", "v1.js"))
	if err != nil || !bytes.Equal(gotRuntime, publishedRuntime) {
		t.Fatalf("runtime = %q, %v", gotRuntime, err)
	}
}

func TestVendorRejectsSymlinkedManagedVersionDirectory(t *testing.T) {
	repo, outside := t.TempDir(), t.TempDir()
	copyManifest(t, repo, "icons-ui.yaml")
	managed := filepath.Join(repo, "vendor", "icons", "ui", "heroicons")
	if err := os.MkdirAll(managed, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(managed, "v2.2.0")); err != nil {
		t.Fatal(err)
	}
	err := Run(context.Background(), Dependencies{Repo: repo}, []string{"vendor"}, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "symbolic-link component") {
		t.Fatalf("Run(vendor) error = %v, want managed symlink rejection", err)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 0 {
		t.Fatalf("vendor wrote outside root: %v, %v", entries, err)
	}
}

func TestExportAndCatalogRejectSymlinkedDist(t *testing.T) {
	repo, outside, output := t.TempDir(), t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "catalog.json"), []byte(`{"not":"catalog"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "dist")); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"export", "--output", output}, {"catalog"}} {
		err := Run(context.Background(), Dependencies{Repo: repo}, args, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "symbolic-link component") {
			t.Fatalf("Run(%q) error = %v, want managed symlink rejection", args, err)
		}
	}
	if entries, err := os.ReadDir(output); err != nil || len(entries) != 0 {
		t.Fatalf("export used outside dist: %v, %v", entries, err)
	}
}

func TestExportCancellationAfterEnumerationLeavesNewOutputAbsent(t *testing.T) {
	repo, outputParent := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dist", "release.txt"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outputParent, "new-output")
	ctx, cancel := context.WithCancel(context.Background())
	exportAfterEnumerationHook = cancel
	t.Cleanup(func() { exportAfterEnumerationHook = nil })

	err := Run(ctx, Dependencies{Repo: repo}, []string{"export", "--output", output}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(export) error = %v, want context canceled", err)
	}
	if _, err := os.Lstat(output); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("cancelled export created output %q: %v", output, err)
	}
}

func TestExportCancellationDoesNotRemoveReplacementOutputDirectory(t *testing.T) {
	repo, outputParent := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, "dist"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "dist", "release.txt"), []byte("release"), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(outputParent, "new-output")
	ctx, cancel := context.WithCancel(context.Background())
	exportAfterOutputRootHook = func() {
		if err := os.Remove(output); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(output, 0o755); err != nil {
			t.Fatal(err)
		}
		cancel()
	}
	t.Cleanup(func() { exportAfterOutputRootHook = nil })

	err := Run(ctx, Dependencies{Repo: repo}, []string{"export", "--output", output}, io.Discard, io.Discard)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Run(export) error = %v, want context canceled", err)
	}
	if info, err := os.Lstat(output); err != nil || !info.IsDir() {
		t.Fatalf("cancelled export removed replacement output %q: %v, %v", output, info, err)
	}
}

func copyManifest(t *testing.T, repo, name string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "manifests", name))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "manifests", name), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixtureRepo(t *testing.T) string {
	t.Helper()
	repo, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func campaignFixtureRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, name := range []string{
		"manifests/default.yaml",
		"manifests/campaigns.yaml",
		"dist/catalog.json",
		"dist/themes.json",
		"dist/campaigns.json",
		"dist/release.json",
		"dist/campaign/v1.js",
	} {
		data, err := os.ReadFile(filepath.Join(fixtureRepo(t), filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(repo, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(output, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	release := fixtureRelease(t, repo)
	defaultManifest := []byte("schema_version: 1\nrelease: " + release + "\ntheme: araihu\n")
	if err := os.WriteFile(filepath.Join(repo, "manifests", "default.yaml"), defaultManifest, 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func promotedSnapshotFixture(t *testing.T) string {
	t.Helper()
	repo := campaignFixtureRepo(t)
	latestRelease := fixtureRelease(t, repo)
	defaultManifest := filepath.Join(repo, "manifests", "default.yaml")
	if err := os.WriteFile(defaultManifest, []byte("schema_version: 1\nrelease: v0.0.9\ntheme: araihu\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	files := fstest.MapFS{}
	snapshot := filepath.Join(repo, "releases", "v0.0.9")
	if err := os.MkdirAll(snapshot, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"catalog.json", "themes.json", "campaigns.json", "campaign/v1.js"} {
		data, err := os.ReadFile(filepath.Join(repo, "dist", name))
		if err != nil {
			t.Fatal(err)
		}
		data = bytes.ReplaceAll(data, []byte(latestRelease), []byte("v0.0.9"))
		target := filepath.Join(snapshot, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if name == "catalog.json" {
			if err := os.WriteFile(filepath.Join(snapshot, name+".real"), data, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		files[name] = &fstest.MapFile{Data: data}
	}
	document, err := releasemeta.Build(releasemeta.Input{Release: "v0.0.9", IdentityRevision: 11, RuntimeVersion: 1, Files: files})
	if err != nil {
		t.Fatal(err)
	}
	releaseJSON, err := releasemeta.Encode(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(snapshot, "release.json"), releaseJSON, 0o644); err != nil {
		t.Fatal(err)
	}
	return repo
}

func fixtureRelease(t *testing.T, repo string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, "dist", "release.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document releasemeta.Document
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document.Release
}
