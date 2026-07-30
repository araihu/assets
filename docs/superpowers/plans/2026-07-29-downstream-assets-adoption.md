# Downstream Assets Adoption Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adopt the released Assets and App Shell presentation-channel contracts in every relevant Arai Hû web surface while preserving local fallbacks, application branding boundaries, CSP, and existing behavior.

**Architecture:** Each application pins an immutable Assets release as its offline/default fallback and explicitly configures the App Shell or direct DOM contract. The remote channel supplies immediate seasonal presentation; repository updates preserve offline behavior. Applications retain ownership of theme preferences and authentication/security policy.

**Tech Stack:** Go 1.26.5, templ, Goshtoso App Shells, existing static generators, Node/Vinext where present, HTMX/Alpine where present.

## Global Constraints

- Start only from clean isolated worktrees and refreshed accepted bases.
- Consume released versions, never cross-repository `replace` directives in final commits.
- Do not hand-edit generated `*_templ.go`, generated `public/`, or bundled JavaScript.
- Every enrolled page renders a usable embedded baseline before the runtime.
- Every replaceable logo/image has fixed intrinsic dimensions.
- Explicit user theme preference cancels seasonal presentation.
- Campaign runtime failure leaves the app unchanged.
- Cross-origin consumers allow only `https://araihu.com` in applicable CSP
  `script-src`, `connect-src`, `style-src`, `img-src`, and `font-src` directives;
  no consumer adds wildcard origins or `unsafe-inline`.
- Manja caller/spec-provided branding is never campaign-managed.
- Xisnove CSP remains nonce-safe; do not add `unsafe-inline`.
- Backend-only binaries, agents, operators, and CLIs receive no browser runtime.
- Use Go 1.26.5 in touched modules and CI.

---

### Task 1: Enroll Goshtoso Charts as the App Shell canary

**Repository:** `goshtoso-charts`

**Base:** Refreshed `origin/main` after `341837d`, then update to released Goshtoso and App Shell versions.

**Files:**
- Modify: `site/go.mod`
- Modify: `site/go.sum`
- Modify: `site/internal/pages/shell.go`
- Modify: `site/internal/brand/brand.go`
- Modify: `site/internal/brand/brand_test.go` or create it if absent
- Modify: `site/internal/server/server_test.go`
- Modify: `components/interactive/theme_runtime.go`
- Modify: `components/interactive/theme_runtime_test.go`

**Interfaces:**
- Uses `componentdocshell.Brand.ManagedLogo` with `/brand/goshtoso-logo-transparent.svg`, width 120, height 32.
- Uses `PresentationChannelConfig` with public Ahairu runtime/channel URLs and accepted SRI.
- Existing chart runtime continues observing root `class` and `data-theme`.

- [ ] **Step 1: Write failing shell enrollment tests**

```go
for _, want := range []string{
	`data-theme-source="default"`,
	`data-asset-brand="logo"`,
	`width="120"`, `height="32"`,
	`data-campaign-toggle`,
	`src="https://araihu.com/assets/campaign/v1.js"`,
} {
	if !strings.Contains(body, want) { t.Errorf("missing %q", want) }
}
```

Use the accepted same-origin policy for the actual Charts host. If the runtime
must be cross-origin there, the Assets runtime and channel CORS contract must be
proven before using the absolute URL; do not silently bypass CORS.

- [ ] **Step 2: Run focused site tests and prove enrollment is absent**

Run: `cd site && GOWORK=off go test ./internal/pages ./internal/server -count=1`

Expected: FAIL.

- [ ] **Step 3: Replace inline shell logo with managed URL asset**

Keep `LandingLogo()` inline for the stable content specimen if desired. Replace
only the header `brand.Logo()` path with `ManagedBrandAsset`. Preserve 120×32
layout and existing favicon handler. Configure `ManageFavicon: true` only if the
runtime supports the link hook in accepted tests.

- [ ] **Step 4: Configure the presentation channel**

Set runtime URL, channel URL, exact SHA-384 SRI, and labels:

```go
UseCampaignLabel: "Use seasonal appearance",
UseBaselineLabel: "Use standard appearance",
```

Keep `PersistPreferences: true`. Do not add a second theme store.

- [ ] **Step 5: Prove interactive charts redraw on campaign theme changes**

