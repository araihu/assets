# Goshtoso Icon Catalog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a generic accessible sprite icon component, bundled typed Heroicons defaults, and Goshtoso’s first generator for assets schema-v1 catalogs.

**Architecture:** The root module renders safe same-origin or inline-document `<use>` references without owning brand geometry. A standard-library generator validates a pinned assets catalog fixture and emits typed `icon.Symbol` constants; the same command can generate project-local Arai Hû bindings without placing brand names in Goshtoso.

**Tech Stack:** Go 1.26.5, templ v0.3.1020, Tailwind CSS v4, standard-library JSON and `go/format`, Goshtoso demo/E2E stack.

## Global Constraints

- Work only in `/tmp/gs-icon-catalog`, branched from freshly fetched `origin/main`; preserve the dirty primary checkout.
- Root and `site/` modules declare `go 1.26.5` and `toolchain go1.26.5`; CI runs that exact patch.
- Consume the immutable assets release-candidate catalog/sprite hashes recorded in the control-plane ledger.
- Add no third-party Go runtime dependency.
- Generic component API contains no Arai Hû product names.
- Default Heroicons constants have values equal to canonical `hi-*` sprite symbols.
- Blank label or `Decorative: true` is decorative; a nonblank label with `Decorative: false` is an image.
- Do not force fill/stroke on the root SVG; compatible sprite content inherits CSS `color` itself.
- Never hand-edit `*_templ.go`, generated skill references, or generated CSS.
- Same-origin external sprites are supported; cross-origin external `<use>` remains deployment-dependent.

---

## File map

- `components/icon/{types.go,component.go,icon.templ,icon_templ.go,icon_test.go}` — public component.
- `components/icon/heroicons/{names_gen.go,names_test.go}` — generated defaults.
- `components/component.go` and inventory/identity tests — `KindIcon` registration.
- `assets/icons/{heroicons.svg,HEROICONS_LICENSE.txt}` and `assets/embed.go` — bundled default assets.
- `internal/iconcatalog/{catalog.go,generate.go,main.go,*_test.go,testdata/**}` — generator implementation.
- `cmd/iconcatalog/main.go` — generator command.
- `site/internal/pages/demo/components/icon.templ` and generated file — demos.
- Demo registry/catalog/attribution files and `site/tests/e2e/icon_test.go` — discoverability and proof.
- README, usage docs, agent skill, CI — public guidance and drift gates.

### Task 1: Establish Go 1.26.5 and catalog generator contract

**Files:** Modify root/site `go.mod`, CI toolchain config; create `internal/iconcatalog/{catalog.go,generate.go,main.go,catalog_test.go,generate_test.go,testdata/catalog.json,testdata/names.golden}` and `cmd/iconcatalog/main.go`.

**Interfaces:** Produce `Load(io.Reader) (Catalog, error)`, `Generate(Catalog, Options) ([]byte, error)`, and `Run([]string, io.Writer, io.Writer) error`.

- [ ] **Step 1: Create the mandated worktree and toolchain**

```bash
git fetch origin
git worktree add -b feat/icon-catalog /tmp/gs-icon-catalog origin/main
cd /tmp/gs-icon-catalog
go work init . ./site
go mod edit -go=1.26.5 -toolchain=go1.26.5
(cd site && go mod edit -go=1.26.5 -toolchain=go1.26.5)
GOTOOLCHAIN=auto go version
```

Expected: `go version go1.26.5`.

- [ ] **Step 2: Write failing schema and golden tests**

```go
func TestLoadRejectsUnsupportedSchemaAndDuplicateSymbol(t *testing.T) {
	_, err := Load(strings.NewReader(`{"schemaVersion":2,"assets":[]}`))
	require.ErrorContains(t, err, "unsupported schemaVersion")
	_, err = Load(strings.NewReader(duplicateSymbolFixture))
	require.ErrorContains(t, err, "duplicate spriteSymbol")
}
func TestGenerateTypedSymbolsDeterministically(t *testing.T) {
	got, err := Generate(fixture(t), Options{Package:"heroicons", Namespace:"ui", Product:"heroicons", SpriteURL:"/assets/icons/heroicons.svg", ConstPrefix:"Icon"})
	require.NoError(t, err); require.Equal(t, golden(t, "names.golden"), got)
}
```

- [ ] **Step 3: Implement the standard-library generator**

```go
type Options struct { CatalogPath, OutputPath, Package, Namespace, Product, SpriteURL, ConstPrefix string; Check bool }
func Load(r io.Reader) (Catalog, error)
func Generate(c Catalog, opts Options) ([]byte, error)
func Run(args []string, stdout, stderr io.Writer) error
```

