# Ahairu Assets Delivery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Serve complete immutable Arai Hû releases and mutable presentation channels from the existing Ahairu Worker, enroll Ahairu as the first campaign canary, and connect the existing brand/license site to the new release contract.

**Architecture:** Ahairu CI downloads and verifies every tagged Assets release plus the resolved channel bundle, then the Ahairu build collision-safely assembles that cumulative input with the generated site. The existing Worker maps three extensionless channel URLs to JSON files and applies exact cache headers without calculating campaigns. Site templates explicitly enroll the deferred runtime and declare bounded brand/toggle hooks.

**Tech Stack:** Go 1.26.5, templ v0.3.1020, Goshtoso, vanilla Worker JavaScript, Node test runner, Wrangler 4, GitHub Actions, Cloudflare Workers Static Assets.

## Global Constraints

- Base implementation on refreshed `origin/main` containing merge `9074d24`; do not repeat `/brand`, `/license`, metadata, social, sitemap, robots, or v0.1.0 catalog work.
- Preserve multilingual routes and `Accept-Language` behavior.
- Deploy one complete `public/` tree. Never overlay a partial channel update on production.
- Retain every prior immutable `/assets/releases/vX.Y.Z/**` path in subsequent deployments.
- Treat Assets artifacts as untrusted input until release/checksum/channel validation passes.
- The Worker may map routes and set headers, but may not resolve dates or campaigns.
- Explicit app enrollment only. No Worker HTML rewriting.
- Keep Cloudflare authority in Ahairu. Assets receives no Cloudflare credential.
- Use Go 1.26.5 and regenerate templ output; never hand-edit `*_templ.go`.

---

### Task 1: Add a rooted, verified Assets bundle assembler

**Files:**
- Create: `internal/assetbundle/assemble.go`
- Create: `internal/assetbundle/assemble_test.go`
- Modify: `cmd/ahairu/main.go`
- Modify: `cmd/ahairu/main_test.go`

**Interfaces:**
- Produces: `assetbundle.Assemble(ctx context.Context, source fs.FS, destination *os.Root) error`
- Produces: `assetbundle.Validate(source fs.FS) (assetbundle.Bundle, error)`
- `Bundle` records immutable releases, `latest.json`, `default.json`, `current.json`, and `campaign/v1.js`.

- [ ] **Step 1: Write failing traversal, checksum, and retention tests**

```go
func TestValidateRejectsChannelOutsideImmutableRelease(t *testing.T) {
	input := fixtureBundle(t)
	input.Write("releases/current.json", []byte(`{"schemaVersion":1,"theme":{"stylesheet":"/private/theme.css"}}`))
	if _, err := Validate(input.FS()); err == nil || !strings.Contains(err.Error(), "immutable release") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestAssembleRetainsEveryImmutableRelease(t *testing.T) {
	destination := rootedFixture(t, "releases/v0.1.0/release.json", []byte("old"))
	if err := Assemble(context.Background(), fixtureV011(t), destination); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"releases/v0.1.0/release.json", "releases/v0.1.1/release.json"} {
		if _, err := destination.Stat(name); err != nil { t.Errorf("%s: %v", name, err) }
	}
}
```

- [ ] **Step 2: Run the focused test and prove the package is absent**

Run: `go test ./internal/assetbundle -count=1`

Expected: FAIL because the package does not exist.

- [ ] **Step 3: Implement strict bundle validation**

```go
type Bundle struct {
	Releases []string
	Latest   Channel
	Default  Channel
	Current  Channel
}

type Channel struct {
	SchemaVersion int    `json:"schemaVersion"`
	Release       string `json:"release"`
	Digest        string `json:"digest"`
}
```

Walk with `fs.WalkDir`. Reject symlinks, non-regular files, backslashes,
traversal, unknown top-level directories, duplicate release IDs, malformed JSON,
unavailable release references, and checksum mismatches. Parse full channel
documents using strict decoders even when this narrow struct is used for
indexing.

- [ ] **Step 4: Implement collision-safe rooted assembly**

Copy regular files below `public/assets/`. Existing identical bytes are
accepted. Existing different bytes fail with both logical paths named. Never
follow destination symlinks. On failure, build into a new staging root so the
caller can discard it without damaging the previous `public/` tree.

