# Assets Control Plane Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Extend Arai Hû Assets with strict multi-theme releases, date-bounded campaign resolution, a constrained deferred browser runtime, and deterministic immutable/channel publication bundles.

**Architecture:** New focused Go packages own theme parsing, campaign parsing, channel resolution, and release metadata. Existing build/catalog packages assemble their deterministic outputs without changing catalog schema v1. A small standalone JavaScript runtime consumes the resolved channel; GitHub Actions invokes the credential-free CLI and publishes artifacts or dispatches Ahairu.

**Tech Stack:** Go 1.26.5, `io/fs`, strict `yaml.v3`, `encoding/json`, vanilla JavaScript, Node built-in test runner, GitHub Actions.

## Global Constraints

- Preserve `dist/catalog.json` schema v1 and all existing canonical-name semantics.
- Additive icons are patch-compatible; removal, rename, or semantic mutation is not.
- Campaign dates are UTC `YYYY-MM-DD`, inclusive at both ends, with no time fields.
- Reject overlapping enabled campaigns and every unresolved theme/icon/brand reference.
- Generated output is deterministic: sorted keys/lists where applicable, two-space JSON, one final newline, no meaningless timestamps.
- `themes/araihu.css` becomes managed release output; source remains outside `dist`.
- Only `vendor` may access the network. Build, verify, theme, campaign, and publish commands are offline.
- CLI remains credential-free and writes only below an explicitly owned output directory.
- Runtime may mutate only documented root attributes, stylesheets, image hooks, toggle hooks, and lifecycle events.
- Use `go 1.26.0` plus `toolchain go1.26.5` unless repository policy is intentionally changed in a dedicated commit; CI must execute exactly Go 1.26.5.

---

### Task 1: Add strict theme manifest and release catalog

**Files:**
- Create: `manifests/themes.yaml`
- Create: `themes/araihu-signal-night.css`
- Create: `internal/themes/themes.go`
- Create: `internal/themes/themes_test.go`
- Create: `docs/themes-schema.md`
- Modify: `internal/build/build.go`
- Modify: `internal/build/build_test.go`

**Interfaces:**
- Produces: `themes.Load(fsys fs.FS, name string) (themes.Manifest, error)`
- Produces: `themes.Manifest.Validate() error`
- Produces: `themes.Manifest.Catalog(release string) (themes.Catalog, error)`
- Produces: `themes.Encode(catalog themes.Catalog) ([]byte, error)`
- `themes.Theme` fields: `ID`, `CSSPath`, `TokenContract`, `SHA256`.

- [ ] **Step 1: Write failing strict-decoder and validation tests**

```go
func TestLoadRejectsUnknownField(t *testing.T) {
	_, err := Load(fstest.MapFS{"themes.yaml": {Data: []byte("schema_version: 1\nthemes: []\nextra: true\n")}}, "themes.yaml")
	if err == nil || !strings.Contains(err.Error(), "field extra not found") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestValidateRejectsDuplicateThemeAndTraversal(t *testing.T) {
	manifest := Manifest{SchemaVersion: 1, TokenContract: "goshtoso-theme-v1", Themes: []Theme{
		{ID: "araihu", CSSPath: "themes/araihu.css"},
		{ID: "araihu", CSSPath: "../secret.css"},
	}}
	if err := manifest.Validate(); err == nil {
		t.Fatal("Validate() succeeded")
	}
}
```

- [ ] **Step 2: Run the focused test and prove the package is absent**

Run: `go test ./internal/themes -count=1`

Expected: FAIL because `internal/themes` does not exist.

- [ ] **Step 3: Implement the strict manifest and deterministic catalog**

```go
type Manifest struct {
	SchemaVersion int     `yaml:"schema_version"`
	TokenContract string  `yaml:"token_contract"`
	Themes        []Theme `yaml:"themes"`
}

type Theme struct {
	ID      string `yaml:"id" json:"id"`
	CSSPath string `yaml:"css_path" json:"cssPath"`
}

type Catalog struct {
	SchemaVersion int            `json:"schemaVersion"`
	Release       string         `json:"release"`
	TokenContract string         `json:"tokenContract"`
	Themes        []CatalogTheme `json:"themes"`
}
```