Extend the existing MutationObserver/runtime test so changing `data-theme` from
`araihu` to `araihu-signal-night` schedules one redraw and does not recreate or
lose live chart state.

- [ ] **Step 6: Update released dependencies and run full gates**

Run:

```bash
GOWORK=off go test ./... -count=1
cd site
GOWORK=off go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add site/go.mod site/go.sum site/internal components/interactive
git commit -m "feat(site): enroll asset presentation channel"
```

### Task 2: Enroll Manja without touching caller-provided branding

**Repository:** `manja`

**Base:** Refreshed `origin/main` at or after `5d5005e`; never use the dirty primary checkout or the frozen release-track worktree.

**Files:**
- Modify: `go.mod`, `go.sum`
- Modify: `internal/web/templates/layout.templ`
- Modify: `internal/web/templates/public.templ`
- Modify: `internal/web/public.go`
- Modify: `internal/web/public_test.go`
- Modify: `internal/web/server.go`
- Modify: `internal/web/server_test.go`
- Modify: catalog-selected files below `internal/web/static/`
- Regenerate: relevant `internal/web/templates/*_templ.go`
- Modify: `site/go.mod`, `site/go.sum`
- Modify: `site/internal/site/pages.go`
- Modify: `site/internal/site/static/theme.js`
- Modify: `site/internal/site/server.go`
- Modify: site tests
- Modify: catalog-selected files below `site/internal/site/static/`

**Interfaces:**
- Default Manja brand uses fixed-size `data-asset-brand` hooks.
- `idx.Branding.Logo.Src`, caller favicon, and other OpenAPI/spec branding remain unmarked and authoritative.
- Existing `theme` preference key sets `data-theme-source="preference"` when present.

- [ ] **Step 1: Write failing default-versus-caller branding tests**

```go
func TestDefaultBrandEnrollsButCallerBrandDoesNot(t *testing.T) {
	baseline := renderPublic(t, defaultIndex())
	if !strings.Contains(baseline, `data-asset-brand="logo"`) { t.Fatal("default logo is unmanaged") }
	custom := renderPublic(t, indexWithLogo("/customer.svg"))
	if strings.Contains(custom, `src="/customer.svg" data-asset-brand`) { t.Fatal("caller logo became campaign managed") }
}
```

Add favicon equivalents and assert fixed dimensions.

- [ ] **Step 2: Run focused tests and prove behavior is absent**

Run: `GOWORK=off go test ./internal/web -count=1`

Expected: FAIL.

- [ ] **Step 3: Add direct root/runtime/toggle contract to the renderer**

If the refreshed Manja base already uses `consoleshell`, configure its released
presentation API. Otherwise add the exact generic DOM hooks directly to the
existing layout. Reuse its `theme` key; set the source marker in its first-paint
script. Keep CSP nonce requirements for inline bootstrap and allow the
same-origin deferred external runtime without adding `unsafe-inline`.

- [ ] **Step 4: Gate managed hooks on application-owned default branding**

Derive one boolean from the existing branding model. Only default Manja logo and
favicon receive hooks. Do not infer ownership from URL shape.

- [ ] **Step 5: Update the product site source and regenerate output**

Apply the same marker/runtime/toggle ordering to `site/`. Update source static
assets from the accepted catalog-selected release. Run its generator; do not
edit generated `public` bytes manually.

- [ ] **Step 6: Run full Manja gates**

Run:

```bash
npm ci
npm run api:bundle
npm run api:lint
go run github.com/a-h/templ/cmd/templ generate
GOWORK=off go test ./...
cd site
GOWORK=off go test ./... -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/web site
git commit -m "feat(web): adopt seasonal asset fallback"
```

### Task 3: Enroll the Pajé product site

**Repository:** `paje`

**Base:** Resolve authenticated remote access first. Require a clean worktree at the current accepted remote main; do not assume local `52bcbc4` is current merely because SSH fetch failed.

**Files:**
- Modify: root `go.mod` and any CI toolchain pins only as required for Go 1.26.5.
- Modify: `site/generator/main.go`
- Modify: `site/generator/araihu.css`
- Modify: `site/app/layout.tsx`
- Modify: `site/app/brand.tsx`
- Modify: `site/app/page.tsx`
- Modify: `site/app/theme-toggle.tsx`
- Modify: `site/tests/rendered-html.test.mjs`
- Modify generated output only through documented site build commands.
- Modify: `.github/workflows/deploy-site.yml`