- [ ] **Step 5: Wire `ahairu build --asset-bundle <directory>`**

Refactor process-free command parsing so tests invoke:

```text
ahairu build --asset-bundle /verified/input
```

Require the flag in CI/production builds. Permit an explicit fixture path in
tests. Remove hidden reliance on the old subtree when assembling public release
channels; retain the v0.1.0 embedded subset only as the site baseline.

- [ ] **Step 6: Run focused build tests**

Run: `go test ./internal/assetbundle ./cmd/ahairu -count=1`

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/assetbundle cmd/ahairu
git commit -m "feat(build): assemble verified asset releases"
```

### Task 2: Map public channels and exact cache headers

**Files:**
- Modify: `src/worker.js`
- Modify: `src/worker.test.js`
- Modify: `cmd/checksite/main.go`
- Modify: `cmd/checksite/main_test.go`

**Interfaces:**
- Public maps:
  - `/assets/releases/latest` -> `/assets/releases/latest.json`
  - `/assets/releases/default` -> `/assets/releases/default.json`
  - `/assets/releases/current` -> `/assets/releases/current.json`
- Produces channel headers `Content-Type: application/json; charset=utf-8` and `Cache-Control: public, max-age=60, must-revalidate`.
- Produces immutable header `Cache-Control: public, max-age=31536000, immutable` below `/assets/releases/vMAJOR.MINOR.PATCH/`.
- Produces `Access-Control-Allow-Origin: *` and `Cross-Origin-Resource-Policy: cross-origin` for public campaign/runtime/release resources; credentials are not allowed.

- [ ] **Step 1: Write failing Worker route/header tests**

```js
test("maps the extensionless current channel", async () => {
  const { response, requested } = await request("/assets/releases/current");
  assert.equal(requested.pathname, "/assets/releases/current.json");
  assert.equal(response.headers.get("content-type"), "application/json; charset=utf-8");
  assert.equal(response.headers.get("cache-control"), "public, max-age=60, must-revalidate");
});

test("marks versioned assets immutable", async () => {
  const { response } = await request("/assets/releases/v0.1.1/catalog.json");
  assert.equal(response.headers.get("cache-control"), "public, max-age=31536000, immutable");
});

test("permits anonymous public asset consumption", async () => {
  const { response } = await request("/assets/releases/current", { Origin: "https://goshtoso.araihu.com" });
  assert.equal(response.headers.get("access-control-allow-origin"), "*");
  assert.equal(response.headers.get("access-control-allow-credentials"), null);
});
```

- [ ] **Step 2: Run Worker tests and prove current behavior fails**

Run: `node --test src/worker.test.js`

Expected: FAIL because extensionless release channels return 404.

- [ ] **Step 3: Add a closed route map and response header wrapper**

```js
const releaseChannels = new Map([
  ["/assets/releases/latest", "/assets/releases/latest.json"],
  ["/assets/releases/default", "/assets/releases/default.json"],
  ["/assets/releases/current", "/assets/releases/current.json"],
]);