Use `yaml.Decoder.KnownFields(true)`, reject multiple documents, validate
lower-kebab IDs and `fs.ValidPath`, require `.css`, sort catalog themes by ID,
and hash the generated CSS bytes during build assembly.

- [ ] **Step 4: Declare the baseline Arai Hû theme**

```yaml
schema_version: 1
token_contract: goshtoso-theme-v1
themes:
  - id: araihu
    css_path: themes/araihu.css
  - id: araihu-signal-night
    css_path: themes/araihu-signal-night.css
```

The second theme is a restrained internal campaign proof using the approved
midnight, storm, paper, and lime-signal palette. It must implement the same
token contract, pass contrast checks, and remain inactive by default.

- [ ] **Step 5: Integrate `themes.json` and theme CSS into deterministic build output**

Add `themes.json` and `themes/araihu.css` to `generatedPaths`. Load the source
CSS once into `build.Inputs`; compute its hash from captured bytes; never reread
the live path during publication.

- [ ] **Step 6: Run focused and build tests**

Run: `go test ./internal/themes ./internal/build -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add manifests/themes.yaml internal/themes docs/themes-schema.md internal/build
git commit -m "feat(themes): publish strict theme catalog"
```

### Task 2: Add strict date-only campaign calendar

**Files:**
- Create: `manifests/campaigns.yaml`
- Create: `internal/campaigns/campaigns.go`
- Create: `internal/campaigns/campaigns_test.go`
- Create: `docs/campaigns-schema.md`

**Interfaces:**
- Produces: `campaigns.Load(fsys fs.FS, name string) (campaigns.Manifest, error)`
- Produces: `campaigns.Manifest.Validate() error`
- Produces: `campaigns.Date.Parse(string) (campaigns.Date, error)` through `campaigns.ParseDate(string)`
- Produces: `campaigns.Manifest.Active(date campaigns.Date) (*campaigns.Campaign, error)`
- `Campaign` owns `ID`, `Enabled`, `StartsOn`, `EndsOn`, `Theme`, `Toggle`, and `Brand`.

- [ ] **Step 1: Write failing boundary and overlap tests**

```go
func TestActiveUsesInclusiveUTCDateRange(t *testing.T) {
	m := Manifest{SchemaVersion: 1, Campaigns: []Campaign{{
		ID: "halloween-2026", Enabled: true,
		StartsOn: mustDate(t, "2026-10-01"), EndsOn: mustDate(t, "2026-10-31"),
		Theme: "araihu-halloween",
	}}}
	for _, raw := range []string{"2026-10-01", "2026-10-31"} {
		active, err := m.Active(mustDate(t, raw))
		if err != nil || active == nil || active.ID != "halloween-2026" {
			t.Fatalf("Active(%s) = %#v, %v", raw, active, err)
		}
	}
}

func TestValidateRejectsEnabledOverlap(t *testing.T) {
	m := fixtureManifest("2026-10-01", "2026-10-31", "2026-10-31", "2026-11-02")
	if err := m.Validate(); err == nil || !strings.Contains(err.Error(), "overlap") {
		t.Fatalf("Validate() error = %v", err)
	}
}
```

- [ ] **Step 2: Run the focused test and prove the package is absent**

Run: `go test ./internal/campaigns -count=1`

Expected: FAIL because `internal/campaigns` does not exist.

- [ ] **Step 3: Implement a JSON/YAML date type that rejects time values**

```go
const dateLayout = "2006-01-02"

type Date struct{ time.Time }

func ParseDate(raw string) (Date, error) {
	parsed, err := time.Parse(dateLayout, raw)
	if err != nil || parsed.Format(dateLayout) != raw {
		return Date{}, fmt.Errorf("campaign date %q must use YYYY-MM-DD", raw)
	}
	return Date{Time: parsed.UTC()}, nil
}
```

Implement `UnmarshalYAML`, `MarshalJSON`, and `String` using the exact layout.

- [ ] **Step 4: Implement strict campaign types and overlap validation**

