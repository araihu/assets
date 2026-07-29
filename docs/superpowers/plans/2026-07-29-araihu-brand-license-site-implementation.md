# Arai Hû Brand and License Site Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a beautiful, static, multilingual `/brand/` and `/license/` experience at `araihu.com` using released assets and reusable Goshtoso primitives.

**Architecture:** The static Go builder renders typed localized page models through a shared templ layout, generates metadata/JSON-LD/sitemap/robots/manifest, and copies a pinned `assets` v0.1.0 distributable subset. Cloudflare Worker routing provides canonical slash and English redirects; browser tests validate the generated site without adding client runtime.

**Tech Stack:** Go 1.26.5, templ v0.3.1020, released Goshtoso icon support, static CSS/HTML, Node test runner with existing Puppeteer, Cloudflare Workers static assets.

## Global Constraints

- Work in `/tmp/ah-brand-license` from freshly fetched `origin/main`; the current primary checkout is clean but remains untouched.
- `go.mod` declares `go 1.26.5` and `toolchain go1.26.5`; CI uses that exact patch.
- Canonical English routes are `/brand/` and `/license/`; `/en/brand[/]` and `/en/license[/]` permanently redirect.
- Localized routes are `/pt-br/.../` and `/es/.../`; localized license pages prominently state that English terms govern.
- Assets come from the recorded `assets` v0.1.0 release candidate/release, never a mutable subtree branch.
- Arai Hû bindings are generated into this repository; no brand names enter Goshtoso.
- Public pages have no HTMX/Alpine requirement; use static Goshtoso primitives and semantic HTML.
- Metadata is unique, absolute, localized, reciprocal, and validated; redirect URLs never enter sitemap/hreflang.
- The bespoke social image is generated once after copy/layout stabilize, inspected, and retried once only if unusable.
- Production deploy and authoritative English license publication require the final user checkpoint.

---

## File map

- `site/model.go`, `pages.go`, `content.go` — locale/page types and localized content.
- `site/layout.templ`, `metadata.templ`, generated files — document shell and metadata.
- `site/brand_page.templ`, `license_page.templ`, generated files — public pages.
- `site/structured_data.go`, tests — safe JSON-LD values.
- `site/static.go`, tests — robots, sitemap, manifest, assets/downloads.
- `site/brandicons/catalog_gen.go` — Goshtoso-generated local brand constants.
- `site/brand-assets/**` — pinned distributable subset, catalog, checksums, notices/licenses.
- `site/social/{brand.png,license.png}` — inspected 1200×630 images.
- `cmd/ahairu/` and `cmd/checksite/` — deterministic build and output validation.
- `src/worker.js`, tests — exact route/redirect/language behavior.
- `test/browser/site.test.js` — responsive, metadata, interaction, and console validation.

### Task 1: Upgrade the site toolchain and introduce typed pages

**Files:** Modify `go.mod`, `go.sum`, `cmd/ahairu/main.go`, tests, `site/content.go`; create `site/model.go`, `site/pages.go`, `site/pages_test.go`.

**Interfaces:** Produce `Pages() []Page` over types below.

```go
type Locale struct { Key, Language, OGLocale, Label string }
type PageKind string
const (PageHome PageKind="home"; PageBrand PageKind="brand"; PageLicense PageKind="license")
type Alternate struct { Language, URL string }
type PageMeta struct { Kind PageKind; Locale Locale; Path, CanonicalURL, Title, Description, SocialImageURL, Robots string; Alternates []Alternate; StructuredData any }
type Page struct { Meta PageMeta; Navigation Navigation; Home *HomeContent; Brand *BrandContent; License *LicenseContent }
```

- [ ] **Step 1: Create worktree and select Go 1.26.5**

```bash
git fetch origin
git worktree add -b feat/brand-license /tmp/ah-brand-license origin/main
cd /tmp/ah-brand-license
go mod edit -go=1.26.5 -toolchain=go1.26.5
GOTOOLCHAIN=auto go version
```

- [ ] **Step 2: Write failing route-model tests**

```go
func TestPagesContainNineCanonicalLocalizedPages(t *testing.T) {
	pages := Pages(); require.Len(t, pages, 9)
	requirePage(t, pages, "/brand/", "en", PageBrand)
	requirePage(t, pages, "/pt-br/license/", "pt-BR", PageLicense)
	requirePage(t, pages, "/es/brand/", "es", PageBrand)
}
func TestLocaleNavigationPreservesPageKind(t *testing.T) {
	p := requirePage(t, Pages(), "/pt-br/brand/", "pt-BR", PageBrand)
	requireLocaleLink(t, p.Navigation, "es", "/es/brand/")
}
```