function withAssetHeaders(response, pathname) {
  const headers = new Headers(response.headers);
  if (releaseChannels.has(pathname)) {
    headers.set("Content-Type", "application/json; charset=utf-8");
    headers.set("Cache-Control", "public, max-age=60, must-revalidate");
  } else if (/^\/assets\/releases\/v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?\//.test(pathname)) {
    headers.set("Cache-Control", "public, max-age=31536000, immutable");
  }
  if (pathname.startsWith("/assets/releases/") || pathname.startsWith("/assets/campaign/")) {
    headers.set("Access-Control-Allow-Origin", "*");
    headers.set("Cross-Origin-Resource-Policy", "cross-origin");
  }
  return new Response(response.body, { status: response.status, statusText: response.statusText, headers });
}
```

Handle `GET` and `HEAD`; preserve method semantics and locale `Vary` behavior.
Because responses never use credentials, wildcard CORS does not vary by Origin.
Do not map arbitrary extensionless asset paths.

- [ ] **Step 4: Extend `checksite` to validate release tree and channel references**

Require all three channels, runtime, referenced release documents, checksums,
same-origin absolute paths, correct SemVer directories, and no missing retained
release declared by the cumulative bundle inventory.

- [ ] **Step 5: Run focused tests**

Run: `node --test src/worker.test.js && go test ./cmd/checksite -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add src/worker.js src/worker.test.js cmd/checksite
git commit -m "feat(worker): serve versioned asset channels"
```

### Task 3: Explicitly enroll Ahairu as the first campaign canary

**Files:**
- Modify: `site/layout.templ`
- Modify: `site/home.templ`
- Modify: `site/brand_page.templ`
- Modify: `site/brand.css`
- Modify: `site/pages_test.go`
- Modify: `cmd/checksite/main.go`
- Modify: `cmd/checksite/main_test.go`
- Regenerate: `site/layout_templ.go`
- Regenerate: `site/home_templ.go`
- Regenerate: `site/brand_page_templ.go`

**Interfaces:**
- Root baseline: `<html data-theme="araihu" data-theme-source="default">`.
- Runtime: `<script src="/assets/campaign/v1.js" data-channel="/assets/releases/current" defer integrity="..." crossorigin="anonymous"></script>`.
- Brand hooks: `data-asset-brand="logo"` and `data-asset-brand="icon"` on Arai Hû-owned fixed-size images only.
- Toggle hooks: `data-campaign-toggle` and child `data-campaign-toggle-icon`.

- [ ] **Step 1: Write failing generated-HTML contract tests**

```go
for _, want := range []string{
	`data-theme-source="default"`,
	`src="/assets/campaign/v1.js"`,
	`data-channel="/assets/releases/current"`,
	`data-asset-brand="icon"`,
	`data-campaign-toggle`,
} {
	if !strings.Contains(html, want) { t.Errorf("missing %s", want) }
}
```

Require the runtime script after baseline styles and the first-paint attributes
before body content. Require non-empty width and height on every replaceable
image.

- [ ] **Step 2: Run focused tests and prove hooks are absent**

Run: `go test ./site ./cmd/checksite -count=1`

Expected: FAIL.

- [ ] **Step 3: Add root marker and deferred runtime script**

Compute and pin the SRI SHA-384 from the accepted Assets release byte. The
runtime script URL stays same-origin and uses `defer`; no inline campaign code.

- [ ] **Step 4: Add bounded brand and toggle markup**

Mark the header Arai Hû icon and the primary identity proof logo. Do not mark
product project icons, misuse examples, minimum-size specimens, or download
previews: those demonstrate specific catalog assets and must remain stable.

Add one initially hidden button in the header:

```html
<button type="button" hidden data-campaign-toggle aria-pressed="false">
  <span data-campaign-toggle-icon aria-hidden="true"></span>
  <span class="sr-only">Use the standard Arai Hû appearance</span>
</button>
```

The runtime unhides it only for an active eligible campaign. CSS reserves its
size and supplies visible focus.

- [ ] **Step 5: Regenerate and test**

Run: `templ generate && go test ./site ./cmd/checksite -count=1`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add site cmd/checksite
git commit -m "feat(site): enroll seasonal asset runtime"
```

### Task 4: Point brand downloads and metadata at immutable release channels

**Files:**
- Modify: `site/brand.go`
- Modify: `site/brand_assets_test.go`
- Modify: `site/brand_page.templ`
- Modify: `site/public_content.go`
- Modify: `site/static.go`
- Modify: `cmd/checksite/main.go`
- Regenerate: `site/brand_page_templ.go`

**Interfaces:**
- Baseline embed remains pinned to one exact released Assets version.
- Public downloads use `BrandAssetsPublicPrefix` (for the first update, `/assets/releases/v0.1.1/`), never `current`.
- `/brand` identifies the actual release and links `release.json`, `catalog.json`, `themes.json`, `campaigns.json`, `checksums.txt`, and archives when present.

- [ ] **Step 1: Write failing release-link and checksum tests**

Require every download path to begin with `BrandAssetsPublicPrefix`, and redefine
that prefix as `/assets/releases/v0.1.1/` after the Assets release is accepted.
Require the embedded `release.json` hash and its document hashes to match copied
bytes.

- [ ] **Step 2: Run focused tests and prove v0.1.0-only assumptions fail**

Run: `go test ./site ./cmd/checksite -count=1`

Expected: FAIL after new assertions.

- [ ] **Step 3: Replace the embedded subset from the accepted Assets release**