Use the exact assets JSON tags. Reject unsupported schema, empty/duplicate names or symbols, identifier collisions, non-SVG/non-sprite selections, and incompatible color behavior. Sort by canonical name then symbol; emit `go/format` source and source catalog hash comment. `-check` compares bytes without writing.

- [ ] **Step 4: Pass, tidy, and commit**

```bash
go test ./internal/iconcatalog -count=1
go mod tidy && (cd site && go mod tidy)
git add go.mod go.sum site/go.mod site/go.sum internal/iconcatalog cmd/iconcatalog .github
git commit -m "feat(icon): add catalog binding generator"
```

### Task 2: Implement the generic sprite component

**Files:** Create component files/tests; modify `components/component.go`, `display_identity_test.go`, `component_test.go`, `public_renderable_inventory_test.go`.

**Interfaces:** Public API below.

```go
type Symbol string
type Mode string
const (ModeExternal Mode = ""; ModeInline Mode = "inline")
type Size string
const (SizeXS Size="xs"; SizeSM Size="sm"; SizeMD Size="md"; SizeLG Size="lg"; SizeXL Size="xl")
type Config struct { SpriteURL string; Symbol Symbol; Size Size; Label string; Decorative bool; RootClass string; Mode Mode }
func Icon(Config) Instance
```

- [ ] **Step 1: Write failing accessibility, validation, and escaping tests**