- [ ] **Step 3: Implement locale and page construction**

English home remains `/en/`; English brand/license omit `/en`. Build canonical URLs from constant `https://araihu.com`, not request state. Every page gets reciprocal `en`, `pt-BR`, `es`, and `x-default` alternates.

- [ ] **Step 4: Pass, tidy, and commit**

```bash
go test ./site ./cmd/ahairu -count=1
go mod tidy
git add go.mod go.sum cmd/ahairu site/model.go site/pages.go site/pages_test.go site/content.go
git commit -m "feat(site): model multilingual brand pages"
```

### Task 2: Generate complete metadata and static discovery files

**Files:** Create metadata templ/source tests, structured data source/tests, static source/tests, `cmd/checksite/main.go` and tests.

**Interfaces:** Produce `Layout(Page, templ.Component)`, `StructuredData(Page) any`, `SiteManifest() []byte`, `Robots() []byte`, and `Sitemap([]Page) ([]byte, error)`.

- [ ] **Step 1: Write failing metadata matrix tests**

```go
func TestEveryPageHasCompleteAbsoluteMetadata(t *testing.T) {
	for _, p := range Pages() {
		html := renderMetadata(t, p)
		require.Contains(t, html, `rel="canonical" href="https://araihu.com`)
		require.Contains(t, html, `property="og:image" content="https://araihu.com/`)
		require.Contains(t, html, `name="twitter:card" content="summary_large_image"`)
		requireHreflangSet(t, html, "en", "pt-BR", "es", "x-default")
	}
}
func TestSitemapHasNineCanonicalPagesAndNoRedirects(t *testing.T) { requireCanonicalSitemap(t, Pages(), 9) }
```

- [ ] **Step 2: Implement metadata templates**

Emit localized title/description, absolute canonical, theme color, favicon/touch/manifest, robots, OG title/description/url/locale/alternate/image/type, X title/description/image/card, reciprocal alternates, and JSON-LD through `templ.JSONScript(...).WithType("application/ld+json")`; never use `templ.Raw`.

- [ ] **Step 3: Implement structured data**

Brand graph uses stable `https://araihu.com/#organization` and `#brand` IDs with publisher/logo relationship. License uses `WebPage`, canonical, `inLanguage`, visible version/effective date, and publisher reference.

- [ ] **Step 4: Implement static files and output checker**

Sitemap lists three home, three brand, and three license pages. Robots points to absolute sitemap. Manifest references released icons. Checker rejects missing/relative canonical and social URLs, duplicate titles/descriptions, missing alternates, invalid JSON-LD, redirect URLs in sitemap, missing local files, and social images not exactly 1200×630.

- [ ] **Step 5: Generate templ, pass, and commit**

```bash
templ generate
go test ./site ./cmd/checksite -run 'Metadata|StructuredData|Sitemap|Manifest|Check' -count=1
git add site cmd/checksite
git commit -m "feat(site): generate discoverable page metadata"
```

### Task 3: Import the released asset subset and local brand bindings

**Files:** Replace `site/brand-assets/**` with released subset; create generated `site/brandicons/catalog_gen.go`; modify builder/static tests.

**Interfaces:** Consume recorded catalog/checksum/release commit and Goshtoso generator. Produce exact public prefix `/assets/araihu/v0.1.0/` and typed local symbols.

- [ ] **Step 1: Write failing integrity tests**

```go
func TestBundledBrandAssetsMatchChecksums(t *testing.T) { requireAllChecksums(t, "brand-assets/checksums.txt") }
func TestEveryDownloadExistsAndHasCatalogHash(t *testing.T) {
	for _, p := range Pages() { for _, d := range downloads(p) { requireDownloadMatchesCatalog(t, d) } }
}
```

- [ ] **Step 2: Export only the distributable subset**

Import `catalog.json`, checksums, `brand/**`, `icons/brand/**`, `platform/web/**`, `licenses/**`, and NOTICE from the immutable release. Remove numbered concepts/reviews from `site/brand-assets`; do not re-add the old subtree snapshot.

- [ ] **Step 3: Generate project-local bindings**