```go
type IconRef struct {
	Asset string `yaml:"asset" json:"asset"`
	Mode  string `yaml:"mode" json:"mode"`
}

type Toggle struct {
	EnabledIcon  IconRef `yaml:"enabled_icon" json:"enabledIcon"`
	DisabledIcon IconRef `yaml:"disabled_icon" json:"disabledIcon"`
}

type Brand struct {
	Logo string `yaml:"logo" json:"logo"`
	Icon string `yaml:"icon" json:"icon"`
}

type Campaign struct {
	ID       string `yaml:"id" json:"id"`
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	StartsOn Date   `yaml:"starts_on" json:"startsOn"`
	EndsOn   Date   `yaml:"ends_on" json:"endsOn"`
	Theme    string `yaml:"theme" json:"theme"`
	Toggle   Toggle `yaml:"toggle" json:"toggle"`
	Brand    Brand  `yaml:"brand" json:"brand"`
}
```

Accept modes `asset` and `sprite`. Require `Asset` to be a canonical catalog
name; the resolver derives the immutable URL and exact catalog sprite symbol.
Reject duplicate IDs, inverted ranges, control characters, unknown fields, and
overlap among enabled campaigns. Disabled campaigns remain fully validated.

- [ ] **Step 5: Add an empty, valid initial calendar**

```yaml
schema_version: 1
campaigns:
  - id: signal-night-proof-2026
    enabled: false
    starts_on: 2026-08-01
    ends_on: 2026-08-02
    theme: araihu-signal-night
    toggle:
      enabled_icon:
        asset: ui-hi-16-solid-sparkles
        mode: sprite
      disabled_icon:
        asset: ui-hi-16-solid-moon
        mode: sprite
    brand:
      logo: araihu-logo-tinted-transparent-optical
      icon: araihu-icon-tinted-transparent-optical
```

- [ ] **Step 6: Run focused tests**

Run: `go test ./internal/campaigns -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add manifests/campaigns.yaml internal/campaigns docs/campaigns-schema.md
git commit -m "feat(campaigns): validate date-only calendar"
```

### Task 3: Add default promotion and resolved channel documents

**Files:**
- Create: `manifests/default.yaml`
- Create: `internal/channels/channels.go`
- Create: `internal/channels/channels_test.go`
- Modify: `docs/campaigns-schema.md`

**Interfaces:**
- Produces: `channels.LoadDefault(fsys fs.FS, name string) (channels.Default, error)`
- Produces: `channels.Resolve(input channels.Input) (channels.Document, error)`
- Produces: `channels.Encode(document channels.Document) ([]byte, error)`
- Consumes: `catalog.Catalog`, `themes.Catalog`, `campaigns.Manifest`.

- [ ] **Step 1: Write failing precedence and reference tests**

```go
func TestResolveUsesCampaignThenDefault(t *testing.T) {
	input := fixtureInput(t)
	active, err := Resolve(input.withDate("2026-10-31"))
	if err != nil || active.Source != "campaign" || active.Campaign.ID != "halloween-2026" {
		t.Fatalf("active = %#v, %v", active, err)
	}
	baseline, err := Resolve(input.withDate("2026-11-01"))
	if err != nil || baseline.Source != "default" || baseline.Theme.ID != "araihu" {
		t.Fatalf("baseline = %#v, %v", baseline, err)
	}
}

func TestResolveRejectsMissingCatalogReference(t *testing.T) {
	input := fixtureInput(t)
	input.Campaigns.Campaigns[0].Brand.Logo = "missing-logo"
	if _, err := Resolve(input); err == nil || !strings.Contains(err.Error(), "missing-logo") {
		t.Fatalf("Resolve() error = %v", err)
	}
}
```

- [ ] **Step 2: Run the focused test and prove the package is absent**

Run: `go test ./internal/channels -count=1`

Expected: FAIL because `internal/channels` does not exist.

- [ ] **Step 3: Implement strict promotion input and resolved document**