Use the release artifact, not the Assets source tree. Include only catalog-
selected brand/platform files and required release support documents. Preserve
license files and upstream checksums exactly.

- [ ] **Step 4: Update `/brand` release links and copy**

Keep multilingual guidance and existing metadata. Change release labels and
download URLs from `v0.1.0` to the accepted patch. Link `/license` for
distribution terms. Do not expose source concepts or review history.

- [ ] **Step 5: Regenerate and test the whole static tree**

Run: `templ generate && npm run build && go run ./cmd/checksite public`

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add site cmd/checksite
git commit -m "build(site): pin seasonal asset release"
```

### Task 5: Make CI assemble and deploy one complete Worker version

**Files:**
- Modify: `.github/workflows/ci.yml`
- Modify: `.github/workflows/deploy.yml`
- Modify: `package.json`
- Modify: `README.md`

**Interfaces:**
- Accepts `repository_dispatch` with immutable Assets release URL/ID, channel artifact URL/ID, and SHA-256 values.
- Deployment job downloads and verifies both, runs full assembly/check, performs `wrangler deploy`, and records the Worker version.
- Preserves normal deploy after a successful Ahairu `main` CI run using the promoted asset inputs.

- [ ] **Step 1: Add failing workflow contract tests to `cmd/checksite` or a focused script**

Assert pinned actions, Go 1.26.5, Node 24, one deployment concurrency group,
`repository_dispatch`, artifact hash verification, `npm run check`, direct
`wrangler deploy`, and absence of an Assets-side Cloudflare secret.

- [ ] **Step 2: Run the workflow contract and prove current deploy hook is insufficient**

Run: `go test ./cmd/checksite -run Workflow -count=1`

Expected: FAIL because current deploy only POSTs a hook and cannot receive an
Assets bundle.

- [ ] **Step 3: Implement verified artifact acquisition**

Mint or receive a short-lived selected-repository GitHub App token. Download the
exact immutable release/channel artifacts named in the dispatch payload. Verify
both hashes before extraction. Reject archives containing symlinks, absolute
paths, traversal, or unexpected roots.

- [ ] **Step 4: Implement complete direct deployment**

Build into a new staging directory, validate, then invoke Wrangler with Ahairu's
Cloudflare API token/account configuration. Set:

```yaml
concurrency:
  group: ahairu-production
  cancel-in-progress: false
```

Pin all actions by full commit SHA and use protected `production` environment
credentials.

- [ ] **Step 5: Run local CI parity checks**

Run:

```bash
templ generate
git diff --exit-code -- 'site/*_templ.go'
npm ci
npm run check
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add .github/workflows package.json README.md
git commit -m "ci: deploy complete asset channel bundle"
```

### Task 6: Local canary acceptance and deployment handoff

**Files:**
- Create: `docs/seasonal-assets-canary.md`
- Modify only if evidence finds a defect: implementation files from Tasks 1-5.

**Interfaces:**
- Produces exact local and public probes consumed by the release plan.

- [ ] **Step 1: Run complete local checks**

Run:

```bash
templ generate
git diff --exit-code -- 'site/*_templ.go'
go test ./... -count=1
node --test src/worker.test.js
npm run check
git diff --check
```

Expected: PASS.

- [ ] **Step 2: Exercise Worker behavior locally**

Start one Wrangler dev session on its reported local URL. Probe `GET` and `HEAD`
for channels, runtime, two immutable releases, `/brand/`, `/license/`, localized
routes, an unknown extensionless route, and a missing file. Record statuses,
content types, cache headers, and bodies/hashes.

- [ ] **Step 3: Exercise browser preference behavior**

Using the existing browser test harness, prove baseline first paint, campaign
apply, saved preference win, active opt-out, reload persistence, CSS/image
failure fallback, expiry restoration, fixed dimensions, and reduced-motion
hooks.

- [ ] **Step 4: Record rollback procedure**

Document how to identify and redeploy the preceding complete Worker version.
Name the 60-second convergence expectation after a corrective deployment.

- [ ] **Step 5: Commit evidence and stop before production deployment**

```bash
git add docs/seasonal-assets-canary.md
git commit -m "docs: record seasonal asset canary"
```

Return the clean commit range and test evidence to the control plane. Production
deployment occurs only after the release plan accepts the Assets and Ahairu
batches.