```bash
go run github.com/araihu/goshtoso/cmd/iconcatalog \
  -catalog site/brand-assets/catalog.json -namespace brand \
  -sprite-url /assets/araihu/v0.1.0/icons/brand/sprite.svg \
  -package brandicons -const-prefix Icon -out site/brandicons/catalog_gen.go
```

Use a local `go.work` against reviewed `/tmp/gs-icon-catalog` until a released Goshtoso version exists; do not commit a local `replace` directive.

- [ ] **Step 4: Make the builder catalog-driven**

Copy only catalog-addressed files and release support files. Reject hard-coded `?rev=a8a9647a`, absent catalog entries, checksum mismatch, and unexpected historical directories.

- [ ] **Step 5: Pass and commit**

```bash
go test ./site ./cmd/ahairu -run 'Asset|Download|Catalog|Checksum' -count=1
git add site/brand-assets site/brandicons cmd/ahairu site/static.go site/static_test.go
git commit -m "feat(site): consume released brand catalog"
```

### Task 4: Build the curated brand and license pages

**Files:** Create layout/brand/license templ sources/generated files, `site/brand.go`, tests, `site/brand.css`; refactor `site/home.templ` to shared layout.

**Interfaces:** Produce `BrandPage(Page) templ.Component` and `LicensePage(Page) templ.Component` using typed content.

- [ ] **Step 1: Write failing content and semantic rendering tests**

```go
func TestBrandPageContainsGuidanceDownloadsAndLicenseLink(t *testing.T) {
	html := renderPage(t, requireBrand(t, "en"))
	for _, text := range []string{"Minimum size", "Clear space", "Incorrect use", "Download", "catalog.json", "/license/"} { require.Contains(t, html, text) }
}
func TestLocalizedLicenseShowsEnglishAuthorityNotice(t *testing.T) {
	for _, lang := range []string{"pt-br", "es"} { requireAuthorityNotice(t, renderPage(t, requireLicense(t, lang)), "/license/") }
}
```

- [ ] **Step 2: Implement shared static layout**

Use Goshtoso styles/theme, generic icon component, `pageheader.PageHeader`, and `panel.Panel` where they improve structure. Use semantic download links and static `<pre><code>`; do not load HTMX/Alpine merely for code blocks.

- [ ] **Step 3: Implement curated brand content**

Sections: identity, primary icon/logo, approved light/dark/monochrome/designed tint, minimum sizes, clear space, incorrect uses, web/mobile examples, downloads, sprite/catalog/checksums/Goshtoso integration, attribution, prominent license link. Use representative assets; exhaustive matrices remain in the proof site.

- [ ] **Step 4: Implement license content**

English page visibly states version/effective date and separates Arai Hû brand permission from Heroicons MIT. Cover unmodified integration/docs redistribution, attribution/notices/no endorsement, and permission requirements for modified marks, standalone redistribution, merchandise, another identity, or implied affiliation. Localized pages place the informational-translation notice before terms.

- [ ] **Step 5: Generate, test, and commit**

```bash
templ generate
go test ./site -run 'Brand|License|Layout|Content' -count=1
git add site
git commit -m "feat(site): publish brand and license guidance"
```

### Task 5: Implement exact Worker routes and redirects

**Files:** Modify `src/worker.js`; create/modify `src/worker.test.js` and package scripts.

**Interfaces:** Canonical routes plus 308 redirects preserving query strings; unknown extensionless paths return 404.

- [ ] **Step 1: Write failing route-table tests**

```js
test('redirects English and slash aliases permanently', async () => {
  await expectRedirect('/en/brand?x=1', '/brand/?x=1', 308)
  await expectRedirect('/pt-br/license', '/pt-br/license/', 308)
})
test('unknown extensionless route is not English home', async () => {
  assert.equal((await request('/not-a-page')).status, 404)
})
```

- [ ] **Step 2: Replace fallback routing with an explicit table**

Map six canonical brand/license pages and their no-slash forms. Preserve existing home negotiation, add `Vary: Accept-Language`, preserve GET versus HEAD for static assets, and exclude redirects from generated metadata.

- [ ] **Step 3: Pass and commit**

```bash
node --test src/worker.test.js
git add src/worker.js src/worker.test.js package.json package-lock.json
git commit -m "fix(worker): route canonical brand pages explicitly"
```

### Task 6: Generate and inspect bespoke social images

**Files:** Create `site/social/brand.png`, `site/social/license.png`; modify page metadata/content if final copy changes.