```go
type Default struct {
	SchemaVersion int    `yaml:"schema_version"`
	Release       string `yaml:"release"`
	Theme         string `yaml:"theme"`
}

type Input struct {
	Date       campaigns.Date
	Default    Default
	Catalog    catalog.Catalog
	Themes     themes.Catalog
	Campaigns  campaigns.Manifest
	PublicRoot string
}

type Document struct {
	SchemaVersion  int               `json:"schemaVersion"`
	RuntimeVersion int               `json:"runtimeVersion"`
	Release        string            `json:"release"`
	Source         string            `json:"source"`
	Theme          ResolvedTheme     `json:"theme"`
	Campaign       *ResolvedCampaign `json:"campaign,omitempty"`
	Digest         string            `json:"digest"`
}
```

Resolve canonical names through the catalog. Emit absolute paths on the
configured public channel origin below `/assets/releases/<release>/`. Resolve
theme CSS similarly. Sort
and encode before computing the digest; compute the digest over the document
with `Digest` empty, then encode the completed document.

- [ ] **Step 4: Declare the initial default promotion**

```yaml
schema_version: 1
release: v0.1.0
theme: araihu
```

- [ ] **Step 5: Test deterministic bytes and unchanged digest**

Run: `go test ./internal/channels -count=20`

Expected: PASS with byte-identical output on every iteration.

- [ ] **Step 6: Commit**

```bash
git add manifests/default.yaml internal/channels docs/campaigns-schema.md
git commit -m "feat(channels): resolve default and campaign state"
```

### Task 4: Add release inventory and immutable public bundle

**Files:**
- Create: `internal/releasemeta/release.go`
- Create: `internal/releasemeta/release_test.go`
- Create: `docs/release-schema.md`
- Modify: `internal/build/build.go`
- Modify: `internal/build/build_test.go`
- Modify: `internal/release/archive.go`
- Modify: `internal/release/archive_test.go`

**Interfaces:**
- Produces: `releasemeta.Build(input releasemeta.Input) (releasemeta.Document, error)`
- Produces: `releasemeta.Encode(document releasemeta.Document) ([]byte, error)`
- Produces: `release.PublicBundle(ctx context.Context, destination *os.Root, releaseID string, source fs.FS, paths []string) error`.

- [ ] **Step 1: Write failing inventory and collision tests**

```go
func TestBuildInventoriesFilesAndDocumentHashes(t *testing.T) {
	doc, err := Build(Input{Release: "v0.1.1", IdentityRevision: 11, RuntimeVersion: 1, Files: fixtureFiles()})
	if err != nil {
		t.Fatal(err)
	}
	if doc.Files[0].Path != "catalog.json" || len(doc.Files[0].SHA256) != 64 {
		t.Fatalf("files = %#v", doc.Files)
	}
}

func TestPublicBundleRejectsDifferentByteCollision(t *testing.T) {
	err := assembleFixture(t, map[string][]byte{"releases/v0.1.1/catalog.json": []byte("old")}, map[string][]byte{"releases/v0.1.1/catalog.json": []byte("new")})
	if err == nil || !strings.Contains(err.Error(), "collision") {
		t.Fatalf("assemble error = %v", err)
	}
}
```

- [ ] **Step 2: Run focused tests and prove the package/behavior is absent**

Run: `go test ./internal/releasemeta ./internal/release -count=1`

Expected: FAIL.

- [ ] **Step 3: Implement sorted release inventory**

```go
type File struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

type Document struct {
	SchemaVersion    int    `json:"schemaVersion"`
	Release          string `json:"release"`
	IdentityRevision int    `json:"identityRevision"`
	RuntimeVersion   int    `json:"runtimeVersion"`
	CatalogSHA256    string `json:"catalogSha256"`
	ThemesSHA256     string `json:"themesSha256"`
	CampaignsSHA256  string `json:"campaignsSha256"`
	Files            []File `json:"files"`
}
```

Require valid SemVer tag syntax, exact hashes for the three documents, sorted
unique paths, no `release.json` self-hash, no symlinks, and only regular files.

- [ ] **Step 4: Wrap immutable archive contents at the public release root**

The immutable public bundle for one release must contain:

```text
releases/v0.1.1/<release files>
```

Assembly accepts existing identical files and rejects different-byte
collisions. Ahairu CI owns cumulative assembly: it downloads every tagged Assets
release, verifies each archive, and combines the single-release public bundles
with the small channel bundle.