```go
func TestAccessibilityMatrix(t *testing.T) {
	require.Contains(t, render(t, Config{SpriteURL:"/s.svg", Symbol:"check", Label:"Approved"}), `role="img" aria-label="Approved"`)
	require.Contains(t, render(t, Config{SpriteURL:"/s.svg", Symbol:"check", Decorative:true}), `aria-hidden="true"`)
	require.NotContains(t, render(t, Config{SpriteURL:"/s.svg", Symbol:"check", Decorative:true}), `role="img"`)
}
func TestRenderRejectsMissingOrUnsafeReference(t *testing.T) {
	require.Error(t, renderErr(Config{}))
	require.Error(t, renderErr(Config{SpriteURL:"javascript:x", Symbol:"check"}))
	require.Error(t, renderErr(Config{SpriteURL:"/s.svg", Symbol:`bad\"x`}))
}
```

- [ ] **Step 2: Implement types, validation, and templ source**

External href is `SpriteURL + "#" + Symbol`; inline-document href is `"#" + Symbol`. Require URL for external mode; allow relative/http/https only. Validate symbol as lower ASCII letters/digits/hyphens. Map sizes to fixed classes and append escaped `RootClass`. Do not expose arbitrary root attributes.

- [ ] **Step 3: Register component identity**

Add `KindIcon`, update `allKinds`/subset/order assertions, inventory total from 81 to 82, render-method total to 164, and provide a valid icon instance in display-identity tests.

- [ ] **Step 4: Generate, test, and commit**

```bash
templ generate
go test ./components/icon ./components -count=1
git add components/icon components/component.go components/*test.go
git commit -m "feat(icon): render accessible sprite symbols"
```

### Task 3: Bundle the released Heroicons sprite and typed names

**Files:** Add bundled sprite/license/names/tests; modify `assets/embed.go` and asset handler tests.

**Interfaces:** Produce `heroicons.SpriteURL = "/assets/icons/heroicons.svg"` and constants such as `Icon16SolidAcademicCap icon.Symbol = "hi-16-solid-academic-cap"`.

- [ ] **Step 1: Copy only ledger-verified release files**

Copy the UI sprite, catalog fixture/subset, and exact MIT notice from the recorded assets release-candidate commit. Verify all three hashes before staging.

- [ ] **Step 2: Generate names and write resolution tests**

```bash
go run ./cmd/iconcatalog -catalog internal/iconcatalog/testdata/heroicons-catalog.json \
  -namespace ui -product heroicons -sprite-url /assets/icons/heroicons.svg \
  -package heroicons -const-prefix Icon -out components/icon/heroicons/names_gen.go
```

```go
func TestEveryGeneratedSymbolExistsExactlyOnceInSprite(t *testing.T) { requireGeneratedNamesResolve(t) }
func TestHandlerServesSpriteAndLicense(t *testing.T) { requireAsset200(t, "/assets/icons/heroicons.svg") }
```

- [ ] **Step 3: Add embed paths and drift check**

Embed `assets/icons/**`; verify sprite content type and immutable response behavior through existing handler conventions. Run generator with `-check` in CI.

- [ ] **Step 4: Pass and commit**

```bash
go test ./components/icon/heroicons ./assets ./internal/iconcatalog -count=1
git add assets/icons assets/embed.go assets/*test.go components/icon/heroicons internal/iconcatalog/testdata
git commit -m "feat(icon): bundle typed Heroicons defaults"
```

### Task 4: Add demo, catalog, attribution, and documentation

**Files:** Create icon demo source/generated file; modify demo registry, component catalog/tests, attribution source/generated file, README, `docs/USAGE.md`, and `.agents/skills/using-goshtoso/SKILL.md` plus generated compatibility reference.

**Interfaces:** Demo shows external/inline-document, accessible/decorative, size, and current-color-through-CSS variants.

- [ ] **Step 1: Write failing catalog registration test**

```go
func TestCatalogContainsIconInDisplaySection(t *testing.T) {
	e := requireEntry(t, components.KindIcon)
	require.Equal(t, "github.com/araihu/goshtoso/components/icon", e.Package)
	require.Equal(t, "Display", e.Section)
}
```

- [ ] **Step 2: Build the demo with existing component conventions**

Use one preview and code block per variant. Demonstrate `color` inheritance without root fill overrides. Attribution names Heroicons, MIT, bundled sprite path, and retained notice.

- [ ] **Step 3: Document generic and generated usage**

Document same-origin external sprite expectation, inline-document mode, label/decorative invariant, project-local generator example, catalog schema rejection, and absence of Arai Hû names from Goshtoso.

- [ ] **Step 4: Regenerate and commit**

```bash
templ generate
go run ./cmd/skillgen
just css
go test ./site/internal/pages/catalog/... -count=1
git add site README.md docs .agents .claude assets/styles.css
git commit -m "docs(icon): publish sprite component guidance"
```

### Task 5: Prove the component in a real browser

**Files:** Create `site/tests/e2e/icon_test.go`; modify CI drift gates.

**Interfaces:** Prove demo routing, symbol rendering, accessibility, current color, asset serving, and console cleanliness.

- [ ] **Step 1: Write the failing E2E test**

```go
func TestIcon(t *testing.T) {
	page := newPage(t, browser)
	require.NoError(t, page.Goto(serverURL+"/components/icon"))
	requireAttribute(t, page, `[data-variant="accessible"] svg`, "role", "img")
	requireAttribute(t, page, `[data-variant="decorative"] svg`, "aria-hidden", "true")
	requireSymbolPaintsCurrentColor(t, page, `[data-variant="current-color"]`)
	requireNoConsoleErrors(t, page)
}
```

- [ ] **Step 2: Run focused browser and module gates**

```bash
go test ./site/tests/e2e/... -count=1 -timeout 5m -run TestIcon
go test ./... -count=1
(cd site && go test $(go list ./... | grep -v /tests/e2e) -count=1)
```

- [ ] **Step 3: Run generated-file and quality gates**

```bash
go run ./cmd/iconcatalog -catalog internal/iconcatalog/testdata/heroicons-catalog.json -namespace ui -product heroicons -sprite-url /assets/icons/heroicons.svg -package heroicons -const-prefix Icon -out components/icon/heroicons/names_gen.go -check
go run ./cmd/vendorgen -check
golangci-lint run
(cd site && golangci-lint run)
go vet ./...
(cd site && go vet ./...)
git diff --exit-code
```

- [ ] **Step 4: Commit**

```bash
git add site/tests/e2e/icon_test.go .github/workflows
git commit -m "test(icon): verify sprite rendering end to end"
```

### Task 6: Prepare Goshtoso release evidence

**Files:** Modify release notes/checklist and changelog according to existing release workflow.

**Interfaces:** Produce a reviewed commit suitable for a Goshtoso release; do not tag or update consumers yet.

- [ ] **Step 1: Run complete gates with Go 1.26.5**

```bash
GOTOOLCHAIN=auto go version
templ generate
just css
go run ./cmd/skillgen
go test ./... -count=1
(cd site && go test $(go list ./... | grep -v /tests/e2e) -count=1)
go test ./site/tests/e2e/... -count=1 -timeout 15m
golangci-lint run
(cd site && golangci-lint run)
go build -o bin/server ./site/cmd/server
git diff --exit-code
```

- [ ] **Step 2: Record compatibility inputs**

Record assets release-candidate commit, catalog hash, UI sprite hash, schema version, default sprite path, generator command, and the Goshtoso commit in the control-plane ledger.

- [ ] **Step 3: Commit release notes without tagging**

```bash
git add CHANGELOG.md docs
git commit -m "docs: prepare icon catalog release"
```

Tagging and the `ahairu` dependency update wait for user approval and the cross-repository gate.