**Interfaces:** One generated two-panel master, deterministically cropped into two inspected 1200×630 static images with no unreadable or incorrect generated text.

- [ ] **Step 1: Stabilize copy and art direction before generation**

Confirm final titles, Arai Hû spelling, Calibration Bench visual direction, and which approved logo/icon files can be composited without regeneration.

- [ ] **Step 2: Use the Sites/image-generation workflow once**

Make one image-generation request for a coordinated two-panel social master:
brand overview on the left and license/usage card on the right,
paper/midnight/cobalt/lime, approved identity as supplied reference, sparse
text. After inspection, crop the two equal panels mechanically into 1200×630
files. Do not make exploratory generations before this request.

- [ ] **Step 3: Inspect exact output**

Verify dimensions, spelling, accents, identity geometry, contrast, crops, and small-card legibility. Retry once only if unusable; if the retry remains bad, omit generated text and compose approved SVG plus HTML/CSS-rendered typography into PNG.

- [ ] **Step 4: Test and commit**

```bash
go test ./site -run 'Social|Metadata' -count=1
git add site/social site/content.go
git commit -m "feat(site): add brand social previews"
```

### Task 7: Prove static output, metadata, and visual quality

**Files:** Create `test/browser/site.test.js`; modify package scripts, README, DESIGN.md, PRODUCT.md, and CI.

**Interfaces:** Produce deterministic `public/`, browser evidence, Goshtoso parity findings, and deployment-ready but undeployed Worker bundle.

- [ ] **Step 1: Write browser tests**

Test all nine canonical pages; reciprocal alternates; JSON-LD parse; absolute OG/X/canonical; no console/network errors; downloads 200; mobile overflow; light/dark/transparent/tinted examples; keyboard focus; reduced motion; stable variant geometry; redirects and 404s.

- [ ] **Step 2: Add scripts**

```json
{
  "test:metadata": "go test ./site -run 'Metadata|StructuredData|Sitemap|Manifest' -count=1",
  "test:routes": "node --test src/worker.test.js",
  "test:visual": "node --test test/browser/site.test.js"
}
```

- [ ] **Step 3: Run Impeccable and Sites reviews**

Compare public-page hierarchy/confidence with the engineering proof at 375, 768, 1280, and 1440 widths in both schemes. Record page-specific art direction separately from reusable Goshtoso limitations; do not hide structural defects with fragile JavaScript.

- [ ] **Step 4: Run complete local gates**

```bash
GOTOOLCHAIN=auto go version
templ generate
git diff --exit-code -- site/*_templ.go
go run ./cmd/ahairu
go test ./... -count=1
go run ./cmd/checksite public
node --test src/worker.test.js
node --test test/browser/site.test.js
npx wrangler deploy --dry-run
```

- [ ] **Step 5: Commit release evidence without deployment**

```bash
git add README.md DESIGN.md PRODUCT.md package.json package-lock.json test .github
git commit -m "test(site): verify public brand experience"
```

`public/` is ignored; stage it only if repository policy changes explicitly. Record exact commits and dry-run output in the control-plane ledger.

### Task 8: Pin released dependencies after approval

**Files:** Modify `go.mod`, `go.sum`, asset release metadata, README/release evidence.

**Interfaces:** Replace local workspace compatibility with public assets `v0.1.0` and the newly tagged Goshtoso version.

- [ ] **Step 1: Wait for explicit tag approval and published tags**

Do not invent a pseudo-version or commit a local replace. Once tags exist, update through normal module/release mechanisms.

- [ ] **Step 2: Pin and rebuild from a clean module cache path**

```bash
goshtoso_version=$(sed -n 's/^goshtosoRelease=//p' /tmp/araihu-assets-v0.1/docs/control-plane/assets-v0.1.0-ledger.md)
test -n "$goshtoso_version"
go get "github.com/araihu/goshtoso@$goshtoso_version"
go mod tidy
GOWORK=off go test ./... -count=1
GOWORK=off go run ./cmd/ahairu
go run ./cmd/checksite public
```

The symbolic released version is the control-plane field created by the approved Goshtoso tag; no mutable branch is accepted.

- [ ] **Step 3: Commit the pin and present deployment checkpoint**

```bash
git add go.mod go.sum site/brand-assets README.md
git commit -m "build: pin released brand assets and Goshtoso"
```

Production hook invocation and post-deploy HTTP verification require explicit user approval.