- [ ] **Step 5: Integrate `release.json`, `campaigns.json`, and updated checksums into `dist`**

Capture all input bytes before writing. Generate `catalog.json`, `themes.json`,
and `campaigns.json`; then build `release.json`; then build `checksums.txt` and
archives. Explicitly test the dependency order.

- [ ] **Step 6: Run focused and reproducibility tests**

Run: `go test ./internal/releasemeta ./internal/release ./internal/build -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/releasemeta internal/release internal/build docs/release-schema.md
git commit -m "feat(release): publish cumulative immutable inventory"
```

### Task 5: Implement the constrained deferred campaign runtime

**Files:**
- Create: `runtime/campaign/v1.js`
- Create: `runtime/campaign/v1.test.js`
- Create: `docs/campaign-runtime-v1.md`
- Modify: `internal/build/build.go`
- Modify: `internal/build/build_test.go`

**Interfaces:**
- Consumes: resolved channel document schema v1 with `runtimeVersion: 1`.
- Consumes DOM hooks: `[data-asset-brand="logo"]`, `[data-asset-brand="icon"]`, `[data-campaign-toggle]`, and `[data-campaign-toggle-icon]`.
- Reads only namespaced opt-out key `araihu.assets.campaign.v1.optout.<campaign-id>`.
- Dispatches: `araihu:campaign:before-apply`, `araihu:campaign:applied`, `araihu:campaign:restored`, and `araihu:campaign:error`.

- [ ] **Step 1: Write failing Node tests around a fake DOM/fetch boundary**

```js
test("explicit preference prevents campaign mutation", async () => {
  const fixture = runtimeFixture({ themeSource: "preference" });
  await fixture.start(activeCampaign);
  assert.equal(fixture.root.dataset.theme, "minimal");
  assert.equal(fixture.brandLogo.src, fixture.baselineLogo);
});

test("theme and images finish before atomic mutation", async () => {
  const fixture = runtimeFixture({ themeSource: "default" });
  const pending = fixture.start(activeCampaign);
  assert.deepEqual(fixture.mutations, []);
  fixture.resolveTheme();
  fixture.resolveImages();
  await pending;
  assert.deepEqual(fixture.mutations.at(-1), { theme: "araihu-halloween", campaign: "halloween-2026" });
});
```

- [ ] **Step 2: Run tests and prove the runtime is absent**

Run: `node --test runtime/campaign/v1.test.js`

Expected: FAIL because the runtime does not exist.

- [ ] **Step 3: Implement one IIFE with injected internal boundaries**

```js
(function (window, document) {
  "use strict";
  var VERSION = 1;
  var root = document.documentElement;
  var script = document.currentScript;
  var channelURL = (script && script.dataset.channel) || "/assets/releases/current";

  async function refresh() {
    // Fetch, validate schema/runtime, honor source and opt-out, preload,
    // then commit one bounded DOM mutation.
  }

  window.AraiHuCampaign = Object.freeze({ version: VERSION, refresh: refresh });
  refresh().catch(function () { dispatchError("refresh-failed"); });
})(window, document);
```

Keep validation explicit. Resolve the configured channel URL first, require
HTTPS outside local development, and accept assets only when their origin equals
the channel origin and their path begins `/assets/releases/`. Ahairu itself is
same-origin; enrolled Arai Hû subdomains use anonymous CORS from the canonical
asset origin. Do not pass server-provided strings to `innerHTML`.
Construct SVG elements through DOM APIs and copy only the selected validated
`symbol` children for sprite icons.

- [ ] **Step 4: Implement preference and opt-out behavior**

If `data-theme-source="preference"`, show no campaign mutation. If the active
campaign key is opted out, set `data-theme-source="campaign-opt-out"` and keep
the baseline. On toggle after activation, restore captured theme/source and
brand URLs, persist `"1"`, and dispatch `araihu:campaign:restored`.

Observe only the root `data-theme-source` attribute. If an enrolled application
changes it to `preference`, restore campaign-owned brand substitutions without
overwriting the newly selected theme. If it changes back to `default`, rerun
eligibility and reapply the active campaign unless its campaign-specific opt-out
key remains set.