**Interfaces:**
- Pajé baseline remains usable without remote runtime.
- Theme/dark preference and campaign opt-out are separate controls and keys.
- Header logo becomes a fixed-size managed image or equivalent URL-bearing hook; inline SVG source remains available for baseline generation if needed.

- [ ] **Step 1: Repair read-only remote access and freeze the base SHA**

Use authenticated HTTPS or the configured GitHub CLI without printing tokens.
Fetch, record the accepted remote SHA in the control ledger, and create an
isolated worktree. Stop if the remote repository cannot be authenticated.

- [ ] **Step 2: Write failing rendered-HTML tests**

Require `data-theme-source`, deferred runtime after baseline code, channel URL,
SRI, fixed-size managed brand hook, exactly one campaign toggle, and unchanged
metadata/favicon fallback.

- [ ] **Step 3: Run the site test and prove enrollment is absent**

Run: `cd site && npm test`

Expected: FAIL.

- [ ] **Step 4: Separate dark-mode and campaign controls**

Keep `theme-toggle.tsx` responsible only for its existing color-scheme state.
Add campaign toggle markup as a separate component or server-rendered element
owned by the runtime. It may not write the dark-mode preference key.

- [ ] **Step 5: Convert only the replaceable header brand to a URL hook**

Preserve decorative/content inline brand specimens that should not change.
Render the header logo from the catalog-selected immutable fallback with fixed
dimensions and `data-asset-brand="logo"`.

- [ ] **Step 6: Align generator and Vinext output**

Make both build paths emit the same root marker, hooks, script ordering, and
fallback URLs. Extend `rendered-html.test.mjs` to compare the contract rather
than implementation formatting.

- [ ] **Step 7: Run Pajé gates**

Run:

```bash
cd site
npm test
npm run lint
npm run build
```