- [ ] **Step 5: Test failures, direct icons, sprites, reload, expiry, and reduced motion hooks**

Run: `node --test runtime/campaign/v1.test.js`

Expected: PASS. Tests must prove failed fetch/CSS/image/sprite operations leave
the DOM unchanged and event details contain stable codes, not thrown objects.

- [ ] **Step 6: Add runtime bytes to generated output**

Copy the captured runtime into `dist/campaign/v1.js`; include it in release
inventory and checksums. A byte change to this entrypoint requires coordinated
SRI updates in consumers.

- [ ] **Step 7: Commit**

```bash
git add runtime/campaign docs/campaign-runtime-v1.md internal/build
git commit -m "feat(runtime): apply constrained seasonal campaigns"
```

### Task 6: Expose deterministic CLI commands

**Files:**
- Modify: `internal/app/app.go`
- Modify: `internal/app/app_test.go`
- Modify: `cmd/araihu-assets/main_test.go`
- Modify: `Makefile`
- Modify: `README.md`

**Interfaces:**
- Produces exact commands:
  - `araihu-assets themes validate`
  - `araihu-assets campaigns validate`
  - `araihu-assets campaigns resolve --date YYYY-MM-DD`
  - `araihu-assets campaigns publish --date YYYY-MM-DD --output <directory>`

- [ ] **Step 1: Write failing dispatch, usage, and exit-code tests**

```go
func TestCampaignResolvePrintsOneDocument(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(context.Background(), Dependencies{Repo: fixtureRepo(t)}, []string{"campaigns", "resolve", "--date", "2026-10-31"}, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), `"schemaVersion": 1`) || stderr.Len() != 0 {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}
```

Add tests for missing/invalid dates, extra arguments, missing output, existing
different-byte collision, existing identical output, cancellation, and an
unowned output path.

- [ ] **Step 2: Run focused tests and prove dispatch is absent**

Run: `go test ./internal/app ./cmd/araihu-assets -count=1`

Expected: FAIL with unknown command or missing fixture behavior.

- [ ] **Step 3: Add bounded subcommand dispatch**

```go
case "themes":
	return runThemes(ctx, deps, args[1:], stdout, stderr)
case "campaigns":
	return runCampaigns(ctx, deps, args[1:], stdout, stderr)
```

Reuse `openRepository`, rooted filesystem helpers, context checks, usage errors,
and collision-safe output ownership. `resolve` writes JSON to stdout only.
`publish` writes only `releases/latest.json`, `releases/default.json`,
`releases/current.json`, and `campaign/v1.js` below the caller's new output
root. It does not rebuild immutable release history.

- [ ] **Step 4: Add Make targets**

```make
.PHONY: themes-check campaigns-check

themes-check:
	go run ./cmd/araihu-assets themes validate

campaigns-check:
	go run ./cmd/araihu-assets campaigns validate
	go run ./cmd/araihu-assets campaigns resolve --date 2026-10-31 >/dev/null
```

- [ ] **Step 5: Run CLI and full Go tests**

Run: `make themes-check campaigns-check && go test ./... -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/app cmd/araihu-assets Makefile README.md
git commit -m "feat(cli): validate and publish campaign channels"
```

### Task 7: Add guarded release and campaign workflows

**Files:**
- Create: `.github/workflows/release.yml`
- Create: `.github/workflows/campaigns.yml`
- Create: `scripts/check-release-workflows.sh`
- Create: `scripts/check-release-workflows_test.sh`
- Modify: `.github/workflows/ci.yml`

**Interfaces:**
- Release workflow produces immutable release archives and moves `latest` only.
- Campaign workflow produces a channel bundle and dispatches Ahairu only when
  the resolved digest changes.
- Required repository secrets are GitHub App credentials for selected-repo
  dispatch; Assets receives no Cloudflare credential.

- [ ] **Step 1: Write failing workflow structure tests**

```sh
test "$(yq '.on.schedule[0].cron' .github/workflows/campaigns.yml)" = '0 0 * * *'
grep -F 'concurrency:' .github/workflows/campaigns.yml
grep -F 'manifests/campaigns.yaml' .github/workflows/campaigns.yml
grep -F 'manifests/default.yaml' .github/workflows/campaigns.yml
! grep -F 'CLOUDFLARE' .github/workflows/campaigns.yml
```

Implement equivalent parsing without assuming `yq` is installed in CI; the
test fixture may use Ruby or Python already available on Ubuntu.

- [ ] **Step 2: Run the workflow test and prove files are absent**

Run: `./scripts/check-release-workflows_test.sh`

Expected: FAIL.

- [ ] **Step 3: Implement release workflow**

Trigger only on tags matching `v*`. Check out the exact tag, install Go 1.26.5,
run the complete offline gate, build archives, verify checksums, create or update
the matching GitHub Release, and publish an immutable release-bundle artifact.
Write the `latest` candidate separately; never edit `default` or `current`.

- [ ] **Step 4: Implement campaign workflow**

Triggers:

```yaml
on:
  schedule:
    - cron: "0 0 * * *"
  workflow_dispatch:
    inputs:
      date:
        description: UTC date override in YYYY-MM-DD form
        required: false
        type: string
  push:
    branches: [main]
    paths:
      - manifests/campaigns.yaml
      - manifests/default.yaml
```

Use one non-cancelling deployment concurrency group. Resolve the date, build a
channel bundle, compare its digest with the last accepted channel artifact, and
stop cleanly on equality. On change, mint a short-lived selected-repository
GitHub App token and dispatch Ahairu with immutable artifact identity and hash.

- [ ] **Step 5: Pin actions and restrict permissions**

Pin every action by commit SHA. Default workflow permissions to `contents:
read`; grant `contents: write` only to the release job that publishes a GitHub
Release. Grant the App only the selected repository dispatch permission.

- [ ] **Step 6: Run structure and full CI contract tests**

Run: `./scripts/check-release-workflows_test.sh && ./scripts/check-ci-workflow.sh && go test ./... -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows scripts/check-release-workflows.sh scripts/check-release-workflows_test.sh
git commit -m "ci: publish guarded asset channels"
```

### Task 8: Regenerate, verify, and prepare the patch release

**Files:**
- Modify: generated `dist/**`
- Modify: `README.md`
- Modify: `docs/integration/consumers.md`
- Modify: `docs/integration/goshtoso.md`
- Create: `docs/release-v0.1.1-checklist.md`

**Interfaces:**
- Produces release candidate `v0.1.1` unless implementation reveals a
  compatibility break requiring a higher SemVer version.
- Produces the exact immutable bundle consumed by Ahairu and downstream plans.

- [ ] **Step 1: Set release metadata to `v0.1.1` in the single authoritative source**

Replace the current hard-coded `releaseVersion` with one explicit release
source used by catalog, release metadata, checksums, and archives. Add a test
that all emitted release fields agree.

- [ ] **Step 2: Generate all artifacts offline**

Run: `make generate && make proof`

Expected: generated `dist` includes catalog, theme, campaign, runtime, release
metadata, checksums, sprites, platform outputs, proof, and archives.

- [ ] **Step 3: Run the complete repository gate**

Run:

```bash
make check
make proof-check
go test ./... -count=1
go vet ./...
make verify
make release
git diff --check
```

Expected: PASS.

- [ ] **Step 4: Prove deterministic bytes from a second clean output**

Create two new temporary directories with `mktemp -d`. Run `campaigns publish`
for the same date into each. Compare recursive SHA-256 manifests and require
exact equality. Remove only those validated temporary directories afterward.

- [ ] **Step 5: Review patch compatibility against `v0.1.0`**

Decode both catalogs. Require every `v0.1.0` canonical asset and semantic field
to remain unchanged. Record additions and release-document changes in the
checklist.

- [ ] **Step 6: Commit generated release candidate**

```bash
git add dist README.md docs/integration docs/release-v0.1.1-checklist.md
git commit -m "build: prepare assets v0.1.1"
```

- [ ] **Step 7: Stop before tag publication for integration review**

Provide the clean commit range, exact test output, catalog compatibility report,
and archive hashes to the control plane. Tagging occurs only in the release
plan after Ahairu assembly accepts the bundle.