Run the root Go tests named by current CI after Go toolchain alignment.

Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add go.mod site .github/workflows/deploy-site.yml
git commit -m "feat(site): adopt seasonal asset fallback"
```

### Task 4: Enroll Xisnove after active milestone integration

**Repository:** `xisnove`

**Base:** The current primary checkout is a heavily dirty `codex/milestone-4a-control-plane` branch and contains an untracked `site/`. Do not mutate it. The control plane must first identify the accepted milestone integration SHA and the owning branch for `site/`.

**Files:**
- Modify after ownership resolution: `ui/go.mod`, `ui/go.sum`
- Modify: `ui/internal/view/pages.templ`
- Modify: `ui/internal/view/pages_test.go`
- Modify: `ui/internal/web/server.go`
- Modify: `ui/internal/web/server_test.go`
- Modify catalog-selected files below `ui/internal/web/static/`
- Regenerate: `ui/internal/view/pages_templ.go`
- Modify accepted tracked equivalents of `site/home.templ`, `site/brand.go`, `site/cmd/x9-site/main.go`, and tests.

**Interfaces:**
- Preserve existing `xisnove-theme` preference semantics.
- Preserve nonce CSP and exact same-origin asset routes.
- Mark only Xisnove-owned fixed-size header/side-nav logo, mark, and favicon assets.
- Backend API, server, agent, operator, and CLI remain unchanged.

- [ ] **Step 1: Resolve branch ownership before creating a worktree**

Inspect live control-plane sessions and repository branches. Require the active
milestone owner to integrate or explicitly hand off. Require `site/` to exist in
the chosen committed base. Record SHA and owner in the ledger.

- [ ] **Step 2: Write failing CSP and enrollment tests**

```go
for _, want := range []string{
	`data-theme-source="default"`,
	`data-asset-brand="logo"`,
	`data-campaign-toggle`,
	`src="/assets/campaign/v1.js"`,
} {
	if !strings.Contains(body, want) { t.Errorf("missing %q", want) }
}
if strings.Contains(csp, "'unsafe-inline'") { t.Fatal("unsafe CSP regression") }
```

Require the external runtime route to be present in `script-src` under existing
nonce/same-origin rules. Test all three responsive brand images and favicon.

- [ ] **Step 3: Run focused tests and prove enrollment is absent**

Run:

```bash
cd ui
GOWORK=off go test ./internal/view ./internal/web -count=1
```

Expected: FAIL.

- [ ] **Step 4: Add preference source and explicit runtime**

Extend the existing first-paint script: only a stored `xisnove-theme` marks
`preference`. Add deferred same-origin runtime, channel, and SRI without inline
runtime code.

- [ ] **Step 5: Add bounded managed brand hooks**

Reuse existing hashed same-origin handlers for the embedded fallback. Preserve
the three current responsive assets and their fixed dimensions. Mark Xisnove-
owned nodes only.

- [ ] **Step 6: Adopt the same contract in the committed static site**

Update its source templates and embedded V11 fallback. Regenerate output through
`cmd/x9-site`; do not copy the dirty untracked primary `site/` directory.

- [ ] **Step 7: Run focused and full UI/site gates**

Run:

```bash
go run github.com/a-h/templ/cmd/templ generate
cd ui
GOWORK=off go test ./... -count=1
cd ../site
GOWORK=off go test ./... -count=1
```

Run the existing browser smoke focused on shell/theme/CSP. Expected: PASS.

- [ ] **Step 8: Commit**

```bash
git add ui site
git commit -m "feat(ui): adopt seasonal asset fallback"
```

### Task 5: Align remaining Go toolchains without visual churn

**Repositories:** `metaru`, `paje`, and any touched module not already selecting Go 1.26.5.

**Files:**
- Modify: `metaru/go.mod`
- Modify: `paje/go.mod`
- Modify additional `go.mod`, `go.sum`, and CI workflow files only when the audit proves they select an older compiler.

**Interfaces:**
- Every touched module selects Go toolchain 1.26.5.
- Language-version changes are separate from asset behavior commits.

- [ ] **Step 1: Audit every repository module and CI pin**

Run a sorted scan of all `go.mod` and workflow setup-go values below the Arai Hû
workspace, excluding `.git`, vendor, and unrelated worktrees. Record modules
already compliant.

- [ ] **Step 2: Update Metaru toolchain**

Change `toolchain go1.26.3` to `toolchain go1.26.5`. Preserve its chosen `go`
language version unless tests or dependency requirements justify raising it.
Run `GOWORK=off go test ./... -count=1`.

- [ ] **Step 3: Update Pajé compiler selection**

Make the root module select Go 1.26.5 consistently with its site generator.
Run all root module tests and the site generator tests.

- [ ] **Step 4: Verify Assets and Xisnove exact toolchains**

Their `go 1.26.0` plus `toolchain go1.26.5` already selects the current compiler.
Do not churn language versions merely for visual consistency. Require CI's
`go version` output to be `go1.26.5`.

- [ ] **Step 5: Commit one isolated toolchain change per repository**

```bash
git add go.mod go.sum .github/workflows
git commit -m "build: select Go 1.26.5"
```

Do not combine Metaru and Pajé history; each repository gets its own commit and
verification evidence.

### Task 6: Cross-consumer fallback audit

**Repositories:** All relevant consumers above.

**Files:**
- Create or modify each repository's asset integration documentation.
- Modify only defects found by the audit in owned implementation files.

**Interfaces:**
- Produces a matrix of application, Assets release, catalog hash, managed hooks,
  preference key, runtime URL/SRI, fallback test, and update command.

- [ ] **Step 1: Verify exact released dependency graph**

Reject local replaces except a repository's established self-module replace.
Require tagged Goshtoso and App Shell versions in all downstream modules.

- [ ] **Step 2: Verify fallback hashes against Assets `release.json`**

For every copied SVG/CSS/PNG, select the canonical catalog or themes entry and
compare SHA-256. Reject files copied from concepts, review scaffolds, or source
masters.

- [ ] **Step 3: Verify preference ownership**

Record each existing key and prove only the owning app reads/writes it. The
campaign runtime reads only its namespaced campaign opt-out key and root marker.

- [ ] **Step 4: Run all repository full gates**

Run the exact commands in Tasks 1-5 from clean worktrees. Record commit SHA,
command, exit code, and duration. Do not claim a repository passes from a
different branch.

- [ ] **Step 5: Hand accepted commits to the control plane**

For each repository return base SHA, commit range, changed paths, focused/full
evidence, and any intentional exclusions. Do not push, tag, or deploy in this
task.
